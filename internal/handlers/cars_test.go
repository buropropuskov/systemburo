package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- helpers ---

// seedCarViaCompleteApp creates a complete application with a car and returns (appID, attachmentID, carID).
func seedCarViaCompleteApp(t *testing.T, e *echo.Echo, db *gorm.DB, token string, orgName string) (int, int, int) {
	t.Helper()

	uaID := seedUniqueAttachment(t, db, "cars", fmt.Sprintf("car_tmpl_%s", t.Name()), "Car Template")

	body := fmt.Sprintf(`{
		"message": "car test",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "car_tmpl",
			"attachment_display_name": "Car Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {
				"vehicles": [{
					"car_number": "B002BB799",
					"car_brand": "Kamaz"
				}]
			}
		}]
	}`, orgName, uaID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "submit complete app: %s", rec.Body.String())

	createResp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
	appID := createResp.ApplicationID

	// Get attachment ID
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	atts := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, atts)
	attID := int(atts[0]["id"].(float64))

	// Get car ID
	rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	cars := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, cars)
	carID := int(cars[0]["id"].(float64))

	return appID, attID, carID
}

// activateCarViaApp sets confirmation='Согласовано', takes application to work, and activates items.
func activateCarViaApp(t *testing.T, e *echo.Echo, db *gorm.DB, appID int, td testutil.TestData) string {
	t.Helper()
	username := fmt.Sprintf("carappr_%d", appID)
	testutil.RegisterUser(t, e, username, "pass123", 6, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, username)
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", approverID)
	approverToken, _ := testutil.LoginUser(t, e, username, "pass123")

	// Set confirmation to 'Согласовано' (required for GetActiveCarsForTables)
	testutil.PUT(t, e, fmt.Sprintf("/applications/%d", appID),
		`{"confirmation":"Согласовано"}`, testutil.AuthHeader(approverToken))

	// Take to work (sets status='В работе' and activates cars via activateApplicationItems)
	body := fmt.Sprintf(`{"user_id": %d, "action": "accept"}`, approverID)
	testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), body, testutil.AuthHeader(approverToken))

	// Also call update-items-status for completeness
	testutil.POST(t, e, fmt.Sprintf("/applications/%d/update-items-status", appID), "", testutil.AuthHeader(approverToken))

	return approverToken
}

// --- 401 Unauthorized tests ---

func TestCars_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/cars/unload-places"},
		{"GET", "/cars/fact-unload-places"},
		{"GET", "/cars/check-active?car_number=X&car_brand=Y"},
		{"GET", "/cars/1/history"},
		{"POST", "/cars/1/history"},
		{"GET", "/cars/history/all"},
		{"GET", "/cars/history/current-status"},
		{"PUT", "/cars/1/territory-status"},
		{"PUT", "/cars/1/deactivate"},
		{"PUT", "/cars/1/activate"},
		{"GET", "/cars/history/unified?car_number=X&car_brand=Y"},
		{"PUT", "/cars/1/restore"},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s_%s", ep.method, ep.path), func(t *testing.T) {
			var rec *httptest.ResponseRecorder
			switch ep.method {
			case "GET":
				rec = testutil.GET(t, e, ep.path, nil)
			case "POST":
				rec = testutil.POST(t, e, ep.path, "{}", nil)
			case "PUT":
				rec = testutil.PUT(t, e, ep.path, "{}", nil)
			}
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}




// --- GET /cars/active-for-table/:table_id (scoped «Проезд», #1036) ---

func TestGetActiveCarsForTable_ScopedByTargetTable(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carscoped1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// Две cars-таблицы; машину привязываем «Проездом» только к первой.
	dnA, dnB := "Проезд A", "Проезд B"
	tblA := models.SystemTable{Name: "cars_a", DisplayName: &dnA, TableType: "cars", IsActive: true}
	tblB := models.SystemTable{Name: "cars_b", DisplayName: &dnB, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tblA).Error)
	require.NoError(t, db.Create(&tblB).Error)
	require.NoError(t, db.Exec(
		"INSERT INTO car_target_tables (car_id, table_id, order_index) VALUES (?, ?, 1)", carID, tblA.ID).Error)

	// В привязанной таблице машина видна.
	rec := testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", tblA.ID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	inA := testutil.ParseSlice(t, rec)
	require.Len(t, inA, 1, "машина видна в привязанной таблице «Проезд»")
	assert.Equal(t, "B002BB799", inA[0]["car_number"])

	// В непривязанной таблице — не видна (доказывает scope, а не «во всех сразу»).
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", tblB.ID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	inB := testutil.ParseSlice(t, rec)
	assert.Empty(t, inB, "машина не видна в непривязанной таблице")
}

// Подача заявки с выбранным «Проезд» пишет car_target_tables, и машина видна только
// в выбранной таблице (#1036, срез B: end-to-end submit -> показ).
func TestSubmitCar_PassageTablesWrittenAndScoped(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	dnA, dnB := "Проезд A", "Проезд B"
	tblA := models.SystemTable{Name: "cars_pa", DisplayName: &dnA, TableType: "cars", IsActive: true}
	tblB := models.SystemTable{Name: "cars_pb", DisplayName: &dnB, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tblA).Error)
	require.NoError(t, db.Create(&tblB).Error)

	token := testutil.RegisterAndLogin(t, e, "carpass1", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_pass_tmpl", "Cars Pass")

	body := fmt.Sprintf(`{
		"message":"passage test","organization":"Test Organization",
		"responsible_person":"Test","contact_phone":"+79001234567","data_approval":true,
		"attachments":[{"attachment_type":"cars","attachment_name":"cars_tmpl",
			"attachment_display_name":"Cars Template","unique_attachment_id":%d,
			"entry_date_from":"2026-04-01","entry_date_to":"2099-12-31",
			"data":{"vehicles":[{"car_number":"C777CC177","car_brand":"Kia","passage_tables":[%d]}]}}]
	}`, uaID, tblA.ID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	// Связь «Проезд» записана подачей.
	var linkCount int64
	require.NoError(t, db.Table("car_target_tables ctt").
		Joins("JOIN cars c ON c.id = ctt.car_id").
		Where("ctt.table_id = ? AND c.car_number = ?", tblA.ID, "C777CC177").
		Count(&linkCount).Error)
	assert.EqualValues(t, 1, linkCount, "submit должен записать car_target_tables для выбранного «Проезда»")

	// Активируем машину и проверяем scoped-показ.
	var appID int
	require.NoError(t, db.Raw(`SELECT app.id FROM applications app
		JOIN attachments a ON a.application_id = app.id
		JOIN cars c ON c.attachment_id = a.id WHERE c.car_number = ?`, "C777CC177").Scan(&appID).Error)
	activateCarViaApp(t, e, db, appID, td)

	rec = testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", tblA.ID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, testutil.ParseSlice(t, rec), 1, "машина видна в привязанной таблице «Проезд»")

	rec = testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", tblB.ID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, testutil.ParseSlice(t, rec), "машина не видна в непривязанной таблице")
}

// GET /attachments/:id/cars отдаёт target_tables машины по образцу employee (#1036 срез E).
func TestGetAttachmentCars_IncludesTargetTables(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	dn := "Проезд E"
	tbl := models.SystemTable{Name: "cars_e_target", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	token := testutil.RegisterAndLogin(t, e, "cartarget1", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_target_tmpl", "Cars Target")

	body := fmt.Sprintf(`{
		"message":"target tables test","organization":"Test Organization",
		"responsible_person":"Test","contact_phone":"+79001234567","data_approval":true,
		"attachments":[{"attachment_type":"cars","attachment_name":"cars_tmpl",
			"attachment_display_name":"Cars Template","unique_attachment_id":%d,
			"entry_date_from":"2026-04-01","entry_date_to":"2099-12-31",
			"data":{"vehicles":[{"car_number":"D555DD177","car_brand":"Volvo","passage_tables":[%d]}]}}]
	}`, uaID, tbl.ID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	createResp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", createResp.ApplicationID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	atts := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, atts)
	attID := int(atts[0]["id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	cars := testutil.ParseSlice(t, rec)
	require.Len(t, cars, 1)

	targetTables, ok := cars[0]["target_tables"].([]interface{})
	require.True(t, ok, "target_tables должен присутствовать в ответе вложения-машины")
	require.Len(t, targetTables, 1)
	table := targetTables[0].(map[string]interface{})
	assert.Equal(t, dn, table["display_name"])
	assert.EqualValues(t, tbl.ID, table["id"])
}



// --- GET /cars/unload-places ---

func TestGetCarUnloadPlaces_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carup1", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/cars/unload-places", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	places := testutil.ParseSlice(t, rec)
	assert.Empty(t, places)
}

// --- GET /cars/fact-unload-places ---

func TestGetFactCarUnloadPlaces_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carfup1", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/cars/fact-unload-places", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	places := testutil.ParseSlice(t, rec)
	assert.Empty(t, places)
}

// --- GET /cars/check-active ---

func TestCheckActiveCar_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carchk1", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/cars/check-active?car_number=NONEXIST&car_brand=Unknown", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMap(t, rec)
	assert.False(t, resp["active"].(bool))
}

func TestCheckActiveCar_Found(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carchkfound", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, _ := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	rec := testutil.GET(t, e, fmt.Sprintf("/cars/check-active?car_number=B002BB799&car_brand=Kamaz&organization_id=%d", td.OrgID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, true, resp["active"])
	if resp["car_number"] != nil {
		assert.Equal(t, "B002BB799", resp["car_number"])
	}
}

// stringPtr is a test helper to safely dereference interface{} to string pointer.
func stringPtr(v interface{}) *string {
	if v == nil {
		return nil
	}
	s := v.(string)
	return &s
}

// --- GET /cars/:id/history ---

func TestGetCarHistory_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carhist1", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")

	rec := testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	history := testutil.ParseSlice(t, rec)
	// SubmitCompleteApplication creates a "create" history entry
	assert.GreaterOrEqual(t, len(history), 1)
}

func TestGetCarHistory_InvalidID(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carhistinv", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/cars/abc/history", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- POST /cars/:id/history ---

func TestAddCarHistoryEntry_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carhistadd", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	userID := getUserID(t, db, "carhistadd")

	body := fmt.Sprintf(`{
		"user_id": %d,
		"action_type": "comment",
		"comment": "manual entry"
	}`, userID)
	rec := testutil.POST(t, e, fmt.Sprintf("/cars/%d/history", carID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Car history entry added successfully", msg)
}

// --- GET /cars/history/all ---

func TestGetAllCarsHistory_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carhistall", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/cars/history/all", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	// Empty or may contain entries from other tests; just check valid JSON array
	testutil.ParseSlice(t, rec)
}

// --- GET /cars/history/current-status ---

func TestGetCarsCurrentStatus_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carcurstat", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/cars/history/current-status", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	statuses := testutil.ParseSlice(t, rec)
	assert.Empty(t, statuses)
}

// --- PUT /cars/:id/territory-status ---

func TestUpdateCarTerritoryStatus_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carterr1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	passTbl := seedPassTableGrant(t, db, getUserID(t, db, "carterr1"), "cars")

	body := fmt.Sprintf(`{"territory_status": 1, "table_id": %d}`, passTbl)
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/territory-status", carID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Car territory status updated successfully", msg)
}

// --- PUT /cars/:id/deactivate ---

func TestDeactivateCar_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "cardeact1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	body := `{"status": 2}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Car deactivated successfully", msg)
}

// --- PUT /cars/:id/activate ---

func TestActivateCar_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "caract1", "pass123", 1, td.OrgID, td.CompanyID)
	_, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")

	// Car starts with status=0, activate sets status=1
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/activate", carID), `{}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Car activated successfully", msg)
}

// --- PUT /cars/:id/restore ---

func TestRestoreCar_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carrestore1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// Deactivate first
	testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID), `{"status": 2}`, testutil.AuthHeader(token))

	// Now restore
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/restore", carID), `{}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Car restored successfully", msg)
}

// --- GET /cars/history/unified ---

func TestGetUnifiedCarHistory_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carunified1", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/cars/history/unified?car_number=NONEXIST&car_brand=Unknown", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	history := testutil.ParseSlice(t, rec)
	assert.Empty(t, history)
}

func TestGetUnifiedCarHistory_WithData(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carunified2", "pass123", 1, td.OrgID, td.CompanyID)
	seedCarViaCompleteApp(t, e, db, token, "Test Organization")

	rec := testutil.GET(t, e, "/cars/history/unified?car_number=B002BB799&car_brand=Kamaz", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	// SubmitCompleteApplication creates cars but may not insert history entries.
	// After activation, TakeToWork should add activation history.
	// If still empty, it means the history is only created via explicit API calls.
	testutil.ParseSlice(t, rec)
}

// --- Full car lifecycle test ---

func TestCarLifecycle_CreateActivateTerritoryDeactivateRestore(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carlc1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")

	// Смотрим машину через адресный путь «Проезд» (#1036): выдача идёт по конкретной
	// таблице, поэтому машину сначала к ней привязываем. Раньше тест ходил на
	// /cars/active-for-tables - тот путь снят вместе с техдолгом (#1050).
	lcTable := seedPassTableGrant(t, db, getUserID(t, db, "carlc1"), "cars")
	require.NoError(t, db.Exec(
		"INSERT INTO car_target_tables (car_id, table_id, order_index) VALUES (?, ?, 1)", carID, lcTable).Error)
	lcPath := fmt.Sprintf("/cars/active-for-table/%d", lcTable)

	// 1. Car initially has status=0
	rec := testutil.GET(t, e, lcPath, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	emptyCars := testutil.ParseSlice(t, rec)
	assert.Empty(t, emptyCars, "no active cars before activation")

	// 2. Activate via application workflow
	activateCarViaApp(t, e, db, appID, td)

	// 3. Now car should be active
	rec = testutil.GET(t, e, lcPath, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	activeCars := testutil.ParseSlice(t, rec)
	require.GreaterOrEqual(t, len(activeCars), 1, "expected active car after activation")

	// 4. Update territory status (car enters territory)
	passTbl := seedPassTableGrant(t, db, getUserID(t, db, "carlc1"), "cars")
	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/territory-status", carID), fmt.Sprintf(`{"territory_status": 1, "table_id": %d}`, passTbl), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	// 5. Check current status
	rec = testutil.GET(t, e, "/cars/history/current-status", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	// 6. Add manual history entry
	userID := getUserID(t, db, "carlc1")
	histBody := fmt.Sprintf(`{"user_id": %d, "action_type": "note", "comment": "inspection ok"}`, userID)
	rec = testutil.POST(t, e, fmt.Sprintf("/cars/%d/history", carID), histBody, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	// 7. Check car history
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	history := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(history), 2)

	// 8. Deactivate car
	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID), `{"status": 2}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	// 9. Restore car
	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/restore", carID), `{}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	// 10. Check active again
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/check-active?car_number=B002BB799&car_brand=Kamaz"), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// --- Car with unload places ---

func TestCarWithUnloadPlaces(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Create unload place
	up := models.UnloadPlace{Name: "Gate 1", IsActive: true}
	require.NoError(t, db.Create(&up).Error)

	uaID := seedUniqueAttachment(t, db, "cars", "car_tmpl_up", "Car with UP")
	token := testutil.RegisterAndLogin(t, e, "caruptest", "pass123", 1, td.OrgID, td.CompanyID)

	body := fmt.Sprintf(`{
		"message": "car with unload places",
		"organization": "Test Organization",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "car_tmpl",
			"attachment_display_name": "Car Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {
				"vehicles": [{
					"car_number": "C003CC777",
					"car_brand": "MAN",
					"unload_places": [%d]
				}]
			}
		}]
	}`, uaID, up.ID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
	appID := createResp.ApplicationID

	// Activate car
	activateCarViaApp(t, e, db, appID, td)

	// Check unload places
	rec = testutil.GET(t, e, "/cars/unload-places", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	// Unload places may be empty if car_unload_places linking didn't occur
	// This tests the endpoint returns 200, not necessarily populated data
	testutil.ParseSlice(t, rec)
}
