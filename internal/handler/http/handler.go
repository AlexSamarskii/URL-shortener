package handler

import (
	"net/http"
	"strconv"
	"time"

	"url_shortener/internal/entity"
	"url_shortener/internal/entity/dto"
	"url_shortener/internal/pkg/metrics"
	"url_shortener/internal/usecase"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service usecase.Shortener
}

func NewHandler(service usecase.Shortener) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Shorten(c *gin.Context) {
	var req dto.ShortenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		metrics.HTTPRequestsTotal.WithLabelValues("POST", "/shorten", "400").Inc()
		return
	}

	resp, err := h.service.Shorten(c.Request.Context(), usecase.ShortenRequest{
		URL:       req.URL,
		ExpiresIn: req.ExpiresIn,
		Alias:     req.Alias,
	})
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case err == entity.ErrURLInvalid:
			status = http.StatusBadRequest
		case err == entity.ErrAliasExists:
			status = http.StatusConflict
		case err == entity.ErrGenerateCode:
			status = http.StatusInternalServerError
		default:
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{"error": err.Error()})
		metrics.HTTPRequestsTotal.WithLabelValues("POST", "/shorten", strconv.Itoa(status)).Inc()
		return
	}

	c.JSON(http.StatusOK, dto.ShortenResponse{
		ShortCode: resp.ShortCode,
		ShortURL:  resp.ShortURL,
		ExpiresAt: resp.ExpiresAt,
	})
	metrics.HTTPRequestsTotal.WithLabelValues("POST", "/shorten", "200").Inc()
}

func (h *Handler) Redirect(c *gin.Context) {
	shortCode := c.Param("code")
	if shortCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "short code required"})
		metrics.HTTPRequestsTotal.WithLabelValues("GET", "/:code", "400").Inc()
		return
	}

	start := time.Now()
	originalURL, err := h.service.GetOriginalURL(c.Request.Context(), shortCode)
	latency := time.Since(start).Seconds()

	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case err == entity.ErrURLNotFound:
			status = http.StatusNotFound
		case err == entity.ErrURLExpired:
			status = http.StatusGone
		default:
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{"error": err.Error()})
		metrics.HTTPRequestsTotal.WithLabelValues("GET", "/:code", strconv.Itoa(status)).Inc()
		metrics.RedirectLatency.WithLabelValues("false").Observe(latency)
		return
	}

	c.Redirect(http.StatusMovedPermanently, originalURL)
	metrics.HTTPRequestsTotal.WithLabelValues("GET", "/:code", "301").Inc()
	metrics.RedirectLatency.WithLabelValues("true").Observe(latency)
}
