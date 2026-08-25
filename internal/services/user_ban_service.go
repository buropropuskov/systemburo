package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/realtime"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UserBanService отвечает за блокировку и разблокировку пользователей.
// При бане отзываются все активные refresh-токены, чтобы юзер потерял
// доступ на следующем `/refresh` -- это работает в связке с frontend
// polling по `/me/permissions`.
type UserBanService struct {
	db                  *gorm.DB
	resolver            *PermissionResolver
	banCache            *BanCheckService
	recorder            AuditRecorder
	realtimePublisher   realtime.Publisher
	notificationService NotificationService
}

// UserBanServiceOption конфигурирует UserBanService при создании.
type UserBanServiceOption func(*UserBanService)

// WithBanRealtimePublisher включает real-time сигнал user.banned/user.unbanned
// адресно заблокированному (#840): его App.vue мгновенно перезапрашивает права,
// баннер ЧС всплывает и UI блокируется без ожидания 30с-опроса /me/permissions.
// Опционально, best-effort nil-safe: только сигнал, на сам бан не влияет.
func WithBanRealtimePublisher(p realtime.Publisher) UserBanServiceOption {
	return func(s *UserBanService) { s.realtimePublisher = p }
}

// WithBanNotifications подключает персистентные уведомления о блокировке и
// разблокировке учётки (#1748 S3). Опционально, nil-safe: без неё уведомления
// просто не создаются (тесты, offline) - на сам бан не влияет.
func WithBanNotifications(ns NotificationService) UserBanServiceOption {
	return func(s *UserBanService) { s.notificationService = ns }
}

// NewUserBanService конструирует сервис. banCache опционален: если nil,
// инвалидация ban-кэша пропускается (полезно для тестов). recorder пишет
// историю блокировок в audit_log[user] рядом с остальными действиями юзера.
func NewUserBanService(db *gorm.DB, resolver *PermissionResolver, banCache *BanCheckService, recorder AuditRecorder, opts ...UserBanServiceOption) *UserBanService {
	s := &UserBanService{db: db, resolver: resolver, banCache: banCache, recorder: recorder}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// notifyBanChanged шлёт заблокированному/разблокированному адресный сигнал
// (scope user:<id>) - его App.vue перезапросит права. Best-effort, nil-safe.
// Звать ПОСЛЕ успешного commit бана/разбана.
func (s *UserBanService) notifyBanChanged(targetUserID int, eventType string) {
	if s.realtimePublisher == nil {
		return
	}
	s.realtimePublisher.Publish(targetUserID, realtime.Event{
		Type:  eventType,
		Scope: fmt.Sprintf("user:%d", targetUserID),
	})
}

// notifyBanned создаёт персистентное уведомление владельцу заблокированной учётки
// (#1748 S3). Отдельно от notifyBanChanged (тот только толкает real-time сигнал
// перечитать права #840) - эта запись остаётся в ленте уведомлений и объясняет
// причину блокировки. targetUserID==actorUserID отсекается уже в Ban() (самобан
// запрещён раньше), проверка здесь - дополнительная защита от несогласованного
// вызова в будущем. Best-effort: ошибка не должна откатывать сам бан.
func (s *UserBanService) notifyBanned(ctx context.Context, targetUserID, actorUserID int, reason string) {
	if s.notificationService == nil || targetUserID == actorUserID {
		return
	}

	// Кто именно заблокировал, в тексте не называем: человеку важен факт и причина,
	// а не должность того, кто нажал кнопку. Единое правило для всех уведомлений об
	// учётной записи - см. смену пароля и роли (#974).
	message := "Ваша учётная запись заблокирована."
	if reason != "" {
		message = fmt.Sprintf("Ваша учётная запись заблокирована. Причина: %s", reason)
	}

	dataPayload := map[string]any{
		"reason":    reason,
		"banned_at": time.Now().UTC().Format(time.RFC3339),
		"banned_by": actorUserID,
	}
	dataBytes, err := json.Marshal(dataPayload)
	if err != nil {
		slog.Warn("не удалось сериализовать payload уведомления о блокировке учётки", "error", err)
		return
	}
	dataStr := string(dataBytes)

	if err := s.notificationService.CreateForUser(
		ctx, targetUserID, NotificationTypeUserBanned,
		"Учётная запись заблокирована", message, &dataStr,
	); err != nil {
		slog.Warn("не удалось создать уведомление о блокировке учётки", "user_id", targetUserID, "error", err)
	}
}

// notifyUnbanned - симметричная пара notifyBanned для снятия блокировки.
// reason - причина СНЯТОЙ блокировки (для контекста в data), не новая.
func (s *UserBanService) notifyUnbanned(ctx context.Context, targetUserID, actorUserID int, reason string) {
	if s.notificationService == nil || targetUserID == actorUserID {
		return
	}

	dataPayload := map[string]any{
		"reason":      reason,
		"unbanned_at": time.Now().UTC().Format(time.RFC3339),
		"unbanned_by": actorUserID,
	}
	dataBytes, err := json.Marshal(dataPayload)
	if err != nil {
		slog.Warn("не удалось сериализовать payload уведомления о разблокировке учётки", "error", err)
		return
	}
	dataStr := string(dataBytes)

	if err := s.notificationService.CreateForUser(
		ctx, targetUserID, NotificationTypeUserUnbanned,
		"Учётная запись разблокирована", "Доступ к вашей учётной записи восстановлен.", &dataStr,
	); err != nil {
		slog.Warn("не удалось создать уведомление о разблокировке учётки", "user_id", targetUserID, "error", err)
	}
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
		s.notifyBanChanged(targetUserID, "user.banned")
		s.notifyBanned(ctx, targetUserID, actorUserID, trimmedReason)
	}
	return err
}

// targetUserID резолвит username -> id (0 если не найден). Bulk-операции ключуются
// по username наружу, а сам бан - по user ID (зеркало userService.targetUserID).
func (s *UserBanService) targetUserID(ctx context.Context, username string) int {
	var id int
	s.db.WithContext(ctx).Table("users").Select("id").Where("username = ?", username).Scan(&id)
	return id
}

// BulkBan блокирует набор пользователей через Ban. Самобан, супер-админ и
// несуществующие честно попадают в Errors (частичный успех, 207) - операция не
// падает целиком. reason единый на всю пачку.
func (s *UserBanService) BulkBan(ctx context.Context, actorUserID int, usernames []string, reason string) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, u := range uniqueStrings(usernames) {
		id := s.targetUserID(ctx, u)
		if id == 0 {
			res.addError(0, u, "Пользователь не найден")
			continue
		}
		if id == actorUserID {
			res.addError(id, u, "Нельзя заблокировать самого себя")
			continue
		}
		if err := s.Ban(ctx, id, actorUserID, reason); err != nil {
			res.addError(id, u, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkUnban разблокирует набор пользователей через Unban.
func (s *UserBanService) BulkUnban(ctx context.Context, actorUserID int, usernames []string) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, u := range uniqueStrings(usernames) {
		id := s.targetUserID(ctx, u)
		if id == 0 {
			res.addError(0, u, "Пользователь не найден")
			continue
		}
		if err := s.Unban(ctx, id, actorUserID); err != nil {
			res.addError(id, u, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
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
	s.notifyBanChanged(targetUserID, "user.unbanned")
	unbannedReason := ""
	if hasPrev && prev.BanReason != nil {
		unbannedReason = *prev.BanReason
	}
	s.notifyUnbanned(ctx, targetUserID, actorUserID, unbannedReason)
	return nil
}
