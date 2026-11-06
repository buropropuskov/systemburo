package handlers

import (
	"log/slog"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// GetGate отдаёт состояние согласия для текущего пользователя (#1567): нужно ли его
// спрашивать, какой редакции и какой текст показать.
//
// Required уже учитывает и настройку, и исключение супер-администратора: у гейта
// должен быть один источник правды, иначе фронт и сервер разойдутся в том, кого
// закрывать. Супер-администратор -- аварийная дверь: если запрос согласия включён с
// битой настройкой, систему всё равно можно чинить через интерфейс.
func (h *ConsentHandler) GetGate(c echo.Context) error {
	ctx := c.Request().Context()
	req, err := h.gate.CurrentRequirement(ctx)
	if err != nil {
		return err
	}

	state := models.PDConsentGateState{Version: req.Version, Hash: req.Hash, VersionAt: req.VersionAt, Text: req.Text}
	// Документ - скачиваемая копия рядом с текстом, и провалить из-за неё весь ответ
	// нельзя: без ответа пользователь не увидит окна и не сможет согласиться вовсе.
	// Поэтому ошибку логируем, но состояние отдаём.
	doc, docErr := h.settings.GetDataProcessingDoc(ctx)
	if docErr != nil {
		slog.Warn("pd_consent gate: не удалось прочитать документ согласия", "error", docErr)
	} else {
		state.Document = doc
	}

	if IsSuperAdmin(c) || !req.Enabled {
		return RespondSuccess(c, state)
	}
	userID, err := h.resolveUserID(c)
	if err != nil {
		return err
	}
	accepted, err := h.gate.AcceptedVersion(ctx, userID)
	if err != nil {
		return err
	}
	state.Required = accepted < req.Version
	return RespondSuccess(c, state)
}

// Accept подтверждает редакцию, которую пользователь видел до отправки формы.
func (h *ConsentHandler) Accept(c echo.Context) error {
	var shown models.PDConsentRevision
	if err := BindAndValidate(c, &shown); err != nil {
		return err
	}
	userID, err := h.resolveUserID(c)
	if err != nil {
		return err
	}
	req, consent, err := h.service.AcceptProcessing(c.Request().Context(), userID, shown, c.RealIP(), c.Request().UserAgent(), true)
	if err != nil {
		return err
	}
	if consent != nil {
		h.gate.Invalidate(userID)
	}
	return RespondSuccess(c, models.PDConsentGateState{Version: req.Version, Hash: req.Hash, VersionAt: req.VersionAt, Text: req.Text})
}
