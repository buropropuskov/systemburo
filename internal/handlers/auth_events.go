package handlers

import (
	"strconv"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// AuthEventHandler - чтение истории входов пользователя (auth_events).
type AuthEventHandler struct {
	reader *services.AuthEventReader
}

// NewAuthEventHandler конструирует handler.
func NewAuthEventHandler(reader *services.AuthEventReader) *AuthEventHandler {
	return &AuthEventHandler{reader: reader}
}

// ListForUser godoc
// @Summary      История входов пользователя
// @Description  События аутентификации пользователя (вход/выход/неудачные попытки/блокировки/сессии). Фильтры: category (login/logout/failed/locked/session), from, to (YYYY-MM-DD). Пагинация page/limit.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        username  path   string  true   "Имя пользователя"
// @Param        category  query  string  false  "Категория события (login/logout/failed/locked/session)"
// @Param        from      query  string  false  "Дата с (YYYY-MM-DD, включительно)"
// @Param        to        query  string  false  "Дата по (YYYY-MM-DD, включительно)"
// @Param        page      query  int     false  "Страница"
// @Param        limit     query  int     false  "Размер страницы (<=100)"
// @Success      200 {object} Response
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/auth-events [get]
func (h *AuthEventHandler) ListForUser(c echo.Context) error {
	ctx := c.Request().Context()
	username := c.Param("username")

	userID, ok, err := h.reader.ResolveUserID(ctx, username)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.NotFound("Пользователь не найден")
	}

	f := models.AuthEventFilter{
		UserID:   userID,
		Category: c.QueryParam("category"),
	}
	// Границы периода считаем в МСК: выбранный на фронте день - это московские сутки,
	// иначе UTC-полночь режет день по 03:00 МСК и события «съезжают». created_at хранится
	// как timestamptz, поэтому сравнение с МСК-инстантом корректно.
	loc := services.AnalyticsLocation()
	if v := c.QueryParam("from"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return apperr.Validation("Некорректная дата from (ожидается YYYY-MM-DD)")
		}
		f.From = &t
	}
	if v := c.QueryParam("to"); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, loc)
		if err != nil {
			return apperr.Validation("Некорректная дата to (ожидается YYYY-MM-DD)")
		}
		end := t.Add(24 * time.Hour) // to включительно - граница на начало след. дня
		f.To = &end
	}
	f.Page, _ = strconv.Atoi(c.QueryParam("page"))
	f.Limit, _ = strconv.Atoi(c.QueryParam("limit"))

	resp, err := h.reader.ListForUser(ctx, f)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}
