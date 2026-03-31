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
