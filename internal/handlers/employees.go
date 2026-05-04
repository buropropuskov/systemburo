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
	resp, err := h.service.CreateEmployee(c.Request().Context(), req)
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
