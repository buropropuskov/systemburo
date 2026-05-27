package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// diffUniqueCar сравнивает значимые поля UniqueCar до и после апдейта.
// Возвращает только реально изменившиеся поля.
func diffUniqueCar(before, after *models.UniqueCar) []fieldChange {
	changes := make([]fieldChange, 0)
	addStr := func(field string, oldP, newP *string) {
		if !equalStrPtr(oldP, newP) {
			changes = append(changes, fieldChange{Field: field, Old: copyStrPtr(oldP), New: copyStrPtr(newP)})
		}
	}
	addInt := func(field string, oldP, newP *int) {
		if !equalIntPtr(oldP, newP) {
			changes = append(changes, fieldChange{Field: field, Old: intPtrToStrPtr(oldP), New: intPtrToStrPtr(newP)})
		}
	}
	addStr("number", before.Number, after.Number)
	addStr("mark", before.Mark, after.Mark)
	addInt("organization_id", before.OrganizationID, after.OrganizationID)
	addInt("company_id", before.CompanyID, after.CompanyID)
	addInt("format_id", before.FormatID, after.FormatID)
	addInt("user_id", before.UserID, after.UserID)
	return changes
}

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
	Status               bool       `json:"status"`
	CreatedAt            *time.Time `json:"created_at"`
	OrganizationName     *string    `json:"organization_name"`
	CompanyName          *string    `json:"company_name"`
	FormatName           *string    `json:"format_name"`
	UserName             *string    `json:"user_name"`
	ActiveEntryDateTo    *string    `json:"active_entry_date_to"`
	ActiveEntryTimeFrom  *string    `json:"active_entry_time_from"`
	ActiveEntryTimeTo    *string    `json:"active_entry_time_to"`
	ActiveAppOrgName     *string    `json:"active_app_org_name"`
	ActiveAppCompanyName *string    `json:"active_app_company_name"`
}

// NewUniqueCarRequest -- тело запроса на создание/обновление машины.
type NewUniqueCarRequest struct {
	Number         string `json:"number" validate:"required,min=1,max=50"`
	Mark           string `json:"mark" validate:"omitempty,max=100"`
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
	Number     string              `json:"number" validate:"required"`
	Mark       string              `json:"mark"`
	UpdateData NewUniqueCarRequest `json:"update_data"`
}

// BatchCreateCarsResponse -- результат пакетного создания машин.
type BatchCreateCarsResponse struct {
	CreatedCars  []UniqueCarResponse `json:"created_cars"`
	Errors       []string            `json:"errors"`
	SuccessCount int                 `json:"success_count"`
	ErrorCount   int                 `json:"error_count"`
}

// UniqueCarHistoryItem -- запись истории мастер-машины с username вызывающего.
type UniqueCarHistoryItem struct {
	ID            int       `json:"id"`
	UniqueCarID   int       `json:"unique_car_id"`
	UserID        *int      `json:"user_id"`
	Username      *string   `json:"username"`
	UserLastName  *string   `json:"user_last_name"`
	UserFirstName *string   `json:"user_first_name"`
	ActionType    string    `json:"action_type"`
	FieldName     *string   `json:"field_name"`
	OldValue      *string   `json:"old_value"`
	NewValue      *string   `json:"new_value"`
	Comment       *string   `json:"comment"`
	CreatedAt     time.Time `json:"created_at"`
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
	GetHistory(ctx context.Context, username string, id int) ([]UniqueCarHistoryItem, error)
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

// GetOwnerInfo возвращает информацию о владельце для фильтрации машин.
func (s *uniqueCarService) GetOwnerInfo(ctx context.Context, username string) (*CarOwnerInfo, error) {
	return s.getCarOwnerInfo(ctx, username)
}

// GetAll возвращает список уникальных автомобилей с фильтрацией по типу владельца.
func (s *uniqueCarService) GetAll(ctx context.Context, username string, filterType string) ([]UniqueCarWithRelations, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	query := s.db.WithContext(ctx).
		Table("unique_cars uc").
		Select(`uc.id, uc.number, uc.mark, uc.organization_id, uc.company_id,
			uc.format_id, uc.user_id, uc.created_at,
			o.name as organization_name, c.name as company_name,
			lpf.name as format_name, u.username as user_name,
			COALESCE((
				SELECT true FROM cars cr
				JOIN attachments a ON cr.attachment_id = a.id
				JOIN applications app ON a.application_id = app.id
				WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
				AND cr.status = 1
				AND app.status IN ('В работе', 'Завершено')
				AND CURRENT_DATE <= a.entry_date_to::date
				LIMIT 1
			), false) as status,
			(SELECT a.entry_date_to FROM cars cr
				JOIN attachments a ON cr.attachment_id = a.id
				JOIN applications app ON a.application_id = app.id
				WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
				AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
				AND CURRENT_DATE <= a.entry_date_to::date
				ORDER BY a.entry_date_to DESC LIMIT 1
			) as active_entry_date_to,
			(SELECT a.entry_time_from FROM cars cr
				JOIN attachments a ON cr.attachment_id = a.id
				JOIN applications app ON a.application_id = app.id
				WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
				AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
				AND CURRENT_DATE <= a.entry_date_to::date
				ORDER BY a.entry_date_to DESC LIMIT 1
			) as active_entry_time_from,
			(SELECT a.entry_time_to FROM cars cr
				JOIN attachments a ON cr.attachment_id = a.id
				JOIN applications app ON a.application_id = app.id
				WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
				AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
				AND CURRENT_DATE <= a.entry_date_to::date
				ORDER BY a.entry_date_to DESC LIMIT 1
			) as active_entry_time_to,
			(SELECT ao.name FROM cars cr
				JOIN attachments a ON cr.attachment_id = a.id
				JOIN applications app ON a.application_id = app.id
				LEFT JOIN organizations ao ON app.organization_id = ao.id
				WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
				AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
				AND CURRENT_DATE <= a.entry_date_to::date
				ORDER BY a.entry_date_to DESC LIMIT 1
			) as active_app_org_name,
			(SELECT ac.name FROM cars cr
				JOIN attachments a ON cr.attachment_id = a.id
				JOIN applications app ON a.application_id = app.id
				LEFT JOIN companies ac ON app.company_id = ac.id
				WHERE LOWER(TRIM(cr.car_number)) = LOWER(TRIM(uc.number))
				AND cr.status = 1 AND app.status IN ('В работе', 'Завершено')
				AND CURRENT_DATE <= a.entry_date_to::date
				ORDER BY a.entry_date_to DESC LIMIT 1
			) as active_app_company_name`).
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

// Create создаёт уникальный автомобиль с проверкой уникальности.
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
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Ошибка при создании автомобиля")
	}

	slog.Info("уникальный автомобиль создан", "id", car.ID)
	return carToResponse(&car), nil
}

// CreateBatch создаёт несколько уникальных автомобилей пакетно.
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
			slog.Error("не удалось создать автомобиль в пакетной операции", "number", req.Number, "mark", req.Mark, "error", err)
			errors = append(errors, "Ошибка при создании автомобиля "+req.Number+" "+req.Mark)
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

// Update обновляет уникальный автомобиль по ID с проверкой прав и уникальности.
func (s *uniqueCarService) Update(ctx context.Context, username string, id int, req NewUniqueCarRequest) (*UniqueCarResponse, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	// Полная запись «до апдейта» — нужна для проверки прав и аудита изменений.
	var existing models.UniqueCar
	if err := s.db.WithContext(ctx).First(&existing, id).Error; err != nil {
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

	if err := s.recordCarChanges(ctx, &existing, &updated, ownerInfo.UserID); err != nil {
		slog.Error("не удалось записать аудит изменений автомобиля", "id", id, "error", err)
	}

	return carToResponse(&updated), nil
}

// UpdateByNumber обновляет уникальный автомобиль по номеру и марке.
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

	if err := s.recordCarChanges(ctx, &existing, &updated, ownerInfo.UserID); err != nil {
		slog.Error("не удалось записать аудит изменений автомобиля", "id", existing.ID, "error", err)
	}

	return carToResponse(&updated), nil
}

// Delete удаляет уникальный автомобиль с проверкой прав.
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

// recordCarChanges сравнивает старое и новое состояние UniqueCar
// и пишет по одной записи data_changed на каждое изменённое поле.
func (s *uniqueCarService) recordCarChanges(ctx context.Context, before, after *models.UniqueCar, userID int) error {
	changes := diffUniqueCar(before, after)
	if len(changes) == 0 {
		return nil
	}

	uid := userID
	records := make([]models.UniqueCarHistory, 0, len(changes))
	for _, c := range changes {
		field := c.Field
		records = append(records, models.UniqueCarHistory{
			UniqueCarID: after.ID,
			UserID:      &uid,
			ActionType:  "data_changed",
			FieldName:   &field,
			OldValue:    c.Old,
			NewValue:    c.New,
		})
	}
	if err := s.db.WithContext(ctx).Create(&records).Error; err != nil {
		return fmt.Errorf("create unique_car history: %w", err)
	}
	return nil
}

// GetHistory возвращает историю изменений мастер-записи машины.
// Доступ: у пользователя должны быть права редактирования (canEditCar) -
// иначе он не имеет права видеть аудит.
func (s *uniqueCarService) GetHistory(ctx context.Context, username string, id int) ([]UniqueCarHistoryItem, error) {
	ownerInfo, err := s.getCarOwnerInfo(ctx, username)
	if err != nil {
		return nil, err
	}

	var existing models.UniqueCar
	if err := s.db.WithContext(ctx).Select("user_id, organization_id, company_id").
		First(&existing, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Car not found")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car")
	}
	if !s.canEditCar(&existing, ownerInfo) {
		return nil, echo.NewHTTPError(http.StatusForbidden, "You don't have permission to view this car history")
	}

	items := make([]UniqueCarHistoryItem, 0)
	err = s.db.WithContext(ctx).
		Table("unique_cars_history h").
		Select(`h.id, h.unique_car_id, h.user_id, u.username, u.last_name as user_last_name,
			u.first_name as user_first_name, h.action_type, h.field_name, h.old_value,
			h.new_value, h.comment, h.created_at`).
		Joins("LEFT JOIN users u ON h.user_id = u.id").
		Where("h.unique_car_id = ?", id).
		Order("h.created_at DESC, h.id DESC").
		Scan(&items).Error
	if err != nil {
		slog.Error("failed to load unique_car history", "id", id, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching history")
	}
	return items, nil
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
