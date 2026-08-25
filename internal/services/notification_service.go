package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/realtime"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// notificationAggregationWindow -- окно схлопывания повторов одного типа/ключа в одну
// запись ленты (#1748). Второе и следующие события той же группы, пока прежняя запись
// ещё не прочитана и не старше окна, не заводят новую строку, а обновляют count и
// last_event_at существующей.
const notificationAggregationWindow = 30 * time.Minute

// NotificationService -- интерфейс бизнес-логики уведомлений.
type NotificationService interface {
	GetByUserID(ctx context.Context, userID int) ([]models.Notification, error)
	// GetByUserIDPaginated -- страница ленты (limit/offset, filter=all|unread) плюс
	// общее количество непрочитанных, не зависящее от страницы/фильтра (#1748).
	GetByUserIDPaginated(ctx context.Context, userID, limit, offset int, filter string) (notifications []models.Notification, total int64, unreadCount int64, err error)
	MarkRead(ctx context.Context, userID int, id int, req models.MarkNotificationReadRequest) (*models.Notification, error)
	// MarkAllRead отмечает прочитанными все непрочитанные уведомления пользователя,
	// возвращает их количество.
	MarkAllRead(ctx context.Context, userID int) (int64, error)
	Delete(ctx context.Context, userID int, id int) error
	DeleteAll(ctx context.Context, userID int) error
	Create(ctx context.Context, req models.CreateNotificationRequest) (*models.Notification, error)
	CreateForUser(ctx context.Context, userID int, notifType, title, message string, data *string) error
	// CreateForUserGrouped -- то же самое, но с явным ключом схлопывания повторов.
	// Пустой groupKey отключает схлопывание для этого вызова. Заготовка для срезов
	// S3-S5, где ключ считается по своей логике (например, по получателю рассылки).
	CreateForUserGrouped(ctx context.Context, userID int, notifType, title, message string, data *string, groupKey string) error
	// GetPreferences -- каталог целиком, сгруппированный по категориям, с эффективным
	// состоянием переключателя для этого пользователя.
	GetPreferences(ctx context.Context, userID int) ([]models.NotificationPreferenceCategory, error)
	// UpdatePreferences сохраняет батч изменений подписки. Валидация (код есть в
	// каталоге, mandatory нельзя выключить) проходит целиком до записи -- частично
	// применённого батча не бывает.
	UpdatePreferences(ctx context.Context, userID int, items []models.NotificationPreferenceItemUpdate) error
}

type notificationService struct {
	db                 *gorm.DB
	realtimePublisher  realtime.Publisher
	permissionResolver *PermissionResolver
	pushSender         PushSender
}

// NotificationServiceOption конфигурирует notificationService при создании.
type NotificationServiceOption func(*notificationService)

// WithNotificationRealtimePublisher включает публикацию real-time сигнала
// "новое уведомление" (#840) адресно юзеру, чтобы фронт мгновенно перезапросил
// колокольчик вместо ожидания 30с-поллинга. Опционально: без неё сигналы не
// шлются (тесты, offline).
func WithNotificationRealtimePublisher(p realtime.Publisher) NotificationServiceOption {
	return func(s *notificationService) { s.realtimePublisher = p }
}

// WithNotificationPermissionResolver подключает резолвер прав: по нему экран
// настроек прячет типы, которых человек всё равно не получит (#1748). Опционально,
// nil-safe: без резолвера показываются все ненулевые типы.
func WithNotificationPermissionResolver(r *PermissionResolver) NotificationServiceOption {
	return func(s *notificationService) { s.permissionResolver = r }
}

// WithNotificationPushSender подключает Web Push рассылку (#974): доставку "сверху" над
// уже записанным в БД и опубликованным в реальном времени уведомлением, в тех же точках,
// что и realtimePublisher.Publish. Опционально, nil-safe: без неё push просто не уходит
// (тесты, offline, выключенные на сервере VAPID-ключи).
func WithNotificationPushSender(p PushSender) NotificationServiceOption {
	return func(s *notificationService) { s.pushSender = p }
}

// NewNotificationService создаёт реализацию NotificationService.
func NewNotificationService(db *gorm.DB, opts ...NotificationServiceOption) NotificationService {
	s := &notificationService{db: db}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *notificationService) GetByUserID(ctx context.Context, userID int) ([]models.Notification, error) {
	notifications := make([]models.Notification, 0)
	// COALESCE на last_event_at (#1748): у схлопнутого уведомления именно этот момент -
	// момент последнего события в группе, и лента должна поднимать его наверх, как
	// будто оно только что создано заново. У обычного, не схлопнутого уведомления
	// last_event_at пуст, и сортировка падает обратно на created_at. id DESC вторым
	// ключом - устойчивость при совпадении момента (например, массовая рассылка).
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("COALESCE(last_event_at, created_at) DESC, id DESC").
		Find(&notifications).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching notifications")
	}
	return notifications, nil
}

// notificationListQuery -- базовый запрос ленты с фильтром read/unread, без сортировки и
// лимита. Строится заново для Count и для Find по отдельности (эталон -
// GetApplicationsPaginated): переиспользование одного *gorm.DB между двумя выполнениями
// не гарантирует независимость условий между вызовами.
func (s *notificationService) notificationListQuery(ctx context.Context, userID int, filter string) *gorm.DB {
	q := s.db.WithContext(ctx).Model(&models.Notification{}).Where("user_id = ?", userID)
	if filter == "unread" {
		q = q.Where("is_read = ?", false)
	}
	return q
}

func (s *notificationService) GetByUserIDPaginated(ctx context.Context, userID, limit, offset int, filter string) ([]models.Notification, int64, int64, error) {
	var total int64
	if err := s.notificationListQuery(ctx, userID, filter).Count(&total).Error; err != nil {
		return nil, 0, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error counting notifications")
	}

	notifications := make([]models.Notification, 0, limit)
	err := s.notificationListQuery(ctx, userID, filter).
		Order("COALESCE(last_event_at, created_at) DESC, id DESC").
		Limit(limit).Offset(offset).
		Find(&notifications).Error
	if err != nil {
		return nil, 0, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching notifications")
	}

	// unread_count игнорирует и фильтр, и страницу -- бейдж колокольчика должен
	// показывать общее непрочитанное количество, а не то, что попало в текущую выборку.
	var unread int64
	if err := s.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Count(&unread).Error; err != nil {
		return nil, 0, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error counting unread notifications")
	}

	return notifications, total, unread, nil
}

func (s *notificationService) findOwned(ctx context.Context, userID int, id int) (*models.Notification, error) {
	var n models.Notification
	err := s.db.WithContext(ctx).First(&n, id).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Notification not found")
	}
	if n.UserID != userID {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	return &n, nil
}

func (s *notificationService) MarkRead(ctx context.Context, userID int, id int, req models.MarkNotificationReadRequest) (*models.Notification, error) {
	n, err := s.findOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	if err := s.db.WithContext(ctx).Model(n).Update("is_read", req.IsRead).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating notification")
	}

	n.IsRead = req.IsRead
	return n, nil
}

// MarkAllRead -- один UPDATE по user_id AND is_read=false, без выборки в память.
func (s *notificationService) MarkAllRead(ctx context.Context, userID int) (int64, error) {
	result := s.db.WithContext(ctx).Model(&models.Notification{}).
		Where("user_id = ? AND is_read = ?", userID, false).
		Update("is_read", true)
	if result.Error != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error marking notifications read")
	}
	return result.RowsAffected, nil
}

func (s *notificationService) Delete(ctx context.Context, userID int, id int) error {
	_, err := s.findOwned(ctx, userID, id)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Delete(&models.Notification{}, id).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting notification")
	}
	return nil
}

func (s *notificationService) DeleteAll(ctx context.Context, userID int) error {
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.Notification{}).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting notifications")
	}
	return nil
}

// Create создаёт уведомление (admin endpoint + внутренние триггеры). В отличие от
// CreateForUser* НЕ проходит гейт подписки -- ручная рассылка администратора
// доставляется всегда, её адресат не выбирал отключить именно этот тип.
func (s *notificationService) Create(ctx context.Context, req models.CreateNotificationRequest) (*models.Notification, error) {
	if req.UserID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "user_id is required")
	}
	if req.Title == nil || *req.Title == "" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "title is required")
	}
	notifType := ""
	if req.Type != nil {
		notifType = *req.Type
	}
	deliverable, err := s.recipientAcceptsNotifications(ctx, req.UserID, notifType)
	if err != nil {
		return nil, err
	}
	// Отправку вручную отклоняем с ошибкой, а не тихо: администратор должен увидеть,
	// почему уведомление не ушло, - в отличие от фоновых триггеров, где адресат
	// отсеивается молча.
	if !deliverable {
		return nil, echo.NewHTTPError(http.StatusConflict, "recipient is banned or inactive")
	}
	n := models.Notification{
		UserID:  req.UserID,
		Type:    req.Type,
		Title:   req.Title,
		Message: req.Message,
		Data:    req.Data,
	}
	if err := s.db.WithContext(ctx).Create(&n).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error creating notification")
	}
	// Тот же real-time сигнал, что и в CreateForUser (#840): admin-эндпоинт и
	// внутренние триггеры создают уведомление адресно req.UserID - доставляем
	// мгновенно, а не через 30с-поллинг. best-effort, на возврат не влияет.
	if s.realtimePublisher != nil {
		s.realtimePublisher.Publish(n.UserID, realtime.Event{Type: "notification.new", Scope: "notifications"})
	}
	s.sendPush(ctx, &n)
	return &n, nil
}

// CreateForUser -- helper для триггеров из других сервисов. Ключ схлопывания
// вычисляется автоматически по каталогу и содержимому data (см. autoNotificationGroupKey).
// Ошибки логируются но не прерывают основной flow (уведомления не должны блокировать
// бизнес-операции) -- это ответственность вызывающей стороны, сам метод ошибку возвращает.
func (s *notificationService) CreateForUser(ctx context.Context, userID int, notifType, title, message string, data *string) error {
	return s.CreateForUserGrouped(ctx, userID, notifType, title, message, data, autoNotificationGroupKey(notifType, data))
}

// CreateForUserGrouped -- общая реализация за CreateForUser и будущими вызовами S3-S5 со
// своим ключом. Порядок: гейт подписки -> попытка схлопнуть в существующую запись группы
// -> обычное создание, если схлопнуть не удалось (группы ещё нет, окно истекло, либо
// прежняя запись успела стать прочитанной).
func (s *notificationService) CreateForUserGrouped(ctx context.Context, userID int, notifType, title, message string, data *string, groupKey string) error {
	if userID <= 0 || title == "" {
		return nil
	}

	deliverable, err := s.recipientAcceptsNotifications(ctx, userID, notifType)
	if err != nil {
		return err
	}
	if !deliverable {
		return nil
	}

	allowed, err := s.notificationAllowed(ctx, userID, notifType)
	if err != nil {
		return err
	}
	if !allowed {
		return nil
	}

	if groupKey != "" {
		collapsedID, collapsed, err := s.collapseNotification(ctx, userID, notifType, groupKey, message, data)
		if err != nil {
			return err
		}
		if collapsed {
			if s.realtimePublisher != nil {
				s.realtimePublisher.Publish(userID, realtime.Event{Type: "notification.new", Scope: "notifications"})
			}
			nt, ti, m := notifType, title, message
			s.sendPush(ctx, &models.Notification{ID: collapsedID, UserID: userID, Type: &nt, Title: &ti, Message: &m, Data: data})
			return nil
		}
		// RowsAffected == 0 - падаем в обычное создание ниже, оно заведёт новую запись
		// с тем же group_key (следующее событие схлопнется уже в неё).
	}

	t, ti, m := notifType, title, message
	n := models.Notification{
		UserID:  userID,
		Type:    &t,
		Title:   &ti,
		Message: &m,
		Data:    data,
	}
	if groupKey != "" {
		gk := groupKey
		n.GroupKey = &gk
	}
	if err := s.db.WithContext(ctx).Create(&n).Error; err != nil {
		return err
	}
	if s.realtimePublisher != nil {
		s.realtimePublisher.Publish(userID, realtime.Event{Type: "notification.new", Scope: "notifications"})
	}
	s.sendPush(ctx, &n)
	return nil
}

// collapseNotification пытается схлопнуть событие в существующую непрочитанную запись той
// же группы не старше notificationAggregationWindow. Один UPDATE с условием прямо в
// запросе (не SELECT, потом UPDATE): RowsAffected==0 значит группы ещё нет, окно истекло,
// либо прежняя запись стала прочитанной между вычислением ключа и записью -- в любом из
// этих случаев вызывающая сторона обязана упасть в обычное создание, а не потерять событие.
// Возвращает id схлопнутой записи (Returning) -- push поверх схлопнутого события всё
// равно ссылается на конкретное уведомление, а не только на факт обновления счётчика.
func (s *notificationService) collapseNotification(ctx context.Context, userID int, notifType, groupKey, message string, data *string) (int, bool, error) {
	cutoff := time.Now().Add(-notificationAggregationWindow)
	var row models.Notification
	result := s.db.WithContext(ctx).Model(&row).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
		Where("user_id = ? AND type = ? AND group_key = ? AND is_read = ? AND COALESCE(last_event_at, created_at) >= ?",
			userID, notifType, groupKey, false, cutoff).
		Updates(map[string]any{
			"count":         gorm.Expr("count + 1"),
			"last_event_at": time.Now(),
			"message":       message,
			"data":          data,
		})
	if result.Error != nil {
		return 0, false, echo.NewHTTPError(http.StatusInternalServerError, "Error collapsing notification")
	}
	return row.ID, result.RowsAffected > 0, nil
}

// sendPush -- push поверх уже созданного или схлопнутого уведомления (#974), в тех же
// точках, где публикуется real-time сигнал notification.new: гейт подписки
// (notificationAllowed) уже пройден выше по CreateForUserGrouped, отдельного набора
// "что слать в push" эта функция не заводит. Нет-op, если push не подключён (nil-safe).
func (s *notificationService) sendPush(ctx context.Context, n *models.Notification) {
	if s.pushSender == nil {
		return
	}
	title, message, notifType := "", "", ""
	if n.Title != nil {
		title = *n.Title
	}
	if n.Message != nil {
		message = *n.Message
	}
	if n.Type != nil {
		notifType = *n.Type
	}
	payload := PushPayload{Title: title, Message: message, Type: notifType, NotificationID: n.ID}
	if p := parseNotificationDataPayload(n.Data); p != nil {
		if id, ok := notificationPayloadInt(p, "application_id"); ok {
			payload.ApplicationID = &id
		}
	}
	s.pushSender.Send(ctx, n.UserID, payload)
}

// recipientAcceptsNotifications отсекает получателей, отключённых от системы:
// заблокированных (is_banned) и архивных (is_active = false). Подписка тут ни при чём -
// человек не выключал уведомления, это система перестала его обслуживать.
//
// Проверка нужна отдельно от подписки, потому что web push живёт независимо от сессии:
// заблокированный в систему не войдёт, но подписка его браузера остаётся рабочей, и
// уведомления продолжали приходить ему на устройство после отключения учётной записи.
//
// Исключение одно - сообщение о самой блокировке. UserBanService заводит его уже ПОСЛЕ
// того, как флаг проставлен, и без исключения человек не узнал бы, почему его выкинуло.
//
// Пользователя нет (удалён между событием и доставкой) - доставлять некому; это не
// ошибка вызывающего, поэтому возвращаем отказ, а не 500.
func (s *notificationService) recipientAcceptsNotifications(ctx context.Context, userID int, notifType string) (bool, error) {
	if notifType == NotificationTypeUserBanned {
		return true, nil
	}

	var user models.User
	err := s.db.WithContext(ctx).Select("is_banned, is_active").First(&user, userID).Error
	switch {
	case err == nil:
		return !user.IsBanned && user.IsActive, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return false, nil
	default:
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Error checking notification recipient")
	}
}

// notificationAllowed решает, доставлять ли уведомление CreateForUser*. Mandatory-типы
// идут всегда (иначе пользователь не узнает о блокировке собственной учётной записи), тип
// вне каталога тоже идёт всегда, но с предупреждением в лог (неизвестный код не должен
// пропасть молча). Остальные -- по персональной подписке с дефолтом каталога, если строки
// override нет.
func (s *notificationService) notificationAllowed(ctx context.Context, userID int, notifType string) (bool, error) {
	meta, ok := NotificationTypeMeta(notifType)
	if !ok {
		slog.Warn("уведомление типа вне каталога - доставляется без проверки подписки", "type", notifType)
		return true, nil
	}
	if meta.Mandatory {
		return true, nil
	}

	var pref models.UserNotificationPreference
	err := s.db.WithContext(ctx).Where("user_id = ? AND type_code = ?", userID, notifType).First(&pref).Error
	switch {
	case err == nil:
		return pref.Enabled, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return meta.DefaultEnabled, nil
	default:
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Error checking notification preference")
	}
}

// autoNotificationGroupKey вычисляет ключ схлопывания для CreateForUser по коду типа и
// телу data (#1748). application_answer группируется по вопросу (question:<question_id>,
// см. questionNotificationPayload в application_question_service.go), остальные
// Aggregatable-типы с application_id в data - по заявке (app:<application_id>). Пустой
// результат -- тип не Aggregatable либо в data не нашлось подходящего поля: нормальная
// ветка "схлопывания нет", не ошибка.
func autoNotificationGroupKey(notifType string, data *string) string {
	meta, ok := NotificationTypeMeta(notifType)
	if !ok || !meta.Aggregatable {
		return ""
	}

	payload := parseNotificationDataPayload(data)
	if payload == nil {
		return ""
	}

	if notifType == NotificationTypeApplicationAnswer {
		if id, ok := notificationPayloadInt(payload, "question_id"); ok {
			return fmt.Sprintf("question:%d", id)
		}
		return ""
	}

	if id, ok := notificationPayloadInt(payload, "application_id"); ok {
		return fmt.Sprintf("app:%d", id)
	}

	return ""
}

func parseNotificationDataPayload(data *string) map[string]any {
	if data == nil || *data == "" {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(*data), &payload); err != nil {
		return nil
	}
	return payload
}

// notificationPayloadInt читает целочисленное поле из распарсенного JSON: json.Unmarshal
// в map[string]any всегда кладёт числа как float64, других вариантов у payload'ов
// уведомлений (json.Marshal(map[string]any{...int...})) не бывает.
func notificationPayloadInt(payload map[string]any, key string) (int, bool) {
	v, ok := payload[key]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	if !ok {
		return 0, false
	}
	return int(f), true
}

// GetPreferences -- каталог целиком по категориям, с эффективным состоянием (override,
// иначе дефолт каталога; Mandatory всегда true) для этого пользователя.
func (s *notificationService) GetPreferences(ctx context.Context, userID int) ([]models.NotificationPreferenceCategory, error) {
	var prefs []models.UserNotificationPreference
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&prefs).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching notification preferences")
	}
	overrides := make(map[string]bool, len(prefs))
	for _, p := range prefs {
		overrides[p.TypeCode] = p.Enabled
	}

	// Гейт по правам: тип, для которого у человека нет права, на экран не попадает.
	// Заявителю незачем настраивать уведомления о заполнении файлового архива или о
	// новых обращениях обратной связи - он их всё равно не получит, а переключатель
	// создаёт впечатление, что получит. Резолвер не подключён (тесты, offline) -
	// показываем всё, кроме скрытых: лучше лишний переключатель, чем пустой экран.
	var granted PermissionSet
	haveGranted := false
	if s.permissionResolver != nil {
		set, err := s.permissionResolver.Resolve(ctx, userID)
		if err != nil {
			slog.Warn("настройки уведомлений: не удалось резолвить права", "user_id", userID, "error", err)
		} else {
			granted, haveGranted = set, true
		}
	}

	byCategory := make(map[NotificationCategory][]models.NotificationPreferenceItem)
	for _, meta := range NotificationCatalog() {
		if meta.HiddenInSettings {
			continue
		}
		if meta.Permission != "" && haveGranted && !granted.Has(meta.Permission) {
			continue
		}
		enabled := meta.DefaultEnabled
		if v, ok := overrides[meta.Code]; ok {
			enabled = v
		}
		if meta.Mandatory {
			enabled = true
		}
		byCategory[meta.Category] = append(byCategory[meta.Category], models.NotificationPreferenceItem{
			TypeCode:       meta.Code,
			Category:       string(meta.Category),
			Label:          meta.Label,
			Description:    meta.Description,
			Mandatory:      meta.Mandatory,
			DefaultEnabled: meta.DefaultEnabled,
			Enabled:        enabled,
		})
	}

	out := make([]models.NotificationPreferenceCategory, 0, len(notificationCategoryOrder))
	for _, cat := range NotificationCategories() {
		// Категория, из которой права и скрытие вымели все типы, на экран не идёт -
		// иначе останется пустой заголовок без единой строки.
		if len(byCategory[cat]) == 0 {
			continue
		}
		out = append(out, models.NotificationPreferenceCategory{
			Category: string(cat),
			Items:    byCategory[cat],
		})
	}
	return out, nil
}

// UpdatePreferences сохраняет батч одним upsert-ом (эталон - buildFieldConfigRows /
// attachmentFieldConfigService.Save): валидация всех строк проходит ДО записи, поэтому
// невалидная строка в батче не оставляет частично применённых изменений. Дедуп
// last-wins -- PUT идемпотентен, повтор кода в одном батче иначе уронил бы PG "ON
// CONFLICT cannot affect row a second time".
func (s *notificationService) UpdatePreferences(ctx context.Context, userID int, items []models.NotificationPreferenceItemUpdate) error {
	if len(items) == 0 {
		return nil
	}

	byCode := make(map[string]models.NotificationPreferenceItemUpdate, len(items))
	order := make([]string, 0, len(items))
	for _, item := range items {
		if _, seen := byCode[item.TypeCode]; !seen {
			order = append(order, item.TypeCode)
		}
		byCode[item.TypeCode] = item
	}

	rows := make([]models.UserNotificationPreference, 0, len(order))
	for _, code := range order {
		item := byCode[code]
		meta, ok := NotificationTypeMeta(item.TypeCode)
		if !ok {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("неизвестный тип уведомления %q", item.TypeCode))
		}
		if meta.Mandatory && !item.Enabled {
			return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("тип уведомления %q нельзя отключить", item.TypeCode))
		}
		rows = append(rows, models.UserNotificationPreference{
			UserID:   userID,
			TypeCode: item.TypeCode,
			Enabled:  item.Enabled,
		})
	}

	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "type_code"}},
		DoUpdates: clause.AssignmentColumns([]string{"enabled", "updated_at"}),
	}).Create(&rows).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error saving notification preferences")
	}
	return nil
}
