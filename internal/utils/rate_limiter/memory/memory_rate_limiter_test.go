package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiter(t *testing.T) {
	maxReqs := 100
	window := 60 * time.Second
	lim := NewRateLimiter(maxReqs, window)
	assert.NotNil(t, lim)
	tb := lim.(*tokenBucket)
	assert.Equal(t, float64(maxReqs)/window.Seconds(), tb.rate)
	assert.Equal(t, maxReqs, tb.capacity)
	assert.Equal(t, 2*window, tb.threshold)
	assert.NotNil(t, tb.buckets)
	assert.NotNil(t, tb.stopCh)
}

func TestTokenBucket_Allow_FirstRequest(t *testing.T) {
	lim := NewRateLimiter(10, time.Minute).(*tokenBucket)
	ctx := context.Background()
	allowed, err := lim.Allow(ctx, "client1")
	require.NoError(t, err)
	assert.True(t, allowed)

	lim.mu.Lock()
	defer lim.mu.Unlock()
	state, exists := lim.buckets["client1"]
	assert.True(t, exists)
	assert.Equal(t, float64(lim.capacity-1), state.tokens)
}

func TestTokenBucket_Allow_WithinCapacity(t *testing.T) {
	capacity := 5
	lim := NewRateLimiter(capacity, time.Minute).(*tokenBucket)
	ctx := context.Background()
	id := "test"

	for i := 0; i < capacity; i++ {
		allowed, err := lim.Allow(ctx, id)
		require.NoError(t, err)
		assert.True(t, allowed)
	}
	allowed, err := lim.Allow(ctx, id)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestTokenBucket_Allow_Refill(t *testing.T) {
	rate := 2.0
	capacity := 5
	window := time.Duration(float64(capacity)/rate) * time.Second
	lim := &tokenBucket{
		buckets:   make(map[string]*bucketState),
		rate:      rate,
		capacity:  capacity,
		threshold: 2 * window,
		stopCh:    make(chan struct{}),
	}
	go lim.cleanup()
	defer lim.Close()
	ctx := context.Background()
	id := "refill"

	for i := 0; i < capacity; i++ {
		allowed, _ := lim.Allow(ctx, id)
		assert.True(t, allowed)
	}
	allowed, _ := lim.Allow(ctx, id)
	assert.False(t, allowed)

	time.Sleep(1 * time.Second)

	for i := 0; i < 2; i++ {
		allowed, err := lim.Allow(ctx, id)
		require.NoError(t, err)
		assert.True(t, allowed)
	}

	allowed, _ = lim.Allow(ctx, id)
	assert.False(t, allowed)
}

func TestTokenBucket_refill_NoElapsed(t *testing.T) {
	lim := NewRateLimiter(10, time.Minute).(*tokenBucket)
	state := &bucketState{tokens: 5.0, lastRefill: time.Now()}
	now := state.lastRefill
	lim.refill(now, state)
	assert.Equal(t, 5.0, state.tokens)
	assert.Equal(t, now, state.lastRefill)
}

func TestTokenBucket_refill_Cap(t *testing.T) {
	lim := NewRateLimiter(10, time.Minute).(*tokenBucket)
	state := &bucketState{tokens: 8.0, lastRefill: time.Now().Add(-10 * time.Second)}
	now := time.Now()
	lim.refill(now, state)
	assert.InDelta(t, 9.667, state.tokens, 0.01)
	assert.Equal(t, now, state.lastRefill)
	state = &bucketState{tokens: 5.0, lastRefill: time.Now().Add(-100 * time.Second)}
	lim.refill(now, state)
	assert.Equal(t, float64(lim.capacity), state.tokens)
}

func TestTokenBucket_Allow_NoRefillWhenExists(t *testing.T) {
	lim := NewRateLimiter(10, time.Minute).(*tokenBucket)
	ctx := context.Background()
	id := "existing"

	allowed, _ := lim.Allow(ctx, id)
	assert.True(t, allowed)

	lim.mu.Lock()
	state := lim.buckets[id]
	state.lastRefill = state.lastRefill.Add(-5 * time.Second)
	oldTokens := state.tokens
	lim.mu.Unlock()

	allowed, _ = lim.Allow(ctx, id)
	assert.True(t, allowed)
	lim.mu.Lock()
	defer lim.mu.Unlock()
	assert.Greater(t, lim.buckets[id].tokens, oldTokens-1)
}

func TestTokenBucket_Close(t *testing.T) {
	lim := NewRateLimiter(10, time.Minute).(*tokenBucket)
	lim.Close()
	lim.Close()
	_, ok := <-lim.stopCh
	assert.False(t, ok)
}

func TestTokenBucket_cleanup_RemovesInactive(t *testing.T) {
	lim := NewRateLimiter(10, time.Minute).(*tokenBucket)
	ctx := context.Background()
	lim.Allow(ctx, "active")
	lim.Allow(ctx, "inactive")

	lim.mu.Lock()
	lim.buckets["inactive"].lastRefill = time.Now().Add(-lim.threshold - time.Second)
	lim.mu.Unlock()

	lim.cleanupOnce()

	lim.mu.Lock()
	defer lim.mu.Unlock()
	_, existsActive := lim.buckets["active"]
	assert.True(t, existsActive)
	_, existsInactive := lim.buckets["inactive"]
	assert.False(t, existsInactive)
}

func (l *tokenBucket) cleanupOnce() {
	l.mu.Lock()
	now := time.Now()
	for id, state := range l.buckets {
		if now.Sub(state.lastRefill) > l.threshold {
			delete(l.buckets, id)
		}
	}
	l.mu.Unlock()
}

func TestTokenBucket_cleanup_GoroutineStops(t *testing.T) {
	lim := NewRateLimiter(10, time.Minute).(*tokenBucket)
	time.Sleep(10 * time.Millisecond)
	lim.Close()
	time.Sleep(10 * time.Millisecond)
}

func TestTokenBucket_Allow_ContextCancel(t *testing.T) {
	lim := NewRateLimiter(10, time.Minute).(*tokenBucket)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	allowed, err := lim.Allow(ctx, "any")
	assert.NoError(t, err)
	assert.True(t, allowed)
}
