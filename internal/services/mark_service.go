package services

import (
	"context"
	"errors"
	"net/http"

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
	GetHistory(ctx context.Context, id int) ([]models.MarkHistory, error)
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

func (s *markService) GetHistory(ctx context.Context, id int) ([]models.MarkHistory, error) {
	history := make([]models.MarkHistory, 0)
	if err := s.db.WithContext(ctx).
		Where("mark_id = ?", id).
		Order("created_at DESC").
		Find(&history).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка получения истории")
	}
	return history, nil
}
