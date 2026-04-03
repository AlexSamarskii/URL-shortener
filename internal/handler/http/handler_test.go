package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/AlexSamarskii/URL-shortener/internal/entity"
	"github.com/AlexSamarskii/URL-shortener/internal/entity/dto"
	"github.com/AlexSamarskii/URL-shortener/internal/usecase"
	"github.com/AlexSamarskii/URL-shortener/internal/usecase/mocks"
)

func TestHandler_Shorten_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	reqBody := dto.ShortenRequest{
		URL: "https://example.com",
	}
	jsonBody, _ := json.Marshal(reqBody)

	mockService.EXPECT().
		Shorten(gomock.Any(), gomock.Any()).
		Return(&usecase.ShortenResponse{
			ShortCode: "abc123",
			ShortURL:  "http://short.com/abc123",
			ExpiresAt: nil,
		}, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/shorten", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Shorten(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp dto.ShortenResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "abc123", resp.ShortCode)
}

func TestHandler_Shorten_BadRequest(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/shorten", bytes.NewReader([]byte("{invalid")))

	handler.Shorten(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Shorten_ServiceError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	reqBody := dto.ShortenRequest{URL: "invalid"}
	jsonBody, _ := json.Marshal(reqBody)

	mockService.EXPECT().
		Shorten(gomock.Any(), gomock.Any()).
		Return(nil, entity.ErrURLInvalid)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/shorten", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Shorten(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandler_Shorten_AliasExists(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	alias := "myalias"
	reqBody := dto.ShortenRequest{
		URL:   "https://example.com",
		Alias: &alias,
	}
	jsonBody, _ := json.Marshal(reqBody)

	mockService.EXPECT().
		Shorten(gomock.Any(), gomock.Any()).
		Return(nil, entity.ErrAliasExists)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/shorten", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Shorten(c)

	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestHandler_Redirect_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	mockService.EXPECT().
		GetOriginalURL(gomock.Any(), "abc123").
		Return("https://example.com", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/abc123", nil)
	c.Params = gin.Params{{Key: "code", Value: "abc123"}}

	handler.Redirect(c)

	assert.Equal(t, http.StatusMovedPermanently, w.Code)
	assert.Equal(t, "https://example.com", w.Header().Get("Location"))
}

func TestHandler_Redirect_NotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	mockService.EXPECT().
		GetOriginalURL(gomock.Any(), "missing").
		Return("", entity.ErrURLNotFound)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/missing", nil)
	c.Params = gin.Params{{Key: "code", Value: "missing"}}

	handler.Redirect(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandler_Redirect_Expired(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	mockService.EXPECT().
		GetOriginalURL(gomock.Any(), "expired").
		Return("", entity.ErrURLExpired)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/expired", nil)
	c.Params = gin.Params{{Key: "code", Value: "expired"}}

	handler.Redirect(c)

	assert.Equal(t, http.StatusGone, w.Code)
}

func TestHandler_Shorten_GenerateCodeError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	reqBody := dto.ShortenRequest{
		URL: "https://example.com",
	}
	jsonBody, _ := json.Marshal(reqBody)

	mockService.EXPECT().
		Shorten(gomock.Any(), gomock.Any()).
		Return(nil, entity.ErrGenerateCode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/shorten", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Shorten(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_Shorten_UnknownError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	reqBody := dto.ShortenRequest{
		URL: "https://example.com",
	}
	jsonBody, _ := json.Marshal(reqBody)

	unknownErr := errors.New("something went wrong")
	mockService.EXPECT().
		Shorten(gomock.Any(), gomock.Any()).
		Return(nil, unknownErr)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/shorten", bytes.NewReader(jsonBody))
	c.Request.Header.Set("Content-Type", "application/json")

	handler.Shorten(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), unknownErr.Error())
}

func TestHandler_Redirect_EmptyCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Params = gin.Params{}

	handler.Redirect(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "short code required")
}

func TestHandler_Redirect_UnknownError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockService := mocks.NewMockShortener(ctrl)
	handler := NewHandler(mockService)

	unknownErr := errors.New("database connection failed")
	mockService.EXPECT().
		GetOriginalURL(gomock.Any(), "abc123").
		Return("", unknownErr)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/abc123", nil)
	c.Params = gin.Params{{Key: "code", Value: "abc123"}}

	handler.Redirect(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), unknownErr.Error())
}
