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

// UserTypeHistoryService - аудит действий над типами пользователей (#411).
type UserTypeHistoryService interface {
	// Log пишет запись аудита. details - произвольный JSON, опционально.
	// Ошибка логирования не возвращается: аудит не должен ломать основное действие.
	Log(ctx context.Context, userTypeID int, actorUserID *int, actionType string, details interface{})
	// GetHistory возвращает историю по типу пользователя (новые сверху).
	GetHistory(ctx context.Context, userTypeID int) ([]models.UserTypeHistoryItem, error)
}

type userTypeHistoryService struct {
	db *gorm.DB
}

// NewUserTypeHistoryService создаёт реализацию UserTypeHistoryService.
func NewUserTypeHistoryService(db *gorm.DB) UserTypeHistoryService {
	return &userTypeHistoryService{db: db}
}

func (s *userTypeHistoryService) Log(ctx context.Context, userTypeID int, actorUserID *int, actionType string, details interface{}) {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			slog.Error("user_type_history: marshal details", "type", userTypeID, "error", err)
		} else {
			raw = b
		}
	}
	entry := models.UserTypeHistory{
		UserTypeID:  userTypeID,
		ActorUserID: actorUserID,
		ActionType:  actionType,
		Details:     raw,
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		slog.Error("user_type_history: insert", "type", userTypeID, "action", actionType, "error", err)
	}
}

func (s *userTypeHistoryService) GetHistory(ctx context.Context, userTypeID int) ([]models.UserTypeHistoryItem, error) {
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
		Table("user_type_histories AS h").
		Select(`h.id, h.action_type, h.details, h.actor_user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS actor_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.actor_user_id").
		Where("h.user_type_id = ?", userTypeID).
		Order("h.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user type history")
	}

	items := make([]models.UserTypeHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.UserTypeHistoryItem{
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
