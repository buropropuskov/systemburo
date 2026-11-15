package middleware

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
)

// SafeRecover preserves HTTP recovery without printing panic values or stacks.
func SafeRecover() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) (err error) {
			defer func() {
				if value := recover(); value != nil {
					if value == http.ErrAbortHandler {
						panic(value)
					}
					slog.ErrorContext(c.Request().Context(), "request panic",
						"request_id", GetRequestID(c), "route", c.Path())
					err = echo.ErrInternalServerError
				}
			}()
			return next(c)
		}
	}
}
