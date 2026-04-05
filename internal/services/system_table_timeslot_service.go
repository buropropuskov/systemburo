package services

import (
	"context"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// GetTimeSlots возвращает временные слоты системной таблицы.
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

// AddTimeSlot добавляет временной слот к системной таблице.
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

// UpdateTimeSlot обновляет временной слот системной таблицы.
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

// DeleteTimeSlot удаляет временной слот системной таблицы.
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
