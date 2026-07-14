package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnloadPlaces_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/unload-places", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUnloadPlaces_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create
	body := `{"name":"Test Place","description":"A test unload place","map_link":"https://maps.example.com"}`
	rec := testutil.POST(t, e, "/unload-places", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	assert.NotNil(t, createResp["id"])
	placeID := int(createResp["id"].(float64))
	assert.Greater(t, placeID, 0)

	// Get by ID
	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	getResp := testutil.ParseMap(t, rec)
	assert.Equal(t, "Test Place", getResp["name"])
	assert.Equal(t, "A test unload place", getResp["description"])
	assert.Equal(t, "active", getResp["status"])

	// Get All
	rec = testutil.GET(t, e, "/unload-places", h)
	require.Equal(t, http.StatusOK, rec.Code)

	listResp := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(listResp), 1)

	// Update
	updateBody := `{"name":"Updated Place","status":"maintenance"}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/unload-places/%d", placeID), updateBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify update
	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	getResp = testutil.ParseMap(t, rec)
	assert.Equal(t, "Updated Place", getResp["name"])
	assert.Equal(t, "maintenance", getResp["status"])

	// Delete = архив (soft-delete)
	rec = testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Архивное место скрыто из дефолтного списка
	def := testutil.ParseSlice(t, testutil.GET(t, e, "/unload-places", h))
	for _, p := range def {
		assert.NotEqual(t, float64(placeID), p["id"], "архивное место не должно быть в дефолтном списке")
	}

	// ...но видно с include_archived (is_active=false)
	arch := testutil.ParseSlice(t, testutil.GET(t, e, "/unload-places?include_archived=true", h))
	var found bool
	for _, p := range arch {
		if int(p["id"].(float64)) == placeID {
			found = true
			assert.Equal(t, false, p["is_active"])
		}
	}
	assert.True(t, found, "архивное место должно быть видно с include_archived")

	// Restore возвращает в дефолтный список
	rec = testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/restore", placeID), "", h)
	require.Equal(t, http.StatusOK, rec.Code)
	def = testutil.ParseSlice(t, testutil.GET(t, e, "/unload-places", h))
	found = false
	for _, p := range def {
		if int(p["id"].(float64)) == placeID {
			found = true
			assert.Equal(t, true, p["is_active"])
		}
	}
	assert.True(t, found, "восстановленное место должно быть в дефолтном списке")
}

func TestUnloadPlaces_History(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	created := testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Ист Место"}`, h))
	id := int(created["id"].(float64))
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/unload-places/%d", id), `{"name":"Ист Место 2"}`, h).Code)
	// Смена только статуса (без имени) не должна попадать в историю как переименование.
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/unload-places/%d", id), `{"status":"maintenance"}`, h).Code)
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d", id), h).Code)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/restore", id), "", h).Code)

	hist := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/history", id), h))
	require.Len(t, hist, 4)
	// Новые сверху: restored, archived, renamed, created.
	assert.Equal(t, "restored", hist[0]["action_type"])
	assert.Equal(t, "archived", hist[1]["action_type"])
	assert.Equal(t, "renamed", hist[2]["action_type"])
	assert.Equal(t, "created", hist[3]["action_type"])
	assert.NotEmpty(t, hist[0]["actor_name"])
	assert.NotEmpty(t, hist[3]["actor_name"])
	assert.Equal(t, "Ист Место 2", hist[2]["details"].(map[string]interface{})["name"])
	assert.Equal(t, "Ист Место", hist[3]["details"].(map[string]interface{})["name"])
}

func TestUnloadPlaces_GetByID_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/unload-places/99999", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUnloadPlaces_Restore_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/unload-places/99999/restore", "", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUnloadPlaces_TimeSlots_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create a place first
	rec := testutil.POST(t, e, "/unload-places", `{"name":"Slot Test Place"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	createResp := testutil.ParseMap(t, rec)
	placeID := int(createResp["id"].(float64))

	// Add time slot
	slotBody := `{"day_of_week":0,"open_time":"08:00","close_time":"17:00"}`
	rec = testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/time-slots", placeID), slotBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	slotResp := testutil.ParseMap(t, rec)
	slotID := int(slotResp["id"].(float64))
	assert.Greater(t, slotID, 0)

	// Get time slots
	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/time-slots", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	slots := testutil.ParseSlice(t, rec)
	assert.Len(t, slots, 1)
	assert.Equal(t, float64(0), slots[0]["day_of_week"])
	assert.Equal(t, "08:00", slots[0]["open_time"])
	assert.Equal(t, "17:00", slots[0]["close_time"])

	// Update time slot
	updateSlot := `{"open_time":"09:00","close_time":"18:00"}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/unload-places/%d/time-slots/%d", placeID, slotID), updateSlot, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify update
	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/time-slots", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	slots = testutil.ParseSlice(t, rec)
	assert.Equal(t, "09:00", slots[0]["open_time"])
	assert.Equal(t, "18:00", slots[0]["close_time"])

	// Delete time slot
	rec = testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d/time-slots/%d", placeID, slotID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify deleted
	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/time-slots", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	slots = testutil.ParseSlice(t, rec)
	assert.Len(t, slots, 0)
}

func TestUnloadPlaces_TimeSlots_InvalidTime(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create a place
	rec := testutil.POST(t, e, "/unload-places", `{"name":"Bad Time Place"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	cr := testutil.ParseMap(t, rec)
	placeID := int(cr["id"].(float64))

	// Invalid open_time format
	rec = testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/time-slots", placeID),
		`{"day_of_week":0,"open_time":"invalid","close_time":"17:00"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Invalid day_of_week
	rec = testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/time-slots", placeID),
		`{"day_of_week":7,"open_time":"08:00","close_time":"17:00"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUnloadPlaces_TimeSlots_PlaceNotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/unload-places/99999/time-slots",
		`{"day_of_week":0,"open_time":"08:00","close_time":"17:00"}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUnloadPlaces_ResponseStructure(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create place and verify response structure includes time_slots, photos, current_status
	rec := testutil.POST(t, e, "/unload-places", `{"name":"Structure Test"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	cr := testutil.ParseMap(t, rec)
	placeID := int(cr["id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	details := testutil.ParseMap(t, rec)

	assert.Contains(t, details, "id")
	assert.Contains(t, details, "name")
	assert.Contains(t, details, "status")
	assert.Contains(t, details, "is_active")
	assert.Contains(t, details, "current_status")
	assert.Contains(t, details, "time_slots")
	assert.Contains(t, details, "photos")
	assert.Contains(t, details, "created_at")
	assert.Contains(t, details, "updated_at")
}

// TestUnloadPlaces_Warning_RoundTrip проверяет, что свободное предупреждение
// (#1183) сохраняется при создании и обновлении и возвращается в DTO места.
func TestUnloadPlaces_Warning_RoundTrip(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create с warning
	rec := testutil.POST(t, e, "/unload-places",
		`{"name":"Warn Place","warning":"Пост 72: только малогабарит"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	placeID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// GET отдаёт warning
	getResp := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/unload-places/%d", placeID), h))
	assert.Equal(t, "Пост 72: только малогабарит", getResp["warning"])

	// Update меняет warning
	rec = testutil.PUT(t, e, fmt.Sprintf("/unload-places/%d", placeID),
		`{"warning":"Изменённое предупреждение"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	getResp = testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/unload-places/%d", placeID), h))
	assert.Equal(t, "Изменённое предупреждение", getResp["warning"])
}
