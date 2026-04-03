package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"url_shortener/internal/entity"
	"url_shortener/internal/repository/mocks"
	bloommocks "url_shortener/internal/utils/bloom/mocks"
	cachemocks "url_shortener/internal/utils/cache/mocks"
)

func TestShortenerService_Shorten_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	req := ShortenRequest{URL: "https://example.com"}

	mockRepo.EXPECT().
		GetURLByOriginalURL(gomock.Any(), "https://example.com").
		Return(nil, entity.ErrNotFound)

	mockRepo.EXPECT().
		CheckShortCodeExists(gomock.Any(), gomock.Any()).
		Return(false, nil)

	mockRepo.EXPECT().
		CreateURL(gomock.Any(), gomock.Any()).
		Return(nil)

	mockBloom.EXPECT().
		Add(gomock.Any()).
		Times(1)

	mockCache.EXPECT().
		Set(gomock.Any(), gomock.Any(), "https://example.com", gomock.Any()).
		Return(nil)

	resp, err := service.Shorten(context.Background(), req)
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

	existing := &entity.URL{
		ShortCode:   "abc123",
		OriginalURL: "https://example.com",
		ExpiresAt:   nil,
	}

	mockRepo.EXPECT().
		GetURLByOriginalURL(gomock.Any(), "https://example.com").
		Return(existing, nil)

	resp, err := service.Shorten(context.Background(), ShortenRequest{URL: "https://example.com"})
	require.NoError(t, err)
	assert.Equal(t, "abc123", resp.ShortCode)
	assert.Equal(t, "http://short.com/abc123", resp.ShortURL)
}

func TestShortenerService_Shorten_WithAlias(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	alias := "myalias101"
	req := ShortenRequest{
		URL:   "https://example.com",
		Alias: &alias,
	}

	mockRepo.EXPECT().
		GetURLByOriginalURL(gomock.Any(), "https://example.com").
		Return(nil, entity.ErrNotFound)

	mockRepo.EXPECT().
		CheckShortCodeExists(gomock.Any(), alias).
		Return(false, nil)

	mockRepo.EXPECT().
		CreateURL(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, url *entity.URL) error {
			assert.Equal(t, alias, url.ShortCode)
			return nil
		})

	mockBloom.EXPECT().Add([]byte(alias)).Times(1)
	mockCache.EXPECT().Set(gomock.Any(), gomock.Any(), "https://example.com", gomock.Any()).Return(nil)

	resp, err := service.Shorten(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, alias, resp.ShortCode)
}

func TestShortenerService_Shorten_AliasExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	alias := "existing10"
	req := ShortenRequest{
		URL:   "https://example.com",
		Alias: &alias,
	}

	mockRepo.EXPECT().
		CheckShortCodeExists(gomock.Any(), alias).
		Return(true, nil)

	_, err := service.Shorten(context.Background(), req)
	assert.ErrorIs(t, err, entity.ErrAliasExists)
}

func TestShortenerService_Shorten_InvalidURL(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	testCases := []struct {
		name    string
		url     string
		wantErr error
	}{
		{"empty", "", entity.ErrURLInvalid},
		{"no scheme", "example.com", entity.ErrURLInvalid},
		{"wrong scheme", "ftp://example.com", entity.ErrURLInvalid},
		{"no host", "http://", entity.ErrURLInvalid},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.Shorten(context.Background(), ShortenRequest{URL: tc.url})
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestShortenerService_Shorten_GenerateCodeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockRepo.EXPECT().
		GetURLByOriginalURL(gomock.Any(), "https://example.com").
		Return(nil, entity.ErrNotFound)

	mockRepo.EXPECT().
		CheckShortCodeExists(gomock.Any(), gomock.Any()).
		Return(true, nil).Times(10) // maxAttempts = 10

	_, err := service.Shorten(context.Background(), ShortenRequest{URL: "https://example.com"})
	assert.ErrorIs(t, err, entity.ErrGenerateCode)
}

func TestShortenerService_Shorten_RepoCreateConflict(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockRepo.EXPECT().
		GetURLByOriginalURL(gomock.Any(), "https://example.com").
		Return(nil, entity.ErrNotFound)

	mockRepo.EXPECT().
		CheckShortCodeExists(gomock.Any(), gomock.Any()).
		Return(false, nil)

	mockRepo.EXPECT().
		CreateURL(gomock.Any(), gomock.Any()).
		Return(entity.ErrAlreadyExists)

	_, err := service.Shorten(context.Background(), ShortenRequest{URL: "https://example.com"})
	assert.Error(t, err)
}

func TestShortenerService_Shorten_WithExpiration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	expiresIn := 3600
	req := ShortenRequest{
		URL:       "https://example.com",
		ExpiresIn: &expiresIn,
	}

	mockRepo.EXPECT().
		GetURLByOriginalURL(gomock.Any(), "https://example.com").
		Return(nil, entity.ErrNotFound)

	mockRepo.EXPECT().
		CheckShortCodeExists(gomock.Any(), gomock.Any()).
		Return(false, nil)

	mockRepo.EXPECT().
		CreateURL(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, url *entity.URL) error {
			assert.NotNil(t, url.ExpiresAt)
			return nil
		})

	mockBloom.EXPECT().Add(gomock.Any())
	mockCache.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	resp, err := service.Shorten(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, resp.ExpiresAt)
}

func TestShortenerService_GetOriginalURL_CacheHit(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockBloom.EXPECT().
		Test([]byte("abc123")).
		Return(true)

	mockCache.EXPECT().
		Get(gomock.Any(), "url:abc123").
		Return("https://example.com", nil)

	url, err := service.GetOriginalURL(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", url)
}

func TestShortenerService_GetOriginalURL_FromRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	urlRecord := &entity.URL{
		ShortCode:   "abc123",
		OriginalURL: "https://example.com",
		ExpiresAt:   nil,
	}

	mockBloom.EXPECT().
		Test([]byte("abc123")).
		Return(true)

	mockCache.EXPECT().
		Get(gomock.Any(), "url:abc123").
		Return("", entity.ErrNotFound)

	mockRepo.EXPECT().
		GetURLByShortCode(gomock.Any(), "abc123").
		Return(urlRecord, nil)

	mockCache.EXPECT().
		Set(gomock.Any(), "url:abc123", "https://example.com", gomock.Any()).
		Return(nil)

	url, err := service.GetOriginalURL(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", url)
}

func TestShortenerService_GetOriginalURL_NotFoundBloomFalse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockBloom.EXPECT().
		Test([]byte("abc123")).
		Return(false)

	_, err := service.GetOriginalURL(context.Background(), "abc123")
	assert.ErrorIs(t, err, entity.ErrURLNotFound)
}

func TestShortenerService_GetOriginalURL_NotFoundRepo(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockBloom.EXPECT().
		Test([]byte("abc123")).
		Return(true)

	mockCache.EXPECT().
		Get(gomock.Any(), "url:abc123").
		Return("", entity.ErrNotFound)

	mockRepo.EXPECT().
		GetURLByShortCode(gomock.Any(), "abc123").
		Return(nil, entity.ErrNotFound)

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

	expiredAt := time.Now().UTC().Add(-1 * time.Hour)
	urlRecord := &entity.URL{
		ShortCode:   "abc123",
		OriginalURL: "https://example.com",
		ExpiresAt:   &expiredAt,
	}

	mockBloom.EXPECT().
		Test([]byte("abc123")).
		Return(true)

	mockCache.EXPECT().
		Get(gomock.Any(), "url:abc123").
		Return("", entity.ErrNotFound)

	mockRepo.EXPECT().
		GetURLByShortCode(gomock.Any(), "abc123").
		Return(urlRecord, nil)

	_, err := service.GetOriginalURL(context.Background(), "abc123")
	assert.ErrorIs(t, err, entity.ErrURLExpired)
}

func TestShortenerService_GetOriginalURL_RepoError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockBloom.EXPECT().
		Test([]byte("abc123")).
		Return(true)

	mockCache.EXPECT().
		Get(gomock.Any(), "url:abc123").
		Return("", entity.ErrNotFound)

	mockRepo.EXPECT().
		GetURLByShortCode(gomock.Any(), "abc123").
		Return(nil, errors.New("database error"))

	_, err := service.GetOriginalURL(context.Background(), "abc123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database error")
}

func TestShortenerService_GetOriginalURL_CacheErrorNonNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	urlRecord := &entity.URL{
		ShortCode:   "abc123",
		OriginalURL: "https://example.com",
		ExpiresAt:   nil,
	}

	mockBloom.EXPECT().
		Test([]byte("abc123")).
		Return(true)

	mockCache.EXPECT().
		Get(gomock.Any(), "url:abc123").
		Return("", errors.New("redis connection refused"))

	mockRepo.EXPECT().
		GetURLByShortCode(gomock.Any(), "abc123").
		Return(urlRecord, nil)

	mockCache.EXPECT().
		Set(gomock.Any(), "url:abc123", "https://example.com", gomock.Any()).
		Return(nil)

	url, err := service.GetOriginalURL(context.Background(), "abc123")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", url)
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
		{"empty", "", true},
		{"no host", "http://", true},
		{"with path", "https://example.com/path", false},
		{"with query", "https://example.com?q=1", false},
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

func TestShortenerService_Shorten_GetURLByOriginalURL_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	dbErr := errors.New("database connection lost")
	mockRepo.EXPECT().
		GetURLByOriginalURL(gomock.Any(), "https://example.com").
		Return(nil, dbErr)

	_, err := service.Shorten(context.Background(), ShortenRequest{URL: "https://example.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check existing URL")
}

func TestShortenerService_Shorten_CreateURL_AlreadyExists_WithoutAlias(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")

	mockRepo.EXPECT().
		GetURLByOriginalURL(gomock.Any(), "https://example.com").
		Return(nil, entity.ErrNotFound)

	mockRepo.EXPECT().
		CheckShortCodeExists(gomock.Any(), gomock.Any()).
		Return(false, nil)

	mockRepo.EXPECT().
		CreateURL(gomock.Any(), gomock.Any()).
		Return(entity.ErrAlreadyExists)

	_, err := service.Shorten(context.Background(), ShortenRequest{URL: "https://example.com"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "short code already exists")
}

func TestShortenerService_ValidateAlias_WrongLength(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")
	ctx := context.Background()

	err := service.validateAlias(ctx, "short")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alias must be exactly 10 characters")

	err = service.validateAlias(ctx, "verylongalias")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "alias must be exactly 10 characters")
}

func TestShortenerService_ValidateAlias_InvalidChar(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")
	ctx := context.Background()

	invalidAliases := []string{"abc!123456", "abc@123456", "abc#123456", "abc 123456"}
	for _, alias := range invalidAliases {
		err := service.validateAlias(ctx, alias)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "alias contains invalid character")
	}
}

func TestShortenerService_ValidateAlias_CheckError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")
	ctx := context.Background()
	alias := "myalias101"

	mockRepo.EXPECT().
		CheckShortCodeExists(ctx, alias).
		Return(false, errors.New("database error"))

	err := service.validateAlias(ctx, alias)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check alias")
}

func TestShortenerService_GenerateUniqueCode_CheckError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := mocks.NewMockRepository(ctrl)
	mockCache := cachemocks.NewMockCache(ctrl)
	mockBloom := bloommocks.NewMockBloomFilter(ctrl)

	service := NewShortenerService(mockRepo, mockCache, mockBloom, 10, "http://short.com")
	ctx := context.Background()

	mockRepo.EXPECT().
		CheckShortCodeExists(gomock.Any(), gomock.Any()).
		Return(false, errors.New("db error"))

	_, err := service.generateUniqueCode(ctx)
	assert.Error(t, err)
}

func TestValidateURL_EdgeCases(t *testing.T) {
	err := validateURL("://example.com")
	assert.Error(t, err)

	err = validateURL("")
	assert.Error(t, err)

	err = validateURL("http://")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing host")
}

func TestCalcTTL(t *testing.T) {
	service := &ShortenerService{}

	ttl := service.calcTTL(nil)
	assert.Equal(t, defaultCacheTTL, ttl)

	future := time.Now().UTC().Add(1 * time.Hour)
	ttl = service.calcTTL(&future)
	assert.InDelta(t, time.Hour.Seconds(), ttl.Seconds(), 1.0)

	past := time.Now().UTC().Add(-1 * time.Hour)
	ttl = service.calcTTL(&past)
	assert.Equal(t, defaultCacheTTL, ttl)
}
