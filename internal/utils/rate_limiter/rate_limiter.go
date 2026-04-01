package ratelimiter

import "context"

type RateLimiter interface {
	Allow(ctx context.Context, identifier string) (bool, error)
}
