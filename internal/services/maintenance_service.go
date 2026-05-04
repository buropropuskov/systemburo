package services

import (
	"context"
	"errors"
	"fmt"
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
type MaintenanceStatus struct {
	Enabled      bool   `json:"enabled"`
	Message      string `json:"message,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	SupportEmail string `json:"support_email,omitempty"`
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
	Enable(ctx context.Context, adminUserID int, adminUsername, message, supportEmail string) error
	Disable(ctx context.Context, adminUserID int, adminUsername string) error
	// InvalidateCache сбрасывает in-memory кэш GetStatusCached. Вызывается
	// внутри Enable/Disable - чтобы свежий флаг применился сразу.
	InvalidateCache()
}

type maintenanceService struct {
	db *gorm.DB

	mu         sync.RWMutex
	cache      *MaintenanceStatus
	cachedAt   time.Time
	cacheTTL   time.Duration
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
	keyMaintenanceMessage      = "maintenance.message"
	keyMaintenanceSupportEmail = "maintenance.support_email"
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
	return &MaintenanceStatus{
		Enabled:      s.readSetting(ctx, keyMaintenanceEnabled) == "true",
		Message:      s.readSetting(ctx, keyMaintenanceMessage),
		StartedAt:    s.readSetting(ctx, keyMaintenanceStartedAt),
		SupportEmail: s.readSetting(ctx, keyMaintenanceSupportEmail),
	}, nil
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

// Enable — транзакция: 4 UPSERT setting + UPDATE refresh_tokens для
// non-admin юзеров (типы != 6) + audit_event. Так же сбрасываем кэш
// чтобы middleware увидел новый флаг сразу.
func (s *maintenanceService) Enable(ctx context.Context, adminUserID int, adminUsername, message, supportEmail string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := upsertSetting(tx, keyMaintenanceEnabled, "true"); err != nil {
			return err
		}
		if err := upsertSetting(tx, keyMaintenanceStartedAt, now); err != nil {
			return err
		}
		if err := upsertSetting(tx, keyMaintenanceMessage, message); err != nil {
			return err
		}
		if err := upsertSetting(tx, keyMaintenanceSupportEmail, supportEmail); err != nil {
			return err
		}
		// Отзываем refresh-токены всех не-админов. Через ≤15 мин access
		// истечёт и их выкинет на /login, где получат 503 → /maintenance.
		if err := tx.Exec(`
			UPDATE refresh_tokens
			SET is_revoked = true
			WHERE user_id IN (SELECT id FROM users WHERE type_id != 6)
			  AND is_revoked = false
		`).Error; err != nil {
			return fmt.Errorf("revoke non-admin refresh tokens: %w", err)
		}
		auditUserID := &adminUserID
		return tx.Create(&models.AuthEvent{
			UserID:    auditUserID,
			Username:  adminUsername,
			EventType: "maintenance_enabled",
			Success:   true,
			CreatedAt: time.Now().UTC(),
		}).Error
	})
	if err != nil {
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
			EventType: "maintenance_disabled",
			Success:   true,
			CreatedAt: time.Now().UTC(),
		}).Error
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "не удалось выключить режим обслуживания")
	}
	s.InvalidateCache()
	return nil
}
