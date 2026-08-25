package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

type ApproverHandler struct {
	service services.ApproverService
}

func NewApproverHandler(service services.ApproverService) *ApproverHandler {
	return &ApproverHandler{service: service}
}

// GetAll godoc
// @Summary      Список утверждающих заявок
// @Tags         application-approvers
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.ApplicationApproverWithUser
// @Failure      403 {object} models.HTTPError
// @Router       /application-approvers [get]
func (h *ApproverHandler) GetAll(c echo.Context) error {
	result, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// GetRecipients godoc
// @Summary      Принимающие для строки получателей заявки
// @Description  Отдаёт только отображаемые имена: маску, если она задана администратором, иначе ФИО. Доступно любому авторизованному работнику.
// @Tags         application-approvers
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.ApplicationRecipient
// @Router       /application-approvers/recipients [get]
func (h *ApproverHandler) GetRecipients(c echo.Context) error {
	result, err := h.service.GetRecipients(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// GetAvailableUsers godoc
// @Summary      Пользователи, доступные для назначения утверждающими
// @Tags         application-approvers
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.AvailableApproverUser
// @Failure      403 {object} models.HTTPError
// @Router       /application-approvers/available-users [get]
func (h *ApproverHandler) GetAvailableUsers(c echo.Context) error {
	result, err := h.service.GetAvailableUsers(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// Create godoc
// @Summary      Добавить утверждающего
// @Tags         application-approvers
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        body body models.CreateApproverRequest true "User ID"
// @Success      201 {object} map[string]string
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /application-approvers [post]
func (h *ApproverHandler) Create(c echo.Context) error {
	username := c.Get("username").(string)

	var req models.CreateApproverRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.service.Create(c.Request().Context(), req.UserID, username); err != nil {
		return err
	}
	return RespondCreated(c, map[string]string{
		"message": "Approver added successfully",
	})
}

// Update godoc
// @Summary      Задать/снять маску отображаемого имени принимающего
// @Tags         application-approvers
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Approver ID"
// @Param        body body models.UpdateApproverRequest true "Display name (null/empty снимает маску)"
// @Success      200 {object} map[string]string
// @Failure      400 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /application-approvers/{id} [patch]
func (h *ApproverHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateApproverRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	actorUsername, _ := c.Get("username").(string)
	if err := h.service.Update(c.Request().Context(), id, req.DisplayName, actorUsername); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]string{
		"message": "Approver updated successfully",
	})
}

// Delete godoc
// @Summary      Удалить утверждающего
// @Tags         application-approvers
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "Approver ID"
// @Success      200 {object} map[string]string
// @Failure      404 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /application-approvers/{id} [delete]
func (h *ApproverHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	actorUsername, _ := c.Get("username").(string)
	if err := h.service.Delete(c.Request().Context(), id, actorUsername); err != nil {
		return err
	}
	return RespondSuccess(c, map[string]string{
		"message": "Approver deleted successfully",
	})
}

// GetHistory godoc
// @Summary      Журнал принимающих заявки
// @Description  Глобальный аудит-лог: кто и когда был добавлен или удалён из принимающих
// @Tags         application-approvers
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.ApplicationApproverHistoryItem
// @Failure      403 {object} models.HTTPError
// @Router       /application-approvers/history [get]
func (h *ApproverHandler) GetHistory(c echo.Context) error {
	history, err := h.service.GetHistory(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, history)
}

// IsApprover godoc
// @Summary      Роли текущего пользователя в согласовании заявок
// @Description  Возвращает только ответ про себя: is_approver - принимающий,
// @Description  is_reviewer - согласующий хоть в одной организации или компании.
// @Description  Полный состав принимающих отдаёт GET /application-approvers, он
// @Description  закрыт правом администратора.
// @Tags         application-approvers
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]bool
// @Router       /application-approvers/me [get]
func (h *ApproverHandler) IsApprover(c echo.Context) error {
	username := GetUsername(c)
	ctx := c.Request().Context()
	isApprover, err := h.service.IsApprover(ctx, username)
	if err != nil {
		return err
	}
	isReviewer, err := h.service.IsReviewer(ctx, username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]bool{"is_approver": isApprover, "is_reviewer": isReviewer})
}
