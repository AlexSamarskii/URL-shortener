package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexSamarskii/URL-shortener/internal/entity"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateURL(ctx context.Context, url *entity.URL) error {
	query := `INSERT INTO urls (id, short_code, original_url, expires_at, created_at, updated_at)
              VALUES ($1, $2, $3, $4, $5, $6)`
	_, err := r.db.Exec(ctx, query, url.ID, url.ShortCode, url.OriginalURL, url.ExpiresAt, url.CreatedAt, url.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return entity.ErrAlreadyExists
		}
		return err
	}
	return nil
}

func (r *Repository) GetURLByShortCode(ctx context.Context, shortCode string) (*entity.URL, error) {
	query := `
		SELECT id, short_code, original_url, expires_at, created_at, updated_at
		FROM urls
		WHERE short_code = $1
	`
	var url entity.URL
	var expiresAt *time.Time
	err := r.db.QueryRow(ctx, query, shortCode).Scan(
		&url.ID,
		&url.ShortCode,
		&url.OriginalURL,
		&expiresAt,
		&url.CreatedAt,
		&url.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	url.ExpiresAt = expiresAt
	return &url, nil
}

func (r *Repository) GetURLByOriginalURL(ctx context.Context, originalURL string) (*entity.URL, error) {
	query := `
		SELECT id, short_code, original_url, expires_at, created_at, updated_at
		FROM urls
		WHERE original_url = $1
		LIMIT 1
	`
	var url entity.URL
	var expiresAt *time.Time
	err := r.db.QueryRow(ctx, query, originalURL).Scan(
		&url.ID,
		&url.ShortCode,
		&url.OriginalURL,
		&expiresAt,
		&url.CreatedAt,
		&url.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, entity.ErrNotFound
		}
		return nil, err
	}
	url.ExpiresAt = expiresAt
	return &url, nil
}

func (r *Repository) CheckShortCodeExists(ctx context.Context, shortCode string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM urls WHERE short_code = $1)`
	var exists bool
	err := r.db.QueryRow(ctx, query, shortCode).Scan(&exists)
	return exists, err
}

func (r *Repository) Close() {
	r.db.Close()
}
