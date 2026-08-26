package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
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

	body := `{"name":"New Organization","type":"Организация"}`
	rec := testutil.POST(t, e, "/organizations", body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	org := testutil.ParseMap(t, rec)
	assert.Equal(t, "New Organization", org["name"])
	assert.Equal(t, "Организация", org["type"])
	assert.NotZero(t, org["id"])

	// #1046: тип обязателен и должен быть валиден - невалидный и пустой дают 400.
	assert.Equal(t, http.StatusBadRequest,
		testutil.POST(t, e, "/organizations", `{"name":"Плохой тип","type":"Ерунда"}`, testutil.AuthHeader(token)).Code)
	assert.Equal(t, http.StatusBadRequest,
		testutil.POST(t, e, "/organizations", `{"name":"Без типа"}`, testutil.AuthHeader(token)).Code)
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

	// #1046: тип опционален - можно задать валидный и снять через null.
	changed := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d", td.OrgID), `{"name":"Updated Organization","type":"Подрядчик"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, changed.Code)
	assert.Equal(t, "Подрядчик", testutil.ParseMap(t, changed)["type"])

	cleared := testutil.PUT(t, e, fmt.Sprintf("/organizations/%d", td.OrgID), `{"name":"Updated Organization","type":null}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, cleared.Code)
	assert.Nil(t, testutil.ParseMap(t, cleared)["type"])

	// Невалидный тип при обновлении - 400.
	assert.Equal(t, http.StatusBadRequest,
		testutil.PUT(t, e, fmt.Sprintf("/organizations/%d", td.OrgID), `{"name":"Updated Organization","type":"Ерунда"}`, testutil.AuthHeader(token)).Code)
}

func TestOrganizations_Delete(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create a new org to delete (the seeded one has the admin user)
	createRec := testutil.POST(t, e, "/organizations", `{"name":"To Delete","type":"Отдел"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	created := testutil.ParseMap(t, createRec)
	orgID := int(created["id"].(float64))

	rec := testutil.DELETE(t, e, fmt.Sprintf("/organizations/%d", orgID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Organization archived", msg)

	// Архивная организация исчезает из списка по умолчанию, но видна с include_archived.
	def := testutil.ParseSlice(t, testutil.GET(t, e, "/organizations/with-users", testutil.AuthHeader(token)))
	for _, o := range def {
		assert.NotEqual(t, float64(orgID), o["id"], "архивная организация не должна быть в списке по умолчанию")
	}
	arch := testutil.ParseSlice(t, testutil.GET(t, e, "/organizations/with-users?include_archived=true", testutil.AuthHeader(token)))
	var foundArchived bool
	for _, o := range arch {
		if int(o["id"].(float64)) == orgID {
			foundArchived = true
			assert.Equal(t, false, o["is_active"])
		}
	}
	assert.True(t, foundArchived, "архивная организация должна быть видна с include_archived")
}

func TestOrganizations_Restore(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	created := testutil.ParseMap(t, testutil.POST(t, e, "/organizations", `{"name":"To Restore","type":"Отдел"}`, testutil.AuthHeader(token)))
	orgID := int(created["id"].(float64))

	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/organizations/%d", orgID), testutil.AuthHeader(token)).Code)

	rec := testutil.POST(t, e, fmt.Sprintf("/organizations/%d/restore", orgID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Organization restored", testutil.ParseMessage(t, rec))

	// После восстановления снова в списке по умолчанию.
	def := testutil.ParseSlice(t, testutil.GET(t, e, "/organizations/with-users", testutil.AuthHeader(token)))
	var found bool
	for _, o := range def {
		if int(o["id"].(float64)) == orgID {
			found = true
			assert.Equal(t, true, o["is_active"])
		}
	}
	assert.True(t, found, "восстановленная организация должна быть в списке по умолчанию")
}

func TestOrganizations_Restore_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, fmt.Sprintf("/organizations/%d/restore", td.OrgID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestOrganizations_History(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	created := testutil.ParseMap(t, testutil.POST(t, e, "/organizations", `{"name":"Ист Орг","type":"Организация"}`, h))
	id := int(created["id"].(float64))
	// Смена только имени (тип тот же) - «renamed».
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d", id), `{"name":"Ист Орг 2","type":"Организация"}`, h).Code)
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/organizations/%d", id), h).Code)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, fmt.Sprintf("/organizations/%d/restore", id), "", h).Code)

	hist := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/history", id), h))
	require.Len(t, hist, 4)
	// Новые сверху: restored, archived, renamed, created.
	assert.Equal(t, "restored", hist[0]["action_type"])
	assert.Equal(t, "archived", hist[1]["action_type"])
	assert.Equal(t, "renamed", hist[2]["action_type"])
	assert.Equal(t, "created", hist[3]["action_type"])
	assert.NotEmpty(t, hist[0]["actor_name"])

	// Различаем, что изменилось: только тип -> «retyped», имя+тип -> «updated».
	created2 := testutil.ParseMap(t, testutil.POST(t, e, "/organizations", `{"name":"Тип Орг","type":"Подрядчик"}`, h))
	id2 := int(created2["id"].(float64))
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d", id2), `{"name":"Тип Орг","type":"Отдел"}`, h).Code)
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d", id2), `{"name":"Тип Орг 2","type":"Арендатор"}`, h).Code)

	hist2 := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/history", id2), h))
	require.Len(t, hist2, 3)
	assert.Equal(t, "updated", hist2[0]["action_type"])
	assert.Equal(t, "retyped", hist2[1]["action_type"])
	assert.Equal(t, "created", hist2[2]["action_type"])
}

func TestOrganizations_Restore_NameConflict_Fails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Создаём «Конфликт», архивируем, создаём новую активную «Конфликт».
	first := testutil.ParseMap(t, testutil.POST(t, e, "/organizations", `{"name":"Конфликт","type":"Организация"}`, testutil.AuthHeader(token)))
	firstID := int(first["id"].(float64))
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/organizations/%d", firstID), testutil.AuthHeader(token)).Code)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/organizations", `{"name":"Конфликт","type":"Организация"}`, testutil.AuthHeader(token)).Code)

	// Восстановление первой невозможно - активное имя занято.
	rec := testutil.POST(t, e, fmt.Sprintf("/organizations/%d/restore", firstID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOrganizations_Create_DuplicateActiveName_Fails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/organizations", `{"name":"Dup Org","type":"Организация"}`, testutil.AuthHeader(token)).Code)
	rec := testutil.POST(t, e, "/organizations", `{"name":"Dup Org","type":"Организация"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
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
	assert.Contains(t, orgs[0], "type")
	assert.Contains(t, orgs[0], "user_count")
	assert.Contains(t, orgs[0], "unload_places")
	// Засиженная SeedTestData организация без типа -> «не указан» (NULL/nil).
	for _, o := range orgs {
		if int(o["id"].(float64)) == td.OrgID {
			assert.Nil(t, o["type"], "у старой организации тип не указан (NULL)")
		}
	}
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

// TestOrganizations_GetOrganizationUsers_RequiredApprovalGated (#2013): признак
// обязательного согласующего - карта того, кто в организации проводит решения.
// Маршрут открыт любому вошедшему - его же дёргает форма подачи заявки у обычного
// заявителя (CreateApplication.vue: loadDefaultApprovers, сбор required_users при
// отправке), чтобы показать и проставить дефолтных согласующих СВОЕЙ организации.
// Поэтому признак виден заявителю для СВОЕЙ организации, но не для чужой, и в любом
// случае виден тому, у кого есть право на раздел справочников (админ, редактирующий
// состав произвольной организации через ResponsibleUsersSection.vue).
func TestOrganizations_GetOrganizationUsers_RequiredApprovalGated(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	foreignOrgID, _ := seedOrgAndCompany(t, db, "OrgUsersGateForeign")
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterUser(t, e, "reqapproveruser", "pass123", 1, td.OrgID, td.CompanyID)
	body := `{"users":[{"username":"reqapproveruser","required_approval":true}]}`
	require.Equal(t, http.StatusOK,
		testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID), body, testutil.AuthHeader(adminToken)).Code)

	// Заявитель из ТОЙ ЖЕ организации: форма подачи должна увидеть настоящее значение -
	// иначе чипы дефолтных согласующих (#884) перестанут показываться.
	ownToken := testutil.RegisterAndLogin(t, e, "orgapplicant_own", "pass123", 1, td.OrgID, td.CompanyID)
	recOwn := testutil.GET(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID), testutil.AuthHeader(ownToken))
	require.Equal(t, http.StatusOK, recOwn.Code)
	usersOwn := testutil.ParseSlice(t, recOwn)
	require.Len(t, usersOwn, 1)
	assert.Equal(t, true, usersOwn[0]["required_approval"], "заявитель должен видеть признак для своей организации")

	// Заявитель из ЧУЖОЙ организации: маршрут отдаёт 200 (форма не ломается), но
	// признак согласования скрыт - иначе любой вошедший узнаёт карту согласования
	// чужой организации по id.
	foreignToken := testutil.RegisterAndLogin(t, e, "orgapplicant_foreign", "pass123", 1, foreignOrgID, td.CompanyID)
	recForeign := testutil.GET(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID), testutil.AuthHeader(foreignToken))
	require.Equal(t, http.StatusOK, recForeign.Code)
	usersForeign := testutil.ParseSlice(t, recForeign)
	require.Len(t, usersForeign, 1)
	assert.Nil(t, usersForeign[0]["required_approval"], "заявитель чужой организации не должен видеть признак")

	// Пользователь с правом на справочники видит настоящее значение для ЛЮБОЙ
	// организации, включая чужую по отношению к нему самому.
	directoriesToken := testutil.RegisterAndLogin(t, e, "directoriesviewer", "pass123", 1, foreignOrgID, td.CompanyID)
	testutil.GrantPermission(t, getUserID(t, db, "directoriesviewer"), services.KeyPageAdminDirectories)
	recPriv := testutil.GET(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID), testutil.AuthHeader(directoriesToken))
	require.Equal(t, http.StatusOK, recPriv.Code)
	usersPriv := testutil.ParseSlice(t, recPriv)
	require.Len(t, usersPriv, 1)
	assert.Equal(t, true, usersPriv[0]["required_approval"], "право на справочники должно раскрывать признак для любой организации")
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

	// #1046: /members возвращает привязанных по organization_id (участники), а не
	// ответственных из junction. Заводим ответственного, НЕ являющегося участником
	// (organization_id пуст), и неактивного участника - оба вне members.
	testutil.RegisterUser(t, e, "orgrespnotmember", "pass123456", 1, 0, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/users", td.OrgID),
		`{"users":[{"username":"orgrespnotmember"}]}`, testutil.AuthHeader(token)).Code)

	inactiveOrg := td.OrgID
	inactive := models.User{Username: "orginactivemember", Password: "x", OrganizationID: &inactiveOrg, TypeID: 1}
	require.NoError(t, db.Create(&inactive).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", inactive.ID).Update("is_active", false).Error)

	members := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/members", td.OrgID), testutil.AuthHeader(token)))
	names := map[string]bool{}
	for _, m := range members {
		names[m["username"].(string)] = true
	}
	assert.True(t, names["testadmin"], "активный привязанный пользователь должен быть в members")
	assert.False(t, names["orgrespnotmember"], "ответственный без organization_id не участник")
	assert.False(t, names["orginactivemember"], "неактивный участник исключён")
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

// bulkIDSet собирает множество id из ответа-списка (для проверки привязок).
func bulkIDSet(items []map[string]interface{}) map[int]bool {
	s := map[int]bool{}
	for _, it := range items {
		s[int(it["id"].(float64))] = true
	}
	return s
}

// TestOrganizations_BulkOperations покрывает все групповые эндпоинты под одним
// SetupTestApp/RegisterAdmin (handlers-пакет на грани CI-timeout под -race, argon2
// в RegisterAdmin дорог - не плодим setup). Подтесты изолированы созданием
// собственных организаций.
func TestOrganizations_BulkOperations(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	createOrg := func(t *testing.T, name string) int {
		return int(testutil.ParseMap(t, testutil.POST(t, e, "/organizations", fmt.Sprintf(`{"name":%q,"type":"Организация"}`, name), h))["id"].(float64))
	}

	t.Run("type", func(t *testing.T) {
		a := createOrg(t, "Бул А")
		b := createOrg(t, "Бул Б")

		rec := testutil.POST(t, e, "/organizations/bulk/type", fmt.Sprintf(`{"ids":[%d,%d],"type":"Подрядчик"}`, a, b), h)
		require.Equal(t, http.StatusOK, rec.Code)
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(2), res["success_count"])
		assert.Equal(t, float64(0), res["error_count"])

		// Оба реально сменили тип.
		types := map[int]string{}
		for _, o := range testutil.ParseSlice(t, testutil.GET(t, e, "/organizations/with-users-extended", h)) {
			if tp, ok := o["type"].(string); ok {
				types[int(o["id"].(float64))] = tp
			}
		}
		assert.Equal(t, "Подрядчик", types[a])
		assert.Equal(t, "Подрядчик", types[b])

		// Назначение УЖЕ соответствующего типа - no-op успех БЕЗ записи в историю
		// (иначе переиспользуемый Update при неизменных name+type залогировал бы
		// дефолтный «renamed»).
		noop := createOrg(t, "Бул Ноуп") // создаётся с типом "Организация"
		noopRes := testutil.ParseMap(t, testutil.POST(t, e, "/organizations/bulk/type", fmt.Sprintf(`{"ids":[%d],"type":"Организация"}`, noop), h))
		assert.Equal(t, float64(1), noopRes["success_count"], "no-op смена типа - успех")
		hist := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/history", noop), h))
		require.Len(t, hist, 1, "no-op не должен добавлять запись в историю")
		assert.Equal(t, "created", hist[0]["action_type"])

		// Дубли id в наборе не раздувают success_count (dedup).
		dupRes := testutil.ParseMap(t, testutil.POST(t, e, "/organizations/bulk/type", fmt.Sprintf(`{"ids":[%d,%d],"type":"Отдел"}`, noop, noop), h))
		assert.Equal(t, float64(1), dupRes["success_count"], "дубли id дедуплицируются")

		// Частичный успех: несуществующий id -> в errors, существующий проходит, статус 207.
		prec := testutil.POST(t, e, "/organizations/bulk/type", fmt.Sprintf(`{"ids":[%d,999999],"type":"Отдел"}`, a), h)
		require.Equal(t, http.StatusMultiStatus, prec.Code)
		pres := testutil.ParseMap(t, prec)
		assert.Equal(t, float64(1), pres["success_count"])
		assert.Equal(t, float64(1), pres["error_count"])
		errs := pres["errors"].([]interface{})
		require.Len(t, errs, 1)
		assert.Equal(t, float64(999999), errs[0].(map[string]interface{})["id"])

		// Невалидный тип -> 400 на весь запрос (валидация до цикла).
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/organizations/bulk/type", fmt.Sprintf(`{"ids":[%d],"type":"Ерунда"}`, a), h).Code)
		// Пустой список -> 400.
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/organizations/bulk/type", `{"ids":[],"type":"Отдел"}`, h).Code)
	})

	t.Run("unload-places", func(t *testing.T) {
		a := createOrg(t, "Плейс А")
		b := createOrg(t, "Плейс Б")
		p1 := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Место 1","description":"d","status":"active"}`, h))["id"].(float64))
		p2 := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Место 2","description":"d","status":"active"}`, h))["id"].(float64))

		// replace: p1 обеим.
		rec := testutil.POST(t, e, "/organizations/bulk/unload-places", fmt.Sprintf(`{"ids":[%d,%d],"unload_place_ids":[%d],"mode":"replace"}`, a, b, p1), h)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["success_count"])
		for _, id := range []int{a, b} {
			set := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/unload-places", id), h)))
			assert.True(t, set[p1] && len(set) == 1, "после replace у организации ровно p1")
		}

		// add: p2 обеим -> union {p1,p2}.
		rec2 := testutil.POST(t, e, "/organizations/bulk/unload-places", fmt.Sprintf(`{"ids":[%d,%d],"unload_place_ids":[%d],"mode":"add"}`, a, b, p2), h)
		require.Equal(t, http.StatusOK, rec2.Code)
		for _, id := range []int{a, b} {
			set := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/unload-places", id), h)))
			assert.True(t, set[p1] && set[p2] && len(set) == 2, "после add у организации p1 и p2")
		}

		// Некорректный режим -> 400.
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/organizations/bulk/unload-places", fmt.Sprintf(`{"ids":[%d],"unload_place_ids":[%d],"mode":"bogus"}`, a, p1), h).Code)
	})

	t.Run("tables", func(t *testing.T) {
		a := createOrg(t, "Табл А")
		b := createOrg(t, "Табл Б")
		t1 := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", `{"name":"bulk-t1","display_name":"Bulk T1","table_type":"cars"}`, h))["id"].(float64))
		t2 := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", `{"name":"bulk-t2","display_name":"Bulk T2","table_type":"cars"}`, h))["id"].(float64))

		// replace: t1 обеим.
		rec := testutil.POST(t, e, "/organizations/bulk/tables", fmt.Sprintf(`{"ids":[%d,%d],"table_ids":[%d],"mode":"replace"}`, a, b, t1), h)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["success_count"])
		for _, id := range []int{a, b} {
			set := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/tables", id), h)))
			assert.True(t, set[t1] && len(set) == 1)
		}

		// add: t2 обеим -> union.
		rec2 := testutil.POST(t, e, "/organizations/bulk/tables", fmt.Sprintf(`{"ids":[%d,%d],"table_ids":[%d],"mode":"add"}`, a, b, t2), h)
		require.Equal(t, http.StatusOK, rec2.Code)
		for _, id := range []int{a, b} {
			set := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/tables", id), h)))
			assert.True(t, set[t1] && set[t2] && len(set) == 2)
		}
	})

	t.Run("users", func(t *testing.T) {
		a := createOrg(t, "Юзер А")
		b := createOrg(t, "Юзер Б")
		testutil.RegisterUser(t, e, "bulkresp1", "pass123", 1, td.OrgID, td.CompanyID)
		testutil.RegisterUser(t, e, "bulkresp2", "pass123", 1, td.OrgID, td.CompanyID)

		// replace: resp1 обеим с required_approval=true; primary не назначается.
		rec := testutil.POST(t, e, "/organizations/bulk/users", fmt.Sprintf(`{"ids":[%d,%d],"users":[{"username":"bulkresp1","required_approval":true}],"mode":"replace"}`, a, b), h)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["success_count"])
		usersA := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/users", a), h))
		require.Len(t, usersA, 1)
		assert.Equal(t, "bulkresp1", usersA[0]["username"])
		assert.Equal(t, true, usersA[0]["required_approval"])
		assert.Equal(t, false, usersA[0]["is_primary"])

		// add: resp2 обеим -> {resp1(required_approval сохранён true), resp2(false)}.
		rec2 := testutil.POST(t, e, "/organizations/bulk/users", fmt.Sprintf(`{"ids":[%d,%d],"users":[{"username":"bulkresp2","required_approval":false}],"mode":"add"}`, a, b), h)
		require.Equal(t, http.StatusOK, rec2.Code)
		byName := map[string]map[string]interface{}{}
		for _, u := range testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/users", a), h)) {
			byName[u["username"].(string)] = u
		}
		require.Len(t, byName, 2)
		assert.Equal(t, true, byName["bulkresp1"]["required_approval"], "add сохраняет флаги существующего")
		assert.Equal(t, false, byName["bulkresp2"]["required_approval"])

		// primary сохраняется в replace + required_approval ИНДИВИДУАЛЕН на каждого:
		// назначим resp1 primary напрямую, затем bulk-replace [resp1:approval=true, resp2:approval=false].
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/users", a),
			`{"users":[{"username":"bulkresp1","is_primary":true},{"username":"bulkresp2"}]}`, h).Code)
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/organizations/bulk/users",
			fmt.Sprintf(`{"ids":[%d],"users":[{"username":"bulkresp1","required_approval":true},{"username":"bulkresp2","required_approval":false}],"mode":"replace"}`, a), h).Code)
		byName2 := map[string]map[string]interface{}{}
		for _, u := range testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/users", a), h)) {
			byName2[u["username"].(string)] = u
		}
		assert.Equal(t, true, byName2["bulkresp1"]["is_primary"], "primary оставшегося сохраняется при replace")
		assert.Equal(t, false, byName2["bulkresp2"]["is_primary"])
		assert.Equal(t, true, byName2["bulkresp1"]["required_approval"], "required_approval индивидуален: resp1=true")
		assert.Equal(t, false, byName2["bulkresp2"]["required_approval"], "required_approval индивидуален: resp2=false")
	})

	t.Run("archive-restore", func(t *testing.T) {
		// td.OrgID активна и с пользователями (admin) -> архив заблокирован; пустая -> ок.
		empty := int(testutil.ParseMap(t, testutil.POST(t, e, "/organizations", `{"name":"Пустая для архива","type":"Отдел"}`, h))["id"].(float64))

		rec := testutil.POST(t, e, "/organizations/bulk/archive", fmt.Sprintf(`{"ids":[%d,%d]}`, td.OrgID, empty), h)
		require.Equal(t, http.StatusMultiStatus, rec.Code)
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(1), res["success_count"])
		assert.Equal(t, float64(1), res["error_count"])
		errs := res["errors"].([]interface{})
		require.Len(t, errs, 1)
		e0 := errs[0].(map[string]interface{})
		assert.Equal(t, float64(td.OrgID), e0["id"])
		assert.Equal(t, "Test Organization", e0["name"])
		assert.Contains(t, e0["error"].(string), "активными пользователями")

		// empty реально в архиве.
		archSet := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, "/organizations/with-users?include_archived=true", h)))
		assert.True(t, archSet[empty])
		defSet := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, "/organizations/with-users", h)))
		assert.False(t, defSet[empty], "архивная не в списке по умолчанию")

		// bulk restore -> снова активна.
		rrec := testutil.POST(t, e, "/organizations/bulk/restore", fmt.Sprintf(`{"ids":[%d]}`, empty), h)
		require.Equal(t, http.StatusOK, rrec.Code)
		assert.Equal(t, float64(1), testutil.ParseMap(t, rrec)["success_count"])
		assert.True(t, bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, "/organizations/with-users", h)))[empty])
	})
}

func TestOrganizations_Bulk_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID))

	// Гейт requireAdmin бьёт до цикла - 403 на весь запрос.
	assert.Equal(t, http.StatusForbidden,
		testutil.POST(t, e, "/organizations/bulk/type", fmt.Sprintf(`{"ids":[%d],"type":"Отдел"}`, td.OrgID), h).Code)
	assert.Equal(t, http.StatusForbidden,
		testutil.POST(t, e, "/organizations/bulk/archive", fmt.Sprintf(`{"ids":[%d]}`, td.OrgID), h).Code)
}

// TestOrganizations_BulkAudit проверяет, что групповые и одиночные изменения привязок
// (места/таблицы/ответственные) пишут в audit_log запись «было -> стало» с деталями,
// и что неизменяющая операция запись НЕ добавляет (skip на пустом diff).
func TestOrganizations_BulkAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	createOrg := func(name string) int {
		return int(testutil.ParseMap(t, testutil.POST(t, e, "/organizations", fmt.Sprintf(`{"name":%q,"type":"Организация"}`, name), h))["id"].(float64))
	}
	history := func(id int) []map[string]interface{} {
		return testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/history", id), h))
	}
	findAction := func(hist []map[string]interface{}, action string) map[string]interface{} {
		for _, it := range hist {
			if it["action_type"] == action {
				return it
			}
		}
		return nil
	}
	detailsOf := func(it map[string]interface{}) map[string]interface{} {
		d, _ := it["details"].(map[string]interface{})
		return d
	}
	names := func(v interface{}) []string {
		arr, _ := v.([]interface{})
		out := make([]string, 0, len(arr))
		for _, x := range arr {
			out = append(out, x.(string))
		}
		return out
	}

	t.Run("unload-places added/removed - bulk и одиночный путь", func(t *testing.T) {
		org := createOrg("Аудит Места")
		p1 := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Аудит Место 1","description":"d","status":"active"}`, h))["id"].(float64))
		p2 := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Аудит Место 2","description":"d","status":"active"}`, h))["id"].(float64))

		// bulk replace: обе привязки добавлены.
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/organizations/bulk/unload-places",
			fmt.Sprintf(`{"ids":[%d],"unload_place_ids":[%d,%d],"mode":"replace"}`, org, p1, p2), h).Code)
		rec := findAction(history(org), "unload_places_changed")
		require.NotNil(t, rec, "bulk назначение мест пишет запись в историю")
		assert.NotEmpty(t, rec["actor_name"], "актор проставлен")
		assert.ElementsMatch(t, []string{"Аудит Место 1", "Аудит Место 2"}, names(detailsOf(rec)["added"]))
		assert.Empty(t, names(detailsOf(rec)["removed"]))

		// Одиночный путь (PUT) с тем же набором - no-op, новой записи нет.
		before := len(history(org))
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/unload-places", org),
			fmt.Sprintf(`{"unload_place_ids":[%d,%d]}`, p1, p2), h).Code)
		assert.Len(t, history(org), before, "неизменяющее обновление не пишет историю")

		// Одиночный путь снимает p2 - запись с removed.
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/unload-places", org),
			fmt.Sprintf(`{"unload_place_ids":[%d]}`, p1), h).Code)
		rec2 := findAction(history(org), "unload_places_changed")
		require.NotNil(t, rec2)
		assert.ElementsMatch(t, []string{"Аудит Место 2"}, names(detailsOf(rec2)["removed"]))
		assert.Empty(t, names(detailsOf(rec2)["added"]))
	})

	t.Run("type no-op одиночный PUT не пишет историю", func(t *testing.T) {
		org := createOrg("Аудит Тип")
		before := len(history(org)) // created
		// PUT с теми же name+type - ничего не меняется, ложная «переименована» не пишется.
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d", org),
			`{"name":"Аудит Тип","type":"Организация"}`, h).Code)
		assert.Len(t, history(org), before, "неизменяющий PUT не добавляет запись")
		// Реальная смена типа - запись с from.
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d", org),
			`{"name":"Аудит Тип","type":"Отдел"}`, h).Code)
		rec := findAction(history(org), "retyped")
		require.NotNil(t, rec)
		from, _ := detailsOf(rec)["from"].(map[string]interface{})
		require.NotNil(t, from, "запись смены типа несёт from")
		assert.Equal(t, "Организация", from["type"])
	})

	t.Run("tables added", func(t *testing.T) {
		org := createOrg("Аудит Таблицы")
		t1 := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", `{"name":"audit-t1","display_name":"Аудит Т1","table_type":"cars"}`, h))["id"].(float64))
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/organizations/bulk/tables",
			fmt.Sprintf(`{"ids":[%d],"table_ids":[%d],"mode":"replace"}`, org, t1), h).Code)
		rec := findAction(history(org), "tables_changed")
		require.NotNil(t, rec, "bulk назначение таблиц пишет историю")
		assert.ElementsMatch(t, []string{"Аудит Т1"}, names(detailsOf(rec)["added"]))
	})

	t.Run("responsibles added + approval_changed", func(t *testing.T) {
		org := createOrg("Аудит Ответственные")
		testutil.RegisterUser(t, e, "auditresp1", "pass123", 1, td.OrgID, td.CompanyID)

		// Назначили resp1 с approval=false - запись responsibles_changed с added.
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/organizations/bulk/users",
			fmt.Sprintf(`{"ids":[%d],"users":[{"username":"auditresp1","required_approval":false}],"mode":"replace"}`, org), h).Code)
		rec := findAction(history(org), "responsibles_changed")
		require.NotNil(t, rec, "bulk назначение ответственных пишет историю")
		added, _ := detailsOf(rec)["added"].([]interface{})
		require.Len(t, added, 1)
		a0 := added[0].(map[string]interface{})
		assert.Equal(t, "auditresp1", a0["username"])
		assert.Equal(t, false, a0["required_approval"])

		// Тот же набор, но approval=true - запись только с approval_changed (added/removed пусты).
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/organizations/bulk/users",
			fmt.Sprintf(`{"ids":[%d],"users":[{"username":"auditresp1","required_approval":true}],"mode":"replace"}`, org), h).Code)
		rec2 := findAction(history(org), "responsibles_changed")
		require.NotNil(t, rec2)
		ch, _ := detailsOf(rec2)["approval_changed"].([]interface{})
		require.Len(t, ch, 1, "смена флага согласования пишется в approval_changed")
		c0 := ch[0].(map[string]interface{})
		assert.Equal(t, "auditresp1", c0["username"])
		assert.Equal(t, false, c0["from"])
		assert.Equal(t, true, c0["to"])
		_, hasAdded := detailsOf(rec2)["added"]
		assert.False(t, hasAdded, "при одной лишь смене согласования added отсутствует")

		// Повтор того же набора без изменений - истории не прибавляется.
		before := len(history(org))
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/organizations/bulk/users",
			fmt.Sprintf(`{"ids":[%d],"users":[{"username":"auditresp1","required_approval":true}],"mode":"replace"}`, org), h).Code)
		assert.Len(t, history(org), before, "неизменяющее назначение ответственных не пишет историю")
	})

	t.Run("смена главного ответственного пишет primary_changed", func(t *testing.T) {
		org := createOrg("Аудит Главный")
		testutil.RegisterUser(t, e, "auditprim1", "pass123", 1, td.OrgID, td.CompanyID)
		testutil.RegisterUser(t, e, "auditprim2", "pass123", 1, td.OrgID, td.CompanyID)

		// Назначили оба, главный - prim1.
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/users", org),
			`{"users":[{"username":"auditprim1","is_primary":true},{"username":"auditprim2"}]}`, h).Code)
		// Сменили главного на prim2 (набор и согласование те же).
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/organizations/%d/users", org),
			`{"users":[{"username":"auditprim1"},{"username":"auditprim2","is_primary":true}]}`, h).Code)

		rec := findAction(history(org), "responsibles_changed")
		require.NotNil(t, rec, "смена главного пишет историю")
		pc, _ := detailsOf(rec)["primary_changed"].(map[string]interface{})
		require.NotNil(t, pc, "смена главного пишет primary_changed")
		from, _ := pc["from"].(map[string]interface{})
		to, _ := pc["to"].(map[string]interface{})
		assert.Equal(t, "auditprim1", from["username"])
		assert.Equal(t, "auditprim2", to["username"])
		// Набор не менялся - added/removed отсутствуют.
		_, hasAdded := detailsOf(rec)["added"]
		assert.False(t, hasAdded, "при одной лишь смене главного added отсутствует")
	})
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

// createOrg заводит организацию через API и возвращает её ID (helper для reassign-тестов).
func createOrg(t *testing.T, e *echo.Echo, token, name string) int {
	t.Helper()
	rec := testutil.POST(t, e, "/organizations", fmt.Sprintf(`{"name":%q,"type":"Отдел"}`, name), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "create org: %s", rec.Body.String())
	return int(testutil.ParseMap(t, rec)["id"].(float64))
}

// TestOrganizations_BlockingUsersAndReassign проверяет полный флоу: список блокеров,
// перенос всех в другую организацию освобождает исходную (её можно архивировать),
// повторный перенос идемпотентен, аудит смены org пишется на каждого.
func TestOrganizations_BlockingUsersAndReassign(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	srcID := createOrg(t, e, token, "Источник")
	tgtID := createOrg(t, e, token, "Цель")

	// Два активных участника исходной организации - они блокируют архивацию.
	testutil.RegisterUser(t, e, "blocker1", "pass123", 1, srcID, td.CompanyID)
	testutil.RegisterUser(t, e, "blocker2", "pass123", 1, srcID, td.CompanyID)
	// Неактивный участник: не блокирует и НЕ должен переноситься.
	srcRef := srcID
	inactive := models.User{Username: "blockerarchived", Password: "x", OrganizationID: &srcRef, TypeID: 1}
	require.NoError(t, db.Create(&inactive).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", inactive.ID).Update("is_active", false).Error)

	// Список блокеров = только активные участники.
	blockers := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/members", srcID), testutil.AuthHeader(token)))
	names := map[string]bool{}
	for _, b := range blockers {
		names[b["username"].(string)] = true
	}
	assert.True(t, names["blocker1"] && names["blocker2"], "оба активных участника в блокерах")
	assert.False(t, names["blockerarchived"], "неактивный участник не блокер")
	assert.Len(t, blockers, 2)

	// Пока есть активные - архивация запрещена.
	assert.Equal(t, http.StatusBadRequest, testutil.DELETE(t, e, fmt.Sprintf("/organizations/%d", srcID), testutil.AuthHeader(token)).Code)

	// Перенос всех блокеров в целевую.
	rec := testutil.POST(t, e, fmt.Sprintf("/organizations/%d/reassign-users", srcID), fmt.Sprintf(`{"target_id":%d}`, tgtID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["reassigned"])

	// Блокеры теперь в целевой, исходная свободна.
	tgtMembers := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/members", tgtID), testutil.AuthHeader(token)))
	tgtNames := map[string]bool{}
	for _, m := range tgtMembers {
		tgtNames[m["username"].(string)] = true
	}
	assert.True(t, tgtNames["blocker1"] && tgtNames["blocker2"], "оба перенесены в целевую")
	assert.Empty(t, testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/members", srcID), testutil.AuthHeader(token))), "исходная без блокеров")

	// Аудит смены организации записан на каждого перенесённого.
	var auditCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND action = ?", models.AuditEntityUser, models.UserActionOrgChanged).
		Count(&auditCount).Error)
	assert.EqualValues(t, 2, auditCount, "org_changed аудит на каждого перенесённого")

	// Теперь исходную можно архивировать.
	assert.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/organizations/%d", srcID), testutil.AuthHeader(token)).Code)

	// Идемпотентность: повторный перенос без блокеров - 200, reassigned:0.
	again := testutil.POST(t, e, fmt.Sprintf("/organizations/%d/reassign-users", srcID), fmt.Sprintf(`{"target_id":%d}`, tgtID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, again.Code, again.Body.String())
	assert.Equal(t, float64(0), testutil.ParseMap(t, again)["reassigned"])
}

// TestOrganizations_ReassignUsers_Validation проверяет гейт и валидацию цели.
func TestOrganizations_ReassignUsers_Validation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	srcID := createOrg(t, e, token, "Источник-В")
	tgtID := createOrg(t, e, token, "Цель-В")
	archivedID := createOrg(t, e, token, "Архив-В")
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/organizations/%d", archivedID), testutil.AuthHeader(token)).Code)

	reassign := func(id int, body string) int {
		return testutil.POST(t, e, fmt.Sprintf("/organizations/%d/reassign-users", id), body, testutil.AuthHeader(token)).Code
	}

	// Не указана цель / нулевая цель.
	assert.Equal(t, http.StatusBadRequest, reassign(srcID, `{}`))
	assert.Equal(t, http.StatusBadRequest, reassign(srcID, `{"target_id":0}`))
	// Цель = источнику.
	assert.Equal(t, http.StatusBadRequest, reassign(srcID, fmt.Sprintf(`{"target_id":%d}`, srcID)))
	// Несуществующая цель.
	assert.Equal(t, http.StatusBadRequest, reassign(srcID, `{"target_id":999999}`))
	// Архивная цель.
	assert.Equal(t, http.StatusBadRequest, reassign(srcID, fmt.Sprintf(`{"target_id":%d}`, archivedID)))
	// Несуществующий источник.
	assert.Equal(t, http.StatusNotFound, reassign(999999, fmt.Sprintf(`{"target_id":%d}`, tgtID)))

	// Обычный пользователь (не админ) не имеет доступа к обоим endpoint-ам.
	userToken := testutil.RegisterAndLogin(t, e, "plainuser", "pass123", 1, td.OrgID, td.CompanyID)
	assert.Equal(t, http.StatusForbidden, testutil.GET(t, e, fmt.Sprintf("/organizations/%d/members", srcID), testutil.AuthHeader(userToken)).Code)
	assert.Equal(t, http.StatusForbidden, testutil.POST(t, e, fmt.Sprintf("/organizations/%d/reassign-users", srcID), fmt.Sprintf(`{"target_id":%d}`, tgtID), testutil.AuthHeader(userToken)).Code)
}
