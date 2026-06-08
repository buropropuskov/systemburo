package services

import (
	"context"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// timeSlots -- общий стор временных слотов для системных таблиц. Слот можно
// добавить только к активной таблице.
func (s *systemTableService) timeSlots() timeSlotStore[models.SystemTableTimeSlot] {
	return timeSlotStore[models.SystemTableTimeSlot]{
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
		newSlot: func(tableID int, req models.CreateTimeSlotRequest) models.SystemTableTimeSlot {
			return models.SystemTableTimeSlot{
				TableID:   tableID,
				DayOfWeek: req.DayOfWeek,
				OpenTime:  req.OpenTime,
				CloseTime: req.CloseTime,
				IsNextDay: req.IsNextDay != nil && *req.IsNextDay,
				IsActive:  req.IsActive == nil || *req.IsActive,
			}
		},
	}
}

// GetTimeSlots возвращает временные слоты системной таблицы.
func (s *systemTableService) GetTimeSlots(ctx context.Context, tableID int) ([]models.SystemTableTimeSlot, error) {
	return s.timeSlots().list(ctx, tableID)
}

// AddTimeSlot добавляет временной слот к системной таблице.
func (s *systemTableService) AddTimeSlot(ctx context.Context, tableID int, req models.CreateTimeSlotRequest) (int, error) {
	return s.timeSlots().add(ctx, tableID, req)
}

// UpdateTimeSlot обновляет временной слот системной таблицы.
func (s *systemTableService) UpdateTimeSlot(ctx context.Context, tableID, slotID int, req models.UpdateTimeSlotRequest) error {
	return s.timeSlots().update(ctx, tableID, slotID, req)
}

// DeleteTimeSlot удаляет временной слот системной таблицы.
func (s *systemTableService) DeleteTimeSlot(ctx context.Context, tableID, slotID int) error {
	return s.timeSlots().remove(ctx, tableID, slotID)
}
