package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	webpush "github.com/SherClockHolmes/webpush-go"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Web Push (#974): доставка уведомлений в браузер, пока вкладка системы закрыта, поверх
// уже записанной в БД и опубликованной в реальном времени ленты (notification_service.go).

// pushSubscriptionFailureLimit -- после скольких подряд неудачных попыток доставки
// подписка считается мёртвой и удаляется, даже если push-сервис ни разу явно не ответил
// 404/410 (сеть, таймаут, 5xx). Порог с запасом от разовых сбоев push-сервиса
// (недоступность на пару минут не должна отписывать человека), но не даёт мёртвой строке
// копиться вечно: при рассылке на события заявок это около недели-двух активности.
const pushSubscriptionFailureLimit = 10

// pushPayloadMaxMessageLen -- предел длины текста в payload push-уведомления, в рунах.
// У push-сервисов есть потолок размера зашифрованного сообщения (webpush.MaxRecordSize
// = 4096 байт), заголовок и служебные поля payload съедают часть бюджета, поэтому текст
// обрезаем с запасом, а не пытаемся угадать точный лимит после шифрования.
const pushPayloadMaxMessageLen = 500

// PushSendTimeout -- таймаут одной отправки в push-сервис. Рассылка идёт вне основного
// пути (см. Send), но не должна виснуть вечно, если сервис не отвечает.
const PushSendTimeout = 10 * time.Second

// pushMessageTTLSeconds -- TTL заголовок push-запроса: сколько push-сервис хранит
// недоставленное сообщение, если устройство сейчас офлайн. Сутки - уведомление о
// событии заявки актуально в пределах рабочего дня-двух, более старое лучше не
// показывать вовсе, чем доставить с большим опозданием и без контекста.
const pushMessageTTLSeconds = 24 * 60 * 60

// PushPayload -- полезная нагрузка push-уведомления: компактная копия того, что уже
// легло в notifications, под предел размера push-сервисов. ApplicationID опционален --
// фронт открывает заявку по нему, если событие с ней связано.
type PushPayload struct {
	Title          string `json:"title"`
	Message        string `json:"message"`
	Type           string `json:"type"`
	NotificationID int    `json:"notification_id"`
	ApplicationID  *int   `json:"application_id,omitempty"`
}

// PushSender -- то, что нужно от push-рассылки notificationService (#974). Узкий
// интерфейс, а не прямая зависимость от pushService: точка интеграции легко подменяется
// в тестах и остаётся nil-safe, если push вообще не подключён (offline, тесты).
type PushSender interface {
	Send(ctx context.Context, userID int, payload PushPayload)
}

// PushService -- подписки браузеров и доставка Web Push поверх уже созданного
// уведомления.
type PushService interface {
	// Subscribe -- upsert по endpoint: повторная подписка того же браузера не плодит
	// строк, а общий компьютер со сменившимся входом переезжает на нового владельца.
	Subscribe(ctx context.Context, userID int, endpoint, p256dh, auth, userAgent string) error
	// Unsubscribe снимает подписку, только если она принадлежит userID.
	Unsubscribe(ctx context.Context, userID int, endpoint string) error
	// ListDevices -- подписки пользователя для экрана настроек, без ключей и endpoint.
	ListDevices(ctx context.Context, userID int) ([]models.PushDevice, error)
	// Send рассылает payload всем подпискам пользователя. Не возвращает ошибку -- это
	// доставка "сверху" над уже свершившимся событием (notification_service.go), сбой
	// одной или всех подписок не должен всплывать в вызывающий код.
	Send(ctx context.Context, userID int, payload PushPayload)
	// Configured сообщает, заданы ли на сервере VAPID-ключи -- статус для экрана настроек.
	Configured() bool
	// PublicKey -- VAPID public key для PushManager.subscribe на фронте. Пустая строка,
	// если push не настроен.
	PublicKey() string
}

type pushService struct {
	db         *gorm.DB
	publicKey  string
	privateKey string
	subscriber string
	httpClient webpush.HTTPClient
	// sync -- true заставляет Send отправлять и ждать завершения в вызывающей
	// горутине. Нужен тестам, которые проверяют результат рассылки (создание/удаление
	// подписки, счётчик неудач) сразу после вызова, без гонки с фоновой горутиной. В
	// проде остаётся false -- см. WithPushSyncSend.
	sync bool
}

// PushServiceOption конфигурирует pushService при создании.
type PushServiceOption func(*pushService)

// WithPushSyncSend переключает Send на синхронную отправку в вызывающей горутине.
func WithPushSyncSend() PushServiceOption {
	return func(s *pushService) { s.sync = true }
}

// WithPushHTTPClient подменяет HTTP-клиент отправки -- точка внедрения httptest-сервера
// в тестах вместо реального push-сервиса браузера.
func WithPushHTTPClient(c webpush.HTTPClient) PushServiceOption {
	return func(s *pushService) { s.httpClient = c }
}

// NewPushService создаёт сервис Web Push. Пустые publicKey/privateKey -- штатный режим
// "push выключен": Subscribe/Unsubscribe/ListDevices работают как обычно (подписка на
// экране настроек сохраняется), а Send молча ничего не отправляет.
func NewPushService(db *gorm.DB, publicKey, privateKey, subscriber string, opts ...PushServiceOption) PushService {
	s := &pushService{db: db, publicKey: publicKey, privateKey: privateKey, subscriber: subscriber}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *pushService) Configured() bool {
	return s.publicKey != "" && s.privateKey != ""
}

func (s *pushService) PublicKey() string {
	return s.publicKey
}

func (s *pushService) Subscribe(ctx context.Context, userID int, endpoint, p256dh, auth, userAgent string) error {
	if userID <= 0 {
		return fmt.Errorf("push subscribe: user_id is required")
	}
	if endpoint == "" || p256dh == "" || auth == "" {
		return fmt.Errorf("push subscribe: endpoint and keys are required")
	}
	sub := models.PushSubscription{
		UserID:    userID,
		Endpoint:  endpoint,
		P256dh:    p256dh,
		Auth:      auth,
		UserAgent: nullableString(userAgent),
	}
	// ON CONFLICT (endpoint): тот же браузер подписывается второй раз (например, после
	// повторной регистрации service worker) -- обновляем владельца и ключи вместо
	// дубля. failed_count/last_error явно сбрасываются: если endpoint переехал на
	// нового пользователя, история неудач прежнего владельца тут ни при чём.
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "endpoint"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_id", "p256dh", "auth", "user_agent", "failed_count", "last_error",
		}),
	}).Create(&sub).Error; err != nil {
		return fmt.Errorf("upsert push subscription: %w", err)
	}
	return nil
}

func (s *pushService) Unsubscribe(ctx context.Context, userID int, endpoint string) error {
	if endpoint == "" {
		return fmt.Errorf("push unsubscribe: endpoint is required")
	}
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND endpoint = ?", userID, endpoint).
		Delete(&models.PushSubscription{}).Error; err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	return nil
}

func (s *pushService) ListDevices(ctx context.Context, userID int) ([]models.PushDevice, error) {
	var rows []models.PushSubscription
	if err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list push subscriptions: %w", err)
	}
	out := make([]models.PushDevice, 0, len(rows))
	for _, r := range rows {
		ua := ""
		if r.UserAgent != nil {
			ua = *r.UserAgent
		}
		out = append(out, models.PushDevice{
			ID: r.ID, UserAgent: ua, CreatedAt: r.CreatedAt, LastSuccessAt: r.LastSuccessAt,
		})
	}
	return out, nil
}

// Send -- см. PushService.Send. Пустые VAPID-ключи -- штатный no-op (push выключен на
// сервере), рассылка не запускается вовсе.
func (s *pushService) Send(ctx context.Context, userID int, payload PushPayload) {
	if !s.Configured() {
		return
	}
	if s.sync {
		s.sendNow(ctx, userID, payload)
		return
	}
	// Своя горутина и свой контекст с таймаутом (не ctx запроса): рассылка не должна
	// ни тормозить, ни обрываться вместе с HTTP-запросом, который уже успел ответить
	// клиенту к моменту, когда push реально уходит.
	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), PushSendTimeout)
		defer cancel()
		s.sendNow(sendCtx, userID, payload)
	}()
}

func (s *pushService) sendNow(ctx context.Context, userID int, payload PushPayload) {
	var subs []models.PushSubscription
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&subs).Error; err != nil {
		slog.Error("push: не удалось прочитать подписки пользователя", "user_id", userID, "error", err)
		return
	}
	if len(subs) == 0 {
		return
	}
	message := buildPushMessage(payload)
	for _, sub := range subs {
		s.deliver(ctx, sub, message)
	}
}

func (s *pushService) deliver(ctx context.Context, sub models.PushSubscription, message []byte) {
	resp, err := webpush.SendNotificationWithContext(ctx, message, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, &webpush.Options{
		HTTPClient:      s.httpClient,
		Subscriber:      s.subscriber,
		TTL:             pushMessageTTLSeconds,
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
	})
	if err != nil {
		s.recordFailure(ctx, sub, err.Error())
		return
	}
	defer resp.Body.Close()

	switch {
	case shouldDropSubscription(resp.StatusCode):
		// 404/410 (RFC 8030): push-сервис подтверждает, что endpoint больше не
		// существует -- браузер снял подписку сам. Удаляем сразу, без накопления
		// счётчика неудач: ждать десять таких ответов подряд незачем.
		if err := s.db.WithContext(ctx).Delete(&models.PushSubscription{}, sub.ID).Error; err != nil {
			slog.Error("push: не удалось удалить мёртвую подписку", "subscription_id", sub.ID, "error", err)
		}
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		s.recordSuccess(ctx, sub.ID)
	default:
		s.recordFailure(ctx, sub, fmt.Sprintf("push service responded %d", resp.StatusCode))
	}
}

// shouldDropSubscription решает по HTTP-статусу push-сервиса, мертва ли подписка
// однозначно (без накопления счётчика неудач). 404/410 по RFC 8030 значат, что
// push-сервис сам подтверждает: endpoint больше не существует.
func shouldDropSubscription(statusCode int) bool {
	return statusCode == http.StatusNotFound || statusCode == http.StatusGone
}

func (s *pushService) recordSuccess(ctx context.Context, id int) {
	now := time.Now()
	if err := s.db.WithContext(ctx).Model(&models.PushSubscription{}).Where("id = ?", id).
		Updates(map[string]any{"last_success_at": now, "failed_count": 0, "last_error": nil}).Error; err != nil {
		slog.Error("push: не удалось отметить успешную доставку", "subscription_id", id, "error", err)
	}
}

// recordFailure копит счётчик неудач и удаляет подписку, если она перевалила
// pushSubscriptionFailureLimit -- иначе мёртвые строки (сеть/таймаут/5xx без явного
// 404/410) копились бы вечно.
func (s *pushService) recordFailure(ctx context.Context, sub models.PushSubscription, errMsg string) {
	newCount := sub.FailedCount + 1
	if newCount >= pushSubscriptionFailureLimit {
		if err := s.db.WithContext(ctx).Delete(&models.PushSubscription{}, sub.ID).Error; err != nil {
			slog.Error("push: не удалось удалить подписку после серии неудач", "subscription_id", sub.ID, "error", err)
			return
		}
		slog.Warn("push: подписка удалена после серии неудачных доставок",
			"subscription_id", sub.ID, "failed_count", newCount, "last_error", errMsg)
		return
	}
	if err := s.db.WithContext(ctx).Model(&models.PushSubscription{}).Where("id = ?", sub.ID).
		Updates(map[string]any{"failed_count": newCount, "last_error": errMsg}).Error; err != nil {
		slog.Error("push: не удалось записать неудачную доставку", "subscription_id", sub.ID, "error", err)
	}
}

// buildPushMessage сериализует payload в JSON для webpush.SendNotificationWithContext,
// обрезая длинный текст под предел размера push-сообщений (pushPayloadMaxMessageLen).
func buildPushMessage(payload PushPayload) []byte {
	payload.Message = truncatePushText(payload.Message, pushPayloadMaxMessageLen)
	body, err := json.Marshal(payload)
	if err != nil {
		// Marshal валится только на нерепрезентируемых значениях (каналы/функции),
		// которых в PushPayload нет -- сюда код дойти не должен, но best-effort
		// пустой payload лучше, чем паника на фоне бизнес-операции.
		slog.Error("push: не удалось сериализовать payload", "error", err)
		return []byte("{}")
	}
	return body
}

// truncatePushText режет текст по границе рун, чтобы не разрезать многобайтовый символ
// (кириллица) пополам.
func truncatePushText(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

// nullableString -- пустая строка становится nil: пустой User-Agent незачем хранить как
// пустую строку в nullable-колонке.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
