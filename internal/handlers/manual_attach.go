package handlers

import (
	"net/http"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// ManualAttachHandler -- HTTP-обработчик привязки ручного вложения-сироты к заявке (#1049 режим-2).
type ManualAttachHandler struct {
	service services.ManualAttachService
}

// NewManualAttachHandler создаёт новый экземпляр ManualAttachHandler.
func NewManualAttachHandler(service services.ManualAttachService) *ManualAttachHandler {
	return &ManualAttachHandler{service: service}
}

// AttachToApplication обрабатывает POST /attachments/:id/attach-to-application.
// @Summary Привязка ручного вложения к заявке (#1049, только super/admin)
// @Description Усыновляет вложение-сироту в заявку (application_id) ЛИБО перевешивает его сущности на существующее вложение заявки (target_attachment_id). Ровно одно поле.
// @Tags attachments
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path int true "ID ручного вложения-сироты"
// @Param body body services.AttachToApplicationRequest true "Целевая заявка или вложение"
// @Success 200 {object} services.AttachToApplicationResponse
// @Failure 400 {object} models.HTTPError
// @Failure 403 {object} models.HTTPError
// @Failure 404 {object} models.HTTPError
// @Failure 422 {object} models.HTTPError
// @Router /attachments/{id}/attach-to-application [post]
func (h *ManualAttachHandler) AttachToApplication(c echo.Context) error {
	id, err := ParseID(c, "id")
	if err != nil {
		return err
	}
	// Голый Bind (конвенция границы #888): оба поля опциональны, обязательность одного из
	// них - взаимоисключающая доменная валидация (XOR), не выражается статичным тегом и живёт
	// в сервисе AttachToApplication.
	var req services.AttachToApplicationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	resp, err := h.service.AttachToApplication(c.Request().Context(), id, req, GetUserID(c))
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}
