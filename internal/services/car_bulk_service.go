package services

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// --- DTO групповых операций над car_target_tables (#1194, S1) ---

// BulkMoveCarsTableRequest -- тело POST /cars/bulk/move-table: снимает у набора
// машин привязку к FromTableID и заменяет её ToTableIDs. Прочие привязки машины
// (к таблицам, не равным FromTableID) не трогает.
type BulkMoveCarsTableRequest struct {
	IDs         []int `json:"ids"`
	FromTableID int   `json:"from_table_id"`
	ToTableIDs  []int `json:"to_table_ids"`
}

// BulkAddCarsTableRequest -- тело POST /cars/bulk/add-table: добавляет набор машин
// в TableIDs, объединяя с текущими привязками (существующие не снимаются).
type BulkAddCarsTableRequest struct {
	IDs      []int `json:"ids"`
	TableIDs []int `json:"table_ids"`
}

// BulkUnbindCarsTableRequest -- тело POST /cars/bulk/unbind-table: снимает у
// набора машин привязку к TableID. Если это последняя привязка машины - машина
// деактивируется (как единичный DeactivateCar).
type BulkUnbindCarsTableRequest struct {
	IDs     []int `json:"ids"`
	TableID int   `json:"table_id"`
}

// BulkMoveTable переносит набор машин из одной таблицы «Проезд» в другие (#1194).
// Валидация состава таблиц (существование + table_type=cars) - структурная, на весь
// запрос сразу (400 при провале), а не partial-ошибка per-машина: некорректный
// table_id - проблема запроса, а не отдельной машины.
func (s *carService) BulkMoveTable(ctx context.Context, req BulkMoveCarsTableRequest, actorID int) (*BulkOpResult, error) {
	if req.FromTableID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не указана исходная таблица")
	}
	allTableIDs := append([]int{req.FromTableID}, req.ToTableIDs...)
	if err := s.validateCarsTargetTables(ctx, allTableIDs); err != nil {
		return nil, err
	}
	tableNames, err := s.systemTableNames(ctx, allTableIDs)
	if err != nil {
		return nil, err
	}
	toIDs := uniqueInts(req.ToTableIDs)
	carNumbers := s.carNumbersByIDs(ctx, req.IDs)

	res := newBulkResult()
	for _, carID := range uniqueInts(req.IDs) {
		if err := s.moveCarTable(ctx, carID, req.FromTableID, toIDs, tableNames, &actorID); err != nil {
			res.addError(carID, carNumbers[carID], bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}

	s.tablesProducer.NotifyTables(ctx, allTableIDs)
	s.enqueueArchiveExportForCars(ctx, req.IDs)
	return res.finalize(), nil
}

// BulkAddTable добавляет набор машин в дополнительные таблицы «Проезд» (#1194):
// объединение с текущими привязками, существующие не снимаются.
func (s *carService) BulkAddTable(ctx context.Context, req BulkAddCarsTableRequest, actorID int) (*BulkOpResult, error) {
	if err := s.validateCarsTargetTables(ctx, req.TableIDs); err != nil {
		return nil, err
	}
	tableIDs := uniqueInts(req.TableIDs)
	carNumbers := s.carNumbersByIDs(ctx, req.IDs)

	res := newBulkResult()
	for _, carID := range uniqueInts(req.IDs) {
		if err := s.addCarTables(ctx, carID, tableIDs, &actorID); err != nil {
			res.addError(carID, carNumbers[carID], bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}

	s.tablesProducer.NotifyTables(ctx, tableIDs)
	s.enqueueArchiveExportForCars(ctx, req.IDs)
	return res.finalize(), nil
}

// BulkUnbindTable снимает привязку набора машин к одной таблице «Проезд» (#1194).
// Пустой итоговый набор целевых таблиц машины -> деактивация (как единичный
// DeactivateCar).
func (s *carService) BulkUnbindTable(ctx context.Context, req BulkUnbindCarsTableRequest, actorID int) (*BulkOpResult, error) {
	if req.TableID <= 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не указана таблица")
	}
	if err := s.validateCarsTargetTables(ctx, []int{req.TableID}); err != nil {
		return nil, err
	}
	carNumbers := s.carNumbersByIDs(ctx, req.IDs)

	res := newBulkResult()
	for _, carID := range uniqueInts(req.IDs) {
		if err := s.unbindCarTable(ctx, carID, req.TableID, &actorID); err != nil {
			res.addError(carID, carNumbers[carID], bulkErrMsg(err))
			continue
		}
		res.SuccessCount++
	}

	s.tablesProducer.NotifyTables(ctx, []int{req.TableID})
	s.enqueueArchiveExportForCars(ctx, req.IDs)
	return res.finalize(), nil
}

// enqueueArchiveExportForCars резолвит машины в заявки через их вложение и
// ставит заявки в очередь на пересборку файлового архива (#1615, B1): слепок
// заявки хранит посты «Проезд» каждой машины, а bulk-операции меняют их в обход
// application_assignment_service, у которого свой enqueue после commit.
func (s *carService) enqueueArchiveExportForCars(ctx context.Context, carIDs []int) {
	if s.blankExports == nil {
		return
	}
	unique := uniqueInts(carIDs)
	if len(unique) == 0 {
		return
	}
	var appIDs []int
	err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT a.application_id
		FROM cars c
		JOIN attachments a ON a.id = c.attachment_id
		WHERE c.id IN ? AND a.application_id IS NOT NULL
	`, unique).Scan(&appIDs).Error
	if err != nil {
		slog.Warn("не удалось резолвить заявки для пересборки архива после bulk-операции с машинами", "error", err)
		return
	}
	s.blankExports.EnqueueApplications(appIDs, BlankExportReasonUpdate)
}

// moveCarTable - одна машина операции BulkMoveTable, своя транзакция (партиционирует
// ошибку, чтобы одна проблемная машина не роняла весь пакет).
func (s *carService) moveCarTable(ctx context.Context, carID, fromTableID int, toIDs []int, tableNames map[int]string, actorID *int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		car, err := loadActiveCarForBulk(tx, carID)
		if err != nil {
			return err
		}

		del := tx.Where("car_id = ? AND table_id = ?", carID, fromTableID).Delete(&models.CarTargetTable{})
		if del.Error != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error unbinding car table")
		}
		if del.RowsAffected == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Машина не привязана к исходной таблице")
		}

		remainingSet, err := bindCarTables(tx, carID, toIDs)
		if err != nil {
			return err
		}

		carNumber := ""
		if car.CarNumber != nil {
			carNumber = *car.CarNumber
		}
		comment := fmt.Sprintf("Автомобиль %s перенесён из таблицы «%s» в «%s»",
			carNumber, tableNames[fromTableID], joinTableNames(toIDs, tableNames))
		if err := s.recorder.Record(ctx, tx, models.AuditEntityCar, &carID, models.AuditActionMovedBetweenTables, actorID,
			carAuditDetails{Comment: &comment, TableID: &fromTableID}); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car history entry")
		}

		if len(remainingSet) == 0 {
			if err := s.deactivateCarTx(ctx, tx, carID, DeactivateCarRequest{Status: 0, UserID: actorID}); err != nil {
				return err
			}
		}
		return nil
	})
}

// addCarTables - одна машина операции BulkAddTable, своя транзакция.
func (s *carService) addCarTables(ctx context.Context, carID int, tableIDs []int, actorID *int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := loadActiveCarForBulk(tx, carID); err != nil {
			return err
		}

		current, err := currentCarTableIDs(tx, carID)
		if err != nil {
			return err
		}
		currentSet := make(map[int]struct{}, len(current)+len(tableIDs))
		for _, id := range current {
			currentSet[id] = struct{}{}
		}
		for _, tableID := range tableIDs {
			if _, ok := currentSet[tableID]; ok {
				continue
			}
			if err := tx.Create(&models.CarTargetTable{CarID: carID, TableID: tableID, Source: "manual"}).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error binding car table")
			}
			currentSet[tableID] = struct{}{}
			if err := recordAddedToTable(ctx, s.recorder, tx, models.AuditEntityCar, carID, tableID, actorID); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car table history")
			}
		}
		return nil
	})
}

// unbindCarTable - одна машина операции BulkUnbindTable, своя транзакция.
func (s *carService) unbindCarTable(ctx context.Context, carID, tableID int, actorID *int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := loadActiveCarForBulk(tx, carID); err != nil {
			return err
		}

		del := tx.Where("car_id = ? AND table_id = ?", carID, tableID).Delete(&models.CarTargetTable{})
		if del.Error != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error unbinding car table")
		}
		if del.RowsAffected == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Машина не привязана к этой таблице")
		}

		if err := recordUnboundFromTable(ctx, s.recorder, tx, models.AuditEntityCar, carID, tableID, actorID); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error adding car table history")
		}

		var remaining int64
		if err := tx.Model(&models.CarTargetTable{}).Where("car_id = ?", carID).Count(&remaining).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error counting car tables")
		}
		if remaining == 0 {
			if err := s.deactivateCarTx(ctx, tx, carID, DeactivateCarRequest{Status: 0, UserID: actorID}); err != nil {
				return err
			}
		}
		return nil
	})
}

// loadActiveCarForBulk грузит машину внутри bulk-транзакции и проверяет, что она
// активна (status=1) - неактивную/несуществующую машину bulk-операция над
// car_target_tables не трогает, это ошибка конкретной строки, не всего пакета.
func loadActiveCarForBulk(tx *gorm.DB, carID int) (*models.Car, error) {
	var car models.Car
	if err := tx.Select("id", "car_number", "car_brand", "status").First(&car, carID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, echo.NewHTTPError(http.StatusNotFound, "Машина не найдена")
		}
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if car.Status == nil || *car.Status != 1 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Машина не активна")
	}
	return &car, nil
}

// currentCarTableIDs - текущие id таблиц «Проезд» машины внутри bulk-транзакции.
func currentCarTableIDs(tx *gorm.DB, carID int) ([]int, error) {
	var ids []int
	if err := tx.Model(&models.CarTargetTable{}).Where("car_id = ?", carID).Pluck("table_id", &ids).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error reading car tables")
	}
	return ids, nil
}

// bindCarTables добавляет машине недостающие привязки к tableIDs (дедуп с уже
// существующими) и возвращает итоговый набор id таблиц машины после операции -
// пустой означает, что вызывающий должен деактивировать машину. Используется
// moveCarTable ("Перенести") - целевая привязка source=manual (перенос - ручное
// действие, #1227), исходная application-привязка при этом снимается отдельно.
func bindCarTables(tx *gorm.DB, carID int, tableIDs []int) (map[int]struct{}, error) {
	current, err := currentCarTableIDs(tx, carID)
	if err != nil {
		return nil, err
	}
	set := make(map[int]struct{}, len(current)+len(tableIDs))
	for _, id := range current {
		set[id] = struct{}{}
	}
	for _, tableID := range tableIDs {
		if _, ok := set[tableID]; ok {
			continue
		}
		if err := tx.Create(&models.CarTargetTable{CarID: carID, TableID: tableID, Source: "manual"}).Error; err != nil {
			return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error binding car table")
		}
		set[tableID] = struct{}{}
	}
	return set, nil
}

// validateCarsTargetTables проверяет, что все переданные id существуют в
// system_tables и относятся к типу «cars» - иначе групповая операция привязала бы
// машину к чужой (people) таблице, молча ломая читатели, ожидающие там только cars
// (#1194, тип-матч).
func (s *carService) validateCarsTargetTables(ctx context.Context, ids []int) error {
	unique := uniqueInts(ids)
	if len(unique) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не указаны целевые таблицы")
	}
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Where("id IN ? AND table_type = ?", unique, models.TableTypeCars).
		Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error validating target tables")
	}
	if int(count) != len(unique) {
		return echo.NewHTTPError(http.StatusBadRequest, "Одна или несколько таблиц не найдены либо не относятся к типу «Проезд»")
	}
	return nil
}

// systemTableNames резолвит id таблиц в отображаемые имена (для человекочитаемого
// comment записи «перенесён из ... в ...»).
func (s *carService) systemTableNames(ctx context.Context, ids []int) (map[int]string, error) {
	unique := uniqueInts(ids)
	out := make(map[int]string, len(unique))
	if len(unique) == 0 {
		return out, nil
	}
	type row struct {
		ID          int
		Name        string
		DisplayName *string
	}
	var rows []row
	if err := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Select("id, name, display_name").Where("id IN ?", unique).Scan(&rows).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Error loading table names")
	}
	for _, r := range rows {
		name := r.Name
		if r.DisplayName != nil && *r.DisplayName != "" {
			name = *r.DisplayName
		}
		out[r.ID] = name
	}
	return out, nil
}

// joinTableNames склеивает отображаемые имена таблиц в порядке ids через запятую
// (для comment); id без резолвнутого имени пропускается.
func joinTableNames(ids []int, names map[int]string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		if name, ok := names[id]; ok {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
}

// carNumbersByIDs - вспомогательная выборка номеров машин для человекочитаемого
// поля Name в BulkItemError; несуществующий/недоступный id просто отсутствует в
// карте (addError получит пустую строку - как у остальных bulk-методов справочников).
func (s *carService) carNumbersByIDs(ctx context.Context, ids []int) map[int]string {
	unique := uniqueInts(ids)
	out := make(map[int]string, len(unique))
	if len(unique) == 0 {
		return out
	}
	type row struct {
		ID        int
		CarNumber *string
	}
	var rows []row
	if err := s.db.WithContext(ctx).Model(&models.Car{}).Select("id, car_number").
		Where("id IN ?", unique).Scan(&rows).Error; err != nil {
		return out
	}
	for _, r := range rows {
		if r.CarNumber != nil {
			out[r.ID] = *r.CarNumber
		}
	}
	return out
}
