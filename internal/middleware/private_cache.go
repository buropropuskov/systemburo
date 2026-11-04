package middleware

import (
	"strings"

	"systemburo/internal/download"

	"github.com/labstack/echo/v4"
)

// PrivateAPICache prevents caches from retaining API responses, including public
// settings and authentication responses. Register it globally before middleware
// that can reject a request so their error responses receive the same policy.
// Handlers may still set their stream-specific policy (for example SSE no-cache).
// Assets and HTML outside /api retain their existing cache policy.
func PrivateAPICache() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			if path == "/api" || strings.HasPrefix(path, "/api/") {
				download.NoStore(c)
			}
			return next(c)
		}
	}
}
