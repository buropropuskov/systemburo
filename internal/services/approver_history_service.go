package services

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ApproverHistoryService — аудит действий над принимающими заявки (#417).
type ApproverHistoryService interface {
	// Log пишет запись аудита. Ошибка не возвращается: аудит не ломает основное действие.
	Log(ctx context.Context, approverUserID int, approverName string, actorUserID *int, actionType string)
	// GetAll возвращает глобальный журнал принимающих (новые сверху).
	GetAll(ctx context.Context) ([]models.ApplicationApproverHistoryItem, error)
}

type approverHistoryService struct {
	db *gorm.DB
}

// NewApproverHistoryService создаёт реализацию ApproverHistoryService.
func NewApproverHistoryService(db *gorm.DB) ApproverHistoryService {
	return &approverHistoryService{db: db}
}

func (s *approverHistoryService) Log(ctx context.Context, approverUserID int, approverName string, actorUserID *int, actionType string) {
	entry := models.ApplicationApproverHistory{
		ApproverUserID: approverUserID,
		ApproverName:   approverName,
		ActorUserID:    actorUserID,
		ActionType:     actionType,
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		slog.Error("approver_history: insert", "approver_user_id", approverUserID, "action", actionType, "error", err)
	}
}

func (s *approverHistoryService) GetAll(ctx context.Context) ([]models.ApplicationApproverHistoryItem, error) {
	type row struct {
		ID             int       `gorm:"column:id"`
		ApproverUserID int       `gorm:"column:approver_user_id"`
		ApproverName   string    `gorm:"column:approver_name"`
		ActionType     string    `gorm:"column:action_type"`
		ActorUserID    *int      `gorm:"column:actor_user_id"`
		ActorName      string    `gorm:"column:actor_name"`
		CreatedAt      time.Time `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("application_approver_histories AS h").
		Select(`h.id, h.approver_user_id, h.approver_name, h.action_type, h.actor_user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS actor_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.actor_user_id").
		Order("h.created_at DESC, h.id DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching approver history")
	}

	items := make([]models.ApplicationApproverHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.ApplicationApproverHistoryItem{
			ID:             r.ID,
			ApproverUserID: r.ApproverUserID,
			ApproverName:   r.ApproverName,
			ActionType:     r.ActionType,
			ActorUserID:    r.ActorUserID,
			ActorName:      r.ActorName,
			CreatedAt:      r.CreatedAt,
		})
	}
	return items, nil
}
