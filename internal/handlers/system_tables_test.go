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
