package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
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
// @Description  Возвращает список уникальных сотрудников с фильтрацией по владельцу. Без per_page -
// @Description  полный массив (legacy). С per_page - пагинация + серверный поиск search_query
// @Description  (#1158, срез 3, для EmployeeView).
// @Tags         unique-employees
// @Produce      json
// @Security     BearerAuth
// @Param        filter_type  query string false "Тип фильтра: user, organization, company, all, all_system"
// @Param        search_query query string false "Поисковый запрос (ФИО/должность/организация/компания/гражданство)"
// @Param        page         query int    false "Номер страницы (с per_page)"
// @Param        per_page     query int    false "Размер страницы (<=100); наличие включает пагинацию"
// @Success      200 {array} services.UniqueEmployeeWithRelations
// @Failure      401 {object} models.HTTPError
// @Router       /unique-employees [get]
func (h *UniqueEmployeeHandler) GetAll(c echo.Context) error {
	username := c.Get("username").(string)
	filterType := c.QueryParam("filter_type")
	if filterType == "" {
		filterType = "user"
	}

	// Legacy mode: без per_page отдаём полный массив без поиска, как раньше.
	if c.QueryParam("per_page") == "" {
		employees, err := h.service.GetAll(c.Request().Context(), username, filterType)
		if err != nil {
			return err
		}
		return RespondSuccess(c, employees)
	}

	var params models.PaginationParams
	if err := c.Bind(&params); err != nil {
		params = models.PaginationParams{}
	}
	params.Normalize()

	searchQuery := c.QueryParam("search_query")
	employees, total, err := h.service.GetAllPaginated(c.Request().Context(), username, filterType, searchQuery, params.Page, params.PerPage)
	if err != nil {
		return err
	}
	return RespondPaginated(c, employees, models.PaginationMeta{
		Total: total, Page: params.Page, PerPage: params.PerPage,
	})
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
	if err := BindAndValidate(c, &req); err != nil {
		return err
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
	if err := BindAndValidate(c, &req); err != nil {
		return err
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

// GetHistory godoc
// @Summary      История изменений мастер-сотрудника
// @Description  Возвращает аудит изменений мастер-записи сотрудника (data_changed)
// @Tags         unique-employees
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID сотрудника"
// @Success      200 {array} services.UniqueEmployeeHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError "Нет прав"
// @Failure      404 {object} models.HTTPError "Не найден"
// @Router       /unique-employees/{id}/history [get]
func (h *UniqueEmployeeHandler) GetHistory(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	items, err := h.service.GetHistory(c.Request().Context(), username, id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// GetRegistryLog godoc
// @Summary      Журнал реестра сотрудников
// @Description  Все события реестра: создание, правка полей, удаление - с автором и
// @Description  временем. Единственный способ узнать, кем и когда удалена запись: у
// @Description  исчезнувшей строки истории по id больше нет. Доступен администратору.
// @Tags         unique-employees
// @Produce      json
// @Security     BearerAuth
// @Param        limit query int false "Сколько записей вернуть (по умолчанию и максимум 500)"
// @Success      200 {array} services.UniqueEmployeeHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError "Не администратор"
// @Router       /unique-employees/history [get]
func (h *UniqueEmployeeHandler) GetRegistryLog(c echo.Context) error {
	username := c.Get("username").(string)
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	items, err := h.service.GetRegistryLog(c.Request().Context(), username, limit)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// Lookup godoc
// @Summary      Найти сотрудника по ФИО
// @Description  Поиск сотрудника (LOWER/TRIM) для открытия карточки со страницы ЧС. 404 если нет.
// @Tags         unique-employees
// @Produce      json
// @Security     BearerAuth
// @Param        last_name query string true "Фамилия"
// @Param        first_name query string true "Имя"
// @Param        middle_name query string false "Отчество"
// @Success      200 {object} services.UniqueEmployeeWithRelations
// @Failure      400 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /unique-employees/lookup [get]
func (h *UniqueEmployeeHandler) Lookup(c echo.Context) error {
	lastName := c.QueryParam("last_name")
	firstName := c.QueryParam("first_name")
	if lastName == "" || firstName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "last_name и first_name обязательны")
	}
	emp, err := h.service.LookupByFIO(c.Request().Context(), lastName, firstName, c.QueryParam("middle_name"))
	if err != nil {
		return err
	}
	if emp == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Сотрудник не найден")
	}
	return RespondSuccess(c, emp)
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
