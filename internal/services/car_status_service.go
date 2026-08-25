package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// GetCarsCurrentStatus возвращает текущий территориальный статус активных автомобилей.
func (s *carService) GetCarsCurrentStatus(ctx context.Context) ([]CarCurrentStatus, error) {
	type statusRow struct {
		ID                 int
		TerritoryStatus    *int
		TerritoryEntryTime *time.Time
		LastExitTime       *time.Time
	}

	rows := make([]statusRow, 0)
	err := s.db.WithContext(ctx).Raw(`
		SELECT
			c.id,
			c.territory_status,
			c.territory_entry_time,
			(
				SELECT created_at
				FROM ` + carsHistoryUnion + ` ch
				WHERE car_id = c.id AND action_type = 'exit'
				ORDER BY created_at DESC
				LIMIT 1
			) AS last_exit_time
		FROM cars c
		WHERE c.status = 1
	`).Scan(&rows).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error fetching cars status")
	}

	items := make([]CarCurrentStatus, 0, len(rows))
	for _, r := range rows {
		ts := 0
		if r.TerritoryStatus != nil {
			ts = *r.TerritoryStatus
		}
		items = append(items, CarCurrentStatus{
			CarID:           r.ID,
			TerritoryStatus: ts,
			EntryTime:       FormatUTCPtr(r.TerritoryEntryTime),
			LastExitTime:    FormatUTCPtr(r.LastExitTime),
		})
	}
	return items, nil
}

// UpdateCarTerritoryStatus обновляет территориальный статус автомобиля (въезд/выезд).
func (s *carService) UpdateCarTerritoryStatus(ctx context.Context, carID int, req UpdateCarTerritoryStatusRequest) error {
	now := time.Now().UTC()
	actionType := "unknown"
	if req.TerritoryStatus == 1 {
		actionType = "entry"
	} else if req.TerritoryStatus == 2 {
		actionType = "exit"
	}

	var car models.Car
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Select("id", "car_number", "car_brand", "territory_status", "attachment_id").
			First(&car, carID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return echo.NewHTTPError(http.StatusNotFound, "Car not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}

		updates := map[string]interface{}{
			"territory_status": req.TerritoryStatus,
			"updated_at":       now,
		}
		if req.TerritoryStatus == 1 {
			updates["territory_entry_time"] = now
		}
		if err := tx.Model(&models.Car{}).Where("id = ?", carID).Updates(updates).Error; err != nil {
			slog.Error("не удалось обновить территориальный статус автомобиля", "car_id", carID, "status", req.TerritoryStatus, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error updating car territory status")
		}

		carNumber := ""
		if car.CarNumber != nil {
			carNumber = *car.CarNumber
		}
		var comment string
		if req.TerritoryStatus == 1 {
			comment = fmt.Sprintf("Автомобиль %s въехал на территорию", carNumber)
		} else if req.TerritoryStatus == 2 {
			comment = fmt.Sprintf("Автомобиль %s выехал с территории", carNumber)
		}

		details := carAuditDetails{Comment: &comment, TableID: req.TableID}
		// Данные пропуска "по факту" (#1132): при въезде и наличии введённого номера
		// кладём снимок в details.metadata записи entry -> он доедет до карточки через
		// carsHistoryUnion (details->'metadata'). Выезд/пустой номер снимок не пишут.
		if req.TerritoryStatus == 1 && req.Pass != nil && strings.TrimSpace(req.Pass.Number) != "" {
			if raw, err := json.Marshal(req.Pass); err == nil {
				details.Metadata = raw
			} else {
				slog.Error("не удалось сериализовать данные пропуска по факту", "car_id", carID, "error", err)
			}
		}

		if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &carID, actionType, req.UserID, details); err != nil {
			slog.Error("не удалось добавить запись в историю автомобиля", "car_id", carID, "action_type", actionType, "error", err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
		}
		slog.Info("территориальный статус автомобиля обновлён", "car_id", carID, "action_type", actionType, "status", req.TerritoryStatus)
		return nil
	})
	if err != nil {
		return err
	}

	// Въезд/выезд изменил строку машины - сигналим аудитории её таблиц «Проезд»
	// обновиться live (#840 V2.3, scoped #1036).
	s.tablesProducer.NotifyCarsChanged(ctx, carID)

	return nil
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
