package usecase

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/AlexSamarskii/URL-shortener/internal/entity"
	"github.com/AlexSamarskii/URL-shortener/internal/repository"
	"github.com/AlexSamarskii/URL-shortener/internal/utils/bloom"
	"github.com/AlexSamarskii/URL-shortener/internal/utils/cache"
)

const (
	alphabet    = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_"
	base        = len(alphabet)
	maxAttempts = 10
)

const defaultCacheTTL = 24 * time.Hour

type Shortener interface {
	Shorten(ctx context.Context, req ShortenRequest) (*ShortenResponse, error)
	GetOriginalURL(ctx context.Context, shortCode string) (string, error)
}

type ShortenerService struct {
	repo         repository.Repository
	cache        cache.Cache
	bloom        bloom.BloomFilter
	codeLength   int
	domainPrefix string
}

func NewShortenerService(
	repo repository.Repository,
	cache cache.Cache,
	bloom bloom.BloomFilter,
	codeLength int,
	domainPrefix string,
) *ShortenerService {
	return &ShortenerService{
		repo:         repo,
		cache:        cache,
		bloom:        bloom,
		codeLength:   codeLength,
		domainPrefix: strings.TrimSuffix(domainPrefix, "/"),
	}
}

type ShortenRequest struct {
	URL       string
	ExpiresIn *int
	Alias     *string
}

type ShortenResponse struct {
	ShortCode string
	ShortURL  string
	ExpiresAt *time.Time
}

func (s *ShortenerService) Shorten(ctx context.Context, req ShortenRequest) (*ShortenResponse, error) {
	if err := validateURL(req.URL); err != nil {
		return nil, fmt.Errorf("%w: %v", entity.ErrURLInvalid, err)
	}

	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		t := time.Now().UTC().Add(time.Duration(*req.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	if req.Alias != nil && *req.Alias != "" {
		return s.shortenWithAlias(ctx, req.URL, *req.Alias, expiresAt)
	}

	existing, err := s.repo.GetURLByOriginalURL(ctx, req.URL)
	if err == nil && existing != nil {
		return &ShortenResponse{
			ShortCode: existing.ShortCode,
			ShortURL:  fmt.Sprintf("%s/%s", s.domainPrefix, existing.ShortCode),
			ExpiresAt: existing.ExpiresAt,
		}, nil
	}
	if err != nil && !errors.Is(err, entity.ErrNotFound) {
		return nil, fmt.Errorf("failed to check existing URL: %w", err)
	}

	shortCode, err := s.generateUniqueCodeFromURL(ctx, req.URL, expiresAt)
	if err != nil {
		return nil, err
	}

	s.bloom.Add([]byte(shortCode))
	ttl := s.calcTTL(expiresAt)
	if err := s.cache.Set(ctx, cacheKey(shortCode), req.URL, ttl); err != nil {
		slog.Error("failed to set cache", "short_code", shortCode, "error", err)
	}

	return &ShortenResponse{
		ShortCode: shortCode,
		ShortURL:  fmt.Sprintf("%s/%s", s.domainPrefix, shortCode),
		ExpiresAt: expiresAt,
	}, nil
}

func (s *ShortenerService) shortenWithAlias(ctx context.Context, originalURL, alias string, expiresAt *time.Time) (*ShortenResponse, error) {
	if err := s.validateAliasFormat(alias); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	urlRecord := &entity.URL{
		ShortCode:   alias,
		OriginalURL: originalURL,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
	}

	err := s.repo.CreateURL(ctx, urlRecord)
	if err != nil {
		if errors.Is(err, entity.ErrAlreadyExists) {
			return nil, entity.ErrAliasExists
		}
		return nil, fmt.Errorf("failed to create alias: %w", err)
	}

	s.bloom.Add([]byte(alias))
	ttl := s.calcTTL(expiresAt)
	if err := s.cache.Set(ctx, cacheKey(alias), originalURL, ttl); err != nil {
		slog.Error("failed to set cache for alias", "short_code", alias, "error", err)
	}

	return &ShortenResponse{
		ShortCode: alias,
		ShortURL:  fmt.Sprintf("%s/%s", s.domainPrefix, alias),
		ExpiresAt: expiresAt,
	}, nil
}

func (s *ShortenerService) validateAliasFormat(alias string) error {
	if len(alias) != s.codeLength {
		return fmt.Errorf("alias must be exactly %d characters", s.codeLength)
	}
	for _, ch := range alias {
		if !strings.ContainsRune(alphabet, ch) {
			return fmt.Errorf("alias contains invalid character: %c", ch)
		}
	}
	return nil
}

func (s *ShortenerService) generateUniqueCodeFromURL(ctx context.Context, originalURL string, expiresAt *time.Time) (string, error) {
	now := time.Now().UTC()
	for salt := 0; salt < maxAttempts; salt++ {
		data := originalURL + ":" + strconv.Itoa(salt)
		hasher := fnv.New64a()
		hasher.Write([]byte(data))
		num := hasher.Sum64()
		code := padLeft(encodeBase62Full(num), s.codeLength)

		urlRecord := &entity.URL{
			ShortCode:   code,
			OriginalURL: originalURL,
			ExpiresAt:   expiresAt,
			CreatedAt:   now,
		}
		err := s.repo.CreateURL(ctx, urlRecord)
		if err == nil {
			return code, nil
		}
		if errors.Is(err, entity.ErrAlreadyExists) {
			continue
		}
		return "", fmt.Errorf("failed to create URL: %w", err)
	}
	return "", entity.ErrGenerateCode
}

func (s *ShortenerService) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	if !s.bloom.Test([]byte(shortCode)) {
		return "", entity.ErrURLNotFound
	}

	cachedURL, err := s.cache.Get(ctx, cacheKey(shortCode))
	if err == nil {
		return cachedURL, nil
	}

	urlRecord, err := s.repo.GetURLByShortCode(ctx, shortCode)
	if err != nil {
		if errors.Is(err, entity.ErrNotFound) {
			return "", entity.ErrURLNotFound
		}
		return "", err
	}

	if urlRecord.ExpiresAt != nil && urlRecord.ExpiresAt.Before(time.Now().UTC()) {
		return "", entity.ErrURLExpired
	}

	ttl := s.calcTTL(urlRecord.ExpiresAt)
	if err := s.cache.Set(ctx, cacheKey(shortCode), urlRecord.OriginalURL, ttl); err != nil {
		slog.Error("failed to set cache after repo", "short_code", shortCode, "error", err)
	}

	return urlRecord.OriginalURL, nil
}

func encodeBase62Full(num uint64) string {
	if num == 0 {
		return string(alphabet[0])
	}
	var digits []byte
	n := num
	for n > 0 {
		remainder := n % uint64(base)
		digits = append([]byte{alphabet[remainder]}, digits...)
		n = n / uint64(base)
	}
	return string(digits)
}

func padLeft(s string, length int) string {
	if len(s) > length {
		return s[len(s)-length:]
	}
	if len(s) == length {
		return s
	}
	return strings.Repeat(string(alphabet[0]), length-len(s)) + s
}

func cacheKey(shortCode string) string {
	return "url:" + shortCode
}

func (s *ShortenerService) calcTTL(expiresAt *time.Time) time.Duration {
	if expiresAt != nil {
		ttl := time.Until(*expiresAt)
		if ttl > 0 {
			return ttl
		}
	}
	return defaultCacheTTL
}

func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("scheme must be http or https")
	}
	if parsed.Host == "" {
		return errors.New("missing host")
	}
	return nil
}
