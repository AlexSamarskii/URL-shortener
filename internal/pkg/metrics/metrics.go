package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	RedirectLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "redirect_latency_seconds",
			Help:    "Latency of redirect requests",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"cache_hit"},
	)

	RateLimitBlocked = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "rate_limit_blocked_total",
			Help: "Total number of requests blocked by rate limiter",
		},
		[]string{"identifier"},
	)
)
