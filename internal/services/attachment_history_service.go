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

// AttachmentHistoryService - аудит действий над шаблонами вложений (#416).
type AttachmentHistoryService interface {
	// Log пишет запись аудита. Ошибка не возвращается: аудит не ломает основное действие.
	Log(ctx context.Context, attachmentID int, actorUserID *int, actionType string, details interface{})
	// GetHistory возвращает историю по шаблону вложения (новые сверху).
	GetHistory(ctx context.Context, attachmentID int) ([]models.UniqueAttachmentHistoryItem, error)
}

type attachmentHistoryService struct {
	db *gorm.DB
}

// NewAttachmentHistoryService создаёт реализацию AttachmentHistoryService.
func NewAttachmentHistoryService(db *gorm.DB) AttachmentHistoryService {
	return &attachmentHistoryService{db: db}
}

func (s *attachmentHistoryService) Log(ctx context.Context, attachmentID int, actorUserID *int, actionType string, details interface{}) {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			slog.Error("attachment_history: marshal details", "attachment", attachmentID, "error", err)
		} else {
			raw = b
		}
	}
	entry := models.UniqueAttachmentHistory{
		UniqueAttachmentID: attachmentID,
		ActorUserID:        actorUserID,
		ActionType:         actionType,
		Details:            raw,
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		slog.Error("attachment_history: insert", "attachment", attachmentID, "action", actionType, "error", err)
	}
}

func (s *attachmentHistoryService) GetHistory(ctx context.Context, attachmentID int) ([]models.UniqueAttachmentHistoryItem, error) {
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
		Table("unique_attachment_histories AS h").
		Select(`h.id, h.action_type, h.details, h.actor_user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS actor_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.actor_user_id").
		Where("h.unique_attachment_id = ?", attachmentID).
		Order("h.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching attachment history")
	}

	items := make([]models.UniqueAttachmentHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.UniqueAttachmentHistoryItem{
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
