package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// NotificationHandler -- HTTP-обработчики уведомлений.
type NotificationHandler struct {
	service services.NotificationService
}

// NewNotificationHandler создаёт новый экземпляр NotificationHandler.
func NewNotificationHandler(service services.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: service}
}

// GetNotifications godoc
// @Summary      Получение уведомлений текущего пользователя
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.Notification
// @Failure      401 {object} models.HTTPError
// @Router       /notifications [get]
func (h *NotificationHandler) GetNotifications(c echo.Context) error {
	userID := GetUserID(c)
	notifications, err := h.service.GetByUserID(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, notifications)
}

// MarkRead godoc
// @Summary      Отметить уведомление как прочитанное/непрочитанное
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID уведомления"
// @Param        request body models.MarkNotificationReadRequest true "Статус прочтения"
// @Success      200 {object} models.Notification
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /notifications/{id}/read [put]
func (h *NotificationHandler) MarkRead(c echo.Context) error {
	userID := GetUserID(c)
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	var req models.MarkNotificationReadRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	notification, err := h.service.MarkRead(c.Request().Context(), userID, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, notification)
}

// Delete godoc
// @Summary      Удаление уведомления
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID уведомления"
// @Success      200 {string} string "Уведомление удалено"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /notifications/{id} [delete]
func (h *NotificationHandler) Delete(c echo.Context) error {
	userID := GetUserID(c)
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	if err := h.service.Delete(c.Request().Context(), userID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Уведомление удалено")
}

// DeleteAll godoc
// @Summary      Удаление всех уведомлений текущего пользователя
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {string} string "Все уведомления удалены"
// @Failure      401 {object} models.HTTPError
// @Router       /notifications [delete]
func (h *NotificationHandler) DeleteAll(c echo.Context) error {
	userID := GetUserID(c)
	if err := h.service.DeleteAll(c.Request().Context(), userID); err != nil {
		return err
	}
	return RespondMessage(c, "Все уведомления удалены")
}

// Create godoc
// @Summary      Создание уведомления (admin-only)
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateNotificationRequest true "Данные уведомления"
// @Success      200 {object} models.Notification
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /notifications [post]
func (h *NotificationHandler) Create(c echo.Context) error {
	typeID := c.Get("type_id").(int)
	if typeID != 5 && typeID != 6 {
		return echo.NewHTTPError(403, "Insufficient permissions")
	}
	var req models.CreateNotificationRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	n, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, n)
}
