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

// VehicleBlacklistHistoryService - аудит действий над чёрным списком машин (#443).
type VehicleBlacklistHistoryService interface {
	// Log пишет запись аудита через переданный executor (передавать tx для
	// атомарности с основной операцией). details - произвольный JSON, опционально.
	// В отличие от лучших-усилий SystemTableHistory, ошибку возвращаем: запись
	// аудита - часть транзакции каскада, её провал должен откатить всю операцию.
	Log(ctx context.Context, exec *gorm.DB, entityID int, userID *int, actionType string, details interface{}) error
	// GetHistory возвращает историю по записи (новые сверху) с именем пользователя.
	GetHistory(ctx context.Context, entityID int) ([]models.VehicleBlacklistHistoryItem, error)
	// GetAllHistory - весь журнал ЧС машин (все записи, включая удалённые), новые сверху.
	GetAllHistory(ctx context.Context) ([]models.VehicleBlacklistHistoryItem, error)
}

type vehicleBlacklistHistoryService struct {
	db *gorm.DB
}

// NewVehicleBlacklistHistoryService создаёт реализацию.
func NewVehicleBlacklistHistoryService(db *gorm.DB) VehicleBlacklistHistoryService {
	return &vehicleBlacklistHistoryService{db: db}
}

func (s *vehicleBlacklistHistoryService) Log(ctx context.Context, exec *gorm.DB, entityID int, userID *int, actionType string, details interface{}) error {
	var raw json.RawMessage
	if details != nil {
		b, err := json.Marshal(details)
		if err != nil {
			return fmt.Errorf("marshal vehicle blacklist history details: %w", err)
		}
		raw = b
	}
	entry := models.VehicleBlacklistHistory{
		EntityID:   entityID,
		ActionType: actionType,
		Details:    raw,
		UserID:     userID,
	}
	if err := exec.WithContext(ctx).Create(&entry).Error; err != nil {
		return fmt.Errorf("insert vehicle blacklist history: %w", err)
	}
	return nil
}

func (s *vehicleBlacklistHistoryService) GetHistory(ctx context.Context, entityID int) ([]models.VehicleBlacklistHistoryItem, error) {
	return s.query(ctx, &entityID)
}

func (s *vehicleBlacklistHistoryService) GetAllHistory(ctx context.Context) ([]models.VehicleBlacklistHistoryItem, error) {
	return s.query(ctx, nil)
}

// query читает журнал ЧС машин (новые сверху). entityID == nil - весь журнал.
func (s *vehicleBlacklistHistoryService) query(ctx context.Context, entityID *int) ([]models.VehicleBlacklistHistoryItem, error) {
	type row struct {
		ID         int             `gorm:"column:id"`
		EntityID   int             `gorm:"column:entity_id"`
		ActionType string          `gorm:"column:action_type"`
		Details    json.RawMessage `gorm:"column:details"`
		UserID     *int            `gorm:"column:user_id"`
		UserName   string          `gorm:"column:user_name"`
		CreatedAt  time.Time       `gorm:"column:created_at"`
	}
	q := s.db.WithContext(ctx).
		Table("vehicle_blacklist_histories AS h").
		Select(`h.id, h.entity_id, h.action_type, h.details, h.user_id,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS user_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.user_id")
	if entityID != nil {
		q = q.Where("h.entity_id = ?", *entityID)
	}

	var rows []row
	if err := q.Order("h.created_at DESC").Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения истории чёрного списка")
	}

	items := make([]models.VehicleBlacklistHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.VehicleBlacklistHistoryItem{
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
