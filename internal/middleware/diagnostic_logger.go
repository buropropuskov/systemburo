package middleware

import (
	"log/slog"
	"time"

	"systemburo/internal/httpx"

	"github.com/labstack/echo/v5"
)

// DiagnosticLogger records only routing metadata, never request values.
func DiagnosticLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			start := time.Now()
			err := next(c)
			route := c.Path()
			if route == "" {
				route = "/unmatched"
			}
			slog.InfoContext(c.Request().Context(), "http request",
				"request_id", GetRequestID(c),
				"method", c.Request().Method,
				"route", route,
				"status", httpx.StatusCode(c, err),
				"duration_ms", time.Since(start).Milliseconds())
			return err
		}
	}
}
