package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"systemburo/internal/models"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	maxFileSize             = 10 * 1024 * 1024 // 10 MB
	systemTableUploadDir    = "./uploads/system_tables"
	systemTableUploadPrefix = "/uploads/system_tables"
)

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
	db *gorm.DB
}

// NewSystemTableService создаёт реализацию SystemTableService.
func NewSystemTableService(db *gorm.DB) SystemTableService {
	return &systemTableService{db: db}
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

// loadTableDetails загружает поля, слоты и фото для одной таблицы.
func (s *systemTableService) loadTableDetails(ctx context.Context, table models.SystemTable) (*models.SystemTableWithDetails, error) {
	fields := make([]models.TableField, 0)
	if err := s.db.WithContext(ctx).
		Where("table_id = ?", table.ID).
		Order("display_order").
		Find(&fields).Error; err != nil {
		fields = []models.TableField{}
	}

	slots := make([]models.SystemTableTimeSlot, 0)
	if err := s.db.WithContext(ctx).
		Where("table_id = ?", table.ID).
		Order("day_of_week, open_time").
		Find(&slots).Error; err != nil {
		slots = []models.SystemTableTimeSlot{}
	}

	photos := make([]models.SystemTablePhoto, 0)
	if err := s.db.WithContext(ctx).
		Where("table_id = ?", table.ID).
		Order("is_main DESC, uploaded_at DESC").
		Find(&photos).Error; err != nil {
		photos = []models.SystemTablePhoto{}
	}

	status := "active"
	if table.Status != "" {
		status = table.Status
	}

	return &models.SystemTableWithDetails{
		Table:         table,
		Fields:        fields,
		TimeSlots:     slots,
		Photos:        photos,
		CurrentStatus: computeCurrentStatus(status, slots),
	}, nil
}

func (s *systemTableService) GetAll(ctx context.Context) ([]models.SystemTableWithDetails, error) {
	tables := make([]models.SystemTable, 0)
	if err := s.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("display_name").
		Find(&tables).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system tables")
	}

	result := make([]models.SystemTableWithDetails, 0, len(tables))
	for _, t := range tables {
		details, err := s.loadTableDetails(ctx, t)
		if err != nil {
			return nil, err
		}
		result = append(result, *details)
	}

	return result, nil
}

func (s *systemTableService) GetByID(ctx context.Context, id int) (*models.SystemTableWithDetails, error) {
	var table models.SystemTable
	if err := s.db.WithContext(ctx).
		Where("id = ? AND is_active = ?", id, true).
		First(&table).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system table")
	}

	return s.loadTableDetails(ctx, table)
}

func (s *systemTableService) GetByName(ctx context.Context, name string) (*models.SystemTableWithDetails, error) {
	var table models.SystemTable
	if err := s.db.WithContext(ctx).
		Where("name = ? AND is_active = ?", name, true).
		First(&table).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Таблица не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching system table")
	}

	return s.loadTableDetails(ctx, table)
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
				return echo.NewHTTPError(http.StatusInternalServerError, "Error creating table fields")
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return table.ID, nil
}

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
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating system table")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}

	return nil
}

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
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting system table")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}

	return nil
}

// --- Временные слоты ---

func (s *systemTableService) GetTimeSlots(ctx context.Context, tableID int) ([]models.SystemTableTimeSlot, error) {
	slots := make([]models.SystemTableTimeSlot, 0)
	if err := s.db.WithContext(ctx).
		Where("table_id = ?", tableID).
		Order("day_of_week, open_time").
		Find(&slots).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching time slots")
	}
	return slots, nil
}

func (s *systemTableService) AddTimeSlot(ctx context.Context, tableID int, req models.CreateTimeSlotRequest) (int, error) {
	// Проверяем существование таблицы
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Where("id = ? AND is_active = ?", tableID, true).
		Count(&count).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error checking system table")
	}
	if count == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}

	// Валидация времени (формат HH:MM)
	if _, err := time.Parse("15:04", req.OpenTime); err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Неверный формат времени открытия. Используйте ЧЧ:ММ")
	}
	if _, err := time.Parse("15:04", req.CloseTime); err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Неверный формат времени закрытия. Используйте ЧЧ:ММ")
	}

	if req.DayOfWeek < 0 || req.DayOfWeek > 6 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "День недели должен быть от 0 (Пн) до 6 (Вс)")
	}

	isNextDay := req.IsNextDay != nil && *req.IsNextDay
	isActive := req.IsActive == nil || *req.IsActive

	slot := models.SystemTableTimeSlot{
		TableID:   tableID,
		DayOfWeek: req.DayOfWeek,
		OpenTime:  req.OpenTime,
		CloseTime: req.CloseTime,
		IsNextDay: isNextDay,
		IsActive:  isActive,
	}

	if err := s.db.WithContext(ctx).Create(&slot).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error adding time slot")
	}

	return slot.ID, nil
}

func (s *systemTableService) UpdateTimeSlot(ctx context.Context, tableID, slotID int, req models.UpdateTimeSlotRequest) error {
	var slot models.SystemTableTimeSlot
	if err := s.db.WithContext(ctx).
		Where("id = ? AND table_id = ?", slotID, tableID).
		First(&slot).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Временной слот не найден")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching time slot")
	}

	if req.DayOfWeek != nil {
		if *req.DayOfWeek < 0 || *req.DayOfWeek > 6 {
			return echo.NewHTTPError(http.StatusBadRequest, "День недели должен быть от 0 (Пн) до 6 (Вс)")
		}
		slot.DayOfWeek = *req.DayOfWeek
	}
	if req.OpenTime != nil {
		if _, err := time.Parse("15:04", *req.OpenTime); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Неверный формат времени открытия")
		}
		slot.OpenTime = *req.OpenTime
	}
	if req.CloseTime != nil {
		if _, err := time.Parse("15:04", *req.CloseTime); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Неверный формат времени закрытия")
		}
		slot.CloseTime = *req.CloseTime
	}
	if req.IsNextDay != nil {
		slot.IsNextDay = *req.IsNextDay
	}
	if req.IsActive != nil {
		slot.IsActive = *req.IsActive
	}

	result := s.db.WithContext(ctx).
		Model(&models.SystemTableTimeSlot{}).
		Where("id = ? AND table_id = ?", slotID, tableID).
		Updates(map[string]interface{}{
			"day_of_week": slot.DayOfWeek,
			"open_time":   slot.OpenTime,
			"close_time":  slot.CloseTime,
			"is_next_day": slot.IsNextDay,
			"is_active":   slot.IsActive,
			"updated_at":  time.Now(),
		})
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating time slot")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Временной слот не найден")
	}
	return nil
}

func (s *systemTableService) DeleteTimeSlot(ctx context.Context, tableID, slotID int) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND table_id = ?", slotID, tableID).
		Delete(&models.SystemTableTimeSlot{})
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting time slot")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Временной слот не найден")
	}
	return nil
}

// --- Фотографии ---

func (s *systemTableService) UploadPhoto(ctx context.Context, tableID int, username string, file *multipart.FileHeader) (int, error) {
	// Получаем ID пользователя
	var userID int
	err := s.db.WithContext(ctx).
		Table("users").
		Select("id").
		Where("username = ?", username).
		Row().
		Scan(&userID)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	// Проверяем существование таблицы
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Where("id = ? AND is_active = ?", tableID, true).
		Count(&count).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if count == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}

	if file.Size > maxFileSize {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "File too large. Max 10MB")
	}

	// Создаём директорию
	if err := os.MkdirAll(systemTableUploadDir, 0o755); err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create upload directory")
	}

	ext := filepath.Ext(file.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	uniqueName := fmt.Sprintf("%s_%d%s", uuid.New().String(), tableID, ext)
	savePath := filepath.Join(systemTableUploadDir, uniqueName)
	fileURL := fmt.Sprintf("%s/%s", systemTableUploadPrefix, uniqueName)

	src, err := file.Open()
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Error reading file")
	}
	defer src.Close()

	dst, err := os.Create(savePath)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Failed to write file")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Failed to write file")
	}

	// Определяем MIME
	mimeType := file.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Первая фотография -- главная
	var photoCount int64
	s.db.WithContext(ctx).Model(&models.SystemTablePhoto{}).
		Where("table_id = ?", tableID).
		Count(&photoCount)

	isMain := photoCount == 0
	fileSize := file.Size

	photo := models.SystemTablePhoto{
		TableID:  tableID,
		PhotoURL: fileURL,
		FileName: &file.Filename,
		FileSize: &fileSize,
		MimeType: &mimeType,
		IsMain:   isMain,
		UploadedBy: &userID,
	}

	if err := s.db.WithContext(ctx).Create(&photo).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	return photo.ID, nil
}

func (s *systemTableService) DeletePhoto(ctx context.Context, tableID, photoID int) error {
	var photo models.SystemTablePhoto
	if err := s.db.WithContext(ctx).
		Where("id = ? AND table_id = ?", photoID, tableID).
		First(&photo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Фотография не найдена")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching photo")
	}

	// Удаляем файл
	filePath := "." + photo.PhotoURL
	if _, err := os.Stat(filePath); err == nil {
		_ = os.Remove(filePath)
	}

	// Удаляем запись
	result := s.db.WithContext(ctx).
		Where("id = ? AND table_id = ?", photoID, tableID).
		Delete(&models.SystemTablePhoto{})
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting photo")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Фотография не найдена")
	}

	// Если удалили главную -- назначаем следующую
	if photo.IsMain {
		var next models.SystemTablePhoto
		if err := s.db.WithContext(ctx).
			Where("table_id = ? AND id != ?", tableID, photoID).
			Order("uploaded_at").
			First(&next).Error; err == nil {
			s.db.WithContext(ctx).
				Model(&models.SystemTablePhoto{}).
				Where("id = ?", next.ID).
				Update("is_main", true)
		}
	}

	return nil
}

func (s *systemTableService) SetMainPhoto(ctx context.Context, tableID, photoID int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Сбрасываем is_main для всех фото таблицы
		if err := tx.Model(&models.SystemTablePhoto{}).
			Where("table_id = ?", tableID).
			Update("is_main", false).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error resetting main photo")
		}

		// Устанавливаем новую главную
		result := tx.Model(&models.SystemTablePhoto{}).
			Where("id = ? AND table_id = ?", photoID, tableID).
			Update("is_main", true)
		if result.Error != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error setting main photo")
		}
		if result.RowsAffected == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "Фотография не найдена")
		}

		return nil
	})
}
