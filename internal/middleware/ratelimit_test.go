package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mw "systemburo/internal/middleware"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginRateLimit_AllowsUntilLimit(t *testing.T) {
	e := echo.New()
	limiter := mw.LoginRateLimit(3, time.Minute)
	e.POST("/login", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, limiter)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "попытка %d должна пройти", i+1)
	}
}

func TestLoginRateLimit_BlocksAfterLimit(t *testing.T) {
	e := echo.New()
	limiter := mw.LoginRateLimit(3, time.Minute)
	e.POST("/login", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, limiter)

	// Первые 3 - ок
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}

	// 4-я - 429 + Retry-After
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("Retry-After"), "должен быть Retry-After header")
	assert.Contains(t, rec.Body.String(), "Слишком много попыток")
}

func TestLoginRateLimit_PerIPIsolation(t *testing.T) {
	e := echo.New()
	limiter := mw.LoginRateLimit(2, time.Minute)
	e.POST("/login", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, limiter)

	// IP1 исчерпывает лимит.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}

	// IP1: 3-я попытка -> 429
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)

	// IP2 - свой счётчик, должен работать.
	req = httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "10.0.0.2:5678"
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "другой IP имеет независимый счётчик")
}
