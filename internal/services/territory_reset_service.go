package services

import (
	"context"
	"fmt"
	"log/slog"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// TerritoryResetService сбрасывает территориальные статусы сотрудников и машин.
type TerritoryResetService interface {
	// ResetExitedStatuses переводит всех со статусом "Покинул/Выехал" (2) на "Не входил/Не въезжал" (0).
	// Статус "На территории" (1) не затрагивается. История не пишется.
	ResetExitedStatuses(ctx context.Context) (employeesReset, carsReset int64, err error)
}

type territoryResetService struct {
	db *gorm.DB
}

func NewTerritoryResetService(db *gorm.DB) TerritoryResetService {
	return &territoryResetService{db: db}
}

func (s *territoryResetService) ResetExitedStatuses(ctx context.Context) (employeesReset, carsReset int64, err error) {
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Employee{}).Where("territory_status = ?", 2).Update("territory_status", 0)
		if res.Error != nil {
			return fmt.Errorf("сброс статусов сотрудников: %w", res.Error)
		}
		employeesReset = res.RowsAffected

		res = tx.Model(&models.Car{}).Where("territory_status = ?", 2).Update("territory_status", 0)
		if res.Error != nil {
			return fmt.Errorf("сброс статусов машин: %w", res.Error)
		}
		carsReset = res.RowsAffected

		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	slog.Info("ежедневный сброс территориальных статусов выполнен",
		"employees_reset", employeesReset,
		"cars_reset", carsReset,
	)
	return employeesReset, carsReset, nil
}
