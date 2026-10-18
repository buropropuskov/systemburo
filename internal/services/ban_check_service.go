package services

import (
	"context"
	"errors"
	"sync"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// BanCheckService читает актуальный бан и активность; поля кэша сохранены для совместимости.
type BanCheckService struct {
	db    *gorm.DB
	ttl   time.Duration
	cache sync.Map
	// Защищает поколение инвалидации и публикацию; SQL выполняется вне lock.
	cacheMu         sync.Mutex
	cacheGeneration uint64
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

// Status читает актуальное состояние без TTL между экземплярами backend.
func (s *BanCheckService) Status(ctx context.Context, userID int) (banned, active bool, err error) {
	var user models.User
	if err := s.db.WithContext(ctx).Select("id, is_banned, is_active").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, false, nil
		}
		return false, false, err
	}
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
	s.cacheMu.Lock()
	s.cacheGeneration++
	s.cache.Delete(userID)
	s.cacheMu.Unlock()
}
