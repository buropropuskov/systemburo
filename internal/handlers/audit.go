package handlers

import (
	"strconv"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// AuditHandler - generic endpoint журнала аудита (#870).
type AuditHandler struct {
	reader services.AuditReader
}

// NewAuditHandler создаёт AuditHandler.
func NewAuditHandler(reader services.AuditReader) *AuditHandler {
	return &AuditHandler{reader: reader}
}

// GetAuditLog godoc
// @Summary      Журнал аудита
// @Description  Единый журнал действий (#870). Фильтры: entity_type, entity_id, action, actor_user_id, from, to (YYYY-MM-DD). Пагинация page/per_page.
// @Tags         audit
// @Produce      json
// @Security     BearerAuth
// @Param        entity_type   query  string  false  "Тип сущности (citizenship, car, application, ...)"
// @Param        entity_id     query  int     false  "ID сущности (история одной сущности)"
// @Param        action        query  string  false  "Тип действия (created/updated/...)"
// @Param        actor_user_id query  int     false  "ID пользователя-инициатора"
// @Param        from          query  string  false  "Дата с (YYYY-MM-DD, включительно)"
// @Param        to            query  string  false  "Дата по (YYYY-MM-DD, включительно)"
// @Param        page          query  int     false  "Страница"
// @Param        per_page      query  int     false  "Размер страницы (<=100)"
// @Success      200  {object}  Response
// @Router       /api/audit [get]
func (h *AuditHandler) GetAuditLog(c echo.Context) error {
	q := services.AuditQuery{
		EntityType: c.QueryParam("entity_type"),
		Action:     c.QueryParam("action"),
	}

	if v := c.QueryParam("entity_id"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil {
			return apperr.Validation("Некорректный entity_id")
		}
		q.EntityID = &id
	}
	if v := c.QueryParam("actor_user_id"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil {
			return apperr.Validation("Некорректный actor_user_id")
		}
		q.ActorUserID = &id
	}
	if v := c.QueryParam("from"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return apperr.Validation("Некорректная дата from (ожидается YYYY-MM-DD)")
		}
		q.From = &t
	}
	if v := c.QueryParam("to"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			return apperr.Validation("Некорректная дата to (ожидается YYYY-MM-DD)")
		}
		end := t.Add(24 * time.Hour) // to включительно - граница на начало след. дня
		q.To = &end
	}

	var p models.PaginationParams
	if err := c.Bind(&p); err != nil {
		p = models.PaginationParams{}
	}
	p.Normalize()
	q.Page = p.Page
	q.PerPage = p.PerPage

	items, total, err := h.reader.List(c.Request().Context(), q)
	if err != nil {
		return err
	}
	return RespondPaginated(c, items, models.PaginationMeta{Total: total, Page: p.Page, PerPage: p.PerPage})
}
