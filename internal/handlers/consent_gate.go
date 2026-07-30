package handlers

import (
	"log/slog"

	"systemburo/internal/models"
	"systemburo/internal/services"

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
	req, err := h.gate.Requirement(ctx)
	if err != nil {
		return err
	}

	state := models.PDConsentGateState{Version: req.Version, VersionAt: req.VersionAt, Text: req.Text}
	// Документ - скачиваемая копия рядом с текстом, и провалить из-за неё весь ответ
	// нельзя: без ответа пользователь не увидит окна и не сможет согласиться вовсе.
	// Поэтому ошибку логируем, но состояние отдаём.
	doc, docErr := h.settings.GetDataProcessingDoc(ctx)
	if docErr != nil {
		slog.Warn("pd_consent gate: не удалось прочитать документ согласия", "error", docErr)
	} else {
		state.Document = doc
	}

	if IsSuperAdmin(c) {
		return RespondSuccess(c, state)
	}
	userID, err := h.resolveUserID(c)
	if err != nil {
		return err
	}
	needs, err := h.gate.NeedsConsent(ctx, userID)
	if err != nil {
		return err
	}
	state.Required = needs
	return RespondSuccess(c, state)
}

// Accept записывает согласие текущего пользователя на текущую редакцию текста.
// Редакцию и хэш штампует сервер: поля запроса тут нет намеренно, иначе клиент
// прислал бы заведомо большое число и освободился от всех будущих переподтверждений.
func (h *ConsentHandler) Accept(c echo.Context) error {
	ctx := c.Request().Context()
	req, err := h.gate.Requirement(ctx)
	if err != nil {
		return err
	}
	// Запрос согласия могли выключить, пока пользователь читал текст. Отвечать
	// ошибкой на такой клик неправильно: подтверждать уже нечего, окно просто
	// закрывается. Записи при этом не делаем - она была бы про пустое требование.
	if !req.Enabled {
		return RespondSuccess(c, models.PDConsentGateState{Version: req.Version, VersionAt: req.VersionAt, Text: req.Text})
	}
	userID, err := h.resolveUserID(c)
	if err != nil {
		return err
	}
	grant := models.GrantConsentRequest{ConsentType: services.ConsentTypePDProcessing}
	if _, err := h.service.Grant(ctx, userID, grant, c.RealIP(), c.Request().UserAgent(), req.Version, req.Hash); err != nil {
		return err
	}
	// Без сброса кэша доступ открылся бы только по истечении TTL: фронт уже снял
	// окно, а API продолжал бы отказывать.
	h.gate.Invalidate(userID)

	needs, err := h.gate.NeedsConsent(ctx, userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, models.PDConsentGateState{
		Required:  needs,
		Version:   req.Version,
		VersionAt: req.VersionAt,
		Text:      req.Text,
	})
}
