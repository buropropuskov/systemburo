package services

import (
	"context"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// allowedImageTypes -- допустимые MIME-типы для загрузки фотографий.
var allowedImageTypes = []string{
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
}

// SystemTableService -- интерфейс бизнес-логики системных таблиц.
type SystemTableService interface {
	GetAll(ctx context.Context) ([]models.SystemTableWithDetails, error)
	GetByID(ctx context.Context, id int) (*models.SystemTableWithDetails, error)
	GetByName(ctx context.Context, name string) (*models.SystemTableWithDetails, error)
	Create(ctx context.Context, req models.CreateSystemTableRequest) (int, error)
	Update(ctx context.Context, id int, req models.UpdateSystemTableRequest) error
	Delete(ctx context.Context, id int) error

	// Временные слоты
	GetTimeSlots(ctx context.Context, tableID int) ([]models.SystemTableTimeSlot, error)
	AddTimeSlot(ctx context.Context, tableID int, req models.CreateTimeSlotRequest) (int, error)
	UpdateTimeSlot(ctx context.Context, tableID, slotID int, req models.UpdateTimeSlotRequest) error
	DeleteTimeSlot(ctx context.Context, tableID, slotID int) error

	// Фотографии
	UploadPhoto(ctx context.Context, tableID int, username string, file *multipart.FileHeader) (int, error)
	DeletePhoto(ctx context.Context, tableID, photoID int) error
	SetMainPhoto(ctx context.Context, tableID, photoID int) error
}

type systemTableService struct {
	db          *gorm.DB
	uploadDir   string
	maxFileSize int64
	permSvc     PermissionService
}

// NewSystemTableService создаёт реализацию SystemTableService.
func NewSystemTableService(db *gorm.DB, uploadDir string, maxFileSize int64, permSvc PermissionService) SystemTableService {
	return &systemTableService{
		db:          db,
		uploadDir:   uploadDir,
		maxFileSize: maxFileSize,
		permSvc:     permSvc,
	}
}

// computeCurrentStatus вычисляет текущий статус (open/closed) на основании расписания и статуса таблицы.
func computeCurrentStatus(tableStatus string, slots []models.SystemTableTimeSlot) string {
	if tableStatus != "active" {
		return "closed"
	}

	now := time.Now()
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
		Preload("TimeSlots", func(db *gorm.DB) *gorm.DB {
			return db.Order("day_of_week, open_time")
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
	if table.TimeSlots == nil {
		table.TimeSlots = []models.SystemTableTimeSlot{}
	}
	if table.Photos == nil {
		table.Photos = []models.SystemTablePhoto{}
	}

	status := "active"
	if table.Status != "" {
		status = table.Status
	}

	return &models.SystemTableWithDetails{
		Table:         table,
		Fields:        table.Fields,
		TimeSlots:     table.TimeSlots,
		Photos:        table.Photos,
		CurrentStatus: computeCurrentStatus(status, table.TimeSlots),
	}, nil
}

// GetAll возвращает все активные системные таблицы с полями, слотами и фотографиями.
func (s *systemTableService) GetAll(ctx context.Context) ([]models.SystemTableWithDetails, error) {
	tables := make([]models.SystemTable, 0)
	if err := s.db.WithContext(ctx).
		Preload("Fields", func(db *gorm.DB) *gorm.DB {
			return db.Order("display_order")
		}).
		Preload("TimeSlots", func(db *gorm.DB) *gorm.DB {
			return db.Order("day_of_week, open_time")
		}).
		Preload("Photos", func(db *gorm.DB) *gorm.DB {
			return db.Order("is_main DESC, uploaded_at DESC")
		}).
		Where("is_active = ?", true).
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
		slots := t.TimeSlots
		if slots == nil {
			slots = []models.SystemTableTimeSlot{}
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
			Table:         t,
			Fields:        fields,
			TimeSlots:     slots,
			Photos:        photos,
			CurrentStatus: computeCurrentStatus(status, slots),
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

// GetByName возвращает системную таблицу по имени с деталями.
func (s *systemTableService) GetByName(ctx context.Context, name string) (*models.SystemTableWithDetails, error) {
	query := s.db.WithContext(ctx).Where("name = ? AND is_active = ?", name, true)
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
type defaultField struct {
	Name      string
	FieldType string
}

// getDefaultFields возвращает набор полей по умолчанию для типа таблицы.
func getDefaultFields(tableType string) []defaultField {
	switch tableType {
	case "cars":
		return []defaultField{
			{"car_number", "text"},
			{"car_brand", "text"},
			{"organization", "text"},
			{"unload_place", "text"},
			{"valid_until", "date"},
			{"time_range", "text"},
			{"status", "text"},
		}
	case "people":
		return []defaultField{
			{"organization", "text"},
			{"last_name", "text"},
			{"first_name", "text"},
			{"middle_name", "text"},
			{"valid_until", "date"},
			{"pass_time", "text"},
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
			tf := models.TableField{
				TableID:      table.ID,
				FieldName:    f.Name,
				FieldType:    &fieldType,
				DisplayOrder: &order,
				IsVisible:    true,
			}
			if err := tx.Create(&tf).Error; err != nil {
				slog.Error("не удалось создать поле таблицы", "table_id", table.ID, "field", f.Name, "error", err)
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating table fields")
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
		"updated_at": time.Now(),
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

	slog.Info("системная таблица обновлена", "id", id)
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
		slog.Error("не ��далось удалить системную таблицу", "id", id, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting system table")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}

	slog.Info("сист��мная таблица удалена (мягко)", "id", id)
	return nil
}

