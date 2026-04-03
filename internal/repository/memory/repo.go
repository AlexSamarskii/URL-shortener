package memory

import (
	"context"
	"sync"
	"time"

	"github.com/AlexSamarskii/URL-shortener/internal/entity"
	"github.com/AlexSamarskii/URL-shortener/internal/repository"
)

const cleanupPeriod = 5 * time.Minute

type repo struct {
	mu     sync.RWMutex
	urls   map[string]*entity.URL
	orig   map[string]string
	stopCh chan struct{}
	once   sync.Once
}

func NewRepository() repository.Repository {
	r := &repo{
		urls:   make(map[string]*entity.URL),
		orig:   make(map[string]string),
		stopCh: make(chan struct{}),
	}
	go r.cleanupExpired()
	return r
}

func isExpired(url *entity.URL) bool {
	return url.ExpiresAt != nil && url.ExpiresAt.Before(time.Now().UTC())
}

func (r *repo) CreateURL(ctx context.Context, url *entity.URL) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.urls[url.ShortCode]; exists {
		return entity.ErrAlreadyExists
	}
	r.urls[url.ShortCode] = url
	r.orig[url.OriginalURL] = url.ShortCode
	return nil
}

func (r *repo) GetURLByShortCode(ctx context.Context, shortCode string) (*entity.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	url, ok := r.urls[shortCode]
	if !ok {
		return nil, entity.ErrNotFound
	}
	if isExpired(url) {
		delete(r.urls, shortCode)
		delete(r.orig, url.OriginalURL)
		return nil, entity.ErrExpired
	}
	return url, nil
}

func (r *repo) GetURLByOriginalURL(ctx context.Context, originalURL string) (*entity.URL, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	shortCode, ok := r.orig[originalURL]
	if !ok {
		return nil, entity.ErrNotFound
	}
	url, ok := r.urls[shortCode]
	if !ok {
		delete(r.orig, originalURL)
		return nil, entity.ErrNotFound
	}
	if isExpired(url) {
		delete(r.urls, shortCode)
		delete(r.orig, originalURL)
		return nil, entity.ErrExpired
	}
	return url, nil
}

func (r *repo) CheckShortCodeExists(ctx context.Context, shortCode string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.urls[shortCode]
	if !ok {
		return false, nil
	}
	return true, nil
}

func (r *repo) cleanupExpired() {
	ticker := time.NewTicker(cleanupPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			for code, url := range r.urls {
				if isExpired(url) {
					delete(r.urls, code)
					delete(r.orig, url.OriginalURL)
				}
			}
			r.mu.Unlock()
		case <-r.stopCh:
			return
		}
	}
}

func (r *repo) Close() {
	r.once.Do(func() {
		close(r.stopCh)
	})
}
