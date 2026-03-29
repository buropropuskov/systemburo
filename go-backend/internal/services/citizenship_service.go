package services

import (
	"context"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CitizenshipService -- интерфейс бизнес-логики гражданств.
type CitizenshipService interface {
	GetAll(ctx context.Context) ([]models.Citizenship, error)
	Create(ctx context.Context, typeID int, req models.CreateCitizenshipRequest) (int, error)
	Update(ctx context.Context, typeID int, id int, req models.UpdateCitizenshipRequest) error
	Delete(ctx context.Context, typeID int, id int) error
	ClearDefaults(ctx context.Context, typeID int) error
}

type citizenshipService struct {
	db *gorm.DB
}

// NewCitizenshipService создаёт реализацию CitizenshipService.
func NewCitizenshipService(db *gorm.DB) CitizenshipService {
	return &citizenshipService{db: db}
}

// checkAdmin проверяет, что пользователь с данным type_id является администратором
// (код типа "manager" или "buropropuskov").
func (s *citizenshipService) checkAdmin(ctx context.Context, typeID int) error {
	var code string
	err := s.db.WithContext(ctx).
		Table("user_types").
		Select("code").
		Where("id = ?", typeID).
		Row().
		Scan(&code)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}
	if code != "manager" && code != "buropropuskov" {
		return echo.NewHTTPError(http.StatusForbidden, "Insufficient permissions")
	}
	return nil
}

func (s *citizenshipService) GetAll(ctx context.Context) ([]models.Citizenship, error) {
	var citizenships []models.Citizenship
	if err := s.db.WithContext(ctx).Order("name").Find(&citizenships).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching citizenships")
	}
	return citizenships, nil
}

func (s *citizenshipService) Create(ctx context.Context, typeID int, req models.CreateCitizenshipRequest) (int, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return 0, err
	}

	isDefault := req.IsDefault != nil && *req.IsDefault
	patentRequired := req.PatentRequired != nil && *req.PatentRequired

	citizenship := models.Citizenship{
		Name:           req.Name,
		Icon:           req.Icon,
		IsDefault:      isDefault,
		PatentRequired: patentRequired,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Если выбран как гражданство по умолчанию, снимаем флаг у остальных
		if isDefault {
			if err := tx.Model(&models.Citizenship{}).
				Where("is_default = ?", true).
				Update("is_default", false).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error clearing default citizenships")
			}
		}

		if err := tx.Create(&citizenship).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error creating citizenship")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return citizenship.ID, nil
}

func (s *citizenshipService) Update(ctx context.Context, typeID int, id int, req models.UpdateCitizenshipRequest) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

	isActive := req.IsActive == nil || *req.IsActive
	isDefault := req.IsDefault != nil && *req.IsDefault
	patentRequired := req.PatentRequired != nil && *req.PatentRequired

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Если выбран как гражданство по умолчанию, снимаем флаг у остальных
		if isDefault {
			if err := tx.Model(&models.Citizenship{}).
				Where("is_default = ? AND id != ?", true, id).
				Update("is_default", false).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error clearing default citizenships")
			}
		}

		result := tx.Model(&models.Citizenship{}).
			Where("id = ?", id).
			Updates(map[string]interface{}{
				"name":            req.Name,
				"icon":            req.Icon,
				"is_active":       isActive,
				"is_default":      isDefault,
				"patent_required": patentRequired,
			})
		if result.Error != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating citizenship")
		}
		if result.RowsAffected == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "Гражданство не найдено")
		}
		return nil
	})
}

func (s *citizenshipService) Delete(ctx context.Context, typeID int, id int) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

	result := s.db.WithContext(ctx).Delete(&models.Citizenship{}, id)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting citizenship")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Гражданство не найдено")
	}
	return nil
}

func (s *citizenshipService) ClearDefaults(ctx context.Context, typeID int) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Model(&models.Citizenship{}).
		Where("is_default = ?", true).
		Update("is_default", false).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error clearing default citizenships")
	}
	return nil
}
