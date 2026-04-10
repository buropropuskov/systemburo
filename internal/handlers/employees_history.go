package handlers

import (
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// EmployeesHistoryHandler -- HTTP-обработчики истории сотрудников.
type EmployeesHistoryHandler struct {
	service services.EmployeesHistoryService
}

// NewEmployeesHistoryHandler создаёт новый экземпляр EmployeesHistoryHandler.
func NewEmployeesHistoryHandler(service services.EmployeesHistoryService) *EmployeesHistoryHandler {
	return &EmployeesHistoryHandler{service: service}
}

// GetByEmployee обрабатывает GET /employees/:id/history.
// @Summary Получение истории конкретного сотрудника
// @Tags employees-history
// @Security BearerAuth
// @Produce json
// @Param id path int true "ID сотрудника"
// @Success 200 {array} services.EmployeeHistoryItem
// @Router /employees/{id}/history [get]
func (h *EmployeesHistoryHandler) GetByEmployee(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	items, err := h.service.GetByEmployee(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// GetUnified обрабатывает GET /employees/history/unified.
// @Summary Получение объединённой истории по ФИО
// @Tags employees-history
// @Security BearerAuth
// @Produce json
// @Param last_name query string true "Фамилия"
// @Param first_name query string true "Имя"
// @Param middle_name query string false "Отчество"
// @Success 200 {array} services.EmployeeHistoryItem
// @Router /employees/history/unified [get]
func (h *EmployeesHistoryHandler) GetUnified(c echo.Context) error {
	lastName := c.QueryParam("last_name")
	firstName := c.QueryParam("first_name")
	if lastName == "" || firstName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "last_name and first_name are required")
	}
	middleName := c.QueryParam("middle_name")

	items, err := h.service.GetUnified(c.Request().Context(), lastName, firstName, middleName)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// GetAll обрабатывает GET /employees/history/all.
// @Summary Получение истории въездов/выходов всех сотрудников
// @Tags employees-history
// @Security BearerAuth
// @Produce json
// @Success 200 {array} services.EmployeeHistoryItem
// @Router /employees/history/all [get]
func (h *EmployeesHistoryHandler) GetAll(c echo.Context) error {
	items, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// GetCurrentStatus обрабатывает GET /employees/history/current-status.
// @Summary Получение текущего территориального статуса сотрудников
// @Tags employees-history
// @Security BearerAuth
// @Produce json
// @Success 200 {array} services.EmployeeCurrentStatus
// @Router /employees/history/current-status [get]
func (h *EmployeesHistoryHandler) GetCurrentStatus(c echo.Context) error {
	items, err := h.service.GetCurrentStatus(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// GetByTable обрабатывает GET /employees/history/table/:table_id.
// @Summary Получение истории сотрудников для конкретной таблицы
// @Tags employees-history
// @Security BearerAuth
// @Produce json
// @Param table_id path int true "ID таблицы"
// @Success 200 {array} services.EmployeeHistoryItem
// @Router /employees/history/table/{table_id} [get]
func (h *EmployeesHistoryHandler) GetByTable(c echo.Context) error {
	tableID, err := ParseID(c, "table_id")
	if err != nil {
		return err
	}
	items, err := h.service.GetByTable(c.Request().Context(), tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}
