package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// notificationsListMaxLimit -- потолок limit на GET /notifications: страница ленты не
// должна расти произвольно (#1748).
const notificationsListMaxLimit = 100

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
// @Description  Без limit - полный массив без пагинации (legacy, обратная совместимость).
// @Description  С limit - страница (+offset, +filter=all|unread), meta несёт total и
// @Description  unread_count по всем уведомлениям пользователя (#1748).
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Param        limit  query int    false "Размер страницы (<=100); наличие включает пагинацию"
// @Param        offset query int    false "Смещение страницы"
// @Param        filter query string false "all|unread, по умолчанию all"
// @Success      200 {array} models.Notification
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /notifications [get]
func (h *NotificationHandler) GetNotifications(c echo.Context) error {
	userID := GetUserID(c)

	// Legacy mode: без limit отдаём полный массив, как раньше - фронт колокольчика
	// дёргает этот путь без пагинации, и так должно оставаться до среза S7 (#1748).
	limitParam := c.QueryParam("limit")
	if limitParam == "" {
		notifications, err := h.service.GetByUserID(c.Request().Context(), userID)
		if err != nil {
			return err
		}
		return RespondSuccess(c, notifications)
	}

	limit, err := strconv.Atoi(limitParam)
	if err != nil || limit <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid limit")
	}
	if limit > notificationsListMaxLimit {
		limit = notificationsListMaxLimit
	}

	offset := 0
	if offsetParam := c.QueryParam("offset"); offsetParam != "" {
		offset, err = strconv.Atoi(offsetParam)
		if err != nil || offset < 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid offset")
		}
	}

	filter := c.QueryParam("filter")
	if filter == "" {
		filter = "all"
	}
	if filter != "all" && filter != "unread" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid filter")
	}

	notifications, total, unread, err := h.service.GetByUserIDPaginated(c.Request().Context(), userID, limit, offset, filter)
	if err != nil {
		return err
	}
	return RespondPaginated(c, notifications, models.NotificationListMeta{
		PaginationMeta: models.PaginationMeta{Total: total, Page: offset/limit + 1, PerPage: limit},
		UnreadCount:    unread,
	})
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

// MarkAllRead godoc
// @Summary      Отметить все уведомления прочитанными
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.MarkAllReadResponse
// @Failure      401 {object} models.HTTPError
// @Router       /notifications/read-all [put]
func (h *NotificationHandler) MarkAllRead(c echo.Context) error {
	userID := GetUserID(c)
	count, err := h.service.MarkAllRead(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, models.MarkAllReadResponse{Updated: count})
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

// GetPreferences godoc
// @Summary      Настройки подписки на типы уведомлений
// @Description  Каталог целиком, сгруппированный по категориям, с эффективным
// @Description  состоянием переключателя для текущего пользователя (#1748).
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.NotificationPreferenceCategory
// @Failure      401 {object} models.HTTPError
// @Router       /notifications/preferences [get]
func (h *NotificationHandler) GetPreferences(c echo.Context) error {
	userID := GetUserID(c)
	prefs, err := h.service.GetPreferences(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, prefs)
}

// UpdatePreferences godoc
// @Summary      Батч-правка подписки на типы уведомлений
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.UpdateNotificationPreferencesRequest true "Список изменений"
// @Success      200 {string} string "Настройки уведомлений сохранены"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /notifications/preferences [put]
func (h *NotificationHandler) UpdatePreferences(c echo.Context) error {
	userID := GetUserID(c)
	var req models.UpdateNotificationPreferencesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdatePreferences(c.Request().Context(), userID, req.Items); err != nil {
		return err
	}
	return RespondMessage(c, "Настройки уведомлений сохранены")
}
