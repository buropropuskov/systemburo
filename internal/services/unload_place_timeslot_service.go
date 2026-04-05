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

// GetTimeSlots возвращает временные слоты места разгрузки.
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

// AddTimeSlot добавляет временной слот к месту разгрузки.
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
		slog.Error("не удалось добавить временной слот", "place_id", placeID, "error", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error adding time slot")
	}
	slog.Info("временной слот добавлен", "id", slot.ID, "place_id", placeID)
	return slot.ID, nil
}

// UpdateTimeSlot обновляет временной слот места разгрузки.
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
		slog.Error("не удалось обновить временной слот", "slot_id", slotID, "place_id", placeID, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating time slot")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Временной слот не найден")
	}
	slog.Info("временной слот обновлён", "slot_id", slotID, "place_id", placeID)
	return nil
}

// DeleteTimeSlot удаляет временной слот места разгрузки.
func (s *unloadPlaceService) DeleteTimeSlot(ctx context.Context, placeID, slotID int) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND unload_place_id = ?", slotID, placeID).
		Delete(&models.UnloadPlaceTimeSlot{})
	if result.Error != nil {
		slog.Error("не удалось удалить временной слот", "slot_id", slotID, "place_id", placeID, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting time slot")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Временной слот не найден")
	}
	slog.Info("временной слот удалён", "slot_id", slotID, "place_id", placeID)
	return nil
}
