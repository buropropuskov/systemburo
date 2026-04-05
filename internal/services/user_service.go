package services

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UserService — интерфейс бизнес-логики управления пользователями (admin-only).
type UserService interface {
	// Create создаёт нового пользователя (admin-only).
	Create(ctx context.Context, callerTypeID int, req models.RegisterRequest) error
	// GetAll в��звращает список всех ��ользователей с организацией, компанией и типом.
	GetAll(ctx context.Context, callerTypeID int) ([]models.UserInfoResponse, error)
	// UpdateType обновляет тип пользователя.
	UpdateType(ctx context.Context, callerTypeID int, username string, req models.UpdateUserTypeRequest) error
	// UpdatePassword обновляет пароль пользователя.
	UpdatePassword(ctx context.Context, callerTypeID int, username string, req models.UpdatePasswordRequest) error
	// UpdateInfo обновляет ФИО, должность, email и телефон пользователя.
	UpdateInfo(ctx context.Context, callerTypeID int, username string, req models.UpdateUserInfoRequest) error
	// UpdateOrganization обновляе�� организацию пользовате��я.
	UpdateOrganization(ctx context.Context, callerTypeID int, username string, req models.UpdateUserOrganizationRequest) error
	// UpdateCompany обновляе�� компан��ю пользователя.
	UpdateCompany(ctx context.Context, callerTypeID int, username string, req models.UpdateUserCompanyRequest) error
	// Delete удаляет пользователя по username.
	Delete(ctx context.Context, callerTypeID int, username string) error
}

type userService struct {
	db *gorm.DB
}

// NewUserService создаёт новый экземпляр сервиса управления пользователями.
func NewUserService(db *gorm.DB) UserService {
	return &userService{db: db}
}

// checkAdmin проверяет, что вызывающий пользователь является администратором
// (код типа "manager" или "buropropuskov").
func (s *userService) checkAdmin(ctx context.Context, typeID int) error {
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

// Create создаёт нового пользователя. Только admin (manager/buropropuskov).
func (s *userService) Create(ctx context.Context, callerTypeID int, req models.RegisterRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	user := models.User{
		Username:       req.Username,
		Password:       hashPassword(req.Password),
		OrganizationID: req.OrganizationID,
		CompanyID:      req.CompanyID,
		TypeID:         req.TypeID,
		LastName:       req.LastName,
		FirstName:      req.FirstName,
		MiddleName:     req.MiddleName,
		Position:       req.Position,
		Email:          req.Email,
		Phone:          req.Phone,
	}
	if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return echo.NewHTTPError(http.StatusBadRequest, "Username already exists")
		}
		slog.Error("не удалось создать пользователя", "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error creating user")
	}
	return nil
}

// GetAll возвращает всех пользователей с JOIN на организацию, компанию и тип.
func (s *userService) GetAll(ctx context.Context, callerTypeID int) ([]models.UserInfoResponse, error) {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return nil, err
	}

	result := make([]models.UserInfoResponse, 0)
	err := s.db.WithContext(ctx).
		Table("users u").
		Select(`u.id, u.username,
			o.name as organization, u.organization_id,
			c.name as company, u.company_id,
			u.type_id, ut.name as user_type,
			u.last_name, u.first_name, u.middle_name,
			u.position, u.email, u.phone`).
		Joins("LEFT JOIN organizations o ON u.organization_id = o.id").
		Joins("LEFT JOIN companies c ON u.company_id = c.id").
		Joins("LEFT JOIN user_types ut ON u.type_id = ut.id").
		Order("u.username").
		Scan(&result).Error
	if err != nil {
		slog.Error("не удалось получить список пользователей", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching users")
	}

	return result, nil
}

// UpdateType обновляет type_id пользователя с проверкой существования типа.
func (s *userService) UpdateType(ctx context.Context, callerTypeID int, username string, req models.UpdateUserTypeRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	// Проверяем существование типа
	var exists bool
	if err := s.db.WithContext(ctx).
		Table("user_types").
		Select("COUNT(1) > 0").
		Where("id = ?", req.TypeID).
		Row().Scan(&exists); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking user type")
	}
	if !exists {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid user type")
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("type_id", req.TypeID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user type")
	}

	return nil
}

// UpdatePassword хеширует и обновляет пароль пользователя.
func (s *userService) UpdatePassword(ctx context.Context, callerTypeID int, username string, req models.UpdatePasswordRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	hashed := hashPassword(req.Password)

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("password", hashed).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating password")
	}

	return nil
}

// UpdateInfo обновляет персональные данные пользователя.
func (s *userService) UpdateInfo(ctx context.Context, callerTypeID int, username string, req models.UpdateUserInfoRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Updates(map[string]interface{}{
			"last_name":   req.LastName,
			"first_name":  req.FirstName,
			"middle_name": req.MiddleName,
			"position":    req.Position,
			"email":       req.Email,
			"phone":       req.Phone,
		}).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating user info")
	}

	return nil
}

// UpdateOrganization обновляет organization_id пользователя.
func (s *userService) UpdateOrganization(ctx context.Context, callerTypeID int, username string, req models.UpdateUserOrganizationRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("organization_id", req.OrganizationID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating organization")
	}

	return nil
}

// UpdateCompany обновляет company_id пользователя.
func (s *userService) UpdateCompany(ctx context.Context, callerTypeID int, username string, req models.UpdateUserCompanyRequest) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Update("company_id", req.CompanyID).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating company")
	}

	return nil
}

// Delete удаляет пользователя по username.
func (s *userService) Delete(ctx context.Context, callerTypeID int, username string) error {
	if err := s.checkAdmin(ctx, callerTypeID); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).
		Table("users").
		Where("username = ?", username).
		Delete(&models.User{}).Error; err != nil {
		slog.Error("не удалось удалить пользователя", "username", username, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting user")
	}

	return nil
}
