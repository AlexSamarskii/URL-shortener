package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AlexSamarskii/URL-shortener/internal/entity"
)

const testKey = "test-key"
const testValue = "test-value"

func TestNewCache(t *testing.T) {
	c := NewCache()
	assert.NotNil(t, c)
	mc, ok := c.(*memoryCache)
	assert.True(t, ok)
	assert.NotNil(t, mc.data)
	assert.NotNil(t, mc.stopCh)
	select {
	case <-mc.stopCh:
		t.Fatal("stopCh не должен быть закрыт сразу после создания")
	default:
	}
	mc.Close()
}

func TestSetGetDelete(t *testing.T) {
	c := NewCache()
	ctx := context.Background()

	err := c.Set(ctx, testKey, testValue, 0)
	require.NoError(t, err)

	val, err := c.Get(ctx, testKey)
	require.NoError(t, err)
	assert.Equal(t, testValue, val)

	err = c.Delete(ctx, testKey)
	require.NoError(t, err)

	_, err = c.Get(ctx, testKey)
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestGetExpired(t *testing.T) {
	c := NewCache()
	ctx := context.Background()

	ttl := 100 * time.Millisecond
	err := c.Set(ctx, testKey, testValue, ttl)
	require.NoError(t, err)

	val, err := c.Get(ctx, testKey)
	require.NoError(t, err)
	assert.Equal(t, testValue, val)

	time.Sleep(ttl + 50*time.Millisecond)

	_, err = c.Get(ctx, testKey)
	assert.ErrorIs(t, err, entity.ErrExpired)

	_, err = c.Get(ctx, testKey)
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestGetNotFound(t *testing.T) {
	c := NewCache()
	ctx := context.Background()

	_, err := c.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestSetWithTTL(t *testing.T) {
	c := NewCache().(*memoryCache)
	ctx := context.Background()

	ttl := 5 * time.Minute
	err := c.Set(ctx, testKey, testValue, ttl)
	require.NoError(t, err)

	c.mu.RLock()
	it, ok := c.data[testKey]
	c.mu.RUnlock()
	require.True(t, ok)
	assert.False(t, it.expiration.IsZero())
	delta := time.Until(it.expiration)
	assert.InDelta(t, ttl.Seconds(), delta.Seconds(), 1.0)
}

func TestSetWithZeroTTL(t *testing.T) {
	c := NewCache().(*memoryCache)
	ctx := context.Background()

	err := c.Set(ctx, testKey, testValue, 0)
	require.NoError(t, err)

	c.mu.RLock()
	it, ok := c.data[testKey]
	c.mu.RUnlock()
	require.True(t, ok)
	assert.True(t, it.expiration.IsZero())
}

func TestClose(t *testing.T) {
	c := NewCache().(*memoryCache)
	c.Close()
	c.Close()
	_, ok := <-c.stopCh
	assert.False(t, ok)
}

func TestIsExpired(t *testing.T) {
	c := NewCache().(*memoryCache)
	now := time.Now()
	itemZero := &item{expiration: time.Time{}}
	assert.False(t, c.isExpired(itemZero))
	itemFuture := &item{expiration: now.Add(1 * time.Hour)}
	assert.False(t, c.isExpired(itemFuture))
	itemPast := &item{expiration: now.Add(-1 * time.Hour)}
	assert.True(t, c.isExpired(itemPast))
}
