package services

import (
	"context"
	"errors"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// MarkService - бизнес-логика справочника марок автомобилей с историчностью.
type MarkService interface {
	GetAll(ctx context.Context, includeArchived bool) ([]models.Mark, error)
	GetByID(ctx context.Context, id int) (*models.Mark, error)
	Create(ctx context.Context, req models.CreateMarkRequest, userID int) (*models.Mark, error)
	Update(ctx context.Context, id int, req models.UpdateMarkRequest, userID int) error
	Archive(ctx context.Context, id int, userID int) error
	Restore(ctx context.Context, id int, userID int) error
	GetHistory(ctx context.Context, id int) ([]models.MarkHistoryItem, error)
}

type markService struct {
	db *gorm.DB
}

// NewMarkService создаёт реализацию MarkService.
func NewMarkService(db *gorm.DB) MarkService {
	return &markService{db: db}
}

func (s *markService) GetAll(ctx context.Context, includeArchived bool) ([]models.Mark, error) {
	marks := make([]models.Mark, 0)
	q := s.db.WithContext(ctx).Order("name")
	if !includeArchived {
		q = q.Where("is_active = ?", true)
	}
	if err := q.Find(&marks).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения марок")
	}
	return marks, nil
}

func (s *markService) GetByID(ctx context.Context, id int) (*models.Mark, error) {
	var m models.Mark
	if err := s.db.WithContext(ctx).First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Марка не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения марки")
	}
	return &m, nil
}

func (s *markService) Create(ctx context.Context, req models.CreateMarkRequest, userID int) (*models.Mark, error) {
	mark := models.Mark{
		Name:            req.Name,
		IsActive:        true,
		CreatedByUserID: &userID,
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&mark).Error; err != nil {
			return err
		}
		return tx.Create(&models.MarkHistory{
			MarkID:     mark.ID,
			ActionType: models.MarkActionCreated,
			NewValue:   &req.Name,
			UserID:     &userID,
		}).Error
	})
	if err != nil {
		// uniqueIndex на name дублирует - 409.
		return nil, echo.NewHTTPError(http.StatusConflict, "Марка с таким именем уже существует")
	}
	return &mark, nil
}

func (s *markService) Update(ctx context.Context, id int, req models.UpdateMarkRequest, userID int) error {
	var existing models.Mark
	if err := s.db.WithContext(ctx).First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Марка не найдена")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения марки")
	}
	if existing.Name == req.Name {
		return nil
	}
	oldName := existing.Name
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&existing).Update("name", req.Name).Error; err != nil {
			return err
		}
		return tx.Create(&models.MarkHistory{
			MarkID:     id,
			ActionType: models.MarkActionRenamed,
			OldValue:   &oldName,
			NewValue:   &req.Name,
			UserID:     &userID,
		}).Error
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusConflict, "Марка с таким именем уже существует")
	}
	return nil
}

func (s *markService) Archive(ctx context.Context, id int, userID int) error {
	return s.setActive(ctx, id, false, userID, models.MarkActionArchived)
}

func (s *markService) Restore(ctx context.Context, id int, userID int) error {
	return s.setActive(ctx, id, true, userID, models.MarkActionRestored)
}

func (s *markService) setActive(ctx context.Context, id int, active bool, userID int, action string) error {
	var existing models.Mark
	if err := s.db.WithContext(ctx).First(&existing, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "Марка не найдена")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения марки")
	}
	if existing.IsActive == active {
		return nil // no-op
	}
	if active {
		// Partial-unique теперь только среди активных: при восстановлении проверяем,
		// что нет активной марки с тем же именем - иначе Update упал бы 500 вместо 409.
		var cnt int64
		if err := s.db.WithContext(ctx).Model(&models.Mark{}).
			Where("name = ? AND is_active = ? AND id <> ?", existing.Name, true, id).Count(&cnt).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка проверки имени марки")
		}
		if cnt > 0 {
			return echo.NewHTTPError(http.StatusConflict, "Активная марка с таким именем уже существует")
		}
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&existing).Update("is_active", active).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Ошибка обновления марки")
		}
		return tx.Create(&models.MarkHistory{
			MarkID:     id,
			ActionType: action,
			UserID:     &userID,
		}).Error
	})
}

func (s *markService) GetHistory(ctx context.Context, id int) ([]models.MarkHistoryItem, error) {
	type row struct {
		ID         int       `gorm:"column:id"`
		MarkID     int       `gorm:"column:mark_id"`
		ActionType string    `gorm:"column:action_type"`
		OldValue   *string   `gorm:"column:old_value"`
		NewValue   *string   `gorm:"column:new_value"`
		UserID     *int      `gorm:"column:user_id"`
		UserName   string    `gorm:"column:user_name"`
		Comment    *string   `gorm:"column:comment"`
		CreatedAt  time.Time `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).
		Table("mark_histories AS h").
		Select(`h.id, h.mark_id, h.action_type, h.old_value, h.new_value, h.user_id, h.comment,
			COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '') AS user_name,
			h.created_at`).
		Joins("LEFT JOIN users u ON u.id = h.user_id").
		Where("h.mark_id = ?", id).
		Order("h.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения истории")
	}

	items := make([]models.MarkHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.MarkHistoryItem{
			ID:         r.ID,
			MarkID:     r.MarkID,
			ActionType: r.ActionType,
			OldValue:   r.OldValue,
			NewValue:   r.NewValue,
			UserID:     r.UserID,
			UserName:   r.UserName,
			Comment:    r.Comment,
			CreatedAt:  r.CreatedAt,
		})
	}
	return items, nil
}
