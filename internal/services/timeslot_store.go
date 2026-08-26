package services

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// timeSlotModel -- контракт модели временного слота: общий стор читает ID после
// создания. Реализуют SystemTableTimeSlot и UnloadPlaceTimeSlot.
type timeSlotModel interface {
	GetID() int
}

// timeSlotStore -- generic-CRUD временных слотов поверх произвольной модели слота.
// Раньше эта логика была продублирована в system_table и unload_place сервисах;
// специфика таблицы (имя FK-колонки, проверка родителя, конструктор слота) задаётся
// полями, остальное общее. Для single-owner таблиц (расписание Бюро) fkColumn пуст:
// фильтрации по родителю нет, parentID игнорируется.
type timeSlotStore[T timeSlotModel] struct {
	db          *gorm.DB
	entity      string                                                 // для логов: "system_table" | "unload_place" | "bureau"
	fkColumn    string                                                 // "table_id" | "unload_place_id"; "" для single-owner
	checkParent func(ctx context.Context, parentID int) error          // существование родителя + текст 404
	newSlot     func(parentID int, req models.CreateTimeSlotRequest) T // конструктор слота с FK и полями
}

// scopeParent ограничивает запрос родителем для parent-keyed стора. Для
// single-owner стора (fkColumn == "") фильтр не добавляется -- таблица
// обслуживает единственного владельца, parentID не используется.
func (st timeSlotStore[T]) scopeParent(q *gorm.DB, parentID int) *gorm.DB {
	if st.fkColumn == "" {
		return q
	}
	return q.Where(st.fkColumn+" = ?", parentID)
}

// list возвращает слоты родителя, отсортированные по дню и времени открытия.
func (st timeSlotStore[T]) list(ctx context.Context, parentID int) ([]T, error) {
	slots := make([]T, 0)
	if err := st.scopeParent(st.db.WithContext(ctx), parentID).
		Order("day_of_week, open_time").
		Find(&slots).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching time slots")
	}
	return slots, nil
}

// add валидирует и создаёт слот после проверки существования родителя.
func (st timeSlotStore[T]) add(ctx context.Context, parentID int, req models.CreateTimeSlotRequest) (int, error) {
	if err := st.checkParent(ctx, parentID); err != nil {
		return 0, err
	}
	if err := validateClock(req.OpenTime, "открытия"); err != nil {
		return 0, err
	}
	if err := validateClock(req.CloseTime, "закрытия"); err != nil {
		return 0, err
	}
	if err := validateDayOfWeek(req.DayOfWeek); err != nil {
		return 0, err
	}

	slot := st.newSlot(parentID, req)
	if err := st.db.WithContext(ctx).Create(&slot).Error; err != nil {
		slog.Error("не удалось добавить временной слот", "entity", st.entity, "parent_id", parentID, "error", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error adding time slot")
	}
	slog.Info("временной слот добавлен", "entity", st.entity, "id", slot.GetID(), "parent_id", parentID)
	return slot.GetID(), nil
}

// update применяет переданные поля к существующему слоту (404, если слота нет).
func (st timeSlotStore[T]) update(ctx context.Context, parentID, slotID int, req models.UpdateTimeSlotRequest) error {
	var existing T
	if err := st.scopeParent(st.db.WithContext(ctx).Where("id = ?", slotID), parentID).
		First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errTimeSlotNotFound()
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching time slot")
	}

	// updated_at всегда в наборе -> RowsAffected отражает реальное наличие строки,
	// а не "значения не изменились".
	updates := map[string]interface{}{"updated_at": time.Now().UTC()}
	if req.DayOfWeek != nil {
		if err := validateDayOfWeek(*req.DayOfWeek); err != nil {
			return err
		}
		updates["day_of_week"] = *req.DayOfWeek
	}
	if req.OpenTime != nil {
		if err := validateClock(*req.OpenTime, "открытия"); err != nil {
			return err
		}
		updates["open_time"] = *req.OpenTime
	}
	if req.CloseTime != nil {
		if err := validateClock(*req.CloseTime, "закрытия"); err != nil {
			return err
		}
		updates["close_time"] = *req.CloseTime
	}
	if req.IsNextDay != nil {
		updates["is_next_day"] = *req.IsNextDay
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	result := st.scopeParent(st.db.WithContext(ctx).Model(new(T)).Where("id = ?", slotID), parentID).
		Updates(updates)
	if result.Error != nil {
		slog.Error("не удалось обновить временной слот", "entity", st.entity, "slot_id", slotID, "parent_id", parentID, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating time slot")
	}
	if result.RowsAffected == 0 {
		return errTimeSlotNotFound()
	}
	slog.Info("временной слот обновлён", "entity", st.entity, "slot_id", slotID, "parent_id", parentID)
	return nil
}

// remove удаляет слот родителя (404, если слота нет).
func (st timeSlotStore[T]) remove(ctx context.Context, parentID, slotID int) error {
	result := st.scopeParent(st.db.WithContext(ctx).Where("id = ?", slotID), parentID).
		Delete(new(T))
	if result.Error != nil {
		slog.Error("не удалось удалить временной слот", "entity", st.entity, "slot_id", slotID, "parent_id", parentID, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting time slot")
	}
	if result.RowsAffected == 0 {
		return errTimeSlotNotFound()
	}
	slog.Info("временной слот удалён", "entity", st.entity, "slot_id", slotID, "parent_id", parentID)
	return nil
}

// errTimeSlotNotFound -- 404 для отсутствующего слота. Функция, а не общая
// переменная: echo может проставлять Internal на *HTTPError, делиться экземпляром нельзя.
func errTimeSlotNotFound() error {
	return echo.NewHTTPError(http.StatusNotFound, "Временной слот не найден")
}

// validateClock проверяет формат времени ЧЧ:ММ. label -- "открытия"/"закрытия".
func validateClock(value, label string) error {
	if _, err := time.Parse("15:04", value); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Неверный формат времени "+label+". Используйте ЧЧ:ММ")
	}
	return nil
}

// validateDayOfWeek проверяет день недели в диапазоне 0 (Пн) -- 6 (Вс).
func validateDayOfWeek(day int) error {
	if day < 0 || day > 6 {
		return echo.NewHTTPError(http.StatusBadRequest, "День недели должен быть от 0 (Пн) до 6 (Вс)")
	}
	return nil
}
