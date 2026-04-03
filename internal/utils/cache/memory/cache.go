package memory

import (
	"context"
	"sync"
	"time"
	"github.com/AlexSamarskii/URL-shortener/internal/entity"
	"github.com/AlexSamarskii/URL-shortener/internal/utils/cache"
)

const cleanupPeriod = 1 * time.Minute
const empty = ""

type item struct {
	value      string
	expiration time.Time
}

type memoryCache struct {
	mu     sync.RWMutex
	data   map[string]item
	stopCh chan struct{}
	once   sync.Once
}

func NewCache() cache.Cache {
	c := &memoryCache{
		data:   make(map[string]item),
		stopCh: make(chan struct{}),
	}
	go c.cleanup()
	return c
}

func (c *memoryCache) Close() {
	c.once.Do(func() {
		close(c.stopCh)
	})
}

func (c *memoryCache) isExpired(it *item) bool {
	return !it.expiration.IsZero() && it.expiration.Before(time.Now())
}

func (c *memoryCache) Get(ctx context.Context, key string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	it, ok := c.data[key]
	if !ok {
		return empty, entity.ErrNotFound
	}
	if c.isExpired(&it) {
		delete(c.data, key)
		return empty, entity.ErrExpired
	}
	return it.value, nil
}

func (c *memoryCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiration time.Time
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	} else {
		expiration = time.Time{}
	}

	c.data[key] = item{
		value:      value,
		expiration: expiration,
	}
	return nil
}

func (c *memoryCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
	return nil
}

func (c *memoryCache) cleanup() {
	ticker := time.NewTicker(cleanupPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			for k, it := range c.data {
				if c.isExpired(&it) {
					delete(c.data, k)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}
