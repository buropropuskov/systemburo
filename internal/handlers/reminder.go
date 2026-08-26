package handlers

import (
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// ReminderHandler -- HTTP-обработчики отчётов по напоминаниям согласующим (#1315).
type ReminderHandler struct {
	service services.ReminderService
}

// NewReminderHandler создаёт новый экземпляр ReminderHandler.
func NewReminderHandler(service services.ReminderService) *ReminderHandler {
	return &ReminderHandler{service: service}
}

// GetStuckApprovals godoc
// @Summary      Зависшие согласования
// @Description  Текущий снимок заявок, ждущих решения согласующего дольше настроенного порога молчания. Не зависит от периода вкладки: показывает состояние на данный момент, как журнал решений
// @Tags         statistics
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} Response
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /statistics/stuck-approvals [get]
func (h *ReminderHandler) GetStuckApprovals(c echo.Context) error {
	rows, err := h.service.ListStuckApprovals(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, rows)
}
