package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Response is the unified API response envelope.
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RespondSuccess wraps data in a success envelope with status 200.
func RespondSuccess(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, Response{Success: true, Data: data})
}

// RespondCreated wraps data in a success envelope with status 201.
func RespondCreated(c echo.Context, data any) error {
	return c.JSON(http.StatusCreated, Response{Success: true, Data: data})
}

// RespondMessage sends a success envelope with a message string as data.
func RespondMessage(c echo.Context, msg string) error {
	return c.JSON(http.StatusOK, Response{Success: true, Data: msg})
}

// CustomHTTPErrorHandler wraps all errors in the unified envelope format.
// Replaces Echo's default error handler. Handles echo.HTTPError, plain errors,
// and service-layer errors that are already echo.HTTPError.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	msg := "Internal server error"

	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		switch m := he.Message.(type) {
		case string:
			msg = m
		case error:
			msg = m.Error()
		default:
			msg = http.StatusText(code)
		}
	} else {
		msg = err.Error()
	}

	c.JSON(code, Response{Success: false, Error: msg})
}
