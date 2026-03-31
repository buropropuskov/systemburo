package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompanies_GetAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/companies", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	companies := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(companies), 1)
	assert.Contains(t, companies[0], "id")
	assert.Contains(t, companies[0], "name")
}

func TestCompanies_GetAll_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/companies", nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCompanies_Create(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	body := `{"name":"New Company"}`
	rec := testutil.POST(t, e, "/companies", body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	comp := testutil.ParseMap(t, rec)
	assert.Equal(t, "New Company", comp["name"])
	assert.NotZero(t, comp["id"])
}

func TestCompanies_Create_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"name":"Forbidden Company"}`
	rec := testutil.POST(t, e, "/companies", body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCompanies_Update(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	body := `{"name":"Updated Company"}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/companies/%d", td.CompanyID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	comp := testutil.ParseMap(t, rec)
	assert.Equal(t, "Updated Company", comp["name"])
	assert.Equal(t, float64(td.CompanyID), comp["id"])
}

func TestCompanies_Update_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"name":"Updated"}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/companies/%d", td.CompanyID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCompanies_Delete(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create a new company to delete (seeded one has the admin user)
	createRec := testutil.POST(t, e, "/companies", `{"name":"To Delete"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	created := testutil.ParseMap(t, createRec)
	compID := int(created["id"].(float64))

	rec := testutil.DELETE(t, e, fmt.Sprintf("/companies/%d", compID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Company deleted", msg)
}

func TestCompanies_Delete_WithUsers_Fails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// The seeded company has the admin user, so delete should fail
	rec := testutil.DELETE(t, e, fmt.Sprintf("/companies/%d", td.CompanyID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCompanies_Delete_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.DELETE(t, e, fmt.Sprintf("/companies/%d", td.CompanyID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCompanies_GetWithUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/companies/with-users", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	companies := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(companies), 1)

	for _, c := range companies {
		if int(c["id"].(float64)) == td.CompanyID {
			assert.Contains(t, c, "user_count")
			assert.GreaterOrEqual(t, c["user_count"].(float64), float64(1))
			return
		}
	}
	t.Error("Test company not found in with-users response")
}

func TestCompanies_GetWithUsersExtended(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/companies/with-users-extended", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	companies := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(companies), 1)
	assert.Contains(t, companies[0], "id")
	assert.Contains(t, companies[0], "name")
	assert.Contains(t, companies[0], "user_count")
	assert.Contains(t, companies[0], "unload_places")
}

func TestCompanies_WithUsers_MultipleUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterUser(t, e, "compuser2", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "compuser3", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/companies/with-users", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	companies := testutil.ParseSlice(t, rec)

	for _, c := range companies {
		if int(c["id"].(float64)) == td.CompanyID {
			// admin + compuser2 + compuser3 = 3
			assert.Equal(t, float64(3), c["user_count"].(float64))
			return
		}
	}
	t.Error("Test company not found")
}

func TestCompanies_GetUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	users := testutil.ParseSlice(t, rec)
	assert.NotNil(t, users)
}

func TestCompanies_UpdateUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterUser(t, e, "compuser1", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"users":[{"username":"compuser1","is_primary":true,"required_approval":true}]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Company users updated successfully", msg)

	// Verify
	getRec := testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec.Code)
	users := testutil.ParseSlice(t, getRec)
	assert.Len(t, users, 1)
	assert.Equal(t, "compuser1", users[0]["username"])
	assert.Equal(t, true, users[0]["is_primary"])
	assert.Equal(t, true, users[0]["required_approval"])
}

func TestCompanies_UpdateUsers_MultiplePrimary_Fails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterUser(t, e, "cu1", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "cu2", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"users":[{"username":"cu1","is_primary":true},{"username":"cu2","is_primary":true}]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCompanies_UpdateUsers_ReplaceStrategy(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterUser(t, e, "cu1", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "cu2", "pass123", 1, td.OrgID, td.CompanyID)

	// Assign cu1
	body1 := `{"users":[{"username":"cu1","is_primary":true}]}`
	rec1 := testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), body1, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec1.Code)

	// Replace with cu2
	body2 := `{"users":[{"username":"cu2","is_primary":true}]}`
	rec2 := testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), body2, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec2.Code)

	// Verify only cu2 is assigned now
	getRec := testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec.Code)
	users := testutil.ParseSlice(t, getRec)
	assert.Len(t, users, 1)
	assert.Equal(t, "cu2", users[0]["username"])
}

func TestCompanies_GetTables(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, fmt.Sprintf("/companies/%d/tables", td.CompanyID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	tables := testutil.ParseSlice(t, rec)
	assert.NotNil(t, tables)
}

func TestCompanies_UpdateTables(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create a system table
	createBody := `{"name":"comp-table","display_name":"Company Table","table_type":"cars"}`
	createRec := testutil.POST(t, e, "/system-tables", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	table := testutil.ParseMap(t, createRec)
	tableID := int(table["id"].(float64))

	// Assign table to company
	body := fmt.Sprintf(`{"table_ids":[%d]}`, tableID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/companies/%d/tables", td.CompanyID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Company tables updated successfully", msg)

	// Verify
	getRec := testutil.GET(t, e, fmt.Sprintf("/companies/%d/tables", td.CompanyID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec.Code)
	tables := testutil.ParseSlice(t, getRec)
	assert.Len(t, tables, 1)
	assert.Equal(t, "comp-table", tables[0]["name"])
}

func TestCompanies_UpdateTables_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"table_ids":[]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/companies/%d/tables", td.CompanyID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCompanies_GetUnloadPlaces(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, fmt.Sprintf("/companies/%d/unload-places", td.CompanyID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	places := testutil.ParseSlice(t, rec)
	assert.NotNil(t, places)
}

func TestCompanies_UpdateUnloadPlaces(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create an unload place
	createBody := `{"name":"Company Unload Place","description":"desc","status":"active"}`
	createRec := testutil.POST(t, e, "/unload-places", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	place := testutil.ParseMap(t, createRec)
	placeID := int(place["id"].(float64))

	// Assign to company
	body := fmt.Sprintf(`{"unload_place_ids":[%d]}`, placeID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/companies/%d/unload-places", td.CompanyID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Unload places updated successfully", msg)

	// Verify
	getRec := testutil.GET(t, e, fmt.Sprintf("/companies/%d/unload-places", td.CompanyID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec.Code)
	places := testutil.ParseSlice(t, getRec)
	assert.Len(t, places, 1)
	assert.Equal(t, "Company Unload Place", places[0]["name"])
}

func TestCompanies_UpdateUnloadPlaces_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"unload_place_ids":[]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/companies/%d/unload-places", td.CompanyID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}
