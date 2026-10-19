package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
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

// Different unverified token strings must not create separate IP budgets.
// Verified account isolation is covered by TestAccountRateLimit_SharedIdentity.
func TestRateLimit_UnverifiedTokensShareIPBudget(t *testing.T) {
	e := echo.New()
	e.Use(mw.RateLimit(2, 60))
	e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	const jwtHeaderPrefix = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	tokenA := jwtHeaderPrefix + ".payloadAAAAAAAAAAAA.signatureAAAA"
	tokenB := jwtHeaderPrefix + ".payloadBBBBBBBBBBBB.signatureBBBB"

	do := func(token string) int {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec.Code
	}

	// Exhaust this connection's budget.
	require.Equal(t, http.StatusOK, do(tokenA))
	require.Equal(t, http.StatusOK, do(tokenA))
	require.Equal(t, http.StatusTooManyRequests, do(tokenA))

	assert.Equal(t, http.StatusTooManyRequests, do(tokenB), "смена непроверенной строки не меняет IP-бюджет")
	assert.Equal(t, http.StatusTooManyRequests, do(""))
}

// TestRateLimit_SetsRetryAfterOn429 - глобальный лимитер отдаёт Retry-After,
// чтобы клиент знал, через сколько повторить.
func TestRateLimit_SetsRetryAfterOn429(t *testing.T) {
	e := echo.New()
	e.Use(mw.RateLimit(1, 60))
	e.GET("/x", func(c echo.Context) error { return c.String(http.StatusOK, "ok") })

	const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.payload.signature"
	do := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	require.Equal(t, http.StatusOK, do().Code)
	rec := do()
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	// Retry-After - реальный остаток окна (не полное окно): для только что занятого
	// слота это ~window, но зависит от границы секунды, поэтому проверяем диапазон.
	ra, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	assert.Greater(t, ra, 0)
	assert.LessOrEqual(t, ra, 60)
}
