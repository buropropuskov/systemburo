package services

import (
	"context"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// timeSlots -- общий стор временных слотов для мест разгрузки. В отличие от
// системных таблиц, слот можно добавить и к архивному месту (is_active не
// проверяется -- сохранение расписания при архивации).
func (s *unloadPlaceService) timeSlots() timeSlotStore[models.UnloadPlaceTimeSlot] {
	return timeSlotStore[models.UnloadPlaceTimeSlot]{
		db:       s.db,
		entity:   "unload_place",
		fkColumn: "unload_place_id",
		checkParent: func(ctx context.Context, placeID int) error {
			var count int64
			if err := s.db.WithContext(ctx).Model(&models.UnloadPlace{}).
				Where("id = ?", placeID).Count(&count).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error checking unload place")
			}
			if count == 0 {
				return echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
			}
			return nil
		},
		newSlot: func(placeID int, req models.CreateTimeSlotRequest) models.UnloadPlaceTimeSlot {
			return models.UnloadPlaceTimeSlot{
				UnloadPlaceID: placeID,
				DayOfWeek:     req.DayOfWeek,
				OpenTime:      req.OpenTime,
				CloseTime:     req.CloseTime,
				IsNextDay:     req.IsNextDay != nil && *req.IsNextDay,
				IsActive:      req.IsActive == nil || *req.IsActive,
			}
		},
	}
}

// GetTimeSlots возвращает временные слоты места разгрузки.
func (s *unloadPlaceService) GetTimeSlots(ctx context.Context, placeID int) ([]models.UnloadPlaceTimeSlot, error) {
	return s.timeSlots().list(ctx, placeID)
}

// AddTimeSlot добавляет временной слот к месту разгрузки.
func (s *unloadPlaceService) AddTimeSlot(ctx context.Context, placeID int, req CreateTimeSlotRequest) (int, error) {
	return s.timeSlots().add(ctx, placeID, models.CreateTimeSlotRequest(req))
}

// UpdateTimeSlot обновляет временной слот места разгрузки.
func (s *unloadPlaceService) UpdateTimeSlot(ctx context.Context, placeID, slotID int, req UpdateTimeSlotRequest) error {
	return s.timeSlots().update(ctx, placeID, slotID, models.UpdateTimeSlotRequest(req))
}

// DeleteTimeSlot удаляет временной слот места разгрузки.
func (s *unloadPlaceService) DeleteTimeSlot(ctx context.Context, placeID, slotID int) error {
	return s.timeSlots().remove(ctx, placeID, slotID)
}
