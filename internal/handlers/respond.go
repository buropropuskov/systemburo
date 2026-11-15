package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"systemburo/internal/apperr"
	"systemburo/internal/httpx"

	"github.com/labstack/echo/v5"
)

// Response is the unified API response envelope.
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RespondSuccess wraps data in a success envelope with status 200.
func RespondSuccess(c *echo.Context, data any) error {
	return c.JSON(http.StatusOK, Response{Success: true, Data: data})
}

// RespondCreated wraps data in a success envelope with status 201.
func RespondCreated(c *echo.Context, data any) error {
	return c.JSON(http.StatusCreated, Response{Success: true, Data: data})
}

// RespondAccepted wraps data in a success envelope with status 202: запрос принят,
// но обработка асинхронна (фоновый воркер) и результат этим ответом не гарантирован.
func RespondAccepted(c *echo.Context, data any) error {
	return c.JSON(http.StatusAccepted, Response{Success: true, Data: data})
}

// RespondMessage sends a success envelope with a message string as data.
func RespondMessage(c *echo.Context, msg string) error {
	return c.JSON(http.StatusOK, Response{Success: true, Data: msg})
}

// RespondPaginated wraps data + pagination meta in a success envelope. meta is typically
// models.PaginationMeta, but any is accepted so callers can embed it with extra fields
// (e.g. models.NotificationListMeta adds unread_count, #1748).
func RespondPaginated(c *echo.Context, data any, meta any) error {
	return c.JSON(http.StatusOK, Response{Success: true, Data: data, Meta: meta})
}

// CustomHTTPErrorHandler wraps all errors in the unified envelope format.
// Replaces Echo's default error handler. Единая точка маппинга ошибок в статус и
// тело {success:false,error}: типизированные apperr.Error -> свой статус/сообщение,
// echo.HTTPError поддерживает обёртки. Технические 5xx имеют нейтральный ответ.
func CustomHTTPErrorHandler(c *echo.Context, err error) {
	if httpx.ResponseCommitted(c) {
		return
	}

	code := httpx.StatusCode(c, err)
	msg := "Internal server error"

	var ae *apperr.Error
	if errors.As(err, &ae) && ae != nil {
		msg = ae.Message
		// Доп. заголовки ошибки (Retry-After и т.п.) выставляем до тела ответа.
		for k, v := range ae.Headers {
			c.Response().Header().Set(k, v)
		}
	} else if code < http.StatusInternalServerError {
		msg = http.StatusText(code)
		var he *echo.HTTPError
		if errors.As(err, &he) && he != nil && he.Message != "" {
			msg = he.Message
		}
	}

	if code >= http.StatusInternalServerError {
		slog.Error("internal error",
			"status", code, "route", c.Path(), "method", c.Request().Method,
			"request_id", c.Response().Header().Get(echo.HeaderXRequestID),
			"error_type", fmt.Sprintf("%T", err))
	}
	_ = c.JSON(code, Response{Success: false, Error: msg})
}
