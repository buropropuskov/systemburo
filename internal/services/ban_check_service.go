package services

import (
	"context"
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
	expiresAt time.Time
}

// NewBanCheckService создаёт сервис с заданным TTL кэша (рекомендуется 30s).
func NewBanCheckService(db *gorm.DB, ttl time.Duration) *BanCheckService {
	return &BanCheckService{db: db, ttl: ttl}
}

// IsBanned возвращает true если у пользователя is_banned=true.
// На cache miss или истёкшую запись делает один SELECT.
func (s *BanCheckService) IsBanned(ctx context.Context, userID int) (bool, error) {
	if v, ok := s.cache.Load(userID); ok {
		entry := v.(banEntry)
		if time.Now().Before(entry.expiresAt) {
			return entry.banned, nil
		}
	}

	var user models.User
	if err := s.db.WithContext(ctx).Select("id, is_banned").First(&user, userID).Error; err != nil {
		return false, err
	}
	s.cache.Store(userID, banEntry{
		banned:    user.IsBanned,
		expiresAt: time.Now().Add(s.ttl),
	})
	return user.IsBanned, nil
}

// Invalidate сбрасывает кэш для конкретного пользователя.
// Вызывается из UserBanService при Ban/Unban.
func (s *BanCheckService) Invalidate(userID int) {
	s.cache.Delete(userID)
}
