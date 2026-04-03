package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"

	"github.com/AlexSamarskii/URL-shortener/internal/pkg/metrics"
)

func TestMetricsMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	resetMetrics := func() {
		metrics.HTTPRequestsTotal.Reset()
		metrics.HTTPRequestDuration.Reset()
	}
	resetMetrics()
	t.Cleanup(resetMetrics)

	tests := []struct {
		name           string
		method         string
		path           string
		handlerStatus  int
		expectedStatus string
		expectedPath   string
		skipMetric     bool
	}{
		{
			name:           "GET /:code success 200",
			method:         "GET",
			path:           "/abc123",
			handlerStatus:  http.StatusOK,
			expectedStatus: "2xx",
			expectedPath:   "/:code",
		},
		{
			name:           "POST /shorten 201",
			method:         "POST",
			path:           "/shorten",
			handlerStatus:  http.StatusCreated,
			expectedStatus: "2xx",
			expectedPath:   "/shorten",
		},
		{
			name:           "GET /:code not found 404",
			method:         "GET",
			path:           "/missing",
			handlerStatus:  http.StatusNotFound,
			expectedStatus: "4xx",
			expectedPath:   "/:code",
		},
		{
			name:           "GET /:code internal error 500",
			method:         "GET",
			path:           "/error",
			handlerStatus:  http.StatusInternalServerError,
			expectedStatus: "5xx",
			expectedPath:   "/:code",
		},
		{
			name:           "GET /metrics should be skipped",
			method:         "GET",
			path:           "/metrics",
			handlerStatus:  http.StatusOK,
			expectedStatus: "",
			expectedPath:   "",
			skipMetric:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetMetrics()

			router := gin.New()
			router.Use(MetricsMiddleware())
			if tt.expectedPath != "" {
				router.Handle(tt.method, tt.expectedPath, func(c *gin.Context) {
					c.Status(tt.handlerStatus)
				})
			} else {
				router.Handle(tt.method, tt.path, func(c *gin.Context) {
					c.Status(tt.handlerStatus)
				})
			}

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.handlerStatus, w.Code)

			if tt.skipMetric {
				countRequests := testutil.CollectAndCount(metrics.HTTPRequestsTotal)
				countDuration := testutil.CollectAndCount(metrics.HTTPRequestDuration)
				assert.Equal(t, 0, countRequests)
				assert.Equal(t, 0, countDuration)
				return
			}

			expectedRequestValue := `
				# HELP http_requests_total Total number of HTTP requests
				# TYPE http_requests_total counter
				http_requests_total{endpoint="` + tt.expectedPath + `",method="` + tt.method + `",status="` + tt.expectedStatus + `"} 1
			`
			err := testutil.CollectAndCompare(metrics.HTTPRequestsTotal,
				strings.NewReader(expectedRequestValue), "http_requests_total")
			assert.NoError(t, err)

			countDuration := testutil.CollectAndCount(metrics.HTTPRequestDuration)
			assert.GreaterOrEqual(t, countDuration, 1)
		})
	}
}

func TestStatusStatusCodeClass(t *testing.T) {
	tests := []struct {
		status   int
		expected string
	}{
		{200, "2xx"},
		{201, "2xx"},
		{299, "2xx"},
		{300, "3xx"},
		{301, "3xx"},
		{399, "3xx"},
		{400, "4xx"},
		{404, "4xx"},
		{499, "4xx"},
		{500, "5xx"},
		{503, "5xx"},
		{599, "5xx"},
		{600, "unknown"},
		{100, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, statusCodeClass(tt.status))
		})
	}
}
