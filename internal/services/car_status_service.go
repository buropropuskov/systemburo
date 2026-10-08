package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// GetCarsCurrentStatus возвращает текущий территориальный статус активных автомобилей.
func (s *carService) GetCarsCurrentStatus(ctx context.Context, viewerID int, scope ElementScope) ([]CarCurrentStatus, error) {
	rows, err := loadPassageCurrent(ctx, s.db, viewerID, ElementCar, scope)
	if err != nil {
		return nil, err
	}
	out := make([]CarCurrentStatus, 0, len(rows))
	for _, r := range rows {
		out = append(out, CarCurrentStatus{CarID: r.EntityID, TerritoryStatus: r.TerritoryStatus, EntryTime: r.EntryTime, LastExitTime: r.LastExitTime, CanRevert: r.CanRevert, LastMarkTableID: r.LastMarkTableID, PassageState: r.PassageState, EffectivePeriod: r.EffectivePeriod, ServerNow: r.ServerNow})
	}
	return out, nil
}

// UpdateCarTerritoryStatus обновляет территориальный статус автомобиля (въезд/выезд).
func (s *carService) UpdateCarTerritoryStatus(ctx context.Context, carID int, req UpdateCarTerritoryStatusRequest) error {
	actor := 0
	if req.UserID != nil {
		actor = *req.UserID
	}
	commands := NewPassageCommandService(s.db, s.recorder)
	commands.SetAfterChange(func(ctx context.Context, _ ElementKind, id int) { s.tablesProducer.NotifyCarsChanged(ctx, id) })
	_, err := commands.Mark(ctx, actor, ElementCar, carID, PassageCommandRequest{TableID: req.TableID, ExpectedLastEventID: req.ExpectedLastEventID, TerritoryStatus: req.TerritoryStatus, Pass: req.Pass})
	return err
}

// RevertCarPassage отменяет последнюю отметку проезда машины и откатывает её
// территориальный статус (#2437). Сигнал таблицам шлём тем же способом, что и при
// самой отметке: строка изменилась, и посты обязаны увидеть это без перезагрузки.
func (s *carService) RevertCarPassage(ctx context.Context, carID int, req RevertPassageRequest) error {
	commands := NewPassageCommandService(s.db, s.recorder)
	commands.SetAfterChange(func(ctx context.Context, _ ElementKind, id int) { s.tablesProducer.NotifyCarsChanged(ctx, id) })
	_, err := commands.Revert(ctx, req.ActorUserID, ElementCar, carID, PassageCommandRequest{TableID: req.TableID, ExpectedLastEventID: req.ExpectedLastEventID, TerritoryStatus: req.TerritoryStatus, Reason: req.Reason})
	return err
}

// DeactivateCar деактивирует автомобиль и записывает удаление в историю.
func (s *carService) DeactivateCar(ctx context.Context, carID int, req DeactivateCarRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.deactivateCarTx(ctx, tx, carID, req)
	})
}

// deactivateCarTx выполняет саму деактивацию машины внутри уже открытой транзакции.
// Вынесено из DeactivateCar для переиспользования bulk-операциями (#1194): когда
// снятие/перенос последней привязки к таблице «Проезд» оставляет машину без единой
// таблицы, она деактивируется тем же путём, что и единичный DeactivateCar, но в той
// же tx, что и сама привязка (без вложенной транзакции).
func (s *carService) deactivateCarTx(ctx context.Context, tx *gorm.DB, carID int, req DeactivateCarRequest) error {
	var car models.Car
	if err := tx.Select("id", "car_number", "car_brand").
		First(&car, carID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Car not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	now := time.Now().UTC()
	if err := tx.Model(&models.Car{}).Where("id = ?", carID).Updates(map[string]interface{}{
		"status":       req.Status,
		"date_removed": now,
		"updated_at":   now,
	}).Error; err != nil {
		slog.Error("не удалось деактивировать автомобиль", "car_id", carID, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deactivating car")
	}

	carNumber := ""
	carBrand := ""
	if car.CarNumber != nil {
		carNumber = *car.CarNumber
	}
	if car.CarBrand != nil {
		carBrand = *car.CarBrand
	}
	comment := fmt.Sprintf("Автомобиль %s %s удалён пользователем", carNumber, carBrand)
	actionType := "delete"
	if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &carID, actionType, req.UserID, carAuditDetails{Comment: &comment, TableID: req.TableID}); err != nil {
		slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", actionType, "error", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
	}
	slog.Info("автомобиль деактивирован", "car_id", carID)
	return nil
}

// ActivateCar вводит автомобиль в работу и записывает активацию в историю.
func (s *carService) ActivateCar(ctx context.Context, carID int, req ActivateCarRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var car models.Car
		if err := tx.Select("id", "car_number", "car_brand").
			First(&car, carID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Car not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		now := time.Now().UTC()
		if err := tx.Model(&models.Car{}).Where("id = ?", carID).Updates(map[string]interface{}{
			"status":       1,
			"date_removed": nil,
			"updated_at":   now,
		}).Error; err != nil {
			slog.Error("не удалось активировать автомобиль", "car_id", carID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error activating car")
		}

		carNumber := ""
		carBrand := ""
		if car.CarNumber != nil {
			carNumber = *car.CarNumber
		}
		if car.CarBrand != nil {
			carBrand = *car.CarBrand
		}
		comment := fmt.Sprintf("Автомобиль %s %s введён в работу", carNumber, carBrand)
		actionType := "activate"
		if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &carID, actionType, req.UserID, carAuditDetails{Comment: &comment}); err != nil {
			slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
		}
		slog.Info("автомобиль активирован", "car_id", carID)
		return nil
	})
}

// RestoreCar восстанавливает удалённый автомобиль и записывает восстановление в историю.
func (s *carService) RestoreCar(ctx context.Context, carID int, req RestoreCarRequest) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var car models.Car
		if err := tx.Select("id", "car_number", "car_brand").
			First(&car, carID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Car not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		now := time.Now().UTC()
		if err := tx.Model(&models.Car{}).Where("id = ?", carID).Updates(map[string]interface{}{
			"status":       1,
			"date_removed": nil,
			"updated_at":   now,
		}).Error; err != nil {
			slog.Error("не удалось восстановить автомобиль", "car_id", carID, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error restoring car")
		}

		carNumber := ""
		carBrand := ""
		if car.CarNumber != nil {
			carNumber = *car.CarNumber
		}
		if car.CarBrand != nil {
			carBrand = *car.CarBrand
		}
		comment := fmt.Sprintf("Автомобиль %s %s восстановлен", carNumber, carBrand)
		actionType := "restore"
		if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &carID, actionType, req.UserID, carAuditDetails{Comment: &comment}); err != nil {
			slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
		}
		slog.Info("автомобиль восстановлен", "car_id", carID)
		return nil
	})
}
