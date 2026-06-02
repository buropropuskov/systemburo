package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

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
