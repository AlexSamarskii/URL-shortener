package redis

import (
	"context"
	"strings"
	"testing"
	"time"

	client "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"url_shortener/internal/entity"
)

func setupRedisContainer(t *testing.T) (*client.Client, func()) {
	ctx := context.Background()
	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(wait.ForLog("Ready to accept connections").WithOccurrence(1)),
	)
	require.NoError(t, err)

	endpoint, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)
	addr := strings.TrimPrefix(endpoint, "redis://")

	client := client.NewClient(&client.Options{
		Addr: addr,
	})
	err = client.Ping(ctx).Err()
	require.NoError(t, err)

	cleanup := func() {
		client.Close()
		redisContainer.Terminate(ctx)
	}
	return client, cleanup
}

func TestNewCache(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	cache := NewCache(client)
	assert.NotNil(t, cache)
}

func TestCache_SetGet(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	cache := NewCache(client)
	ctx := context.Background()
	key := "test-key"
	value := "test-value"
	ttl := 5 * time.Second

	err := cache.Set(ctx, key, value, ttl)
	require.NoError(t, err)

	val, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, val)
}

func TestCache_GetNotFound(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	cache := NewCache(client)
	ctx := context.Background()

	val, err := cache.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, entity.ErrNotFound)
	assert.Empty(t, val)
}

func TestCache_Delete(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	cache := NewCache(client)
	ctx := context.Background()
	key := "to-delete"

	err := cache.Set(ctx, key, "value", 0)
	require.NoError(t, err)

	err = cache.Delete(ctx, key)
	require.NoError(t, err)

	_, err = cache.Get(ctx, key)
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestCache_Expiration(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	cache := NewCache(client)
	ctx := context.Background()
	key := "expiring"
	value := "temp"
	ttl := 1 * time.Second

	err := cache.Set(ctx, key, value, ttl)
	require.NoError(t, err)

	val, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, value, val)

	time.Sleep(ttl + 100*time.Millisecond)

	_, err = cache.Get(ctx, key)
	assert.ErrorIs(t, err, entity.ErrNotFound)
}

func TestCache_SetZeroTTL(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	cache := NewCache(client)
	ctx := context.Background()
	key := "forever"

	err := cache.Set(ctx, key, "value", 0)
	require.NoError(t, err)

	time.Sleep(1 * time.Second)
	val, err := cache.Get(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, "value", val)
}

func TestCache_Set_ClientClosed(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	cache := NewCache(client)
	client.Close()
	ctx := context.Background()
	err := cache.Set(ctx, "key", "value", 0)
	assert.Error(t, err)
}

func TestCache_Get_ClientClosed(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	cache := NewCache(client)
	client.Close()
	ctx := context.Background()
	_, err := cache.Get(ctx, "key")
	assert.Error(t, err)
}

func TestCache_Delete_ClientClosed(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	cache := NewCache(client)
	client.Close()
	ctx := context.Background()
	err := cache.Delete(ctx, "key")
	assert.Error(t, err)
}
