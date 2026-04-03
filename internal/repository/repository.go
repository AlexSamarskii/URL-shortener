package repository

import (
	"context"
	"github.com/AlexSamarskii/URL-shortener/internal/entity"
)

type Repository interface {
	CreateURL(ctx context.Context, url *entity.URL) error
	GetURLByShortCode(ctx context.Context, shortCode string) (*entity.URL, error)
	GetURLByOriginalURL(ctx context.Context, originalURL string) (*entity.URL, error)
	CheckShortCodeExists(ctx context.Context, shortCode string) (bool, error)
	Close()
}
