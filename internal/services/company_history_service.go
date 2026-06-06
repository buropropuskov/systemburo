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

// CompanyHistoryService - аудит действий над компаниями (#412).
type CompanyHistoryService interface {
	// Log пишет запись аудита. Ошибка не возвращается: аудит не ломает основное действие.
	Log(ctx context.Context, companyID int, actorUserID *int, actionType string, details interface{})
	// GetHistory возвращает историю по компании (новые сверху).
	GetHistory(ctx context.Context, companyID int) ([]models.CompanyHistoryItem, error)
}

type companyHistoryService struct {
	db *gorm.DB
}

// NewCompanyHistoryService создаёт реализацию CompanyHistoryService.
func NewCompanyHistoryService(db *gorm.DB) CompanyHistoryService {
	return &companyHistoryService{db: db}
}

func (s *companyHistoryService) Log(ctx context.Context, companyID int, actorUserID *int, actionType string, details interface{}) {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			slog.Error("company_history: marshal details", "company", companyID, "error", err)
		} else {
			raw = b
		}
	}
	entry := models.CompanyHistory{
		CompanyID:   companyID,
		ActorUserID: actorUserID,
		ActionType:  actionType,
		Details:     raw,
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		slog.Error("company_history: insert", "company", companyID, "action", actionType, "error", err)
	}
}

func (s *companyHistoryService) GetHistory(ctx context.Context, companyID int) ([]models.CompanyHistoryItem, error) {
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
		Table("company_histories AS h").
		Select(`h.id, h.action_type, h.details, h.actor_user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS actor_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.actor_user_id").
		Where("h.company_id = ?", companyID).
		Order("h.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company history")
	}

	items := make([]models.CompanyHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.CompanyHistoryItem{
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
