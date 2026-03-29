package middleware

import (
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
// If the client sends an X-Request-ID header, that value is reused;
// otherwise a new UUID v4 is generated. The ID is stored in the
// echo.Context and set as a response header.
func RequestID() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqID := c.Request().Header.Get(HeaderRequestID)
			if reqID == "" {
				reqID = uuid.New().String()
			}

			c.Set(ContextKeyRequestID, reqID)
			c.Response().Header().Set(HeaderRequestID, reqID)

			return next(c)
		}
	}
}

// GetRequestID extracts the request ID from the echo.Context.
// Returns an empty string if no request ID is set.
func GetRequestID(c echo.Context) string {
	if id, ok := c.Get(ContextKeyRequestID).(string); ok {
		return id
	}
	return ""
}
