package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"systemburo/internal/apperr"
	"systemburo/internal/handlers"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

// TestCustomHTTPErrorHandler_AppErr -- единая точка маппинга доменных ошибок:
// apperr.* -> свой статус + envelope {success:false,error}; echo.HTTPError и
// прочее -> как раньше.
func TestCustomHTTPErrorHandler_AppErr(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
		wantMsg  string
	}{
		{"validation", apperr.Validation("файл не выбран"), http.StatusBadRequest, "файл не выбран"},
		{"not found", apperr.NotFound("место не найдено"), http.StatusNotFound, "место не найдено"},
		{"conflict", apperr.Conflict("уже существует"), http.StatusConflict, "уже существует"},
		{"forbidden", apperr.Forbidden("нет доступа"), http.StatusForbidden, "нет доступа"},
		{"wrapped keeps code", apperr.Validation("плохой ввод", errors.New("low-level")), http.StatusBadRequest, "плохой ввод"},
		{"echo httperror still works", echo.NewHTTPError(http.StatusTeapot, "teapot"), http.StatusTeapot, "teapot"},
		{"wrapped echo", fmt.Errorf("private wrapper: %w", echo.NewHTTPError(http.StatusForbidden, "denied")), http.StatusForbidden, "denied"},
		{"echo 500 private message", echo.NewHTTPError(500, "private SQL or path"), 500, "Internal server error"},
		{"wrapped echo 503", fmt.Errorf("private wrapper: %w", echo.NewHTTPError(503, "private detail")), 503, "Internal server error"},
		{"domain safe server message", apperr.Internal("Не удалось открыть шаблон", errors.New("private path")), 500, "Не удалось открыть шаблон"},
		{"trusted maintenance", apperr.New(503, "Сервис временно недоступен: технические работы"), 503, "Сервис временно недоступен: технические работы"},
		{"generic", errors.New("private SQL or path"), 500, "Internal server error"},
		{"invalid echo status", echo.NewHTTPError(200, "private detail"), 500, "Internal server error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)

			handlers.CustomHTTPErrorHandler(c, tc.err)

			require.Equal(t, tc.wantCode, rec.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, false, body["success"])
			require.Equal(t, tc.wantMsg, body["error"])
		})
	}
}

// TestCustomHTTPErrorHandler_LogsServerErrors -- 5xx обязан попадать в лог любым
// путём. У apperr.Error это было всегда, у echo.HTTPError - нет, и сервисы отдают
// 500 именно так, поэтому авария оставалась без следа. 4xx в лог не идут: это
// нормальный ответ на неверный запрос, а не отказ системы.
func TestCustomHTTPErrorHandler_LogsServerErrors(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		wantLog bool
	}{
		{"echo 500 логируется", echo.NewHTTPError(http.StatusInternalServerError, "не удалось"), true},
		{"echo 503 логируется", echo.NewHTTPError(http.StatusServiceUnavailable, "недоступно"), true},
		{"echo 400 не логируется", echo.NewHTTPError(http.StatusBadRequest, "неверный ввод"), false},
		{"apperr 500 логируется", apperr.Internal("внутренняя ошибка"), true},
		{"apperr 404 не логируется", apperr.NotFound("не найдено"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})))
			defer slog.SetDefault(prev)

			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/private-person?token=private-token", nil), rec)
			c.SetPath("/api/:id")
			rec.Header().Set(echo.HeaderXRequestID, "generated-request-id")

			handlers.CustomHTTPErrorHandler(c, tc.err)

			logged := strings.Contains(buf.String(), "internal error")
			require.Equal(t, tc.wantLog, logged, "лог: %q", buf.String())
			if tc.wantLog {
				require.Contains(t, buf.String(), "/api/:id")
				require.Contains(t, buf.String(), "request_id=generated-request-id")
				require.Contains(t, buf.String(), "error_type=")
				require.NotContains(t, buf.String(), "private-person")
				require.NotContains(t, buf.String(), "private-token")
				require.NotContains(t, buf.String(), tc.err.Error())
			}
		})
	}
}

func TestCustomHTTPErrorHandler_DomainHeadersAndCommittedResponse(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	handlers.CustomHTTPErrorHandler(c, fmt.Errorf("private wrapper: %w",
		apperr.New(503, "Сервис временно занят").WithHeader("Retry-After", "2")))
	require.Equal(t, 503, rec.Code)
	require.Equal(t, "2", rec.Header().Get("Retry-After"))
	require.NotContains(t, rec.Body.String(), "private wrapper")

	before := rec.Body.String()
	handlers.CustomHTTPErrorHandler(c, echo.ErrInternalServerError)
	require.Equal(t, 503, rec.Code)
	require.Equal(t, before, rec.Body.String())
}
