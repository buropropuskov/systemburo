package handlers

import (
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// GetPDConsentSettings отдаёт текст согласия на обработку ПД, требуемую версию и флаг
// обязательности. Пока флаг выключен, согласие у пользователей не запрашивается (#1567).
func (h *SettingsHandler) GetPDConsentSettings(c echo.Context) error {
	settings, err := h.service.GetPDConsentSettings(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, settings)
}

// UpdatePDConsentText сохраняет текст согласия. Версию не двигает: правка опечатки не
// должна заставлять всех соглашаться заново -- для этого есть BumpPDConsentVersion.
func (h *SettingsHandler) UpdatePDConsentText(c echo.Context) error {
	var req models.UpdatePDConsentTextRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	ctx := c.Request().Context()
	if err := h.service.SetPDConsentText(ctx, req.Text); err != nil {
		return err
	}
	settings, err := h.service.GetPDConsentSettings(ctx)
	if err != nil {
		return err
	}
	return RespondSuccess(c, settings)
}

// UpdatePDConsentRequired включает или выключает запрос согласия при входе. Включение с
// пустым текстом сервис отклоняет.
func (h *SettingsHandler) UpdatePDConsentRequired(c echo.Context) error {
	var req models.UpdatePDConsentRequiredRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	ctx := c.Request().Context()
	if err := h.service.SetPDConsentRequired(ctx, *req.Required); err != nil {
		return err
	}
	settings, err := h.service.GetPDConsentSettings(ctx)
	if err != nil {
		return err
	}
	return RespondSuccess(c, settings)
}

// BumpPDConsentVersion поднимает требуемую версию согласия -- система запросит его
// заново у всех, кто соглашался с прежней редакцией текста.
func (h *SettingsHandler) BumpPDConsentVersion(c echo.Context) error {
	ctx := c.Request().Context()
	if _, err := h.service.BumpPDConsentVersion(ctx); err != nil {
		return err
	}
	settings, err := h.service.GetPDConsentSettings(ctx)
	if err != nil {
		return err
	}
	return RespondSuccess(c, settings)
}
