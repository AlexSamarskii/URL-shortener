package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/AlexSamarskii/URL-shortener/internal/entity"
	"github.com/AlexSamarskii/URL-shortener/internal/repository/mocks"
	bloommocks "github.com/AlexSamarskii/URL-shortener/internal/utils/bloom/mocks"
	cachemocks "github.com/AlexSamarskii/URL-shortener/internal/utils/cache/mocks"
)

func TestShortenerService_Shorten_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	ctx := context.Background()
	req := ShortenRequest{URL: "https://example.com"}

	mockRepo.EXPECT().GetURLByOriginalURL(ctx, "https://example.com").Return(nil, entity.ErrNotFound)
	mockRepo.EXPECT().CreateURL(gomock.Any(), gomock.Any()).Return(nil)
	mockBloom.EXPECT().Add(gomock.Any())
	mockCache.EXPECT().Set(gomock.Any(), gomock.Any(), "https://example.com", gomock.Any()).Return(nil)

	resp, err := service.Shorten(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ShortCode)
	assert.Equal(t, "http://short.com/"+resp.ShortCode, resp.ShortURL)
	assert.Nil(t, resp.ExpiresAt)
}

func TestShortenerService_Shorten_ExistingURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	existing := &entity.URL{ShortCode: "abc123", OriginalURL: "https://example.com"}
	mockRepo.EXPECT().GetURLByOriginalURL(gomock.Any(), "https://example.com").Return(existing, nil)

	resp, err := service.Shorten(context.Background(), ShortenRequest{URL: "https://example.com"})
	require.NoError(t, err)
	assert.Equal(t, "abc123", resp.ShortCode)
	assert.Equal(t, "http://short.com/abc123", resp.ShortURL)
}

func TestShortenerService_Shorten_InvalidURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	_, err := service.Shorten(context.Background(), ShortenRequest{URL: "invalid"})
	assert.ErrorIs(t, err, entity.ErrURLInvalid)
}

func TestShortenerService_Shorten_WithExpiration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	expiresIn := 3600
	req := ShortenRequest{URL: "https://example.com", ExpiresIn: &expiresIn}

	mockRepo.EXPECT().GetURLByOriginalURL(gomock.Any(), "https://example.com").Return(nil, entity.ErrNotFound)
	mockRepo.EXPECT().CreateURL(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, url *entity.URL) error {
		assert.NotNil(t, url.ExpiresAt)
		return nil
	})
	mockBloom.EXPECT().Add(gomock.Any())
	mockCache.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	resp, err := service.Shorten(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp.ExpiresAt)
}

func TestShortenerService_Shorten_WithAlias_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	alias := "myalias101"
	req := ShortenRequest{URL: "https://example.com", Alias: &alias}

	mockRepo.EXPECT().CreateURL(gomock.Any(), gomock.Any()).Return(nil)
	mockBloom.EXPECT().Add([]byte(alias))
	mockCache.EXPECT().Set(gomock.Any(), cacheKey(alias), "https://example.com", gomock.Any()).Return(nil)

	resp, err := service.Shorten(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, alias, resp.ShortCode)
}

func TestShortenerService_Shorten_WithAlias_AlreadyExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	alias := "existing10"
	req := ShortenRequest{URL: "https://example.com", Alias: &alias}

	mockRepo.EXPECT().CreateURL(gomock.Any(), gomock.Any()).Return(entity.ErrAlreadyExists)

	_, err := service.Shorten(context.Background(), req)
	assert.ErrorIs(t, err, entity.ErrAliasExists)
}

func TestShortenerService_Shorten_WithAlias_InvalidFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	alias := "short"
	req := ShortenRequest{URL: "https://example.com", Alias: &alias}

	_, err := service.Shorten(context.Background(), req)
	assert.ErrorContains(t, err, "alias must be exactly 10 characters")
}

func TestShortenerService_Shorten_GenerateCodeConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockRepo.EXPECT().GetURLByOriginalURL(gomock.Any(), "https://example.com").Return(nil, entity.ErrNotFound)
	mockRepo.EXPECT().CreateURL(gomock.Any(), gomock.Any()).Return(entity.ErrAlreadyExists).Times(maxAttempts)

	_, err := service.Shorten(context.Background(), ShortenRequest{URL: "https://example.com"})
	assert.ErrorIs(t, err, entity.ErrGenerateCode)
}

func TestShortenerService_Shorten_GenerateCodeOtherError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockRepo.EXPECT().GetURLByOriginalURL(gomock.Any(), "https://example.com").Return(nil, entity.ErrNotFound)
	mockRepo.EXPECT().CreateURL(gomock.Any(), gomock.Any()).Return(errors.New("db error"))

	_, err := service.Shorten(context.Background(), ShortenRequest{URL: "https://example.com"})
	assert.ErrorContains(t, err, "failed to create URL")
}

func TestShortenerService_GetOriginalURL_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockBloom.EXPECT().Test([]byte("abc123")).Return(true)
	mockCache.EXPECT().Get(gomock.Any(), "url:abc123").Return("https://example.com", nil)

	url, err := service.GetOriginalURL(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", url)
}

func TestShortenerService_GetOriginalURL_CacheMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	urlRecord := &entity.URL{ShortCode: "abc123", OriginalURL: "https://example.com"}

	mockBloom.EXPECT().Test([]byte("abc123")).Return(true)
	mockCache.EXPECT().Get(gomock.Any(), "url:abc123").Return("", entity.ErrNotFound)
	mockRepo.EXPECT().GetURLByShortCode(gomock.Any(), "abc123").Return(urlRecord, nil)
	mockCache.EXPECT().Set(gomock.Any(), "url:abc123", "https://example.com", gomock.Any()).Return(nil)

	url, err := service.GetOriginalURL(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", url)
}

func TestShortenerService_GetOriginalURL_NotFoundInBloom(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockBloom.EXPECT().Test([]byte("nonexistent")).Return(false)

	_, err := service.GetOriginalURL(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, entity.ErrURLNotFound)
}

func TestShortenerService_GetOriginalURL_NotFoundInRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockBloom.EXPECT().Test([]byte("abc123")).Return(true)
	mockCache.EXPECT().Get(gomock.Any(), "url:abc123").Return("", entity.ErrNotFound)
	mockRepo.EXPECT().GetURLByShortCode(gomock.Any(), "abc123").Return(nil, entity.ErrNotFound)

	_, err := service.GetOriginalURL(context.Background(), "abc123")
	assert.ErrorIs(t, err, entity.ErrURLNotFound)
}

func TestShortenerService_GetOriginalURL_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	expiredAt := time.Now().UTC().Add(-time.Hour)
	urlRecord := &entity.URL{ShortCode: "abc123", OriginalURL: "https://example.com", ExpiresAt: &expiredAt}

	mockBloom.EXPECT().Test([]byte("abc123")).Return(true)
	mockCache.EXPECT().Get(gomock.Any(), "url:abc123").Return("", entity.ErrNotFound)
	mockRepo.EXPECT().GetURLByShortCode(gomock.Any(), "abc123").Return(urlRecord, nil)

	_, err := service.GetOriginalURL(context.Background(), "abc123")
	assert.ErrorIs(t, err, entity.ErrURLExpired)
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://example.com", false},
		{"valid https", "https://example.com", false},
		{"no scheme", "example.com", true},
		{"wrong scheme", "ftp://example.com", true},
		{"no host", "http://", true},
		{"empty", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCalcTTL(t *testing.T) {
	s := &ShortenerService{}
	ttl := s.calcTTL(nil)
	assert.Equal(t, defaultCacheTTL, ttl)

	future := time.Now().Add(time.Hour)
	ttl = s.calcTTL(&future)
	assert.InDelta(t, time.Hour.Seconds(), ttl.Seconds(), 1.0)

	past := time.Now().Add(-time.Hour)
	ttl = s.calcTTL(&past)
	assert.Equal(t, defaultCacheTTL, ttl)
}

func TestEncodeBase62Full(t *testing.T) {
	assert.Equal(t, "0", encodeBase62Full(0))
	assert.Equal(t, "1", encodeBase62Full(1))
	assert.Equal(t, "A", encodeBase62Full(10))
	assert.Equal(t, "z", encodeBase62Full(61))
}

func TestPadLeft(t *testing.T) {
	assert.Equal(t, "000123", padLeft("123", 6))
	assert.Equal(t, "12345", padLeft("12345", 5))
	assert.Equal(t, "123456", padLeft("123456", 5))
}
