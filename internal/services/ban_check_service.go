package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// BanCheckService проверяет users.is_banned с in-memory кэшем.
// Используется глобальным middleware на каждом protected-запросе,
// поэтому SQL не должен срабатывать чаще раза в TTL.
//
// Инвалидация - явная, при Ban()/Unban() в UserBanService.
type BanCheckService struct {
	db    *gorm.DB
	ttl   time.Duration
	cache sync.Map
}

type banEntry struct {
	banned    bool
	active    bool
	expiresAt time.Time
}

// NewBanCheckService создаёт сервис с заданным TTL кэша (рекомендуется 30s).
func NewBanCheckService(db *gorm.DB, ttl time.Duration) *BanCheckService {
	return &BanCheckService{db: db, ttl: ttl}
}

// Status возвращает (banned, active) пользователя на горячем пути.
// Архивный (is_active=false) юзер блокируется так же мгновенно, как забаненный,
// чтобы офбординг не дожидался истечения access-токена. На cache miss/истёкшую
// запись делает один SELECT; при ошибке БД отдаёт active=true (caller fail-open).
func (s *BanCheckService) Status(ctx context.Context, userID int) (banned, active bool, err error) {
	if v, ok := s.cache.Load(userID); ok {
		entry := v.(banEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.banned, entry.active, nil
		}
	}

	var user models.User
	if err := s.db.WithContext(ctx).Select("id, is_banned, is_active").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Несуществующий юзер (напр. валидный JWT на удалённую запись) -
			// трактуем как inactive, чтобы middleware дал 403, а не fail-open.
			return false, false, nil
		}
		// Транзиентная ошибка БД - active=true, caller делает fail-open.
		return false, true, err
	}
	s.cache.Store(userID, banEntry{
		banned:    user.IsBanned,
		active:    user.IsActive,
		expiresAt: time.Now().Add(s.ttl),
	})
	return user.IsBanned, user.IsActive, nil
}

// IsBanned - обратная совместимость (используется в интеграционных тестах).
func (s *BanCheckService) IsBanned(ctx context.Context, userID int) (bool, error) {
	banned, _, err := s.Status(ctx, userID)
	return banned, err
}

// Invalidate сбрасывает кэш для конкретного пользователя.
// Вызывается из UserBanService при Ban/Unban.
func (s *BanCheckService) Invalidate(userID int) {
	s.cache.Delete(userID)
}
