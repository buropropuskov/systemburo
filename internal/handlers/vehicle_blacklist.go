package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// VehicleBlacklistHandler - HTTP-обработчики чёрного списка автомобилей (#443).
type VehicleBlacklistHandler struct {
	service services.VehicleBlacklistService
}

// NewVehicleBlacklistHandler создаёт обработчик.
func NewVehicleBlacklistHandler(service services.VehicleBlacklistService) *VehicleBlacklistHandler {
	return &VehicleBlacklistHandler{service: service}
}

// GetAll godoc
// @Summary      Список чёрного списка машин
// @Tags         vehicle-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        include_archived query bool false "Включать снятые записи"
// @Success      200 {array} models.VehicleBlacklist
// @Router       /vehicle-blacklist [get]
func (h *VehicleBlacklistHandler) GetAll(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	items, err := h.service.GetAll(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// Create godoc
// @Summary      Добавить машину в чёрный список
// @Tags         vehicle-blacklist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateVehicleBlacklistRequest true "Данные записи"
// @Success      201 {object} models.VehicleBlacklist
// @Router       /vehicle-blacklist [post]
func (h *VehicleBlacklistHandler) Create(c echo.Context) error {
	var req models.CreateVehicleBlacklistRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	entry, err := h.service.Create(c.Request().Context(), req, userID)
	if err != nil {
		return err
	}
	return RespondCreated(c, entry)
}

// Delete godoc
// @Summary      Снять машину с чёрного списка (архивация)
// @Tags         vehicle-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID записи"
// @Success      200 {string} string "Машина снята с чёрного списка"
// @Router       /vehicle-blacklist/{id} [delete]
func (h *VehicleBlacklistHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Archive(c.Request().Context(), id, userID); err != nil {
		return err
	}
	return RespondMessage(c, "Машина снята с чёрного списка")
}

// Restore godoc
// @Summary      Вернуть машину в чёрный список
// @Tags         vehicle-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID записи"
// @Success      200 {string} string "Машина возвращена в чёрный список"
// @Router       /vehicle-blacklist/{id}/restore [post]
func (h *VehicleBlacklistHandler) Restore(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Restore(c.Request().Context(), id, userID); err != nil {
		return err
	}
	return RespondMessage(c, "Машина возвращена в чёрный список")
}

// Check godoc
// @Summary      Проверить, в чёрном ли списке машина
// @Tags         vehicle-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        car_number query string true "Номер машины"
// @Param        mark_id query int true "ID марки"
// @Success      200 {object} models.VehicleBlacklistCheckResult
// @Router       /vehicle-blacklist/check [get]
func (h *VehicleBlacklistHandler) Check(c echo.Context) error {
	carNumber := c.QueryParam("car_number")
	markID, err := strconv.Atoi(c.QueryParam("mark_id"))
	if carNumber == "" || err != nil || markID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "car_number и mark_id обязательны")
	}
	res, err := h.service.Check(c.Request().Context(), carNumber, markID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, res)
}

// GetHistory godoc
// @Summary      История записи чёрного списка машин
// @Tags         vehicle-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID записи"
// @Success      200 {array} models.VehicleBlacklistHistoryItem
// @Router       /vehicle-blacklist/{id}/history [get]
func (h *VehicleBlacklistHandler) GetHistory(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	history, err := h.service.GetHistory(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, history)
}
