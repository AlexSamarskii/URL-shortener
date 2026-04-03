package middleware

import (
	"time"

	"github.com/AlexSamarskii/URL-shortener/internal/pkg/metrics"
	"github.com/gin-gonic/gin"
)

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		method := c.Request.Method

		if path == "/metrics" {
			c.Next()
			return
		}

		c.Next()

		duration := time.Since(start).Seconds()
		status := c.Writer.Status()

		metrics.HTTPRequestDuration.WithLabelValues(method, path).Observe(duration)
		metrics.HTTPRequestsTotal.WithLabelValues(method, path, statusCodeClass(status)).Inc()
	}
}

func statusCodeClass(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "unknown"
	}
}
