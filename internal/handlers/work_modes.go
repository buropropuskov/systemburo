package handlers

import (
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// WorkModesHandler -- HTTP-обработчик агрегатора режимов работы (read-only).
type WorkModesHandler struct {
	service services.WorkModesService
}

// NewWorkModesHandler создаёт новый обработчик режимов работы.
func NewWorkModesHandler(service services.WorkModesService) *WorkModesHandler {
	return &WorkModesHandler{service: service}
}

// GetWorkModes возвращает расписания Бюро, мест разгрузки и мест прохода в единой
// форме слота с текущим статусом каждого объекта.
// @Summary      Режимы работы
// @Description  Агрегирует расписания Бюро, мест разгрузки и мест прохода (постов) в единую форму слота с current_status. Неархивные объекты, включая операционно неактивные.
// @Tags         work-modes
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} services.WorkModesResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /work-modes [get]
func (h *WorkModesHandler) GetWorkModes(c echo.Context) error {
	modes, err := h.service.GetWorkModes(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, modes)
}
