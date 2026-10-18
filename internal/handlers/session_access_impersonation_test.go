package handlers_test

import (
	"context"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

func TestSessionAccess_ImpersonationRechecksCurrentState(t *testing.T) {
	for _, change := range []string{"actor-family", "actor-right", "actor-ban", "actor-password", "target-active", "target-ban", "target-password", "target-rank"} {
		t.Run(change, func(t *testing.T) {
			w := newSecurityGateWorld(t)
			mode, rec := impersonate(t, w.e, w.admin, w.userID)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			claims, err := services.DecodeAccessToken(mode, []byte(testutil.TestJWTSecret))
			require.NoError(t, err)
			resolver := services.NewSessionAccessService(w.db, testutil.TestJWTRefreshSecret)
			identity, err := resolver.ResolveAccess(context.Background(), claims)
			require.NoError(t, err)
			switch change {
			case "actor-family":
				require.NoError(t, w.db.Model(&models.RefreshToken{}).Where("user_id = ? AND family_id = ?", w.adminID, identity.FamilyID).Update("is_revoked", true).Error)
			case "actor-right":
				require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.adminID).Updates(map[string]any{"is_admin": false, "is_super_admin": false}).Error)
			case "actor-ban":
				require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.adminID).Update("is_banned", true).Error)
			case "actor-password":
				require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.adminID).Update("must_change_password", true).Error)
			case "target-active":
				require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.userID).Update("is_active", false).Error)
			case "target-ban":
				require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.userID).Update("is_banned", true).Error)
			case "target-password":
				require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.userID).Update("must_change_password", true).Error)
			case "target-rank":
				require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.userID).Update("is_super_admin", true).Error)
			}
			_, err = resolver.RequireLive(context.Background(), identity)
			sessionAccessStatus(t, err, http.StatusUnauthorized)
		})
	}
}

func TestSessionAccess_LegacyModeLogoutCannotRevokeTarget(t *testing.T) {
	w := newSecurityGateWorld(t)
	actor, actorCookie := testutil.LoginUser(t, w.e, "testadmin", "adminpass_long_enough_for_32chars!")
	mode, rec := impersonate(t, w.e, actor, w.userID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	claims, err := services.DecodeAccessToken(mode, []byte(testutil.TestJWTSecret))
	require.NoError(t, err)
	claims.TokenVersion, claims.SessionID, claims.SessionUserID = 0, "", 0
	legacy, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testutil.TestJWTSecret))
	require.NoError(t, err)
	h := testutil.AuthHeader(legacy)
	h.Set("Cookie", services.RefreshCookieName+"="+w.refresh)
	rec = testutil.POST(t, w.e, "/logout", `{}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	resolver := services.NewSessionAccessService(w.db, testutil.TestJWTRefreshSecret)
	_, err = resolver.ResolveRefreshCookie(context.Background(), w.refresh)
	require.NoError(t, err, "target cookie belongs to a different owner and must survive")
	h.Set("Cookie", services.RefreshCookieName+"="+actorCookie)
	rec = testutil.POST(t, w.e, "/logout", `{}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, err = resolver.ResolveRefreshCookie(context.Background(), actorCookie)
	sessionAccessStatus(t, err, http.StatusUnauthorized)
	_, err = resolver.ResolveRefreshCookie(context.Background(), w.refresh)
	require.NoError(t, err)
}
