package services

import (
	"context"
	"errors"
	"fmt"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// AuthEventReader читает историю аутентификационных событий (auth_events) для UI.
// Только чтение: сами события пишет auth_service при login/logout/refresh/отказе,
// поэтому новой записи/модели тут нет - лишь пагинированная выборка по пользователю.
type AuthEventReader struct {
	db *gorm.DB
}

// NewAuthEventReader конструирует reader.
func NewAuthEventReader(db *gorm.DB) *AuthEventReader {
	return &AuthEventReader{db: db}
}

// ResolveUserID возвращает id пользователя по username.
// Второй результат false - пользователь не найден (handler отдаёт 404).
func (r *AuthEventReader) ResolveUserID(ctx context.Context, username string) (int, bool, error) {
	var u models.User
	err := r.db.WithContext(ctx).Select("id").Where("username = ?", username).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("resolve user %q: %w", username, err)
	}
	return u.ID, true, nil
}

// ListForUser возвращает страницу истории входов пользователя (фильтр по user_id).
// Фильтр по user_id (а не username) стабилен и точен: у событий существующего юзера
// user_id проставлен всегда, а "user not found" события к нему не относятся.
func (r *AuthEventReader) ListForUser(ctx context.Context, f models.AuthEventFilter) (models.AuthEventPageResponse, error) {
	cond := r.db.WithContext(ctx).Model(&models.AuthEvent{}).Where("user_id = ?", f.UserID)
	if types := models.AuthEventCategoryTypes(f.Category); len(types) > 0 {
		cond = cond.Where("event_type IN ?", types)
	}
	if f.From != nil {
		cond = cond.Where("created_at >= ?", *f.From)
	}
	if f.To != nil {
		cond = cond.Where("created_at <= ?", *f.To)
	}

	var total int64
	if err := cond.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return models.AuthEventPageResponse{}, fmt.Errorf("count auth events: %w", err)
	}

	page, limit := normalizeAuthPage(f.Page, f.Limit)
	items := make([]models.AuthEventResponse, 0, limit)
	if err := cond.Session(&gorm.Session{}).
		Select("id, event_type, success, ip_address, user_agent, detail, created_at").
		Order("created_at DESC, id DESC").
		Limit(limit).Offset((page - 1) * limit).
		Scan(&items).Error; err != nil {
		return models.AuthEventPageResponse{}, fmt.Errorf("list auth events: %w", err)
	}

	return models.AuthEventPageResponse{Items: items, Total: total, Page: page, Limit: limit}, nil
}

func normalizeAuthPage(page, limit int) (int, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	return page, limit
}
