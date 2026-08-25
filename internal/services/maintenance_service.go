package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// MaintenanceStatus — публичное состояние режима "Технические работы".
// Используется фронтом на /maintenance (авто-рефреш) и /login (решить,
// показывать ли форму логина для обычного юзера).
//
// StartedAt — момент фактического включения, PlannedStart/PlannedEnd — окно,
// объявленное пользователям. Окно опционально: пустые значения означают
// работы без объявленного срока.
type MaintenanceStatus struct {
	Enabled      bool   `json:"enabled"`
	Message      string `json:"message,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	PlannedStart string `json:"planned_start,omitempty"`
	PlannedEnd   string `json:"planned_end,omitempty"`
	SupportEmail string `json:"support_email,omitempty"`
	SupportPhone string `json:"support_phone,omitempty"`
}

// MaintenanceParams — что супер-админ задаёт при включении режима.
// Даты приходят уже нормализованными в RFC3339 UTC (разбор в handler).
type MaintenanceParams struct {
	Message      string
	PlannedStart string
	PlannedEnd   string
	SupportEmail string
	SupportPhone string
}

// MaintenanceService включает/выключает режим технических работ и отдаёт
// текущий статус. Enable: транзакция (settings + revoke non-admin refresh +
// audit). Для защиты от регулярных запросов middleware использует
// GetStatusCached с 10-сек in-memory кэшем.
type MaintenanceService interface {
	GetStatus(ctx context.Context) (*MaintenanceStatus, error)
	// GetStatusCached — тот же GetStatus, но с in-memory кэшем. Используется
	// middleware чтобы не бить БД на каждом запросе.
	GetStatusCached(ctx context.Context) *MaintenanceStatus
	Enable(ctx context.Context, adminUserID int, adminUsername string, params MaintenanceParams) error
	Disable(ctx context.Context, adminUserID int, adminUsername string) error
	// InvalidateCache сбрасывает in-memory кэш GetStatusCached. Вызывается
	// внутри Enable/Disable - чтобы свежий флаг применился сразу.
	InvalidateCache()
}

type maintenanceService struct {
	db *gorm.DB

	mu       sync.RWMutex
	cache    *MaintenanceStatus
	cachedAt time.Time
	cacheTTL time.Duration

	notificationService NotificationService
}

// MaintenanceServiceOption конфигурирует maintenanceService при создании.
type MaintenanceServiceOption func(*maintenanceService)

// WithMaintenanceNotifications включает уведомление maintenance_scheduled (#1748)
// при объявлении окна плановых работ. Опционально: без неё уведомления не шлются
// (тесты, offline).
func WithMaintenanceNotifications(ns NotificationService) MaintenanceServiceOption {
	return func(s *maintenanceService) { s.notificationService = ns }
}

// NewMaintenanceService — конструктор. cacheTTL для in-memory кэша статуса
// (10 сек в prod).
func NewMaintenanceService(db *gorm.DB, opts ...MaintenanceServiceOption) MaintenanceService {
	s := &maintenanceService{
		db:       db,
		cacheTTL: 10 * time.Second,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

const (
	keyMaintenanceEnabled      = "maintenance.enabled"
	keyMaintenanceStartedAt    = "maintenance.started_at"
	keyMaintenancePlannedStart = "maintenance.planned_start"
	keyMaintenancePlannedEnd   = "maintenance.planned_end"
	keyMaintenanceMessage      = "maintenance.message"
	keyMaintenanceSupportEmail = "maintenance.support_email"
	keyMaintenanceSupportPhone = "maintenance.support_phone"
)

const (
	eventMaintenanceEnabled      = "maintenance_enabled"
	eventMaintenanceDisabled     = "maintenance_disabled"
	eventMaintenanceAutoDisabled = "maintenance_auto_disabled"
)

// readSetting возвращает значение настройки из БД или пустую строку.
func (s *maintenanceService) readSetting(ctx context.Context, key string) string {
	var setting models.SystemSetting
	if err := s.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
		return ""
	}
	return setting.Value
}

func (s *maintenanceService) GetStatus(ctx context.Context) (*MaintenanceStatus, error) {
	st := &MaintenanceStatus{
		Enabled:      s.readSetting(ctx, keyMaintenanceEnabled) == "true",
		Message:      s.readSetting(ctx, keyMaintenanceMessage),
		StartedAt:    s.readSetting(ctx, keyMaintenanceStartedAt),
		PlannedStart: s.readSetting(ctx, keyMaintenancePlannedStart),
		PlannedEnd:   s.readSetting(ctx, keyMaintenancePlannedEnd),
		SupportEmail: s.readSetting(ctx, keyMaintenanceSupportEmail),
		SupportPhone: s.readSetting(ctx, keyMaintenanceSupportPhone),
	}
	if st.Enabled && windowExpired(st.PlannedEnd, time.Now().UTC()) {
		s.autoDisable(ctx)
		st.Enabled = false
	}
	return st, nil
}

// windowExpired — объявленное окно работ уже закончилось. Пустое или
// неразбираемое значение авто-снятия не вызывает: режим гасится вручную.
func windowExpired(plannedEnd string, now time.Time) bool {
	if plannedEnd == "" {
		return false
	}
	end, err := time.Parse(time.RFC3339, plannedEnd)
	if err != nil {
		return false
	}
	return now.After(end)
}

// autoDisable снимает режим по истечении объявленного окна - страховка на
// случай, когда админ забыл выключить режим или потерял доступ к системе.
// Условие value = 'true' делает переключение однократным: из параллельных
// запросов строку меняет ровно один, он же пишет событие аудита.
func (s *maintenanceService) autoDisable(ctx context.Context) {
	res := s.db.WithContext(ctx).Exec(
		`UPDATE system_settings SET value = 'false' WHERE key = ? AND value = 'true'`,
		keyMaintenanceEnabled)
	if res.Error != nil {
		slog.Error("maintenance auto-disable failed", "error", res.Error)
		return
	}
	if res.RowsAffected == 0 {
		return
	}
	s.InvalidateCache()
	event := &models.AuthEvent{
		EventType: eventMaintenanceAutoDisabled,
		Success:   true,
		Detail:    "истекло объявленное окно технических работ",
		CreatedAt: time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx).Create(event).Error; err != nil {
		slog.Error("maintenance auto-disable audit failed", "error", err)
	}
	slog.Info("maintenance mode auto-disabled: planned window expired")
}

func (s *maintenanceService) GetStatusCached(ctx context.Context) *MaintenanceStatus {
	s.mu.RLock()
	if s.cache != nil && time.Since(s.cachedAt) < s.cacheTTL {
		st := *s.cache
		s.mu.RUnlock()
		return &st
	}
	s.mu.RUnlock()

	st, err := s.GetStatus(ctx)
	if err != nil {
		// На ошибке возвращаем "не в maintenance" чтобы не положить сайт.
		return &MaintenanceStatus{Enabled: false}
	}
	s.mu.Lock()
	s.cache = st
	s.cachedAt = time.Now().UTC()
	s.mu.Unlock()
	return st
}

func (s *maintenanceService) InvalidateCache() {
	s.mu.Lock()
	s.cache = nil
	s.cachedAt = time.Time{}
	s.mu.Unlock()
}

// upsertSetting — INSERT или UPDATE по unique(key).
func upsertSetting(tx *gorm.DB, key, value string) error {
	var existing models.SystemSetting
	if err := tx.Where("key = ?", key).First(&existing).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.Create(&models.SystemSetting{Key: key, Value: value, Type: "string"}).Error
	}
	existing.Value = value
	return tx.Save(&existing).Error
}

// notifyMaintenanceScheduled уведомляет активных пользователей об объявленном окне
// плановых технических работ (#1748). Enable() в этом сервисе всегда включает режим
// немедленно (keyMaintenanceEnabled='true' пишется в той же транзакции, отдельного
// "запланировать на будущее без включения сейчас" пути нет) - поэтому шлём
// уведомление в момент задания окна, а не отдельным флагом "запланировано". Без
// окна (params.PlannedStart/PlannedEnd пусты) не шлём: пользователи и так упрутся
// в заглушку техработ без объявленного срока, сообщать нечего. Суперадминов не
// исключаем - только самого включившего (он и так знает).
func (s *maintenanceService) notifyMaintenanceScheduled(ctx context.Context, adminUserID int, params MaintenanceParams) {
	if s.notificationService == nil || params.PlannedStart == "" || params.PlannedEnd == "" {
		return
	}
	start, err := time.Parse(time.RFC3339, params.PlannedStart)
	if err != nil {
		slog.Warn("техработы: некорректное начало окна, уведомление не отправлено", "err", err)
		return
	}
	end, err := time.Parse(time.RFC3339, params.PlannedEnd)
	if err != nil || !end.After(start) {
		slog.Warn("техработы: некорректное окончание окна, уведомление не отправлено", "err", err)
		return
	}

	title := "Плановые технические работы"
	body := fmt.Sprintf("Технические работы начнутся %s (МСК) и продлятся %s.",
		start.In(AnalyticsLocation()).Format("02.01.2006 15:04"), formatMaintenanceDuration(end.Sub(start)))

	ids, err := activeUserIDs(ctx, s.db)
	if err != nil {
		slog.Warn("техработы: не удалось собрать аудиторию уведомления", "err", err)
		return
	}
	for _, uid := range ids {
		if uid == adminUserID {
			continue
		}
		if err := s.notificationService.CreateForUser(ctx, uid, NotificationTypeMaintenanceScheduled, title, body, nil); err != nil {
			slog.Warn("не удалось уведомить о плановых технических работах", "user_id", uid, "error", err)
		}
	}
}

// formatMaintenanceDuration форматирует длительность окна работ короткой русской
// фразой без склонений ("2 ч 30 мин", "45 мин") - обходит согласование "час/часа/часов".
func formatMaintenanceDuration(d time.Duration) string {
	if d < time.Minute {
		return "менее минуты"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	switch {
	case hours > 0 && minutes > 0:
		return fmt.Sprintf("%d ч %d мин", hours, minutes)
	case hours > 0:
		return fmt.Sprintf("%d ч", hours)
	default:
		return fmt.Sprintf("%d мин", minutes)
	}
}

// Enable — транзакция: UPSERT настроек режима + UPDATE refresh_tokens для
// не-супер-админов + audit_event. Так же сбрасываем кэш чтобы middleware
// увидел новый флаг сразу.
func (s *maintenanceService) Enable(ctx context.Context, adminUserID int, adminUsername string, params MaintenanceParams) error {
	now := time.Now().UTC().Format(time.RFC3339)
	settings := []struct{ key, value string }{
		{keyMaintenanceEnabled, "true"},
		{keyMaintenanceStartedAt, now},
		{keyMaintenanceMessage, params.Message},
		{keyMaintenancePlannedStart, params.PlannedStart},
		{keyMaintenancePlannedEnd, params.PlannedEnd},
		{keyMaintenanceSupportEmail, params.SupportEmail},
		{keyMaintenanceSupportPhone, params.SupportPhone},
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, setting := range settings {
			if err := upsertSetting(tx, setting.key, setting.value); err != nil {
				return fmt.Errorf("upsert %s: %w", setting.key, err)
			}
		}
		// Отзываем refresh-токены всех не-супер-админов. Через ≤15 мин access
		// истечёт и их выкинет на /login, где получат 503 → /maintenance.
		if err := tx.Exec(`
			UPDATE refresh_tokens
			SET is_revoked = true
			WHERE user_id IN (SELECT id FROM users WHERE is_super_admin = false)
			  AND is_revoked = false
		`).Error; err != nil {
			return fmt.Errorf("revoke non-admin refresh tokens: %w", err)
		}
		auditUserID := &adminUserID
		return tx.Create(&models.AuthEvent{
			UserID:    auditUserID,
			Username:  adminUsername,
			EventType: eventMaintenanceEnabled,
			Success:   true,
			CreatedAt: time.Now().UTC(),
		}).Error
	})
	if err != nil {
		slog.Error("enable maintenance failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "не удалось включить режим обслуживания")
	}
	s.InvalidateCache()
	s.notifyMaintenanceScheduled(ctx, adminUserID, params)
	return nil
}

func (s *maintenanceService) Disable(ctx context.Context, adminUserID int, adminUsername string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertSetting(tx, keyMaintenanceEnabled, "false"); err != nil {
			return err
		}
		auditUserID := &adminUserID
		return tx.Create(&models.AuthEvent{
			UserID:    auditUserID,
			Username:  adminUsername,
			EventType: eventMaintenanceDisabled,
			Success:   true,
			CreatedAt: time.Now().UTC(),
		}).Error
	})
	if err != nil {
		slog.Error("disable maintenance failed", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "не удалось выключить режим обслуживания")
	}
	s.InvalidateCache()
	return nil
}
