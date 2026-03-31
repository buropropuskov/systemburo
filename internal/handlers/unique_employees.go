package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// UniqueEmployeeHandler -- HTTP-обработчики уникальных сотрудников.
type UniqueEmployeeHandler struct {
	service services.UniqueEmployeeService
}

// NewUniqueEmployeeHandler создаёт новый экземпляр обработчика уникальных сотрудников.
func NewUniqueEmployeeHandler(service services.UniqueEmployeeService) *UniqueEmployeeHandler {
	return &UniqueEmployeeHandler{service: service}
}

// GetAll godoc
// @Summary      Получение уникальных сотрудников
// @Description  Возвращает список уникальных сотрудников с фильтрацией по владельцу
// @Tags         unique-employees
// @Produce      json
// @Security     BearerAuth
// @Param        filter_type query string false "Тип фильтра: user, organization, company, all"
// @Success      200 {array} services.UniqueEmployeeWithRelations
// @Failure      401 {object} models.HTTPError
// @Router       /unique-employees [get]
func (h *UniqueEmployeeHandler) GetAll(c echo.Context) error {
	username := c.Get("username").(string)
	filterType := c.QueryParam("filter_type")
	if filterType == "" {
		filterType = "user"
	}

	employees, err := h.service.GetAll(c.Request().Context(), username, filterType)
	if err != nil {
		return err
	}
	return RespondSuccess(c, employees)
}

// Create godoc
// @Summary      Создание уникального сотрудника
// @Description  Создаёт нового уникального сотрудника с проверкой уникальности паспортных данных
// @Tags         unique-employees
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.NewUniqueEmployeeRequest true "Данные сотрудника"
// @Success      200 {object} services.UniqueEmployeeResponse
// @Failure      400 {object} models.HTTPError "Дубликат"
// @Failure      401 {object} models.HTTPError
// @Router       /unique-employees [post]
func (h *UniqueEmployeeHandler) Create(c echo.Context) error {
	username := c.Get("username").(string)
	var req services.NewUniqueEmployeeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	employee, err := h.service.Create(c.Request().Context(), username, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, employee)
}

// Update godoc
// @Summary      Обновление уникального сотрудника
// @Description  Обновляет данные уникального сотрудника по ID
// @Tags         unique-employees
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID сотрудника"
// @Param        request body services.NewUniqueEmployeeRequest true "Данные сотрудника"
// @Success      200 {object} services.UniqueEmployeeResponse
// @Failure      400 {object} models.HTTPError "Дубликат"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError "Нет прав"
// @Failure      404 {object} models.HTTPError "Не найден"
// @Router       /unique-employees/{id} [put]
func (h *UniqueEmployeeHandler) Update(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	var req services.NewUniqueEmployeeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	employee, err := h.service.Update(c.Request().Context(), username, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, employee)
}

// Delete godoc
// @Summary      Удаление уникального сотрудника
// @Description  Удаляет уникального сотрудника по ID с проверкой прав
// @Tags         unique-employees
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID сотрудника"
// @Success      200 {object} map[string]string "message: Employee deleted successfully"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError "Нет прав"
// @Failure      404 {object} models.HTTPError "Не найден"
// @Router       /unique-employees/{id} [delete]
func (h *UniqueEmployeeHandler) Delete(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	if err := h.service.Delete(c.Request().Context(), username, id); err != nil {
		return err
	}
	return RespondMessage(c, "Employee deleted successfully")
}

// GetOwnershipInfo godoc
// @Summary      Информация о владельце для сотрудников
// @Description  Возвращает данные о привязке пользователя к организации/компании
// @Tags         unique-employees
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} services.EmployeeOwnerInfo
// @Failure      401 {object} models.HTTPError
// @Router       /unique-employees/ownership-info [get]
func (h *UniqueEmployeeHandler) GetOwnershipInfo(c echo.Context) error {
	username := c.Get("username").(string)
	info, err := h.service.GetOwnerInfo(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, info)
}
