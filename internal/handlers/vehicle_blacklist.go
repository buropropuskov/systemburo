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

// Update godoc
// @Summary      Редактировать запись чёрного списка (номер, марка, причина)
// @Tags         vehicle-blacklist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID записи"
// @Param        request body models.UpdateVehicleBlacklistRequest true "Номер, марка, причина"
// @Success      200 {object} models.VehicleBlacklist
// @Router       /vehicle-blacklist/{id} [put]
func (h *VehicleBlacklistHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateVehicleBlacklistRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	entry, err := h.service.Update(c.Request().Context(), id, req, userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, entry)
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

// Impact godoc
// @Summary      Предпросмотр последствий внесения машины в чёрный список
// @Description  Где машина сейчас фигурирует: какие активные строки перестанут действовать, из каких таблиц постов уйдут, в каких заявках есть. Ничего не меняет.
// @Tags         vehicle-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        car_number query string true "Номер машины"
// @Param        mark_id    query int    true "ID марки"
// @Success      200 {object} map[string]interface{} "success + данные предпросмотра"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /vehicle-blacklist/impact [get]
func (h *VehicleBlacklistHandler) Impact(c echo.Context) error {
	carNumber := c.QueryParam("car_number")
	markID, err := strconv.Atoi(c.QueryParam("mark_id"))
	if carNumber == "" || err != nil || markID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "car_number и mark_id обязательны")
	}
	impact, err := h.service.Impact(c.Request().Context(), carNumber, markID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, impact)
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

// GetAllHistory godoc
// @Summary      Весь журнал чёрного списка машин
// @Tags         vehicle-blacklist
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.VehicleBlacklistHistoryItem
// @Router       /vehicle-blacklist/history [get]
func (h *VehicleBlacklistHandler) GetAllHistory(c echo.Context) error {
	history, err := h.service.GetAllHistory(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, history)
}

// BulkArchive godoc
// @Summary      Групповое снятие машин с чёрного списка
// @Tags         vehicle-blacklist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID записей"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /vehicle-blacklist/bulk/archive [post]
func (h *VehicleBlacklistHandler) BulkArchive(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны записи")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkArchive(c.Request().Context(), req.IDs, userID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkRestore godoc
// @Summary      Групповое восстановление машин в чёрный список
// @Tags         vehicle-blacklist
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID записей"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /vehicle-blacklist/bulk/restore [post]
func (h *VehicleBlacklistHandler) BulkRestore(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны записи")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkRestore(c.Request().Context(), req.IDs, userID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// Purge godoc
// @Summary      Удалить запись чёрного списка машин навсегда
// @Tags         vehicle-blacklist
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID записи"
// @Success      200 {string} string "Запись удалена навсегда"
// @Router       /vehicle-blacklist/{id}/purge [delete]
func (h *VehicleBlacklistHandler) Purge(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Purge(c.Request().Context(), id, userID); err != nil {
		return err
	}
	return RespondMessage(c, "Запись удалена навсегда")
}
