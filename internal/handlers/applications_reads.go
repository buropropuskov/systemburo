package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

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

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	users, err := h.service.GetApplicationResponsibleUsers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, users)
}

// GetApplicationParticipants godoc
// @Summary      Участники заявки
// @Description  Возвращает всех участников заявки одним списком: отправителя, принявшего
// @Description  в работу, согласующих, ответственных и читателей. На каждого - роли
// @Description  машинными ключами, должность, организация, компания и контакты.
// @Description  Один человек - одна запись, даже если ролей у него несколько.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array}  services.ApplicationParticipant
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/participants [get]
func (h *ApplicationHandler) GetApplicationParticipants(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	participants, err := h.service.GetApplicationParticipants(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, participants)
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

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
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

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), req.ApplicationID, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	userID := c.Get("user_id").(int)
	req.UserID = userID

	if err := h.service.AddHistoryEntry(c.Request().Context(), req); err != nil {
		return err
	}
	return RespondMessage(c, "History entry added successfully")
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

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
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

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	attachments, err := h.service.GetApplicationAttachments(c.Request().Context(), id, viewerUserID(c))
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

	appID, err := h.service.GetApplicationIDByAttachment(c.Request().Context(), id)
	if err != nil {
		return err
	}
	// Вложение-сирота ручного добавления (#1049, application_id NULL -> appID 0) не
	// принадлежит заявке: app-detail путь к нему закрыт для всех, включая super и
	// принимающего (иначе они байпасят CanAccessApplication на appID 0). Ручные машины
	// доступны только через таблицы (/cars/active-for-table), не через вложение заявки.
	if appID == 0 {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), appID, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	canView, err := h.service.CanViewAttachment(c.Request().Context(), appID, id, viewerUserID(c))
	if err != nil {
		return err
	}
	if !canView {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	// Карточка заявки: сюда доходят автор, согласующие, принимающий и супер-админ
	// (CanAccessApplication выше). Им состав нужен целиком, вместе с непринятым
	// дополнением - иначе автор не увидит, что его добавка ушла на согласование,
	// а согласующему нечего будет решать (#1685).
	cars, err := h.service.GetAttachmentCars(c.Request().Context(), id, services.SupplementScopeAll)
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

	appID, err := h.service.GetApplicationIDByAttachment(c.Request().Context(), id)
	if err != nil {
		return err
	}
	// Вложение-сирота ручного добавления (#1049, application_id NULL -> appID 0) не
	// принадлежит заявке: app-detail путь к нему закрыт для всех, включая super и
	// принимающего (иначе они байпасят CanAccessApplication на appID 0). Ручные машины
	// доступны только через таблицы (/cars/active-for-table), не через вложение заявки.
	if appID == 0 {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), appID, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	canView, err := h.service.CanViewAttachment(c.Request().Context(), appID, id, viewerUserID(c))
	if err != nil {
		return err
	}
	if !canView {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	employees, err := h.service.GetAttachmentEmployees(c.Request().Context(), id, services.SupplementScopeAll)
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

	appID, err := h.service.GetApplicationIDByAttachment(c.Request().Context(), id)
	if err != nil {
		return err
	}
	// Вложение-сирота ручного добавления (#1049, application_id NULL -> appID 0) не
	// принадлежит заявке: app-detail путь к нему закрыт для всех, включая super и
	// принимающего (иначе они байпасят CanAccessApplication на appID 0). Ручные машины
	// доступны только через таблицы (/cars/active-for-table), не через вложение заявки.
	if appID == 0 {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), appID, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	canView, err := h.service.CanViewAttachment(c.Request().Context(), appID, id, viewerUserID(c))
	if err != nil {
		return err
	}
	if !canView {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	items, err := h.service.GetAttachmentItems(c.Request().Context(), id, services.SupplementScopeAll)
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

	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
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

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	reads, err := h.service.GetReads(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, reads)
}
