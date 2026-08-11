package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

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
	// rotationStatus - состояние плановой смены паролей (#1909), подключается
	// тем же способом и по той же причине.
	rotationStatus *services.PasswordRotationStatusService
	// rotation - сам прогон плановой смены (#1910).
	rotation *services.PasswordRotationService
}

// SetRotationService подключает сервис плановой смены паролей.
func (h *SettingsHandler) SetRotationService(s *services.PasswordRotationService) {
	h.rotation = s
}

// SetRotationStatusService подключает счётчик состояния плановой смены паролей.
func (h *SettingsHandler) SetRotationStatusService(s *services.PasswordRotationStatusService) {
	h.rotationStatus = s
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

// GetMailStatus godoc
// @Summary      Состояние настройки почты
// @Description  Сообщает, настроена ли отправка писем (задан ли SMTP_HOST).
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]bool
// @Failure      403 {object} models.HTTPError
// @Router       /settings/mail/status [get]
//
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

// GetPasswordRotationStatus godoc
// @Summary      Состояние проверки сроков действия паролей
// @Description  Настроена ли почта, когда ближайшая проверка сроков и скольких работников она затронет.
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} services.PasswordRotationStatus
// @Failure      403 {object} models.HTTPError
// @Failure      503 {object} models.HTTPError
// @Router       /settings/password-rotation/status [get]
//
// GetPasswordRotationStatus отдаёт состояние проверки сроков: у скольких работников
// пароль уже истёк, скольким уйдёт предупреждение и настроена ли почта. Без этих
// чисел администратор включал бы проверку вслепую.
func (h *SettingsHandler) GetPasswordRotationStatus(c echo.Context) error {
	if h.rotationStatus == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Состояние проверки сроков паролей недоступно")
	}
	status, err := h.rotationStatus.Get(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Не удалось посчитать состояние плановой смены")
	}
	return RespondSuccess(c, status)
}

// RunPasswordRotation godoc
// @Summary      Сменить пароли всем работникам
// @Description  Ручное обновление: придумывает новый пароль всем действующим работникам с адресом почты и высылает его письмом. Возвращает управление сразу, письма ставятся в очередь.
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @Success      202 {object} map[string]any
// @Failure      403 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Failure      412 {object} models.HTTPError
// @Router       /settings/password-rotation/run [post]
//
// RunPasswordRotation запускает обновление паролей вручную. Ответ отдаётся сразу:
// ждать в интерфейсе, пока разойдутся сотни писем, нельзя, а сама смена идёт
// быстро - письма разбирает почтовый воркер по своему темпу.
func (h *SettingsHandler) RunPasswordRotation(c echo.Context) error {
	if h.rotation == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Обновление паролей недоступно")
	}
	if h.rotation.IsRunning() {
		// 409, а не молчаливый второй прогон: двойной клик выдал бы работнику два
		// пароля подряд, и рабочим оказался бы только последний.
		return echo.NewHTTPError(http.StatusConflict, "Смена паролей уже выполняется")
	}
	if h.mail == nil || !h.mail.Enabled() {
		return echo.NewHTTPError(http.StatusPreconditionFailed,
			"Почта не настроена: менять пароли, не имея канала доставки, нельзя")
	}

	userID := GetUserID(c)
	// Контекст запроса здесь не годится: он умирает вместе с ответом, а прогон
	// продолжается. Берём фоновый со своим сроком.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		if _, err := h.rotation.Run(ctx, userID); err != nil {
			slog.Error("ручная смена паролей завершилась ошибкой", "error", err, "started_by", userID)
		}
	}()

	return c.JSON(http.StatusAccepted, map[string]any{
		"success": true,
		"data":    map[string]any{"started": true},
	})
}

// GetPasswordRotationLast отдаёт итог последнего прогона в этом процессе.
// Перезапуск сервера его забывает - это осознанно: сведения справочные, ради них
// заводить таблицу прогонов незачем, а факты смены и так лежат в журнале действий.
func (h *SettingsHandler) GetPasswordRotationLast(c echo.Context) error {
	if h.rotation == nil {
		return RespondSuccess(c, map[string]any{"last": nil})
	}
	return RespondSuccess(c, map[string]any{
		"running": h.rotation.IsRunning(),
		"last":    h.rotation.LastResult(),
	})
}
