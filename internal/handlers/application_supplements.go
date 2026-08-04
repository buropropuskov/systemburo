package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// CreateSupplement godoc
// @Summary      Дополнить поданную заявку
// @Description  Добавляет людей, машины или ТМЦ во вложения уже поданной заявки (#1685).
// @Description  Пока заявка не принята в работу, добавка вливается в текущий круг согласования
// @Description  (статус раунда merged); у заявки в работе заводится отдельный раунд (pending),
// @Description  а согласование и статус самой заявки не откатываются - уже допущенные строки
// @Description  остаются на КПП.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        request body services.CreateSupplementRequest true "Состав дополнения"
// @Success      200 {object} services.CreateSupplementResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/supplements [post]
func (h *ApplicationHandler) CreateSupplement(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.CreateSupplementRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	username := c.Get("username").(string)
	resp, err := h.service.CreateSupplement(c.Request().Context(), username, id, IsSuperAdmin(c), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// GetApplicationSupplements godoc
// @Summary      Дополнения заявки
// @Description  Возвращает раунды дополнения заявки (новые сверху) с составом голосующих.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array}  services.SupplementInfo
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/supplements [get]
func (h *ApplicationHandler) GetApplicationSupplements(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	supplements, err := h.service.GetApplicationSupplements(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, supplements)
}
