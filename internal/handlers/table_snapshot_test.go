package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

// TestTableSnapshot_AllActiveTables_CoversSupportedSkipsOthers: дневная джоба снимает
// каждую активную cars/people-таблицу и пропускает архивные и неподдерживаемые типы.
func TestTableSnapshot_AllActiveTables_CoversSupportedSkipsOthers(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dnCars, dnPeople, dnArch, dnJunk := "Активные машины", "Активные люди", "Архивная", "Мусорный тип"
	carsTbl := models.SystemTable{Name: "all_cars", DisplayName: &dnCars, TableType: models.TableTypeCars, IsActive: true}
	peopleTbl := models.SystemTable{Name: "all_people", DisplayName: &dnPeople, TableType: models.TableTypePeople, IsActive: true}
	archTbl := models.SystemTable{Name: "all_arch", DisplayName: &dnArch, TableType: models.TableTypeCars}
	junkTbl := models.SystemTable{Name: "all_junk", DisplayName: &dnJunk, TableType: "passage", IsActive: true}
	for _, tbl := range []*models.SystemTable{&carsTbl, &peopleTbl, &archTbl, &junkTbl} {
		require.NoError(t, db.Create(tbl).Error)
	}
	// IsActive: false в структуре не сохраняется (gorm default:true игнорирует zero-value),
	// поэтому архивируем таблицу явным апдейтом.
	require.NoError(t, db.Model(&archTbl).UpdateColumn("is_active", false).Error)

	created, failed, err := newSnapshotService(db).SnapshotAllActiveTables(context.Background(), models.SnapshotReasonScheduled)
	require.NoError(t, err)
	assert.Equal(t, 2, created, "снимок только активных cars+people")
	assert.Equal(t, 0, failed)

	assertHasSnapshot := func(tableID int, want bool) {
		var n int64
		require.NoError(t, db.Model(&models.TableSnapshot{}).Where("table_id = ?", tableID).Count(&n).Error)
		if want {
			assert.EqualValues(t, 1, n, "ожидался снимок таблицы %d", tableID)
		} else {
			assert.Zero(t, n, "таблица %d не должна сниматься", tableID)
		}
	}
	assertHasSnapshot(carsTbl.ID, true)
	assertHasSnapshot(peopleTbl.ID, true)
	assertHasSnapshot(archTbl.ID, false)
	assertHasSnapshot(junkTbl.ID, false)

	var snap models.TableSnapshot
	require.NoError(t, db.Where("table_id = ?", carsTbl.ID).First(&snap).Error)
	assert.Equal(t, models.SnapshotReasonScheduled, snap.Reason)
	assert.Nil(t, snap.ActorUserID, "дневной снимок без актора")
}

// TestTableSnapshot_ManualEndpoint_CreatesRowWithActor: POST /system-tables/:id/snapshots
// -> 200, пишет версию с reason=manual и актором-инициатором.
func TestTableSnapshot_ManualEndpoint_CreatesRowWithActor(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_manual", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "snap_manual")

	dn := "Ручной снимок"
	tbl := models.SystemTable{Name: "manual_snap_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	rec := testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/snapshots", tbl.ID), "", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual snapshot: %s", rec.Body.String())

	var snap models.TableSnapshot
	require.NoError(t, db.Where("table_id = ?", tbl.ID).First(&snap).Error)
	assert.Equal(t, models.SnapshotReasonManual, snap.Reason)
	require.NotNil(t, snap.ActorUserID)
	assert.Equal(t, userID, *snap.ActorUserID, "актор - инициатор запроса")
}

// TestTableSnapshot_ManualEndpoint_Unauthorized: без токена ручной снимок отбивается.
func TestTableSnapshot_ManualEndpoint_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.POST(t, e, "/system-tables/1/snapshots", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestTableSnapshot_ScheduledBeforeReset_PreservesExitedState: снимок ДО сброса хранит
// статус «выехал» (2), а сброс обнуляет строку в БД - версия остаётся источником
// суточного состояния. Порядок snapshot-then-reset (main.snapshotThenReset).
func TestTableSnapshot_ScheduledBeforeReset_PreservesExitedState(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_reset", "pass123", 1, td.OrgID, td.CompanyID)

	dn := "Снимок до сброса"
	tbl := models.SystemTable{Name: "reset_snap_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	// Машина выехала (2) - именно её сброс обнулит.
	require.NoError(t, db.Model(&models.Car{}).Where("id = ?", carID).Update("territory_status", 2).Error)

	// 1) Снимок всех активных таблиц.
	created, failed, err := newSnapshotService(db).SnapshotAllActiveTables(context.Background(), models.SnapshotReasonScheduled)
	require.NoError(t, err)
	require.Equal(t, 1, created)
	require.Equal(t, 0, failed)

	// 2) Затем сброс: 2 -> 0.
	_, carsReset, err := services.NewTerritoryResetService(db).ResetExitedStatuses(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, carsReset, "выехавшая машина сброшена")

	// БД обнулена, но версия хранит статус «выехал».
	var dbStatus int
	require.NoError(t, db.Model(&models.Car{}).Where("id = ?", carID).Select("territory_status").Scan(&dbStatus).Error)
	assert.Equal(t, 0, dbStatus, "в БД статус сброшен")

	var snap models.TableSnapshot
	require.NoError(t, db.Where("table_id = ?", tbl.ID).First(&snap).Error)
	var counts models.SnapshotCounts
	require.NoError(t, json.Unmarshal(snap.Counts, &counts))
	assert.Equal(t, 1, counts.Exited, "снимок сохранил выехавшую машину")

	var payload models.SnapshotPayload
	require.NoError(t, json.Unmarshal(snap.Payload, &payload))
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(payload.Rows, &rows))
	require.Len(t, rows, 1)
	assert.Equal(t, float64(2), rows[0]["territory_status"], "версия помнит статус до сброса")
}

// TestTableSnapshot_FactCars_IncludedOnlyWhenShowFactTable: машина «по факту» попадает
// в слепок только у таблицы с show_fact_table=true (помечена is_fact), иначе - нет.
func TestTableSnapshot_FactCars_IncludedOnlyWhenShowFactTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_fact", "pass123", 1, td.OrgID, td.CompanyID)

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	// Машина «по факту» на территории: выпадает из основного листинга, попадает в fact.
	require.NoError(t, db.Model(&models.Car{}).Where("id = ?", carID).Updates(map[string]any{
		"car_number":       "по факту",
		"territory_status": 1,
	}).Error)

	dnOn, dnOff := "С фактом", "Без факта"
	withFact := models.SystemTable{Name: "fact_on", DisplayName: &dnOn, TableType: models.TableTypeCars, IsActive: true, ShowFactTable: true}
	noFact := models.SystemTable{Name: "fact_off", DisplayName: &dnOff, TableType: models.TableTypeCars, IsActive: true, ShowFactTable: false}
	require.NoError(t, db.Create(&withFact).Error)
	require.NoError(t, db.Create(&noFact).Error)

	svc := newSnapshotService(db)

	// show_fact_table=true: снимок содержит машину «по факту» с is_fact=true.
	onID, err := svc.SnapshotTable(context.Background(), withFact.ID, models.SnapshotReasonManual, nil)
	require.NoError(t, err)
	var onSnap models.TableSnapshot
	require.NoError(t, db.First(&onSnap, onID).Error)
	var onCounts models.SnapshotCounts
	require.NoError(t, json.Unmarshal(onSnap.Counts, &onCounts))
	assert.Equal(t, 1, onCounts.Total, "машина «по факту» в слепке")
	assert.Equal(t, 1, onCounts.OnTerritory)
	var onPayload models.SnapshotPayload
	require.NoError(t, json.Unmarshal(onSnap.Payload, &onPayload))
	var onRows []map[string]any
	require.NoError(t, json.Unmarshal(onPayload.Rows, &onRows))
	require.Len(t, onRows, 1)
	assert.Equal(t, true, onRows[0]["is_fact"], "строка помечена is_fact")

	// show_fact_table=false: та же машина в слепок не попадает.
	offID, err := svc.SnapshotTable(context.Background(), noFact.ID, models.SnapshotReasonManual, nil)
	require.NoError(t, err)
	var offSnap models.TableSnapshot
	require.NoError(t, db.First(&offSnap, offID).Error)
	var offCounts models.SnapshotCounts
	require.NoError(t, json.Unmarshal(offSnap.Counts, &offCounts))
	assert.Equal(t, 0, offCounts.Total, "без show_fact_table машина «по факту» не снимается")
}

// failingCarLister имитирует сбой листинга машин, чтобы проверить, что провал одной
// таблицы в SnapshotAllActiveTables не роняет остальные (per-table failed++/continue).
type failingCarLister struct{}

func (failingCarLister) GetActiveCarsForTables(_ context.Context) ([]services.TableCarResponse, error) {
	return nil, fmt.Errorf("boom: не удалось получить список машин")
}

func (failingCarLister) GetFactCarsForTables(_ context.Context) ([]services.TableCarResponse, error) {
	return nil, fmt.Errorf("boom: не удалось получить список машин «по факту»")
}

// TestTableSnapshot_AllActiveTables_ContinuesOnPerTableFailure: если снимок одной таблицы
// падает (сбой листинга машин), джоба учитывает её в failed и всё равно снимает остальные -
// дневной проход обязан дойти до сброса статусов. created=1 (people), failed=1 (cars), err=nil.
func TestTableSnapshot_AllActiveTables_ContinuesOnPerTableFailure(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dnCars, dnPeople := "Сбойные машины", "Рабочие люди"
	carsTbl := models.SystemTable{Name: "fail_cars", DisplayName: &dnCars, TableType: models.TableTypeCars, IsActive: true}
	peopleTbl := models.SystemTable{Name: "ok_people", DisplayName: &dnPeople, TableType: models.TableTypePeople, IsActive: true}
	require.NoError(t, db.Create(&carsTbl).Error)
	require.NoError(t, db.Create(&peopleTbl).Error)

	// Листинг машин всегда падает; сотрудники - реальные, поэтому people-таблица снимается.
	svc := services.NewTableSnapshotService(
		db,
		failingCarLister{},
		services.NewEmployeeService(db, services.NewAuditRecorder(db)),
		services.NewEmployeesHistoryService(db),
	)

	created, failed, err := svc.SnapshotAllActiveTables(context.Background(), models.SnapshotReasonScheduled)
	require.NoError(t, err, "провал одной таблицы не всплывает наверх - сброс должен продолжиться")
	assert.Equal(t, 1, created, "people-таблица снята несмотря на сбой машин")
	assert.Equal(t, 1, failed, "сбойная cars-таблица учтена в failed")

	var carsN, peopleN int64
	require.NoError(t, db.Model(&models.TableSnapshot{}).Where("table_id = ?", carsTbl.ID).Count(&carsN).Error)
	require.NoError(t, db.Model(&models.TableSnapshot{}).Where("table_id = ?", peopleTbl.ID).Count(&peopleN).Error)
	assert.Zero(t, carsN, "сбойная таблица осталась без снимка")
	assert.EqualValues(t, 1, peopleN, "рабочая таблица снята")
}
