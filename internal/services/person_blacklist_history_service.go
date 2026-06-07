package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// PersonBlacklistHistoryService - аудит действий над чёрным списком людей (#443).
type PersonBlacklistHistoryService interface {
	// Log пишет запись аудита через переданный executor (tx для атомарности с каскадом).
	// Ошибку возвращает: запись - часть транзакции, провал откатывает операцию.
	Log(ctx context.Context, exec *gorm.DB, entityID int, userID *int, actionType string, details interface{}) error
	GetHistory(ctx context.Context, entityID int) ([]models.PersonBlacklistHistoryItem, error)
}

type personBlacklistHistoryService struct {
	db *gorm.DB
}

// NewPersonBlacklistHistoryService создаёт реализацию.
func NewPersonBlacklistHistoryService(db *gorm.DB) PersonBlacklistHistoryService {
	return &personBlacklistHistoryService{db: db}
}

func (s *personBlacklistHistoryService) Log(ctx context.Context, exec *gorm.DB, entityID int, userID *int, actionType string, details interface{}) error {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("marshal person blacklist history details: %w", err)
		}
		raw = b
	}
	entry := models.PersonBlacklistHistory{
		EntityID:   entityID,
		ActionType: actionType,
		Details:    raw,
		UserID:     userID,
	}
	if err := exec.WithContext(ctx).Create(&entry).Error; err != nil {
		return fmt.Errorf("insert person blacklist history: %w", err)
	}
	return nil
}

func (s *personBlacklistHistoryService) GetHistory(ctx context.Context, entityID int) ([]models.PersonBlacklistHistoryItem, error) {
	type row struct {
		ID         int             `gorm:"column:id"`
		EntityID   int             `gorm:"column:entity_id"`
		ActionType string          `gorm:"column:action_type"`
		Details    json.RawMessage `gorm:"column:details"`
		UserID     *int            `gorm:"column:user_id"`
		UserName   string          `gorm:"column:user_name"`
		CreatedAt  time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("person_blacklist_histories AS h").
		Select(`h.id, h.entity_id, h.action_type, h.details, h.user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS user_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.user_id").
		Where("h.entity_id = ?", entityID).
		Order("h.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения истории чёрного списка")
	}

	items := make([]models.PersonBlacklistHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.PersonBlacklistHistoryItem{
			ID:         r.ID,
			EntityID:   r.EntityID,
			ActionType: r.ActionType,
			Details:    r.Details,
			UserID:     r.UserID,
			UserName:   r.UserName,
			CreatedAt:  r.CreatedAt,
		})
	}
	return items, nil
}
