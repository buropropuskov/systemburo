package middleware

import (
	"net/http"
	"strings"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// JWTAuth creates middleware that validates Bearer tokens.
func JWTAuth(jwtSecret []byte) echo.MiddlewareFunc {
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

			username, _ := claims.GetSubject()
			c.Set("username", username)
			c.Set("user_id", claims.UserID)
			c.Set("is_super_admin", claims.IsSuperAdmin)

			return next(c)
		}
	}
}
