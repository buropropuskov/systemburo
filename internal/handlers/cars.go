package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// CarHandler -- HTTP-обработчики автомобилей в заявках.
type CarHandler struct {
	service services.CarService
}

// NewCarHandler создаёт новый экземпляр CarHandler.
func NewCarHandler(service services.CarService) *CarHandler {
	return &CarHandler{service: service}
}

// GetActiveCarsForTables обрабатывает GET /cars/active-for-tables.
// @Summary Получение активных машин для таблиц
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Success 200 {array} services.TableCarResponse
// @Router /cars/active-for-tables [get]
func (h *CarHandler) GetActiveCarsForTables(c echo.Context) error {
	cars, err := h.service.GetActiveCarsForTables(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cars)
}

// GetFactCarsForTables обрабатывает GET /cars/fact-for-tables.
// @Summary Получение машин «по факту» для таблиц
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Success 200 {array} services.TableCarResponse
// @Router /cars/fact-for-tables [get]
func (h *CarHandler) GetFactCarsForTables(c echo.Context) error {
	cars, err := h.service.GetFactCarsForTables(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, cars)
}

// GetCarUnloadPlaces обрабатывает GET /cars/unload-places.
// @Summary Получение связей активных машин с местами разгрузки
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Success 200 {array} services.CarUnloadPlaceInfo
// @Router /cars/unload-places [get]
func (h *CarHandler) GetCarUnloadPlaces(c echo.Context) error {
	places, err := h.service.GetCarUnloadPlaces(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, places)
}

// GetFactCarUnloadPlaces обрабатывает GET /cars/fact-unload-places.
// @Summary Получение связей машин «по факту» с местами разгрузки
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Success 200 {array} services.CarUnloadPlaceInfo
// @Router /cars/fact-unload-places [get]
func (h *CarHandler) GetFactCarUnloadPlaces(c echo.Context) error {
	places, err := h.service.GetFactCarUnloadPlaces(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, places)
}

// CheckActiveCar обрабатывает GET /cars/check-active.
// @Summary Проверка наличия активной машины по параметрам
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Param car_number query string true "Номер автомобиля"
// @Param car_brand query string true "Марка автомобиля"
// @Param organization_id query int false "ID организации"
// @Param company_id query int false "ID компании"
// @Success 200 {object} services.CheckActiveCarResponse
// @Router /cars/check-active [get]
func (h *CarHandler) CheckActiveCar(c echo.Context) error {
	var req services.CheckActiveCarRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	resp, err := h.service.CheckActiveCar(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

// GetCarHistory обрабатывает GET /cars/:id/history.
// @Summary Получение истории конкретного автомобиля
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Param id path int true "ID автомобиля"
// @Success 200 {array} services.CarHistoryItemResponse
// @Router /cars/{id}/history [get]
func (h *CarHandler) GetCarHistory(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid car ID")
	}
	items, err := h.service.GetCarHistory(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}

// AddCarHistoryEntry обрабатывает POST /cars/:id/history.
// @Summary Добавление записи в историю автомобиля
// @Tags cars
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID автомобиля"
// @Param body body services.AddCarHistoryRequest true "Данные записи истории"
// @Success 200 {object} map[string]interface{}
// @Router /cars/{id}/history [post]
func (h *CarHandler) AddCarHistoryEntry(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid car ID")
	}
	var req services.AddCarHistoryRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.AddCarHistoryEntry(c.Request().Context(), id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Car history entry added successfully",
	})
}

// GetAllCarsHistory обрабатывает GET /cars/history/all.
// @Summary Получение истории въездов/выездов всех автомобилей
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Success 200 {array} services.AllCarsHistoryItem
// @Router /cars/history/all [get]
func (h *CarHandler) GetAllCarsHistory(c echo.Context) error {
	items, err := h.service.GetAllCarsHistory(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}

// GetCarsCurrentStatus обрабатывает GET /cars/history/current-status.
// @Summary Получение текущего территориального статуса активных автомобилей
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Success 200 {array} services.CarCurrentStatus
// @Router /cars/history/current-status [get]
func (h *CarHandler) GetCarsCurrentStatus(c echo.Context) error {
	items, err := h.service.GetCarsCurrentStatus(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}

// UpdateCarTerritoryStatus обрабатывает PUT /cars/:id/territory-status.
// @Summary Обновление статуса нахождения автомобиля на территории
// @Tags cars
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID автомобиля"
// @Param body body services.UpdateTerritoryStatusRequest true "Новый территориальный статус"
// @Success 200 {object} map[string]interface{}
// @Router /cars/{id}/territory-status [put]
func (h *CarHandler) UpdateCarTerritoryStatus(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid car ID")
	}
	var req services.UpdateTerritoryStatusRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.UpdateCarTerritoryStatus(c.Request().Context(), id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":          true,
		"message":          "Car territory status updated successfully",
		"territory_status": req.TerritoryStatus,
	})
}

// DeactivateCar обрабатывает PUT /cars/:id/deactivate.
// @Summary Деактивация автомобиля
// @Tags cars
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID автомобиля"
// @Param body body services.DeactivateCarRequest true "Данные деактивации"
// @Success 200 {object} map[string]interface{}
// @Router /cars/{id}/deactivate [put]
func (h *CarHandler) DeactivateCar(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid car ID")
	}
	var req services.DeactivateCarRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.DeactivateCar(c.Request().Context(), id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Car deactivated successfully",
	})
}

// ActivateCar обрабатывает PUT /cars/:id/activate.
// @Summary Активация автомобиля (ввод в работу)
// @Tags cars
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID автомобиля"
// @Param body body services.ActivateCarRequest true "Данные активации"
// @Success 200 {object} map[string]interface{}
// @Router /cars/{id}/activate [put]
func (h *CarHandler) ActivateCar(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid car ID")
	}
	var req services.ActivateCarRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.ActivateCar(c.Request().Context(), id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Car activated successfully",
	})
}

// RestoreCar обрабатывает PUT /cars/:id/restore.
// @Summary Восстановление удалённого автомобиля
// @Tags cars
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID автомобиля"
// @Param body body services.RestoreCarRequest true "Данные восстановления"
// @Success 200 {object} map[string]interface{}
// @Router /cars/{id}/restore [put]
func (h *CarHandler) RestoreCar(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid car ID")
	}
	var req services.RestoreCarRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.RestoreCar(c.Request().Context(), id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Car restored successfully",
	})
}

// GetUnifiedCarHistory обрабатывает GET /cars/history/unified.
// @Summary Получение объединённой истории для всех машин с одинаковыми параметрами
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Param car_number query string true "Номер автомобиля"
// @Param car_brand query string true "Марка автомобиля"
// @Param organization_id query int false "ID организации"
// @Param company_id query int false "ID компании"
// @Success 200 {array} services.CarHistoryItemResponse
// @Router /cars/history/unified [get]
func (h *CarHandler) GetUnifiedCarHistory(c echo.Context) error {
	var req services.UnifiedCarHistoryQuery
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	items, err := h.service.GetUnifiedCarHistory(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, items)
}
