package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

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
	// Привязка «Проезд» к снимаемой таблице (#1036): снимок скоуплен по car_target_tables.
	require.NoError(t, db.Exec(
		"INSERT INTO car_target_tables (car_id, table_id, order_index) VALUES (?, ?, 1)", carID, tbl.ID).Error)
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

	// Снимок версии теперь под правом table.<name>.versions - выдаём его юзеру.
	testutil.GrantTableVerb(t, userID, tbl.Name, "versions")

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
	// Привязка «Проезд» к снимаемой таблице (#1036): снимок скоуплен по car_target_tables.
	require.NoError(t, db.Exec(
		"INSERT INTO car_target_tables (car_id, table_id, order_index) VALUES (?, ?, 1)", carID, tbl.ID).Error)
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
	// Привязка «Проезд» к обеим таблицам (#1036), чтобы тест проверял эффект
	// show_fact_table, а не scope: fact-машина видна в обеих по «Проезду».
	for _, tid := range []int{withFact.ID, noFact.ID} {
		require.NoError(t, db.Exec(
			"INSERT INTO car_target_tables (car_id, table_id, order_index) VALUES (?, ?, 1)", carID, tid).Error)
	}

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

func (failingCarLister) GetActiveCarsForTable(_ context.Context, _ int) ([]services.TableCarResponse, error) {
	return nil, fmt.Errorf("boom: не удалось получить список машин")
}

func (failingCarLister) GetFactCarsForTable(_ context.Context, _ int) ([]services.TableCarResponse, error) {
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

// seedSnapshot вставляет версию напрямую (минуя сбор строк) - для тестов чтения/чистки,
// где важны метаданные/payload и возраст, а не механика снятия. Payload - валидный
// cars-слепок с одной строкой.
func seedSnapshot(t *testing.T, db *gorm.DB, tableID int, reason string, actor *int, takenAt time.Time, counts models.SnapshotCounts) int {
	t.Helper()
	countsJSON, err := json.Marshal(counts)
	require.NoError(t, err)
	payloadJSON, err := json.Marshal(models.SnapshotPayload{
		TableType: models.TableTypeCars,
		Rows:      json.RawMessage(`[{"id":1,"territory_status":1}]`),
	})
	require.NoError(t, err)
	snap := models.TableSnapshot{
		TableID:     tableID,
		TakenAt:     takenAt,
		Reason:      reason,
		ActorUserID: actor,
		Payload:     payloadJSON,
		Counts:      countsJSON,
	}
	require.NoError(t, db.Create(&snap).Error)
	return snap.ID
}

// TestTableSnapshot_List_ReturnsMetadataWithoutPayload: GET списка отдаёт версии
// метаданными (reason, автор, распакованные counts) в порядке taken_at DESC, БЕЗ payload.
func TestTableSnapshot_List_ReturnsMetadataWithoutPayload(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_list", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "snap_list")

	dn := "Список версий"
	tbl := models.SystemTable{Name: "list_snap_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	base := time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC)
	seedSnapshot(t, db, tbl.ID, models.SnapshotReasonScheduled, nil, base, models.SnapshotCounts{OnTerritory: 2, Total: 2})
	newer := seedSnapshot(t, db, tbl.ID, models.SnapshotReasonManual, &userID, base.Add(48*time.Hour), models.SnapshotCounts{Exited: 1, Total: 1})

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots", tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Data []struct {
			ID        int                   `json:"id"`
			Reason    string                `json:"reason"`
			ActorName string                `json:"actor_name"`
			Counts    models.SnapshotCounts `json:"counts"`
			Payload   json.RawMessage       `json:"payload"`
		} `json:"data"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2)
	assert.EqualValues(t, 2, resp.Meta.Total)
	// taken_at DESC: свежий (manual) первым.
	assert.Equal(t, newer, resp.Data[0].ID)
	assert.Equal(t, models.SnapshotReasonManual, resp.Data[0].Reason)
	assert.Equal(t, 1, resp.Data[0].Counts.Exited, "counts распакованы в списке")
	assert.NotEmpty(t, resp.Data[0].ActorName, "автор ручного снимка подставлен")
	assert.Empty(t, resp.Data[0].Payload, "payload в списке не отдаётся")
	assert.Empty(t, resp.Data[1].ActorName, "дневной снимок без актора")
}

// TestTableSnapshot_List_FilterByPeriod: фильтр from по дате отсекает версии старше границы.
func TestTableSnapshot_List_FilterByPeriod(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_filter", "pass123", 1, td.OrgID, td.CompanyID)

	dn := "Фильтр версий"
	tbl := models.SystemTable{Name: "filter_snap_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	seedSnapshot(t, db, tbl.ID, models.SnapshotReasonScheduled, nil, time.Date(2026, 1, 10, 8, 0, 0, 0, time.UTC), models.SnapshotCounts{Total: 1})
	mid := seedSnapshot(t, db, tbl.ID, models.SnapshotReasonScheduled, nil, time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC), models.SnapshotCounts{Total: 1})

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots?from=2026-06-01", tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Data []struct {
			ID int `json:"id"`
		} `json:"data"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1, "только версия после границы")
	assert.EqualValues(t, 1, resp.Meta.Total)
	assert.Equal(t, mid, resp.Data[0].ID)

	// Верхняя граница date-only включает весь день (endOfDay): to=дата снимка отдаёт
	// снимок этого дня, снятый в 08:00 (при трактовке границы как полуночи он бы выпал).
	recTo := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots?to=2026-01-10", tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recTo.Code, recTo.Body.String())
	var respTo struct {
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(recTo.Body.Bytes(), &respTo))
	assert.EqualValues(t, 1, respTo.Meta.Total, "снимок в 08:00 верхней границы-дня включён (endOfDay)")

	// Кривой фильтр даты - явная 400, не молчаливое игнорирование.
	bad := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots?from=notadate", tbl.ID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, bad.Code)
}

// TestPermissions_ReconcileAllTablePermissions_BackfillsMissingVerb: реконсиляция
// догенеривает недостающий глагол таблице, созданной до его появления, и идемпотентна.
func TestPermissions_ReconcileAllTablePermissions_BackfillsMissingVerb(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dn := "Реконсиляция прав"
	tbl := models.SystemTable{Name: "reconcile_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	svc := services.NewPermissionService(db)
	require.NoError(t, svc.AutoGenerateForTable(context.Background(), tbl.ID, tbl.Name))

	// Симулируем таблицу «до глагола versions»: удаляем это право.
	require.NoError(t, db.Where("key = ?", "table.reconcile_tbl.versions").Delete(&models.Permission{}).Error)
	var before int64
	require.NoError(t, db.Model(&models.Permission{}).Where("key LIKE ?", "table.reconcile_tbl.%").Count(&before).Error)
	require.EqualValues(t, 9, before, "versions удалён - осталось 9")

	// Реконсиляция восстанавливает недостающее право.
	require.NoError(t, svc.ReconcileAllTablePermissions(context.Background()))
	var after int64
	require.NoError(t, db.Model(&models.Permission{}).Where("key LIKE ?", "table.reconcile_tbl.%").Count(&after).Error)
	assert.EqualValues(t, 10, after, "versions догенерирован")

	var versionsPerm models.Permission
	require.NoError(t, db.Where("key = ?", "table.reconcile_tbl.versions").First(&versionsPerm).Error)
	assert.Equal(t, "table", versionsPerm.Category)
	require.NotNil(t, versionsPerm.EntityID)
	assert.Equal(t, tbl.ID, *versionsPerm.EntityID)

	// Идемпотентность: повторный вызов не плодит дублей.
	require.NoError(t, svc.ReconcileAllTablePermissions(context.Background()))
	var again int64
	require.NoError(t, db.Model(&models.Permission{}).Where("key LIKE ?", "table.reconcile_tbl.%").Count(&again).Error)
	assert.EqualValues(t, 10, again, "повторная реконсиляция без дублей")
}

// TestTableSnapshot_Get_ReturnsPayload: GET версии отдаёт полный payload со строками.
func TestTableSnapshot_Get_ReturnsPayload(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_get", "pass123", 1, td.OrgID, td.CompanyID)

	dn := "Получение версии"
	tbl := models.SystemTable{Name: "get_snap_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	sid := seedSnapshot(t, db, tbl.ID, models.SnapshotReasonManual, nil, time.Now().UTC(), models.SnapshotCounts{OnTerritory: 1, Total: 1})

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots/%d", tbl.ID, sid), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Data struct {
			ID      int                    `json:"id"`
			Payload models.SnapshotPayload `json:"payload"`
			Counts  models.SnapshotCounts  `json:"counts"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, sid, resp.Data.ID)
	assert.Equal(t, models.TableTypeCars, resp.Data.Payload.TableType)
	assert.Equal(t, 1, resp.Data.Counts.OnTerritory)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(resp.Data.Payload.Rows, &rows))
	require.Len(t, rows, 1, "payload со строками отдан")
}

// TestTableSnapshot_Get_WrongTable_404: версию нельзя прочитать через ID чужой таблицы.
func TestTableSnapshot_Get_WrongTable_404(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_scope", "pass123", 1, td.OrgID, td.CompanyID)

	dnA, dnB := "Таблица A", "Таблица B"
	tblA := models.SystemTable{Name: "scope_a", DisplayName: &dnA, TableType: models.TableTypeCars, IsActive: true}
	tblB := models.SystemTable{Name: "scope_b", DisplayName: &dnB, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tblA).Error)
	require.NoError(t, db.Create(&tblB).Error)
	sid := seedSnapshot(t, db, tblA.ID, models.SnapshotReasonManual, nil, time.Now().UTC(), models.SnapshotCounts{Total: 1})

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots/%d", tblB.ID, sid), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code, "снимок таблицы A недоступен через таблицу B")
}

// TestTableSnapshot_Cleanup_DeletesOldKeepsFresh: admin-чистка удаляет версии старше
// порога, свежие остаются; удаление скоуплено по таблице.
func TestTableSnapshot_Cleanup_DeletesOldKeepsFresh(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	dn := "Чистка версий"
	tbl := models.SystemTable{Name: "cleanup_snap_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	now := time.Now().UTC()
	oldSid := seedSnapshot(t, db, tbl.ID, models.SnapshotReasonScheduled, nil, now.AddDate(0, -14, 0), models.SnapshotCounts{Total: 1})
	freshSid := seedSnapshot(t, db, tbl.ID, models.SnapshotReasonScheduled, nil, now.AddDate(0, -1, 0), models.SnapshotCounts{Total: 1})

	rec := testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/snapshots?older_than=12", tbl.ID), testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var resp struct {
		Data struct {
			Deleted int64 `json:"deleted"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.EqualValues(t, 1, resp.Data.Deleted, "удалена одна старая версия")

	assert.ErrorIs(t, db.First(&models.TableSnapshot{}, oldSid).Error, gorm.ErrRecordNotFound, "старая версия удалена")
	require.NoError(t, db.First(&models.TableSnapshot{}, freshSid).Error, "свежая версия осталась")
}

// TestTableSnapshot_Cleanup_Forbidden_NormalUser: чистка недоступна не-админу (requireAdmin).
func TestTableSnapshot_Cleanup_Forbidden_NormalUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_nodel", "pass123", 1, td.OrgID, td.CompanyID)

	dn := "Чистка без прав"
	tbl := models.SystemTable{Name: "nodel_snap_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	sid := seedSnapshot(t, db, tbl.ID, models.SnapshotReasonScheduled, nil, time.Now().UTC().AddDate(0, -24, 0), models.SnapshotCounts{Total: 1})

	rec := testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/snapshots?older_than=1", tbl.ID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code, "не-админ не чистит версии")
	require.NoError(t, db.First(&models.TableSnapshot{}, sid).Error, "версия на месте - удаления не было")
}

// TestTableSnapshot_Cleanup_BadOlderThan_400: без валидного older_than чистка - 400 (даже админу).
func TestTableSnapshot_Cleanup_BadOlderThan_400(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	dn := "Плохой порог"
	tbl := models.SystemTable{Name: "badthr_snap_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	missing := testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/snapshots", tbl.ID), testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, missing.Code, "без older_than - 400")

	zero := testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/snapshots?older_than=0", tbl.ID), testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, zero.Code, "older_than=0 - 400")
}

// seedCarSnapshotWithCyrillic создаёт версию cars-таблицы с кириллическими значениями
// в payload - чтобы экспорт (особенно PDF) прогонял реальную кириллицу.
func seedCarSnapshotWithCyrillic(t *testing.T, db *gorm.DB, tableID int) int {
	t.Helper()
	rows := json.RawMessage(`[{"id":1,"car_number":"А123ВС77","car_brand":"Камаз","organization":"ООО Ромашка","company":"Компания","application_number":"№ 42","territory_status":1,"unload_places":["Склад №1"]}]`)
	payloadJSON, err := json.Marshal(models.SnapshotPayload{TableType: models.TableTypeCars, Rows: rows})
	require.NoError(t, err)
	countsJSON, err := json.Marshal(models.SnapshotCounts{OnTerritory: 1, Total: 1})
	require.NoError(t, err)
	snap := models.TableSnapshot{
		TableID: tableID, TakenAt: time.Now().UTC(), Reason: models.SnapshotReasonManual,
		Payload: payloadJSON, Counts: countsJSON,
	}
	require.NoError(t, db.Create(&snap).Error)
	return snap.ID
}

// TestTableSnapshot_Export_Xlsx: экспорт версии в xlsx - 200, верный content-type,
// attachment-заголовок, тело - валидный zip (xlsx).
func TestTableSnapshot_Export_Xlsx(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_xlsx", "pass123", 1, td.OrgID, td.CompanyID)
	dn := "Экспорт машин"
	tbl := models.SystemTable{Name: "export_cars_x", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	sid := seedCarSnapshotWithCyrillic(t, db, tbl.ID)

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots/%d/export?format=xlsx", tbl.ID, sid), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), ".xlsx")
	body := rec.Body.Bytes()
	require.NotEmpty(t, body, "xlsx не пустой")
	assert.True(t, bytes.HasPrefix(body, []byte{'P', 'K', 0x03, 0x04}), "xlsx - это zip-архив")
}

// TestTableSnapshot_Export_Pdf_EmbedsCyrillic: экспорт версии в pdf - 200, content-type
// pdf, валидная сигнатура, встроен кириллический шрифт DejaVu (кириллица рендерится
// глифами, а не заменяется на «?»).
func TestTableSnapshot_Export_Pdf_EmbedsCyrillic(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_pdf", "pass123", 1, td.OrgID, td.CompanyID)
	dn := "Экспорт машин"
	tbl := models.SystemTable{Name: "export_cars_p", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	sid := seedCarSnapshotWithCyrillic(t, db, tbl.ID)

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots/%d/export?format=pdf", tbl.ID, sid), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), ".pdf")
	body := rec.Body.Bytes()
	require.NotEmpty(t, body, "pdf не пустой")
	assert.True(t, bytes.HasPrefix(body, []byte("%PDF")), "валидная сигнатура PDF")
	raw := string(body)
	assert.Contains(t, raw, "/BaseFont /utf8dejavu", "встроен кириллический шрифт DejaVu")
	assert.Contains(t, raw, "FontFile2", "TTF-программа шрифта встроена (не core-шрифт)")
}

// TestTableSnapshot_Export_Current: sid=current экспортирует текущее состояние таблицы
// (без версии в БД), формат по умолчанию xlsx.
func TestTableSnapshot_Export_Current(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_cur", "pass123", 1, td.OrgID, td.CompanyID)
	dn := "Текущее состояние"
	tbl := models.SystemTable{Name: "export_current", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots/current/export", tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", rec.Header().Get("Content-Type"))
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte{'P', 'K', 0x03, 0x04}), "текущее состояние выгружено в xlsx")
}

// TestTableSnapshot_Export_InvalidFormat_400: неизвестный формат отбивается 400.
func TestTableSnapshot_Export_InvalidFormat_400(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_badfmt", "pass123", 1, td.OrgID, td.CompanyID)
	dn := "Экспорт машин"
	tbl := models.SystemTable{Name: "export_badfmt", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	sid := seedCarSnapshotWithCyrillic(t, db, tbl.ID)

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots/%d/export?format=csv", tbl.ID, sid), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "формат csv не поддерживается")
}

// TestTableSnapshot_Export_NotFound_404: экспорт несуществующей версии - 404.
func TestTableSnapshot_Export_NotFound_404(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_exp404", "pass123", 1, td.OrgID, td.CompanyID)
	dn := "Экспорт машин"
	tbl := models.SystemTable{Name: "export_404", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots/999999/export?format=xlsx", tbl.ID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code, "несуществующая версия - 404")
}

// TestTableSnapshot_Export_WrongTable_404: экспорт версии через ID чужой таблицы -
// 404 (IDOR закрыт, как у Get). Новая поверхность /export фиксируется явно.
func TestTableSnapshot_Export_WrongTable_404(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "snap_exp_idor", "pass123", 1, td.OrgID, td.CompanyID)
	dnA, dnB := "Таблица A", "Таблица B"
	tblA := models.SystemTable{Name: "export_idor_a", DisplayName: &dnA, TableType: models.TableTypeCars, IsActive: true}
	tblB := models.SystemTable{Name: "export_idor_b", DisplayName: &dnB, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tblA).Error)
	require.NoError(t, db.Create(&tblB).Error)
	sid := seedCarSnapshotWithCyrillic(t, db, tblA.ID)

	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/snapshots/%d/export?format=xlsx", tblB.ID, sid), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code, "версия таблицы A недоступна на экспорт через таблицу B")
}

// TestTableSnapshot_CapturesColumnStructure: снимок сохраняет настройку колонок таблицы
// (все поля, в порядке display_order, со скрытыми) - чтобы просмотр версии показал ровно
// те столбцы, что были настроены на момент слепка (самодостаточность, замечание #980).
func TestTableSnapshot_CapturesColumnStructure(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	_ = testutil.SeedTestData(t, db)

	dn := "Снимок колонок"
	tbl := models.SystemTable{Name: "snap_cols_tbl", DisplayName: &dn, TableType: models.TableTypeCars, IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	ord := func(n int) *int { return &n }
	fields := []models.TableField{
		{TableID: tbl.ID, FieldName: "car_brand", DisplayOrder: ord(1), IsVisible: true, Width: 15},
		{TableID: tbl.ID, FieldName: "car_number", DisplayOrder: ord(0), IsVisible: true, Width: 20},
		{TableID: tbl.ID, FieldName: "organization", DisplayOrder: ord(2), IsVisible: false, Width: 10},
	}
	require.NoError(t, db.Create(&fields).Error)
	// GORM с gorm:"default:true" игнорирует IsVisible:false при Create (bool zero-value) -
	// в проде столбец скрывают UPDATE'ом из конструктора; в фикстуре делаем так же.
	require.NoError(t, db.Model(&models.TableField{}).
		Where("table_id = ? AND field_name = ?", tbl.ID, "organization").
		Update("is_visible", false).Error)

	snapID, err := newSnapshotService(db).SnapshotTable(context.Background(), tbl.ID, models.SnapshotReasonManual, nil)
	require.NoError(t, err)

	var snap models.TableSnapshot
	require.NoError(t, db.First(&snap, snapID).Error)
	var payload models.SnapshotPayload
	require.NoError(t, json.Unmarshal(snap.Payload, &payload))

	require.Len(t, payload.Fields, 3, "все колонки таблицы сохранены в снимке")
	assert.Equal(t, "car_number", payload.Fields[0].FieldName, "порядок колонок по display_order")
	assert.Equal(t, "car_brand", payload.Fields[1].FieldName)
	assert.Equal(t, "organization", payload.Fields[2].FieldName)
	assert.False(t, payload.Fields[2].IsVisible, "скрытая колонка запомнена как скрытая")
	assert.True(t, payload.Fields[0].IsVisible)
	assert.Equal(t, 20, payload.Fields[0].Width, "ширина колонки сохранена в снимке")
}
