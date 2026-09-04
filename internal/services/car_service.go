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
	// CreateManualCars добавляет машины прямо в таблицу без заявки (#1049, режим-1):
	// создаёт вложение-сироту (application_id NULL, is_manual, org/company на вложении),
	// сами машины со status=1 и привязку к целевым таблицам - одной транзакцией.
	CreateManualCars(ctx context.Context, req ManualCarRequest, userID int) (*ManualCarResponse, error)
	// GetActiveCarsForTable возвращает активные машины конкретной таблицы «Проезд» (#1036).
	GetActiveCarsForTable(ctx context.Context, tableID int) ([]TableCarResponse, error)
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
	// GetCarsHistoryByTable возвращает историю въездов/выездов таблицы проходной.
	GetCarsHistoryByTable(ctx context.Context, tableID int) ([]AllCarsHistoryItem, error)
	// GetCarsCurrentStatus возвращает текущий территориальный статус активных машин.
	GetCarsCurrentStatus(ctx context.Context) ([]CarCurrentStatus, error)
	// UpdateCarTerritoryStatus обновляет статус нахождения на территории (въезд/выезд).
	UpdateCarTerritoryStatus(ctx context.Context, carID int, req UpdateCarTerritoryStatusRequest) error
	// DeactivateCar деактивирует автомобиль (мягкое удаление).
	DeactivateCar(ctx context.Context, carID int, req DeactivateCarRequest) error
	// ActivateCar вводит автомобиль в работу.
	ActivateCar(ctx context.Context, carID int, req ActivateCarRequest) error
	// RestoreCar восстанавливает удалённый автомобиль.
	RestoreCar(ctx context.Context, carID int, req RestoreCarRequest) error
	// GetUnifiedCarHistory возвращает объединённую историю для всех машин с одинаковыми параметрами.
	GetUnifiedCarHistory(ctx context.Context, req UnifiedCarHistoryQuery) ([]CarHistoryItemResponse, error)
	// BulkMoveTable переносит набор машин из одной таблицы «Проезд» в другие (#1194,
	// групповая операция): FromTableID снимается, ToTableIDs добавляются (объединение
	// с уже существующими у машины привязками, кроме FromTableID). Пустой итоговый
	// набор целевых таблиц -> машина деактивируется (как единичный DeactivateCar).
	BulkMoveTable(ctx context.Context, req BulkMoveCarsTableRequest, actorID int) (*BulkOpResult, error)
	// BulkAddTable добавляет набор машин в дополнительные таблицы «Проезд» (#1194):
	// объединение с текущими привязками, существующие не снимаются.
	BulkAddTable(ctx context.Context, req BulkAddCarsTableRequest, actorID int) (*BulkOpResult, error)
	// BulkUnbindTable снимает привязку набора машин к одной таблице «Проезд» (#1194).
	// Пустой итоговый набор целевых таблиц -> машина деактивируется (как единичный
	// DeactivateCar).
	BulkUnbindTable(ctx context.Context, req BulkUnbindCarsTableRequest, actorID int) (*BulkOpResult, error)

	// SetBlankExportEnqueuer подключает очередь файлового архива (#1615, B1).
	SetBlankExportEnqueuer(e BlankExportEnqueuer)
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

// ManualCarRequest -- тело запроса ручного добавления машин в таблицу (#1049, режим-1
// без заявки). org/company и время действия живут на вложении-сироте, само вложение
// получает is_manual=true и application_id NULL. TableID -- таблица, из шапки которой
// нажали «Добавить вручную»: машина гарантированно попадёт в неё (плюс любые таблицы
// «Проезда», выбранные в форме, через Vehicles[].TargetTables).
type ManualCarRequest struct {
	OrganizationID int             `json:"organization_id"`
	CompanyID      *int            `json:"company_id"`
	TableID        int             `json:"table_id"`
	EntryDateFrom  *string         `json:"entry_date_from"`
	EntryDateTo    *string         `json:"entry_date_to"`
	EntryTimeFrom  *string         `json:"entry_time_from"`
	EntryTimeTo    *string         `json:"entry_time_to"`
	RoofAccess     bool            `json:"roof_access"`
	FreeParking    bool            `json:"free_parking"`
	Vehicles       []ManualVehicle `json:"vehicles"`
}

// ManualVehicle -- одна машина в запросе ручного добавления (зеркало полей VehicleForm).
type ManualVehicle struct {
	CarNumber    string  `json:"car_number"`
	CarBrand     string  `json:"car_brand"`
	MarkID       *int    `json:"mark_id"`
	MarkName     *string `json:"mark_name"`
	UnloadPlace  *string `json:"unload_place"`
	UnloadPlaces []int   `json:"unload_places"`
	TargetTables []int   `json:"target_tables"`
}

// ManualCarResponse -- ответ после ручного добавления машин.
type ManualCarResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	AttachmentID int    `json:"attachment_id"`
	CarIDs       []int  `json:"car_ids"`
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

// FactPassData -- данные, введённые охранником при пропуске машины "по факту" (#1132):
// снимок реального номера/марки/формата, снятый на КПП при въезде. Пишется в
// details.metadata записи entry истории машины; cars.car_number/mark НЕ меняет
// (исходный плейсхолдер "по факту" в строке таблицы сохраняется).
type FactPassData struct {
	Number     string  `json:"number"`
	FormatID   *int    `json:"format_id,omitempty"`
	FormatName *string `json:"format_name,omitempty"`
	MarkID     *int    `json:"mark_id,omitempty"`
	MarkName   *string `json:"mark_name,omitempty"`
}

// UpdateCarTerritoryStatusRequest -- тело PUT /cars/:id/territory-status. Встраивает
// общий UpdateTerritoryStatusRequest (territory_status/user_id/table_id) и добавляет
// опциональный Pass -- данные пропуска "по факту" (#1132), которые охранник вводит в
// модалке при въезде. Учитывается только при въезде (territory_status=1); при выезде
// и при отсутствии данных поведение прежнее.
type UpdateCarTerritoryStatusRequest struct {
	UpdateTerritoryStatusRequest
	Pass *FactPassData `json:"pass,omitempty"`
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
	// TargetTablesCount - число таблиц «Проезд», к которым привязана машина (#1194):
	// FE показывает per-row «Убрать» без подменю при 1 (снятие с единственной =
	// деактивация) и с подменю «из этой/из всех» при >1.
	TargetTablesCount int `json:"target_tables_count"`
	// TargetTables - сами привязки «Проезд» с источником (#1227): карточка машины из
	// контекста проходной различает «из заявки» (application) и «добавлено» (manual).
	TargetTables []CarPassageTableRef `json:"target_tables"`
}

// CarPassageTableRef -- привязка машины к таблице «Проезд» с источником добавления
// (#1227). Локальный тип detail-пути проходной - НЕ путать с общим TableInfoRef
// (application_service.go), у которого нет source. Зеркало у сотрудников -
// EmployeePassageTableRef (employee_service.go).
type CarPassageTableRef struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
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
	// blankExports - постановка заявки в очередь на выгрузку в файловый архив
	// (#1615, B1): bulk-перенос машины между таблицами «Проезд» меняет то, что
	// хранит слепок заявки (заявка.json). Сеттер - тот же порядок инициализации,
	// что у applicationService.SetBlankExportEnqueuer.
	blankExports BlankExportEnqueuer
	// notificationService - уведомление инициатора о первом проходе по заявке
	// (#1748, S4). Опционально: без неё UpdateCarTerritoryStatus просто не шлёт.
	notificationService NotificationService
}

// CarServiceOption конфигурирует carService при создании.
type CarServiceOption func(*carService)

// WithCarTablesProducer включает публикацию tables.refresh при въезде/выезде
// машины (#840 V2.3): строка видна во всех cars-таблицах, обновляем их live.
func WithCarTablesProducer(p *TablesRefreshPublisher) CarServiceOption {
	return func(s *carService) { s.tablesProducer = p }
}

// WithCarNotifications включает уведомление инициатора заявки о первом проходе
// по ней (#1748, S4) при въезде машины.
func WithCarNotifications(n NotificationService) CarServiceOption {
	return func(s *carService) { s.notificationService = n }
}

// SetBlankExportEnqueuer подключает очередь файлового архива (#1615, B1).
func (s *carService) SetBlankExportEnqueuer(e BlankExportEnqueuer) {
	s.blankExports = e
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

// CreateManualCars добавляет машины в таблицу без заявки (#1049, режим-1). Создаёт
// вложение-сироту (application_id NULL, is_manual, org/company на вложении), затем сами
// машины со status=1 (одобрения нет - сразу активны), их места разгрузки, привязку к
// целевым таблицам и записи аудита. Всё одной транзакцией: частичного добавления быть
// не должно.
func (s *carService) CreateManualCars(ctx context.Context, req ManualCarRequest, userID int) (*ManualCarResponse, error) {
	if req.OrganizationID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не указана организация")
	}
	if req.TableID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не указана таблица")
	}
	if len(req.Vehicles) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не указаны машины")
	}
	for _, v := range req.Vehicles {
		if strings.TrimSpace(v.CarNumber) == "" {
			return nil, echo.NewHTTPError(http.StatusBadRequest, "У машины не указан номер")
		}
	}

	var attID int
	carIDs := make([]int, 0, len(req.Vehicles))

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		statusOne := 1
		att := models.Attachment{
			ApplicationID:   nil,
			AttachmentType:  "cars",
			EntryDateFrom:   req.EntryDateFrom,
			EntryDateTo:     req.EntryDateTo,
			EntryTimeFrom:   req.EntryTimeFrom,
			EntryTimeTo:     req.EntryTimeTo,
			RoofAccess:      req.RoofAccess,
			FreeParking:     req.FreeParking,
			OrganizationID:  &req.OrganizationID,
			CompanyID:       req.CompanyID,
			IsManual:        true,
			CreatedByUserID: &userID,
			Status:          &statusOne,
		}
		if err := tx.Create(&att).Error; err != nil {
			slog.Error("не удалось создать ручное вложение", "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error creating manual attachment")
		}
		attID = att.ID

		// Дедуп-union мест всех машин вложения для attachment_unload_places (источник
		// видимости мест для охранника, S6). car_unload_places пишем на каждую машину.
		attachPlaces := make(map[int]struct{})

		for _, v := range req.Vehicles {
			carStatus := statusOne
			car := models.Car{
				AttachmentID:  attID,
				CarNumber:     &v.CarNumber,
				CarBrand:      &v.CarBrand,
				MarkID:        v.MarkID,
				MarkName:      v.MarkName,
				UnloadPlace:   v.UnloadPlace,
				EntryDateFrom: req.EntryDateFrom,
				EntryTimeFrom: req.EntryTimeFrom,
				EntryDateTo:   req.EntryDateTo,
				EntryTimeTo:   req.EntryTimeTo,
				Status:        &carStatus,
			}
			if err := tx.Create(&car).Error; err != nil {
				slog.Error("не удалось создать ручную машину", "car_number", v.CarNumber, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating manual car")
			}
			carIDs = append(carIDs, car.ID)

			for _, placeID := range v.UnloadPlaces {
				orderIdx := 1
				cup := models.CarUnloadPlace{CarID: car.ID, UnloadPlaceID: placeID, OrderIndex: &orderIdx}
				if err := tx.Create(&cup).Error; err != nil {
					slog.Error("не удалось создать связь машины с местом разгрузки", "car_id", car.ID, "unload_place_id", placeID, "error", err)
					return echo.NewHTTPError(http.StatusInternalServerError, "Error creating car unload place")
				}
				attachPlaces[placeID] = struct{}{}
			}

			// Целевые таблицы: таблица со страницы (req.TableID, гарантирует показ там,
			// откуда добавили) объединяется с выбранными в форме «Проездом».
			targetTables := map[int]struct{}{req.TableID: {}}
			for _, tableID := range v.TargetTables {
				if tableID > 0 {
					targetTables[tableID] = struct{}{}
				}
			}
			for tableID := range targetTables {
				ctt := models.CarTargetTable{CarID: car.ID, TableID: tableID, Source: "manual"}
				if err := tx.Create(&ctt).Error; err != nil {
					slog.Error("не удалось привязать машину к таблице", "car_id", car.ID, "table_id", tableID, "error", err)
					return echo.NewHTTPError(http.StatusInternalServerError, "Error linking car to table")
				}
				// История «добавлен в таблицу проходной» (#1085), в той же tx (как соседний create-Record).
				if err := recordAddedToTable(ctx, s.recorder, tx, models.AuditEntityCar, car.ID, tableID, &userID); err != nil {
					slog.Error("не удалось записать историю попадания машины в таблицу", "car_id", car.ID, "table_id", tableID, "error", err)
					return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car table history entry")
				}
			}

			comment := fmt.Sprintf("Автомобиль %s %s добавлен вручную", v.CarNumber, v.CarBrand)
			if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &car.ID, "create", &userID, carAuditDetails{Comment: &comment}); err != nil {
				slog.Error("не удалось записать историю ручной машины", "car_id", car.ID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
			}
		}

		for placeID := range attachPlaces {
			if err := tx.Exec("INSERT INTO attachment_unload_places (attachment_id, unload_place_id) VALUES (?, ?) ON CONFLICT DO NOTHING", attID, placeID).Error; err != nil {
				slog.Error("не удалось записать место вложения", "attachment_id", attID, "unload_place_id", placeID, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating attachment unload place")
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Ручные машины появились в целевых таблицах live - обновляем их аудиторию (#1049,
	// по target-таблице, т.к. заявки нет). Best-effort, вне транзакции.
	s.tablesProducer.NotifyCarsChangedBatch(ctx, carIDs)

	slog.Info("ручные машины добавлены", "attachment_id", attID, "count", len(carIDs), "user_id", userID)
	return &ManualCarResponse{
		Success:      true,
		Message:      "Machines added successfully",
		AttachmentID: attID,
		CarIDs:       carIDs,
	}, nil
}

// CheckActiveCar проверяет наличие активного автомобиля по номеру, марке и организации.
//
// "Сейчас" берётся в московской зоне: entry_date_to и entry_time_to -- это дата и
// час из заявки, записанные по московским часам бюро. Сравнение с UTC давало машине
// лишние три часа действия пропуска, а между 21:00 и 24:00 МСК UTC-дата отставала
// на сутки, и вчерашняя заявка считалась активной (та же природа, что у #868).
func (s *carService) CheckActiveCar(ctx context.Context, req CheckActiveCarRequest) (*CheckActiveCarResponse, error) {
	now := time.Now().In(moscowWorkModeLoc)
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
