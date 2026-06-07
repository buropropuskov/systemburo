package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UnloadPlaceHistoryService - аудит действий над местами разгрузки (#413).
type UnloadPlaceHistoryService interface {
	// Log пишет запись аудита. Ошибка не возвращается: аудит не ломает основное действие.
	Log(ctx context.Context, unloadPlaceID int, actorUserID *int, actionType string, details interface{})
	// GetHistory возвращает историю по месту разгрузки (новые сверху).
	GetHistory(ctx context.Context, unloadPlaceID int) ([]models.UnloadPlaceHistoryItem, error)
}

type unloadPlaceHistoryService struct {
	db *gorm.DB
}

// NewUnloadPlaceHistoryService создаёт реализацию UnloadPlaceHistoryService.
func NewUnloadPlaceHistoryService(db *gorm.DB) UnloadPlaceHistoryService {
	return &unloadPlaceHistoryService{db: db}
}

func (s *unloadPlaceHistoryService) Log(ctx context.Context, unloadPlaceID int, actorUserID *int, actionType string, details interface{}) {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			slog.Error("unload_place_history: marshal details", "place", unloadPlaceID, "error", err)
		} else {
			raw = b
		}
	}
	entry := models.UnloadPlaceHistory{
		UnloadPlaceID: unloadPlaceID,
		ActorUserID:   actorUserID,
		ActionType:    actionType,
		Details:       raw,
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		slog.Error("unload_place_history: insert", "place", unloadPlaceID, "action", actionType, "error", err)
	}
}

func (s *unloadPlaceHistoryService) GetHistory(ctx context.Context, unloadPlaceID int) ([]models.UnloadPlaceHistoryItem, error) {
	type row struct {
		ID          int             `gorm:"column:id"`
		ActionType  string          `gorm:"column:action_type"`
		Details     json.RawMessage `gorm:"column:details"`
		ActorUserID *int            `gorm:"column:actor_user_id"`
		ActorName   string          `gorm:"column:actor_name"`
		CreatedAt   time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("unload_place_histories AS h").
		Select(`h.id, h.action_type, h.details, h.actor_user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS actor_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.actor_user_id").
		Where("h.unload_place_id = ?", unloadPlaceID).
		Order("h.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unload place history")
	}

	items := make([]models.UnloadPlaceHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.UnloadPlaceHistoryItem{
			ID:          r.ID,
			ActionType:  r.ActionType,
			Details:     r.Details,
			ActorUserID: r.ActorUserID,
			ActorName:   r.ActorName,
			CreatedAt:   r.CreatedAt,
		})
	}
	return items, nil
}
