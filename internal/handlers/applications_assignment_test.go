package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Тесты доназначения постов и мест разгрузки элементам заявки принимающим (#1393).
// Идут через реальные эндпоинты: SQL исполняется, гейты проверяются целиком.

// seedAssignApp создаёт заявку нужного статуса с cars-вложением и одной машиной.
func seedAssignApp(t *testing.T, db *gorm.DB, orgID, senderID int, status, number string) (appID, carID int) {
	t.Helper()
	num := "APP-ASSIGN-" + number
	confirmation := models.ConfirmationApproved
	app := models.Application{
		ApplicationNumber: &num,
		Confirmation:      &confirmation,
		Status:            &status,
		OrganizationID:    orgID,
		SenderUserID:      senderID,
	}
	require.NoError(t, db.Create(&app).Error)

	st := 1
	from, to := "2026-07-01", "2099-12-31"
	att := models.Attachment{
		ApplicationID:  &app.ID,
		AttachmentType: "cars",
		EntryDateFrom:  &from,
		EntryDateTo:    &to,
		Status:         &st,
	}
	require.NoError(t, db.Create(&att).Error)

	car := models.Car{AttachmentID: att.ID, CarNumber: &number, Status: &st}
	require.NoError(t, db.Create(&car).Error)
	return app.ID, car.ID
}

// seedAssignEmployee добавляет к заявке people-вложение с одним сотрудником.
func seedAssignEmployee(t *testing.T, db *gorm.DB, appID int, lastName string) int {
	t.Helper()
	st := 1
	from, to := "2026-07-01", "2099-12-31"
	att := models.Attachment{
		ApplicationID:  &appID,
		AttachmentType: "people",
		EntryDateFrom:  &from,
		EntryDateTo:    &to,
		Status:         &st,
	}
	require.NoError(t, db.Create(&att).Error)

	ln := lastName
	emp := models.Employee{AttachmentID: &att.ID, LastName: &ln, Status: &st}
	require.NoError(t, db.Create(&emp).Error)
	return emp.ID
}

func carTableIDs(t *testing.T, db *gorm.DB, carID int) []int {
	t.Helper()
	var ids []int
	require.NoError(t, db.Raw("SELECT table_id FROM car_target_tables WHERE car_id = ? ORDER BY table_id", carID).Scan(&ids).Error)
	return ids
}

// Принимающий добавляет пост машине: привязка появляется с источником approver,
// прежние посты из заявки остаются на месте.
func TestAssignTables_AddKeepsApplicationOnesAndMarksSource(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	fromApp := seedCarsTable(t, db, "assign_app_post", "Пост из заявки")
	extra := seedCarsTable(t, db, "assign_extra_post", "Пост принимающего")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "A111AA777")
	require.NoError(t, db.Create(&models.CarTargetTable{CarID: carID, TableID: fromApp, Source: "application"}).Error)

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"table_ids":[%d],"mode":"add"}`, carID, extra)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "назначение: %s", rec.Body.String())

	assert.ElementsMatch(t, []int{fromApp, extra}, carTableIDs(t, db, carID), "к посту из заявки добавился новый")

	var source string
	require.NoError(t, db.Raw("SELECT source FROM car_target_tables WHERE car_id = ? AND table_id = ?", carID, extra).Scan(&source).Error)
	assert.Equal(t, "approver", source, "добавленное принимающим помечено источником")

	var fromAppSource string
	require.NoError(t, db.Raw("SELECT source FROM car_target_tables WHERE car_id = ? AND table_id = ?", carID, fromApp).Scan(&fromAppSource).Error)
	assert.Equal(t, "application", fromAppSource, "источник строки из заявки не переписан")
}

// Режим replace снимает лишнее: набор становится ровно тем, что передали.
func TestAssignTables_ReplaceRemovesMissingOnes(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	first := seedCarsTable(t, db, "assign_replace_a", "Пост A")
	second := seedCarsTable(t, db, "assign_replace_b", "Пост Б")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "A222AA777")
	require.NoError(t, db.Create(&models.CarTargetTable{CarID: carID, TableID: first, Source: "application"}).Error)

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"table_ids":[%d],"mode":"replace"}`, carID, second)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "замена: %s", rec.Body.String())

	assert.Equal(t, []int{second}, carTableIDs(t, db, carID), "остался только переданный пост")
}

// Пустой список в режиме replace снимает все посты: машина перестаёт быть видна
// на проходной, но действие разрешено - иногда именно это и нужно.
func TestAssignTables_ReplaceWithEmptyClearsAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	tableID := seedCarsTable(t, db, "assign_clear", "Пост")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "A333AA777")
	require.NoError(t, db.Create(&models.CarTargetTable{CarID: carID, TableID: tableID, Source: "application"}).Error)

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"table_ids":[],"mode":"replace"}`, carID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "очистка: %s", rec.Body.String())

	assert.Empty(t, carTableIDs(t, db, carID), "посты сняты")
}

// Одним запросом можно назначить пост всем машинам вложения.
func TestAssignTables_AppliesToSeveralElementsAtOnce(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	tableID := seedCarsTable(t, db, "assign_all", "Общий пост")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, firstCar := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "A444AA777")

	st := 1
	number := "A555AA777"
	var att models.Attachment
	require.NoError(t, db.Where("application_id = ?", appID).First(&att).Error)
	secondCar := models.Car{AttachmentID: att.ID, CarNumber: &number, Status: &st}
	require.NoError(t, db.Create(&secondCar).Error)

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d,%d],"table_ids":[%d],"mode":"add"}`, firstCar, secondCar.ID, tableID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "назначить всем: %s", rec.Body.String())

	assert.Equal(t, []int{tableID}, carTableIDs(t, db, firstCar))
	assert.Equal(t, []int{tableID}, carTableIDs(t, db, secondCar.ID))
}

// Сотрудникам посты прохода назначаются тем же эндпоинтом.
func TestAssignTables_WorksForEmployees(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	tableID := seedCarsTable(t, db, "assign_people", "Проходная")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, _ := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "A666AA777")
	empID := seedAssignEmployee(t, db, appID, "Иванов")

	body := fmt.Sprintf(`{"element_type":"people","element_ids":[%d],"table_ids":[%d],"mode":"add"}`, empID, tableID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "сотрудник: %s", rec.Body.String())

	var ids []int
	require.NoError(t, db.Raw("SELECT table_id FROM employee_target_tables WHERE employee_id = ?", empID).Scan(&ids).Error)
	assert.Equal(t, []int{tableID}, ids)
}

// Не принимающий назначать не может, даже будучи админом заявки.
func TestAssignTables_ForbiddenForNonApprover(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	tableID := seedCarsTable(t, db, "assign_forbidden", "Пост")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "A777AA777")

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"table_ids":[%d],"mode":"add"}`, carID, tableID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code, "назначает только принимающий")
	assert.Empty(t, carTableIDs(t, db, carID))
}

// У закрытой заявки набор мест уже не меняют.
func TestAssignTables_RejectedForClosedApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	tableID := seedCarsTable(t, db, "assign_closed", "Пост")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusCompleted, "A888AA777")

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"table_ids":[%d],"mode":"add"}`, carID, tableID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "по завершённой заявке назначение запрещено")
	assert.Empty(t, carTableIDs(t, db, carID))
}

// Чужую машину через id из другой заявки не тронуть.
func TestAssignTables_RejectsElementFromAnotherApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	tableID := seedCarsTable(t, db, "assign_foreign", "Пост")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, _ := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "A999AA777")
	_, foreignCar := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "B111BB777")

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"table_ids":[%d],"mode":"add"}`, foreignCar, tableID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "элемент чужой заявки отвергнут")
	assert.Empty(t, carTableIDs(t, db, foreignCar), "чужая машина не тронута")
}

// Места разгрузки назначаются своим эндпоинтом и пишутся в историю машины.
func TestAssignUnloadPlaces_AddAndHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	placeID := seedPlace(t, db, "Рампа принимающего")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "C111CC777")

	body := fmt.Sprintf(`{"car_ids":[%d],"place_ids":[%d],"mode":"add"}`, carID, placeID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/unload-places", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "места: %s", rec.Body.String())

	var ids []int
	require.NoError(t, db.Raw("SELECT unload_place_id FROM car_unload_places WHERE car_id = ?", carID).Scan(&ids).Error)
	assert.Equal(t, []int{placeID}, ids)

	rec = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	history := testutil.ParseSlice(t, rec)
	found := false
	for _, h := range history {
		if field, ok := h["field_name"].(string); ok && field == "unload_places" {
			found = true
			assert.Contains(t, fmt.Sprint(h["new_value"]), "Рампа принимающего", "в истории видно новое место")
		}
	}
	assert.True(t, found, "смена мест разгрузки попала в историю машины")
}

// Пост, отключённый уже после подачи заявки, не мешает править соседние: он
// приходит в наборе как есть, а запрещено только назначать отключённое заново.
func TestAssignTables_KeepsAlreadyLinkedInactiveTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	stale := seedCarsTable(t, db, "assign_stale", "Отключённый после подачи")
	fresh := seedCarsTable(t, db, "assign_fresh", "Рабочий пост")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "E111EE777")
	require.NoError(t, db.Create(&models.CarTargetTable{CarID: carID, TableID: stale, Source: "application"}).Error)
	require.NoError(t, db.Exec("UPDATE system_tables SET is_active = false WHERE id = ?", stale).Error)

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"table_ids":[%d,%d],"mode":"replace"}`, carID, stale, fresh)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "уже привязанный отключённый пост не блокирует правку: %s", rec.Body.String())

	assert.ElementsMatch(t, []int{stale, fresh}, carTableIDs(t, db, carID), "старая привязка осталась, новая добавилась")
}

// Отключённый пост назначить нельзя - иначе машина уедет туда, где не пропустят.
func TestAssignTables_RejectsInactiveTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	tableID := seedCarsTable(t, db, "assign_inactive", "Отключённый пост")
	require.NoError(t, db.Exec("UPDATE system_tables SET is_active = false WHERE id = ?", tableID).Error)

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "D111DD777")

	body := fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"table_ids":[%d],"mode":"add"}`, carID, tableID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d/elements/tables", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "неактивный пост отвергнут")
	assert.Empty(t, carTableIDs(t, db, carID))
}
