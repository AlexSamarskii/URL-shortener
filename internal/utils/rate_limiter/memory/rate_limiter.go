package memory

import (
	"context"
	"sync"
	"time"

	limiter "url_shortener/internal/utils/rate_limiter"
)

const cleanupPeriod = 5 * time.Minute

type bucketState struct {
	tokens     float64
	lastRefill time.Time
}

type tokenBucket struct {
	mu        sync.Mutex
	buckets   map[string]*bucketState
	rate      float64
	capacity  int
	threshold time.Duration
	stopCh    chan struct{}
	once      sync.Once
}

// rate = maxReqs / windowSize
// capacity = maxReqs
func NewRateLimiter(maxReqs int, windowSize time.Duration) limiter.RateLimiter {
	rate := float64(maxReqs) / windowSize.Seconds()
	l := &tokenBucket{
		buckets:   make(map[string]*bucketState),
		rate:      rate,
		capacity:  maxReqs,
		threshold: 2 * windowSize,
		stopCh:    make(chan struct{}),
	}
	go l.cleanup()
	return l
}

func (l *tokenBucket) Close() {
	l.once.Do(func() {
		close(l.stopCh)
	})
}

func (l *tokenBucket) refill(now time.Time, state *bucketState) {
	elapsed := now.Sub(state.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}
	newTokens := elapsed * l.rate
	state.tokens += newTokens
	if state.tokens > float64(l.capacity) {
		state.tokens = float64(l.capacity)
	}
	state.lastRefill = now
}

func (l *tokenBucket) Allow(ctx context.Context, identifier string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	state, exists := l.buckets[identifier]
	if !exists {
		state = &bucketState{
			tokens:     float64(l.capacity),
			lastRefill: now,
		}
		l.buckets[identifier] = state
	} else {
		l.refill(now, state)
	}

	if state.tokens >= 1.0 {
		state.tokens -= 1.0
		return true, nil
	}
	return false, nil
}

func (l *tokenBucket) cleanup() {
	ticker := time.NewTicker(cleanupPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			now := time.Now()
			for id, state := range l.buckets {
				if now.Sub(state.lastRefill) > l.threshold {
					delete(l.buckets, id)
				}
			}
			l.mu.Unlock()
		case <-l.stopCh:
			return
		}
	}
}
