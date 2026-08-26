package services

import (
	"context"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// warningWindows -- общий стор предупреждений по окнам для мест разгрузки. Как и
// у time-slots, окно можно добавить к архивному месту (is_active не проверяется --
// сохранение настроек предупреждений при архивации).
func (s *unloadPlaceService) warningWindows() warningWindowStore[models.UnloadPlaceWarningWindow] {
	return warningWindowStore[models.UnloadPlaceWarningWindow]{
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
		newWindow: func(placeID int, req models.WarningWindowRequest) models.UnloadPlaceWarningWindow {
			return models.UnloadPlaceWarningWindow{
				UnloadPlaceID: placeID,
				DayOfWeek:     req.DayOfWeek,
				TimeFrom:      req.TimeFrom,
				TimeTo:        req.TimeTo,
				IsNextDay:     req.IsNextDay != nil && *req.IsNextDay,
				Message:       req.Message,
				IsActive:      req.IsActive == nil || *req.IsActive,
			}
		},
	}
}

// GetWarningWindows возвращает предупреждения по окнам места разгрузки.
func (s *unloadPlaceService) GetWarningWindows(ctx context.Context, placeID int) ([]models.UnloadPlaceWarningWindow, error) {
	return s.warningWindows().list(ctx, placeID)
}

// AddWarningWindow добавляет предупреждение по окну к месту разгрузки.
func (s *unloadPlaceService) AddWarningWindow(ctx context.Context, placeID int, req models.WarningWindowRequest) (int, error) {
	return s.warningWindows().add(ctx, placeID, req)
}

// UpdateWarningWindow обновляет предупреждение по окну места разгрузки.
func (s *unloadPlaceService) UpdateWarningWindow(ctx context.Context, placeID, windowID int, req models.WarningWindowRequest) error {
	return s.warningWindows().update(ctx, placeID, windowID, req)
}

// DeleteWarningWindow удаляет предупреждение по окну места разгрузки.
func (s *unloadPlaceService) DeleteWarningWindow(ctx context.Context, placeID, windowID int) error {
	return s.warningWindows().remove(ctx, placeID, windowID)
}
