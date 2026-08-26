package services

import (
	"context"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// warningWindows -- общий стор предупреждений по окнам для системных таблиц. Как и
// у time-slots, окно можно добавить только к активной таблице (checkParent требует
// is_active). Переиспользует generic-стор из S2a (см. warning_window_store.go).
func (s *systemTableService) warningWindows() warningWindowStore[models.SystemTableWarningWindow] {
	return warningWindowStore[models.SystemTableWarningWindow]{
		db:       s.db,
		entity:   "system_table",
		fkColumn: "table_id",
		checkParent: func(ctx context.Context, tableID int) error {
			var count int64
			if err := s.db.WithContext(ctx).Model(&models.SystemTable{}).
				Where("id = ? AND is_active = ?", tableID, true).
				Count(&count).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error checking system table")
			}
			if count == 0 {
				return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
			}
			return nil
		},
		newWindow: func(tableID int, req models.WarningWindowRequest) models.SystemTableWarningWindow {
			return models.SystemTableWarningWindow{
				TableID:   tableID,
				DayOfWeek: req.DayOfWeek,
				TimeFrom:  req.TimeFrom,
				TimeTo:    req.TimeTo,
				IsNextDay: req.IsNextDay != nil && *req.IsNextDay,
				Message:   req.Message,
				IsActive:  req.IsActive == nil || *req.IsActive,
			}
		},
	}
}

// GetWarningWindows возвращает предупреждения по окнам системной таблицы.
func (s *systemTableService) GetWarningWindows(ctx context.Context, tableID int) ([]models.SystemTableWarningWindow, error) {
	return s.warningWindows().list(ctx, tableID)
}

// AddWarningWindow добавляет предупреждение по окну к системной таблице.
func (s *systemTableService) AddWarningWindow(ctx context.Context, tableID int, req models.WarningWindowRequest) (int, error) {
	return s.warningWindows().add(ctx, tableID, req)
}

// UpdateWarningWindow обновляет предупреждение по окну системной таблицы.
func (s *systemTableService) UpdateWarningWindow(ctx context.Context, tableID, windowID int, req models.WarningWindowRequest) error {
	return s.warningWindows().update(ctx, tableID, windowID, req)
}

// DeleteWarningWindow удаляет предупреждение по окну системной таблицы.
func (s *systemTableService) DeleteWarningWindow(ctx context.Context, tableID, windowID int) error {
	return s.warningWindows().remove(ctx, tableID, windowID)
}
