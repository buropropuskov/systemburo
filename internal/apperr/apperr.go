// Package apperr -- типизированные доменные ошибки приложения.
//
// Хендлеры/сервисы возвращают apperr.Validation/NotFound/... вместо ad-hoc
// echo.NewHTTPError или ручного c.JSON. Маппинг в HTTP-статус, единый envelope
// {success,error} и логирование 5xx делает одно место -- handlers.CustomHTTPErrorHandler.
// Так правила "валидация = 400, не найдено = 404, тело ошибки = {success:false,error}"
// живут в одной точке, а не переизобретаются в каждом обработчике.
package apperr

import (
	"fmt"
	"net/http"
)

// Error -- доменная ошибка с HTTP-статусом и человекочитаемым сообщением.
type Error struct {
	Code    int               // HTTP-статус
	Message string            // сообщение для пользователя (попадает в envelope.error)
	Headers map[string]string // доп. HTTP-заголовки ответа (Retry-After и т.п.), опционально
	err     error             // обёрнутая первопричина (для логов, %w)
}

func (e *Error) Error() string {
	if e.err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.err)
	}
	return e.Message
}

// Unwrap поддерживает errors.Is/As по обёрнутой первопричине.
func (e *Error) Unwrap() error { return e.err }

// WithHeader добавляет HTTP-заголовок, который CustomHTTPErrorHandler выставит в
// ответ вместе с телом ошибки. Пример: Retry-After при 429. Возвращает сам Error
// для чейнинга: apperr.TooManyRequests(msg).WithHeader("Retry-After", "30").
func (e *Error) WithHeader(key, value string) *Error {
	if e.Headers == nil {
		e.Headers = make(map[string]string, 1)
	}
	e.Headers[key] = value
	return e
}

// New создаёт доменную ошибку с произвольным статусом. wrapped опционален.
func New(code int, message string, wrapped ...error) *Error {
	var w error
	if len(wrapped) > 0 {
		w = wrapped[0]
	}
	return &Error{Code: code, Message: message, err: w}
}

// Validation -- 400, невалидный вход.
func Validation(message string, wrapped ...error) *Error {
	return New(http.StatusBadRequest, message, wrapped...)
}

// Unauthorized -- 401.
func Unauthorized(message string, wrapped ...error) *Error {
	return New(http.StatusUnauthorized, message, wrapped...)
}

// Forbidden -- 403, нет доступа.
func Forbidden(message string, wrapped ...error) *Error {
	return New(http.StatusForbidden, message, wrapped...)
}

// NotFound -- 404.
func NotFound(message string, wrapped ...error) *Error {
	return New(http.StatusNotFound, message, wrapped...)
}

// Conflict -- 409, конфликт состояния (дубль, занятость).
func Conflict(message string, wrapped ...error) *Error {
	return New(http.StatusConflict, message, wrapped...)
}

// TooManyRequests -- 429, превышен лимит запросов / временная блокировка.
// Обычно дополняется .WithHeader("Retry-After", "<секунды>").
func TooManyRequests(message string, wrapped ...error) *Error {
	return New(http.StatusTooManyRequests, message, wrapped...)
}

// Internal -- 500, внутренняя ошибка (логируется централизованно).
func Internal(message string, wrapped ...error) *Error {
	return New(http.StatusInternalServerError, message, wrapped...)
}
