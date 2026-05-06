package handlers

import (
	"net/http"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// AccessDenialHandler -- HTTP-обработчики журнала отказов в доступе.
type AccessDenialHandler struct {
	service *services.AccessDenialService
}

// NewAccessDenialHandler конструирует handler.
func NewAccessDenialHandler(s *services.AccessDenialService) *AccessDenialHandler {
	return &AccessDenialHandler{service: s}
}

// List -- GET /access-denials.
// Фильтры через query: user_id, resource (substring), reason, from, to, page, limit.
func (h *AccessDenialHandler) List(c echo.Context) error {
	f, err := h.parseFilter(c)
	if err != nil {
		return err
	}
	resp, err := h.service.List(c.Request().Context(), f)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// ListArchive -- GET /access-denials/archive.
func (h *AccessDenialHandler) ListArchive(c echo.Context) error {
	f, err := h.parseFilter(c)
	if err != nil {
		return err
	}
	resp, err := h.service.ListArchive(c.Request().Context(), f)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// DeleteByFilter -- DELETE /access-denials.
// Удаляет записи активной таблицы по фильтрам. Архив не трогается.
func (h *AccessDenialHandler) DeleteByFilter(c echo.Context) error {
	f, err := h.parseFilter(c)
	if err != nil {
		return err
	}
	count, err := h.service.DeleteByFilter(c.Request().Context(), f)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"deleted": count})
}

// ArchiveOlderThan -- POST /access-denials/archive?cutoff=2026-01-01T00:00:00Z.
// Переносит записи старше cutoff в архив. Без cutoff используется now-3m.
func (h *AccessDenialHandler) ArchiveOlderThan(c echo.Context) error {
	cutoffStr := c.QueryParam("cutoff")
	cutoff := time.Now().AddDate(0, -3, 0)
	if cutoffStr != "" {
		t, err := time.Parse(time.RFC3339, cutoffStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid cutoff: "+err.Error())
		}
		cutoff = t
	}
	count, err := h.service.ArchiveOlderThan(c.Request().Context(), cutoff)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"archived": count})
}

func (h *AccessDenialHandler) parseFilter(c echo.Context) (models.AccessDenialFilter, error) {
	var f models.AccessDenialFilter
	if err := c.Bind(&f); err != nil {
		return models.AccessDenialFilter{}, echo.NewHTTPError(http.StatusBadRequest, "invalid filter: "+err.Error())
	}
	return f, nil
}
