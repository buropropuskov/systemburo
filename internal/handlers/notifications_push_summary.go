package handlers

import "github.com/labstack/echo/v4"

// GetSummary godoc
// @Summary      Сводка использования Web Push (админ)
// @Description  Сколько активных пользователей всего, у скольких есть хотя бы одна живая
// @Description  подписка, разрез живых подписок по платформе браузера и разрез
// @Description  ПОЛЬЗОВАТЕЛЕЙ по платформе последнего успешного входа - включая тех, кто
// @Description  push не подключил (#974). Не личная настройка: гейт page.statistics.
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.PushSummary
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /notifications/push/summary [get]
func (h *PushHandler) GetSummary(c echo.Context) error {
	summary, err := h.service.GetSummary(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, summary)
}
