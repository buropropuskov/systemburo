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


// GetActiveCarsForTable обрабатывает GET /cars/active-for-table/:table_id.
// @Summary Получение активных машин конкретной таблицы «Проезд»
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Param table_id path int true "ID таблицы"
// @Success 200 {array} services.TableCarResponse
// @Router /cars/active-for-table/{table_id} [get]
func (h *CarHandler) GetActiveCarsForTable(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("table_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid table ID")
	}
	cars, err := h.service.GetActiveCarsForTable(c.Request().Context(), tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, cars)
}

// CreateManualCars обрабатывает POST /cars/manual.
// @Summary Ручное добавление машин в таблицу без заявки (#1049)
// @Tags cars
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body services.ManualCarRequest true "Машины, организация/компания и целевая таблица"
// @Success 200 {object} services.ManualCarResponse
// @Failure 400 {object} models.HTTPError
// @Failure 403 {object} models.HTTPError
// @Router /cars/manual [post]
func (h *CarHandler) CreateManualCars(c echo.Context) error {
	var req services.ManualCarRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	resp, err := h.service.CreateManualCars(c.Request().Context(), req, GetUserID(c))
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}


// GetFactCarsForTable обрабатывает GET /cars/fact-for-table/:table_id.
// @Summary Получение машин «по факту» конкретной таблицы «Проезд»
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Param table_id path int true "ID таблицы"
// @Success 200 {array} services.TableCarResponse
// @Router /cars/fact-for-table/{table_id} [get]
func (h *CarHandler) GetFactCarsForTable(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("table_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid table ID")
	}
	cars, err := h.service.GetFactCarsForTable(c.Request().Context(), tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, cars)
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
	return RespondSuccess(c, places)
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
	return RespondSuccess(c, places)
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
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	resp, err := h.service.CheckActiveCar(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
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
	return RespondSuccess(c, items)
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
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.AddCarHistoryEntry(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Car history entry added successfully")
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
	return RespondSuccess(c, items)
}

// GetCarsHistoryByTable обрабатывает GET /cars/history/table/:table_id.
// @Summary Получение истории въездов/выездов таблицы проходной
// @Tags cars
// @Security BearerAuth
// @Produce json
// @Param table_id path int true "ID таблицы"
// @Success 200 {array} services.AllCarsHistoryItem
// @Router /cars/history/table/{table_id} [get]
func (h *CarHandler) GetCarsHistoryByTable(c echo.Context) error {
	tableID, err := ParseID(c, "table_id")
	if err != nil {
		return err
	}
	items, err := h.service.GetCarsHistoryByTable(c.Request().Context(), tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
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
	return RespondSuccess(c, items)
}

// UpdateCarTerritoryStatus обрабатывает PUT /cars/:id/territory-status.
// @Summary Обновление статуса нахождения автомобиля на территории
// @Tags cars
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID автомобиля"
// @Param body body services.UpdateCarTerritoryStatusRequest true "Новый территориальный статус (+ опц. данные пропуска по факту)"
// @Success 200 {object} map[string]interface{}
// @Router /cars/{id}/territory-status [put]
func (h *CarHandler) UpdateCarTerritoryStatus(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid car ID")
	}
	var req services.UpdateCarTerritoryStatusRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateCarTerritoryStatus(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Car territory status updated successfully")
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
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.DeactivateCar(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Car deactivated successfully")
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
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.ActivateCar(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Car activated successfully")
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
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.RestoreCar(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Car restored successfully")
}

// BulkMoveTable обрабатывает POST /cars/bulk/move-table.
// @Summary Групповой перенос машин между таблицами «Проезд» (#1194)
// @Tags cars
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body services.BulkMoveCarsTableRequest true "ID машин, исходная и целевые таблицы"
// @Success 200 {object} services.BulkOpResult
// @Success 207 {object} services.BulkOpResult "Частичный успех"
// @Failure 400 {object} models.HTTPError
// @Failure 403 {object} models.HTTPError
// @Router /cars/bulk/move-table [post]
func (h *CarHandler) BulkMoveTable(c echo.Context) error {
	var req services.BulkMoveCarsTableRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны машины")
	}
	res, err := h.service.BulkMoveTable(c.Request().Context(), req, GetUserID(c))
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkAddTable обрабатывает POST /cars/bulk/add-table.
// @Summary Групповое добавление машин в таблицы «Проезд» (#1194)
// @Tags cars
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body services.BulkAddCarsTableRequest true "ID машин и целевые таблицы"
// @Success 200 {object} services.BulkOpResult
// @Success 207 {object} services.BulkOpResult "Частичный успех"
// @Failure 400 {object} models.HTTPError
// @Failure 403 {object} models.HTTPError
// @Router /cars/bulk/add-table [post]
func (h *CarHandler) BulkAddTable(c echo.Context) error {
	var req services.BulkAddCarsTableRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны машины")
	}
	res, err := h.service.BulkAddTable(c.Request().Context(), req, GetUserID(c))
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkUnbindTable обрабатывает POST /cars/bulk/unbind-table.
// @Summary Групповое снятие машин с таблицы «Проезд» (#1194)
// @Tags cars
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body services.BulkUnbindCarsTableRequest true "ID машин и таблица"
// @Success 200 {object} services.BulkOpResult
// @Success 207 {object} services.BulkOpResult "Частичный успех"
// @Failure 400 {object} models.HTTPError
// @Failure 403 {object} models.HTTPError
// @Router /cars/bulk/unbind-table [post]
func (h *CarHandler) BulkUnbindTable(c echo.Context) error {
	var req services.BulkUnbindCarsTableRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны машины")
	}
	res, err := h.service.BulkUnbindTable(c.Request().Context(), req, GetUserID(c))
	if err != nil {
		return err
	}
	return respondBulk(c, res)
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
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	items, err := h.service.GetUnifiedCarHistory(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}
