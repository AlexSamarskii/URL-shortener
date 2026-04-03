package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/AlexSamarskii/URL-shortener/internal/entity"
)

const (
	testOriginalURL = "https://career.ozon.ru/fintech/vacancy?id=131698788"
	testShortCode   = "test123456"
)

func setupPostgresContainer(t *testing.T) (*pgxpool.Pool, func()) {
	ctx := context.Background()

	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	// Создаём таблицу в упрощённой схеме (без id и updated_at, short_code как PK)
	_, err = pool.Exec(ctx, `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE TABLE IF NOT EXISTS urls (
			short_code VARCHAR(255) PRIMARY KEY,
			original_url TEXT NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_urls_original_url ON urls(original_url);
	`)
	require.NoError(t, err)

	cleanup := func() {
		pool.Close()
		postgresContainer.Terminate(ctx)
	}
	return pool, cleanup
}

func TestCreateURL_Success(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)
	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
		ExpiresAt:   &expiresAt,
		CreatedAt:   now,
	}

	err := repo.CreateURL(ctx, url)
	require.NoError(t, err)

	got, err := repo.GetURLByShortCode(ctx, testShortCode)
	require.NoError(t, err)
	assert.Equal(t, testOriginalURL, got.OriginalURL)
	assert.Equal(t, testShortCode, got.ShortCode)
	assert.WithinDuration(t, expiresAt, *got.ExpiresAt, time.Second)
}

func TestCreateURL_DuplicateShortCode(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(pool)
	ctx := context.Background()

	url1 := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
	}
	err := repo.CreateURL(ctx, url1)
	require.NoError(t, err)

	url2 := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: "https://another.com",
	}
	err = repo.CreateURL(ctx, url2)
	assert.ErrorIs(t, err, entity.ErrAlreadyExists)
}

func TestGetURLByShortCode_Success(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(pool)
	ctx := context.Background()

	now := time.Now().UTC()
	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
		CreatedAt:   now,
	}
	err := repo.CreateURL(ctx, url)
	require.NoError(t, err)

	got, err := repo.GetURLByShortCode(ctx, testShortCode)
	require.NoError(t, err)
	assert.Equal(t, testOriginalURL, got.OriginalURL)
	assert.Equal(t, testShortCode, got.ShortCode)
	assert.Nil(t, got.ExpiresAt)
}

func TestGetURLByShortCode_NotFound(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(pool)
	ctx := context.Background()

	_, err := repo.GetURLByShortCode(ctx, "nonexistent")
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestGetURLByShortCode_Expired(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(pool)
	ctx := context.Background()

	expiredAt := time.Now().UTC().Add(-1 * time.Hour)
	url := &entity.URL{
		ShortCode:   "expired10",
		OriginalURL: testOriginalURL,
		ExpiresAt:   &expiredAt,
	}
	err := repo.CreateURL(ctx, url)
	require.NoError(t, err)

	got, err := repo.GetURLByShortCode(ctx, "expired10")
	require.NoError(t, err)
	assert.NotNil(t, got.ExpiresAt)
	assert.True(t, got.ExpiresAt.Before(time.Now().UTC()))
}

func TestGetURLByOriginalURL_Success(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(pool)
	ctx := context.Background()

	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
	}
	err := repo.CreateURL(ctx, url)
	require.NoError(t, err)

	got, err := repo.GetURLByOriginalURL(ctx, testOriginalURL)
	require.NoError(t, err)
	assert.Equal(t, testShortCode, got.ShortCode)
	assert.Equal(t, testOriginalURL, got.OriginalURL)
}

func TestGetURLByOriginalURL_NotFound(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(pool)
	ctx := context.Background()

	_, err := repo.GetURLByOriginalURL(ctx, "https://notfound.com")
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestGetURLByOriginalURL_Expired(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(pool)
	ctx := context.Background()

	expiredAt := time.Now().UTC().Add(-1 * time.Hour)
	url := &entity.URL{
		ShortCode:   "expired10",
		OriginalURL: testOriginalURL,
		ExpiresAt:   &expiredAt,
	}
	err := repo.CreateURL(ctx, url)
	require.NoError(t, err)

	got, err := repo.GetURLByOriginalURL(ctx, testOriginalURL)
	require.NoError(t, err)
	assert.Equal(t, "expired10", got.ShortCode)
	assert.True(t, got.ExpiresAt.Before(time.Now().UTC()))
}

func TestCheckShortCodeExists(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(pool)
	ctx := context.Background()

	exists, err := repo.CheckShortCodeExists(ctx, testShortCode)
	require.NoError(t, err)
	assert.False(t, exists)

	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
	}
	err = repo.CreateURL(ctx, url)
	require.NoError(t, err)

	exists, err = repo.CheckShortCodeExists(ctx, testShortCode)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestCreateURL_ClosedPool(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()
	repo := NewRepository(pool)

	pool.Close()
	ctx := context.Background()
	url := &entity.URL{
		ShortCode:   "test123",
		OriginalURL: "https://example.com",
	}
	err := repo.CreateURL(ctx, url)
	assert.Error(t, err)
}

func TestGetURLByShortCode_ClosedPool(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()
	repo := NewRepository(pool)

	pool.Close()
	ctx := context.Background()
	_, err := repo.GetURLByShortCode(ctx, "any")
	assert.Error(t, err)
}

func TestGetURLByOriginalURL_ClosedPool(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()
	repo := NewRepository(pool)

	pool.Close()
	ctx := context.Background()
	_, err := repo.GetURLByOriginalURL(ctx, "https://any.com")
	assert.Error(t, err)
}

func TestCheckShortCodeExists_ClosedPool(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()
	repo := NewRepository(pool)

	pool.Close()
	ctx := context.Background()
	_, err := repo.CheckShortCodeExists(ctx, "any")
	assert.Error(t, err)
}

func TestClose(t *testing.T) {
	pool, cleanup := setupPostgresContainer(t)
	defer cleanup()

	repo := NewRepository(pool)
	repo.Close()
	repo.Close()
}
