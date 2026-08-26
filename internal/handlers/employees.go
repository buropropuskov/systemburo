package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// EmployeeHandler -- HTTP-обработчики сотрудников в заявках.
type EmployeeHandler struct {
	service services.EmployeeService
}

// NewEmployeeHandler создаёт новый экземпляр EmployeeHandler.
func NewEmployeeHandler(service services.EmployeeService) *EmployeeHandler {
	return &EmployeeHandler{service: service}
}

// CreateEmployee обрабатывает POST /employees.
// @Summary Создание сотрудника
// @Tags employees
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body services.CreateEmployeeRequest true "Данные сотрудника"
// @Success 200 {object} services.CreateEmployeeResponse
// @Router /employees [post]
func (h *EmployeeHandler) CreateEmployee(c echo.Context) error {
	var req services.CreateEmployeeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := h.service.CreateEmployee(c.Request().Context(), req, GetUserID(c))
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// CreateManualEmployees обрабатывает POST /employees/manual.
// @Summary Ручное добавление сотрудников в таблицу без заявки (#1049)
// @Tags employees
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body services.ManualEmployeeRequest true "Сотрудники, организация/компания и целевая таблица"
// @Success 200 {object} services.ManualEmployeeResponse
// @Failure 400 {object} models.HTTPError
// @Failure 403 {object} models.HTTPError
// @Router /employees/manual [post]
func (h *EmployeeHandler) CreateManualEmployees(c echo.Context) error {
	var req services.ManualEmployeeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	resp, err := h.service.CreateManualEmployees(c.Request().Context(), req, GetUserID(c))
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// UpdateEmployeeTerritoryStatus обрабатывает PUT /employees/:id/territory-status.
// @Summary Обновление статуса нахождения сотрудника на территории
// @Tags employees
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID сотрудника"
// @Param body body services.UpdateTerritoryStatusRequest true "Новый территориальный статус"
// @Success 200 {object} map[string]interface{}
// @Router /employees/{id}/territory-status [put]
func (h *EmployeeHandler) UpdateEmployeeTerritoryStatus(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid employee ID")
	}
	var req services.UpdateTerritoryStatusRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateEmployeeTerritoryStatus(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Employee territory status updated successfully")
}

// GetActiveEmployeesForTable обрабатывает GET /employees/active-for-table/:table_id.
// @Summary Получение активных сотрудников для таблицы
// @Tags employees
// @Security BearerAuth
// @Produce json
// @Param table_id path int true "ID таблицы"
// @Success 200 {array} services.TableEmployeeResponse
// @Router /employees/active-for-table/{table_id} [get]
func (h *EmployeeHandler) GetActiveEmployeesForTable(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("table_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid table ID")
	}
	employees, err := h.service.GetActiveEmployeesForTable(c.Request().Context(), tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, employees)
}

// DeactivateEmployee обрабатывает PUT /employees/:id/deactivate.
// @Summary Деактивация сотрудника
// @Tags employees
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID сотрудника"
// @Param body body services.DeactivateEmployeeRequest true "Данные деактивации"
// @Success 200 {object} map[string]interface{}
// @Router /employees/{id}/deactivate [put]
func (h *EmployeeHandler) DeactivateEmployee(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid employee ID")
	}
	var req services.DeactivateEmployeeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.DeactivateEmployee(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Employee deactivated successfully")
}

// ActivateEmployee обрабатывает PUT /employees/:id/activate.
// @Summary Активация сотрудника (ввод в работу)
// @Tags employees
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID сотрудника"
// @Param body body services.ActivateEmployeeRequest true "Данные активации"
// @Success 200 {object} map[string]interface{}
// @Router /employees/{id}/activate [put]
func (h *EmployeeHandler) ActivateEmployee(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid employee ID")
	}
	var req services.ActivateEmployeeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.ActivateEmployee(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Employee activated successfully")
}

// RestoreEmployee обрабатывает PUT /employees/:id/restore.
// @Summary Восстановление удалённого сотрудника
// @Tags employees
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID сотрудника"
// @Param body body services.RestoreEmployeeRequest true "Данные восстановления"
// @Success 200 {object} map[string]interface{}
// @Router /employees/{id}/restore [put]
func (h *EmployeeHandler) RestoreEmployee(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid employee ID")
	}
	var req services.RestoreEmployeeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.RestoreEmployee(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Employee restored successfully")
}

// BulkMoveTable обрабатывает POST /employees/bulk/move-table.
// @Summary      Групповой перенос сотрудников между таблицами
// @Description  Снимает у выбранных сотрудников привязку к исходной таблице и привязывает к целевым. Требует права admin.
// @Tags         employees
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.EmployeeBulkMoveTableRequest true "ID сотрудников, исходная и целевые таблицы"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /employees/bulk/move-table [post]
func (h *EmployeeHandler) BulkMoveTable(c echo.Context) error {
	var req services.EmployeeBulkMoveTableRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны сотрудники")
	}
	res, err := h.service.BulkMoveTable(c.Request().Context(), req, GetUserID(c))
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkAddTable обрабатывает POST /employees/bulk/add-table.
// @Summary      Групповое добавление сотрудников в таблицы
// @Description  Привязывает выбранных сотрудников к дополнительным таблицам, не отвязывая существующие. Требует права admin.
// @Tags         employees
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.EmployeeBulkAddTableRequest true "ID сотрудников и таблицы"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /employees/bulk/add-table [post]
func (h *EmployeeHandler) BulkAddTable(c echo.Context) error {
	var req services.EmployeeBulkAddTableRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны сотрудники")
	}
	res, err := h.service.BulkAddTable(c.Request().Context(), req, GetUserID(c))
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkUnbindTable обрабатывает POST /employees/bulk/unbind-table.
// @Summary      Групповая отвязка сотрудников от таблицы
// @Description  Снимает у выбранных сотрудников привязку к одной таблице; при снятии последней привязки сотрудник деактивируется. Требует права admin.
// @Tags         employees
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.EmployeeBulkUnbindTableRequest true "ID сотрудников и таблица"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /employees/bulk/unbind-table [post]
func (h *EmployeeHandler) BulkUnbindTable(c echo.Context) error {
	var req services.EmployeeBulkUnbindTableRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны сотрудники")
	}
	res, err := h.service.BulkUnbindTable(c.Request().Context(), req, GetUserID(c))
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}
