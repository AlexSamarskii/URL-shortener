package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type mockRateLimiter struct {
	allowResult bool
	allowError  error
}

func (m *mockRateLimiter) Allow(ctx context.Context, identifier string) (bool, error) {
	return m.allowResult, m.allowError
}

func (m *mockRateLimiter) Close() {}

func TestRateLimitMiddleware_Allow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockLimiter := &mockRateLimiter{
		allowResult: true,
		allowError:  nil,
	}

	router.Use(RateLimitMiddleware(mockLimiter))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "success", w.Body.String())
}

func TestRateLimitMiddleware_Blocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockLimiter := &mockRateLimiter{
		allowResult: false,
		allowError:  nil,
	}

	router.Use(RateLimitMiddleware(mockLimiter))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "rate limit exceeded")
}

func TestRateLimitMiddleware_Error(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockLimiter := &mockRateLimiter{
		allowResult: false,
		allowError:  errors.New("redis connection failed"),
	}

	router.Use(RateLimitMiddleware(mockLimiter))
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "success")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "rate limit check failed")
}
