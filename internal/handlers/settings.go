package handlers

import (
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

type SettingsHandler struct {
	service     services.SettingsService
	fileSvc     services.DocumentFileService
	maxFileSize int64
	// consentGate нужен, чтобы правка текста, тумблера или редакции согласия
	// действовала сразу, а не по истечении TTL кэша гейта (#1567).
	consentGate  *services.PDConsentGateService
	consentStats *services.PDConsentStatsService
}

// NewSettingsHandler создаёт хендлер для управления системными настройками.
func NewSettingsHandler(
	service services.SettingsService,
	fileSvc services.DocumentFileService,
	maxFileSize int64,
	consentGate *services.PDConsentGateService,
	consentStats *services.PDConsentStatsService,
) *SettingsHandler {
	return &SettingsHandler{
		service:      service,
		fileSvc:      fileSvc,
		maxFileSize:  maxFileSize,
		consentGate:  consentGate,
		consentStats: consentStats,
	}
}

// GetAll возвращает все системные настройки. Доступ - под page.admin.settings
// (#7): гейтится роутером, а не хендлером.
func (h *SettingsHandler) GetAll(c echo.Context) error {
	settings, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, settings)
}

// Update обновляет значение конкретной настройки по ключу. Доступ - под
// page.admin.settings (#7): гейтится роутером, а не хендлером.
func (h *SettingsHandler) Update(c echo.Context) error {
	key := c.Param("key")
	var req models.UpdateSettingRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	setting, err := h.service.Update(c.Request().Context(), key, req.Value)
	if err != nil {
		return err
	}
	return RespondSuccess(c, setting)
}

// GetUploadSettings возвращает настройки загрузки файлов для фронтенда.
func (h *SettingsHandler) GetUploadSettings(c echo.Context) error {
	result, err := h.service.GetUploadSettings(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// GetNotificationSettings возвращает длительности уведомлений удаления/восстановления.
func (h *SettingsHandler) GetNotificationSettings(c echo.Context) error {
	result, err := h.service.GetNotificationSettings(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// GetPasswordPolicy возвращает текущую политику требований к паролю для фронтенда
// (живой чеклист в форме). Доступен любому авторизованному.
func (h *SettingsHandler) GetPasswordPolicy(c echo.Context) error {
	return RespondSuccess(c, h.service.GetPasswordPolicy())
}

// GetPublicContacts возвращает контакты Бюро пропусков (телефон, почта). Публичный
// (без JWT): нужен на странице логина и в плашке блокировки до/без авторизации.
func (h *SettingsHandler) GetPublicContacts(c echo.Context) error {
	return RespondSuccess(c, h.service.GetPublicContacts(c.Request().Context()))
}
