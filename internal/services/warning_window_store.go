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

// warningWindowModel -- контракт модели окна-предупреждения: общий стор читает ID
// после создания. Реализуют UnloadPlaceWarningWindow и SystemTableWarningWindow.
type warningWindowModel interface {
	GetID() int
}

// warningWindowStore -- generic-CRUD предупреждений по временным окнам поверх
// произвольной модели окна. Отдельный от timeSlotStore, а не его переиспользование:
// у окна другая схема (nullable day_of_week/time_from/time_to + обязательный
// message), а update перезаписывает запись целиком (см. update). Специфика
// сущности (FK-колонка, проверка родителя, конструктор окна) задаётся полями.
type warningWindowStore[T warningWindowModel] struct {
	db          *gorm.DB
	entity      string                                                // для логов: "unload_place" / "system_table"
	fkColumn    string                                                // "unload_place_id" / "table_id"
	checkParent func(ctx context.Context, parentID int) error         // существование родителя + текст 404
	newWindow   func(parentID int, req models.WarningWindowRequest) T // конструктор окна с FK и полями
}

// scopeParent ограничивает запрос родителем.
func (st warningWindowStore[T]) scopeParent(q *gorm.DB, parentID int) *gorm.DB {
	return q.Where(st.fkColumn+" = ?", parentID)
}

// list возвращает окна родителя: сперва общие (день/время NULL), затем по дню и
// времени начала -- в том же порядке, в каком их удобно показать в редакторе.
func (st warningWindowStore[T]) list(ctx context.Context, parentID int) ([]T, error) {
	windows := make([]T, 0)
	if err := st.scopeParent(st.db.WithContext(ctx), parentID).
		Order("day_of_week NULLS FIRST, time_from NULLS FIRST").
		Find(&windows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching warning windows")
	}
	return windows, nil
}

// add валидирует и создаёт окно после проверки существования родителя.
func (st warningWindowStore[T]) add(ctx context.Context, parentID int, req models.WarningWindowRequest) (int, error) {
	if err := st.checkParent(ctx, parentID); err != nil {
		return 0, err
	}
	if err := validateWarningWindow(req); err != nil {
		return 0, err
	}

	window := st.newWindow(parentID, req)
	if err := st.db.WithContext(ctx).Create(&window).Error; err != nil {
		slog.Error("не удалось добавить предупреждение по окну", "entity", st.entity, "parent_id", parentID, "error", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Error adding warning window")
	}
	slog.Info("предупреждение по окну добавлено", "entity", st.entity, "id", window.GetID(), "parent_id", parentID)
	return window.GetID(), nil
}

// update перезаписывает окно целиком (PUT-семантика). Nullable-поля день/время
// пишутся значением из запроса, включая NULL: у окна это переключатели
// "каждый день / по дню" и "весь день / по времени", partial-update по указателю
// не отличил бы "не трогать" от "сбросить", поэтому редактор шлёт запись целиком.
func (st warningWindowStore[T]) update(ctx context.Context, parentID, windowID int, req models.WarningWindowRequest) error {
	var existing T
	if err := st.scopeParent(st.db.WithContext(ctx).Where("id = ?", windowID), parentID).
		First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errWarningWindowNotFound()
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching warning window")
	}
	if err := validateWarningWindow(req); err != nil {
		return err
	}

	// Указатели day/time кладутся как есть: GORM пишет NULL для nil-указателя и
	// значение для ненулевого -- так окно можно вернуть в "каждый день"/"весь день".
	updates := map[string]interface{}{
		"updated_at":  time.Now().UTC(),
		"day_of_week": req.DayOfWeek,
		"time_from":   req.TimeFrom,
		"time_to":     req.TimeTo,
		"message":     req.Message,
		"is_next_day": req.IsNextDay != nil && *req.IsNextDay,
		"is_active":   req.IsActive == nil || *req.IsActive,
	}

	result := st.scopeParent(st.db.WithContext(ctx).Model(new(T)).Where("id = ?", windowID), parentID).
		Updates(updates)
	if result.Error != nil {
		slog.Error("не удалось обновить предупреждение по окну", "entity", st.entity, "window_id", windowID, "parent_id", parentID, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error updating warning window")
	}
	if result.RowsAffected == 0 {
		return errWarningWindowNotFound()
	}
	slog.Info("предупреждение по окну обновлено", "entity", st.entity, "window_id", windowID, "parent_id", parentID)
	return nil
}

// remove удаляет окно родителя (404, если окна нет).
func (st warningWindowStore[T]) remove(ctx context.Context, parentID, windowID int) error {
	result := st.scopeParent(st.db.WithContext(ctx).Where("id = ?", windowID), parentID).
		Delete(new(T))
	if result.Error != nil {
		slog.Error("не удалось удалить предупреждение по окну", "entity", st.entity, "window_id", windowID, "parent_id", parentID, "error", result.Error)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting warning window")
	}
	if result.RowsAffected == 0 {
		return errWarningWindowNotFound()
	}
	slog.Info("предупреждение по окну удалено", "entity", st.entity, "window_id", windowID, "parent_id", parentID)
	return nil
}

// validateWarningWindow проверяет опциональные поля окна: день недели и формат
// времени -- только когда они заданы (nil = "каждый день"/"весь день", пропуск).
// Хелперы validateClock/validateDayOfWeek общие с timeSlotStore.
func validateWarningWindow(req models.WarningWindowRequest) error {
	if req.DayOfWeek != nil {
		if err := validateDayOfWeek(*req.DayOfWeek); err != nil {
			return err
		}
	}
	if req.TimeFrom != nil {
		if err := validateClock(*req.TimeFrom, "начала окна"); err != nil {
			return err
		}
	}
	if req.TimeTo != nil {
		if err := validateClock(*req.TimeTo, "конца окна"); err != nil {
			return err
		}
	}
	return nil
}

// errWarningWindowNotFound -- 404 для отсутствующего окна. Функция, а не общая
// переменная: echo может проставлять Internal на *HTTPError, делиться экземпляром нельзя.
func errWarningWindowNotFound() error {
	return echo.NewHTTPError(http.StatusNotFound, "Предупреждение по окну не найдено")
}
