package middleware

import (
	"net/http"
	"strings"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// JWTAuth creates middleware that validates Bearer tokens.
func JWTAuth(jwtSecret []byte, sessions services.SessionAccess) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
				return echo.NewHTTPError(http.StatusUnauthorized, "Missing or invalid authorization header")
			}

			tokenString := strings.TrimPrefix(auth, "Bearer ")
			claims, err := services.DecodeAccessToken(tokenString, jwtSecret)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "Invalid token")
			}

			if sessions == nil {
				return echo.NewHTTPError(http.StatusServiceUnavailable, "Проверка сессии временно недоступна")
			}
			var identity models.SessionIdentity
			if c.Request().Method == http.MethodPost && c.Path() == "/api/logout" {
				identity, err = sessions.ResolveLogout(c.Request().Context(), claims)
			} else {
				identity, err = sessions.ResolveAccess(c.Request().Context(), claims)
			}
			if err != nil {
				return err
			}
			SetSessionIdentity(c, identity)

			return next(c)
		}
	}
}

// SetSessionIdentity installs a verified identity for the existing request gates
// and audit recorder. Callers must resolve or revalidate it before installation.
func SetSessionIdentity(c echo.Context, identity models.SessionIdentity) {
	c.Set("username", identity.Username)
	c.Set("user_id", identity.EffectiveUserID)
	c.Set("is_super_admin", identity.IsSuperAdmin)
	c.Set("type_id", identity.TypeID)
	ctx := services.WithSessionIdentity(c.Request().Context(), identity)
	if identity.ImpersonatedBy != nil {
		ctx = services.WithImpersonator(ctx, *identity.ImpersonatedBy)
	}
	c.SetRequest(c.Request().WithContext(ctx))
}
