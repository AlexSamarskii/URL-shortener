package ratelimiter

import (
	"context"
	"time"

	limiter "github.com/AlexSamarskii/URL-shortener/internal/utils/rate_limiter"

	_ "embed"

	"github.com/redis/go-redis/v9"
)

//go:embed rate_limit.lua
var rateLimitScript string

type tokenBucket struct {
	client   *redis.Client
	script   *redis.Script
	rate     float64
	capacity int
	cost     int
}

func NewRateLimiter(client *redis.Client, rate float64, capacity int, cost int) (limiter.RateLimiter, error) {
	script := redis.NewScript(rateLimitScript)
	return &tokenBucket{
		client:   client,
		script:   script,
		rate:     rate,
		capacity: capacity,
		cost:     cost,
	}, nil
}

func (tb *tokenBucket) Allow(ctx context.Context, identifier string) (bool, error) {
	key := "ratelimit:" + identifier
	now := float64(time.Now().Unix())
	res, err := tb.script.Run(ctx, tb.client, []string{key}, now, tb.rate, tb.capacity, tb.cost).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (tb *tokenBucket) Close() {
	tb.client.Close()
}
