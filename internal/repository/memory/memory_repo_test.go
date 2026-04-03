package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AlexSamarskii/URL-shortener/internal/entity"
)

const (
	testOriginalURL = "https://career.ozon.ru/fintech/vacancy?id=131698788"
	testShortCode   = "test123456"
)

func TestCreateURL_Success(t *testing.T) {
	r := NewRepository().(*repo)
	ctx := context.Background()

	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
	}
	err := r.CreateURL(ctx, url)
	require.NoError(t, err)

	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.urls[testShortCode]
	assert.True(t, ok)
	code, ok := r.orig[testOriginalURL]
	assert.True(t, ok)
	assert.Equal(t, testShortCode, code)
}

func TestCreateURL_DuplicateShortCode(t *testing.T) {
	r := NewRepository().(*repo)
	ctx := context.Background()

	url1 := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
	}
	err := r.CreateURL(ctx, url1)
	require.NoError(t, err)

	url2 := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: "https://another.com",
	}
	err = r.CreateURL(ctx, url2)
	assert.ErrorIs(t, err, entity.ErrAlreadyExists)
}

func TestGetURLByShortCode_Success(t *testing.T) {
	r := NewRepository().(*repo)
	ctx := context.Background()

	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
	}
	err := r.CreateURL(ctx, url)
	require.NoError(t, err)

	got, err := r.GetURLByShortCode(ctx, testShortCode)
	require.NoError(t, err)
	assert.Equal(t, testOriginalURL, got.OriginalURL)
}

func TestGetURLByShortCode_NotFound(t *testing.T) {
	r := NewRepository().(*repo)
	ctx := context.Background()

	_, err := r.GetURLByShortCode(ctx, "nonexistent")
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestGetURLByShortCode_Expired(t *testing.T) {
	r := NewRepository().(*repo)
	ctx := context.Background()

	expiredAt := time.Now().UTC().Add(-1 * time.Hour)
	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
		ExpiresAt:   &expiredAt,
	}
	err := r.CreateURL(ctx, url)
	require.NoError(t, err)

	_, err = r.GetURLByShortCode(ctx, testShortCode)
	assert.ErrorIs(t, err, entity.ErrExpired)

	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.urls[testShortCode]
	assert.False(t, ok)
	_, ok = r.orig[testOriginalURL]
	assert.False(t, ok)
}

func TestGetURLByOriginalURL_Success(t *testing.T) {
	r := NewRepository().(*repo)
	ctx := context.Background()

	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
	}
	err := r.CreateURL(ctx, url)
	require.NoError(t, err)

	got, err := r.GetURLByOriginalURL(ctx, testOriginalURL)
	require.NoError(t, err)
	assert.Equal(t, testShortCode, got.ShortCode)
}

func TestGetURLByOriginalURL_InconsistentIndex(t *testing.T) {
	r := NewRepository().(*repo)
	ctx := context.Background()

	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
	}
	err := r.CreateURL(ctx, url)
	require.NoError(t, err)

	r.mu.Lock()
	delete(r.urls, testShortCode)
	r.mu.Unlock()

	_, err = r.GetURLByOriginalURL(ctx, testOriginalURL)
	assert.ErrorIs(t, err, entity.ErrNotFound)

	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.orig[testOriginalURL]
	assert.False(t, ok, "orig должен быть очищен от мусорного ключа")
}

func TestGetURLByOriginalURL_NotFound(t *testing.T) {
	r := NewRepository().(*repo)
	ctx := context.Background()

	_, err := r.GetURLByOriginalURL(ctx, "https://notfound.com")
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestGetURLByOriginalURL_Expired(t *testing.T) {
	r := NewRepository().(*repo)
	ctx := context.Background()

	expiredAt := time.Now().UTC().Add(-1 * time.Hour)
	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
		ExpiresAt:   &expiredAt,
	}
	err := r.CreateURL(ctx, url)
	require.NoError(t, err)

	_, err = r.GetURLByOriginalURL(ctx, testOriginalURL)
	assert.ErrorIs(t, err, entity.ErrExpired)

	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.urls[testShortCode]
	assert.False(t, ok)
	_, ok = r.orig[testOriginalURL]
	assert.False(t, ok)
}

func TestCheckShortCodeExists(t *testing.T) {
	r := NewRepository().(*repo)
	ctx := context.Background()

	exists, err := r.CheckShortCodeExists(ctx, testShortCode)
	require.NoError(t, err)
	assert.False(t, exists)

	url := &entity.URL{
		ShortCode:   testShortCode,
		OriginalURL: testOriginalURL,
	}
	err = r.CreateURL(ctx, url)
	require.NoError(t, err)

	exists, err = r.CheckShortCodeExists(ctx, testShortCode)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestClose(t *testing.T) {
	r := NewRepository().(*repo)
	r.Close()
	r.Close()
	_, ok := <-r.stopCh
	assert.False(t, ok, "stopCh должен быть закрыт")
}

func TestIsExpired(t *testing.T) {
	now := time.Now().UTC()
	urlNil := &entity.URL{ExpiresAt: nil}
	assert.False(t, isExpired(urlNil))

	future := now.Add(1 * time.Hour)
	urlFuture := &entity.URL{ExpiresAt: &future}
	assert.False(t, isExpired(urlFuture))

	past := now.Add(-1 * time.Hour)
	urlPast := &entity.URL{ExpiresAt: &past}
	assert.True(t, isExpired(urlPast))
}
