package middleware

import (
	"strings"

	"systemburo/internal/httpx"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

const (
	// HeaderRequestID is the HTTP header used for request tracing.
	HeaderRequestID = "X-Request-ID"

	// ContextKeyRequestID is the echo.Context key for the request ID.
	ContextKeyRequestID = "request_id"
)

// RequestID returns middleware that ensures every request has a unique ID.
// A UUID supplied by the proxy is reused. Other values are replaced.
// The ID is stored in the
// echo.Context and set as a response header.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqID := c.Request().Header.Get(HeaderRequestID)
			if !validRequestID(reqID) {
				reqID = uuid.New().String()
			}
			reqID = strings.ToLower(reqID)

			c.Set(ContextKeyRequestID, reqID)
			c.SetRequest(c.Request().WithContext(httpx.WithRequestID(c.Request().Context(), reqID)))
			c.Response().Header().Set(HeaderRequestID, reqID)

			return next(c)
		}
	}
}

func validRequestID(value string) bool {
	if len(value) != 32 && len(value) != 36 {
		return false
	}
	_, err := uuid.Parse(value)
	return err == nil
}

// GetRequestID extracts the request ID from the echo.Context.
// Returns an empty string if no request ID is set.
func GetRequestID(c echo.Context) string {
	if id, ok := c.Get(ContextKeyRequestID).(string); ok {
		return id
	}
	return ""
}
