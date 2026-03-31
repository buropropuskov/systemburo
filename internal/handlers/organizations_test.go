package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizations_GetAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/organizations", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	orgs := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(orgs), 1)
	assert.Contains(t, orgs[0], "id")
	assert.Contains(t, orgs[0], "name")
}

func TestOrganizations_GetAll_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/organizations", nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestOrganizations_Create(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	body := `{"name":"New Organization"}`
	rec := testutil.POST(t, e, "/organizations", body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	org := testutil.ParseMap(t, rec)
	assert.Equal(t, "New Organization", org["name"])
	assert.NotZero(t, org["id"])
}

func TestOrganizations_Create_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	// Register a regular user (type_id=1), not admin
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"name":"Forbidden Org"}`
	rec := testutil.POST(t, e, "/organizations", body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestOrganizations_Update(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	body := `{"name":"Updated Organization"}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d", td.OrgID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	org := testutil.ParseMap(t, rec)
	assert.Equal(t, "Updated Organization", org["name"])
	assert.Equal(t, float64(td.OrgID), org["id"])
}

func TestOrganizations_Delete(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create a new org to delete (the seeded one has the admin user)
	createRec := testutil.POST(t, e, "/organizations", `{"name":"To Delete"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	created := testutil.ParseMap(t, createRec)
	orgID := int(created["id"].(float64))

	rec := testutil.DELETE(t, e, fmt.Sprintf("/organizations/%d", orgID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Organization deleted", msg)
}

func TestOrganizations_Delete_WithUsers_Fails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// The seeded org has the admin user, so delete should fail
	rec := testutil.DELETE(t, e, fmt.Sprintf("/organizations/%d", td.OrgID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOrganizations_GetWithUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/organizations/with-users", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	orgs := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(orgs), 1)

	// Find the test org and verify user_count
	for _, o := range orgs {
		if int(o["id"].(float64)) == td.OrgID {
			assert.Contains(t, o, "user_count")
			assert.GreaterOrEqual(t, o["user_count"].(float64), float64(1))
			return
		}
	}
	t.Error("Test organization not found in with-users response")
}

func TestOrganizations_GetWithUsersExtended(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/organizations/with-users-extended", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	orgs := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(orgs), 1)
	assert.Contains(t, orgs[0], "id")
	assert.Contains(t, orgs[0], "name")
	assert.Contains(t, orgs[0], "user_count")
	assert.Contains(t, orgs[0], "unload_places")
}

func TestOrganizations_GetMyOrganization(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/get-organization", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseMap(t, rec)
	assert.Contains(t, resp, "organization")
	assert.Contains(t, resp, "organization_id")
	assert.Equal(t, float64(td.OrgID), resp["organization_id"])
	assert.Equal(t, "Test Organization", resp["organization"])
}

func TestOrganizations_GetMyOrganization_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/get-organization", nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestOrganizations_GetOrganizationUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	users := testutil.ParseSlice(t, rec)
	// Initially no organization_users junction records, so empty array is valid
	assert.NotNil(t, users)
}

func TestOrganizations_UpdateOrganizationUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Register another user to assign as org user
	testutil.RegisterUser(t, e, "orguser1", "pass123", 1, td.OrgID, td.CompanyID)

	isPrimary := true
	_ = isPrimary
	body := fmt.Sprintf(`{"users":[{"username":"orguser1","is_primary":true,"required_approval":false}]}`)
	rec := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Organization users updated successfully", msg)

	// Verify the user is now listed
	getRec := testutil.GET(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec.Code)
	users := testutil.ParseSlice(t, getRec)
	assert.Len(t, users, 1)
	assert.Equal(t, "orguser1", users[0]["username"])
	assert.Equal(t, true, users[0]["is_primary"])
}

func TestOrganizations_UpdateOrganizationUsers_MultiplePrimary_Fails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterUser(t, e, "orguser1", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "orguser2", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"users":[{"username":"orguser1","is_primary":true},{"username":"orguser2","is_primary":true}]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOrganizations_GetOrganizationTables(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, fmt.Sprintf("/organizations/%d/tables", td.OrgID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	tables := testutil.ParseSlice(t, rec)
	assert.NotNil(t, tables)
}

func TestOrganizations_UpdateOrganizationTables(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create a system table to assign
	createBody := `{"name":"test-table","display_name":"Test Table","table_type":"cars"}`
	createRec := testutil.POST(t, e, "/system-tables", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	table := testutil.ParseMap(t, createRec)
	tableID := int(table["id"].(float64))

	// Assign the table to the org
	body := fmt.Sprintf(`{"table_ids":[%d]}`, tableID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/tables", td.OrgID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Organization tables updated successfully", msg)

	// Verify tables are assigned
	getRec := testutil.GET(t, e, fmt.Sprintf("/organizations/%d/tables", td.OrgID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec.Code)
	tables := testutil.ParseSlice(t, getRec)
	assert.Len(t, tables, 1)
	assert.Equal(t, "test-table", tables[0]["name"])
}

func TestOrganizations_UpdateOrganizationTables_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"table_ids":[]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/tables", td.OrgID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestOrganizations_GetOrganizationUnloadPlaces(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, fmt.Sprintf("/organizations/%d/unload-places", td.OrgID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	places := testutil.ParseSlice(t, rec)
	assert.NotNil(t, places)
}

func TestOrganizations_UpdateOrganizationUnloadPlaces(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create an unload place to assign
	createBody := `{"name":"Test Unload Place","description":"desc","status":"active"}`
	createRec := testutil.POST(t, e, "/unload-places", createBody, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	place := testutil.ParseMap(t, createRec)
	placeID := int(place["id"].(float64))

	// Assign to org
	body := fmt.Sprintf(`{"unload_place_ids":[%d]}`, placeID)
	rec := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/unload-places", td.OrgID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Unload places updated successfully", msg)

	// Verify
	getRec := testutil.GET(t, e, fmt.Sprintf("/organizations/%d/unload-places", td.OrgID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, getRec.Code)
	places := testutil.ParseSlice(t, getRec)
	assert.Len(t, places, 1)
	assert.Equal(t, "Test Unload Place", places[0]["name"])
}

func TestOrganizations_UpdateOrganizationUnloadPlaces_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	body := `{"unload_place_ids":[]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/unload-places", td.OrgID), body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestOrganizations_WithUsers_MultipleUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Register additional users in the same org
	testutil.RegisterUser(t, e, "user2", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "user3", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/organizations/with-users", testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	orgs := testutil.ParseSlice(t, rec)

	for _, o := range orgs {
		if int(o["id"].(float64)) == td.OrgID {
			// admin + user2 + user3 = 3 users
			assert.Equal(t, float64(3), o["user_count"].(float64))
			return
		}
	}
	t.Error("Test organization not found")
}
