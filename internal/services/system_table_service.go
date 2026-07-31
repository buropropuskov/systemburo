package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/realtime"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SystemTableService -- интерфейс бизнес-логики системных таблиц.
type SystemTableService interface {
	GetAll(ctx context.Context, includeArchived bool) ([]models.SystemTableWithDetails, error)
	GetByID(ctx context.Context, id int) (*models.SystemTableWithDetails, error)
	GetByName(ctx context.Context, name string, allowArchived bool) (*models.SystemTableWithDetails, error)
	Create(ctx context.Context, req models.CreateSystemTableRequest) (int, error)
	Update(ctx context.Context, id int, req models.UpdateSystemTableRequest) error
	Delete(ctx context.Context, id int) error
	Restore(ctx context.Context, id int) error
	// Групповая архивация/восстановление (по образцу марок/мест разгрузки).
	BulkArchive(ctx context.Context, ids []int) (*BulkOpResult, error)
	BulkRestore(ctx context.Context, ids []int) (*BulkOpResult, error)

	// GetUsage возвращает организации и компании, привязанные к таблице (те же,
	// что блокируют Delete). DetachAll снимает все эти привязки разом,
	// DetachOrganization/DetachCompany - по одной. Все возвращают detached=false
	// без ошибки, если привязки уже нет (идемпотентно).
	GetUsage(ctx context.Context, id int) (*SystemTableUsage, error)
	DetachAll(ctx context.Context, callerUserID, id int) (*SystemTableDetachResult, error)
	DetachOrganization(ctx context.Context, callerUserID, id, organizationID int) (bool, error)
	DetachCompany(ctx context.Context, callerUserID, id, companyID int) (bool, error)

	// Временные слоты
	GetTimeSlots(ctx context.Context, tableID int) ([]models.SystemTableTimeSlot, error)
	AddTimeSlot(ctx context.Context, tableID int, req models.CreateTimeSlotRequest) (int, error)
	UpdateTimeSlot(ctx context.Context, tableID, slotID int, req models.UpdateTimeSlotRequest) error
	DeleteTimeSlot(ctx context.Context, tableID, slotID int) error

	// Предупреждения по временным окнам (#1183)
	GetWarningWindows(ctx context.Context, tableID int) ([]models.SystemTableWarningWindow, error)
	AddWarningWindow(ctx context.Context, tableID int, req models.WarningWindowRequest) (int, error)
	UpdateWarningWindow(ctx context.Context, tableID, windowID int, req models.WarningWindowRequest) error
	DeleteWarningWindow(ctx context.Context, tableID, windowID int) error

	// Фотографии
	UploadPhoto(ctx context.Context, tableID int, username string, photoURL, fileName, mimeType string, fileSize int64) (int, error)
	DeletePhoto(ctx context.Context, tableID, photoID int) error
	SetMainPhoto(ctx context.Context, tableID, photoID int) error

	// Столбцы таблицы (#345): bulk-обновление видимости.
	UpdateFields(ctx context.Context, tableID int, req models.UpdateFieldsRequest) error
	// Столбцы фактовой таблицы (#345).
	UpdateFactFields(ctx context.Context, tableID int, req models.UpdateFieldsRequest) error

	// SeedMissingFields добавляет default-поля, которых ещё нет у существующих таблиц.
	// Вызывается один раз при старте сервиса для миграции старых таблиц.
	SeedMissingFields(ctx context.Context) error

	// GetHistory возвращает историю изменений системной таблицы (новые сверху).
	// Переходный период #870: чтение объединяет legacy system_table_histories и audit_log.
	GetHistory(ctx context.Context, tableID int) ([]models.SystemTableHistoryItem, error)
}

type systemTableService struct {
	db                *gorm.DB
	uploadDir         string
	maxFileSize       int64
	permSvc           PermissionService
	realtimePublisher realtime.Publisher
	recorder          AuditRecorder
}

// SystemTableBinding -- привязанная к таблице организация/компания.
// IsActive=false помечает архивную запись (её всё равно показываем: гейт Delete
// считает по junction без фильтра активности, поэтому она держит таблицу).
type SystemTableBinding struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}

// SystemTableUsage -- организации и компании, привязанные к таблице.
// Набор совпадает с тем, что блокирует удаление (см. Delete).
type SystemTableUsage struct {
	Organizations []SystemTableBinding `json:"organizations"`
	Companies     []SystemTableBinding `json:"companies"`
}

// SystemTableDetachResult -- сколько привязок снято операцией «Отвязать всё».
type SystemTableDetachResult struct {
	OrganizationsDetached int `json:"organizations_detached"`
	CompaniesDetached     int `json:"companies_detached"`
}

// SystemTableServiceOption конфигурирует systemTableService при создании.
type SystemTableServiceOption func(*systemTableService)

// WithSystemTableRealtimePublisher включает real-time сигнал system-tables.refresh
// при изменении набора таблиц (#840): список таблиц в нав-меню у всех обновляется
// мгновенно, не дожидаясь 60с-опроса. Опционально.
func WithSystemTableRealtimePublisher(p realtime.Publisher) SystemTableServiceOption {
	return func(s *systemTableService) { s.realtimePublisher = p }
}

// NewSystemTableService создаёт реализацию SystemTableService.
func NewSystemTableService(db *gorm.DB, uploadDir string, maxFileSize int64, permSvc PermissionService, opts ...SystemTableServiceOption) SystemTableService {
	s := &systemTableService{
		db:          db,
		uploadDir:   uploadDir,
		maxFileSize: maxFileSize,
		permSvc:     permSvc,
		recorder:    NewAuditRecorder(db),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// notifyTablesChanged шлёт system-tables.refresh всем активным юзерам (список
// таблиц виден в нав-меню каждому, клиент сам фильтрует по правам). Best-effort,
// nil-safe.
func (s *systemTableService) notifyTablesChanged(ctx context.Context) {
	if s.realtimePublisher == nil {
		return
	}
	ids, err := activeUserIDs(ctx, s.db)
	if err != nil {
		slog.Warn("system-tables.refresh: load active users failed", "err", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	s.realtimePublisher.PublishMany(ids, realtime.Event{Type: "system-tables.refresh", Scope: "system-tables"})
}

// computeCurrentStatus вычисляет текущий статус (open/closed) на основании расписания и статуса таблицы.
func computeCurrentStatus(tableStatus string, slots []models.SystemTableTimeSlot) string {
	return computeCurrentStatusAt(time.Now(), tableStatus, slots)
}

// computeCurrentStatusAt - чистое ядро статуса с инъекцией now (для теста).
// День недели и время берутся в МСК (как bureau computeWorkModeStatus): сервер в
// UTC, слоты заданы в московском дне - без конверсии у границы суток (21:00-24:00
// UTC) currentDay уходит на сутки назад и статус системной таблицы врёт.
func computeCurrentStatusAt(now time.Time, tableStatus string, slots []models.SystemTableTimeSlot) string {
	if tableStatus != "active" {
		return "closed"
	}

	now = now.In(moscowWorkModeLoc)
	// 0=Пн, 6=Вс (Go Weekday: 0=Вс, 1=Пн ... 6=Сб)
	goDay := int(now.Weekday())
	currentDay := (goDay + 6) % 7
	currentTime := now.Format("15:04")

	// Круглосуточный слот
	for _, s := range slots {
		if s.DayOfWeek == currentDay && s.IsActive &&
			s.OpenTime == "00:00" && s.CloseTime == "23:59" && !s.IsNextDay {
			return "open"
		}
	}

	for _, s := range slots {
		if s.DayOfWeek != currentDay || !s.IsActive {
			continue
		}
		if s.IsNextDay {
			if currentTime >= s.OpenTime {
				return "open"
			}
		} else {
			if currentTime >= s.OpenTime && currentTime <= s.CloseTime {
				return "open"
			}
		}
	}

	return "closed"
}

// loadTableWithPreload загружает таблицу по условию с Preload связей (1 запрос + 3 Preload вместо 4 отдельных запросов).
func (s *systemTableService) loadTableWithPreload(_ context.Context, query *gorm.DB) (*models.SystemTableWithDetails, error) {
	var table models.SystemTable
	err := query.
		Preload("Fields", func(db *gorm.DB) *gorm.DB {
			return db.Order("display_order")
		}).
		Preload("FactFields", func(db *gorm.DB) *gorm.DB {
			return db.Order("display_order")
		}).
		Preload("TimeSlots", func(db *gorm.DB) *gorm.DB {
			return db.Order("day_of_week, open_time")
		}).
		Preload("WarningWindows", func(db *gorm.DB) *gorm.DB {
			return db.Order("day_of_week NULLS FIRST, time_from NULLS FIRST")
		}).
		Preload("Photos", func(db *gorm.DB) *gorm.DB {
			return db.Order("is_main DESC, uploaded_at DESC")
		}).
		First(&table).Error
	if err != nil {
		return nil, err
	}

	if table.Fields == nil {
		table.Fields = []models.TableField{}
	}
	if table.FactFields == nil {
		table.FactFields = []models.TableFieldFact{}
	}
	if table.TimeSlots == nil {
		table.TimeSlots = []models.SystemTableTimeSlot{}
	}
	if table.WarningWindows == nil {
		table.WarningWindows = []models.SystemTableWarningWindow{}
	}
	if table.Photos == nil {
		table.Photos = []models.SystemTablePhoto{}
	}

	status := "active"
	if table.Status != "" {
		status = table.Status
	}

	return &models.SystemTableWithDetails{
		Table:          table,
		Fields:         table.Fields,
		FactFields:     table.FactFields,
		TimeSlots:      table.TimeSlots,
		WarningWindows: table.WarningWindows,
		Photos:         table.Photos,
		CurrentStatus:  computeCurrentStatus(status, table.TimeSlots),
	}, nil
}

// GetAll возвращает системные таблицы с полями, слотами и фотографиями.
// includeArchived=true возвращает только архивные (is_active=false), false - только активные.
func (s *systemTableService) GetAll(ctx context.Context, includeArchived bool) ([]models.SystemTableWithDetails, error) {
	tables := make([]models.SystemTable, 0)
	if err := s.db.WithContext(ctx).
		Preload("Fields", func(db *gorm.DB) *gorm.DB {
			return db.Order("display_order")
		}).
		Preload("FactFields", func(db *gorm.DB) *gorm.DB {
			return db.Order("display_order")
		}).
		Preload("TimeSlots", func(db *gorm.DB) *gorm.DB {
			return db.Order("day_of_week, open_time")
		}).
		Preload("WarningWindows", func(db *gorm.DB) *gorm.DB {
			return db.Order("day_of_week NULLS FIRST, time_from NULLS FIRST")
		}).
		Preload("Photos", func(db *gorm.DB) *gorm.DB {
			return db.Order("is_main DESC, uploaded_at DESC")
		}).
		Where("is_active = ?", !includeArchived).
		Order("display_name").
		Find(&tables).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system tables")
	}

	if len(tables) == 0 {
		return []models.SystemTableWithDetails{}, nil
	}

	result := make([]models.SystemTableWithDetails, 0, len(tables))
	for _, t := range tables {
		fields := t.Fields
		if fields == nil {
			fields = []models.TableField{}
		}
		factFields := t.FactFields
		if factFields == nil {
			factFields = []models.TableFieldFact{}
		}
		slots := t.TimeSlots
		if slots == nil {
			slots = []models.SystemTableTimeSlot{}
		}
		windows := t.WarningWindows
		if windows == nil {
			windows = []models.SystemTableWarningWindow{}
		}
		photos := t.Photos
		if photos == nil {
			photos = []models.SystemTablePhoto{}
		}

		status := "active"
		if t.Status != "" {
			status = t.Status
		}

		result = append(result, models.SystemTableWithDetails{
			Table:          t,
			Fields:         fields,
			FactFields:     factFields,
			TimeSlots:      slots,
			WarningWindows: windows,
			Photos:         photos,
			CurrentStatus:  computeCurrentStatus(status, slots),
		})
	}

	return result, nil
}

// GetByID возвращает системную таблицу по ID с деталями.
func (s *systemTableService) GetByID(ctx context.Context, id int) (*models.SystemTableWithDetails, error) {
	query := s.db.WithContext(ctx).Where("id = ? AND is_active = ?", id, true)
	result, err := s.loadTableWithPreload(ctx, query)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system table")
	}
	return result, nil
}

// GetByName возвращает системную таблицу по имени с деталями. При allowArchived
// фильтр is_active снимается: в выборку попадают и архивные (is_active=false)
// таблицы - нужно для страницы версий архивной таблицы (кнопка "Версии" из архива).
// Параметр АДДИТИВНЫЙ (активные + архивные), в отличие от include_archived у GetAll,
// который переключает выборку на ТОЛЬКО архивные - потому и имя другое.
// Create проверяет уникальность имени по всем строкам без учёта is_active, так что
// активная и архивная с одним именем штатно не сосуществуют; Order("is_active ASC")
// - защита от гонки создания/легаси-данных, чтобы при таком дубле выбралась архивная.
func (s *systemTableService) GetByName(ctx context.Context, name string, allowArchived bool) (*models.SystemTableWithDetails, error) {
	query := s.db.WithContext(ctx).Where("name = ?", name)
	if allowArchived {
		query = query.Order("is_active ASC")
	} else {
		query = query.Where("is_active = ?", true)
	}
	result, err := s.loadTableWithPreload(ctx, query)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Таблица не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system table")
	}
	return result, nil
}

// defaultField -- описание поля по умолчанию для нового типа таблицы.
// IsVisible: включён ли столбец сразу при создании таблицы.
// Width: относительный вес ширины (flex-grow) - браузер делит ширину пропорционально.
// Priority: приоритет столбца в портретном режиме (1-5). 1 = всегда виден,
// 2 = на компактных экранах, 3-5 = скрывается в портрете.
type defaultField struct {
	Name      string
	FieldType string
	IsVisible bool
	Width     int
	Priority  int
}

// getDefaultFields возвращает набор полей по умолчанию для типа таблицы.
// Базовые - видимы сразу. Расширенные - скрыты, включаются админом в "Колонки".
// Width - относительный вес flex-grow, подобранный под типичный контент столбца.
// Priority - приоритет для портретного режима (см. defaultField).
func getDefaultFields(tableType string) []defaultField {
	switch tableType {
	case "cars":
		return []defaultField{
			{"car_number", "text", true, 10, 1},
			{"car_brand", "text", true, 9, 3},
			{"organization", "text", true, 18, 3},
			{"unload_place", "text", true, 15, 3},
			{"valid_until", "date", true, 12, 2},
			{"time_range", "text", true, 10, 3},
			{"status", "text", true, 7, 2},
			// Расширенные (по умолчанию скрытые)
			{"company", "text", false, 12, 4},
			{"application_id", "text", false, 12, 4},
		}
	case "people":
		return []defaultField{
			{"organization", "text", true, 16, 3},
			{"last_name", "text", true, 14, 1},
			{"first_name", "text", true, 9, 2},
			{"middle_name", "text", true, 11, 3},
			{"valid_until", "date", true, 11, 2},
			{"pass_time", "text", true, 13, 3},
			// Расширенные (по умолчанию скрытые)
			{"position", "text", false, 11, 4},
			{"citizenship_name", "text", false, 10, 4},
			{"company", "text", false, 11, 4},
			{"application_id", "text", false, 12, 4},
		}
	default:
		return nil
	}
}

// getDefaultFactFields - дефолты для FactTable. Отражают то, что сейчас
// рендерится FactTable.vue: для cars - organization+car_brand+valid_until+time_range
// (company скрыт, status/unload_place закомментированы); для people -
// organization+valid_until+pass_time. Остальные поля каталога - скрыты,
// чтобы админ мог их включить, если нужно.
func getDefaultFactFields(tableType string) []defaultField {
	switch tableType {
	case "cars":
		return []defaultField{
			{"organization", "text", true, 18, 3},
			{"car_brand", "text", true, 10, 3},
			{"valid_until", "date", true, 12, 2},
			{"time_range", "text", true, 10, 3},
			// Скрыты (как сейчас в хардкоде)
			{"car_number", "text", false, 10, 1},
			{"unload_place", "text", false, 15, 3},
			{"status", "text", false, 7, 2},
			{"company", "text", false, 12, 4},
			{"application_id", "text", false, 12, 4},
		}
	case "people":
		return []defaultField{
			{"organization", "text", true, 18, 3},
			{"valid_until", "date", true, 12, 2},
			{"pass_time", "text", true, 12, 3},
			// Скрыты
			{"last_name", "text", false, 14, 1},
			{"first_name", "text", false, 9, 2},
			{"middle_name", "text", false, 11, 3},
			{"position", "text", false, 11, 4},
			{"citizenship_name", "text", false, 10, 4},
			{"company", "text", false, 11, 4},
			{"application_id", "text", false, 12, 4},
		}
	default:
		return nil
	}
}

// Create создаёт системную таблицу с полями по умолчанию и автогенерацией разрешений.
func (s *systemTableService) Create(ctx context.Context, req models.CreateSystemTableRequest) (int, error) {
	// Проверяем уникальность имени
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Where("name = ?", req.Name).
		Count(&count).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error checking table existence")
	}
	if count > 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Таблица с таким именем уже существует")
	}

	status := "active"
	if req.Status != nil {
		status = *req.Status
	}

	showFactTable := false
	if req.ShowFactTable != nil {
		showFactTable = *req.ShowFactTable
	}

	table := models.SystemTable{
		Name:                req.Name,
		DisplayName:         &req.DisplayName,
		TableType:           req.TableType,
		ShowFactTable:       showFactTable,
		FactTableHint:       req.FactTableHint,
		Instruction:         req.Instruction,
		MapLink:             req.MapLink,
		Status:              status,
		StatusComment:       req.StatusComment,
		LocationDescription: req.LocationDescription,
		Warning:             req.Warning,
		IsActive:            true,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&table).Error; err != nil {
			slog.Error("не удалось создать системную таблицу", "name", req.Name, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error creating system table")
		}

		fields := getDefaultFields(req.TableType)
		for i, f := range fields {
			order := i
			fieldType := f.FieldType
			priority := f.Priority
			if priority == 0 {
				priority = 3
			}
			tf := models.TableField{
				TableID:      table.ID,
				FieldName:    f.Name,
				FieldType:    &fieldType,
				DisplayOrder: &order,
				IsVisible:    f.IsVisible,
				Width:        f.Width,
				Priority:     priority,
			}
			if err := tx.Create(&tf).Error; err != nil {
				slog.Error("не удалось создать поле таблицы", "table_id", table.ID, "field", f.Name, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating table fields")
			}
		}
		factFields := getDefaultFactFields(req.TableType)
		for i, f := range factFields {
			order := i
			fieldType := f.FieldType
			priority := f.Priority
			if priority == 0 {
				priority = 3
			}
			tff := models.TableFieldFact{
				TableID:      table.ID,
				FieldName:    f.Name,
				FieldType:    &fieldType,
				DisplayOrder: &order,
				IsVisible:    f.IsVisible,
				Width:        f.Width,
				Priority:     priority,
			}
			if err := tx.Create(&tff).Error; err != nil {
				slog.Error("не удалось создать факт-поле таблицы", "table_id", table.ID, "field", f.Name, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating fact fields")
			}
			// GORM v2 при Create игнорирует zero-value bool, БД применяет
			// default:true вместо явного false. Дофикс через Update.
			if !f.IsVisible {
				if err := tx.Model(&models.TableFieldFact{}).
					Where("id = ?", tff.ID).
					Update("is_visible", false).Error; err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "Error setting fact field visibility")
				}
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Auto-generate permissions for the new table
	if s.permSvc != nil {
		displayName := req.DisplayName
		if displayName == "" {
			displayName = req.Name
		}
		if err := s.permSvc.AutoGenerateForTable(ctx, table.ID, req.Name); err != nil {
			slog.Error("не удалось автогенерировать разрешения для таблицы", "table_id", table.ID, "error", err)
		}
	}

	slog.Info("системная таблица создана", "id", table.ID, "name", req.Name)
	s.notifyTablesChanged(ctx)
	return table.ID, nil
}

// Update обновляет системную таблицу по ID.
func (s *systemTableService) Update(ctx context.Context, id int, req models.UpdateSystemTableRequest) error {
	// Проверяем существование
	var table models.SystemTable
	if err := s.db.WithContext(ctx).
		Where("id = ?", id).
		First(&table).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system table")
	}

	updates := map[string]interface{}{
		"updated_at": time.Now().UTC(),
	}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.TableType != nil {
		updates["table_type"] = *req.TableType
	}
	if req.ShowFactTable != nil {
		updates["show_fact_table"] = *req.ShowFactTable
	}
	if req.FactTableHint != nil {
		updates["fact_table_hint"] = *req.FactTableHint
	}
	if req.Instruction != nil {
		updates["instruction"] = *req.Instruction
	}
	if req.MapLink != nil {
		updates["map_link"] = *req.MapLink
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.StatusComment != nil {
		updates["status_comment"] = *req.StatusComment
	}
	if req.LocationDescription != nil {
		updates["location_description"] = *req.LocationDescription
	}
	if req.Warning != nil {
		updates["warning"] = *req.Warning
	}
	if req.FontSize != nil {
		if *req.FontSize < 10 || *req.FontSize > 24 {
			return echo.NewHTTPError(http.StatusBadRequest, "font_size должен быть от 10 до 24")
		}
		updates["font_size"] = *req.FontSize
	}
	if req.RowDensity != nil {
		switch *req.RowDensity {
		case "compact", "normal", "spacious":
			updates["row_density"] = *req.RowDensity
		default:
			return echo.NewHTTPError(http.StatusBadRequest, "row_density должен быть compact|normal|spacious")
		}
	}
	if req.FontSizeFact != nil {
		if *req.FontSizeFact < 10 || *req.FontSizeFact > 24 {
			return echo.NewHTTPError(http.StatusBadRequest, "font_size_fact должен быть от 10 до 24")
		}
		updates["font_size_fact"] = *req.FontSizeFact
	}
	if req.RowDensityFact != nil {
		switch *req.RowDensityFact {
		case "compact", "normal", "spacious":
			updates["row_density_fact"] = *req.RowDensityFact
		default:
			return echo.NewHTTPError(http.StatusBadRequest, "row_density_fact должен быть compact|normal|spacious")
		}
	}

	if len(updates) == 1 {
		// Только updated_at, нечего менять
		return nil
	}

	result := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		slog.Error("не удалось обновить системную таблицу", "id", id, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating system table")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}

	// Если только что включили show_fact_table - сидим факт-поля. Идемпотентно:
	// уже существующие поля не дублируются.
	if req.ShowFactTable != nil && *req.ShowFactTable && !table.ShowFactTable {
		table.ShowFactTable = true
		if err := s.seedFactFields(ctx, []models.SystemTable{table}); err != nil {
			slog.Error("не удалось засидить факт-поля при включении флага",
				"table_id", id, "error", err)
		}
	}

	slog.Info("системная таблица обновлена", "id", id)
	s.notifyTablesChanged(ctx)
	return nil
}

// Delete выполняет мягкое удаление системной таблицы с проверкой зависимостей.
func (s *systemTableService) Delete(ctx context.Context, id int) error {
	// Проверяем привязки к организациям
	var orgCount int64
	if err := s.db.WithContext(ctx).Model(&models.OrganizationTable{}).
		Where("table_id = ?", id).
		Count(&orgCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking organization dependencies")
	}

	// Проверяем привязки к компаниям
	var companyCount int64
	if err := s.db.WithContext(ctx).Model(&models.CompaniesTable{}).
		Where("table_id = ?", id).
		Count(&companyCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking company dependencies")
	}

	if orgCount > 0 || companyCount > 0 {
		msg := "Невозможно удалить таблицу, так как она привязана к: "
		parts := []string{}
		if orgCount > 0 {
			parts = append(parts, fmt.Sprintf("организациям (%d)", orgCount))
		}
		if companyCount > 0 {
			parts = append(parts, fmt.Sprintf("компаниям (%d)", companyCount))
		}
		for i, p := range parts {
			if i > 0 {
				msg += " и "
			}
			msg += p
		}
		return echo.NewHTTPError(http.StatusBadRequest, msg)
	}

	// Мягкое удаление
	result := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Where("id = ?", id).
		Update("is_active", false)
	if result.Error != nil {
		slog.Error("не удалось удалить системную таблицу", "id", id, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting system table")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}

	slog.Info("системная таблица удалена (мягко)", "id", id)
	s.notifyTablesChanged(ctx)
	return nil
}

// loadTableNameForBinding возвращает человекочитаемое имя таблицы (display_name
// с фолбэком на name - то же, что пишет в аудит набора UpdateOrganizationTables)
// и признак существования. Активность таблицы не проверяется: гейт Delete тоже
// её не смотрит, привязки держат таблицу независимо от архивности.
func (s *systemTableService) loadTableNameForBinding(ctx context.Context, id int) (string, bool) {
	var t models.SystemTable
	if err := s.db.WithContext(ctx).Select("display_name", "name").Where("id = ?", id).First(&t).Error; err != nil {
		return "", false
	}
	if t.DisplayName != nil && *t.DisplayName != "" {
		return *t.DisplayName, true
	}
	return t.Name, true
}

// GetUsage возвращает организации и компании, привязанные к таблице. Junction
// читается БЕЗ фильтра is_active орг/компании: набор обязан совпадать с тем, что
// считает гейт в Delete, иначе получилось бы «привязок нет», а удалить нельзя.
// Архивные орг/компании помечаются is_active=false.
func (s *systemTableService) GetUsage(ctx context.Context, id int) (*SystemTableUsage, error) {
	if _, ok := s.loadTableNameForBinding(ctx, id); !ok {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}

	usage := &SystemTableUsage{
		Organizations: make([]SystemTableBinding, 0),
		Companies:     make([]SystemTableBinding, 0),
	}
	if err := s.db.WithContext(ctx).
		Table("organization_tables ot").
		Select("o.id, o.name, o.is_active").
		Joins("JOIN organizations o ON o.id = ot.organization_id").
		Where("ot.table_id = ?", id).
		Order("o.name").
		Scan(&usage.Organizations).Error; err != nil {
		slog.Error("не удалось прочитать привязки организаций таблицы", "id", id, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching organization bindings")
	}
	if err := s.db.WithContext(ctx).
		Table("companies_tables ct").
		Select("c.id, c.name, c.is_active").
		Joins("JOIN companies c ON c.id = ct.company_id").
		Where("ct.table_id = ?", id).
		Order("c.name").
		Scan(&usage.Companies).Error; err != nil {
		slog.Error("не удалось прочитать привязки компаний таблицы", "id", id, "error", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching company bindings")
	}
	return usage, nil
}

// DetachAll снимает привязки таблицы ко ВСЕМ организациям и компаниям (обе
// join-таблицы удаляются в одной транзакции). На каждую затронутую
// организацию/компанию пишется история «таблица убрана из набора» - зеркало
// аудита UpdateOrganizationTables/UpdateTables. После этого таблицу можно
// архивировать (Delete больше не заблокирует). Идемпотентно: повтор по уже
// отвязанной таблице возвращает нулевые счётчики.
func (s *systemTableService) DetachAll(ctx context.Context, callerUserID, id int) (*SystemTableDetachResult, error) {
	name, ok := s.loadTableNameForBinding(ctx, id)
	if !ok {
		return nil, echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}

	// DELETE ... RETURNING внутри одной транзакции: удаляем привязки и получаем
	// id ровно затронутых сущностей атомарно. Отдельный SELECT-перед-DELETE дал бы
	// гонку - конкурентная привязка попала бы под DELETE, но мимо аудита.
	var orgIDs, companyIDs []int
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var removedOrgs []models.OrganizationTable
		if err := tx.Clauses(clause.Returning{Columns: []clause.Column{{Name: "organization_id"}}}).
			Where("table_id = ?", id).Delete(&removedOrgs).Error; err != nil {
			slog.Error("не удалось отвязать организации от таблицы", "id", id, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error detaching organizations")
		}
		var removedCompanies []models.CompaniesTable
		if err := tx.Clauses(clause.Returning{Columns: []clause.Column{{Name: "company_id"}}}).
			Where("table_id = ?", id).Delete(&removedCompanies).Error; err != nil {
			slog.Error("не удалось отвязать компании от таблицы", "id", id, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error detaching companies")
		}
		for _, r := range removedOrgs {
			orgIDs = append(orgIDs, r.OrganizationID)
		}
		for _, r := range removedCompanies {
			companyIDs = append(companyIDs, r.CompanyID)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if len(orgIDs) == 0 && len(companyIDs) == 0 {
		return &SystemTableDetachResult{}, nil
	}

	removed := auditNameDiff{Removed: []string{name}}
	for _, orgID := range orgIDs {
		oid := orgID
		s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &oid, models.OrganizationActionTablesChanged, &callerUserID, removed)
	}
	for _, companyID := range companyIDs {
		cid := companyID
		s.recorder.Log(ctx, nil, models.AuditEntityCompany, &cid, models.CompanyActionTablesChanged, &callerUserID, removed)
	}

	slog.Info("таблица отвязана от всех орг/компаний", "id", id, "orgs", len(orgIDs), "companies", len(companyIDs))
	return &SystemTableDetachResult{
		OrganizationsDetached: len(orgIDs),
		CompaniesDetached:     len(companyIDs),
	}, nil
}

// DetachOrganization снимает привязку таблицы к ОДНОЙ организации. Идемпотентно:
// если привязки уже нет, возвращает false без ошибки. Аудит на организацию
// пишем только при реальном удалении строки, removed = имя таблицы.
func (s *systemTableService) DetachOrganization(ctx context.Context, callerUserID, id, organizationID int) (bool, error) {
	name, ok := s.loadTableNameForBinding(ctx, id)
	if !ok {
		return false, echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}
	res := s.db.WithContext(ctx).
		Where("table_id = ? AND organization_id = ?", id, organizationID).
		Delete(&models.OrganizationTable{})
	if res.Error != nil {
		slog.Error("не удалось отвязать организацию от таблицы", "id", id, "organization_id", organizationID, "error", res.Error)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Error detaching organization")
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	oid := organizationID
	s.recorder.Log(ctx, nil, models.AuditEntityOrganization, &oid, models.OrganizationActionTablesChanged, &callerUserID, auditNameDiff{Removed: []string{name}})
	slog.Info("таблица отвязана от организации", "id", id, "organization_id", organizationID)
	return true, nil
}

// DetachCompany снимает привязку таблицы к ОДНОЙ компании (зеркало
// DetachOrganization, см. его комментарий).
func (s *systemTableService) DetachCompany(ctx context.Context, callerUserID, id, companyID int) (bool, error) {
	name, ok := s.loadTableNameForBinding(ctx, id)
	if !ok {
		return false, echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}
	res := s.db.WithContext(ctx).
		Where("table_id = ? AND company_id = ?", id, companyID).
		Delete(&models.CompaniesTable{})
	if res.Error != nil {
		slog.Error("не удалось отвязать компанию от таблицы", "id", id, "company_id", companyID, "error", res.Error)
		return false, echo.NewHTTPError(http.StatusInternalServerError, "Error detaching company")
	}
	if res.RowsAffected == 0 {
		return false, nil
	}
	cid := companyID
	s.recorder.Log(ctx, nil, models.AuditEntityCompany, &cid, models.CompanyActionTablesChanged, &callerUserID, auditNameDiff{Removed: []string{name}})
	slog.Info("таблица отвязана от компании", "id", id, "company_id", companyID)
	return true, nil
}

// Restore восстанавливает мягко удалённую системную таблицу (is_active=false -> true).
func (s *systemTableService) Restore(ctx context.Context, id int) error {
	var table models.SystemTable
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&table).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system table")
	}
	if table.IsActive {
		return echo.NewHTTPError(http.StatusBadRequest, "Таблица не в архиве")
	}

	result := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Where("id = ?", id).
		Update("is_active", true)
	if result.Error != nil {
		slog.Error("не удалось восстановить системную таблицу", "id", id, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error restoring system table")
	}

	slog.Info("системная таблица восстановлена из архива", "id", id)
	s.notifyTablesChanged(ctx)
	return nil
}

// findTableName достаёт человекочитаемое имя таблицы по id для BulkItemError,
// НЕ фильтруя по is_active. GetByID для этого не годится: он матчит только
// активные таблицы (Where("is_active = true")), а BulkRestore как раз работает
// над архивными - через GetByID имя архивной таблицы никогда бы не нашлось, и
// частичный успех восстановления сообщал бы "не найдена" по каждой строке.
// Пустая строка (таблица не существует вовсе) - FE отображает id-фолбэком.
func (s *systemTableService) findTableName(ctx context.Context, id int) string {
	var t models.SystemTable
	if err := s.db.WithContext(ctx).Select("display_name", "name").Where("id = ?", id).First(&t).Error; err != nil {
		return ""
	}
	if t.DisplayName != nil && *t.DisplayName != "" {
		return *t.DisplayName
	}
	return t.Name
}

// BulkArchive архивирует набор системных таблиц через Delete (мягкое удаление).
// Несуществующие/непривязываемые (org/company) -> в Errors (частичный успех 207),
// не валят операцию. Дубли id дедуплицируются.
func (s *systemTableService) BulkArchive(ctx context.Context, ids []int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		name := s.findTableName(ctx, id)
		if err := s.Delete(ctx, id); err != nil {
			res.addError(id, name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// BulkRestore восстанавливает набор системных таблиц через Restore.
func (s *systemTableService) BulkRestore(ctx context.Context, ids []int) (*BulkOpResult, error) {
	res := newBulkResult()
	for _, id := range uniqueInts(ids) {
		name := s.findTableName(ctx, id)
		if err := s.Restore(ctx, id); err != nil {
			res.addError(id, name, bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}
	return res.finalize(), nil
}

// UpdateFields bulk-обновляет видимость и (опционально) порядок столбцов таблицы.
// Поля, отсутствующие в БД, игнорируются. DisplayOrder применяется только если задан.
func (s *systemTableService) UpdateFields(ctx context.Context, tableID int, req models.UpdateFieldsRequest) error {
	var table models.SystemTable
	if err := s.db.WithContext(ctx).Where("id = ?", tableID).First(&table).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system table")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, f := range req.Fields {
			updates := map[string]interface{}{
				"is_visible": f.IsVisible,
			}
			if f.DisplayOrder != nil {
				updates["display_order"] = *f.DisplayOrder
			}
			if f.Width != nil {
				updates["width"] = *f.Width
			}
			if f.Priority != nil {
				if *f.Priority < 1 || *f.Priority > 5 {
					return echo.NewHTTPError(http.StatusBadRequest, "priority должен быть от 1 до 5")
				}
				updates["priority"] = *f.Priority
			}
			if f.EnlargedIsVisible != nil {
				updates["enlarged_is_visible"] = *f.EnlargedIsVisible
			}
			if f.EnlargedWidth != nil {
				if *f.EnlargedWidth < 0 || *f.EnlargedWidth > 100 {
					return echo.NewHTTPError(http.StatusBadRequest, "enlarged_width должен быть от 0 до 100")
				}
				updates["enlarged_width"] = *f.EnlargedWidth
			}
			if f.EnlargedFontWeight != nil {
				switch *f.EnlargedFontWeight {
				case 0, 400, 500, 600, 700, 800:
					updates["enlarged_font_weight"] = *f.EnlargedFontWeight
				default:
					return echo.NewHTTPError(http.StatusBadRequest, "enlarged_font_weight должен быть 400, 500, 600, 700, 800 или 0")
				}
			}
			result := tx.Model(&models.TableField{}).
				Where("table_id = ? AND field_name = ?", tableID, f.FieldName).
				Updates(updates)
			if result.Error != nil {
				slog.Error("не удалось обновить столбец",
					"table_id", tableID, "field", f.FieldName, "error", result.Error)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating field")
			}
			// GORM пропускает enlarged_is_visible=false (zero-value bool) -
			// дофиксим явным Update как для fact-полей.
			if f.EnlargedIsVisible != nil && !*f.EnlargedIsVisible {
				if err := tx.Model(&models.TableField{}).
					Where("table_id = ? AND field_name = ?", tableID, f.FieldName).
					Update("enlarged_is_visible", false).Error; err != nil {
					return echo.NewHTTPError(http.StatusInternalServerError, "Error setting enlarged visibility")
				}
			}
		}
		return nil
	})
}

// UpdateFactFields - тоже что UpdateFields, но для table_fields_fact.
func (s *systemTableService) UpdateFactFields(ctx context.Context, tableID int, req models.UpdateFieldsRequest) error {
	var table models.SystemTable
	if err := s.db.WithContext(ctx).Where("id = ?", tableID).First(&table).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system table")
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, f := range req.Fields {
			updates := map[string]interface{}{
				"is_visible": f.IsVisible,
			}
			if f.DisplayOrder != nil {
				updates["display_order"] = *f.DisplayOrder
			}
			if f.Width != nil {
				updates["width"] = *f.Width
			}
			if f.Priority != nil {
				if *f.Priority < 1 || *f.Priority > 5 {
					return echo.NewHTTPError(http.StatusBadRequest, "priority должен быть от 1 до 5")
				}
				updates["priority"] = *f.Priority
			}
			result := tx.Model(&models.TableFieldFact{}).
				Where("table_id = ? AND field_name = ?", tableID, f.FieldName).
				Updates(updates)
			if result.Error != nil {
				slog.Error("не удалось обновить факт-столбец",
					"table_id", tableID, "field", f.FieldName, "error", result.Error)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error updating fact field")
			}
		}
		return nil
	})
}

// SeedMissingFields добавляет недостающие default-поля во все существующие таблицы.
// Уже существующие поля не трогает (сохраняет ранее настроенную видимость).
// Идемпотентна - повторные вызовы безопасны. Вызывается один раз при старте сервиса.
func (s *systemTableService) SeedMissingFields(ctx context.Context) error {
	var tables []models.SystemTable
	if err := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Find(&tables).Error; err != nil {
		return fmt.Errorf("load tables: %w", err)
	}

	added := 0
	for _, t := range tables {
		defaults := getDefaultFields(t.TableType)
		if len(defaults) == 0 {
			continue
		}

		var existing []models.TableField
		if err := s.db.WithContext(ctx).
			Where("table_id = ?", t.ID).
			Find(&existing).Error; err != nil {
			slog.Error("не удалось загрузить существующие поля таблицы",
				"table_id", t.ID, "error", err)
			continue
		}

		existingSet := make(map[string]bool, len(existing))
		maxOrder := -1
		for _, f := range existing {
			existingSet[f.FieldName] = true
			if f.DisplayOrder != nil && *f.DisplayOrder > maxOrder {
				maxOrder = *f.DisplayOrder
			}
		}

		// Backfill: для существующих полей с width=0 / priority=0 (старые записи) -
		// проставляем дефолты из getDefaultFields.
		// Отдельно: одноразовый backfill priority когда AutoMigrate проставил всем
		// строкам default 3 (gorm tag). Признак "первый запуск после миграции" -
		// у ВСЕХ полей таблицы priority == 3, при этом каталог содержит != 3.
		// В этом случае применяем каталог. Если хоть одно поле уже не 3 (то есть
		// уже backfill отработал или админ что-то правил) - не трогаем.
		defaultsByName := make(map[string]defaultField, len(defaults))
		hasCatalogVariance := false
		for _, d := range defaults {
			defaultsByName[d.Name] = d
			if d.Priority != 0 && d.Priority != 3 {
				hasCatalogVariance = true
			}
		}
		allDefaultPriority := true
		for _, f := range existing {
			if f.Priority != 3 {
				allDefaultPriority = false
				break
			}
		}
		initialPriorityMigration := allDefaultPriority && hasCatalogVariance
		for _, f := range existing {
			d, ok := defaultsByName[f.FieldName]
			if !ok {
				continue
			}
			backfill := map[string]interface{}{}
			if f.Width == 0 && d.Width != 0 {
				backfill["width"] = d.Width
			}
			needPriority := f.Priority == 0 || initialPriorityMigration
			if needPriority {
				p := d.Priority
				if p == 0 {
					p = 3
				}
				if p != f.Priority {
					backfill["priority"] = p
				}
			}
			if len(backfill) == 0 {
				continue
			}
			if err := s.db.WithContext(ctx).Model(&models.TableField{}).
				Where("id = ?", f.ID).
				Updates(backfill).Error; err != nil {
				slog.Error("не удалось backfill столбца",
					"table_id", t.ID, "field", f.FieldName, "error", err)
			}
		}

		for _, d := range defaults {
			if existingSet[d.Name] {
				continue
			}
			maxOrder++
			order := maxOrder
			fieldType := d.FieldType
			priority := d.Priority
			if priority == 0 {
				priority = 3
			}
			tf := models.TableField{
				TableID:      t.ID,
				FieldName:    d.Name,
				FieldType:    &fieldType,
				DisplayOrder: &order,
				IsVisible:    d.IsVisible,
				Width:        d.Width,
				Priority:     priority,
			}
			if err := s.db.WithContext(ctx).Create(&tf).Error; err != nil {
				slog.Error("не удалось добавить отсутствующее поле",
					"table_id", t.ID, "field", d.Name, "error", err)
				continue
			}
			added++
		}
	}

	if added > 0 {
		slog.Info("досидил отсутствующие поля таблиц (#345)", "added", added)
	}

	// Параллельно сидим факт-поля для таблиц, где показывается фактовая таблица.
	// Для остальных таблиц фактовые поля не создаём - они появятся, когда админ
	// включит "Отображать таблицу по факту" (см. ensureFactFields).
	if err := s.seedFactFields(ctx, tables); err != nil {
		return err
	}
	return nil
}

// seedFactFields идентичен SeedMissingFields, но для table_fields_fact.
// Создаёт записи только для таблиц с ShowFactTable=true. Без backfill priority
// (поля при первом запуске уже создаются с правильными priority через Create).
func (s *systemTableService) seedFactFields(ctx context.Context, tables []models.SystemTable) error {
	added := 0
	for _, t := range tables {
		if !t.ShowFactTable {
			continue
		}
		defaults := getDefaultFactFields(t.TableType)
		if len(defaults) == 0 {
			continue
		}
		var existing []models.TableFieldFact
		if err := s.db.WithContext(ctx).
			Where("table_id = ?", t.ID).
			Find(&existing).Error; err != nil {
			slog.Error("не удалось загрузить существующие факт-поля",
				"table_id", t.ID, "error", err)
			continue
		}
		existingSet := make(map[string]bool, len(existing))
		maxOrder := -1
		for _, f := range existing {
			existingSet[f.FieldName] = true
			if f.DisplayOrder != nil && *f.DisplayOrder > maxOrder {
				maxOrder = *f.DisplayOrder
			}
		}
		for _, d := range defaults {
			if existingSet[d.Name] {
				continue
			}
			maxOrder++
			order := maxOrder
			fieldType := d.FieldType
			priority := d.Priority
			if priority == 0 {
				priority = 3
			}
			tff := models.TableFieldFact{
				TableID:      t.ID,
				FieldName:    d.Name,
				FieldType:    &fieldType,
				DisplayOrder: &order,
				IsVisible:    d.IsVisible,
				Width:        d.Width,
				Priority:     priority,
			}
			if err := s.db.WithContext(ctx).Create(&tff).Error; err != nil {
				slog.Error("не удалось добавить факт-поле",
					"table_id", t.ID, "field", d.Name, "error", err)
				continue
			}
			// GORM v2 при Create игнорирует zero-value bool, БД применяет
			// default:true вместо явного false. Дофикс через Update.
			if !d.IsVisible {
				if err := s.db.WithContext(ctx).Model(&models.TableFieldFact{}).
					Where("id = ?", tff.ID).
					Update("is_visible", false).Error; err != nil {
					slog.Error("не удалось проставить is_visible=false",
						"table_id", t.ID, "field", d.Name, "error", err)
				}
			}
			added++
		}
	}
	if added > 0 {
		slog.Info("досидил факт-поля таблиц (#345)", "added", added)
	}
	// Одноразовый фикс: если у таблицы ВСЕ факт-поля is_visible=true и каталог
	// содержит скрытые - значит сидинг отработал с багом GORM. Применяем каталог.
	if err := s.fixFactVisibilityFromCatalog(ctx, tables); err != nil {
		return err
	}
	return nil
}

// fixFactVisibilityFromCatalog исправляет is_visible на каталожное значение
// для таблиц где ВСЕ факт-поля сейчас true (признак бага из первого PR-B
// релиза). Идемпотентно: если хоть одно поле уже false - не трогаем.
func (s *systemTableService) fixFactVisibilityFromCatalog(ctx context.Context, tables []models.SystemTable) error {
	fixed := 0
	for _, t := range tables {
		if !t.ShowFactTable {
			continue
		}
		defaults := getDefaultFactFields(t.TableType)
		if len(defaults) == 0 {
			continue
		}
		var existing []models.TableFieldFact
		if err := s.db.WithContext(ctx).
			Where("table_id = ?", t.ID).
			Find(&existing).Error; err != nil {
			continue
		}
		allVisible := true
		for _, f := range existing {
			if !f.IsVisible {
				allVisible = false
				break
			}
		}
		anyShouldBeHidden := false
		for _, d := range defaults {
			if !d.IsVisible {
				anyShouldBeHidden = true
				break
			}
		}
		if !(allVisible && anyShouldBeHidden) {
			continue
		}
		defByName := make(map[string]bool, len(defaults))
		for _, d := range defaults {
			defByName[d.Name] = d.IsVisible
		}
		for _, f := range existing {
			want, ok := defByName[f.FieldName]
			if !ok || want == f.IsVisible {
				continue
			}
			if err := s.db.WithContext(ctx).Model(&models.TableFieldFact{}).
				Where("id = ?", f.ID).
				Update("is_visible", want).Error; err != nil {
				slog.Error("не удалось fix факт-видимость",
					"table_id", t.ID, "field", f.FieldName, "error", err)
				continue
			}
			fixed++
		}
	}
	if fixed > 0 {
		slog.Info("исправил факт-видимость до каталога (#345)", "fixed", fixed)
	}
	return nil
}

// GetHistory возвращает историю изменений системной таблицы (новые сверху).
// Переходный период #870: запись идёт в audit_log, старые строки — в замороженной
// system_table_histories. Union объединяет обе таблицы в идентичную форму ответа.
func (s *systemTableService) GetHistory(ctx context.Context, tableID int) ([]models.SystemTableHistoryItem, error) {
	const actorName = `COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name)), ''), u.username, '')`
	// Read-switch #870 (F.3): до-cutover строки system_table_histories подняты в
	// audit_log разовым backfill'ом (details уже jsonb, verbatim), читаем только
	// audit_log. Старая таблица system_table_histories дропнута в дроп-sweep (F.8).
	sql := `
		SELECT a.id, a.action AS action_type, a.details, a.actor_user_id AS user_id,
			` + actorName + ` AS user_name,
			a.created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_user_id
		WHERE a.entity_type = ? AND a.entity_id = ?
		ORDER BY a.created_at DESC, a.id DESC`

	type row struct {
		ID         int             `gorm:"column:id"`
		ActionType string          `gorm:"column:action_type"`
		Details    json.RawMessage `gorm:"column:details"`
		UserID     *int            `gorm:"column:user_id"`
		UserName   string          `gorm:"column:user_name"`
		CreatedAt  time.Time       `gorm:"column:created_at"`
	}
	var rows []row
	if err := s.db.WithContext(ctx).Raw(sql, models.AuditEntitySystemTable, tableID).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system table history")
	}

	// Логин вместо ФИО у акторов, не давших согласия на обработку данных.
	masks := loadConsentMasks(ctx, s.db)
	items := make([]models.SystemTableHistoryItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, models.SystemTableHistoryItem{
			ID:         r.ID,
			ActionType: r.ActionType,
			Details:    r.Details,
			UserID:     r.UserID,
			UserName:   maskName(masks, r.UserID, r.UserName),
			CreatedAt:  r.CreatedAt,
		})
	}
	return items, nil
}
