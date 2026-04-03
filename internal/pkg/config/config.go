package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v2"
)

type HTTPConfig struct {
	Port          int           `yaml:"port"`
	ReadTimeout   time.Duration `yaml:"readTimeout"`
	WriteTimeout  time.Duration `yaml:"writeTimeout"`
	IdleTimeout   time.Duration `yaml:"idleTimeout"`
	EnableMetrics bool          `yaml:"enableMetrics"`
}

type ShortenerConfig struct {
	Domain     string `yaml:"domain"`
	CodeLength int    `yaml:"codeLength"`
}

type PostgresConfig struct {
	Host        string        `yaml:"host"`
	Port        string        `yaml:"port"`
	User        string        `yaml:"user"`
	Password    string        `yaml:"-"`
	DBName      string        `yaml:"dbname"`
	SSLMode     string        `yaml:"sslmode"`
	MaxConns    int32         `yaml:"maxConns"`
	ConnTimeout time.Duration `yaml:"connTimeout"`
	DSN         string        `yaml:"-"`
}

type RedisConfig struct {
	Host         string        `yaml:"host"`
	Port         string        `yaml:"port"`
	Password     string        `yaml:"-"`
	DB           int           `yaml:"db"`
	DialTimeout  time.Duration `yaml:"dialTimeout"`
	ReadTimeout  time.Duration `yaml:"readTimeout"`
	WriteTimeout time.Duration `yaml:"writeTimeout"`
	Addr         string        `yaml:"-"`
}

type RateLimiterConfig struct {
	Enabled     bool          `yaml:"enabled"`
	MaxRequests int           `yaml:"maxRequests"`
	Window      time.Duration `yaml:"window"`
	ScriptPath  string        `yaml:"scriptPath"`
}

type BloomConfig struct {
	EnableBloomFilter bool    `yaml:"enableBloomFilter"`
	ExpectedItems     uint    `yaml:"expectedItems"`
	FalsePositiveProb float64 `yaml:"falsePositiveProb"`
}

type StorageConfig struct {
	Type string `yaml:"type"`
}

type Config struct {
	HTTP        HTTPConfig        `yaml:"http"`
	Shortener   ShortenerConfig   `yaml:"shortener"`
	Storage     StorageConfig     `yaml:"storage"`
	Postgres    PostgresConfig    `yaml:"postgres"`
	Redis       RedisConfig       `yaml:"redis"`
	RateLimiter RateLimiterConfig `yaml:"rateLimiter"`
	Bloom       BloomConfig       `yaml:"bloom"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	yamlFile, err := os.ReadFile("configs/config.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(yamlFile, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.HTTP.Port = getEnvInt("PORT", cfg.HTTP.Port)
	}
	if v := os.Getenv("ENABLE_METRICS"); v != "" {
		cfg.HTTP.EnableMetrics = getEnvBool("ENABLE_METRICS", cfg.HTTP.EnableMetrics)
	}

	if v := os.Getenv("DOMAIN"); v != "" {
		cfg.Shortener.Domain = v
	}
	if v := os.Getenv("SHORT_CODE_LENGTH"); v != "" {
		cfg.Shortener.CodeLength = getEnvInt("SHORT_CODE_LENGTH", cfg.Shortener.CodeLength)
	}

	if v := os.Getenv("STORAGE_TYPE"); v != "" {
		cfg.Storage.Type = v
	}

	if cfg.Storage.Type != "postgres" && cfg.Storage.Type != "memory" {
		return nil, fmt.Errorf("storage.type must be 'postgres' or 'memory', got: %s", cfg.Storage.Type)
	}

	cfg.Postgres.Password = os.Getenv("POSTGRES_PASSWORD")
	if cfg.Postgres.Password == "" {
		return nil, fmt.Errorf("POSTGRES_PASSWORD environment variable is required")
	}
	if v := os.Getenv("POSTGRES_HOST"); v != "" {
		cfg.Postgres.Host = v
	}
	if v := os.Getenv("POSTGRES_PORT"); v != "" {
		cfg.Postgres.Port = v
	}
	if v := os.Getenv("POSTGRES_USER"); v != "" {
		cfg.Postgres.User = v
	}
	if v := os.Getenv("POSTGRES_DB"); v != "" {
		cfg.Postgres.DBName = v
	}
	cfg.Postgres.DSN = fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.Postgres.User, cfg.Postgres.Password,
		cfg.Postgres.Host, cfg.Postgres.Port,
		cfg.Postgres.DBName, cfg.Postgres.SSLMode,
	)

	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.Redis.Addr = v
	} else {
		if v := os.Getenv("REDIS_HOST"); v != "" {
			cfg.Redis.Host = v
		}
		if v := os.Getenv("REDIS_PORT"); v != "" {
			cfg.Redis.Port = v
		}
		cfg.Redis.Addr = fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port)
	}
	if v := os.Getenv("REDIS_DIAL_TIMEOUT_SEC"); v != "" {
		cfg.Redis.DialTimeout = time.Duration(getEnvInt("REDIS_DIAL_TIMEOUT_SEC", int(cfg.Redis.DialTimeout.Seconds()))) * time.Second
	}
	if v := os.Getenv("REDIS_READ_TIMEOUT_SEC"); v != "" {
		cfg.Redis.ReadTimeout = time.Duration(getEnvInt("REDIS_READ_TIMEOUT_SEC", int(cfg.Redis.ReadTimeout.Seconds()))) * time.Second
	}
	if v := os.Getenv("REDIS_WRITE_TIMEOUT_SEC"); v != "" {
		cfg.Redis.WriteTimeout = time.Duration(getEnvInt("REDIS_WRITE_TIMEOUT_SEC", int(cfg.Redis.WriteTimeout.Seconds()))) * time.Second
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		cfg.Redis.DB = getEnvInt("REDIS_DB", cfg.Redis.DB)
	}

	if v := os.Getenv("RATE_LIMIT_MAX"); v != "" {
		cfg.RateLimiter.MaxRequests = getEnvInt("RATE_LIMIT_MAX", cfg.RateLimiter.MaxRequests)
	}
	if v := os.Getenv("RATE_LIMIT_WINDOW_SEC"); v != "" {
		cfg.RateLimiter.Window = time.Duration(getEnvInt("RATE_LIMIT_WINDOW_SEC", int(cfg.RateLimiter.Window.Seconds()))) * time.Second
	}
	if v := os.Getenv("RATE_LIMIT_SCRIPT_PATH"); v != "" {
		cfg.RateLimiter.ScriptPath = v
	}

	if v := os.Getenv("BLOOM_N"); v != "" {
		cfg.Bloom.ExpectedItems = uint(getEnvInt("BLOOM_N", int(cfg.Bloom.ExpectedItems)))
	}
	if v := os.Getenv("BLOOM_P"); v != "" {
		cfg.Bloom.FalsePositiveProb = getEnvFloat("BLOOM_P", cfg.Bloom.FalsePositiveProb)
	}

	return &cfg, nil
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		var i int
		fmt.Sscanf(v, "%d", &i)
		return i
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if v := os.Getenv(key); v != "" {
		var f float64
		fmt.Sscanf(v, "%f", &f)
		return f
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if v := os.Getenv(key); v != "" {
		return v == "true"
	}
	return defaultVal
}
