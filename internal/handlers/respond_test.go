package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"systemburo/internal/apperr"
	"systemburo/internal/handlers"

	"github.com/labstack/echo/v4"
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)

			handlers.CustomHTTPErrorHandler(tc.err, c)

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
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/some/path", nil), rec)

			handlers.CustomHTTPErrorHandler(tc.err, c)

			logged := strings.Contains(buf.String(), "internal error")
			require.Equal(t, tc.wantLog, logged, "лог: %q", buf.String())
			if tc.wantLog {
				require.Contains(t, buf.String(), "/api/some/path", "в логе должен быть путь запроса")
			}
		})
	}
}
