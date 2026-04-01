package usecase

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"url_shortener/internal/entity"
	"url_shortener/internal/repository"
	"url_shortener/internal/utils/bloom"
	"url_shortener/internal/utils/cache"

	"github.com/google/uuid"
)

const (
	alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz_"
	base     = len(alphabet)
)

var (
	ErrURLInvalid   = errors.New("invalid URL")
	ErrAliasExists  = errors.New("alias already exists")
	ErrURLNotFound  = errors.New("url not found")
	ErrURLExpired   = errors.New("url expired")
	ErrGenerateCode = errors.New("failed to generate unique code")
)

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
		return nil, fmt.Errorf("%w: %v", ErrURLInvalid, err)
	}

	if req.Alias != nil && *req.Alias != "" {
		if err := s.validateAlias(ctx, *req.Alias); err != nil {
			return nil, err
		}
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

	var shortCode string
	if req.Alias != nil && *req.Alias != "" {
		shortCode = *req.Alias
	} else {
		var err error
		shortCode, err = s.generateUniqueCode(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrGenerateCode, err)
		}
	}

	var expiresAt *time.Time
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		t := time.Now().UTC().Add(time.Duration(*req.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	now := time.Now().UTC()
	urlRecord := &entity.URL{
		ID:          generateID(),
		ShortCode:   shortCode,
		OriginalURL: req.URL,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateURL(ctx, urlRecord); err != nil {
		return nil, fmt.Errorf("failed to create URL: %w", err)
	}

	s.bloom.Add([]byte(shortCode))

	ttl := s.calcTTL(expiresAt)
	if err := s.cache.Set(ctx, cacheKey(shortCode), req.URL, ttl); err != nil {
		_ = err
	}

	return &ShortenResponse{
		ShortCode: shortCode,
		ShortURL:  fmt.Sprintf("%s/%s", s.domainPrefix, shortCode),
		ExpiresAt: expiresAt,
	}, nil
}

func (s *ShortenerService) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	if !s.bloom.Test([]byte(shortCode)) {
		return "", ErrURLNotFound
	}

	cachedURL, err := s.cache.Get(ctx, cacheKey(shortCode))
	if err == nil {
		return cachedURL, nil
	}
	if !errors.Is(err, entity.ErrNotFound) {
		_ = err
	}

	urlRecord, err := s.repo.GetURLByShortCode(ctx, shortCode)
	if err != nil {
		if errors.Is(err, entity.ErrNotFound) {
			return "", ErrURLNotFound
		}
		return "", err
	}

	if urlRecord.ExpiresAt != nil && urlRecord.ExpiresAt.Before(time.Now().UTC()) {
		return "", ErrURLExpired
	}

	ttl := s.calcTTL(urlRecord.ExpiresAt)
	if err := s.cache.Set(ctx, cacheKey(shortCode), urlRecord.OriginalURL, ttl); err != nil {
		_ = err
	}

	return urlRecord.OriginalURL, nil
}

func (s *ShortenerService) validateAlias(ctx context.Context, alias string) error {
	if len(alias) != s.codeLength {
		return fmt.Errorf("alias must be exactly %d characters", s.codeLength)
	}

	for _, ch := range alias {
		if !strings.ContainsRune(alphabet, ch) {
			return fmt.Errorf("alias contains invalid character: %c", ch)
		}
	}
	exists, err := s.repo.CheckShortCodeExists(ctx, alias)
	if err != nil {
		return fmt.Errorf("failed to check alias: %w", err)
	}
	if exists {
		return ErrAliasExists
	}
	return nil
}

func (s *ShortenerService) generateUniqueCode(ctx context.Context) (string, error) {
	const maxAttempts = 10
	for i := 0; i < maxAttempts; i++ {
		code := generateRandomCode(s.codeLength)
		exists, err := s.repo.CheckShortCodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", ErrGenerateCode
}

func generateRandomCode(length int) string {
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(base)))
		b[i] = alphabet[n.Int64()]
	}
	return string(b)
}

func generateID() string {
	return uuid.New().String()
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
	return 365 * 24 * time.Hour
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
	// Можно добавить блокировку локальных адресов
	// if strings.HasPrefix(parsed.Host, "localhost")...
	return nil
}
