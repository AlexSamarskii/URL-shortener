package ratelimiter

import (
	"context"
	"os"
	"time"

	limiter "github.com/AlexSamarskii/URL-shortener/internal/utils/rate_limiter"

	"github.com/redis/go-redis/v9"
)

type tokenBucket struct {
	client   *redis.Client
	script   *redis.Script
	rate     float64
	capacity int
	cost     int
}

func NewRateLimiter(client *redis.Client, rate float64, capacity int, cost int, scriptPath string) (limiter.RateLimiter, error) {
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		return nil, err
	}
	script := redis.NewScript(string(data))
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
