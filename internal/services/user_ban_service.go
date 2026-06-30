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
	recorder AuditRecorder
}

// NewUserBanService конструирует сервис. banCache опционален: если nil,
// инвалидация ban-кэша пропускается (полезно для тестов). recorder пишет
// историю блокировок в audit_log[user] рядом с остальными действиями юзера.
func NewUserBanService(db *gorm.DB, resolver *PermissionResolver, banCache *BanCheckService, recorder AuditRecorder) *UserBanService {
	return &UserBanService{db: db, resolver: resolver, banCache: banCache, recorder: recorder}
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

	trimmedReason := strings.TrimSpace(reason)
	var banReason *string
	if trimmedReason != "" {
		banReason = &trimmedReason
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
		// История блокировки -- в audit_log[user] (fire-and-forget: провал записи
		// не должен откатывать сам бан). reason тот же trimmed, что и в users.ban_reason.
		s.recorder.Log(ctx, nil, models.AuditEntityUser, &targetUserID, models.UserActionBanned, &actorUserID,
			map[string]any{"reason": trimmedReason})
	}
	return err
}

// Unban разблокирует пользователя. Refresh-токены остаются revoked --
// юзеру нужно перелогиниться (это нормально, т.к. в момент бана сессия
// прервалась).
func (s *UserBanService) Unban(ctx context.Context, targetUserID, actorUserID int) error {
	// Снимок текущей блокировки ДО очистки полей: нужен, чтобы записать в историю
	// снятую причину и момент начала блокировки -- по ним модалка показывает
	// "был в блокировке <срок>, причина: ...".
	var prev models.User
	hasPrev := s.db.WithContext(ctx).Select("ban_reason, banned_at").First(&prev, targetUserID).Error == nil

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
	// Разблокировка в историю: кто/когда + снятая причина и начало блокировки.
	details := map[string]any{}
	if hasPrev {
		if prev.BanReason != nil {
			details["reason"] = *prev.BanReason
		}
		if prev.BannedAt != nil {
			details["banned_at"] = prev.BannedAt.UTC().Format(time.RFC3339)
		}
	}
	var d any
	if len(details) > 0 {
		d = details
	}
	s.recorder.Log(ctx, nil, models.AuditEntityUser, &targetUserID, models.UserActionUnbanned, &actorUserID, d)
	return nil
}
