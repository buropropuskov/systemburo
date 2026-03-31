package services

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CarOwnerInfo -- информация о владельце для фильтрации машин.
type CarOwnerInfo struct {
	HasOrganization bool `json:"has_organization"`
	HasCompany      bool `json:"has_company"`
	OrganizationID  *int `json:"organization_id"`
	CompanyID       *int `json:"company_id"`
	UserID          int  `json:"user_id"`
}

// UniqueCarWithRelations -- машина с данными связанных сущностей.
type UniqueCarWithRelations struct {
	ID               int        `json:"id"`
	Number           *string    `json:"number"`
	Mark             *string    `json:"mark"`
	OrganizationID   *int       `json:"organization_id"`
	CompanyID        *int       `json:"company_id"`
	FormatID         *int       `json:"format_id"`
	UserID           *int       `json:"user_id"`
	Status           bool       `json:"status"`
	CreatedAt        *time.Time `json:"created_at"`
	OrganizationName *string    `json:"organization_name"`
	CompanyName      *string    `json:"company_name"`
	FormatName       *string    `json:"format_name"`
	UserName         *string    `json:"user_name"`
}

// NewUniqueCarRequest -- тело запроса на создание/обновление машины.
type NewUniqueCarRequest struct {
	Number         string `json:"number" validate:"required,min=1,max=50"`
	Mark           string `json:"mark" validate:"max=100"`
	OrganizationID *int   `json:"organization_id"`
	CompanyID      *int   `json:"company_id"`
	FormatID       *int   `json:"format_id"`
	UserID         *int   `json:"user_id"`
}

// UniqueCarResponse -- ответ при создании/обновлении машины.
type UniqueCarResponse struct {
	ID             int        `json:"id"`
	Number         *string    `json:"number"`
	Mark           *string    `json:"mark"`
	OrganizationID *int       `json:"organization_id"`
	CompanyID      *int       `json:"company_id"`
	FormatID       *int       `json:"format_id"`
	UserID         *int       `json:"user_id"`
	Status         bool       `json:"status"`
	CreatedAt      *time.Time `json:"created_at"`
}

// UpdateCarByNumberRequest -- запрос на обновление машины по номеру и марке.
type UpdateCarByNumberRequest struct {
	Number     string          `json:"number"`
	Mark       string          `json:"mark"`
	UpdateData NewUniqueCarRequest `json:"update_data"`
}

// BatchCreateCarsResponse -- результат пакетного создания машин.
type BatchCreateCarsResponse struct {
	CreatedCars  []UniqueCarResponse `json:"created_cars"`
	Errors       []string            `json:"errors"`
	SuccessCount int                 `json:"success_count"`
	ErrorCount   int                 `json:"error_count"`
}

// UniqueCarService -- интерфейс бизнес-логики уникальных машин.
type UniqueCarService interface {
	GetOwnerInfo(ctx context.Context, username string) (*CarOwnerInfo, error)
	GetAll(ctx context.Context, username string, filterType string) ([]UniqueCarWithRelations, error)
	Create(ctx context.Context, username string, req NewUniqueCarRequest) (*UniqueCarResponse, error)
	CreateBatch(ctx context.Context, username string, reqs []NewUniqueCarRequest) (*BatchCreateCarsResponse, int, error)
	Update(ctx context.Context, username string, id int, req NewUniqueCarRequest) (*UniqueCarResponse, error)
	UpdateByNumber(ctx context.Context, username string, req UpdateCarByNumberRequest) (*UniqueCarResponse, error)
	Delete(ctx context.Context, username string, id int) error
}

type uniqueCarService struct {
	db *gorm.DB
}

// NewUniqueCarService создаёт реализацию UniqueCarService.
func NewUniqueCarService(db *gorm.DB) UniqueCarService {
	return &uniqueCarService{db: db}
}

// getCarOwnerInfo получает информацию о владельце по username.
func (s *uniqueCarService) getCarOwnerInfo(ctx context.Context, username string) (*CarOwnerInfo, error) {
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

	return &CarOwnerInfo{
		HasOrganization: result.HasOrganization,
		HasCompany:      result.HasCompany,
		OrganizationID:  result.OrganizationID,
		CompanyID:       result.CompanyID,
		UserID:          result.UserID,
	}, nil
}

func (s *uniqueCarService) GetOwnerInfo(ctx context.Context, username string) (*CarOwnerInfo, error) {
	return s.getCarOwnerInfo(ctx, username)
}

func (s *uniqueCarService) GetAll(ctx context.Context, username string, filterType string) ([]UniqueCarWithRelations, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).
		Table("unique_cars uc").
		Select(`uc.id, uc.number, uc.mark, uc.organization_id, uc.company_id,
			uc.format_id, uc.user_id, uc.status, uc.created_at,
			o.name as organization_name, c.name as company_name,
			lpf.name as format_name, u.username as user_name`).
		Joins("LEFT JOIN organizations o ON uc.organization_id = o.id").
		Joins("LEFT JOIN companies c ON uc.company_id = c.id").
		Joins("LEFT JOIN license_plate_formats lpf ON uc.format_id = lpf.id").
		Joins("LEFT JOIN users u ON uc.user_id = u.id")

	switch filterType {
	case "organization":
		if ownerInfo.HasOrganization {
			orgID := 0
			if ownerInfo.OrganizationID != nil {
				orgID = *ownerInfo.OrganizationID
			}
			query = query.Where("uc.organization_id = ?", orgID)
		} else {
			query = query.Where("uc.user_id = ?", ownerInfo.UserID)
		}
	case "company":
		if ownerInfo.HasCompany {
			compID := 0
			if ownerInfo.CompanyID != nil {
				compID = *ownerInfo.CompanyID
			}
			query = query.Where("uc.company_id = ?", compID)
		} else {
			query = query.Where("uc.user_id = ?", ownerInfo.UserID)
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
		query = query.Where("uc.user_id = ? OR uc.organization_id = ? OR uc.company_id = ?",
			ownerInfo.UserID, orgID, compID)
	case "all_system":
		// Без фильтрации
	default:
		query = query.Where("uc.user_id = ?", ownerInfo.UserID)
	}

	query = query.Order("uc.number, uc.mark")

	cars := make([]UniqueCarWithRelations, 0)
	if err := query.Scan(&cars).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars")
	}

	return cars, nil
}

// carToResponse конвертирует модель UniqueCar в UniqueCarResponse.
func carToResponse(car *models.UniqueCar) *UniqueCarResponse {
	status := false
	if car.Status != nil && *car.Status {
		status = true
	}
	return &UniqueCarResponse{
		ID:             car.ID,
		Number:         car.Number,
		Mark:           car.Mark,
		OrganizationID: car.OrganizationID,
		CompanyID:      car.CompanyID,
		FormatID:       car.FormatID,
		UserID:         car.UserID,
		Status:         status,
		CreatedAt:      &car.CreatedAt,
	}
}

func (s *uniqueCarService) Create(ctx context.Context, username string, req NewUniqueCarRequest) (*UniqueCarResponse, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	// Проверка уникальности для пользователя
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
		Where("user_id = ? AND number = ? AND mark = ?", ownerInfo.UserID, req.Number, req.Mark).
		Count(&count).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
	}
	if count > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль уже привязан к вашему аккаунту")
	}

	// Проверка уникальности для организации
	if req.OrganizationID != nil {
		var orgCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
			Where("organization_id = ? AND number = ? AND mark = ?", *req.OrganizationID, req.Number, req.Mark).
			Count(&orgCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
		}
		if orgCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль с этим номером и маркой уже существует в этой организации")
		}
	}

	// Проверка уникальности для компании
	if req.CompanyID != nil {
		var compCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
			Where("company_id = ? AND number = ? AND mark = ?", *req.CompanyID, req.Number, req.Mark).
			Count(&compCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
		}
		if compCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль с этим номером и маркой уже существует в этой компании")
		}
	}

	userID := ownerInfo.UserID
	if req.UserID != nil {
		userID = *req.UserID
	}

	statusFalse := false
	car := models.UniqueCar{
		Number:         &req.Number,
		Mark:           &req.Mark,
		OrganizationID: req.OrganizationID,
		CompanyID:      req.CompanyID,
		FormatID:       req.FormatID,
		UserID:         &userID,
		Status:         &statusFalse,
	}

	if err := s.db.WithContext(ctx).Create(&car).Error; err != nil {
		slog.Error("не удалось создать уникальный автомобиль", "error", err)
		return nil, echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	slog.Info("уникальный автомобиль создан", "id", car.ID)
	return carToResponse(&car), nil
}

func (s *uniqueCarService) CreateBatch(ctx context.Context, username string, reqs []NewUniqueCarRequest) (*BatchCreateCarsResponse, int, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, 0, err
	}

	createdCars := make([]UniqueCarResponse, 0)
	errors := make([]string, 0)

	for _, req := range reqs {
		// Проверка уникальности для пользователя
		var count int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
			Where("user_id = ? AND number = ? AND mark = ?", ownerInfo.UserID, req.Number, req.Mark).
			Count(&count).Error; err != nil {
			return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
		}
		if count > 0 {
			errors = append(errors, "Автомобиль "+req.Number+" "+req.Mark+" уже привязан к вашему аккаунту")
			continue
		}

		// Проверка уникальности для организации
		if req.OrganizationID != nil {
			var orgCount int64
			if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
				Where("organization_id = ? AND number = ? AND mark = ?", *req.OrganizationID, req.Number, req.Mark).
				Count(&orgCount).Error; err != nil {
				return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
			}
			if orgCount > 0 {
				errors = append(errors, "Автомобиль "+req.Number+" "+req.Mark+" уже существует в этой организации")
				continue
			}
		}

		// Проверка уникальности для компании
		if req.CompanyID != nil {
			var compCount int64
			if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
				Where("company_id = ? AND number = ? AND mark = ?", *req.CompanyID, req.Number, req.Mark).
				Count(&compCount).Error; err != nil {
				return nil, 0, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
			}
			if compCount > 0 {
				errors = append(errors, "Автомобиль "+req.Number+" "+req.Mark+" уже существует в этой компании")
				continue
			}
		}

		userID := ownerInfo.UserID
		if req.UserID != nil {
			userID = *req.UserID
		}

		statusFalse := false
		car := models.UniqueCar{
			Number:         &req.Number,
			Mark:           &req.Mark,
			OrganizationID: req.OrganizationID,
			CompanyID:      req.CompanyID,
			FormatID:       req.FormatID,
			UserID:         &userID,
			Status:         &statusFalse,
		}

		if err := s.db.WithContext(ctx).Create(&car).Error; err != nil {
			errors = append(errors, "Ошибка при создании автомобиля "+req.Number+" "+req.Mark+": "+err.Error())
			continue
		}

		createdCars = append(createdCars, *carToResponse(&car))
	}

	httpStatus := http.StatusOK
	if len(errors) > 0 {
		httpStatus = http.StatusMultiStatus
	}

	return &BatchCreateCarsResponse{
		CreatedCars:  createdCars,
		Errors:       errors,
		SuccessCount: len(createdCars),
		ErrorCount:   len(errors),
	}, httpStatus, nil
}

func (s *uniqueCarService) Update(ctx context.Context, username string, id int, req NewUniqueCarRequest) (*UniqueCarResponse, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	// Проверяем существование и права
	var existing models.UniqueCar
	if err := s.db.WithContext(ctx).Select("user_id, organization_id, company_id").
		First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Car not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car")
	}

	if !s.canEditCar(&existing, ownerInfo) {
		return nil, echo.NewHTTPError(http.StatusForbidden, "You don't have permission to edit this car")
	}

	// Проверка уникальности для пользователя (исключая текущую)
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
		Where("user_id = ? AND number = ? AND mark = ? AND id != ?", ownerInfo.UserID, req.Number, req.Mark, id).
		Count(&count).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
	}
	if count > 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль уже привязан к вашему аккаунту")
	}

	// Проверка уникальности для организации (исключая текущую)
	if req.OrganizationID != nil {
		var orgCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
			Where("organization_id = ? AND number = ? AND mark = ? AND id != ?", *req.OrganizationID, req.Number, req.Mark, id).
			Count(&orgCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
		}
		if orgCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль с этим номером и маркой уже существует в этой организации")
		}
	}

	// Проверка уникальности для компании (исключая текущую)
	if req.CompanyID != nil {
		var compCount int64
		if err := s.db.WithContext(ctx).Model(&models.UniqueCar{}).
			Where("company_id = ? AND number = ? AND mark = ? AND id != ?", *req.CompanyID, req.Number, req.Mark, id).
			Count(&compCount).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking car uniqueness")
		}
		if compCount > 0 {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "Автомобиль с этим номером и маркой уже существует в этой компании")
		}
	}

	userID := ownerInfo.UserID
	if req.UserID != nil {
		userID = *req.UserID
	}

	result := s.db.WithContext(ctx).Model(&models.UniqueCar{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"number":          req.Number,
			"mark":            req.Mark,
			"organization_id": req.OrganizationID,
			"company_id":      req.CompanyID,
			"format_id":       req.FormatID,
			"user_id":         userID,
		})
	if result.Error != nil {
		slog.Error("не удалось обновить уникальный автомобиль", "id", id, "error", result.Error)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating car")
	}
	slog.Info("уникальный автомобиль обновлён", "id", id)

	var updated models.UniqueCar
	if err := s.db.WithContext(ctx).First(&updated, id).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching updated car")
	}

	return carToResponse(&updated), nil
}

func (s *uniqueCarService) UpdateByNumber(ctx context.Context, username string, req UpdateCarByNumberRequest) (*UniqueCarResponse, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	var existing models.UniqueCar
	if err := s.db.WithContext(ctx).
		Where("number = ? AND mark = ?", req.Number, req.Mark).
		First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Car not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car")
	}

	if !s.canEditCar(&existing, ownerInfo) {
		return nil, echo.NewHTTPError(http.StatusForbidden, "You don't have permission to edit this car")
	}

	userID := ownerInfo.UserID
	if req.UpdateData.UserID != nil {
		userID = *req.UpdateData.UserID
	}

	result := s.db.WithContext(ctx).Model(&models.UniqueCar{}).Where("id = ?", existing.ID).
		Updates(map[string]interface{}{
			"number":          req.UpdateData.Number,
			"mark":            req.UpdateData.Mark,
			"organization_id": req.UpdateData.OrganizationID,
			"company_id":      req.UpdateData.CompanyID,
			"format_id":       req.UpdateData.FormatID,
			"user_id":         userID,
		})
	if result.Error != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error updating car")
	}

	var updated models.UniqueCar
	if err := s.db.WithContext(ctx).First(&updated, existing.ID).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching updated car")
	}

	return carToResponse(&updated), nil
}

func (s *uniqueCarService) Delete(ctx context.Context, username string, id int) error {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return err
	}

	var existing models.UniqueCar
	if err := s.db.WithContext(ctx).Select("user_id, organization_id, company_id").
		First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Car not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car")
	}

	if !s.canEditCar(&existing, ownerInfo) {
		return echo.NewHTTPError(http.StatusForbidden, "You don't have permission to delete this car")
	}

	result := s.db.WithContext(ctx).Delete(&models.UniqueCar{}, id)
	if result.Error != nil {
		slog.Error("не удалось удалить уникальный автомобиль", "id", id, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting car")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Car not found")
	}

	slog.Info("уникальный автомобиль удалён", "id", id)
	return nil
}

// canEditCar проверяет права пользователя на редактирование машины.
func (s *uniqueCarService) canEditCar(car *models.UniqueCar, ownerInfo *CarOwnerInfo) bool {
	if car.UserID != nil && *car.UserID == ownerInfo.UserID {
		return true
	}
	if car.OrganizationID != nil && ownerInfo.OrganizationID != nil && *car.OrganizationID == *ownerInfo.OrganizationID {
		return true
	}
	if car.CompanyID != nil && ownerInfo.CompanyID != nil && *car.CompanyID == *ownerInfo.CompanyID {
		return true
	}
	return false
}
