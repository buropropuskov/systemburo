package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// ApplicationHandler HTTP-обработчики для работы с заявками.
type ApplicationHandler struct {
	service  services.ApplicationService
	resolver *services.PermissionResolver
}

// NewApplicationHandler создаёт экземпляр обработчика заявок.
func NewApplicationHandler(service services.ApplicationService, resolver *services.PermissionResolver) *ApplicationHandler {
	return &ApplicationHandler{service: service, resolver: resolver}
}

// GetApplications godoc
// @Summary      Список заявок для Центра заявок
// @Description  Возвращает заявки с фильтрацией. Принимающие видят все, обычные пользователи -- только свои.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        search_query    query string false "Поисковый запрос"
// @Param        organization_id query int    false "ID организации"
// @Param        company_id      query int    false "ID компании"
// @Param        confirmation    query string false "Статус согласования"
// @Param        status          query string false "Статус заявки"
// @Param        date_from       query string false "Дата от (YYYY-MM-DD)"
// @Param        date_to         query string false "Дата до (YYYY-MM-DD)"
// @Success      200 {array}  services.ApplicationWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications [get]
func (h *ApplicationHandler) GetApplications(c echo.Context) error {
	username := c.Get("username").(string)

	var filter services.ApplicationFilter
	if err := c.Bind(&filter); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	if archiveStr := c.QueryParam("archive"); archiveStr != "" {
		archive := archiveStr == "true"
		filter.Archive = &archive
	}
	if activeTodayStr := c.QueryParam("active_today"); activeTodayStr != "" {
		activeToday := activeTodayStr == "true"
		filter.ActiveToday = &activeToday
	}

	// Legacy mode: if per_page not specified, return all (backward compat)
	if c.QueryParam("per_page") == "" {
		apps, err := h.service.GetApplications(c.Request().Context(), username, filter)
		if err != nil {
			return err
		}
		return RespondSuccess(c, apps)
	}

	var params models.PaginationParams
	if err := c.Bind(&params); err != nil {
		params = models.PaginationParams{}
	}
	params.Normalize()

	data, total, err := h.service.GetApplicationsPaginated(
		c.Request().Context(), username, filter, params.Page, params.PerPage,
	)
	if err != nil {
		return err
	}
	return RespondPaginated(c, data, models.PaginationMeta{
		Total: total, Page: params.Page, PerPage: params.PerPage,
	})
}

// GetUserApplications godoc
// @Summary      Заявки текущего пользователя
// @Description  Возвращает все заявки для текущего пользователя с фильтрацией.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        search_query query string false "Поисковый запрос"
// @Param        confirmation query string false "Статус согласования"
// @Param        status       query string false "Статус заявки"
// @Param        date_from    query string false "Дата от (YYYY-MM-DD)"
// @Param        date_to      query string false "Дата до (YYYY-MM-DD)"
// @Success      200 {array}  services.ApplicationWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/user [get]
func (h *ApplicationHandler) GetUserApplications(c echo.Context) error {
	username := c.Get("username").(string)

	var filter services.ApplicationFilter
	if err := c.Bind(&filter); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	apps, err := h.service.GetUserApplications(c.Request().Context(), username, filter)
	if err != nil {
		return err
	}
	return RespondSuccess(c, apps)
}

// GetApplicationByID godoc
// @Summary      Получение заявки по ID
// @Description  Возвращает заявку с обновлением статуса при первом прочтении не отправителем.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id} [get]
func (h *ApplicationHandler) GetApplicationByID(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	app, err := h.service.GetApplicationByID(c.Request().Context(), username, id)
	if err != nil {
		return err
	}

	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	return RespondSuccess(c, app)
}

// GetApplicationDetails godoc
// @Summary      Расширенная информация о заявке
// @Description  Возвращает детальную информацию о заявке с ответственными пользователями.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/details [get]
func (h *ApplicationHandler) GetApplicationDetails(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	details, err := h.service.GetApplicationDetails(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, details)
}

// CreateApplication godoc
// @Summary      Создание простой заявки
// @Description  Создаёт заявку с автоматическим назначением ответственных из организации/компании.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.ApplicationCreateRequest true "Данные заявки"
// @Success      200 {object} services.ApplicationCreateResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications [post]
func (h *ApplicationHandler) CreateApplication(c echo.Context) error {
	username := c.Get("username").(string)

	var req services.ApplicationCreateRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	resp, err := h.service.CreateApplication(c.Request().Context(), username, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// SubmitCompleteApplication godoc
// @Summary      Создание полной заявки с вложениями
// @Description  Создаёт заявку вместе с вложениями (машины, сотрудники, ТМЦ) в одной транзакции.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.CompleteApplicationRequest true "Полные данные заявки"
// @Success      200 {object} services.CompleteApplicationResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/submit-complete-application [post]
func (h *ApplicationHandler) SubmitCompleteApplication(c echo.Context) error {
	username := c.Get("username").(string)

	var req services.CompleteApplicationRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	resp, err := h.service.SubmitCompleteApplication(c.Request().Context(), username, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// UpdateApplication godoc
// @Summary      Обновление заявки
// @Description  Обновляет confirmation, status и/или responsible_comment заявки.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                              true  "ID заявки"
// @Param        request body services.ApplicationUpdateRequest true  "Обновляемые поля"
// @Success      200 {object} services.ApplicationUpdateResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id} [put]
func (h *ApplicationHandler) UpdateApplication(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	var req services.ApplicationUpdateRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	resp, err := h.service.UpdateApplication(c.Request().Context(), username, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// GetUnreadCount godoc
// @Summary      Количество непрочитанных заявок
// @Description  Возвращает количество непрочитанных активных заявок для текущего пользователя.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.UnreadCountResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/unread-count [get]
func (h *ApplicationHandler) GetUnreadCount(c echo.Context) error {
	username := c.Get("username").(string)

	resp, err := h.service.GetUnreadCount(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}
