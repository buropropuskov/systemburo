package handlers_test

import (
	"context"
	"encoding/json"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newSnapshotService собирает сервис снимков поверх реальных листингов машин/сотрудников
// на тест-БД. DB-backed, потому лежит в internal/handlers (единственный DB-бинарь, #706).
func newSnapshotService(db *gorm.DB) services.TableSnapshotService {
	rec := services.NewAuditRecorder(db)
	return services.NewTableSnapshotService(
		db,
		services.NewCarService(db, rec),
		services.NewEmployeeService(db, rec),
		services.NewEmployeesHistoryService(db),
	)
}

// TestTableSnapshot_Cars_CapturesRowsAndCounts: снимок cars-таблицы содержит активную
// машину с её территориальным статусом, counts верны, метаданные (reason/actor) записаны.
func TestTableSnapshot_Cars_CapturesRowsAndCounts(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_cars", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "snap_cars")

	dn := "Снимок машины"
	tbl := models.SystemTable{Name: "snap_cars_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	// На территории.
	require.NoError(t, db.Model(&models.Car{}).Where("id = ?", carID).Update("territory_status", 1).Error)

	snapID, err := newSnapshotService(db).SnapshotTable(context.Background(), tbl.ID, models.SnapshotReasonManual, &userID)
	require.NoError(t, err)
	require.NotZero(t, snapID)

	var snap models.TableSnapshot
	require.NoError(t, db.First(&snap, snapID).Error)
	assert.Equal(t, tbl.ID, snap.TableID)
	assert.Equal(t, models.SnapshotReasonManual, snap.Reason)
	require.NotNil(t, snap.ActorUserID)
	assert.Equal(t, userID, *snap.ActorUserID)

	var counts models.SnapshotCounts
	require.NoError(t, json.Unmarshal(snap.Counts, &counts))
	assert.Equal(t, 1, counts.Total, "одна активная машина")
	assert.Equal(t, 1, counts.OnTerritory, "машина на территории")
	assert.Equal(t, 0, counts.Exited)
	assert.Equal(t, 0, counts.NotEntered)

	var payload models.SnapshotPayload
	require.NoError(t, json.Unmarshal(snap.Payload, &payload))
	assert.Equal(t, models.TableTypeCars, payload.TableType)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(payload.Rows, &rows))
	require.Len(t, rows, 1, "строка машины в слепке")
	assert.Equal(t, float64(carID), rows[0]["id"])
	assert.Equal(t, float64(1), rows[0]["territory_status"], "статус машины сохранён в слепке")
}

// TestTableSnapshot_People_CapturesStatusesAndCounts: снимок people-таблицы содержит
// сотрудника с его территориальным статусом (выехал), counts верны.
func TestTableSnapshot_People_CapturesStatusesAndCounts(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_people", "pass123", 1, td.OrgID, td.CompanyID)

	appID, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")

	// Таблица, к которой привязан сотрудник (seedEmployeeViaCompleteApp создаёт её сам).
	var tableID int
	require.NoError(t, db.Model(&models.EmployeeTargetTable{}).
		Where("employee_id = ?", empID).Select("table_id").Scan(&tableID).Error)
	require.NotZero(t, tableID)

	// Согласованная заявка в работе + активный сотрудник, выехавший с территории -
	// прямые апдейты под фильтры GetActiveEmployeesForTable (детерминированно, без
	// зависимости от того, активирует ли flap take-to-work именно сотрудников).
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).Updates(map[string]any{
		"confirmation": models.ConfirmationApproved,
		"status":       models.StatusInWork,
	}).Error)
	require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", empID).Updates(map[string]any{
		"status":           1,
		"territory_status": 2,
	}).Error)

	snapID, err := newSnapshotService(db).SnapshotTable(context.Background(), tableID, models.SnapshotReasonScheduled, nil)
	require.NoError(t, err)
	require.NotZero(t, snapID)

	var snap models.TableSnapshot
	require.NoError(t, db.First(&snap, snapID).Error)
	assert.Equal(t, tableID, snap.TableID)
	assert.Equal(t, models.SnapshotReasonScheduled, snap.Reason)
	assert.Nil(t, snap.ActorUserID, "scheduled - без актора")

	var counts models.SnapshotCounts
	require.NoError(t, json.Unmarshal(snap.Counts, &counts))
	assert.Equal(t, 1, counts.Total, "один активный сотрудник")
	assert.Equal(t, 1, counts.Exited, "сотрудник выехал")
	assert.Equal(t, 0, counts.OnTerritory)
	assert.Equal(t, 0, counts.NotEntered)

	var payload models.SnapshotPayload
	require.NoError(t, json.Unmarshal(snap.Payload, &payload))
	assert.Equal(t, models.TableTypePeople, payload.TableType)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(payload.Rows, &rows))
	require.Len(t, rows, 1, "строка сотрудника в слепке")
	assert.Equal(t, float64(empID), rows[0]["id"])
	assert.Equal(t, "Ivanov", rows[0]["last_name"])
	assert.Equal(t, float64(2), rows[0]["territory_status"], "статус сотрудника сохранён в слепке")
}

// TestTableSnapshot_EmptyPeopleTable: снимок пустой people-таблицы даёт нулевые
// counts и пустой (не null) массив строк.
func TestTableSnapshot_EmptyPeopleTable(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dn := "Пустая people-таблица"
	tbl := models.SystemTable{Name: "snap_empty_people", DisplayName: &dn, TableType: models.TableTypePeople, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	snapID, err := newSnapshotService(db).SnapshotTable(context.Background(), tbl.ID, models.SnapshotReasonScheduled, nil)
	require.NoError(t, err)

	var snap models.TableSnapshot
	require.NoError(t, db.First(&snap, snapID).Error)

	var counts models.SnapshotCounts
	require.NoError(t, json.Unmarshal(snap.Counts, &counts))
	assert.Equal(t, models.SnapshotCounts{}, counts, "все счётчики нулевые")

	var payload models.SnapshotPayload
	require.NoError(t, json.Unmarshal(snap.Payload, &payload))
	assert.Equal(t, models.TableTypePeople, payload.TableType)
	assert.JSONEq(t, "[]", string(payload.Rows), "пустой массив, не null")
}

// TestTableSnapshot_TableNotFound: несуществующая таблица -> 404, снимок не пишется.
func TestTableSnapshot_TableNotFound(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	_, err := newSnapshotService(db).SnapshotTable(context.Background(), 999999, models.SnapshotReasonManual, nil)
	require.Error(t, err)

	var count int64
	require.NoError(t, db.Model(&models.TableSnapshot{}).Count(&count).Error)
	assert.Zero(t, count)
}

// TestTableSnapshot_UnsupportedTableType: неизвестный table_type -> ошибка, снимок не пишется.
func TestTableSnapshot_UnsupportedTableType(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dn := "Мусорный тип"
	tbl := models.SystemTable{Name: "snap_bad_type", DisplayName: &dn, TableType: "junk", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	_, err := newSnapshotService(db).SnapshotTable(context.Background(), tbl.ID, models.SnapshotReasonManual, nil)
	require.Error(t, err)

	var count int64
	require.NoError(t, db.Model(&models.TableSnapshot{}).Where("table_id = ?", tbl.ID).Count(&count).Error)
	assert.Zero(t, count)
}

// TestTableSnapshot_InvalidReason: некорректный reason отклоняется, снимок не пишется.
func TestTableSnapshot_InvalidReason(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	_ = e

	dn := "Снимок машины"
	tbl := models.SystemTable{Name: "snap_bad_reason", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	_, err := newSnapshotService(db).SnapshotTable(context.Background(), tbl.ID, "hourly", nil)
	require.Error(t, err)

	var count int64
	require.NoError(t, db.Model(&models.TableSnapshot{}).Where("table_id = ?", tbl.ID).Count(&count).Error)
	assert.Zero(t, count, "при невалидном reason строка не создаётся")
}
