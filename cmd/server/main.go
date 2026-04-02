package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"

	handler "url_shortener/internal/handler/http"
	"url_shortener/internal/middleware"
	"url_shortener/internal/pkg/config"
	"url_shortener/internal/pkg/logger"
	"url_shortener/internal/repository"
	repoMemory "url_shortener/internal/repository/memory"
	repoPostgres "url_shortener/internal/repository/postgres"
	"url_shortener/internal/usecase"
	"url_shortener/internal/utils/bloom"
	cacheRedis "url_shortener/internal/utils/cache/redis"
	limiterRedis "url_shortener/internal/utils/rate_limiter/redis"
)

const rateLimitPath = "scripts/rate_limit.lua"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Init()
	slog.Info("starting url shortener",
		"storage_type", cfg.Shortener,
		"port", cfg.HTTP.Port,
		"enable_metrics", cfg.HTTP.EnableMetrics,
	)

	ctx := context.Background()

	var repo repository.Repository
	switch cfg.Storage.Type {
	case "postgres":
		dbConfig, err := pgxpool.ParseConfig(cfg.Postgres.DSN)
		if err != nil {
			slog.Error("failed to parse database url", "error", err)
			os.Exit(1)
		}
		dbConfig.MaxConns = cfg.Postgres.MaxConns
		dbConfig.ConnConfig.ConnectTimeout = cfg.Postgres.ConnTimeout

		pool, err := pgxpool.NewWithConfig(ctx, dbConfig)
		if err != nil {
			slog.Error("failed to connect to postgres", "error", err)
			os.Exit(1)
		}
		repo = repoPostgres.NewRepository(pool)

	case "memory":
		repo = repoMemory.NewRepository()

	default:
		slog.Error("unsupported storage type", "storage_type", cfg.Storage.Type)
		os.Exit(1)
	}

	defer repo.Close()

	if cfg.Redis.Addr == "" {
		slog.Error("Redis address is required for cache and rate limiter")
		os.Exit(1)
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		DialTimeout:  cfg.Redis.DialTimeout,
		ReadTimeout:  cfg.Redis.ReadTimeout,
		WriteTimeout: cfg.Redis.WriteTimeout,
	})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		slog.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}

	cache := cacheRedis.NewCache(redisClient)
	slog.Info("using redis cache", "addr", cfg.Redis.Addr)

	rate := float64(cfg.RateLimiter.MaxRequests) / cfg.RateLimiter.Window.Seconds()
	rateLimiter, err := limiterRedis.NewRateLimiter(redisClient, rate, cfg.RateLimiter.MaxRequests, 1, cfg.RateLimiter.ScriptPath)
	if err != nil {
		slog.Error("failed to create rate limiter", "error", err)
		os.Exit(1)
	}
	slog.Info("using redis rate limiter", "max_req", cfg.RateLimiter.MaxRequests, "window_sec", cfg.RateLimiter.Window.Seconds())

	bloomFilter := bloom.NewBloomFilter(cfg.Bloom.ExpectedItems, cfg.Bloom.FalsePositiveProb)
	slog.Info("bloom filter initialized", "expected_items", cfg.Bloom.ExpectedItems, "false_positive_prob", cfg.Bloom.FalsePositiveProb)

	shortenerService := usecase.NewShortenerService(
		repo,
		cache,
		bloomFilter,
		cfg.Shortener.CodeLength,
		cfg.Shortener.Domain,
	)

	h := handler.NewHandler(shortenerService)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RateLimitMiddleware(rateLimiter))

	router.POST("/shorten", h.Shorten)
	router.GET("/:code", h.Redirect)

	if cfg.HTTP.EnableMetrics {
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
		slog.Info("metrics endpoint enabled", "path", "/metrics")
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}

	go func() {
		slog.Info("server started", "port", cfg.HTTP.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	slog.Info("server stopped")
}
