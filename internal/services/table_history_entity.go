package services

import (
	"context"
	"database/sql"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"net/http"
	"systemburo/internal/models"
	"time"
)

// History reads share table visibility and current bindings, but do not grant
// admission or extend the active-list retention window.
func authorizeTableHistoryRead(ctx context.Context, tx *gorm.DB, actor, tableID int, kind ElementKind) error {
	set, err := NewPermissionResolver(tx).Resolve(ctx, actor)
	if err != nil {
		return err
	}
	var table models.SystemTable
	err = tx.Select("id,name,table_type,is_active,status").First(&table, tableID).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	expected := models.TableTypePeople
	if kind == ElementCar {
		expected = models.TableTypeCars
	}
	if err != nil || set.IsBanned() || !table.IsActive || table.Status != "active" || table.TableType != expected || !set.Has("table."+table.Name+".view") {
		return echo.NewHTTPError(http.StatusForbidden, "Нет доступа к таблице")
	}
	return nil
}
func historyEntityNotFound() error {
	return echo.NewHTTPError(http.StatusNotFound, "Запись удалена или недоступна для просмотра")
}

func (s *carService) GetHistoryCarForTable(ctx context.Context, actor, tableID, entityID int) (*TableCarResponse, error) {
	var out *TableCarResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := authorizeTableHistoryRead(ctx, tx, actor, tableID, ElementCar); err != nil {
			return err
		}
		reader := *s
		reader.db = tx
		now := time.Now().UTC()
		var rows []tableCarRow
		if err := reader.tableCarsBase(ctx, &tableID, now, entityID).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) != 1 {
			return historyEntityNotFound()
		}
		cards, err := reader.enrichTableCars(ctx, rows, now)
		if err != nil {
			return err
		}
		// This is a read-only history card, never an entry/exit command target.
		cards[0].Admission = PassageAdmission{}
		if err := tx.Table("car_unload_places").Where("car_id = ?", entityID).Order("order_index").Pluck("unload_place_id", &cards[0].UnloadPlaceIDs).Error; err != nil {
			return err
		}
		out = &cards[0]
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return out, err
}
func (s *employeeService) GetHistoryEmployeeForTable(ctx context.Context, actor, tableID, entityID int) (*TableEmployeeResponse, error) {
	var out *TableEmployeeResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := authorizeTableHistoryRead(ctx, tx, actor, tableID, ElementEmployee); err != nil {
			return err
		}
		reader := *s
		reader.db = tx
		cards, err := reader.getActiveEmployeesForTableSnapshot(ctx, tableID, time.Now().UTC(), entityID)
		if err != nil {
			return err
		}
		if len(cards) != 1 {
			return historyEntityNotFound()
		}
		cards[0].Admission = PassageAdmission{}
		out = &cards[0]
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return out, err
}
