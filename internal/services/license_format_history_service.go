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

// LicensePlateFormatHistoryService - аудит действий над форматами номеров (#414).
type LicensePlateFormatHistoryService interface {
	// Log пишет запись аудита. Ошибка не возвращается: аудит не ломает основное действие.
	Log(ctx context.Context, formatID int, actorUserID *int, actionType string, details interface{})
	// GetHistory возвращает историю по формату номеров (новые сверху).
	GetHistory(ctx context.Context, formatID int) ([]models.LicensePlateFormatHistoryItem, error)
}

type licensePlateFormatHistoryService struct {
	db *gorm.DB
}

// NewLicensePlateFormatHistoryService создаёт реализацию LicensePlateFormatHistoryService.
func NewLicensePlateFormatHistoryService(db *gorm.DB) LicensePlateFormatHistoryService {
	return &licensePlateFormatHistoryService{db: db}
}

func (s *licensePlateFormatHistoryService) Log(ctx context.Context, formatID int, actorUserID *int, actionType string, details interface{}) {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			slog.Error("license_format_history: marshal details", "format", formatID, "error", err)
		} else {
			raw = b
		}
	}
	entry := models.LicensePlateFormatHistory{
		FormatID:    formatID,
		ActorUserID: actorUserID,
		ActionType:  actionType,
		Details:     raw,
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		slog.Error("license_format_history: insert", "format", formatID, "action", actionType, "error", err)
	}
}

func (s *licensePlateFormatHistoryService) GetHistory(ctx context.Context, formatID int) ([]models.LicensePlateFormatHistoryItem, error) {
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
		Table("license_plate_format_histories AS h").
		Select(`h.id, h.action_type, h.details, h.actor_user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS actor_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.actor_user_id").
		Where("h.format_id = ?", formatID).
		Order("h.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching license plate format history")
	}

	items := make([]models.LicensePlateFormatHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.LicensePlateFormatHistoryItem{
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
