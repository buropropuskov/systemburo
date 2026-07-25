package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemTables_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/system-tables", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSystemTables_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create
	body := `{"name":"test_cars_table","display_name":"Test Cars Table","table_type":"cars"}`
	rec := testutil.POST(t, e, "/system-tables", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	assert.NotNil(t, createResp["id"])
	tableID := int(createResp["id"].(float64))
	assert.Greater(t, tableID, 0)

	// Get by ID
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	getResp := testutil.ParseMap(t, rec)
	table := getResp["table"].(map[string]interface{})
	assert.Equal(t, "test_cars_table", table["name"])
	assert.Equal(t, "Test Cars Table", table["display_name"])
	assert.Equal(t, "cars", table["table_type"])

	// Verify default fields were created for "cars" type
	fields := getResp["fields"].([]interface{})
	assert.Greater(t, len(fields), 0, "expected default fields for cars table type")

	// Get All
	rec = testutil.GET(t, e, "/system-tables", h)
	require.Equal(t, http.StatusOK, rec.Code)

	listResp := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(listResp), 1)

	// Get by Name
	rec = testutil.GET(t, e, "/system-tables/name/test_cars_table", h)
	require.Equal(t, http.StatusOK, rec.Code)

	nameResp := testutil.ParseMap(t, rec)
	nameTable := nameResp["table"].(map[string]interface{})
	assert.Equal(t, "test_cars_table", nameTable["name"])

	// Update
	updateBody := `{"display_name":"Updated Cars Table","status":"maintenance"}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d", tableID), updateBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify update
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	getResp = testutil.ParseMap(t, rec)
	table = getResp["table"].(map[string]interface{})
	assert.Equal(t, "Updated Cars Table", table["display_name"])
	assert.Equal(t, "maintenance", table["status"])

	// Delete (soft delete -- sets is_active=false)
	rec = testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify soft-deleted (not found because GetByID checks is_active=true)
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSystemTables_ArchiveAndRestore(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create table
	body := `{"name":"arch_table","display_name":"Arch Table","table_type":"cars"}`
	rec := testutil.POST(t, e, "/system-tables", body, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Archive via DELETE (soft delete)
	rec = testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Default GetAll - архивная таблица не должна быть в списке
	rec = testutil.GET(t, e, "/system-tables", h)
	require.Equal(t, http.StatusOK, rec.Code)
	for _, m := range testutil.ParseSlice(t, rec) {
		tbl := m["table"].(map[string]interface{})
		assert.NotEqual(t, float64(tableID), tbl["id"], "archived table must NOT be in active list")
	}

	// GetAll?include_archived=true - архивная таблица ДОЛЖНА быть
	rec = testutil.GET(t, e, "/system-tables?include_archived=true", h)
	require.Equal(t, http.StatusOK, rec.Code)
	found := false
	for _, m := range testutil.ParseSlice(t, rec) {
		tbl := m["table"].(map[string]interface{})
		if int(tbl["id"].(float64)) == tableID {
			found = true
			assert.Equal(t, false, tbl["is_active"], "include_archived must return is_active=false rows")
			break
		}
	}
	assert.True(t, found, "archived table must appear in include_archived=true list")

	// Restore - возвращаем из архива
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/restore", tableID), "", h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Повторный restore - 400 (уже не в архиве)
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/restore", tableID), "", h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// После restore таблица снова в активном списке
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Restore несуществующей - 404
	rec = testutil.POST(t, e, "/system-tables/999999/restore", "", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestSystemTables_UsageAndDetachAll: usage показывает привязки, они блокируют
// удаление, «Отвязать всё» их снимает (с аудитом на каждую орг/компанию) и после
// этого таблица архивируется. Повтор detach-all по пустому - идемпотентен.
func TestSystemTables_UsageAndDetachAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"name":"otvyaz_table","display_name":"Отвяз-Таблица","table_type":"cars"}`
	tableID := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", body, h))["id"].(float64))
	require.NoError(t, db.Create(&models.OrganizationTable{OrganizationID: td.OrgID, TableID: tableID}).Error)
	require.NoError(t, db.Create(&models.CompaniesTable{CompanyID: td.CompanyID, TableID: tableID}).Error)

	// usage перечисляет обе привязки
	usage := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/usage", tableID), h))
	orgs := usage["organizations"].([]interface{})
	comps := usage["companies"].([]interface{})
	require.Len(t, orgs, 1)
	require.Len(t, comps, 1)
	assert.Equal(t, float64(td.OrgID), orgs[0].(map[string]interface{})["id"])
	assert.Equal(t, true, orgs[0].(map[string]interface{})["is_active"])
	assert.NotEmpty(t, orgs[0].(map[string]interface{})["name"])

	// пока привязано - удаление заблокировано (400)
	assert.Equal(t, http.StatusBadRequest, testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d", tableID), h).Code)

	// detach-all снимает обе привязки
	detach := testutil.ParseMap(t, testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/detach-all", tableID), "", h))
	assert.Equal(t, float64(1), detach["organizations_detached"])
	assert.Equal(t, float64(1), detach["companies_detached"])

	// аудит «таблица убрана» записан на организацию и компанию (removed = display_name)
	assert.Contains(t, auditDetails(t, db, models.AuditEntityOrganization, td.OrgID, models.OrganizationActionTablesChanged), "Отвяз-Таблица")
	assert.Contains(t, auditDetails(t, db, models.AuditEntityCompany, td.CompanyID, models.CompanyActionTablesChanged), "Отвяз-Таблица")

	// usage теперь пуст
	usage = testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/usage", tableID), h))
	assert.Empty(t, usage["organizations"].([]interface{}))
	assert.Empty(t, usage["companies"].([]interface{}))

	// теперь таблица архивируется
	assert.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d", tableID), h).Code)

	// повторный detach-all идемпотентен (нулевые счётчики, 200)
	detach = testutil.ParseMap(t, testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/detach-all", tableID), "", h))
	assert.Equal(t, float64(0), detach["organizations_detached"])
	assert.Equal(t, float64(0), detach["companies_detached"])
}

// TestSystemTables_Usage_ArchivedBindingVisible: архивная (is_active=false)
// организация всё равно попадает в usage - она держит таблицу (гейт Delete
// считает по junction без фильтра активности), поэтому оператор обязан её видеть.
func TestSystemTables_Usage_ArchivedBindingVisible(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"name":"arch_bind_table","display_name":"Арх-Таблица","table_type":"cars"}`
	tableID := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", body, h))["id"].(float64))
	require.NoError(t, db.Create(&models.OrganizationTable{OrganizationID: td.OrgID, TableID: tableID}).Error)
	require.NoError(t, db.Table("organizations").Where("id = ?", td.OrgID).Update("is_active", false).Error)

	usage := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/usage", tableID), h))
	orgs := usage["organizations"].([]interface{})
	require.Len(t, orgs, 1)
	assert.Equal(t, false, orgs[0].(map[string]interface{})["is_active"], "архивная организация должна быть видна в usage")
}

func TestSystemTables_Usage_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	assert.Equal(t, http.StatusNotFound, testutil.GET(t, e, "/system-tables/99999/usage", h).Code)
	assert.Equal(t, http.StatusNotFound, testutil.POST(t, e, "/system-tables/99999/detach-all", "", h).Code)
}

// TestSystemTables_DetachAll_Forbidden: detach-all под admin-гейтом (меняет
// привязки орг/компаний), обычному пользователю - 403.
func TestSystemTables_DetachAll_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	body := `{"name":"gate_table","display_name":"Гейт-Таблица","table_type":"cars"}`
	tableID := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", body, testutil.AuthHeader(adminToken)))["id"].(float64))

	userToken := testutil.RegisterAndLogin(t, e, "tabledetachuser", "pass123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/detach-all", tableID), "", testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestSystemTables_DetachOneOrgAndCompany: точечная отвязка снимает ОДНУ
// конкретную привязку, остальные остаются; аудит на затронутую сущность; повтор
// по уже снятой идемпотентен (detached:false, 200).
func TestSystemTables_DetachOneOrgAndCompany(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	body := `{"name":"tochtable","display_name":"Точ-Таблица","table_type":"cars"}`
	tableID := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", body, h))["id"].(float64))
	require.NoError(t, db.Create(&models.OrganizationTable{OrganizationID: td.OrgID, TableID: tableID}).Error)
	require.NoError(t, db.Create(&models.CompaniesTable{CompanyID: td.CompanyID, TableID: tableID}).Error)

	// Отвязываем ТОЛЬКО организацию - компания остаётся.
	detach := testutil.ParseMap(t, testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/organizations/%d", tableID, td.OrgID), h))
	assert.Equal(t, true, detach["detached"])
	assert.Contains(t, auditDetails(t, db, models.AuditEntityOrganization, td.OrgID, models.OrganizationActionTablesChanged), "Точ-Таблица")

	usage := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/usage", tableID), h))
	assert.Empty(t, usage["organizations"].([]interface{}), "организация снята")
	require.Len(t, usage["companies"].([]interface{}), 1, "компания осталась")

	// Повтор по уже снятой организации идемпотентен.
	detach = testutil.ParseMap(t, testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/organizations/%d", tableID, td.OrgID), h))
	assert.Equal(t, false, detach["detached"])

	// Отвязываем компанию - таблица свободна, архивируется.
	detach = testutil.ParseMap(t, testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/companies/%d", tableID, td.CompanyID), h))
	assert.Equal(t, true, detach["detached"])
	assert.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d", tableID), h).Code)
}

// TestSystemTables_DetachOne_ForbiddenAndNotFound: точечная отвязка под admin-
// гейтом (403 обычному), несуществующая таблица -> 404.
func TestSystemTables_DetachOne_ForbiddenAndNotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	body := `{"name":"gatetochtable","display_name":"Гейт-Точ-Табл","table_type":"cars"}`
	tableID := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", body, adminH))["id"].(float64))
	require.NoError(t, db.Create(&models.OrganizationTable{OrganizationID: td.OrgID, TableID: tableID}).Error)

	userH := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "tabledetachoneuser", "pass123", 1, td.OrgID, td.CompanyID))
	assert.Equal(t, http.StatusForbidden, testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/organizations/%d", tableID, td.OrgID), userH).Code)

	assert.Equal(t, http.StatusNotFound, testutil.DELETE(t, e, fmt.Sprintf("/system-tables/99999/organizations/%d", td.OrgID), adminH).Code)
	assert.Equal(t, http.StatusNotFound, testutil.DELETE(t, e, fmt.Sprintf("/system-tables/99999/companies/%d", td.CompanyID), adminH).Code)
}

func TestSystemTables_History(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create -> +1 history entry (created)
	body := `{"name":"hist_table","display_name":"Hist Table","table_type":"cars"}`
	rec := testutil.POST(t, e, "/system-tables", body, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Update -> +1 history entry (updated)
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d", tableID),
		`{"display_name":"Hist Table v2"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Delete -> +1 (archived)
	rec = testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Restore -> +1 (restored)
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/restore", tableID), "", h)
	require.Equal(t, http.StatusOK, rec.Code)

	// GET history - 4 записи в порядке убывания времени.
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/history", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	items := testutil.ParseSlice(t, rec)
	require.Len(t, items, 4)

	// Самая свежая запись - restored, дальше archived, updated, created.
	wantActions := []string{"restored", "archived", "updated", "created"}
	for i, want := range wantActions {
		assert.Equal(t, want, items[i]["action_type"], "history[%d].action_type", i)
		assert.NotEmpty(t, items[i]["user_name"], "history[%d].user_name", i)
	}
}

func TestSystemTables_DuplicateName(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"name":"dup_table","display_name":"Dup Table","table_type":"cars"}`
	rec := testutil.POST(t, e, "/system-tables", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Same name again
	rec = testutil.POST(t, e, "/system-tables", body, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSystemTables_GetByName_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/system-tables/name/nonexistent", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Просмотр версий открывается для архивной таблицы: резолвер по имени должен
// находить её только с allow_archived, иначе кнопка "Версии" из архива ведёт
// в "Таблица не найдена".
func TestSystemTables_GetByName_AllowArchived(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"arch_cars","display_name":"Архивная","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Мягко удаляем (is_active=false) - таблица уходит в архив.
	rec = testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Без флага архивная не резолвится - как основная страница таблицы.
	rec = testutil.GET(t, e, "/system-tables/name/arch_cars", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// С флагом - находится, is_active=false (нужно для страницы версий).
	rec = testutil.GET(t, e, "/system-tables/name/arch_cars?allow_archived=1", h)
	require.Equal(t, http.StatusOK, rec.Code)
	tbl := testutil.ParseMap(t, rec)["table"].(map[string]interface{})
	assert.Equal(t, "arch_cars", tbl["name"])
	assert.Equal(t, false, tbl["is_active"])
}

func TestSystemTables_PeopleType_DefaultFields(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"name":"test_people_table","display_name":"Test People","table_type":"people"}`
	rec := testutil.POST(t, e, "/system-tables", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	tableID := int(createResp["id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	getResp := testutil.ParseMap(t, rec)
	fields := getResp["fields"].([]interface{})
	assert.Greater(t, len(fields), 0, "expected default fields for people table type")
}

func TestSystemTables_TimeSlots_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create table first
	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"slot_test_table","display_name":"Slot Test","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	cr := testutil.ParseMap(t, rec)
	tableID := int(cr["id"].(float64))

	// Add time slot
	slotBody := `{"day_of_week":1,"open_time":"09:00","close_time":"18:00"}`
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/time-slots", tableID), slotBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	slotResp := testutil.ParseMap(t, rec)
	slotID := int(slotResp["id"].(float64))
	assert.Greater(t, slotID, 0)

	// Get time slots
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/time-slots", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	slots := testutil.ParseSlice(t, rec)
	assert.Len(t, slots, 1)
	assert.Equal(t, float64(1), slots[0]["day_of_week"])
	assert.Equal(t, "09:00", slots[0]["open_time"])

	// Update time slot
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/time-slots/%d", tableID, slotID),
		`{"open_time":"10:00","is_active":false}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify update
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/time-slots", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	slots = testutil.ParseSlice(t, rec)
	assert.Equal(t, "10:00", slots[0]["open_time"])
	assert.Equal(t, false, slots[0]["is_active"])

	// Delete time slot
	rec = testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/time-slots/%d", tableID, slotID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify empty
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/time-slots", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	slots = testutil.ParseSlice(t, rec)
	assert.Len(t, slots, 0)
}

func TestSystemTables_TimeSlots_TableNotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables/99999/time-slots",
		`{"day_of_week":0,"open_time":"08:00","close_time":"17:00"}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSystemTables_TimeSlots_InvalidTime(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"invalid_time_table","display_name":"IT","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	cr := testutil.ParseMap(t, rec)
	tableID := int(cr["id"].(float64))

	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/time-slots", tableID),
		`{"day_of_week":0,"open_time":"bad","close_time":"17:00"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/time-slots", tableID),
		`{"day_of_week":8,"open_time":"08:00","close_time":"17:00"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSystemTables_ResponseStructure(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"struct_test","display_name":"ST","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	cr := testutil.ParseMap(t, rec)
	tableID := int(cr["id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	details := testutil.ParseMap(t, rec)

	assert.Contains(t, details, "table")
	assert.Contains(t, details, "fields")
	assert.Contains(t, details, "time_slots")
	assert.Contains(t, details, "photos")
	assert.Contains(t, details, "current_status")
}

func TestSystemTables_UpdateFields_TogglesVisibility(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"fields_test","display_name":"FT","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// PUT /system-tables/:id/fields - скрываем status и unload_place
	body := `{"fields":[{"field_name":"status","is_visible":false},{"field_name":"unload_place","is_visible":false}]}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/fields", tableID), body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Re-fetch и проверяем, что is_visible поменялся ровно для двух полей
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	fields := testutil.ParseMap(t, rec)["fields"].([]interface{})

	visibility := map[string]bool{}
	for _, f := range fields {
		fm := f.(map[string]interface{})
		visibility[fm["field_name"].(string)] = fm["is_visible"].(bool)
	}
	assert.False(t, visibility["status"], "status должен быть скрыт")
	assert.False(t, visibility["unload_place"], "unload_place должен быть скрыт")
	assert.True(t, visibility["car_number"], "car_number должен остаться видимым")
	assert.True(t, visibility["car_brand"], "car_brand должен остаться видимым")
}

func TestSystemTables_UpdateFields_UnknownTable_404(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"fields":[{"field_name":"car_number","is_visible":false}]}`
	rec := testutil.PUT(t, e, "/system-tables/999999/fields", body, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSystemTables_UpdateFields_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	body := `{"fields":[{"field_name":"car_number","is_visible":false}]}`
	rec := testutil.PUT(t, e, "/system-tables/1/fields", body, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSystemTables_UpdateFields_PersistsDisplayOrder(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"order_test","display_name":"OT","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Меняем порядок: car_brand -> 0, car_number -> 5 (всё остальное оставляем без изменений).
	body := `{"fields":[
		{"field_name":"car_brand","is_visible":true,"display_order":0},
		{"field_name":"car_number","is_visible":true,"display_order":5}
	]}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/fields", tableID), body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	fields := testutil.ParseMap(t, rec)["fields"].([]interface{})

	orderByName := map[string]int{}
	for _, f := range fields {
		fm := f.(map[string]interface{})
		if order, ok := fm["display_order"].(float64); ok {
			orderByName[fm["field_name"].(string)] = int(order)
		}
	}
	assert.Equal(t, 0, orderByName["car_brand"], "car_brand должен иметь display_order 0")
	assert.Equal(t, 5, orderByName["car_number"], "car_number должен иметь display_order 5")
}

func TestSystemTables_UpdateFields_PersistsWidth(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"width_test","display_name":"WT","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Меняем ширину для car_number и organization.
	body := `{"fields":[
		{"field_name":"car_number","is_visible":true,"width":20},
		{"field_name":"organization","is_visible":true,"width":30}
	]}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/fields", tableID), body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	fields := testutil.ParseMap(t, rec)["fields"].([]interface{})

	widthByName := map[string]int{}
	for _, f := range fields {
		fm := f.(map[string]interface{})
		if w, ok := fm["width"].(float64); ok {
			widthByName[fm["field_name"].(string)] = int(w)
		}
	}
	assert.Equal(t, 20, widthByName["car_number"], "car_number ширина 20")
	assert.Equal(t, 30, widthByName["organization"], "organization ширина 30")
	// car_brand не трогали - должен сохранить дефолт (9).
	assert.Equal(t, 9, widthByName["car_brand"], "car_brand остаётся 9 (дефолт)")
}

// TestSystemTables_UpdateFields_PersistsPriority - #345 Phase 1F:
// PUT /fields сохраняет priority в БД, не задетые поля сохраняют дефолт каталога.
func TestSystemTables_UpdateFields_PersistsPriority(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"prio_test","display_name":"PT","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	body := `{"fields":[
		{"field_name":"car_brand","is_visible":true,"priority":2},
		{"field_name":"organization","is_visible":true,"priority":5}
	]}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/fields", tableID), body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	fields := testutil.ParseMap(t, rec)["fields"].([]interface{})

	prioByName := map[string]int{}
	for _, f := range fields {
		fm := f.(map[string]interface{})
		if p, ok := fm["priority"].(float64); ok {
			prioByName[fm["field_name"].(string)] = int(p)
		}
	}
	assert.Equal(t, 2, prioByName["car_brand"], "car_brand priority=2")
	assert.Equal(t, 5, prioByName["organization"], "organization priority=5")
	// car_number не трогали - дефолт каталога = 1.
	assert.Equal(t, 1, prioByName["car_number"], "car_number priority=1 (дефолт)")
}

// TestSystemTables_UpdateFields_PriorityOutOfRange - валидация priority 1-5.
func TestSystemTables_UpdateFields_PriorityOutOfRange(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"prio_bad","display_name":"PB","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	body := `{"fields":[{"field_name":"car_number","is_visible":true,"priority":9}]}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/fields", tableID), body, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestSystemTables_Update_PersistsAppearance - #345 Phase 1D+1E:
// PUT /system-tables/:id сохраняет font_size и row_density.
func TestSystemTables_Update_PersistsAppearance(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"style_test","display_name":"ST","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d", tableID),
		`{"font_size":18,"row_density":"spacious"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	table := testutil.ParseMap(t, rec)["table"].(map[string]interface{})
	assert.EqualValues(t, 18, table["font_size"], "font_size=18")
	assert.Equal(t, "spacious", table["row_density"], "row_density=spacious")
}

// TestSystemTables_Update_FontSizeOutOfRange - валидация 10-24.
func TestSystemTables_Update_FontSizeOutOfRange(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"fs_bad","display_name":"FB","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d", tableID),
		`{"font_size":30}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestSystemTables_Update_BadRowDensity - валидация enum row_density.
func TestSystemTables_Update_BadRowDensity(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"den_bad","display_name":"DB","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d", tableID),
		`{"row_density":"huge"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestSystemTables_UpdateFactFields_PersistsVisibility - #345 PR-B:
// PUT /:id/fact-fields сохраняет видимость в table_field_facts. Существующие
// fact-поля создаются при включении show_fact_table.
func TestSystemTables_UpdateFactFields_PersistsVisibility(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"fact_vis","display_name":"FV","table_type":"cars","show_fact_table":true}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Скрываем organization, показываем car_number.
	body := `{"fields":[
		{"field_name":"organization","is_visible":false},
		{"field_name":"car_number","is_visible":true}
	]}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/fact-fields", tableID), body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	factFields := testutil.ParseMap(t, rec)["fact_fields"].([]interface{})

	visByName := map[string]bool{}
	for _, f := range factFields {
		fm := f.(map[string]interface{})
		visByName[fm["field_name"].(string)] = fm["is_visible"].(bool)
	}
	assert.False(t, visByName["organization"], "organization скрыта")
	assert.True(t, visByName["car_number"], "car_number видима")
	// Поле, которое не трогали, сохранило дефолт каталога (car_brand=visible).
	assert.True(t, visByName["car_brand"], "car_brand видимо (дефолт)")
}

// TestSystemTables_Update_PersistsAppearanceFact - валидация и сохранение
// font_size_fact и row_density_fact.
func TestSystemTables_Update_PersistsAppearanceFact(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"fact_style","display_name":"FS","table_type":"cars","show_fact_table":true}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d", tableID),
		`{"font_size_fact":20,"row_density_fact":"compact"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	tbl := testutil.ParseMap(t, rec)["table"].(map[string]interface{})
	assert.EqualValues(t, 20, tbl["font_size_fact"], "font_size_fact=20")
	assert.Equal(t, "compact", tbl["row_density_fact"], "row_density_fact=compact")
	// Обычное оформление не изменилось.
	assert.EqualValues(t, 14, tbl["font_size"], "font_size остался 14")
	assert.Equal(t, "normal", tbl["row_density"], "row_density остался normal")
}

// TestSystemTables_Update_FactFontSizeOutOfRange - валидация 10-24 для fact-варианта.
func TestSystemTables_Update_FactFontSizeOutOfRange(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"fact_fs_bad","display_name":"FF","table_type":"cars","show_fact_table":true}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d", tableID),
		`{"font_size_fact":50}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestSystemTables_FactFields_DefaultVisibilityFromCatalog - регрессия:
// при включении show_fact_table факт-поля сохраняют is_visible из каталога
// (часть видна, часть скрыта). Без Select("*") в seedFactFields все скрытые
// поля уезжали в visible=true из-за GORM default tag.
func TestSystemTables_FactFields_DefaultVisibilityFromCatalog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"fact_def_vis","display_name":"FV","table_type":"cars","show_fact_table":true}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	factFields := testutil.ParseMap(t, rec)["fact_fields"].([]interface{})

	visByName := map[string]bool{}
	for _, f := range factFields {
		fm := f.(map[string]interface{})
		visByName[fm["field_name"].(string)] = fm["is_visible"].(bool)
	}
	assert.True(t, visByName["organization"], "organization видим (каталог)")
	assert.True(t, visByName["car_brand"], "car_brand видим (каталог)")
	assert.True(t, visByName["valid_until"], "valid_until видим (каталог)")
	assert.True(t, visByName["time_range"], "time_range видим (каталог)")
	assert.False(t, visByName["car_number"], "car_number скрыт (каталог)")
	assert.False(t, visByName["unload_place"], "unload_place скрыт (каталог)")
	assert.False(t, visByName["status"], "status скрыт (каталог)")
	assert.False(t, visByName["company"], "company скрыт (каталог)")
	assert.False(t, visByName["application_id"], "application_id скрыт (каталог)")
}

// fact_table_hint редактируется тем же rich-text TextConstructor, что и instruction:
// HTML-обёртки форматирования легко переваливают за старый лимит varchar(255), и запись
// падала с "value too long" (юзер ловил это на "привет -" - длина пересекала границу).
// После перевода колонки в text длинная форматированная подсказка должна сохраняться.
func TestSystemTables_FactTableHint_LongFormattedHTML(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"fact_hint_long","display_name":"FH","table_type":"cars","show_fact_table":true}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	hint := `<span class="font-size-16">` + strings.Repeat("привет - ", 50) + `</span>`
	require.Greater(t, len(hint), 255, "подсказка должна быть длиннее старого лимита")
	body, err := json.Marshal(map[string]string{"fact_table_hint": hint})
	require.NoError(t, err)

	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d", tableID), string(body), h)
	require.Equal(t, http.StatusOK, rec.Code, "длинная форматированная подсказка должна сохраняться")

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	table := testutil.ParseMap(t, rec)["table"].(map[string]interface{})
	assert.Equal(t, hint, table["fact_table_hint"], "подсказка round-trip без обрезки")
}

// TestSystemTables_Warning_RoundTrip проверяет, что свободное предупреждение
// (#1183) сохраняется при создании и обновлении, возвращается в DTO таблицы
// и попадает в history-детали (buildUpdateDetails).
func TestSystemTables_Warning_RoundTrip(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create с warning
	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"warn_table","display_name":"Warn Table","table_type":"cars","warning":"Проезд закрыт 12:00-13:00"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// GET отдаёт warning
	table := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h))["table"].(map[string]interface{})
	assert.Equal(t, "Проезд закрыт 12:00-13:00", table["warning"])

	// Update меняет warning
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d", tableID), `{"warning":"Новое предупреждение"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	table = testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h))["table"].(map[string]interface{})
	assert.Equal(t, "Новое предупреждение", table["warning"])

	// warning из update попал в history-детали (buildUpdateDetails)
	items := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/history", tableID), h))
	require.GreaterOrEqual(t, len(items), 1)
	assert.Equal(t, "updated", items[0]["action_type"])
	assert.Equal(t, "Новое предупреждение", items[0]["details"].(map[string]interface{})["warning"])
}

func TestSystemTables_WarningWindows_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create table first
	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"warn_win_table","display_name":"Warn Win","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Add a windowed warning (Пн 12:00-13:00 малогабарит)
	body := `{"day_of_week":1,"time_from":"12:00","time_to":"13:00","message":"Только малогабарит"}`
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/warning-windows", tableID), body, h)
	require.Equal(t, http.StatusOK, rec.Code)
	windowID := int(testutil.ParseMap(t, rec)["id"].(float64))
	assert.Greater(t, windowID, 0)

	// Get warning windows
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/warning-windows", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	windows := testutil.ParseSlice(t, rec)
	require.Len(t, windows, 1)
	assert.Equal(t, float64(1), windows[0]["day_of_week"])
	assert.Equal(t, "12:00", windows[0]["time_from"])
	assert.Equal(t, "13:00", windows[0]["time_to"])
	assert.Equal(t, "Только малогабарит", windows[0]["message"])
	assert.Equal(t, true, windows[0]["is_active"])

	// Warning windows are embedded in the table detail DTO
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	detail := testutil.ParseMap(t, rec)
	embedded, ok := detail["warning_windows"].([]interface{})
	require.True(t, ok, "warning_windows должно присутствовать в DTO таблицы")
	assert.Len(t, embedded, 1)

	// Update to a general warning (каждый день / весь день) -- nullable поля -> NULL
	updateBody := `{"day_of_week":null,"time_from":null,"time_to":null,"message":"Пропуск оформляется заранее"}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/system-tables/%d/warning-windows/%d", tableID, windowID), updateBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify NULL round-trip: сброс дня/времени в NULL реально доезжает до БД
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/warning-windows", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	windows = testutil.ParseSlice(t, rec)
	require.Len(t, windows, 1)
	assert.Nil(t, windows[0]["day_of_week"], "day_of_week должен сброситься в NULL (каждый день)")
	assert.Nil(t, windows[0]["time_from"], "time_from должен сброситься в NULL (весь день)")
	assert.Nil(t, windows[0]["time_to"])
	assert.Equal(t, "Пропуск оформляется заранее", windows[0]["message"])

	// Delete
	rec = testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d/warning-windows/%d", tableID, windowID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/warning-windows", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	windows = testutil.ParseSlice(t, rec)
	assert.Len(t, windows, 0)
}

func TestSystemTables_WarningWindows_Validation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"warn_win_valid","display_name":"Warn Valid","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Пустой message -> 400
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/warning-windows", tableID),
		`{"message":""}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Неверный формат времени -> 400
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/warning-windows", tableID),
		`{"time_from":"invalid","message":"текст"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Неверный день недели -> 400
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/warning-windows", tableID),
		`{"day_of_week":7,"message":"текст"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSystemTables_WarningWindows_TableNotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables/99999/warning-windows",
		`{"message":"текст"}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// Окно можно добавить только к активной таблице (checkParent требует is_active=true,
// в отличие от мест разгрузки). Архивированная таблица -> 404.
func TestSystemTables_WarningWindows_ArchivedTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/system-tables",
		`{"name":"warn_win_archived","display_name":"Warn Archived","table_type":"cars"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	tableID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Архивируем таблицу (soft delete -> is_active=false)
	rec = testutil.DELETE(t, e, fmt.Sprintf("/system-tables/%d", tableID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Добавление окна к архивной таблице отбивается 404
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/warning-windows", tableID),
		`{"message":"текст"}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
