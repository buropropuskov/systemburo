package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// BureauHandler -- HTTP-обработчики расписания работы Бюро (single-owner).
type BureauHandler struct {
	service services.BureauService
}

// NewBureauHandler создаёт новый экземпляр обработчика расписания Бюро.
func NewBureauHandler(service services.BureauService) *BureauHandler {
	return &BureauHandler{service: service}
}

// GetTimeSlots возвращает временные слоты расписания Бюро.
// @Summary      Получение расписания Бюро
// @Description  Возвращает все временные слоты расписания работы Бюро
// @Tags         bureau
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.BureauTimeSlot
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /bureau/time-slots [get]
func (h *BureauHandler) GetTimeSlots(c echo.Context) error {
	slots, err := h.service.GetTimeSlots(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, slots)
}

// AddTimeSlot добавляет временной слот в расписание Бюро.
// @Summary      Добавление слота расписания Бюро
// @Description  Создаёт новый временной слот расписания работы Бюро
// @Tags         bureau
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.CreateTimeSlotRequest true "Данные временного слота"
// @Success      200 {object} map[string]interface{} "id и message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /bureau/time-slots [post]
func (h *BureauHandler) AddTimeSlot(c echo.Context) error {
	var req services.CreateTimeSlotRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.AddTimeSlot(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Временной слот успешно добавлен",
	})
}

// UpdateTimeSlot обновляет временной слот расписания Бюро.
// @Summary      Обновление слота расписания Бюро
// @Description  Обновляет поля временного слота расписания Бюро (только переданные)
// @Tags         bureau
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        slot_id path int true "ID временного слота"
// @Param        request body services.UpdateTimeSlotRequest true "Обновляемые поля"
// @Success      200 {string} string "Временной слот успешно обновлен"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /bureau/time-slots/{slot_id} [put]
func (h *BureauHandler) UpdateTimeSlot(c echo.Context) error {
	slotID, err := strconv.Atoi(c.Param("slot_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid slot_id")
	}
	var req services.UpdateTimeSlotRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateTimeSlot(c.Request().Context(), slotID, req); err != nil {
		return err
	}
	return RespondMessage(c, "Временной слот успешно обновлен")
}

// DeleteTimeSlot удаляет временной слот расписания Бюро.
// @Summary      Удаление слота расписания Бюро
// @Description  Удаляет временной слот расписания работы Бюро
// @Tags         bureau
// @Produce      json
// @Security     BearerAuth
// @Param        slot_id path int true "ID временного слота"
// @Success      200 {string} string "Временной слот успешно удален"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /bureau/time-slots/{slot_id} [delete]
func (h *BureauHandler) DeleteTimeSlot(c echo.Context) error {
	slotID, err := strconv.Atoi(c.Param("slot_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid slot_id")
	}
	if err := h.service.DeleteTimeSlot(c.Request().Context(), slotID); err != nil {
		return err
	}
	return RespondMessage(c, "Временной слот успешно удален")
}
