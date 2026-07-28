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
}

// NewMaintenanceService — конструктор. cacheTTL для in-memory кэша статуса
// (10 сек в prod).
func NewMaintenanceService(db *gorm.DB) MaintenanceService {
	return &maintenanceService{
		db:       db,
		cacheTTL: 10 * time.Second,
	}
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
