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

// CitizenshipHistoryService - аудит действий над гражданствами (#415).
type CitizenshipHistoryService interface {
	// Log пишет запись аудита. Ошибка не возвращается: аудит не ломает основное действие.
	Log(ctx context.Context, citizenshipID int, actorUserID *int, actionType string, details interface{})
	// GetHistory возвращает историю по гражданству (новые сверху).
	GetHistory(ctx context.Context, citizenshipID int) ([]models.CitizenshipHistoryItem, error)
}

type citizenshipHistoryService struct {
	db *gorm.DB
}

// NewCitizenshipHistoryService создаёт реализацию CitizenshipHistoryService.
func NewCitizenshipHistoryService(db *gorm.DB) CitizenshipHistoryService {
	return &citizenshipHistoryService{db: db}
}

func (s *citizenshipHistoryService) Log(ctx context.Context, citizenshipID int, actorUserID *int, actionType string, details interface{}) {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			slog.Error("citizenship_history: marshal details", "citizenship", citizenshipID, "error", err)
		} else {
			raw = b
		}
	}
	entry := models.CitizenshipHistory{
		CitizenshipID: citizenshipID,
		ActorUserID:   actorUserID,
		ActionType:    actionType,
		Details:       raw,
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		slog.Error("citizenship_history: insert", "citizenship", citizenshipID, "action", actionType, "error", err)
	}
}

func (s *citizenshipHistoryService) GetHistory(ctx context.Context, citizenshipID int) ([]models.CitizenshipHistoryItem, error) {
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
		Table("citizenship_histories AS h").
		Select(`h.id, h.action_type, h.details, h.actor_user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS actor_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.actor_user_id").
		Where("h.citizenship_id = ?", citizenshipID).
		Order("h.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching citizenship history")
	}

	items := make([]models.CitizenshipHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.CitizenshipHistoryItem{
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
