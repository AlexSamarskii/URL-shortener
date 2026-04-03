package usecase

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
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
	shortCode, err := s.generateUniqueCodeFromURL(ctx, req.URL, expiresAt, s.codeLength)
	if err != nil {
		return nil, err
	}
	s.bloom.Add([]byte(shortCode))
	ttl := s.calcTTL(expiresAt)
	_ = s.cache.Set(ctx, cacheKey(shortCode), req.URL, ttl)
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

	_, err := s.repo.CreateURL(ctx, urlRecord)
	if err != nil {
		if errors.Is(err, entity.ErrAlreadyExists) {
			return nil, entity.ErrAliasExists
		}
		if errors.Is(err, entity.ErrOriginalURLExists) {
			existing, err := s.repo.GetURLByOriginalURL(ctx, originalURL)
			if err != nil {
				return nil, err
			}
			return &ShortenResponse{
				ShortCode: existing.ShortCode,
				ShortURL:  fmt.Sprintf("%s/%s", s.domainPrefix, existing.ShortCode),
				ExpiresAt: existing.ExpiresAt,
			}, nil
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

func (s *ShortenerService) generateUniqueCodeFromURL(ctx context.Context, originalURL string, expiresAt *time.Time, length int) (string, error) {
	now := time.Now().UTC()
	var maxCodeValue = new(big.Int).Exp(big.NewInt(int64(base)), big.NewInt(int64(length)), nil)
	for salt := 0; salt < maxAttempts; salt++ {
		data := originalURL + ":" + strconv.Itoa(salt)
		hash := sha256.Sum256([]byte(data))
		num := new(big.Int).SetBytes(hash[:])
		remainder := new(big.Int).Mod(num, maxCodeValue)
		code := encodeBase63Fixed(remainder, s.codeLength)

		urlRecord := &entity.URL{
			ShortCode:   code,
			OriginalURL: originalURL,
			ExpiresAt:   expiresAt,
			CreatedAt:   now,
		}
		existingCode, err := s.repo.CreateURL(ctx, urlRecord)
		if err == nil {
			return existingCode, nil
		}
		if errors.Is(err, entity.ErrAlreadyExists) {
			continue
		}
		return "", err
	}
	return "", entity.ErrGenerateCode
}

func encodeBase63Fixed(n *big.Int, length int) string {
	if n.Sign() == 0 {
		return strings.Repeat(string(alphabet[0]), length)
	}
	digits := make([]byte, 0, length)
	tmp := new(big.Int).Set(n)
	zero := big.NewInt(0)
	b := big.NewInt(int64(base))
	for tmp.Cmp(zero) > 0 {
		rem := new(big.Int).Mod(tmp, b)
		idx := int(rem.Int64())
		digits = append([]byte{alphabet[idx]}, digits...)
		tmp.Div(tmp, b)
	}
	if len(digits) < length {
		pad := strings.Repeat(string(alphabet[0]), length-len(digits))
		return pad + string(digits)
	}
	return string(digits)
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
