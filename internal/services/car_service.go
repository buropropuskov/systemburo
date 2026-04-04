package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CarService -- интерфейс бизнес-логики автомобилей в заявках.
type CarService interface {
	// CreateCar создаёт автомобиль и связи с местами разгрузки (транзакция).
	CreateCar(ctx context.Context, req CreateCarRequest, userID int) (*CreateCarResponse, error)
	// GetActiveCarsForTables возвращает активные машины для всех таблиц (без «по факту»).
	GetActiveCarsForTables(ctx context.Context) ([]TableCarResponse, error)
	// GetFactCarsForTables возвращает машины с номером «по факту».
	GetFactCarsForTables(ctx context.Context) ([]TableCarResponse, error)
	// GetCarUnloadPlaces возвращает связи активных машин с местами разгрузки.
	GetCarUnloadPlaces(ctx context.Context) ([]CarUnloadPlaceInfo, error)
	// GetFactCarUnloadPlaces возвращает связи «по факту» машин с местами разгрузки.
	GetFactCarUnloadPlaces(ctx context.Context) ([]CarUnloadPlaceInfo, error)
	// CheckActiveCar проверяет наличие активной машины по номеру, марке и организации/компании.
	CheckActiveCar(ctx context.Context, req CheckActiveCarRequest) (*CheckActiveCarResponse, error)
	// GetCarHistory возвращает историю конкретного автомобиля.
	GetCarHistory(ctx context.Context, carID int) ([]CarHistoryItemResponse, error)
	// AddCarHistoryEntry добавляет запись в историю автомобиля.
	AddCarHistoryEntry(ctx context.Context, carID int, req AddCarHistoryRequest) error
	// GetAllCarsHistory возвращает историю въездов/выездов всех автомобилей.
	GetAllCarsHistory(ctx context.Context) ([]AllCarsHistoryItem, error)
	// GetCarsCurrentStatus возвращает текущий территориальный статус активных машин.
	GetCarsCurrentStatus(ctx context.Context) ([]CarCurrentStatus, error)
	// UpdateCarTerritoryStatus обновляет статус нахождения на территории (въезд/выезд).
	UpdateCarTerritoryStatus(ctx context.Context, carID int, req UpdateTerritoryStatusRequest) error
	// DeactivateCar деактивирует автомобиль (мягкое удаление).
	DeactivateCar(ctx context.Context, carID int, req DeactivateCarRequest) error
	// ActivateCar вводит автомобиль в работу.
	ActivateCar(ctx context.Context, carID int, req ActivateCarRequest) error
	// RestoreCar восстанавливает удалённый автомобиль.
	RestoreCar(ctx context.Context, carID int, req RestoreCarRequest) error
	// GetUnifiedCarHistory возвращает объединённую историю для всех машин с одинаковыми параметрами.
	GetUnifiedCarHistory(ctx context.Context, req UnifiedCarHistoryQuery) ([]CarHistoryItemResponse, error)
}

// --- DTO запросов ---

// CreateCarRequest -- тело запроса на создание автомобиля.
type CreateCarRequest struct {
	CarNumber     string  `json:"car_number"`
	CarBrand      string  `json:"car_brand"`
	UnloadPlace   *string `json:"unload_place"`
	EntryDateFrom *string `json:"entry_date_from"`
	EntryTimeFrom *string `json:"entry_time_from"`
	EntryDateTo   *string `json:"entry_date_to"`
	EntryTimeTo   *string `json:"entry_time_to"`
	UnloadPlaces  []int   `json:"unload_places"`
}

// CreateCarResponse -- ответ после создания автомобиля.
type CreateCarResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	CarID   int    `json:"car_id"`
}

// CheckActiveCarRequest -- параметры запроса проверки активной машины.
type CheckActiveCarRequest struct {
	CarNumber      string `json:"car_number" query:"car_number"`
	CarBrand       string `json:"car_brand" query:"car_brand"`
	OrganizationID *int   `json:"organization_id" query:"organization_id"`
	CompanyID      *int   `json:"company_id" query:"company_id"`
}

// CheckActiveCarResponse -- ответ проверки активной машины.
type CheckActiveCarResponse struct {
	Active            bool    `json:"active"`
	CarID             *int    `json:"car_id,omitempty"`
	CarNumber         *string `json:"car_number,omitempty"`
	CarBrand          *string `json:"car_brand,omitempty"`
	EntryDateTo       *string `json:"entry_date_to,omitempty"`
	EntryTimeTo       *string `json:"entry_time_to,omitempty"`
	ApplicationID     *int    `json:"application_id,omitempty"`
	ApplicationNumber *string `json:"application_number,omitempty"`
	OrganizationName  *string `json:"organization_name,omitempty"`
	CompanyName       *string `json:"company_name,omitempty"`
}

// AddCarHistoryRequest -- тело запроса на добавление записи в историю автомобиля.
type AddCarHistoryRequest struct {
	UserID     *int             `json:"user_id"`
	ActionType string           `json:"action_type"`
	FieldName  *string          `json:"field_name"`
	OldValue   *string          `json:"old_value"`
	NewValue   *string          `json:"new_value"`
	Comment    *string          `json:"comment"`
	Metadata   *json.RawMessage `json:"metadata" swaggertype:"object"`
}

// UpdateTerritoryStatusRequest -- тело запроса обновления территориального статуса.
type UpdateTerritoryStatusRequest struct {
	TerritoryStatus int  `json:"territory_status"`
	UserID          *int `json:"user_id"`
}

// DeactivateCarRequest -- тело запроса деактивации автомобиля.
type DeactivateCarRequest struct {
	Status int  `json:"status"`
	UserID *int `json:"user_id"`
}

// ActivateCarRequest -- тело запроса активации автомобиля.
type ActivateCarRequest struct {
	UserID *int `json:"user_id"`
}

// RestoreCarRequest -- тело запроса восстановления автомобиля.
type RestoreCarRequest struct {
	UserID *int `json:"user_id"`
}

// UnifiedCarHistoryQuery -- параметры запроса объединённой истории.
type UnifiedCarHistoryQuery struct {
	CarNumber      string `json:"car_number" query:"car_number"`
	CarBrand       string `json:"car_brand" query:"car_brand"`
	OrganizationID *int   `json:"organization_id" query:"organization_id"`
	CompanyID      *int   `json:"company_id" query:"company_id"`
}

// --- DTO ответов ---

// TableCarResponse -- автомобиль для отображения в таблице.
type TableCarResponse struct {
	ID                 int       `json:"id"`
	CarNumber          string    `json:"car_number"`
	CarBrand           string    `json:"car_brand"`
	Organization       *string   `json:"organization"`
	OrganizationID     *int      `json:"organization_id"`
	Company            *string   `json:"company"`
	CompanyID          *int      `json:"company_id"`
	UnloadPlace        *string   `json:"unload_place"`
	UnloadPlaces       []string  `json:"unload_places"`
	EntryDateTo        *string   `json:"entry_date_to"`
	EntryTimeFrom      *string   `json:"entry_time_from"`
	EntryTimeTo        *string   `json:"entry_time_to"`
	Status             int       `json:"status"`
	ApplicationID      *int      `json:"application_id"`
	TerritoryStatus    *int      `json:"territory_status"`
	TerritoryEntryTime *string   `json:"territory_entry_time"`
}

// CarUnloadPlaceInfo -- связь автомобиля с местом разгрузки.
type CarUnloadPlaceInfo struct {
	CarID           int    `json:"car_id"`
	UnloadPlaceID   int    `json:"unload_place_id"`
	UnloadPlaceName string `json:"unload_place_name"`
}

// CarHistoryItemResponse -- элемент истории автомобиля.
type CarHistoryItemResponse struct {
	ID           int              `json:"id"`
	CarID        int              `json:"car_id"`
	UserID       *int             `json:"user_id"`
	UserName     string           `json:"user_name"`
	LastName     *string          `json:"last_name"`
	FirstName    *string          `json:"first_name"`
	MiddleName   *string          `json:"middle_name"`
	ActionType   string           `json:"action_type"`
	FieldName    *string          `json:"field_name"`
	OldValue     *string          `json:"old_value"`
	NewValue     *string          `json:"new_value"`
	Comment      *string          `json:"comment"`
	CreatedAt    string           `json:"created_at"`
	Metadata     *json.RawMessage `json:"metadata" swaggertype:"object"`
	CarNumber    *string          `json:"car_number"`
	CarBrand     *string          `json:"car_brand"`
	Organization *string          `json:"organization"`
	Company      *string          `json:"company"`
}

// AllCarsHistoryItem -- элемент общей истории (только entry/exit).
type AllCarsHistoryItem struct {
	ID           int     `json:"id"`
	CarID        int     `json:"car_id"`
	UserID       *int    `json:"user_id"`
	UserName     string  `json:"user_name"`
	ActionType   string  `json:"action_type"`
	Comment      *string `json:"comment"`
	CreatedAt    string  `json:"created_at"`
	CarNumber    *string `json:"car_number"`
	CarBrand     *string `json:"car_brand"`
	Organization *string `json:"organization"`
	Company      *string `json:"company"`
}

// CarCurrentStatus -- текущий территориальный статус автомобиля.
type CarCurrentStatus struct {
	CarID           int     `json:"car_id"`
	TerritoryStatus int     `json:"territory_status"`
	EntryTime       *string `json:"entry_time"`
	LastExitTime    *string `json:"last_exit_time"`
}

// --- Вспомогательные raw-структуры для сканирования SQL результатов ---

type tableCarRow struct {
	ID                 int
	CarNumber          string
	CarBrand           string
	UnloadPlace        *string
	TerritoryStatus    *int
	TerritoryEntryTime *time.Time
	Organization       *string
	OrganizationID     *int
	Company            *string
	CompanyID          *int
	EntryDateTo        *string
	EntryTimeFrom      *string
	EntryTimeTo        *string
	Status             *int
	ApplicationID      *int
}

type carHistoryRow struct {
	ID           int
	CarID        int
	UserID       *int
	UserName     string
	LastName     *string
	FirstName    *string
	MiddleName   *string
	ActionType   string
	FieldName    *string
	OldValue     *string
	NewValue     *string
	Comment      *string
	CreatedAt    time.Time
	Metadata     *string
	CarNumber    *string
	CarBrand     *string
	Organization *string
	Company      *string
}

// --- Реализация ---

type carService struct {
	db *gorm.DB
}

// NewCarService создаёт новый экземпляр CarService.
func NewCarService(db *gorm.DB) CarService {
	return &carService{db: db}
}

// CreateCar создаёт автомобиль с привязкой к местам разгрузки и записью в историю.
func (s *carService) CreateCar(ctx context.Context, req CreateCarRequest, userID int) (*CreateCarResponse, error) {
	var carID int

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		statusZero := 0
		car := models.Car{
			CarNumber:       &req.CarNumber,
			CarBrand:        &req.CarBrand,
			UnloadPlace:     req.UnloadPlace,
			EntryDateFrom:   req.EntryDateFrom,
			EntryTimeFrom:   req.EntryTimeFrom,
			EntryDateTo:     req.EntryDateTo,
			EntryTimeTo:     req.EntryTimeTo,
			Status:          &statusZero,
			TerritoryStatus: &statusZero,
		}
		if err := tx.Create(&car).Error; err != nil {
			slog.Error("не удалось создать автомобиль", "car_number", req.CarNumber, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error creating car")
		}
		carID = car.ID

		for _, placeID := range req.UnloadPlaces {
			orderIdx := 1
			cup := models.CarUnloadPlace{
				CarID:         carID,
				UnloadPlaceID: placeID,
				OrderIndex:    &orderIdx,
			}
			if err := tx.Create(&cup).Error; err != nil {
				slog.Error("не удалось создать связь автомобиля с местом разгрузки", "car_id", carID, "unload_place_id", placeID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating car unload place")
			}
		}

		comment := fmt.Sprintf("Автомобиль %s %s создан", req.CarNumber, req.CarBrand)
		actionType := "create"
		history := models.CarHistory{
			CarID:      carID,
			UserID:     &userID,
			ActionType: actionType,
			Comment:    &comment,
		}
		if err := tx.Create(&history).Error; err != nil {
			slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	slog.Info("автомобиль создан", "id", carID, "car_number", req.CarNumber, "user_id", userID)
	return &CreateCarResponse{
		Success: true,
		Message: "Car created successfully",
		CarID:   carID,
	}, nil
}

// GetActiveCarsForTables возвращает активные автомобили для всех таблиц (без «по факту»).
func (s *carService) GetActiveCarsForTables(ctx context.Context) ([]TableCarResponse, error) {
	rows := make([]tableCarRow, 0)
	err := s.db.WithContext(ctx).
		Table("cars c").
		Select(`c.id, c.car_number, c.car_brand, c.unload_place,
			c.territory_status, c.territory_entry_time,
			o.name AS organization, o.id AS organization_id,
			c2.name AS company, c2.id AS company_id,
			c.entry_date_to, c.entry_time_from, c.entry_time_to,
			c.status, app.id AS application_id`).
		Joins("JOIN attachments a ON c.attachment_id = a.id").
		Joins("JOIN applications app ON a.application_id = app.id").
		Joins("LEFT JOIN organizations o ON app.organization_id = o.id").
		Joins("LEFT JOIN companies c2 ON app.company_id = c2.id").
		Where("c.status = ?", 1).
		Where("app.confirmation = ?", models.ConfirmationApproved).
		Where("app.status IN ?", []string{models.StatusInWork, models.StatusCompleted}).
		Where("LOWER(TRIM(c.car_number)) != ?", "по факту").
		Order("c.car_number").
		Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching active cars")
	}

	return s.enrichTableCars(ctx, rows)
}

// GetFactCarsForTables возвращает автомобили с номером «по факту».
func (s *carService) GetFactCarsForTables(ctx context.Context) ([]TableCarResponse, error) {
	rows := make([]tableCarRow, 0)

	baseQuery := func() *gorm.DB {
		return s.db.WithContext(ctx).
			Table("cars c").
			Select(`c.id, c.car_number, c.car_brand, c.unload_place,
				c.territory_status, c.territory_entry_time,
				o.name AS organization, o.id AS organization_id,
				c2.name AS company, c2.id AS company_id,
				c.entry_date_to, c.entry_time_from, c.entry_time_to,
				c.status, app.id AS application_id`).
			Joins("JOIN attachments a ON c.attachment_id = a.id").
			Joins("JOIN applications app ON a.application_id = app.id").
			Joins("LEFT JOIN organizations o ON app.organization_id = o.id").
			Joins("LEFT JOIN companies c2 ON app.company_id = c2.id").
			Where("c.status = ?", 1).
			Where("app.confirmation = ?", models.ConfirmationApproved).
			Where("app.status IN ?", []string{models.StatusInWork, models.StatusCompleted}).
			Order("organization, c.entry_date_to")
	}

	err := baseQuery().
		Where("LOWER(TRIM(c.car_number)) = ?", "по факту").
		Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching fact cars")
	}

	if len(rows) == 0 {
		err = baseQuery().
			Where("c.car_number ILIKE ? OR c.car_number ILIKE ? OR c.car_number ILIKE ?",
				"%по факту%", "%пофакту%", "%факт%").
			Scan(&rows).Error
		if err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching alternative fact cars")
		}
	}

	return s.enrichTableCars(ctx, rows)
}

// enrichTableCars добавляет unload_places к каждому автомобилю.
func (s *carService) enrichTableCars(ctx context.Context, rows []tableCarRow) ([]TableCarResponse, error) {
	if len(rows) == 0 {
		return []TableCarResponse{}, nil
	}

	carIDs := make([]int, len(rows))
	for i, row := range rows {
		carIDs[i] = row.ID
	}

	type placeRow struct {
		CarID int
		Name  string
	}
	var allPlaces []placeRow
	err := s.db.WithContext(ctx).
		Table("car_unload_places cup").
		Select("cup.car_id, up.name").
		Joins("JOIN unload_places up ON cup.unload_place_id = up.id").
		Where("cup.car_id IN ?", carIDs).
		Order("cup.car_id, cup.order_index").
		Scan(&allPlaces).Error
	if err != nil {
		slog.Error("не удалось загрузить места разгрузки", "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unload places")
	}

	placesByCarID := make(map[int][]string)
	for _, p := range allPlaces {
		placesByCarID[p.CarID] = append(placesByCarID[p.CarID], p.Name)
	}

	cars := make([]TableCarResponse, 0, len(rows))
	for _, row := range rows {
		status := 0
		if row.Status != nil {
			status = *row.Status
		}

		var territoryEntryTimeStr *string
		if row.TerritoryEntryTime != nil {
			s := row.TerritoryEntryTime.Add(3 * time.Hour).Format("2006-01-02T15:04:05+03:00")
			territoryEntryTimeStr = &s
		}

		places := placesByCarID[row.ID]
		if places == nil {
			places = []string{}
		}

		cars = append(cars, TableCarResponse{
			ID:                 row.ID,
			CarNumber:          row.CarNumber,
			CarBrand:           row.CarBrand,
			Organization:       row.Organization,
			OrganizationID:     row.OrganizationID,
			Company:            row.Company,
			CompanyID:          row.CompanyID,
			UnloadPlace:        row.UnloadPlace,
			UnloadPlaces:       places,
			EntryDateTo:        row.EntryDateTo,
			EntryTimeFrom:      row.EntryTimeFrom,
			EntryTimeTo:        row.EntryTimeTo,
			Status:             status,
			ApplicationID:      row.ApplicationID,
			TerritoryStatus:    row.TerritoryStatus,
			TerritoryEntryTime: territoryEntryTimeStr,
		})
	}
	return cars, nil
}

// GetCarUnloadPlaces возвращает связи активных автомобилей с местами разгрузки.
func (s *carService) GetCarUnloadPlaces(ctx context.Context) ([]CarUnloadPlaceInfo, error) {
	places := make([]CarUnloadPlaceInfo, 0)
	err := s.db.WithContext(ctx).
		Table("car_unload_places cup").
		Select("cup.car_id, cup.unload_place_id, up.name AS unload_place_name").
		Joins("JOIN unload_places up ON cup.unload_place_id = up.id").
		Joins("JOIN cars c ON cup.car_id = c.id").
		Joins("JOIN attachments a ON c.attachment_id = a.id").
		Joins("JOIN applications app ON a.application_id = app.id").
		Where("c.status = ?", 1).
		Where("app.confirmation = ?", models.ConfirmationApproved).
		Where("app.status IN ?", []string{models.StatusInWork, models.StatusCompleted}).
		Order("cup.car_id, cup.order_index").
		Scan(&places).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car unload places")
	}
	return places, nil
}

// GetFactCarUnloadPlaces возвращает связи «по факту» автомобилей с местами разгрузки.
func (s *carService) GetFactCarUnloadPlaces(ctx context.Context) ([]CarUnloadPlaceInfo, error) {
	places := make([]CarUnloadPlaceInfo, 0)
	err := s.db.WithContext(ctx).
		Table("car_unload_places cup").
		Select("cup.car_id, cup.unload_place_id, up.name AS unload_place_name").
		Joins("JOIN unload_places up ON cup.unload_place_id = up.id").
		Joins("JOIN cars c ON cup.car_id = c.id").
		Joins("JOIN attachments a ON c.attachment_id = a.id").
		Joins("JOIN applications app ON a.application_id = app.id").
		Where("c.status = ?", 1).
		Where("app.confirmation = ?", models.ConfirmationApproved).
		Where("app.status IN ?", []string{models.StatusInWork, models.StatusCompleted}).
		Where("LOWER(TRIM(c.car_number)) = ?", "по факту").
		Order("cup.car_id, cup.order_index").
		Scan(&places).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching fact car unload places")
	}
	return places, nil
}

// CheckActiveCar проверяет наличие активного автомобиля по номеру, марке и организации.
func (s *carService) CheckActiveCar(ctx context.Context, req CheckActiveCarRequest) (*CheckActiveCarResponse, error) {
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	currentTime := now.Format("15:04:05")

	type checkRow struct {
		ID                int
		CarNumber         *string
		CarBrand          *string
		EntryDateTo       *string
		EntryTimeTo       *string
		ApplicationID     int
		ApplicationNumber *string
		OrganizationName  *string
		CompanyName       *string
	}

	var row checkRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			c.id,
			c.car_number,
			c.car_brand,
			c.entry_date_to,
			c.entry_time_to,
			a.application_id,
			app.application_number,
			COALESCE(o.name, '') AS organization_name,
			COALESCE(comp.name, '') AS company_name
		FROM cars c
		JOIN attachments a ON c.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		LEFT JOIN organizations o ON app.organization_id = o.id
		LEFT JOIN companies comp ON app.company_id = comp.id
		WHERE c.status = 1
		AND LOWER(TRIM(c.car_number)) = LOWER(TRIM(?))
		AND LOWER(TRIM(c.car_brand)) = LOWER(TRIM(?))
		AND (
			(?::integer IS NULL AND app.organization_id IS NULL)
			OR app.organization_id = ?
		)
		AND (
			(?::integer IS NULL AND app.company_id IS NULL)
			OR app.company_id = ?
		)
		AND (
			c.entry_date_to > ?
			OR (c.entry_date_to = ? AND c.entry_time_to > ?)
		)
		LIMIT 1
	`, req.CarNumber, req.CarBrand,
		req.OrganizationID, req.OrganizationID,
		req.CompanyID, req.CompanyID,
		today, today, currentTime,
	).Scan(&row).Error

	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error checking active car")
	}

	if row.ID == 0 {
		return &CheckActiveCarResponse{Active: false}, nil
	}

	appID := row.ApplicationID
	return &CheckActiveCarResponse{
		Active:            true,
		CarID:             &row.ID,
		CarNumber:         row.CarNumber,
		CarBrand:          row.CarBrand,
		EntryDateTo:       row.EntryDateTo,
		EntryTimeTo:       row.EntryTimeTo,
		ApplicationID:     &appID,
		ApplicationNumber: row.ApplicationNumber,
		OrganizationName:  row.OrganizationName,
		CompanyName:       row.CompanyName,
	}, nil
}

// GetCarHistory возвращает историю конкретного автомобиля.
func (s *carService) GetCarHistory(ctx context.Context, carID int) ([]CarHistoryItemResponse, error) {
	rows := make([]carHistoryRow, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			h.id,
			h.car_id,
			h.user_id,
			CONCAT(
				COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) AS user_name,
			u.last_name,
			u.first_name,
			u.middle_name,
			h.action_type,
			h.field_name,
			h.old_value,
			h.new_value,
			h.comment,
			h.created_at,
			h.metadata::text AS metadata
		FROM cars_history h
		LEFT JOIN users u ON h.user_id = u.id
		WHERE h.car_id = ?
		ORDER BY h.created_at DESC
	`, carID).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching car history")
	}

	return s.mapHistoryRows(rows, false), nil
}

// AddCarHistoryEntry добавляет запись в историю автомобиля.
func (s *carService) AddCarHistoryEntry(ctx context.Context, carID int, req AddCarHistoryRequest) error {
	var metadataStr *string
	if req.Metadata != nil {
		s := string(*req.Metadata)
		metadataStr = &s
	}

	history := models.CarHistory{
		CarID:      carID,
		UserID:     req.UserID,
		ActionType: req.ActionType,
		FieldName:  req.FieldName,
		OldValue:   req.OldValue,
		NewValue:   req.NewValue,
		Comment:    req.Comment,
		Metadata:   metadataStr,
	}
	if err := s.db.WithContext(ctx).Create(&history).Error; err != nil {
		slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", req.ActionType, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
	}
	slog.Info("запись в историю автомобиля добавлена", "car_id", carID, "action_type", req.ActionType)
	return nil
}

// GetAllCarsHistory возвращает историю въездов/выездов всех автомобилей.
func (s *carService) GetAllCarsHistory(ctx context.Context) ([]AllCarsHistoryItem, error) {
	type allHistRow struct {
		ID           int
		CarID        int
		UserID       *int
		UserName     string
		ActionType   string
		Comment      *string
		CreatedAt    time.Time
		CarNumber    *string
		CarBrand     *string
		Organization *string
		Company      *string
	}

	rows := make([]allHistRow, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			h.id,
			h.car_id,
			h.user_id,
			CONCAT(
				COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) AS user_name,
			h.action_type,
			h.comment,
			h.created_at,
			c.car_number,
			c.car_brand,
			COALESCE(o.name, '') AS organization,
			COALESCE(c2.name, '') AS company
		FROM cars_history h
		LEFT JOIN users u ON h.user_id = u.id
		JOIN cars c ON h.car_id = c.id
		LEFT JOIN attachments a ON c.attachment_id = a.id
		LEFT JOIN applications app ON a.application_id = app.id
		LEFT JOIN organizations o ON app.organization_id = o.id
		LEFT JOIN companies c2 ON app.company_id = c2.id
		WHERE h.action_type IN ('entry', 'exit')
		ORDER BY h.created_at DESC
	`).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching all cars history")
	}

	items := make([]AllCarsHistoryItem, 0, len(rows))
	for _, r := range rows {
		userName := r.UserName
		if strings.TrimSpace(userName) == "" {
			userName = "Система"
		}
		mskTime := r.CreatedAt.Add(3 * time.Hour)
		items = append(items, AllCarsHistoryItem{
			ID:           r.ID,
			CarID:        r.CarID,
			UserID:       r.UserID,
			UserName:     userName,
			ActionType:   r.ActionType,
			Comment:      r.Comment,
			CreatedAt:    mskTime.Format("2006-01-02T15:04:05+03:00"),
			CarNumber:    r.CarNumber,
			CarBrand:     r.CarBrand,
			Organization: r.Organization,
			Company:      r.Company,
		})
	}
	return items, nil
}

// GetCarsCurrentStatus возвращает текущий территориальный статус активных автомобилей.
func (s *carService) GetCarsCurrentStatus(ctx context.Context) ([]CarCurrentStatus, error) {
	type statusRow struct {
		ID                 int
		TerritoryStatus    *int
		TerritoryEntryTime *time.Time
		LastExitTime       *time.Time
	}

	rows := make([]statusRow, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			c.id,
			c.territory_status,
			c.territory_entry_time,
			(
				SELECT created_at
				FROM cars_history
				WHERE car_id = c.id AND action_type = 'exit'
				ORDER BY created_at DESC
				LIMIT 1
			) AS last_exit_time
		FROM cars c
		WHERE c.status = 1
	`).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars status")
	}

	items := make([]CarCurrentStatus, 0, len(rows))
	for _, r := range rows {
		ts := 0
		if r.TerritoryStatus != nil {
			ts = *r.TerritoryStatus
		}
		var entryTimeStr *string
		if r.TerritoryEntryTime != nil {
			s := r.TerritoryEntryTime.Add(3 * time.Hour).Format("2006-01-02T15:04:05+03:00")
			entryTimeStr = &s
		}
		var lastExitStr *string
		if r.LastExitTime != nil {
			s := r.LastExitTime.Add(3 * time.Hour).Format("2006-01-02T15:04:05+03:00")
			lastExitStr = &s
		}
		items = append(items, CarCurrentStatus{
			CarID:           r.ID,
			TerritoryStatus: ts,
			EntryTime:       entryTimeStr,
			LastExitTime:    lastExitStr,
		})
	}
	return items, nil
}

// UpdateCarTerritoryStatus обновляет территориальный статус автомобиля (въезд/выезд).
func (s *carService) UpdateCarTerritoryStatus(ctx context.Context, carID int, req UpdateTerritoryStatusRequest) error {
	now := time.Now().UTC()
	actionType := "unknown"
	if req.TerritoryStatus == 1 {
		actionType = "entry"
	} else if req.TerritoryStatus == 2 {
		actionType = "exit"
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var car models.Car
		if err := tx.Select("id", "car_number", "car_brand", "territory_status").
			First(&car, carID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Car not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		updates := map[string]interface{}{
			"territory_status": req.TerritoryStatus,
			"updated_at":       now,
		}
		if req.TerritoryStatus == 1 {
			updates["territory_entry_time"] = now
		}
		if err := tx.Model(&models.Car{}).Where("id = ?", carID).Updates(updates).Error; err != nil {
			slog.Error("не удалось обновить территориальный статус автомобиля", "car_id", carID, "status", req.TerritoryStatus, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating car territory status")
		}

		carNumber := ""
		if car.CarNumber != nil {
			carNumber = *car.CarNumber
		}
		var comment string
		if req.TerritoryStatus == 1 {
			comment = fmt.Sprintf("Автомобиль %s въехал на территорию", carNumber)
		} else if req.TerritoryStatus == 2 {
			comment = fmt.Sprintf("Автомобиль %s выехал с территории", carNumber)
		}

		history := models.CarHistory{
			CarID:      carID,
			UserID:     req.UserID,
			ActionType: actionType,
			Comment:    &comment,
			CreatedAt:  now,
		}
		if err := tx.Create(&history).Error; err != nil {
			slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
		}
		slog.Info("территориальный статус автомобиля обновлён", "car_id", carID, "action_type", actionType, "status", req.TerritoryStatus)
		return nil
	})
}

// DeactivateCar деактивирует автомобиль и записывает удаление в историю.
func (s *carService) DeactivateCar(ctx context.Context, carID int, req DeactivateCarRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var car models.Car
		if err := tx.Select("id", "car_number", "car_brand").
			First(&car, carID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Car not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		now := time.Now().UTC()
		today := now.Format("2006-01-02")
		if err := tx.Model(&models.Car{}).Where("id = ?", carID).Updates(map[string]interface{}{
			"status":       req.Status,
			"date_removed": today,
			"updated_at":   now,
		}).Error; err != nil {
			slog.Error("не удалось деактивировать автомобиль", "car_id", carID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error deactivating car")
		}

		carNumber := ""
		carBrand := ""
		if car.CarNumber != nil {
			carNumber = *car.CarNumber
		}
		if car.CarBrand != nil {
			carBrand = *car.CarBrand
		}
		comment := fmt.Sprintf("Автомобиль %s %s удалён пользователем", carNumber, carBrand)
		actionType := "delete"
		history := models.CarHistory{
			CarID:      carID,
			UserID:     req.UserID,
			ActionType: actionType,
			Comment:    &comment,
			CreatedAt:  now,
		}
		if err := tx.Create(&history).Error; err != nil {
			slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
		}
		slog.Info("автомобиль деактивирован", "car_id", carID)
		return nil
	})
}

// ActivateCar вводит автомобиль в работу и записывает активацию в историю.
func (s *carService) ActivateCar(ctx context.Context, carID int, req ActivateCarRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var car models.Car
		if err := tx.Select("id", "car_number", "car_brand").
			First(&car, carID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Car not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		now := time.Now().UTC()
		if err := tx.Model(&models.Car{}).Where("id = ?", carID).Updates(map[string]interface{}{
			"status":       1,
			"date_removed": nil,
			"updated_at":   now,
		}).Error; err != nil {
			slog.Error("не удалось активировать автомобиль", "car_id", carID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error activating car")
		}

		carNumber := ""
		carBrand := ""
		if car.CarNumber != nil {
			carNumber = *car.CarNumber
		}
		if car.CarBrand != nil {
			carBrand = *car.CarBrand
		}
		comment := fmt.Sprintf("Автомобиль %s %s введён в работу", carNumber, carBrand)
		actionType := "activate"
		history := models.CarHistory{
			CarID:      carID,
			UserID:     req.UserID,
			ActionType: actionType,
			Comment:    &comment,
			CreatedAt:  now,
		}
		if err := tx.Create(&history).Error; err != nil {
			slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
		}
		slog.Info("автомобиль активирован", "car_id", carID)
		return nil
	})
}

// RestoreCar восстанавливает удалённый автомобиль и записывает восстановление в историю.
func (s *carService) RestoreCar(ctx context.Context, carID int, req RestoreCarRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var car models.Car
		if err := tx.Select("id", "car_number", "car_brand").
			First(&car, carID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Car not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		now := time.Now().UTC()
		if err := tx.Model(&models.Car{}).Where("id = ?", carID).Updates(map[string]interface{}{
			"status":       1,
			"date_removed": nil,
			"updated_at":   now,
		}).Error; err != nil {
			slog.Error("не удалось восстановить автомобиль", "car_id", carID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error restoring car")
		}

		carNumber := ""
		carBrand := ""
		if car.CarNumber != nil {
			carNumber = *car.CarNumber
		}
		if car.CarBrand != nil {
			carBrand = *car.CarBrand
		}
		comment := fmt.Sprintf("Автомобиль %s %s восстановлен", carNumber, carBrand)
		actionType := "restore"
		history := models.CarHistory{
			CarID:      carID,
			UserID:     req.UserID,
			ActionType: actionType,
			Comment:    &comment,
			CreatedAt:  now,
		}
		if err := tx.Create(&history).Error; err != nil {
			slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
		}
		slog.Info("автомобиль восстановлен", "car_id", carID)
		return nil
	})
}

// GetUnifiedCarHistory возвращает объединённую историю для всех автомобилей с одинаковыми параметрами.
func (s *carService) GetUnifiedCarHistory(ctx context.Context, req UnifiedCarHistoryQuery) ([]CarHistoryItemResponse, error) {
	// Находим все машины с одинаковыми параметрами
	type carIDRow struct {
		ID int
	}
	var carIDs []carIDRow
	err := s.db.WithContext(ctx).Raw(`
		SELECT c.id
		FROM cars c
		JOIN attachments a ON c.attachment_id = a.id
		JOIN applications app ON a.application_id = app.id
		WHERE LOWER(TRIM(c.car_number)) = LOWER(TRIM(?))
		AND LOWER(TRIM(c.car_brand)) = LOWER(TRIM(?))
		AND (
			(?::integer IS NULL AND app.organization_id IS NULL)
			OR app.organization_id = ?
		)
		AND (
			(?::integer IS NULL AND app.company_id IS NULL)
			OR app.company_id = ?
		)
		ORDER BY c.id
	`, req.CarNumber, req.CarBrand,
		req.OrganizationID, req.OrganizationID,
		req.CompanyID, req.CompanyID,
	).Scan(&carIDs).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars")
	}

	if len(carIDs) == 0 {
		return []CarHistoryItemResponse{}, nil
	}

	ids := make([]int, len(carIDs))
	for i, c := range carIDs {
		ids[i] = c.ID
	}

	rows := make([]carHistoryRow, 0)
	err = s.db.WithContext(ctx).Raw(`
		SELECT
			h.id,
			h.car_id,
			h.user_id,
			CONCAT(
				COALESCE(u.last_name, ''),
				CASE WHEN u.first_name IS NOT NULL AND u.first_name != '' THEN ' ' || u.first_name ELSE '' END,
				CASE WHEN u.middle_name IS NOT NULL AND u.middle_name != '' THEN ' ' || u.middle_name ELSE '' END
			) AS user_name,
			u.last_name,
			u.first_name,
			u.middle_name,
			h.action_type,
			h.field_name,
			h.old_value,
			h.new_value,
			h.comment,
			h.created_at,
			h.metadata::text AS metadata,
			c.car_number,
			c.car_brand,
			COALESCE(o.name, '') AS organization,
			COALESCE(c2.name, '') AS company
		FROM cars_history h
		LEFT JOIN users u ON h.user_id = u.id
		JOIN cars c ON h.car_id = c.id
		LEFT JOIN attachments a ON c.attachment_id = a.id
		LEFT JOIN applications app ON a.application_id = app.id
		LEFT JOIN organizations o ON app.organization_id = o.id
		LEFT JOIN companies c2 ON app.company_id = c2.id
		WHERE h.car_id IN ?
		ORDER BY h.created_at DESC
	`, ids).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unified car history")
	}

	return s.mapHistoryRows(rows, true), nil
}

// mapHistoryRows преобразует сырые строки истории в DTO.
func (s *carService) mapHistoryRows(rows []carHistoryRow, includeCarInfo bool) []CarHistoryItemResponse {
	items := make([]CarHistoryItemResponse, 0, len(rows))
	for _, r := range rows {
		userName := r.UserName
		if strings.TrimSpace(userName) == "" {
			userName = "Система"
		}

		mskTime := r.CreatedAt.Add(3 * time.Hour)

		var metadata *json.RawMessage
		if r.Metadata != nil {
			raw := json.RawMessage(*r.Metadata)
			metadata = &raw
		}

		item := CarHistoryItemResponse{
			ID:         r.ID,
			CarID:      r.CarID,
			UserID:     r.UserID,
			UserName:   userName,
			LastName:   r.LastName,
			FirstName:  r.FirstName,
			MiddleName: r.MiddleName,
			ActionType: r.ActionType,
			FieldName:  r.FieldName,
			OldValue:   r.OldValue,
			NewValue:   r.NewValue,
			Comment:    r.Comment,
			CreatedAt:  mskTime.Format("2006-01-02T15:04:05+03:00"),
			Metadata:   metadata,
		}
		if includeCarInfo {
			item.CarNumber = r.CarNumber
			item.CarBrand = r.CarBrand
			item.Organization = r.Organization
			item.Company = r.Company
		}

		items = append(items, item)
	}
	return items
}
