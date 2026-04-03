package db

import (
	"context"

	"github.com/AlexSamarskii/URL-shortener/internal/pkg/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPostgresPool(ctx context.Context, cfg *config.PostgresConfig) (*pgxpool.Pool, error) {
	dbConfig, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, err
	}
	dbConfig.MaxConns = cfg.MaxConns
	dbConfig.ConnConfig.ConnectTimeout = cfg.ConnTimeout
	return pgxpool.NewWithConfig(ctx, dbConfig)
}
