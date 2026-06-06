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

// OrganizationHistoryService - аудит действий над организациями (#412).
type OrganizationHistoryService interface {
	// Log пишет запись аудита. Ошибка не возвращается: аудит не ломает основное действие.
	Log(ctx context.Context, organizationID int, actorUserID *int, actionType string, details interface{})
	// GetHistory возвращает историю по организации (новые сверху).
	GetHistory(ctx context.Context, organizationID int) ([]models.OrganizationHistoryItem, error)
}

type organizationHistoryService struct {
	db *gorm.DB
}

// NewOrganizationHistoryService создаёт реализацию OrganizationHistoryService.
func NewOrganizationHistoryService(db *gorm.DB) OrganizationHistoryService {
	return &organizationHistoryService{db: db}
}

func (s *organizationHistoryService) Log(ctx context.Context, organizationID int, actorUserID *int, actionType string, details interface{}) {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			slog.Error("organization_history: marshal details", "org", organizationID, "error", err)
		} else {
			raw = b
		}
	}
	entry := models.OrganizationHistory{
		OrganizationID: organizationID,
		ActorUserID:    actorUserID,
		ActionType:     actionType,
		Details:        raw,
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		slog.Error("organization_history: insert", "org", organizationID, "action", actionType, "error", err)
	}
}

func (s *organizationHistoryService) GetHistory(ctx context.Context, organizationID int) ([]models.OrganizationHistoryItem, error) {
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
		Table("organization_histories AS h").
		Select(`h.id, h.action_type, h.details, h.actor_user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS actor_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.actor_user_id").
		Where("h.organization_id = ?", organizationID).
		Order("h.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization history")
	}

	items := make([]models.OrganizationHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.OrganizationHistoryItem{
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
