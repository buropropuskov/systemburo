package services

import (
	"context"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"net/http"
	"strings"
	"systemburo/internal/models"
	"testing"
	"time"
)

func TestApplicationOperationKeyWindow(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	keyAt := func(at time.Time) string { return fmt.Sprintf("v1.%d.%s", at.UnixMilli(), strings.Repeat("a", 32)) }
	for _, tc := range []struct {
		name string
		at   time.Time
		code int
	}{
		{"now", now, 0}, {"exact expiry", now.Add(-24 * time.Hour), 0},
		{"expired", now.Add(-24*time.Hour - time.Millisecond), http.StatusConflict},
		{"future boundary", now.Add(5 * time.Minute), 0}, {"future", now.Add(5*time.Minute + time.Millisecond), http.StatusConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateApplicationOperationKey(keyAt(tc.at), now)
			if tc.code == 0 {
				require.NoError(t, err)
				return
			}
			var httpErr *echo.HTTPError
			require.ErrorAs(t, err, &httpErr)
			require.Equal(t, tc.code, httpErr.Code)
		})
	}
	for _, key := range []string{"", "request-id", keyAt(now) + "x", "v1.123." + strings.Repeat("a", 32)} {
		var httpErr *echo.HTTPError
		require.ErrorAs(t, validateApplicationOperationKey(key, now), &httpErr)
		require.Equal(t, http.StatusBadRequest, httpErr.Code)
	}
}

func TestApplicationOperationIdentityAndFingerprint(t *testing.T) {
	now := time.Now().UTC()
	key := fmt.Sprintf("v1.%d.%s", now.UnixMilli(), strings.Repeat("a", 32))
	actor := &models.User{ID: 7, IsActive: true}
	identity := models.SessionIdentity{OwnerUserID: 3, EffectiveUserID: 7}
	ctx := WithSessionIdentity(context.Background(), identity)
	one, err := newApplicationCommand(ctx, actor, "application.submit", key, map[string]int{"a": 1, "b": 2})
	require.NoError(t, err)
	require.Equal(t, 3, one.row.OwnerUserID)
	require.Equal(t, 7, one.row.EffectiveUserID)
	same, err := newApplicationCommand(ctx, actor, "application.submit", key, map[string]int{"b": 2, "a": 1})
	require.NoError(t, err)
	require.Equal(t, one.row.Fingerprint, same.row.Fingerprint)
	changed, err := newApplicationCommand(ctx, actor, "application.submit", key, map[string]int{"a": 2, "b": 2})
	require.NoError(t, err)
	require.NotEqual(t, one.row.Fingerprint, changed.row.Fingerprint)
	identity.OwnerUserID = 0
	_, err = newApplicationCommand(WithSessionIdentity(context.Background(), identity), actor, "application.submit", key, struct{}{})
	require.Error(t, err, "invalid verified identity never falls back to the effective user")
	identity.OwnerUserID = 3
	identity.EffectiveUserID = 8
	_, err = newApplicationCommand(WithSessionIdentity(context.Background(), identity), actor, "application.submit", key, struct{}{})
	require.Error(t, err)
	legacy, err := newApplicationCommand(context.Background(), actor, "application.submit", "", struct{}{})
	require.NoError(t, err)
	require.Nil(t, legacy)
}
