package ratelimiter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	client "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

const testScript = `
-- Token bucket rate limiter
-- KEYS[1] - key
-- ARGV[1] - current timestamp (seconds)
-- ARGV[2] - rate (tokens per second)
-- ARGV[3] - capacity (max tokens)
-- ARGV[4] - cost (tokens to consume)

local key = KEYS[1]
local now = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local capacity = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])

local lastRefill = redis.call('HGET', key, 'last_refill')
local tokens = redis.call('HGET', key, 'tokens')

if lastRefill == false or tokens == false then
    tokens = capacity
    lastRefill = now
else
    tokens = tonumber(tokens)
    lastRefill = tonumber(lastRefill)
    local elapsed = now - lastRefill
    local refill = elapsed * rate
    tokens = math.min(capacity, tokens + refill)
    lastRefill = now
end

if tokens >= cost then
    tokens = tokens - cost
    redis.call('HMSET', key, 'last_refill', lastRefill, 'tokens', tokens)
    redis.call('EXPIRE', key, math.ceil(capacity/rate) + 1) -- enough TTL
    return 1
else
    return 0
end
`

func setupRedisContainer(t *testing.T) (*client.Client, func()) {
	ctx := context.Background()
	redisContainer, err := redis.Run(ctx,
		"redis:7-alpine",
		testcontainers.WithWaitStrategy(wait.ForLog("Ready to accept connections").WithOccurrence(1)),
	)
	require.NoError(t, err)

	endpoint, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)
	addr := endpoint[len("redis://"):]

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

func createTempScriptFile(t *testing.T, content string) string {
	dir := t.TempDir()
	path := filepath.Join(dir, "rate_limit.lua")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
	return path
}

func TestNewRateLimiter_Success(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()

	scriptPath := createTempScriptFile(t, testScript)
	limiter, err := NewRateLimiter(client, 1.0, 10, 1, scriptPath)
	require.NoError(t, err)
	assert.NotNil(t, limiter)
	tb := limiter.(*tokenBucket)
	assert.Equal(t, 1.0, tb.rate)
	assert.Equal(t, 10, tb.capacity)
	assert.Equal(t, 1, tb.cost)
}

func TestNewRateLimiter_InvalidScriptPath(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	_, err := NewRateLimiter(client, 1.0, 10, 1, "/nonexistent/path.lua")
	assert.Error(t, err)
}

func TestAllow_Success(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	scriptPath := createTempScriptFile(t, testScript)
	limiter, err := NewRateLimiter(client, 10.0, 5, 1, scriptPath)
	require.NoError(t, err)

	ctx := context.Background()
	identifier := "test-client"

	for i := 0; i < 5; i++ {
		allowed, err := limiter.Allow(ctx, identifier)
		require.NoError(t, err)
		assert.True(t, allowed)
	}
	allowed, err := limiter.Allow(ctx, identifier)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestAllow_Refill(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	scriptPath := createTempScriptFile(t, testScript)
	limiter, err := NewRateLimiter(client, 2.0, 5, 1, scriptPath)
	require.NoError(t, err)

	ctx := context.Background()
	id := "refill-test"

	for i := 0; i < 5; i++ {
		allowed, _ := limiter.Allow(ctx, id)
		assert.True(t, allowed)
	}
	allowed, _ := limiter.Allow(ctx, id)
	assert.False(t, allowed)

	time.Sleep(1 * time.Second)

	for i := 0; i < 2; i++ {
		allowed, err := limiter.Allow(ctx, id)
		require.NoError(t, err)
		assert.True(t, allowed)
	}
	allowed, _ = limiter.Allow(ctx, id)
	assert.False(t, allowed)
}

func TestAllow_ContextCancel(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	scriptPath := createTempScriptFile(t, testScript)
	limiter, err := NewRateLimiter(client, 1.0, 5, 1, scriptPath)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = limiter.Allow(ctx, "any")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestAllow_ClientClosed(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	scriptPath := createTempScriptFile(t, testScript)
	limiter, err := NewRateLimiter(client, 1.0, 5, 1, scriptPath)
	require.NoError(t, err)

	// Закрываем клиент
	client.Close()
	ctx := context.Background()
	_, err = limiter.Allow(ctx, "any")
	assert.Error(t, err)
}

func TestClose(t *testing.T) {
	client, cleanup := setupRedisContainer(t)
	defer cleanup()
	scriptPath := createTempScriptFile(t, testScript)
	limiter, err := NewRateLimiter(client, 1.0, 5, 1, scriptPath)
	require.NoError(t, err)

	limiter.Close()
	ctx := context.Background()
	_, err = limiter.Allow(ctx, "any")
	assert.Error(t, err)
}
