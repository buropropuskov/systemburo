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

// TestUnloadPlaces_UsageAndDetachAll: usage показывает привязки, они блокируют
// удаление, «Отвязать всё» их снимает (с аудитом на каждую орг/компанию) и после
// этого место архивируется. Повтор detach-all по пустому - идемпотентен.
func TestUnloadPlaces_UsageAndDetachAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	placeID := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Отвяз-Место"}`, h))["id"].(float64))
	require.NoError(t, db.Create(&models.OrganizationUnloadPlace{OrganizationID: td.OrgID, UnloadPlaceID: placeID}).Error)
	require.NoError(t, db.Create(&models.CompaniesUnloadPlace{CompanyID: td.CompanyID, UnloadPlaceID: placeID}).Error)

	// usage перечисляет обе привязки
	usage := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/usage", placeID), h))
	orgs := usage["organizations"].([]interface{})
	comps := usage["companies"].([]interface{})
	require.Len(t, orgs, 1)
	require.Len(t, comps, 1)
	assert.Equal(t, float64(td.OrgID), orgs[0].(map[string]interface{})["id"])
	assert.Equal(t, true, orgs[0].(map[string]interface{})["is_active"])
	assert.NotEmpty(t, orgs[0].(map[string]interface{})["name"])

	// пока привязано - удаление заблокировано (400)
	assert.Equal(t, http.StatusBadRequest, testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d", placeID), h).Code)

	// detach-all снимает обе привязки
	detach := testutil.ParseMap(t, testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/detach-all", placeID), "", h))
	assert.Equal(t, float64(1), detach["organizations_detached"])
	assert.Equal(t, float64(1), detach["companies_detached"])

	// аудит «место убрано» записан на организацию и компанию
	assert.Contains(t, auditDetails(t, db, models.AuditEntityOrganization, td.OrgID, models.OrganizationActionUnloadPlacesChanged), "Отвяз-Место")
	assert.Contains(t, auditDetails(t, db, models.AuditEntityCompany, td.CompanyID, models.CompanyActionUnloadPlacesChanged), "Отвяз-Место")

	// usage теперь пуст
	usage = testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/usage", placeID), h))
	assert.Empty(t, usage["organizations"].([]interface{}))
	assert.Empty(t, usage["companies"].([]interface{}))

	// теперь место архивируется
	assert.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d", placeID), h).Code)

	// повторный detach-all идемпотентен (нулевые счётчики, 200)
	detach = testutil.ParseMap(t, testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/detach-all", placeID), "", h))
	assert.Equal(t, float64(0), detach["organizations_detached"])
	assert.Equal(t, float64(0), detach["companies_detached"])
}

// TestUnloadPlaces_Usage_ArchivedBindingVisible: архивная (is_active=false)
// организация всё равно попадает в usage - она держит место (гейт Delete считает
// по junction без фильтра активности), поэтому оператор обязан её видеть.
func TestUnloadPlaces_Usage_ArchivedBindingVisible(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	placeID := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Арх-Место"}`, h))["id"].(float64))
	require.NoError(t, db.Create(&models.OrganizationUnloadPlace{OrganizationID: td.OrgID, UnloadPlaceID: placeID}).Error)
	require.NoError(t, db.Table("organizations").Where("id = ?", td.OrgID).Update("is_active", false).Error)

	usage := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/usage", placeID), h))
	orgs := usage["organizations"].([]interface{})
	require.Len(t, orgs, 1)
	assert.Equal(t, false, orgs[0].(map[string]interface{})["is_active"], "архивная организация должна быть видна в usage")
}

func TestUnloadPlaces_Usage_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	assert.Equal(t, http.StatusNotFound, testutil.GET(t, e, "/unload-places/99999/usage", h).Code)
	assert.Equal(t, http.StatusNotFound, testutil.POST(t, e, "/unload-places/99999/detach-all", "", h).Code)
}

// TestUnloadPlaces_DetachAll_Forbidden: detach-all под admin-гейтом (меняет
// привязки орг/компаний), обычному пользователю - 403.
func TestUnloadPlaces_DetachAll_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	placeID := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Гейт-Место"}`, testutil.AuthHeader(adminToken)))["id"].(float64))

	userToken := testutil.RegisterAndLogin(t, e, "detachuser", "pass123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/detach-all", placeID), "", testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestUnloadPlaces_DetachOneOrgAndCompany: точечная отвязка снимает ОДНУ
// конкретную привязку (org или company), остальные остаются; аудит на затронутую
// сущность; повтор по уже снятой - идемпотентен (detached:false, 200).
func TestUnloadPlaces_DetachOneOrgAndCompany(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	placeID := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Точ-Место"}`, h))["id"].(float64))
	require.NoError(t, db.Create(&models.OrganizationUnloadPlace{OrganizationID: td.OrgID, UnloadPlaceID: placeID}).Error)
	require.NoError(t, db.Create(&models.CompaniesUnloadPlace{CompanyID: td.CompanyID, UnloadPlaceID: placeID}).Error)

	// Отвязываем ТОЛЬКО организацию - компания остаётся.
	detach := testutil.ParseMap(t, testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d/organizations/%d", placeID, td.OrgID), h))
	assert.Equal(t, true, detach["detached"])
	assert.Contains(t, auditDetails(t, db, models.AuditEntityOrganization, td.OrgID, models.OrganizationActionUnloadPlacesChanged), "Точ-Место")

	usage := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/usage", placeID), h))
	assert.Empty(t, usage["organizations"].([]interface{}), "организация снята")
	require.Len(t, usage["companies"].([]interface{}), 1, "компания осталась")

	// Повтор по уже снятой организации идемпотентен (detached:false, 200).
	detach = testutil.ParseMap(t, testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d/organizations/%d", placeID, td.OrgID), h))
	assert.Equal(t, false, detach["detached"])

	// Отвязываем компанию - место свободно, архивируется.
	detach = testutil.ParseMap(t, testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d/companies/%d", placeID, td.CompanyID), h))
	assert.Equal(t, true, detach["detached"])
	usage = testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/usage", placeID), h))
	assert.Empty(t, usage["companies"].([]interface{}))
	assert.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d", placeID), h).Code)
}

// TestUnloadPlaces_DetachOne_ForbiddenAndNotFound: точечная отвязка под admin-
// гейтом (403 обычному), место/таблица не найдены -> 404.
func TestUnloadPlaces_DetachOne_ForbiddenAndNotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	placeID := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Гейт-Точ"}`, adminH))["id"].(float64))
	require.NoError(t, db.Create(&models.OrganizationUnloadPlace{OrganizationID: td.OrgID, UnloadPlaceID: placeID}).Error)

	userH := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "detachoneuser", "pass123", 1, td.OrgID, td.CompanyID))
	assert.Equal(t, http.StatusForbidden, testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d/organizations/%d", placeID, td.OrgID), userH).Code)

	assert.Equal(t, http.StatusNotFound, testutil.DELETE(t, e, fmt.Sprintf("/unload-places/99999/organizations/%d", td.OrgID), adminH).Code)
	assert.Equal(t, http.StatusNotFound, testutil.DELETE(t, e, fmt.Sprintf("/unload-places/99999/companies/%d", td.CompanyID), adminH).Code)
}

// auditDetails - JSON деталей (как строка) последней записи audit_log для
// сущности/действия.
func auditDetails(t *testing.T, db *gorm.DB, entityType string, entityID int, action string) string {
	t.Helper()
	var row struct {
		Details string `gorm:"column:details"`
	}
	err := db.Table("audit_log").
		Select("details").
		Where("entity_type = ? AND entity_id = ? AND action = ?", entityType, entityID, action).
		Order("created_at DESC, id DESC").
		Limit(1).
		Scan(&row).Error
	require.NoError(t, err)
	return row.Details
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

func TestUnloadPlaces_WarningWindows_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create a place first
	rec := testutil.POST(t, e, "/unload-places", `{"name":"Warning Window Place"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	placeID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Add a windowed warning (Пн 12:00-13:00 малогабарит)
	body := `{"day_of_week":1,"time_from":"12:00","time_to":"13:00","message":"Только малогабарит"}`
	rec = testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/warning-windows", placeID), body, h)
	require.Equal(t, http.StatusOK, rec.Code)
	windowID := int(testutil.ParseMap(t, rec)["id"].(float64))
	assert.Greater(t, windowID, 0)

	// Get warning windows
	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/warning-windows", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	windows := testutil.ParseSlice(t, rec)
	require.Len(t, windows, 1)
	assert.Equal(t, float64(1), windows[0]["day_of_week"])
	assert.Equal(t, "12:00", windows[0]["time_from"])
	assert.Equal(t, "13:00", windows[0]["time_to"])
	assert.Equal(t, "Только малогабарит", windows[0]["message"])
	assert.Equal(t, true, windows[0]["is_active"])

	// Warning windows are embedded in the place detail DTO
	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	detail := testutil.ParseMap(t, rec)
	embedded, ok := detail["warning_windows"].([]interface{})
	require.True(t, ok, "warning_windows должно присутствовать в DTO места")
	assert.Len(t, embedded, 1)

	// Update to a general warning (каждый день / весь день) -- nullable поля -> NULL
	updateBody := `{"day_of_week":null,"time_from":null,"time_to":null,"message":"Пропуск оформляется заранее"}`
	rec = testutil.PUT(t, e, fmt.Sprintf("/unload-places/%d/warning-windows/%d", placeID, windowID), updateBody, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Verify NULL round-trip: сброс дня/времени в NULL реально доезжает до БД
	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/warning-windows", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	windows = testutil.ParseSlice(t, rec)
	require.Len(t, windows, 1)
	assert.Nil(t, windows[0]["day_of_week"], "day_of_week должен сброситься в NULL (каждый день)")
	assert.Nil(t, windows[0]["time_from"], "time_from должен сброситься в NULL (весь день)")
	assert.Nil(t, windows[0]["time_to"])
	assert.Equal(t, "Пропуск оформляется заранее", windows[0]["message"])

	// Delete
	rec = testutil.DELETE(t, e, fmt.Sprintf("/unload-places/%d/warning-windows/%d", placeID, windowID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/unload-places/%d/warning-windows", placeID), h)
	require.Equal(t, http.StatusOK, rec.Code)
	windows = testutil.ParseSlice(t, rec)
	assert.Len(t, windows, 0)
}

func TestUnloadPlaces_WarningWindows_Validation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/unload-places", `{"name":"Warning Validation Place"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	placeID := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Пустой message -> 400
	rec = testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/warning-windows", placeID),
		`{"message":""}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Неверный формат времени -> 400
	rec = testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/warning-windows", placeID),
		`{"time_from":"invalid","message":"текст"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Неверный день недели -> 400
	rec = testutil.POST(t, e, fmt.Sprintf("/unload-places/%d/warning-windows", placeID),
		`{"day_of_week":7,"message":"текст"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUnloadPlaces_WarningWindows_PlaceNotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/unload-places/99999/warning-windows",
		`{"message":"текст"}`, h)
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
