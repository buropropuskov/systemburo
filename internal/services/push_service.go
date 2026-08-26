package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
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

// pushPayloadMaxTitleLen -- предел длины заголовка, в рунах, отдельно от message.
// Заголовки уведомлений в каталоге - короткие человеческие подписи ("Заявка на
// согласование", "Изменился статус заявки"), обычно до полусотни символов, поэтому 100
// - щедрый запас. Обрезаем ЯВНО здесь, а не полагаемся на то, что notifications.title
// в БД varchar(255): это ограничение схемы для ленты уведомлений в интерфейсе, а не
// договорённость с push-сервисом - удобный побочный эффект, а не гарантия.
const pushPayloadMaxTitleLen = 100

// PushSendTimeout -- таймаут одной отправки в push-сервис. Рассылка идёт вне основного
// пути (см. Send), но не должна виснуть вечно, если сервис не отвечает.
const PushSendTimeout = 10 * time.Second

// pushMessageTTLSeconds -- TTL заголовок push-запроса: сколько push-сервис хранит
// недоставленное сообщение, если устройство сейчас офлайн. Сутки - уведомление о
// событии заявки актуально в пределах рабочего дня-двух, более старое лучше не
// показывать вовсе, чем доставить с большим опозданием и без контекста.
const pushMessageTTLSeconds = 24 * 60 * 60

// pushMessageUrgency -- заголовок Urgency (RFC 8030): при какой степени экономии
// заряда push-служба всё ещё будит устройство. Без заголовка сообщение считается
// "normal" и на телефоне с низким зарядом придерживается до следующего пробуждения,
// а уведомление о заявке нужно человеку тогда же, когда оно случилось - в этом весь
// смысл доставки наружу. На вид уведомления (всплывающий баннер) заголовок не влияет
// никак: всплытие решает система по важности своего канала уведомлений, сайту она
// не подчиняется - проверено на Samsung в двух браузерах, см. #974.
const pushMessageUrgency = webpush.UrgencyHigh

// pushMaxConcurrentDeliveries -- общий на весь pushService потолок ОДНОВРЕМЕННЫХ
// исходящих запросов к push-сервисам (не на пользователя и не на одну рассылку).
// Без предела уведомление о новости на пару сотен активных пользователей открыло бы
// столько же одновременных HTTPS-соединений разом (разбор #974): каждый Send()
// сегодня - отдельная горутина без какого-либо троттлинга. 30 - компромисс между
// "не открывать сотни сокетов одновременно" и скоростью рассылки: даже в худшем
// случае, когда каждая отправка тратит весь PushSendTimeout=10с, пара сотен подписок
// разъедутся из очереди за десятки секунд, а не за минуты; запас от типичных
// системных лимитов дескрипторов (обычно 1024) остаётся большим.
const pushMaxConcurrentDeliveries = 30

// PushShutdownGrace -- сколько main.go ждёт завершения уже запущенных отправок при
// остановке сервера, ПОСЛЕ e.Shutdown (см. Shutdown). Короче общего 10-секундного
// окна остановки HTTP-сервера: push - канал поверх основной функциональности, не
// более важный, чем не тянуть время процесса.
const PushShutdownGrace = 5 * time.Second

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
	// GetSummary -- сводка использования Web Push для админского раздела статистики
	// (#974, реализация в push_summary.go): не личная настройка, гейтится page.statistics.
	GetSummary(ctx context.Context) (*models.PushSummary, error)
	// Shutdown ждёт завершения уже запущенных фоновых отправок, но не дольше срока,
	// отведённого ctx. main.go зовёт её ПОСЛЕ e.Shutdown, перед тем как процесс
	// завершится: без этого рантайм Go убивает недоделанные отправки молча вместе со
	// всем процессом. Взводит признак остановки - новые Send() после вызова no-op'ятся.
	Shutdown(ctx context.Context)
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

	// wg отслеживает все запущенные фоновые отправки (#974): Shutdown ждёт её
	// опустошения вместо того, чтобы дать рантайму молча убить их вместе с процессом.
	wg sync.WaitGroup
	// deliverySem -- общий на сервис семафор одновременных отправок, см.
	// pushMaxConcurrentDeliveries.
	deliverySem chan struct{}
	// shuttingDown взводится в начале Shutdown: Send(), вызванный после этого момента,
	// не встаёт в очередь - сервер уже останавливается, новая отправка не успеет уйти
	// за отведённый Shutdown срок, а только продлит ожидание.
	shuttingDown atomic.Bool
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
	s := &pushService{
		db: db, publicKey: publicKey, privateKey: privateKey, subscriber: normalizePushSubscriber(subscriber),
		deliverySem: make(chan struct{}, pushMaxConcurrentDeliveries),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// normalizePushSubscriber снимает схему mailto: с адреса контакта перед передачей в
// webpush-go. Библиотека приписывает mailto: сама всему, что не начинается с https:
// (vapid.go), поэтому готовое значение из VAPID_SUBJECT превращалось в подписи в
// "mailto:mailto:адрес". Google и Mozilla на это поле смотрят сквозь пальцы, а Apple
// отвергает запрос с 403 - на живом iPhone (#974) не доходило НИ ОДНО уведомление, при
// том что Android и компьютер работали. В параметре схему оставляем: человеку, который
// читает .env, она говорит, что здесь адрес почты, а не что-то другое.
func normalizePushSubscriber(subscriber string) string {
	return strings.TrimPrefix(strings.TrimSpace(subscriber), "mailto:")
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
	if s.shuttingDown.Load() {
		return
	}
	// Своя горутина и свой контекст с таймаутом (не ctx запроса): вызывающая сторона
	// (например, рассылка новости всем активным пользователям, notifyNewsPublished)
	// не должна ждать НИ приобретения слота в пуле, ни самой отправки - Send()
	// обязан вернуться мгновенно. Ограничение конкурентности (deliverySem) применяется
	// ВНУТРИ этой уже отсоединённой горутины, а не здесь.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Ждём место в общем пуле НЕ по ctx запроса: тот отменится, как только
		// Echo отправит ответ клиенту, а очередь на отправку может быть длиннее
		// одного запроса (рассылка новости на сотни пользователей). Ограничена
		// только явным Shutdown снаружи - при остановке горутина, застрявшая тут,
		// просто не успеет за отведённый срок и умрёт вместе с процессом,
		// не зависнув никого другого.
		s.deliverySem <- struct{}{}
		defer func() { <-s.deliverySem }()

		sendCtx, cancel := context.WithTimeout(context.Background(), PushSendTimeout)
		defer cancel()
		s.sendNow(sendCtx, userID, payload)
	}()
}

// Shutdown -- см. PushService.Shutdown.
func (s *pushService) Shutdown(ctx context.Context) {
	s.shuttingDown.Store(true)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("push: все отправки завершены до остановки сервера")
	case <-ctx.Done():
		slog.Warn("push: остановка не дождалась части отправок - истёк отведённый срок, часть доставок и отметок об успехе может быть потеряна")
	}
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
		Urgency:         pushMessageUrgency,
		VAPIDPublicKey:  s.publicKey,
		VAPIDPrivateKey: s.privateKey,
	})
	if err != nil {
		if errors.Is(err, webpush.ErrMaxPadExceeded) {
			// Payload не влез в бюджет push-сервиса ДО сетевого запроса (webpush-go
			// считает размер локально, см. buildPushMessage). Это наш баг кодирования,
			// а не проблема подписки - счётчик неудач нарочно не трогаем: он копится,
			// когда подписка "виновата" (сеть, таймаут, 5xx), и после
			// pushSubscriptionFailureLimit подряд таких случаев подписку удаляет
			// recordFailure. Наказывать ни в чём не повинное устройство за нашу
			// ошибку было бы неправильно - логируем громко и переходим к следующей
			// подписке.
			slog.Error("push: payload превысил предел размера push-сервиса - подписка тут ни при чём",
				"subscription_id", sub.ID, "user_id", sub.UserID, "error", err)
			return
		}
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
// обрезая длинный текст под предел размера push-сообщений (pushPayloadMaxMessageLen,
// pushPayloadMaxTitleLen) - явно для обоих полей, не полагаясь на побочные ограничения
// схемы БД.
func buildPushMessage(payload PushPayload) []byte {
	payload.Title = truncatePushText(payload.Title, pushPayloadMaxTitleLen)
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
