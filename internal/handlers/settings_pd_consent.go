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

// UpdatePDConsentText сохраняет текст согласия. Редакцию двигает только по явному
// require_again: правка опечатки не должна заставлять всех соглашаться заново, а
// правка по существу должна -- решает администратор при сохранении.
func (h *SettingsHandler) UpdatePDConsentText(c echo.Context) error {
	var req models.UpdatePDConsentTextRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	ctx := c.Request().Context()
	if err := h.service.SetPDConsentText(ctx, req.Text, req.RequireAgain); err != nil {
		return err
	}
	h.invalidateConsentGate()
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
	h.invalidateConsentGate()
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
	h.invalidateConsentGate()
	settings, err := h.service.GetPDConsentSettings(ctx)
	if err != nil {
		return err
	}
	return RespondSuccess(c, settings)
}

// invalidateConsentGate сбрасывает кэш гейта согласия: без этого правка текста,
// тумблера или редакции доезжала бы до пользователей лишь по истечении TTL.
func (h *SettingsHandler) invalidateConsentGate() {
	h.consentGate.InvalidateAll()
}

// GetPDConsentCollection отдаёт сводку по сбору согласий текущей редакции и список
// тех, кто ещё не подтвердил (#1567). Считается той же меркой, что и гейт, поэтому
// число согласившихся всегда совпадает с числом тех, кого система пускает.
// Параметр full=1 снимает ограничение на длину списка не подтвердивших: он нужен
// выгрузке в файл, где урезанный список означал бы потерю людей.
func (h *SettingsHandler) GetPDConsentCollection(c echo.Context) error {
	full := c.QueryParam("full") == "1"
	collection, err := h.consentStats.Collection(c.Request().Context(), full)
	if err != nil {
		return err
	}
	return RespondSuccess(c, collection)
}
