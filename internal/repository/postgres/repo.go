package postgres

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AlexSamarskii/URL-shortener/internal/entity"
)

type Repository struct {
	db     *pgxpool.Pool
	stopCh chan struct{}
	once   sync.Once
}

func NewRepository(db *pgxpool.Pool) *Repository {
	r := &Repository{
		db:     db,
		stopCh: make(chan struct{}),
	}
	go r.cleanupExpired()
	return r
}

func (r *Repository) Close() {
	r.once.Do(func() {
		close(r.stopCh)
	})
}

func (r *Repository) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, _ = r.db.Exec(ctx, `DELETE FROM urls WHERE expires_at < NOW()`)
			cancel()
		case <-r.stopCh:
			return
		}
	}
}

func (r *Repository) CreateURL(ctx context.Context, url *entity.URL) (string, error) {
	query := `
        INSERT INTO urls (short_code, original_url, expires_at, created_at)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (original_url) DO NOTHING
        RETURNING short_code
    `
	var shortCode string
	err := r.db.QueryRow(ctx, query,
		url.ShortCode, url.OriginalURL, url.ExpiresAt, url.CreatedAt,
	).Scan(&shortCode)

	if err == nil {
		return shortCode, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		var existingCode string
		getQuery := `SELECT short_code FROM urls WHERE original_url = $1`
		err = r.db.QueryRow(ctx, getQuery, url.OriginalURL).Scan(&existingCode)
		if err != nil {
			return "", err
		}
		return existingCode, nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "urls_pkey" {
		return "", entity.ErrAlreadyExists
	}
	return "", err
}

func (r *Repository) GetURLByShortCode(ctx context.Context, shortCode string) (*entity.URL, error) {
	query := `
		SELECT short_code, original_url, expires_at, created_at
		FROM urls
		WHERE short_code = $1
	`
	var url entity.URL
	var expiresAt *time.Time
	err := r.db.QueryRow(ctx, query, shortCode).Scan(
		&url.ShortCode,
		&url.OriginalURL,
		&expiresAt,
		&url.CreatedAt,
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
		SELECT short_code, original_url, expires_at, created_at
		FROM urls
		WHERE original_url = $1
		LIMIT 1
	`
	var url entity.URL
	var expiresAt *time.Time
	err := r.db.QueryRow(ctx, query, originalURL).Scan(
		&url.ShortCode,
		&url.OriginalURL,
		&expiresAt,
		&url.CreatedAt,
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
