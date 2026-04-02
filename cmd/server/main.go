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
	"github.com/prometheus/client_golang/prometheus/promhttp"

	handler "url_shortener/internal/handler/http"
	"url_shortener/internal/midleware"
	"url_shortener/internal/pkg/config"
	"url_shortener/internal/pkg/logger"
	repoMemory "url_shortener/internal/repository/memory"
	"url_shortener/internal/usecase"
	"url_shortener/internal/utils/bloom"
	cache "url_shortener/internal/utils/cache/memory"
	limiter "url_shortener/internal/utils/rate_limiter/memory"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	fmt.Println(cfg)
	logger.Init()
	slog.Info("starting url shortener", "storage_type: ", cfg.StorageType)

	//ctx := context.Background()

	if cfg.StorageType != "memory" {
		fmt.Println(cfg.StorageType)
		slog.Error("only memory storage supported in this main version")
		os.Exit(1)
	}

	repo := repoMemory.NewRepository()
	defer repo.Close()

	cache := cache.NewCache()
	defer cache.Close()

	bloomFilter := bloom.NewBloomFilter(cfg.BloomN, cfg.BloomP)

	rateLimiter := limiter.NewRateLimiter(cfg.RateLimitMax, cfg.RateLimitWindow)
	defer rateLimiter.Close()

	shortenerService := usecase.NewShortenerService(
		repo,
		cache,
		bloomFilter,
		cfg.ShortCodeLength,
		cfg.Domain,
	)

	h := handler.NewHandler(shortenerService)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(midleware.RateLimitMiddleware(rateLimiter))

	router.POST("/shorten", h.Shorten)
	router.GET("/:code", h.Redirect)

	if cfg.EnableMetrics {
		router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("server started", "port", cfg.Port)
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
