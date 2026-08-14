package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

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

	if err := h.service.ForwardApplication(c.Request().Context(), username, id, IsSuperAdmin(c), req); err != nil {
		return err
	}
	return RespondMessage(c, "Application forwarded successfully")
}

// GetForwardMessages godoc
// @Summary      Ветка заявки (пересылки)
// @Description  Возвращает ветку заявки (#967) - все пересылки с автором, получателями и сопроводительным текстом (если он был), хронологически (старые сверху). Видно всем получателям заявки.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array}  services.ForwardMessageItem
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/forward-messages [get]
func (h *ApplicationHandler) GetForwardMessages(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	messages, err := h.service.GetForwardMessages(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, messages)
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

// OverrideBlacklistFlag godoc
// @Summary      Подтвердить пропуск похожего на ЧС элемента
// @Description  Ответственный фиксирует "всё равно пропустить" по конкретному предупреждению (#481), снимая блокировку согласования по нему.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                                    true "ID заявки"
// @Param        request body services.OverrideBlacklistFlagRequest  true "flag_id + комментарий"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/blacklist-overrides [post]
func (h *ApplicationHandler) OverrideBlacklistFlag(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.OverrideBlacklistFlagRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.OverrideBlacklistFlag(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Пропуск подтверждён")
}

// DeleteBlacklistOverride godoc
// @Summary      Отменить подтверждение пропуска
// @Description  Снимает ранее подтверждённый пропуск по флагу (#481), снова блокируя согласование. Право: ответственный по заявке или принимающий.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int true "ID заявки"
// @Param        flag_id query int true "ID предупреждения"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/blacklist-overrides [delete]
func (h *ApplicationHandler) DeleteBlacklistOverride(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	flagID, err := strconv.Atoi(c.QueryParam("flag_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid flag_id")
	}

	if err := h.service.DeleteBlacklistOverride(c.Request().Context(), username, id, flagID); err != nil {
		return err
	}
	return RespondMessage(c, "Подтверждение пропуска отменено")
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

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
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

// WithdrawApplication godoc
// @Summary      Отзыв своей заявки отправителем
// @Description  Отправитель отзывает собственную заявку: статус -> "Отозвана",
// @Description  машины/люди/вложения деактивируются. Обратного пути нет (только дублирование).
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/withdraw [post]
func (h *ApplicationHandler) WithdrawApplication(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	if err := h.service.WithdrawApplication(c.Request().Context(), username, id); err != nil {
		return err
	}
	return RespondMessage(c, "Application withdrawn")
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

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	if err := h.service.UpdateApplicationItemsStatus(c.Request().Context(), id, username); err != nil {
		return err
	}
	return RespondMessage(c, "All items statuses updated successfully")
}

// AssignElementTables godoc
// @Summary      Назначение постов элементам заявки
// @Description  Принимающий добавляет или снимает посты проезда/прохода у машин и сотрудников заявки (#1393).
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                                    true "ID заявки"
// @Param        request body services.AssignElementTablesRequest    true "Элементы, посты и режим"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/elements/tables [put]
func (h *ApplicationHandler) AssignElementTables(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.AssignElementTablesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.AssignElementTables(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Посты обновлены")
}

// RemoveApplicationElements godoc
// @Summary      Удаление людей или машин из поданной заявки
// @Description  Принимающий убирает элемент из заявки: элемент, по которому решение «пропустить» неприемлемо, иначе держал бы всю заявку. Удаление мягкое - строка уходит в корзину, история сохраняется.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                                          true "ID заявки"
// @Param        request body services.RemoveApplicationElementsRequest    true "Тип элементов, идентификаторы и причина"
// @Success      200 {object} map[string]interface{} "success + число убранных"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/elements [delete]
func (h *ApplicationHandler) RemoveApplicationElements(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.RemoveApplicationElementsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	removed, err := h.service.RemoveApplicationElements(c.Request().Context(), username, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]any{"removed": removed})
}

// AssignCarUnloadPlaces godoc
// @Summary      Назначение мест разгрузки машинам заявки
// @Description  Принимающий добавляет или снимает места разгрузки у машин заявки (#1393).
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                                     true "ID заявки"
// @Param        request body services.AssignCarUnloadPlacesRequest    true "Машины, места и режим"
// @Success      200 {object} map[string]interface{} "success + message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/elements/unload-places [put]
func (h *ApplicationHandler) AssignCarUnloadPlaces(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.AssignCarUnloadPlacesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.AssignCarUnloadPlaces(c.Request().Context(), username, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Места разгрузки обновлены")
}
