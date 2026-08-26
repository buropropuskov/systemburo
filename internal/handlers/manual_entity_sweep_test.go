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
)

// #1049 S5 (entity-first sweep): снимки версий, история и real-time для ручных
// сущностей без заявки. Ручные висят на вложении-сироте (application_id NULL,
// is_manual), org/company берутся с самого вложения через COALESCE.

// TestSnapshot_IncludesManualCar: снимок cars-таблицы содержит ручную машину -
// снимок переиспользует листинг GetActiveCarsForTable (ветка is_manual из S3),
// поэтому сирота попадает в payload с пустой заявкой (application_id null).
func TestSnapshot_IncludesManualCar(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	tableID := seedCarsTable(t, db, "snap_manual_cars", "Проезд снимок")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d,
		"vehicles": [{"car_number": "S100SS777", "car_brand": "Ford", "unload_places": []}]}`, td.OrgID, tableID)
	rec := testutil.POST(t, e, "/cars/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	carID := testutil.ParseResponse[services.ManualCarResponse](t, rec).CarIDs[0]

	snapID, err := newSnapshotService(db).SnapshotTable(context.Background(), tableID, models.SnapshotReasonScheduled, nil)
	require.NoError(t, err)

	var snap models.TableSnapshot
	require.NoError(t, db.First(&snap, snapID).Error)
	var payload models.SnapshotPayload
	require.NoError(t, json.Unmarshal(snap.Payload, &payload))
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(payload.Rows, &rows))
	require.Len(t, rows, 1, "ручная машина попала в снимок")
	assert.Equal(t, float64(carID), rows[0]["id"])
	assert.Equal(t, "S100SS777", rows[0]["car_number"])
	assert.Nil(t, rows[0]["application_id"], "ручная машина в снимке без заявки (метка)")
	assert.Equal(t, "Test Organization", rows[0]["organization"], "org в снимке резолвится с вложения")
}

// TestSnapshot_IncludesManualEmployee: снимок people-таблицы содержит ручного
// сотрудника (листинг GetActiveEmployeesForTable, ветка is_manual из S4).
func TestSnapshot_IncludesManualEmployee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, "snap_manual_people", "Проход снимок")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d,
		"employees": [{"last_name": "Petrov", "first_name": "Petr", "citizenship_id": %d,
		"position": "Loader", "passport_series_number": "9999 888777", "target_tables": []}]}`, td.OrgID, tableID, citizenshipID)
	rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	empID := testutil.ParseResponse[services.ManualEmployeeResponse](t, rec).EmployeeIDs[0]

	snapID, err := newSnapshotService(db).SnapshotTable(context.Background(), tableID, models.SnapshotReasonScheduled, nil)
	require.NoError(t, err)

	var snap models.TableSnapshot
	require.NoError(t, db.First(&snap, snapID).Error)
	var payload models.SnapshotPayload
	require.NoError(t, json.Unmarshal(snap.Payload, &payload))
	var rows []map[string]any
	require.NoError(t, json.Unmarshal(payload.Rows, &rows))
	require.Len(t, rows, 1, "ручной сотрудник попал в снимок")
	assert.Equal(t, float64(empID), rows[0]["id"])
	assert.Equal(t, "Petrov", rows[0]["last_name"])
	assert.Nil(t, rows[0]["application_id"], "ручной сотрудник в снимке без заявки (метка)")
	assert.Equal(t, "Test Organization", rows[0]["organization"], "org в снимке резолвится с вложения")
}

// TestCarHistory_Manual_ResolvesOrg: история ручной машины показывает организацию с
// вложения-сироты через COALESCE - и в объединённой (unified: LEFT JOIN applications
// находит машину без заявки, org фильтром COALESCE), и в общей (all по entry/exit).
func TestCarHistory_Manual_ResolvesOrg(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	tableID := seedCarsTable(t, db, "hist_manual_cars", "Проезд история")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d,
		"vehicles": [{"car_number": "H500HH777", "car_brand": "Kamaz", "unload_places": []}]}`, td.OrgID, tableID)
	rec := testutil.POST(t, e, "/cars/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	carID := testutil.ParseResponse[services.ManualCarResponse](t, rec).CarIDs[0]

	// Объединённая история: даже с фильтром по организации ручная машина находится
	// (COALESCE берёт org с вложения), запись «create» несёт организацию.
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/history/unified?car_number=H500HH777&car_brand=Kamaz&organization_id=%d", td.OrgID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "unified: %s", rec.Body.String())
	unified := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, unified, "ручная машина видна в объединённой истории по номеру+марке")
	assert.Equal(t, "Test Organization", unified[0]["organization"], "org в истории резолвится с вложения (COALESCE)")

	// Общая история (entry/exit): регистрируем въезд и проверяем org той же машины.
	recorder := services.NewAuditRecorder(db)
	carSvc := services.NewCarService(db, recorder)
	entry := 1
	require.NoError(t, carSvc.UpdateCarTerritoryStatus(context.Background(), carID, services.UpdateCarTerritoryStatusRequest{
		UpdateTerritoryStatusRequest: services.UpdateTerritoryStatusRequest{
			TerritoryStatus: entry,
			TableID:         &tableID,
		},
	}))

	rec = testutil.GET(t, e, "/cars/history/all", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "all: %s", rec.Body.String())
	all := testutil.ParseSlice(t, rec)
	var found map[string]any
	for _, r := range all {
		if r["car_number"] == "H500HH777" {
			found = r
			break
		}
	}
	require.NotNil(t, found, "въезд ручной машины виден в общей истории")
	assert.Equal(t, "Test Organization", found["organization"], "org в общей истории резолвится с вложения (COALESCE)")
}

// TestEmployeeHistory_Manual_ResolvesOrg: объединённая история ручного сотрудника (по
// ФИО) показывает организацию с вложения-сироты через COALESCE в baseSelectSQL.
func TestEmployeeHistory_Manual_ResolvesOrg(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, "hist_manual_people", "Проход история")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d,
		"employees": [{"last_name": "Sidorov", "first_name": "Semen", "citizenship_id": %d,
		"position": "Loader", "passport_series_number": "1111 222333", "target_tables": []}]}`, td.OrgID, tableID, citizenshipID)
	rec := testutil.POST(t, e, "/employees/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())

	rec = testutil.GET(t, e, "/employees/history/unified?last_name=Sidorov&first_name=Semen", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "unified: %s", rec.Body.String())
	unified := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, unified, "ручной сотрудник виден в объединённой истории по ФИО")
	assert.Equal(t, "Test Organization", unified[0]["organization"], "org в истории резолвится с вложения (COALESCE)")
}

// TestTablesRefresh_ManualCarAdd: ручное добавление машины шлёт tables.refresh
// аудитории целевой таблицы (#1049) - по car_target_tables, т.к. заявки нет.
func TestTablesRefresh_ManualCarAdd(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	tableID := seedCarsTable(t, db, "rt_manual_cars", "Проезд RT")
	scope := fmt.Sprintf("tables:%d", tableID)

	testutil.RegisterAndLogin(t, e, "rt_guard_car", "pass123", 1, td.OrgID, td.CompanyID)
	guardID := getUserID(t, db, "rt_guard_car")
	grantTableView(t, db, guardID, "rt_manual_cars")
	testutil.RegisterAndLogin(t, e, "rt_creator_car", "pass123", 1, td.OrgID, td.CompanyID)
	creatorID := getUserID(t, db, "rt_creator_car")

	fake := &fakePublisher{}
	resolver := services.NewPermissionResolver(db)
	producer := services.NewTablesRefreshPublisher(db, resolver, fake)
	carSvc := services.NewCarService(db, services.NewAuditRecorder(db), services.WithCarTablesProducer(producer))

	_, err := carSvc.CreateManualCars(context.Background(), services.ManualCarRequest{
		OrganizationID: td.OrgID,
		TableID:        tableID,
		Vehicles:       []services.ManualVehicle{{CarNumber: "R700RR777", CarBrand: "MAN"}},
	}, creatorID)
	require.NoError(t, err)

	audience := findTablesRefresh(fake, scope)
	require.NotNil(t, audience, "ручное добавление машины шлёт tables.refresh целевой таблице")
	assert.Contains(t, audience, guardID, "аудитория включает юзера с правом видеть таблицу")
}

// TestTablesRefresh_ManualEmployeeAdd: зеркало для ручного сотрудника - сигнал по
// employee_target_tables.
func TestTablesRefresh_ManualEmployeeAdd(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	citizenshipID := seedCitizenship(t, db)
	tableID := seedPeopleTable(t, db, "rt_manual_people", "Проход RT")
	scope := fmt.Sprintf("tables:%d", tableID)

	testutil.RegisterAndLogin(t, e, "rt_guard_emp", "pass123", 1, td.OrgID, td.CompanyID)
	guardID := getUserID(t, db, "rt_guard_emp")
	grantTableView(t, db, guardID, "rt_manual_people")
	testutil.RegisterAndLogin(t, e, "rt_creator_emp", "pass123", 1, td.OrgID, td.CompanyID)
	creatorID := getUserID(t, db, "rt_creator_emp")

	fake := &fakePublisher{}
	resolver := services.NewPermissionResolver(db)
	producer := services.NewTablesRefreshPublisher(db, resolver, fake)
	empSvc := services.NewEmployeeService(db, services.NewAuditRecorder(db), services.WithEmployeeTablesProducer(producer))

	_, err := empSvc.CreateManualEmployees(context.Background(), services.ManualEmployeeRequest{
		OrganizationID: td.OrgID,
		TableID:        tableID,
		Employees: []services.ManualEmployee{{
			LastName: "Guardov", FirstName: "Gleb", CitizenshipID: citizenshipID,
			Position: "Loader", PassportSeriesNumber: "4444 555666",
		}},
	}, creatorID)
	require.NoError(t, err)

	audience := findTablesRefresh(fake, scope)
	require.NotNil(t, audience, "ручное добавление сотрудника шлёт tables.refresh целевой таблице")
	assert.Contains(t, audience, guardID, "аудитория включает юзера с правом видеть таблицу")
}
