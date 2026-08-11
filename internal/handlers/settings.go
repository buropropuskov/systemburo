package handlers

import (
	"net/http"

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
	// mail подключается сеттером после конструирования (#1906): почтовый сервис
	// создаётся позже хендлера, а в тестах его нет вовсе.
	mail services.MailSender
}

// SetMailSender подключает почтовый сервис. Без него ручка проверки почты
// отвечает «почта не настроена» - ровно как при пустом SMTP_HOST.
func (h *SettingsHandler) SetMailSender(m services.MailSender) {
	h.mail = m
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

// GetMailStatus сообщает, настроена ли отправка почты. Без него администратор
// включал бы плановую рассылку вслепую и узнавал о ненастроенной почте из отчёта
// о несостоявшемся прогоне.
func (h *SettingsHandler) GetMailStatus(c echo.Context) error {
	return RespondSuccess(c, map[string]bool{"configured": h.mail != nil && h.mail.Enabled()})
}

// SendTestMail отправляет проверочное письмо на указанный адрес и отвечает
// синхронно. Настройка чужого почтового сервера без такой кнопки превращается в
// переписку с поддержкой: администратор не видит ни кода отказа, ни причины.
func (h *SettingsHandler) SendTestMail(c echo.Context) error {
	var req models.TestMailRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if h.mail == nil || !h.mail.Enabled() {
		return echo.NewHTTPError(http.StatusBadRequest,
			"Почта не настроена: задайте параметры SMTP в файле параметров и перезапустите систему")
	}

	msg := services.MailMessage{
		To:           req.To,
		Subject:      "Проверка почтовой рассылки",
		Body:         testMailBody,
		TemplateCode: services.MailTemplateTest,
	}
	if err := h.mail.SendNow(c.Request().Context(), msg); err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, services.ExplainMailError(err))
	}
	return RespondSuccess(c, map[string]any{"sent": true, "to": req.To})
}

// testMailBody - текст проверочного письма. Пишется от лица системы и объясняет
// получателю, почему письмо пришло: адрес для проверки часто вводят чужой.
const testMailBody = `Это проверочное письмо системы бюро пропусков.

Оно отправлено вручную из раздела настроек, чтобы убедиться, что почтовый сервер
принимает письма от системы. Отвечать на него не нужно.

Если письмо попало в папку со спамом, проверьте записи SPF и DKIM у домена
отправителя: без них письма системы будут теряться у получателей.`
