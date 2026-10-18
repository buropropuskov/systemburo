package middleware_test

import (
	"context"
	"net/http/httptest"
	"testing"

	mw "systemburo/internal/middleware"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestSessionRequestPolicy_UsesFreshIdentityAndRoute(t *testing.T) {
	policy := mw.NewSessionRequestPolicy(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			require.Equal(t, "/api/events", c.Path())
			require.Equal(t, "GET", c.Request().Method)
			require.Equal(t, 42, c.Get("user_id"))
			require.Equal(t, false, c.Get("is_super_admin"))
			return next(c)
		}
	})
	require.NoError(t, policy.Check(context.Background(), models.SessionIdentity{EffectiveUserID: 42}, "GET", "/api/events"))
}

func TestSessionRequestPolicy_JSONDenialIsNotSuccess(t *testing.T) {
	for _, status := range []int{403, 503} {
		policy := mw.NewSessionRequestPolicy(func(echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error { return c.JSON(status, map[string]bool{"success": false}) }
		})
		var failure *echo.HTTPError
		require.ErrorAs(t, policy.Check(context.Background(), models.SessionIdentity{}, "GET", "/api/events"), &failure)
		require.Equal(t, status, failure.Code)
	}
}

func TestSessionRequestPolicy_PreservesAuditMetadataWithoutTicketQuery(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/events?ticket=synthetic-secret", nil)
	request.RemoteAddr = "192.0.2.10:1234"
	request.Header.Set("User-Agent", "synthetic-agent")
	policy := mw.NewSessionRequestPolicy(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			require.Equal(t, "192.0.2.10", c.RealIP())
			require.Equal(t, "synthetic-agent", c.Request().UserAgent())
			require.Empty(t, c.Request().URL.RawQuery)
			require.Equal(t, "/api/events/ticket", c.Request().RequestURI)
			return next(c)
		}
	})
	require.NoError(t, policy.Check(context.Background(), models.SessionIdentity{}, "POST", "/api/events/ticket", request))
	require.Equal(t, "ticket=synthetic-secret", request.URL.RawQuery, "the original request must not be modified")
}
