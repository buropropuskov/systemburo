package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
