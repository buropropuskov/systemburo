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

// SystemTableHistoryService - логирование CRUD-действий над системными таблицами (#345).
type SystemTableHistoryService interface {
	// Log пишет запись о действии. details - произвольный JSON, опционально.
	Log(ctx context.Context, tableID int, userID *int, actionType string, details interface{}) error
	// GetHistory возвращает историю действий по таблице (новые сверху).
	GetHistory(ctx context.Context, tableID int) ([]models.SystemTableHistoryItem, error)
}

type systemTableHistoryService struct {
	db *gorm.DB
}

// NewSystemTableHistoryService создаёт реализацию SystemTableHistoryService.
func NewSystemTableHistoryService(db *gorm.DB) SystemTableHistoryService {
	return &systemTableHistoryService{db: db}
}

func (s *systemTableHistoryService) Log(ctx context.Context, tableID int, userID *int, actionType string, details interface{}) error {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			slog.Error("system_table_history: marshal details", "table_id", tableID, "error", err)
			return nil // не падаем на лог-ошибке
		}
		raw = b
	}
	entry := models.SystemTableHistory{
		SystemTableID: tableID,
		ActionType:    actionType,
		Details:       raw,
		UserID:        userID,
	}
	if err := s.db.WithContext(ctx).Create(&entry).Error; err != nil {
		slog.Error("system_table_history: insert", "table_id", tableID, "action", actionType, "error", err)
		return nil // лог-ошибка не должна ломать основное действие
	}
	return nil
}

func (s *systemTableHistoryService) GetHistory(ctx context.Context, tableID int) ([]models.SystemTableHistoryItem, error) {
	type row struct {
		ID         int             `gorm:"column:id"`
		ActionType string          `gorm:"column:action_type"`
		Details    json.RawMessage `gorm:"column:details"`
		UserID     *int            `gorm:"column:user_id"`
		UserName   string          `gorm:"column:user_name"`
		CreatedAt  time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("system_table_histories AS h").
		Select(`h.id, h.action_type, h.details, h.user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS user_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.user_id").
		Where("h.system_table_id = ?", tableID).
		Order("h.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system table history")
	}

	items := make([]models.SystemTableHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.SystemTableHistoryItem{
			ID:         r.ID,
			ActionType: r.ActionType,
			Details:    r.Details,
			UserID:     r.UserID,
			UserName:   r.UserName,
			CreatedAt:  r.CreatedAt,
		})
	}
	return items, nil
}
