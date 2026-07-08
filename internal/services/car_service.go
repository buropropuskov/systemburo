package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
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
	// GetActiveCarsForTable возвращает активные машины конкретной таблицы «Проезд» (#1036).
	GetActiveCarsForTable(ctx context.Context, tableID int) ([]TableCarResponse, error)
	// GetFactCarsForTables возвращает машины с номером «по факту».
	GetFactCarsForTables(ctx context.Context) ([]TableCarResponse, error)
	// GetFactCarsForTable возвращает машины «по факту» конкретной таблицы «Проезд» (#1036).
	GetFactCarsForTable(ctx context.Context, tableID int) ([]TableCarResponse, error)
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
	// TableID -- таблица (КПП), из которой отмечен въезд/выезд; пишется в историю,
	// чтобы в карточке истории было видно, где произошло событие.
	TableID *int `json:"table_id"`
}

// DeactivateCarRequest -- тело запроса деактивации автомобиля.
type DeactivateCarRequest struct {
	Status  int  `json:"status"`
	UserID  *int `json:"user_id"`
	TableID *int `json:"table_id"`
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
	ID                 int      `json:"id"`
	CarNumber          string   `json:"car_number"`
	CarBrand           string   `json:"car_brand"`
	Organization       *string  `json:"organization"`
	OrganizationID     *int     `json:"organization_id"`
	Company            *string  `json:"company"`
	CompanyID          *int     `json:"company_id"`
	UnloadPlace        *string  `json:"unload_place"`
	UnloadPlaces       []string `json:"unload_places"`
	EntryDateTo        *string  `json:"entry_date_to"`
	EntryTimeFrom      *string  `json:"entry_time_from"`
	EntryTimeTo        *string  `json:"entry_time_to"`
	Status             int      `json:"status"`
	ApplicationID      *int     `json:"application_id"`
	ApplicationNumber  *string  `json:"application_number"`
	TerritoryStatus    *int     `json:"territory_status"`
	TerritoryEntryTime *string  `json:"territory_entry_time"`
}

// CarUnloadPlaceInfo -- связь автомобиля с местом разгрузки.
type CarUnloadPlaceInfo struct {
	CarID           int    `json:"car_id"`
	UnloadPlaceID   int    `json:"unload_place_id"`
	UnloadPlaceName string `json:"unload_place_name"`
}

// CarHistoryItemResponse -- элемент истории автомобиля.
type CarHistoryItemResponse struct {
	ID            int              `json:"id"`
	CarID         int              `json:"car_id"`
	ApplicationID *int             `json:"application_id"`
	UserID        *int             `json:"user_id"`
	UserName      string           `json:"user_name"`
	LastName      *string          `json:"last_name"`
	FirstName     *string          `json:"first_name"`
	MiddleName    *string          `json:"middle_name"`
	ActionType    string           `json:"action_type"`
	FieldName     *string          `json:"field_name"`
	OldValue      *string          `json:"old_value"`
	NewValue      *string          `json:"new_value"`
	Comment       *string          `json:"comment"`
	CreatedAt     string           `json:"created_at"`
	Metadata      *json.RawMessage `json:"metadata" swaggertype:"object"`
	CarNumber     *string          `json:"car_number"`
	CarBrand      *string          `json:"car_brand"`
	Organization  *string          `json:"organization"`
	Company       *string          `json:"company"`
	TableID       *int             `json:"table_id"`
	TableName     *string          `json:"table_name"`
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
	TableID      *int    `json:"table_id"`
	TableName    *string `json:"table_name"`
}

// CarCurrentStatus -- текущий территориальный статус автомобиля.
type CarCurrentStatus struct {
	CarID           int     `json:"car_id"`
	TerritoryStatus int     `json:"territory_status"`
	EntryTime       *string `json:"entry_time"`
	LastExitTime    *string `json:"last_exit_time"`
}

// --- Реализация ---

type carService struct {
	db             *gorm.DB
	recorder       AuditRecorder
	tablesProducer *TablesRefreshPublisher
}

// CarServiceOption конфигурирует carService при создании.
type CarServiceOption func(*carService)

// WithCarTablesProducer включает публикацию tables.refresh при въезде/выезде
// машины (#840 V2.3): строка видна во всех cars-таблицах, обновляем их live.
func WithCarTablesProducer(p *TablesRefreshPublisher) CarServiceOption {
	return func(s *carService) { s.tablesProducer = p }
}

// NewCarService создаёт новый экземпляр CarService.
func NewCarService(db *gorm.DB, recorder AuditRecorder, opts ...CarServiceOption) CarService {
	s := &carService{db: db, recorder: recorder}
	for _, opt := range opts {
		opt(s)
	}
	return s
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
		if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &carID, "create", &userID, carAuditDetails{Comment: &comment}); err != nil {
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
