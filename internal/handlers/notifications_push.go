package handlers

import (
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// PushHandler -- HTTP-обработчики Web Push подписок (#974): доставка уведомлений в
// браузер при закрытой вкладке, поверх обычной ленты /notifications.
type PushHandler struct {
	service services.PushService
}

// NewPushHandler создаёт новый экземпляр PushHandler.
func NewPushHandler(service services.PushService) *PushHandler {
	return &PushHandler{service: service}
}

// GetStatus godoc
// @Summary      Статус Web Push и список подписанных устройств
// @Description  Публичный VAPID-ключ для PushManager.subscribe, признак "push настроен
// @Description  на сервере" (пустые VAPID-ключи в параметрах = выключен) и список
// @Description  подписок текущего пользователя (#974).
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.PushStatusResponse
// @Failure      401 {object} models.HTTPError
// @Router       /notifications/push/status [get]
func (h *PushHandler) GetStatus(c echo.Context) error {
	userID := GetUserID(c)
	devices, err := h.service.ListDevices(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, models.PushStatusResponse{
		Enabled:   h.service.Configured(),
		PublicKey: h.service.PublicKey(),
		Devices:   devices,
	})
}

// Subscribe godoc
// @Summary      Подписка браузера на Web Push
// @Description  Endpoint и ключи из PushSubscription.toJSON() браузера. Повторная
// @Description  подписка с тем же endpoint переносит её на текущего пользователя (общий
// @Description  компьютер, сменившийся вход), без дублей (#974).
// @Tags         notifications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.PushSubscribeRequest true "Подписка браузера"
// @Success      200 {string} string "Подписка сохранена"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /notifications/push/subscribe [post]
func (h *PushHandler) Subscribe(c echo.Context) error {
	userID := GetUserID(c)
	var req models.PushSubscribeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Subscribe(c.Request().Context(), userID, req.Endpoint, req.Keys.P256dh, req.Keys.Auth, c.Request().UserAgent()); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return RespondMessage(c, "Подписка сохранена")
}

// Unsubscribe godoc
// @Summary      Отписка браузера от Web Push
// @Description  Endpoint в query (?endpoint=...) либо в теле запроса. Снимает только
// @Description  подписку текущего пользователя (#974).
// @Tags         notifications
// @Produce      json
// @Security     BearerAuth
// @Param        endpoint query string false "Endpoint подписки"
// @Param        request body models.PushUnsubscribeRequest false "Endpoint подписки, если не передан в query"
// @Success      200 {string} string "Подписка снята"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /notifications/push/subscribe [delete]
func (h *PushHandler) Unsubscribe(c echo.Context) error {
	userID := GetUserID(c)
	endpoint := c.QueryParam("endpoint")
	if endpoint == "" {
		var req models.PushUnsubscribeRequest
		// Тело необязательно (endpoint мог прийти в query) - ошибку биндинга здесь
		// молча игнорируем, её ловит проверка пустого endpoint ниже.
		if err := c.Bind(&req); err == nil {
			endpoint = req.Endpoint
		}
	}
	if endpoint == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "endpoint is required")
	}
	if err := h.service.Unsubscribe(c.Request().Context(), userID, endpoint); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return RespondMessage(c, "Подписка снята")
}
