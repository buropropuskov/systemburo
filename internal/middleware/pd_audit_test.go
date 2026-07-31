package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// Сверка путей журнала ПДн (#1472). Перечень задавался без префикса /api, а роутер
// вешает всё на api := e.Group("/api") - совпадений не было ни разу, журнал стоял пустым.
func TestIsPDPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"/api/employees", true},
		{"/api/employees/12/history", true},
		{"/api/unique-employees", true},
		{"/api/attachments/5/employees", true},
		{"/api/applications/89/blank", true},
		{"/api/applications/available-attachments/111", true},
		// Сквозной поиск отдаёт ФИО сотрудников: вход другой, данные те же.
		// Строка запроса сюда не доходит, isPDPath получает URL.Path.
		{"/api/search", true},
		// Выгрузка из файлового архива (#1615): ZIP за период уносит бланки сотен
		// заявок с паспортами. Поток байтов идёт мимо JWT по одноразовому билету,
		// поэтому единственный, кто видит это обращение, - журнал по пути.
		{"/api/file-archive/download", true},
		{"/api/file-archive/files/42", true},
		{"/api/file-archive/items", true},
		{"/api/file-archive/estimate", true},
		// Настройки раскладки и сводка места персональных данных не отдают.
		{"/api/file-archive/settings", false},
		{"/api/file-archive/stats", false},
		// без префикса /api такого запроса не бывает: так выглядел старый перечень
		{"/employees", false},
		{"/attachments/5", false},
		// соседние адреса без персональных данных
		{"/api/applications/89", false},
		{"/api/applications/available-attachments", false},
		{"/api/applications/89/attachments", false},
		{"/api/cars", false},
		{"/api/login", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			require.Equal(t, tc.want, isPDPath(tc.path))
		})
	}
}

func TestPathToResource(t *testing.T) {
	cases := map[string]string{
		"/api/employees/7":                          "employee",
		"/api/unique-employees":                     "unique_employee",
		"/api/attachments/5/employees":              "attachment",
		"/api/applications/89/blank":                "attachment_blank",
		"/api/applications/available-attachments/1": "available_attachment",
		"/api/cars":                                 "unknown",
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			require.Equal(t, want, pathToResource(path))
		})
	}
}

func TestMethodToAction(t *testing.T) {
	require.Equal(t, "view", methodToAction("GET"))
	require.Equal(t, "create", methodToAction("POST"))
	require.Equal(t, "update", methodToAction("PUT"))
	require.Equal(t, "update", methodToAction("PATCH"))
	require.Equal(t, "delete", methodToAction("DELETE"))
	require.Equal(t, "OPTIONS", methodToAction("OPTIONS"))
}

// Идентификатор пользователя нужен рядом с именем: по одному имени запись не
// привязать к учётке после переименования или архивации.
func TestPDUserID(t *testing.T) {
	newCtx := func(set func(c echo.Context)) echo.Context {
		e := echo.New()
		c := e.NewContext(httptest.NewRequest("GET", "/api/employees", nil), httptest.NewRecorder())
		if set != nil {
			set(c)
		}
		return c
	}

	require.Nil(t, pdUserID(newCtx(nil)), "без user_id в контексте")
	require.Nil(t, pdUserID(newCtx(func(c echo.Context) { c.Set("user_id", 0) })), "нулевой id не пишем")
	require.Nil(t, pdUserID(newCtx(func(c echo.Context) { c.Set("user_id", "42") })), "чужой тип не пишем")

	id := pdUserID(newCtx(func(c echo.Context) { c.Set("user_id", 42) }))
	require.NotNil(t, id)
	require.Equal(t, 42, *id)
}

// Код ответа для журнала (#1472): Echo вызывает обработчик ошибок уже после цепочки
// middleware, поэтому у отказа Response().Status здесь ещё 200 - и отказ в доступе
// попадал в журнал как успешный просмотр.
func TestPDStatusCode(t *testing.T) {
	newCtx := func() echo.Context {
		e := echo.New()
		return e.NewContext(httptest.NewRequest("GET", "/api/employees", nil), httptest.NewRecorder())
	}

	// у успешного запроса ответ уже записан, поэтому статус берётся из Response
	okCtx := newCtx()
	require.NoError(t, okCtx.NoContent(http.StatusOK))
	require.Equal(t, http.StatusOK, pdStatusCode(okCtx, nil), "успешный запрос")
	require.Equal(t, http.StatusForbidden,
		pdStatusCode(newCtx(), echo.NewHTTPError(http.StatusForbidden, "Access denied")))
	require.Equal(t, http.StatusNotFound,
		pdStatusCode(newCtx(), echo.NewHTTPError(http.StatusNotFound, "не найдено")))
	require.Equal(t, http.StatusInternalServerError,
		pdStatusCode(newCtx(), errors.New("внутренний сбой")), "ошибка не от echo")
}
