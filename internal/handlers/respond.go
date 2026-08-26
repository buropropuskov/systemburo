package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"systemburo/internal/apperr"

	"github.com/labstack/echo/v4"
)

// Response is the unified API response envelope.
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Meta    any    `json:"meta,omitempty"`
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

// RespondAccepted wraps data in a success envelope with status 202: запрос принят,
// но обработка асинхронна (фоновый воркер) и результат этим ответом не гарантирован.
func RespondAccepted(c echo.Context, data any) error {
	return c.JSON(http.StatusAccepted, Response{Success: true, Data: data})
}

// RespondMessage sends a success envelope with a message string as data.
func RespondMessage(c echo.Context, msg string) error {
	return c.JSON(http.StatusOK, Response{Success: true, Data: msg})
}

// RespondPaginated wraps data + pagination meta in a success envelope. meta is typically
// models.PaginationMeta, but any is accepted so callers can embed it with extra fields
// (e.g. models.NotificationListMeta adds unread_count, #1748).
func RespondPaginated(c echo.Context, data any, meta any) error {
	return c.JSON(http.StatusOK, Response{Success: true, Data: data, Meta: meta})
}

// CustomHTTPErrorHandler wraps all errors in the unified envelope format.
// Replaces Echo's default error handler. Единая точка маппинга ошибок в статус и
// тело {success:false,error}: типизированные apperr.Error -> свой статус/сообщение,
// echo.HTTPError -> как раньше, всё прочее -> 500 + лог.
func CustomHTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}

	code := http.StatusInternalServerError
	msg := "Internal server error"

	var ae *apperr.Error
	if errors.As(err, &ae) {
		code = ae.Code
		msg = ae.Message
		// Доп. заголовки ошибки (Retry-After и т.п.) выставляем до тела ответа.
		for k, v := range ae.Headers {
			c.Response().Header().Set(k, v)
		}
		if code >= http.StatusInternalServerError {
			slog.Error("internal error", "error", err, "path", c.Request().URL.Path, "method", c.Request().Method)
		}
	} else if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		switch m := he.Message.(type) {
		case string:
			msg = m
		case error:
			msg = m.Error()
		default:
			msg = http.StatusText(code)
		}
		// 5xx через echo.HTTPError раньше не логировался вовсе: клиент получал
		// текст, а в логах не оставалось ничего, и такую аварию нельзя было
		// разобрать постфактум. Путь этот массовый - сервисы отдают 500 именно так.
		if code >= http.StatusInternalServerError {
			slog.Error("internal error", "error", err, "path", c.Request().URL.Path, "method", c.Request().Method)
		}
	} else {
		slog.Error("unhandled error", "error", err)
		msg = "Internal server error"
	}

	c.JSON(code, Response{Success: false, Error: msg})
}
