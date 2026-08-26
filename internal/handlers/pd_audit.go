package handlers

import (
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// PDAuditHandler -- чтение журнала доступа к персональным данным (152-ФЗ, #1472).
type PDAuditHandler struct {
	service *services.PDAuditService
}

// NewPDAuditHandler конструирует handler.
func NewPDAuditHandler(s *services.PDAuditService) *PDAuditHandler {
	return &PDAuditHandler{service: s}
}

// List godoc
// @Summary      Журнал доступа к персональным данным
// @Description  Кто и когда обращался к данным сотрудников. Фильтры через query: user_id, username, action, resource, only_denied, from, to, page, limit. Право page.admin.pd_audit.
// @Tags         pd-audit
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.PDAuditPageResponse
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /pd-audit [get]
func (h *PDAuditHandler) List(c echo.Context) error {
	var f models.PDAuditFilter
	if err := c.Bind(&f); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid filter: "+err.Error())
	}
	resp, err := h.service.List(c.Request().Context(), f)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}
