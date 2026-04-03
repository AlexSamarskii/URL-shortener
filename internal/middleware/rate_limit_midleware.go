package middleware

import (
	"net/http"

	"github.com/AlexSamarskii/URL-shortener/internal/pkg/metrics"
	limiter "github.com/AlexSamarskii/URL-shortener/internal/utils/rate_limiter"

	"github.com/gin-gonic/gin"
)

func RateLimitMiddleware(limiter limiter.RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		identifier := c.ClientIP()
		allowed, err := limiter.Allow(c.Request.Context(), identifier)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "rate limit check failed"})
			c.Abort()
			return
		}
		if !allowed {
			metrics.RateLimitBlocked.WithLabelValues(identifier).Inc()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	}
}
