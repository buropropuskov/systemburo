package middleware

import (
	"log/slog"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

// SafeRecover preserves HTTP recovery without printing panic values or stacks.
func SafeRecover() echo.MiddlewareFunc {
	return echomw.RecoverWithConfig(echomw.RecoverConfig{
		DisablePrintStack: true,
		LogErrorFunc: func(c echo.Context, err error, _ []byte) error {
			slog.ErrorContext(c.Request().Context(), "request panic",
				"request_id", GetRequestID(c), "route", c.Path(), "error", err)
			return err
		},
	})
}
