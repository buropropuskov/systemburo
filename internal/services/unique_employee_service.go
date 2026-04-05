package services

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// EmployeeOwnerInfo -- информация о владельце для фильтрации сотрудников.
type EmployeeOwnerInfo struct {
	HasOrganization bool `json:"has_organization"`
	HasCompany      bool `json:"has_company"`
	OrganizationID  *int `json:"organization_id"`
	CompanyID       *int `json:"company_id"`
	UserID          int  `json:"user_id"`
}

// UniqueEmployeeWithRelations -- сотрудник с данными связанных сущностей.
type UniqueEmployeeWithRelations struct {
	ID                   int        `json:"id"`
	LastName             *string    `json:"last_name"`
	FirstName            *string    `json:"first_name"`
	MiddleName           *string    `json:"middle_name"`
	OrganizationID       *int       `json:"organization_id"`
	CompanyID            *int       `json:"company_id"`
	CitizenshipID        *int       `json:"citizenship_id"`
	UserID               *int       `json:"user_id"`
	Position             *string    `json:"position"`
	PassportSeriesNumber *string    `json:"passport_series_number"`
	PatentNumber         *string    `json:"patent_number"`
	OtherPermission      *string    `json:"other_permission"`
	Status               bool       `json:"status"`
	CreatedAt            *time.Time `json:"created_at"`
	OrganizationName     *string    `json:"organization_name"`
	CompanyName          *string    `json:"company_name"`
	CitizenshipName      *string    `json:"citizenship_name"`
}

// NewUniqueEmployeeRequest -- тело запроса на создание/обновление сотрудника.
type NewUniqueEmployeeRequest struct {
	LastName             *string `json:"last_name"`
	FirstName            *string `json:"first_name"`
	MiddleName           *string `json:"middle_name"`
	CitizenshipID        *int    `json:"citizenship_id"`
	Position             *string `json:"position"`
	PassportSeriesNumber *string `json:"passport_series_number"`
	PatentNumber         *string `json:"patent_number"`
	OtherPermission      *string `json:"other_permission"`
	OrganizationID       *int    `json:"organization_id"`
	CompanyID            *int    `json:"company_id"`
	UserID               *int    `json:"user_id"`
}

// UniqueEmployeeResponse -- ответ при создании/обновлении сотрудника.
type UniqueEmployeeResponse struct {
	ID                   int        `json:"id"`
	LastName             *string    `json:"last_name"`
	FirstName            *string    `json:"first_name"`
	MiddleName           *string    `json:"middle_name"`
	CitizenshipID        *int       `json:"citizenship_id"`
	Position             *string    `json:"position"`
	PassportSeriesNumber *string    `json:"passport_series_number"`
	PatentNumber         *string    `json:"patent_number"`
	OtherPermission      *string    `json:"other_permission"`
	OrganizationID       *int       `json:"organization_id"`
	CompanyID            *int       `json:"company_id"`
	UserID               *int       `json:"user_id"`
	Status               bool       `json:"status"`
	CreatedAt            *time.Time `json:"created_at"`
}

// UniqueEmployeeService -- интерфейс бизнес-логики уникальных сотрудников.
type UniqueEmployeeService interface {
	GetOwnerInfo(ctx context.Context, username string) (*EmployeeOwnerInfo, error)
	GetAll(ctx context.Context, username string, filterType string) ([]UniqueEmployeeWithRelations, error)
	Create(ctx context.Context, username string, req NewUniqueEmployeeRequest) (*UniqueEmployeeResponse, error)
	Update(ctx context.Context, username string, id int, req NewUniqueEmployeeRequest) (*UniqueEmployeeResponse, error)
	Delete(ctx context.Context, username string, id int) error
}

type uniqueEmployeeService struct {
	db *gorm.DB
}

// NewUniqueEmployeeService создаёт реализацию UniqueEmployeeService.
func NewUniqueEmployeeService(db *gorm.DB) UniqueEmployeeService {
	return &uniqueEmployeeService{db: db}
}

// getEmployeeOwnerInfo получает информацию о владельце по username.
func (s *uniqueEmployeeService) getEmployeeOwnerInfo(ctx context.Context, username string) (*EmployeeOwnerInfo, error) {
	var result struct {
		UserID          int  `gorm:"column:user_id"`
		OrganizationID  *int `gorm:"column:organization_id"`
		CompanyID       *int `gorm:"column:company_id"`
		HasOrganization bool `gorm:"column:has_organization"`
		HasCompany      bool `gorm:"column:has_company"`
	}

	err := s.db.WithContext(ctx).
		Table("users u").
		Select(`u.id as user_id, u.organization_id, u.company_id,
			CASE WHEN o.id IS NOT NULL THEN true ELSE false END as has_organization,
			CASE WHEN c.id IS NOT NULL THEN true ELSE false END as has_company`).
		Joins("LEFT JOIN organizations o ON u.organization_id = o.id").
		Joins("LEFT JOIN companies c ON u.company_id = c.id").
		Where("u.username = ?", username).
		Scan(&result).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching user info")
	}

	return &EmployeeOwnerInfo{
		HasOrganization: result.HasOrganization,
		HasCompany:      result.HasCompany,
		OrganizationID:  result.OrganizationID,
		CompanyID:       result.CompanyID,
		UserID:          result.UserID,
	}, nil
}

// GetOwnerInfo возвращает информацию о владельце для фильтрации сотрудников.
func (s *uniqueEmployeeService) GetOwnerInfo(ctx context.Context, username string) (*EmployeeOwnerInfo, error) {
	return s.getEmployeeOwnerInfo(ctx, username)
}

// GetAll возвращает список уникальных сотрудников с фильтрацией по типу владельца.
func (s *uniqueEmployeeService) GetAll(ctx context.Context, username string, filterType string) ([]UniqueEmployeeWithRelations, error) {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).
		Table("unique_employees ue").
		Select(`ue.id, ue.last_name, ue.first_name, ue.middle_name,
			ue.organization_id, ue.company_id, ue.citizenship_id, ue.user_id,
			ue."position", ue.passport_series_number, ue.patent_number,
			ue.other_permission, ue.status, ue.created_at,
			o.name as organization_name, c.name as company_name,
			cit.name as citizenship_name`).
		Joins("LEFT JOIN organizations o ON ue.organization_id = o.id").
		Joins("LEFT JOIN companies c ON ue.company_id = c.id").
		Joins("LEFT JOIN citizenships cit ON ue.citizenship_id = cit.id")

	switch filterType {
	case "organization":
		if ownerInfo.HasOrganization {
			orgID := 0
			if ownerInfo.OrganizationID != nil {
				orgID = *ownerInfo.OrganizationID
			}
			query = query.Where("ue.organization_id = ?", orgID)
		} else {
			query = query.Where("ue.user_id = ?", ownerInfo.UserID)
		}
	case "company":
		if ownerInfo.HasCompany {
			compID := 0
			if ownerInfo.CompanyID != nil {
				compID = *ownerInfo.CompanyID
			}
			query = query.Where("ue.company_id = ?", compID)
		} else {
			query = query.Where("ue.user_id = ?", ownerInfo.UserID)
		}
	case "all":
		orgID := 0
		if ownerInfo.OrganizationID != nil {
			orgID = *ownerInfo.OrganizationID
		}
		compID := 0
		if ownerInfo.CompanyID != nil {
			compID = *ownerInfo.CompanyID
		}
		query = query.Where("ue.user_id = ? OR ue.organization_id = ? OR ue.company_id = ?",
			ownerInfo.UserID, orgID, compID)
	default:
		query = query.Where("ue.user_id = ?", ownerInfo.UserID)
	}

	query = query.Order("ue.last_name, ue.first_name, ue.middle_name")

	employees := make([]UniqueEmployeeWithRelations, 0)
	if err := query.Scan(&employees).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employees")
	}
	for i := range employees {
		employees[i].PassportSeriesNumber = crypto.DecryptOptional(employees[i].PassportSeriesNumber)
		employees[i].PatentNumber = crypto.DecryptOptional(employees[i].PatentNumber)
	}

	return employees, nil
}

// employeeToResponse конвертирует модель UniqueEmployee в UniqueEmployeeResponse.
func employeeToResponse(emp *models.UniqueEmployee) *UniqueEmployeeResponse {
	status := false
	if emp.Status != nil && *emp.Status {
		status = true
	}
	return &UniqueEmployeeResponse{
		ID:                   emp.ID,
		LastName:             emp.LastName,
		FirstName:            emp.FirstName,
		MiddleName:           emp.MiddleName,
		CitizenshipID:        emp.CitizenshipID,
		Position:             emp.Position,
		PassportSeriesNumber: emp.PassportSeriesNumber,
		PatentNumber:         emp.PatentNumber,
		OtherPermission:      emp.OtherPermission,
		OrganizationID:       emp.OrganizationID,
		CompanyID:            emp.CompanyID,
		UserID:               emp.UserID,
		Status:               status,
		CreatedAt:            &emp.CreatedAt,
	}
}

// Create создаёт уникального сотрудника с проверкой уникальности паспортных данных.
func (s *uniqueEmployeeService) Create(ctx context.Context, username string, req NewUniqueEmployeeRequest) (*UniqueEmployeeResponse, error) {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	// Проверка уникальности паспортных данных для пользователя
	if req.PassportSeriesNumber != nil {
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("user_id = ? AND passport_series_number_hmac = ?", ownerInfo.UserID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey())).
			Count(&count).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if count > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже привязан к вашему аккаунту")
		}
	}

	// Проверка уникальности для организации
	if req.OrganizationID != nil && req.PassportSeriesNumber != nil {
		var orgCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("organization_id = ? AND passport_series_number_hmac = ?", *req.OrganizationID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey())).
			Count(&orgCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if orgCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже существует в этой организации")
		}
	}

	// Проверка уникальности для компании
	if req.CompanyID != nil && req.PassportSeriesNumber != nil {
		var compCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("company_id = ? AND passport_series_number_hmac = ?", *req.CompanyID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey())).
			Count(&compCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if compCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже существует в этой компании")
		}
	}

	userID := ownerInfo.UserID
	if req.UserID != nil {
		userID = *req.UserID
	}

	statusFalse := false
	employee := models.UniqueEmployee{
		LastName:             req.LastName,
		FirstName:            req.FirstName,
		MiddleName:           req.MiddleName,
		CitizenshipID:        req.CitizenshipID,
		Position:             req.Position,
		PassportSeriesNumber: req.PassportSeriesNumber,
		PatentNumber:         req.PatentNumber,
		OtherPermission:      req.OtherPermission,
		OrganizationID:       req.OrganizationID,
		CompanyID:            req.CompanyID,
		UserID:               &userID,
		Status:               &statusFalse,
	}

	if err := s.db.WithContext(ctx).Create(&employee).Error; err != nil {
		slog.Error("не удалось создать уникального сотрудника", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании сотрудника")
	}

	slog.Info("уникальный сотрудник создан", "id", employee.ID)
	return employeeToResponse(&employee), nil
}

// Update обновляет уникального сотрудника по ID с проверкой прав и уникальности.
func (s *uniqueEmployeeService) Update(ctx context.Context, username string, id int, req NewUniqueEmployeeRequest) (*UniqueEmployeeResponse, error) {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	// Проверяем существование и права
	var existing models.UniqueEmployee
	if err := s.db.WithContext(ctx).Select("user_id, organization_id, company_id").
		First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Employee not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employee")
	}

	if !s.canEditEmployee(&existing, ownerInfo) {
		return nil, echo.NewHTTPError(http.StatusForbidden, "You don't have permission to edit this employee")
	}

	// Проверка уникальности паспортных данных для пользователя (исключая текущего)
	if req.PassportSeriesNumber != nil {
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("user_id = ? AND passport_series_number_hmac = ? AND id != ?", ownerInfo.UserID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey()), id).
			Count(&count).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if count > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже привязан к вашему аккаунту")
		}
	}

	// Проверка уникальности для организации (исключая текущего)
	if req.OrganizationID != nil && req.PassportSeriesNumber != nil {
		var orgCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("organization_id = ? AND passport_series_number_hmac = ? AND id != ?", *req.OrganizationID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey()), id).
			Count(&orgCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if orgCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже существует в этой организации")
		}
	}

	// Проверка уникальности для компании (исключая текущего)
	if req.CompanyID != nil && req.PassportSeriesNumber != nil {
		var compCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).
			Where("company_id = ? AND passport_series_number_hmac = ? AND id != ?", *req.CompanyID, crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey()), id).
			Count(&compCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking employee uniqueness")
		}
		if compCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Сотрудник с такими паспортными данными уже существует в этой компании")
		}
	}

	userID := ownerInfo.UserID
	if req.UserID != nil {
		userID = *req.UserID
	}

	updates := map[string]interface{}{
		"last_name":        req.LastName,
		"first_name":       req.FirstName,
		"middle_name":      req.MiddleName,
		"citizenship_id":   req.CitizenshipID,
		"position":         req.Position,
		"other_permission": req.OtherPermission,
		"organization_id":  req.OrganizationID,
		"company_id":       req.CompanyID,
		"user_id":          userID,
	}
	if req.PassportSeriesNumber != nil {
		enc, err := crypto.Encrypt(*req.PassportSeriesNumber, crypto.GetGlobalKey())
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "encryption error")
		}
		updates["passport_series_number"] = enc
		updates["passport_series_number_hmac"] = crypto.ComputeHMAC(*req.PassportSeriesNumber, crypto.GetGlobalKey())
	}
	if req.PatentNumber != nil {
		enc, err := crypto.Encrypt(*req.PatentNumber, crypto.GetGlobalKey())
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "encryption error")
		}
		updates["patent_number"] = enc
		updates["patent_number_hmac"] = crypto.ComputeHMAC(*req.PatentNumber, crypto.GetGlobalKey())
	}
	result := s.db.WithContext(ctx).Model(&models.UniqueEmployee{}).Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		slog.Error("не удалось обновить уникального сотрудника", "id", id, "error", result.Error)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating employee")
	}
	slog.Info("уникальный сотрудник обновлён", "id", id)

	var updated models.UniqueEmployee
	if err := s.db.WithContext(ctx).First(&updated, id).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching updated employee")
	}

	return employeeToResponse(&updated), nil
}

// Delete удаляет уникального сотрудника с проверкой прав.
func (s *uniqueEmployeeService) Delete(ctx context.Context, username string, id int) error {
	ownerInfo, err := s.getEmployeeOwnerInfo(ctx, username)
	if err != nil {
		return err
	}

	var existing models.UniqueEmployee
	if err := s.db.WithContext(ctx).Select("user_id, organization_id, company_id").
		First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Employee not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching employee")
	}

	if !s.canEditEmployee(&existing, ownerInfo) {
		return echo.NewHTTPError(http.StatusForbidden, "You don't have permission to delete this employee")
	}

	result := s.db.WithContext(ctx).Delete(&models.UniqueEmployee{}, id)
	if result.Error != nil {
		slog.Error("не удалось удалить уникального сотрудника", "id", id, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting employee")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Employee not found")
	}

	slog.Info("уникальный сотрудник удалён", "id", id)
	return nil
}

// canEditEmployee проверяет права пользователя на редактирование сотрудника.
func (s *uniqueEmployeeService) canEditEmployee(emp *models.UniqueEmployee, ownerInfo *EmployeeOwnerInfo) bool {
	if emp.UserID != nil && *emp.UserID == ownerInfo.UserID {
		return true
	}
	if emp.OrganizationID != nil && ownerInfo.OrganizationID != nil && *emp.OrganizationID == *ownerInfo.OrganizationID {
		return true
	}
	if emp.CompanyID != nil && ownerInfo.CompanyID != nil && *emp.CompanyID == *ownerInfo.CompanyID {
		return true
	}
	return false
}
