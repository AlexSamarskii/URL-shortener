package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            int
	DatabaseURL     string
	StorageType     string // "postgres" или "memory"
	RedisAddr       string
	RedisPassword   string
	ShortCodeLength int
	RateLimitMax    int
	RateLimitWindow time.Duration
	BloomN          uint
	BloomP          float64
	Domain          string
	EnableMetrics   bool
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:            getEnvInt("PORT", 8080),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/shortener?sslmode=disable"),
		StorageType:     getEnv("STORAGE_TYPE", "postgres"), // postgres или memory
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		ShortCodeLength: getEnvInt("SHORT_CODE_LENGTH", 10),
		RateLimitMax:    getEnvInt("RATE_LIMIT_MAX", 100),
		RateLimitWindow: time.Duration(getEnvInt("RATE_LIMIT_WINDOW_SEC", 60)) * time.Second,
		BloomN:          uint(getEnvInt("BLOOM_N", 1000000)),
		BloomP:          getEnvFloat("BLOOM_P", 0.001),
		Domain:          getEnv("DOMAIN", "http://localhost:8080"),
		EnableMetrics:   getEnvBool("ENABLE_METRICS", true),
	}

	if cfg.ShortCodeLength < 4 || cfg.ShortCodeLength > 20 {
		return nil, fmt.Errorf("SHORT_CODE_LENGTH must be between 4 and 20")
	}
	if cfg.StorageType != "postgres" && cfg.StorageType != "memory" {
		return nil, fmt.Errorf("STORAGE_TYPE must be postgres or memory")
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return defaultVal
}
