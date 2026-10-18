package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"net/http/httptest"
	"systemburo/internal/apperr"
	mw "systemburo/internal/middleware"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestSessionAccess_FileCookieRevocationAndBearerPriority(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	raw, _ := loginForRefreshTests(t, e, db, "photo_owner")
	files := echo.New()
	files.HTTPErrorHandler = e.HTTPErrorHandler
	files.GET("/api/uploads*", func(c echo.Context) error {
		return c.String(http.StatusOK, "synthetic photo")
	}, mw.FileAccess([]byte(testutil.TestJWTSecret), services.NewSessionAccessService(db, testutil.TestJWTRefreshSecret), nil))
	read := func(bearer string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/uploads/system_tables/photo.png", nil)
		req.AddCookie(&http.Cookie{Name: services.RefreshCookieName, Value: raw})
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		rec := httptest.NewRecorder()
		files.ServeHTTP(rec, req)
		return rec
	}
	require.Equal(t, http.StatusOK, read("").Code)
	require.Equal(t, http.StatusUnauthorized, read("invalid").Code, "invalid bearer must not fall back to an active cookie")
	require.NoError(t, newRefreshService(db).Logout(context.Background(), "photo_owner", models.LogoutRequest{RefreshToken: raw}, nil))
	denied := read("")
	require.Equal(t, http.StatusUnauthorized, denied.Code)
	require.NotContains(t, denied.Body.String(), "synthetic photo")
}

func sessionAccessStatus(t *testing.T, err error, code int) {
	t.Helper()
	var domain *apperr.Error
	require.ErrorAs(t, err, &domain)
	require.Equal(t, code, domain.Code)
}

func TestSessionAccess_RotationAndOwnerBoundRevocation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	rawA, _ := loginForRefreshTests(t, e, db, "access_owner")
	svc := newRefreshService(db)
	ctx := context.Background()
	pairA, err := svc.RefreshToken(ctx, models.RefreshTokenRequest{RefreshToken: rawA}, nil)
	require.NoError(t, err)
	claimsA, err := services.DecodeAccessToken(pairA.Token, []byte(testutil.TestJWTSecret))
	require.NoError(t, err)
	_, rawB := testutil.LoginUser(t, e, "access_owner", "correctpass")
	resolver := services.NewSessionAccessService(db, testutil.TestJWTRefreshSecret)
	identity, err := resolver.ResolveAccess(ctx, claimsA)
	require.NoError(t, err)
	require.Equal(t, identity.OwnerUserID, identity.EffectiveUserID)
	pairRotated, err := svc.RefreshToken(ctx, models.RefreshTokenRequest{RefreshToken: pairA.RefreshToken}, nil)
	require.NoError(t, err)
	_, err = resolver.RequireLive(ctx, identity)
	require.NoError(t, err, "rotation must preserve the access family")
	_, err = resolver.ResolveRefreshCookie(ctx, pairA.RefreshToken)
	sessionAccessStatus(t, err, http.StatusUnauthorized)
	require.NoError(t, svc.Logout(ctx, "access_owner", models.LogoutRequest{RefreshToken: pairRotated.RefreshToken}, nil))
	_, err = resolver.RequireLive(ctx, identity)
	sessionAccessStatus(t, err, http.StatusUnauthorized)
	_, err = resolver.ResolveRefreshCookie(ctx, rawB)
	require.NoError(t, err, "a different device must remain active")
}

func TestSessionAccess_LegacyUpgradeAndRepeatedLogout(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	raw, _ := loginForRefreshTests(t, e, db, "access_legacy")
	access, _ := testutil.LoginUser(t, e, "access_legacy", "correctpass")
	claims, err := services.DecodeAccessToken(access, []byte(testutil.TestJWTSecret))
	require.NoError(t, err)
	claims.TokenVersion, claims.SessionUserID, claims.SessionID = 0, 0, ""
	legacy, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testutil.TestJWTSecret))
	require.NoError(t, err)
	rec := testutil.GET(t, e, "/users/me", testutil.AuthHeader(legacy))
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	h := http.Header{"Cookie": []string{"refresh_token=" + raw}}
	rec = testutil.POST(t, e, "/refresh-token", `{}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	upgraded, ok := testutil.ParseMap(t, rec)["token"].(string)
	require.True(t, ok)
	v2, err := services.DecodeAccessToken(upgraded, []byte(testutil.TestJWTSecret))
	require.NoError(t, err)
	require.Equal(t, services.AccessTokenVersion, v2.TokenVersion)
	require.NotEmpty(t, v2.SessionID)
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "refresh_token" {
			raw = cookie.Value
		}
	}
	h = testutil.AuthHeader(legacy)
	h.Set("Cookie", "refresh_token="+raw)
	rec = testutil.POST(t, e, "/logout", `{}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = testutil.POST(t, e, "/logout", `{}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = testutil.GET(t, e, "/users/me", testutil.AuthHeader(upgraded))
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestSessionAccess_BannedOwnReadAfterNewLogin(t *testing.T) {
	w := newSecurityGateWorld(t)
	w.change(t, "ban")
	rec := testutil.GET(t, w.e, "/users/me", testutil.AuthHeader(w.user))
	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	current, _ := testutil.LoginUser(t, w.e, "security_gate_user", testPassword)
	rec = testutil.GET(t, w.e, "/users/me", testutil.AuthHeader(current))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = testutil.GET(t, w.e, "/users/all", testutil.AuthHeader(current))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestSessionAccess_FixedExpiryAndSourceError(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	raw, owner := loginForRefreshTests(t, e, db, "access_clock")
	var row models.RefreshToken
	require.NoError(t, db.Where("user_id = ?", owner).First(&row).Error)
	now := time.Date(2032, 12, 31, 23, 59, 59, 0, time.UTC)
	require.NoError(t, db.Model(&row).Update("expires_at", now.Add(time.Hour)).Error)
	claims := &services.Claims{TokenVersion: services.AccessTokenVersion, SessionID: row.FamilyID, SessionUserID: owner, UserID: owner,
		RegisteredClaims: jwt.RegisteredClaims{Subject: "access_clock", ExpiresAt: jwt.NewNumericDate(now.Add(time.Second))}}
	resolver := services.NewSessionAccessService(db, testutil.TestJWTRefreshSecret, services.WithSessionAccessClock(func() time.Time { return now }))
	_, err := resolver.ResolveAccess(context.Background(), claims)
	require.NoError(t, err)
	claims.ExpiresAt = jwt.NewNumericDate(now)
	_, err = resolver.ResolveAccess(context.Background(), claims)
	sessionAccessStatus(t, err, http.StatusUnauthorized)
	claims.ExpiresAt = jwt.NewNumericDate(now.Add(time.Hour))
	require.NoError(t, db.Model(&row).Update("expires_at", now).Error)
	_, err = resolver.ResolveAccess(context.Background(), claims)
	sessionAccessStatus(t, err, http.StatusUnauthorized)
	_, err = resolver.ResolveRefreshCookie(context.Background(), raw)
	sessionAccessStatus(t, err, http.StatusUnauthorized)
	wrapped, broken := breakUsersQuery(t, db)
	failing := services.NewSessionAccessService(wrapped, testutil.TestJWTRefreshSecret, services.WithSessionAccessClock(func() time.Time { return now }))
	*broken = true
	_, err = failing.ResolveAccess(context.Background(), claims)
	sessionAccessStatus(t, err, http.StatusServiceUnavailable)
	require.True(t, errors.Is(err, errUsersQueryDown))
}
