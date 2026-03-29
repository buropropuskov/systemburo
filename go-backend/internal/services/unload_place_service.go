package services

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CreateUnloadPlaceRequest -- тело запроса на создание места разгрузки.
type CreateUnloadPlaceRequest struct {
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	MapLink       *string `json:"map_link"`
	Status        *string `json:"status"`
	StatusComment *string `json:"status_comment"`
}

// UpdateUnloadPlaceRequest -- тело запроса на обновление места разгрузки.
type UpdateUnloadPlaceRequest struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	MapLink       *string `json:"map_link"`
	Status        *string `json:"status"`
	StatusComment *string `json:"status_comment"`
}

// CreateTimeSlotRequest -- тело запроса на создание временного слота.
type CreateTimeSlotRequest struct {
	DayOfWeek int     `json:"day_of_week"`
	OpenTime  string  `json:"open_time"`
	CloseTime string  `json:"close_time"`
	IsNextDay *bool   `json:"is_next_day"`
	IsActive  *bool   `json:"is_active"`
}

// UpdateTimeSlotRequest -- тело запроса на обновление временного слота.
type UpdateTimeSlotRequest struct {
	DayOfWeek *int    `json:"day_of_week"`
	OpenTime  *string `json:"open_time"`
	CloseTime *string `json:"close_time"`
	IsNextDay *bool   `json:"is_next_day"`
	IsActive  *bool   `json:"is_active"`
}

// UnloadPlaceWithDetails -- место разгрузки с расписанием, фотографиями и текущим статусом.
type UnloadPlaceWithDetails struct {
	ID            int                          `json:"id"`
	Name          string                       `json:"name"`
	Description   *string                      `json:"description"`
	MapLink       *string                      `json:"map_link"`
	Status        string                       `json:"status"`
	StatusComment *string                      `json:"status_comment"`
	IsActive      bool                         `json:"is_active"`
	CurrentStatus string                       `json:"current_status"`
	TimeSlots     []models.UnloadPlaceTimeSlot `json:"time_slots"`
	Photos        []models.UnloadPlacePhoto    `json:"photos"`
	CreatedAt     time.Time                    `json:"created_at"`
	UpdatedAt     time.Time                    `json:"updated_at"`
}

// UnloadPlaceService -- интерфейс бизнес-логики мест разгрузки.
type UnloadPlaceService interface {
	GetAll(ctx context.Context) ([]UnloadPlaceWithDetails, error)
	GetByID(ctx context.Context, id int) (*UnloadPlaceWithDetails, error)
	Create(ctx context.Context, req CreateUnloadPlaceRequest) (int, error)
	Update(ctx context.Context, id int, req UpdateUnloadPlaceRequest) error
	Delete(ctx context.Context, id int) error

	// Временные слоты
	GetTimeSlots(ctx context.Context, placeID int) ([]models.UnloadPlaceTimeSlot, error)
	AddTimeSlot(ctx context.Context, placeID int, req CreateTimeSlotRequest) (int, error)
	UpdateTimeSlot(ctx context.Context, placeID, slotID int, req UpdateTimeSlotRequest) error
	DeleteTimeSlot(ctx context.Context, placeID, slotID int) error

	// Фотографии
	UploadPhoto(ctx context.Context, placeID int, username string, photoURL, fileName, mimeType string, fileSize int64) (int, error)
	DeletePhoto(ctx context.Context, placeID, photoID int) (string, error)
	SetMainPhoto(ctx context.Context, placeID, photoID int) error
}

type unloadPlaceService struct {
	db *gorm.DB
}

// NewUnloadPlaceService создаёт реализацию UnloadPlaceService.
func NewUnloadPlaceService(db *gorm.DB) UnloadPlaceService {
	return &unloadPlaceService{db: db}
}

// computeUnloadPlaceStatus определяет текущий статус (open/closed) по расписанию.
func computeUnloadPlaceStatus(status string, slots []models.UnloadPlaceTimeSlot) string {
	if status != "active" {
		return "closed"
	}

	now := time.Now()
	// 0=Пн, 6=Вс (совпадает с Rust: weekday().num_days_from_monday())
	currentDay := int(now.Weekday()+6) % 7
	currentTime := now.Format("15:04")

	// Проверяем круглосуточный слот
	for _, s := range slots {
		if s.DayOfWeek == currentDay && s.IsActive &&
			s.OpenTime == "00:00" && s.CloseTime == "23:59" && !s.IsNextDay {
			return "open"
		}
	}

	// Проверяем обычные слоты
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

// buildDetails собирает UnloadPlaceWithDetails из места, его слотов и фото.
func (s *unloadPlaceService) buildDetails(ctx context.Context, place models.UnloadPlace) UnloadPlaceWithDetails {
	slots := make([]models.UnloadPlaceTimeSlot, 0)
	s.db.WithContext(ctx).
		Where("unload_place_id = ?", place.ID).
		Order("day_of_week, open_time").
		Find(&slots)

	photos := make([]models.UnloadPlacePhoto, 0)
	s.db.WithContext(ctx).
		Where("unload_place_id = ?", place.ID).
		Order("is_main DESC, uploaded_at DESC").
		Find(&photos)

	status := place.Status
	if status == "" {
		status = "active"
	}

	return UnloadPlaceWithDetails{
		ID:            place.ID,
		Name:          place.Name,
		Description:   place.Description,
		MapLink:       place.MapLink,
		Status:        status,
		StatusComment: place.StatusComment,
		IsActive:      place.IsActive,
		CurrentStatus: computeUnloadPlaceStatus(status, slots),
		TimeSlots:     slots,
		Photos:        photos,
		CreatedAt:     place.CreatedAt,
		UpdatedAt:     place.UpdatedAt,
	}
}

func (s *unloadPlaceService) GetAll(ctx context.Context) ([]UnloadPlaceWithDetails, error) {
	places := make([]models.UnloadPlace, 0)
	if err := s.db.WithContext(ctx).Order("name").Find(&places).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unload places")
	}

	result := make([]UnloadPlaceWithDetails, 0, len(places))
	for _, p := range places {
		result = append(result, s.buildDetails(ctx, p))
	}
	return result, nil
}

func (s *unloadPlaceService) GetByID(ctx context.Context, id int) (*UnloadPlaceWithDetails, error) {
	var place models.UnloadPlace
	if err := s.db.WithContext(ctx).First(&place, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unload place")
	}

	details := s.buildDetails(ctx, place)
	return &details, nil
}

func (s *unloadPlaceService) Create(ctx context.Context, req CreateUnloadPlaceRequest) (int, error) {
	status := "active"
	if req.Status != nil {
		status = *req.Status
	}

	place := models.UnloadPlace{
		Name:          req.Name,
		Description:   req.Description,
		MapLink:       req.MapLink,
		Status:        status,
		StatusComment: req.StatusComment,
		IsActive:      true,
		UpdatedAt:     time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(&place).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error creating unload place")
	}
	return place.ID, nil
}

func (s *unloadPlaceService) Update(ctx context.Context, id int, req UpdateUnloadPlaceRequest) error {
	var place models.UnloadPlace
	if err := s.db.WithContext(ctx).First(&place, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching unload place")
	}

	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
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

	result := s.db.WithContext(ctx).Model(&models.UnloadPlace{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating unload place")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}
	return nil
}

func (s *unloadPlaceService) Delete(ctx context.Context, id int) error {
	// Проверяем привязку к организациям
	var orgCount int64
	if err := s.db.WithContext(ctx).
		Model(&models.OrganizationUnloadPlace{}).
		Where("unload_place_id = ?", id).
		Count(&orgCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking organization dependencies")
	}

	// Проверяем привязку к компаниям
	var companyCount int64
	if err := s.db.WithContext(ctx).
		Model(&models.CompaniesUnloadPlace{}).
		Where("unload_place_id = ?", id).
		Count(&companyCount).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error checking company dependencies")
	}

	if orgCount > 0 || companyCount > 0 {
		msg := "Невозможно удалить место разгрузки, так как оно привязано к: "
		var parts []string
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

	// Получаем URL фотографий для удаления файлов
	var photoURLs []string
	s.db.WithContext(ctx).
		Model(&models.UnloadPlacePhoto{}).
		Where("unload_place_id = ?", id).
		Pluck("photo_url", &photoURLs)

	result := s.db.WithContext(ctx).Delete(&models.UnloadPlace{}, id)
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting unload place")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}
	return nil
}

// --- Временные слоты ---

func (s *unloadPlaceService) GetTimeSlots(ctx context.Context, placeID int) ([]models.UnloadPlaceTimeSlot, error) {
	slots := make([]models.UnloadPlaceTimeSlot, 0)
	if err := s.db.WithContext(ctx).
		Where("unload_place_id = ?", placeID).
		Order("day_of_week, open_time").
		Find(&slots).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching time slots")
	}
	return slots, nil
}

func (s *unloadPlaceService) AddTimeSlot(ctx context.Context, placeID int, req CreateTimeSlotRequest) (int, error) {
	// Проверяем существование места
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.UnloadPlace{}).Where("id = ?", placeID).Count(&count).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error checking unload place")
	}
	if count == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}

	// Валидируем формат времени
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

	slot := models.UnloadPlaceTimeSlot{
		UnloadPlaceID: placeID,
		DayOfWeek:     req.DayOfWeek,
		OpenTime:      req.OpenTime,
		CloseTime:     req.CloseTime,
		IsNextDay:     isNextDay,
		IsActive:      isActive,
		UpdatedAt:     time.Now(),
	}

	if err := s.db.WithContext(ctx).Create(&slot).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error adding time slot")
	}
	return slot.ID, nil
}

func (s *unloadPlaceService) UpdateTimeSlot(ctx context.Context, placeID, slotID int, req UpdateTimeSlotRequest) error {
	var slot models.UnloadPlaceTimeSlot
	if err := s.db.WithContext(ctx).
		Where("id = ? AND unload_place_id = ?", slotID, placeID).
		First(&slot).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Временной слот не найден")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching time slot")
	}

	// Определяем новые значения с fallback на текущие
	dayOfWeek := slot.DayOfWeek
	if req.DayOfWeek != nil {
		dayOfWeek = *req.DayOfWeek
	}
	if dayOfWeek < 0 || dayOfWeek > 6 {
		return echo.NewHTTPError(http.StatusBadRequest, "День недели должен быть от 0 (Пн) до 6 (Вс)")
	}

	openTime := slot.OpenTime
	if req.OpenTime != nil {
		if _, err := time.Parse("15:04", *req.OpenTime); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Неверный формат времени открытия")
		}
		openTime = *req.OpenTime
	}

	closeTime := slot.CloseTime
	if req.CloseTime != nil {
		if _, err := time.Parse("15:04", *req.CloseTime); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Неверный формат времени закрытия")
		}
		closeTime = *req.CloseTime
	}

	isNextDay := slot.IsNextDay
	if req.IsNextDay != nil {
		isNextDay = *req.IsNextDay
	}

	isActive := slot.IsActive
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	result := s.db.WithContext(ctx).
		Model(&models.UnloadPlaceTimeSlot{}).
		Where("id = ? AND unload_place_id = ?", slotID, placeID).
		Updates(map[string]interface{}{
			"day_of_week": dayOfWeek,
			"open_time":   openTime,
			"close_time":  closeTime,
			"is_next_day": isNextDay,
			"is_active":   isActive,
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

func (s *unloadPlaceService) DeleteTimeSlot(ctx context.Context, placeID, slotID int) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND unload_place_id = ?", slotID, placeID).
		Delete(&models.UnloadPlaceTimeSlot{})
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting time slot")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Временной слот не найден")
	}
	return nil
}

// --- Фотографии ---

func (s *unloadPlaceService) UploadPhoto(ctx context.Context, placeID int, username string, photoURL, fileName, mimeType string, fileSize int64) (int, error) {
	// Проверяем существование места
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.UnloadPlace{}).Where("id = ?", placeID).Count(&count).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if count == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}

	// Получаем ID пользователя
	var userID int
	if err := s.db.WithContext(ctx).
		Table("users").
		Select("id").
		Where("username = ?", username).
		Row().
		Scan(&userID); err != nil {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	// Определяем, должна ли быть фотография главной (первая = главная)
	var photoCount int64
	s.db.WithContext(ctx).
		Model(&models.UnloadPlacePhoto{}).
		Where("unload_place_id = ?", placeID).
		Count(&photoCount)

	isMain := photoCount == 0

	photo := models.UnloadPlacePhoto{
		UnloadPlaceID: placeID,
		PhotoURL:      photoURL,
		FileName:      &fileName,
		FileSize:      &fileSize,
		MimeType:      &mimeType,
		IsMain:        isMain,
		UploadedBy:    &userID,
	}

	if err := s.db.WithContext(ctx).Create(&photo).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	return photo.ID, nil
}

func (s *unloadPlaceService) DeletePhoto(ctx context.Context, placeID, photoID int) (string, error) {
	var photo models.UnloadPlacePhoto
	if err := s.db.WithContext(ctx).
		Where("id = ? AND unload_place_id = ?", photoID, placeID).
		First(&photo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", echo.NewHTTPError(http.StatusNotFound, "Фотография не найдена")
		}
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Error fetching photo")
	}

	photoURL := photo.PhotoURL
	wasMain := photo.IsMain

	result := s.db.WithContext(ctx).
		Where("id = ? AND unload_place_id = ?", photoID, placeID).
		Delete(&models.UnloadPlacePhoto{})
	if result.Error != nil {
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Error deleting photo")
	}
	if result.RowsAffected == 0 {
		return "", echo.NewHTTPError(http.StatusNotFound, "Фотография не найдена")
	}

	// Если удалили главную, назначаем следующую
	if wasMain {
		var next models.UnloadPlacePhoto
		if err := s.db.WithContext(ctx).
			Where("unload_place_id = ? AND id != ?", placeID, photoID).
			Order("uploaded_at").
			First(&next).Error; err == nil {
			s.db.WithContext(ctx).
				Model(&models.UnloadPlacePhoto{}).
				Where("id = ?", next.ID).
				Update("is_main", true)
		}
	}

	return photoURL, nil
}

func (s *unloadPlaceService) SetMainPhoto(ctx context.Context, placeID, photoID int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Сбрасываем is_main у всех фотографий этого места
		if err := tx.Model(&models.UnloadPlacePhoto{}).
			Where("unload_place_id = ?", placeID).
			Update("is_main", false).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error resetting main photo")
		}

		// Устанавливаем новую главную
		result := tx.Model(&models.UnloadPlacePhoto{}).
			Where("id = ? AND unload_place_id = ?", photoID, placeID).
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
