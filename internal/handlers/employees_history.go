package handlers

import (
	"net/http"

	"systemburo/internal/models"
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
// @Summary Получение страницы истории входов/выходов всех сотрудников
// @Tags employees-history
// @Security BearerAuth
// @Produce json
// @Param user_id     query int    false "Кто отметил проход"
// @Param employee_id query int    false "Конкретный сотрудник"
// @Param date_from   query string false "Начало периода, YYYY-MM-DD (московские сутки включительно)"
// @Param date_to     query string false "Конец периода, YYYY-MM-DD (московские сутки включительно)"
// @Param search      query string false "Поиск по ФИО сотрудника, организации, компании и ФИО отметившего"
// @Param order       query string false "Порядок по времени отметки" Enums(asc, desc) default(desc)
// @Param page        query int    false "Страница" default(1)
// @Param per_page    query int    false "Записей на странице (максимум 200)" default(50)
// @Success 200 {object} Response
// @Router /employees/history/all [get]
func (h *EmployeesHistoryHandler) GetAll(c echo.Context) error {
	q, err := bindPassageHistoryQuery(c)
	if err != nil {
		return err
	}
	items, total, err := h.service.GetAll(c.Request().Context(), q)
	if err != nil {
		return err
	}
	return RespondPaginated(c, items, models.PaginationMeta{Total: total, Page: q.Page, PerPage: q.PerPage})
}

// GetFilterOptions обрабатывает GET /employees/history/filter-options.
// @Summary Значения выпадающих списков журнала проходов людей
// @Tags employees-history
// @Security BearerAuth
// @Produce json
// @Param table_id query int false "Сузить до таблицы проходной"
// @Success 200 {object} Response
// @Router /employees/history/filter-options [get]
func (h *EmployeesHistoryHandler) GetFilterOptions(c echo.Context) error {
	tableID, err := optionalIntQuery(c, "table_id")
	if err != nil {
		return err
	}
	options, err := h.service.GetFilterOptions(c.Request().Context(), tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, options)
}

// GetCurrentStatus обрабатывает GET /employees/history/current-status.
// @Summary Получение текущего территориального статуса сотрудников
// @Tags employees-history
// @Security BearerAuth
// @Produce json
// @Success 200 {array} services.EmployeeCurrentStatus
// @Router /employees/history/current-status [get]
func (h *EmployeesHistoryHandler) GetCurrentStatus(c echo.Context) error {
	items, err := h.service.GetCurrentStatus(c.Request().Context(), GetUserID(c))
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
// @Param table_id    path  int    true  "ID таблицы"
// @Param user_id     query int    false "Кто отметил проход"
// @Param employee_id query int    false "Конкретный сотрудник"
// @Param date_from   query string false "Начало периода, YYYY-MM-DD (московские сутки включительно)"
// @Param date_to     query string false "Конец периода, YYYY-MM-DD (московские сутки включительно)"
// @Param search      query string false "Поиск по ФИО сотрудника, организации, компании и ФИО отметившего"
// @Param order       query string false "Порядок по времени отметки" Enums(asc, desc) default(desc)
// @Param page        query int    false "Страница" default(1)
// @Param per_page    query int    false "Записей на странице (максимум 200)" default(50)
// @Success 200 {object} Response
// @Router /employees/history/table/{table_id} [get]
func (h *EmployeesHistoryHandler) GetByTable(c echo.Context) error {
	tableID, err := ParseID(c, "table_id")
	if err != nil {
		return err
	}
	q, err := bindPassageHistoryQuery(c)
	if err != nil {
		return err
	}
	items, total, err := h.service.GetByTable(c.Request().Context(), tableID, q)
	if err != nil {
		return err
	}
	return RespondPaginated(c, items, models.PaginationMeta{Total: total, Page: q.Page, PerPage: q.PerPage})
}
