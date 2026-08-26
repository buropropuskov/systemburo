package handlers_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// grantTableView выдаёт юзеру право table.<name>.view через allow-override, чтобы
// он попал в аудиторию tables.refresh этой таблицы.
func grantTableView(t *testing.T, db *gorm.DB, userID int, tableName string) {
	t.Helper()
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        userID,
		PermissionKey: fmt.Sprintf("table.%s.view", tableName),
		Value:         "allow",
		GrantedAt:     time.Now(),
	}).Error)
}

// reset очищает записи fakePublisher между шагами теста.
func (f *fakePublisher) reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = nil
	f.audiences = nil
}

// findTablesRefresh ищет в записях fakePublisher событие tables.refresh с данным
// scope и возвращает его аудиторию (nil если не найдено).
func findTablesRefresh(fake *fakePublisher, scope string) []int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for i, ev := range fake.events {
		if ev.Type == "tables.refresh" && ev.Scope == scope {
			return fake.audiences[i]
		}
	}
	return nil
}

// TestTablesRefresh_AcceptanceAndCarEntry: принятие заявки с машиной и последующий
// въезд шлют tables.refresh аудитории cars-таблицы (#840 V2.2/V2.3). Машина видна
// только в выбранной таблице «Проезд» (#1036), поэтому сигнал идёт по scope
// tables:<carsTableID> той таблицы, к которой машина привязана passage_tables.
func TestTablesRefresh_AcceptanceAndCarEntry(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Cars-таблица проходной + юзер с правом её видеть.
	dn := "КПП A"
	carsTable := models.SystemTable{Name: "kpp_a", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&carsTable).Error)
	scope := fmt.Sprintf("tables:%d", carsTable.ID)

	testutil.RegisterAndLogin(t, e, "guard_a", "pass123", 1, td.OrgID, td.CompanyID)
	guardID := getUserID(t, db, "guard_a")
	grantTableView(t, db, guardID, "kpp_a")

	// Юзер без права table.kpp_a.view - не должен попасть в аудиторию.
	testutil.RegisterAndLogin(t, e, "guard_nogrant", "pass123", 1, td.OrgID, td.CompanyID)
	nograntID := getUserID(t, db, "guard_nogrant")

	testutil.RegisterAndLogin(t, e, "trtsender", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_tr", "Cars TR")

	fake := &fakePublisher{}
	resolver := services.NewPermissionResolver(db)
	producer := services.NewTablesRefreshPublisher(db, resolver, fake)
	recorder := services.NewAuditRecorder(db)
	appSvc := services.NewApplicationService(
		db,
		services.NewPermissionService(db),
		services.NewNotificationService(db),
		services.NewVehicleBlacklistService(db, recorder),
		services.NewPersonBlacklistService(db, recorder),
		recorder,
		services.WithApplicationTablesProducer(producer),
	)
	carSvc := services.NewCarService(db, recorder, services.WithCarTablesProducer(producer))

	from := "2026-04-01"
	to := "2099-12-31"
	req := services.CompleteApplicationRequest{
		Organization:      "Test Organization",
		ResponsiblePerson: "Test Person",
		ContactPhone:      "+79001234567",
		DataApproval:      true,
		Attachments: []services.AttachmentData{{
			AttachmentType:        "cars",
			AttachmentName:        "cars_template",
			AttachmentDisplayName: "Cars Template",
			UniqueAttachmentID:    uaID,
			EntryDateFrom:         &from,
			EntryDateTo:           &to,
			Data: services.AttachmentContentData{
				Vehicles: &[]services.VehicleInput{{CarNumber: "A003AA777", CarBrand: "Toyota", TargetTables: []int{carsTable.ID}}},
			},
		}},
	}
	created, err := appSvc.SubmitCompleteApplication(context.Background(), "trtsender", req, true)
	require.NoError(t, err)

	// Принятие: активирует машину -> сигнал cars-таблице.
	require.NoError(t, appSvc.UpdateApplicationItemsStatus(context.Background(), created.ApplicationID, "trtsender"))
	audience := findTablesRefresh(fake, scope)
	require.NotNil(t, audience, "принятие заявки с машиной должно послать tables.refresh cars-таблице")
	assert.Contains(t, audience, guardID, "аудитория должна включать юзера с правом table.kpp_a.view")
	assert.NotContains(t, audience, nograntID, "юзер без права table.kpp_a.view не должен попасть в аудиторию")

	// Въезд этой машины -> снова сигнал cars-таблице.
	var carID int
	require.NoError(t, db.Raw(
		`SELECT c.id FROM cars c JOIN attachments a ON a.id = c.attachment_id WHERE a.application_id = ?`,
		created.ApplicationID).Scan(&carID).Error)
	require.NotZero(t, carID)

	fake.reset()
	entry := 1
	require.NoError(t, carSvc.UpdateCarTerritoryStatus(context.Background(), carID, services.UpdateCarTerritoryStatusRequest{
		UpdateTerritoryStatusRequest: services.UpdateTerritoryStatusRequest{
			TerritoryStatus: entry,
			UserID:          &guardID,
			TableID:         &carsTable.ID,
		},
	}))
	audience = findTablesRefresh(fake, scope)
	require.NotNil(t, audience, "въезд машины должен послать tables.refresh cars-таблице")
	assert.Contains(t, audience, guardID)
}

// TestTablesRefresh_EmployeeEntry: въезд сотрудника шлёт tables.refresh его целевой
// people-таблице (#840 V2.3). Сотрудник виден только в своих target-таблицах.
func TestTablesRefresh_EmployeeEntry(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	dn := "Проходная людей"
	peopleTable := models.SystemTable{Name: "kpp_people", DisplayName: &dn, TableType: "people", IsActive: true}
	require.NoError(t, db.Create(&peopleTable).Error)
	scope := fmt.Sprintf("tables:%d", peopleTable.ID)

	testutil.RegisterAndLogin(t, e, "guard_p", "pass123", 1, td.OrgID, td.CompanyID)
	guardID := getUserID(t, db, "guard_p")
	grantTableView(t, db, guardID, "kpp_people")

	// Сотрудник с целевой таблицей (target table) - без заявки, прямой вставкой.
	ln := "Иванов"
	fn := "Иван"
	statusActive := 1
	emp := models.Employee{LastName: &ln, FirstName: &fn, Status: &statusActive}
	require.NoError(t, db.Create(&emp).Error)
	require.NoError(t, db.Create(&models.EmployeeTargetTable{EmployeeID: emp.ID, TableID: peopleTable.ID}).Error)

	fake := &fakePublisher{}
	resolver := services.NewPermissionResolver(db)
	producer := services.NewTablesRefreshPublisher(db, resolver, fake)
	recorder := services.NewAuditRecorder(db)
	empSvc := services.NewEmployeeService(db, recorder, services.WithEmployeeTablesProducer(producer))

	entry := 1
	require.NoError(t, empSvc.UpdateEmployeeTerritoryStatus(context.Background(), emp.ID, services.UpdateTerritoryStatusRequest{
		TerritoryStatus: entry,
		UserID:          &guardID,
		TableID:         &peopleTable.ID,
	}))

	audience := findTablesRefresh(fake, scope)
	require.NotNil(t, audience, "въезд сотрудника должен послать tables.refresh его целевой таблице")
	assert.Contains(t, audience, guardID, "аудитория должна включать юзера с правом table.kpp_people.view")
}
