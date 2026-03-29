package services

import (
	"context"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// ApproverService определяет интерфейс управления утверждающими заявок.
type ApproverService interface {
	GetAll(ctx context.Context, typeID int) ([]models.ApplicationApproverWithUser, error)
	GetAvailableUsers(ctx context.Context, typeID int) ([]models.AvailableApproverUser, error)
	Create(ctx context.Context, typeID int, userID int, createdByUsername string) error
	Delete(ctx context.Context, typeID int, id int) error
}

type approverService struct {
	db *gorm.DB
}

func NewApproverService(db *gorm.DB) ApproverService {
	return &approverService{db: db}
}

func (s *approverService) checkAdmin(ctx context.Context, typeID int) error {
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

func (s *approverService) GetAll(ctx context.Context, typeID int) ([]models.ApplicationApproverWithUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

	result := make([]models.ApplicationApproverWithUser, 0)
	err := s.db.WithContext(ctx).
		Table("application_approvers aa").
		Select(`aa.id, aa.user_id, u.username, u.last_name, u.first_name, u.middle_name,
			u."position", o.name as organization, c.name as company, aa.created_at`).
		Joins("JOIN users u ON u.id = aa.user_id").
		Joins("LEFT JOIN organizations o ON o.id = u.organization_id").
		Joins("LEFT JOIN companies c ON c.id = u.company_id").
		Order("u.last_name, u.first_name").
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching approvers")
	}
	return result, nil
}

func (s *approverService) GetAvailableUsers(ctx context.Context, typeID int) ([]models.AvailableApproverUser, error) {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return nil, err
	}

	result := make([]models.AvailableApproverUser, 0)
	err := s.db.WithContext(ctx).
		Table("users u").
		Select(`u.id, u.username, u.last_name, u.first_name, u.middle_name,
			u."position", o.name as organization, c.name as company`).
		Joins("LEFT JOIN organizations o ON o.id = u.organization_id").
		Joins("LEFT JOIN companies c ON c.id = u.company_id").
		Where("NOT EXISTS (SELECT 1 FROM application_approvers aa WHERE aa.user_id = u.id)").
		Order("u.last_name, u.first_name").
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching available users")
	}
	return result, nil
}

func (s *approverService) Create(ctx context.Context, typeID int, userID int, createdByUsername string) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

	var userExists int64
	s.db.WithContext(ctx).Table("users").Where("id = ?", userID).Count(&userExists)
	if userExists == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "User not found")
	}

	var alreadyApprover int64
	s.db.WithContext(ctx).Table("application_approvers").Where("user_id = ?", userID).Count(&alreadyApprover)
	if alreadyApprover > 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "User is already an approver")
	}

	var createdByID int
	if err := s.db.WithContext(ctx).Table("users").Select("id").Where("username = ?", createdByUsername).Row().Scan(&createdByID); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Current user not found")
	}

	approver := models.ApplicationApprover{
		UserID:    userID,
		CreatedBy: &createdByID,
	}
	if err := s.db.WithContext(ctx).Create(&approver).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error creating approver")
	}
	return nil
}

func (s *approverService) Delete(ctx context.Context, typeID int, id int) error {
	if err := s.checkAdmin(ctx, typeID); err != nil {
		return err
	}

	result := s.db.WithContext(ctx).Delete(&models.ApplicationApprover{}, id)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting approver")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Approver not found")
	}
	return nil
}
