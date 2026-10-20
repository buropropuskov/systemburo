package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// This stub verifies middleware ordering; server-state resolution has separate
// database-backed SessionAccess lifecycle coverage.
type rateLimitSessionStub struct {
	services.SessionAccess
	denied bool
}

func (s *rateLimitSessionStub) ResolveAccess(_ context.Context, claims *services.Claims) (models.SessionIdentity, error) {
	if s.denied {
		return models.SessionIdentity{}, echo.ErrUnauthorized
	}
	return models.SessionIdentity{OwnerUserID: 7, EffectiveUserID: 7, FamilyID: claims.ID}, nil
}

func (s *rateLimitSessionStub) ResolveRefreshCookie(_ context.Context, raw string) (models.SessionIdentity, error) {
	if s.denied || raw != "synthetic-cookie" {
		return models.SessionIdentity{}, echo.ErrUnauthorized
	}
	return models.SessionIdentity{OwnerUserID: 7, EffectiveUserID: 7, FamilyID: "device-cookie"}, nil
}

func TestAccountRateLimit_AuthAndFileShareBudget(t *testing.T) {
	e := echo.New()
	secret := []byte("synthetic-limiter-test-signing-key")
	sessions := &rateLimitSessionStub{}
	limiter := AccountRateLimit(2, 60)
	ok := func(c echo.Context) error { return c.NoContent(http.StatusOK) }
	e.GET("/api/x", ok, JWTAuth(secret, sessions), limiter)
	e.GET("/api/uploads*", ok, FileAccess(secret, sessions, nil), limiter)
	sign := func(id string, expired bool) string {
		claims := &services.Claims{UserID: 7, RegisteredClaims: jwt.RegisteredClaims{ID: id}}
		if expired {
			claims.ExpiresAt = jwt.NewNumericDate(time.Unix(1, 0))
		}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
		require.NoError(t, err)
		return token
	}
	call := func(path, token, cookie string) int {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			r.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
		}
		if cookie != "" {
			r.AddCookie(&http.Cookie{Name: services.RefreshCookieName, Value: cookie})
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, r)
		return rec.Code
	}
	require.Equal(t, 401, call("/api/x", "invalid", ""))
	require.Equal(t, 401, call("/api/x", sign("expired", true), ""))
	sessions.denied = true
	require.Equal(t, 401, call("/api/x", sign("revoked", false), ""))
	sessions.denied = false
	require.Equal(t, 200, call("/api/x", sign("device-a", false), ""))
	require.Equal(t, 401, call("/api/uploads/system_tables/photo.png", "invalid", "synthetic-cookie"))
	require.Equal(t, 200, call("/api/uploads/system_tables/photo.png", "", "synthetic-cookie"))
	require.Equal(t, 429, call("/api/x", sign("refreshed-device-b", false), ""))
}
