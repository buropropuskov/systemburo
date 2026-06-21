package services

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UserBanService отвечает за блокировку и разблокировку пользователей.
// При бане отзываются все активные refresh-токены, чтобы юзер потерял
// доступ на следующем `/refresh` -- это работает в связке с frontend
// polling по `/me/permissions`.
type UserBanService struct {
	db       *gorm.DB
	resolver *PermissionResolver
	banCache *BanCheckService
}

// NewUserBanService конструирует сервис. banCache опционален: если nil,
// инвалидация ban-кэша пропускается (полезно для тестов).
func NewUserBanService(db *gorm.DB, resolver *PermissionResolver, banCache *BanCheckService) *UserBanService {
	return &UserBanService{db: db, resolver: resolver, banCache: banCache}
}

// Ban блокирует пользователя и отзывает его активные refresh-токены.
// Запрещено блокировать самого себя и super-admin (защита от самоудаления).
// reason -- причина блокировки (показывается заблокированному в ЛК); пустая -> NULL.
func (s *UserBanService) Ban(ctx context.Context, targetUserID, actorUserID int, reason string) error {
	if targetUserID == actorUserID {
		return fmt.Errorf("cannot ban yourself")
	}
	var target models.User
	if err := s.db.WithContext(ctx).Select("id, is_super_admin").First(&target, targetUserID).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	if target.IsSuperAdmin {
		return echo.NewHTTPError(http.StatusForbidden, "Нельзя заблокировать супер-администратора")
	}

	var banReason *string
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		banReason = &trimmed
	}

	now := time.Now()
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", targetUserID).
			Updates(map[string]any{
				"is_banned":  true,
				"banned_at":  now,
				"banned_by":  actorUserID,
				"ban_reason": banReason,
			}).Error; err != nil {
			return fmt.Errorf("update user: %w", err)
		}
		// Отзываем все активные refresh-токены: следующий /refresh даст 401 -> логаут.
		if err := tx.Model(&models.RefreshToken{}).
			Where("user_id = ? AND is_revoked = ?", targetUserID, false).
			Updates(map[string]any{"is_revoked": true, "revoked_at": now}).Error; err != nil {
			return fmt.Errorf("revoke refresh tokens: %w", err)
		}
		return nil
	})
	if err == nil {
		s.resolver.Invalidate(targetUserID)
		if s.banCache != nil {
			s.banCache.Invalidate(targetUserID)
		}
	}
	return err
}

// Unban разблокирует пользователя. Refresh-токены остаются revoked --
// юзеру нужно перелогиниться (это нормально, т.к. в момент бана сессия
// прервалась).
func (s *UserBanService) Unban(ctx context.Context, targetUserID int) error {
	if err := s.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", targetUserID).
		Updates(map[string]any{
			"is_banned":  false,
			"banned_at":  nil,
			"banned_by":  nil,
			"ban_reason": nil,
		}).Error; err != nil {
		return fmt.Errorf("unban: %w", err)
	}
	s.resolver.Invalidate(targetUserID)
	if s.banCache != nil {
		s.banCache.Invalidate(targetUserID)
	}
	return nil
}
