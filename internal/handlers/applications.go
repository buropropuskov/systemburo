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
	service services.ApplicationService
}

// NewApplicationHandler создаёт экземпляр обработчика заявок.
func NewApplicationHandler(service services.ApplicationService) *ApplicationHandler {
	return &ApplicationHandler{service: service}
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
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if archiveStr := c.QueryParam("archive"); archiveStr != "" {
		archive := archiveStr == "true"
		filter.Archive = &archive
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
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
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

// ForwardApplication godoc
// @Summary      Пересылка заявки
// @Description  Назначает ответственных и просматривающих для заявки.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                              true "ID заявки"
// @Param        request body services.ForwardApplicationRequest true "Список пользователей"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/forward [post]
func (h *ApplicationHandler) ForwardApplication(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.ForwardApplicationRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.ForwardApplication(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Application forwarded successfully")
}

// ApproveApplicationByUser godoc
// @Summary      Согласование заявки пользователем
// @Description  Пользователь голосует за согласование или отказ заявки.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                          true "ID заявки"
// @Param        request body services.UserApprovalRequest  true "Голос: approved или rejected"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/approve [post]
func (h *ApplicationHandler) ApproveApplicationByUser(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.UserApprovalRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.ApproveApplicationByUser(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Approval status updated successfully")
}

// CheckApprovalStatus godoc
// @Summary      Проверка статуса согласования
// @Description  Возвращает текущие confirmation и status заявки.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {object} services.ApprovalStatusResponse
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/check-approval-status [get]
func (h *ApplicationHandler) CheckApprovalStatus(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	resp, err := h.service.CheckApprovalStatus(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// TakeApplicationToWork godoc
// @Summary      Принятие заявки в работу
// @Description  Принимающий пользователь принимает (accept) или отклоняет (reject) заявку.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                         true "ID заявки"
// @Param        request body services.TakeToWorkRequest   true "Действие: accept или reject"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/take-to-work [post]
func (h *ApplicationHandler) TakeApplicationToWork(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.TakeToWorkRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.TakeApplicationToWork(c.Request().Context(), username, id, req); err != nil {
		return err
	}

	msg := "Application taken to work"
	if req.Action == "reject" {
		msg = "Application rejected"
	}
	return RespondMessage(c, msg)
}

// RevokeApplicationFromWork godoc
// @Summary      Отзыв заявки из работы
// @Description  Принимающий возвращает заявку в статус "В обработке", деактивируя все элементы.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                             true "ID заявки"
// @Param        request body services.RevokeFromWorkRequest   true "Комментарий"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/revoke-from-work [post]
func (h *ApplicationHandler) RevokeApplicationFromWork(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.RevokeFromWorkRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.RevokeApplicationFromWork(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Application revoked from work")
}

// RestoreApplicationToWork godoc
// @Summary      Возврат заявки в обработку
// @Description  Принимающий возвращает заявку в статус "В обработке" для повторного рассмотрения.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                             true "ID заявки"
// @Param        request body services.RevokeFromWorkRequest   true "Комментарий"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/restore-to-work [post]
func (h *ApplicationHandler) RestoreApplicationToWork(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.RevokeFromWorkRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.RestoreApplicationToWork(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Application restored, ready to take to work")
}

// GetApplicationResponsibleUsers godoc
// @Summary      Ответственные пользователи заявки
// @Description  Возвращает список ответственных с информацией о согласовании.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array}  services.ResponsibleUserInfo
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/responsible-users [get]
func (h *ApplicationHandler) GetApplicationResponsibleUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	users, err := h.service.GetApplicationResponsibleUsers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, users)
}

// GetApplicationHistory godoc
// @Summary      История заявки
// @Description  Возвращает записи истории заявки в обратном хронологическом порядке.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array}  services.ApplicationHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/history [get]
func (h *ApplicationHandler) GetApplicationHistory(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	history, err := h.service.GetApplicationHistory(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, history)
}

// AddHistoryEntry godoc
// @Summary      Добавление записи в историю
// @Description  Ручное добавление записи в историю заявки.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.AddHistoryEntryRequest true "Запись истории"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/history [post]
func (h *ApplicationHandler) AddHistoryEntry(c echo.Context) error {
	var req services.AddHistoryEntryRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.AddHistoryEntry(c.Request().Context(), req); err != nil {
		return err
	}
	return RespondMessage(c, "History entry added successfully")
}

// RevokeApproval godoc
// @Summary      Отзыв согласования
// @Description  Пользователь отзывает ранее данное согласование/отказ.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                             true "ID заявки"
// @Param        request body services.RevokeApprovalRequest   true "Комментарий"
// @Success      200 {object} services.RevokeApprovalResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/revoke-approval [post]
func (h *ApplicationHandler) RevokeApproval(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.RevokeApprovalRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	resp, err := h.service.RevokeApproval(c.Request().Context(), username, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// GetApplicationViewers godoc
// @Summary      Просматривающие заявки
// @Description  Возвращает список просматривающих с информацией о пользователе.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array}  services.ViewerWithUser
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/viewers [get]
func (h *ApplicationHandler) GetApplicationViewers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	viewers, err := h.service.GetApplicationViewers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, viewers)
}

// GetApplicationAttachments godoc
// @Summary      Вложения заявки
// @Description  Возвращает вложения заявки с информацией из шаблонов (unique_attachments).
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array}  services.AttachmentInfo
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/attachments [get]
func (h *ApplicationHandler) GetApplicationAttachments(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	attachments, err := h.service.GetApplicationAttachments(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, attachments)
}

// GetAttachmentCars godoc
// @Summary      Автомобили вложения
// @Description  Возвращает автомобили вложения с привязанными местами разгрузки.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID вложения"
// @Success      200 {array}  services.CarWithPlaces
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /attachments/{id}/cars [get]
func (h *ApplicationHandler) GetAttachmentCars(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}

	cars, err := h.service.GetAttachmentCars(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, cars)
}

// GetAttachmentEmployees godoc
// @Summary      Сотрудники вложения
// @Description  Возвращает сотрудников вложения с привязанными таблицами.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID вложения"
// @Success      200 {array}  services.EmployeeWithTables
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /attachments/{id}/employees [get]
func (h *ApplicationHandler) GetAttachmentEmployees(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}

	employees, err := h.service.GetAttachmentEmployees(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, employees)
}

// GetAttachmentItems godoc
// @Summary      ТМЦ вложения
// @Description  Возвращает товарно-материальные ценности вложения.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID вложения"
// @Success      200 {array}  services.ItemInfo
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /attachments/{id}/items [get]
func (h *ApplicationHandler) GetAttachmentItems(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}

	items, err := h.service.GetAttachmentItems(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// MarkAsRead godoc
// @Summary      Отметить заявку прочитанной
// @Description  Текущий пользователь отмечает заявку как прочитанную (идемпотентно).
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/read [post]
func (h *ApplicationHandler) MarkAsRead(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	if err := h.service.MarkAsRead(c.Request().Context(), id, username); err != nil {
		return err
	}
	return RespondMessage(c, "Application marked as read")
}

// GetReads godoc
// @Summary      Прочтения заявки
// @Description  Возвращает список пользователей, прочитавших заявку, с датами.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array}  models.ApplicationReadResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/reads [get]
func (h *ApplicationHandler) GetReads(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	reads, err := h.service.GetReads(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, reads)
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

// UpdateApplicationItemsStatus godoc
// @Summary      Обновление статусов элементов заявки
// @Description  Активирует все машины и сотрудников во вложениях заявки (status = 1).
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/update-items-status [post]
func (h *ApplicationHandler) UpdateApplicationItemsStatus(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	if err := h.service.UpdateApplicationItemsStatus(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondMessage(c, "All items statuses updated successfully")
}
