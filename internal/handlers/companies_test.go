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

	body := `{"name":"New Company","type":"Организация"}`
	rec := testutil.POST(t, e, "/companies", body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	comp := testutil.ParseMap(t, rec)
	assert.Equal(t, "New Company", comp["name"])
	assert.Equal(t, "Организация", comp["type"])
	assert.NotZero(t, comp["id"])

	// #1046: тип обязателен и должен быть валиден - невалидный и пустой дают 400.
	assert.Equal(t, http.StatusBadRequest,
		testutil.POST(t, e, "/companies", `{"name":"Плохой тип","type":"Ерунда"}`, testutil.AuthHeader(token)).Code)
	assert.Equal(t, http.StatusBadRequest,
		testutil.POST(t, e, "/companies", `{"name":"Без типа"}`, testutil.AuthHeader(token)).Code)
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

	// #1046: тип опционален - можно задать валидный и снять через null.
	changed := testutil.PUT(t, e, fmt.Sprintf("/companies/%d", td.CompanyID), `{"name":"Updated Company","type":"Подрядчик"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, changed.Code)
	assert.Equal(t, "Подрядчик", testutil.ParseMap(t, changed)["type"])

	cleared := testutil.PUT(t, e, fmt.Sprintf("/companies/%d", td.CompanyID), `{"name":"Updated Company","type":null}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, cleared.Code)
	assert.Nil(t, testutil.ParseMap(t, cleared)["type"])

	// Невалидный тип при обновлении - 400.
	assert.Equal(t, http.StatusBadRequest,
		testutil.PUT(t, e, fmt.Sprintf("/companies/%d", td.CompanyID), `{"name":"Updated Company","type":"Ерунда"}`, testutil.AuthHeader(token)).Code)
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
	createRec := testutil.POST(t, e, "/companies", `{"name":"To Delete","type":"Отдел"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, createRec.Code)
	created := testutil.ParseMap(t, createRec)
	compID := int(created["id"].(float64))

	rec := testutil.DELETE(t, e, fmt.Sprintf("/companies/%d", compID), testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Company archived", msg)

	// Архивная компания скрыта из списка по умолчанию, видна с include_archived.
	def := testutil.ParseSlice(t, testutil.GET(t, e, "/companies/with-users", testutil.AuthHeader(token)))
	for _, c := range def {
		assert.NotEqual(t, float64(compID), c["id"], "архивная компания не должна быть в списке по умолчанию")
	}
	arch := testutil.ParseSlice(t, testutil.GET(t, e, "/companies/with-users?include_archived=true", testutil.AuthHeader(token)))
	var found bool
	for _, c := range arch {
		if int(c["id"].(float64)) == compID {
			found = true
			assert.Equal(t, false, c["is_active"])
		}
	}
	assert.True(t, found, "архивная компания должна быть видна с include_archived")
}

func TestCompanies_Restore(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	created := testutil.ParseMap(t, testutil.POST(t, e, "/companies", `{"name":"To Restore","type":"Отдел"}`, testutil.AuthHeader(token)))
	compID := int(created["id"].(float64))
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/companies/%d", compID), testutil.AuthHeader(token)).Code)

	rec := testutil.POST(t, e, fmt.Sprintf("/companies/%d/restore", compID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Company restored", testutil.ParseMessage(t, rec))

	def := testutil.ParseSlice(t, testutil.GET(t, e, "/companies/with-users", testutil.AuthHeader(token)))
	var found bool
	for _, c := range def {
		if int(c["id"].(float64)) == compID {
			found = true
			assert.Equal(t, true, c["is_active"])
		}
	}
	assert.True(t, found, "восстановленная компания должна быть в списке по умолчанию")
}

func TestCompanies_Restore_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, fmt.Sprintf("/companies/%d/restore", td.CompanyID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCompanies_History(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	created := testutil.ParseMap(t, testutil.POST(t, e, "/companies", `{"name":"Ист Ко","type":"Организация"}`, h))
	id := int(created["id"].(float64))
	// Смена только имени (тип тот же) - «renamed».
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d", id), `{"name":"Ист Ко 2","type":"Организация"}`, h).Code)
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/companies/%d", id), h).Code)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, fmt.Sprintf("/companies/%d/restore", id), "", h).Code)

	hist := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/history", id), h))
	require.Len(t, hist, 4)
	assert.Equal(t, "restored", hist[0]["action_type"])
	assert.Equal(t, "archived", hist[1]["action_type"])
	assert.Equal(t, "renamed", hist[2]["action_type"])
	assert.Equal(t, "created", hist[3]["action_type"])
	assert.NotEmpty(t, hist[0]["actor_name"])

	// Различаем, что изменилось: только тип -> «retyped», имя+тип -> «updated».
	created2 := testutil.ParseMap(t, testutil.POST(t, e, "/companies", `{"name":"Тип Ко","type":"Подрядчик"}`, h))
	id2 := int(created2["id"].(float64))
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d", id2), `{"name":"Тип Ко","type":"Отдел"}`, h).Code)
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d", id2), `{"name":"Тип Ко 2","type":"Арендатор"}`, h).Code)

	hist2 := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/history", id2), h))
	require.Len(t, hist2, 3)
	assert.Equal(t, "updated", hist2[0]["action_type"])
	assert.Equal(t, "retyped", hist2[1]["action_type"])
	assert.Equal(t, "created", hist2[2]["action_type"])
}

func TestCompanies_History_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, fmt.Sprintf("/companies/%d/history", td.CompanyID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCompanies_Restore_NameConflict_Fails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	first := testutil.ParseMap(t, testutil.POST(t, e, "/companies", `{"name":"Конфликт Ко","type":"Организация"}`, testutil.AuthHeader(token)))
	firstID := int(first["id"].(float64))
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/companies/%d", firstID), testutil.AuthHeader(token)).Code)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/companies", `{"name":"Конфликт Ко","type":"Организация"}`, testutil.AuthHeader(token)).Code)

	rec := testutil.POST(t, e, fmt.Sprintf("/companies/%d/restore", firstID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCompanies_Create_DuplicateActiveName_Fails(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/companies", `{"name":"Dup Co","type":"Организация"}`, testutil.AuthHeader(token)).Code)
	rec := testutil.POST(t, e, "/companies", `{"name":"Dup Co","type":"Организация"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
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
	assert.Contains(t, companies[0], "type")
	assert.Contains(t, companies[0], "user_count")
	assert.Contains(t, companies[0], "unload_places")
	// Засиженная SeedTestData компания без типа -> «не указан» (NULL/nil).
	for _, c := range companies {
		if int(c["id"].(float64)) == td.CompanyID {
			assert.Nil(t, c["type"], "у старой компании тип не указан (NULL)")
		}
	}
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

// TestCompanies_GetUsers_RequiredApprovalGated (#2013): зеркало
// TestOrganizations_GetOrganizationUsers_RequiredApprovalGated - /companies/:id/users
// открыт так же, как /organizations/:id/users, и признак обязательного согласующего
// гейтится тем же образом: своя компания или право на раздел справочников.
func TestCompanies_GetUsers_RequiredApprovalGated(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	_, foreignCompanyID := seedOrgAndCompany(t, db, "CompUsersGateForeign")
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterUser(t, e, "compreqapprover", "pass123", 1, td.OrgID, td.CompanyID)
	body := `{"users":[{"username":"compreqapprover","required_approval":true}]}`
	require.Equal(t, http.StatusOK,
		testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), body, testutil.AuthHeader(adminToken)).Code)

	ownToken := testutil.RegisterAndLogin(t, e, "compapplicant_own", "pass123", 1, td.OrgID, td.CompanyID)
	recOwn := testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), testutil.AuthHeader(ownToken))
	require.Equal(t, http.StatusOK, recOwn.Code)
	usersOwn := testutil.ParseSlice(t, recOwn)
	require.Len(t, usersOwn, 1)
	assert.Equal(t, true, usersOwn[0]["required_approval"], "заявитель должен видеть признак для своей компании")

	foreignToken := testutil.RegisterAndLogin(t, e, "compapplicant_foreign", "pass123", 1, td.OrgID, foreignCompanyID)
	recForeign := testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), testutil.AuthHeader(foreignToken))
	require.Equal(t, http.StatusOK, recForeign.Code)
	usersForeign := testutil.ParseSlice(t, recForeign)
	require.Len(t, usersForeign, 1)
	assert.Nil(t, usersForeign[0]["required_approval"], "заявитель чужой компании не должен видеть признак")

	directoriesToken := testutil.RegisterAndLogin(t, e, "compdirectoriesviewer", "pass123", 1, td.OrgID, foreignCompanyID)
	testutil.GrantPermission(t, getUserID(t, db, "compdirectoriesviewer"), services.KeyPageAdminDirectories)
	recPriv := testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), testutil.AuthHeader(directoriesToken))
	require.Equal(t, http.StatusOK, recPriv.Code)
	usersPriv := testutil.ParseSlice(t, recPriv)
	require.Len(t, usersPriv, 1)
	assert.Equal(t, true, usersPriv[0]["required_approval"], "право на справочники должно раскрывать признак для любой компании")
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

	// #1046: /members возвращает привязанных по company_id (участники), а не
	// ответственных из junction. Заводим ответственного, НЕ являющегося участником
	// (company_id пуст), и неактивного участника - оба вне members.
	testutil.RegisterUser(t, e, "comprespnotmember", "pass123456", 1, td.OrgID, 0)
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID),
		`{"users":[{"username":"comprespnotmember"}]}`, testutil.AuthHeader(token)).Code)

	inactiveComp := td.CompanyID
	inactive := models.User{Username: "compinactivemember", Password: "x", CompanyID: &inactiveComp, TypeID: 1}
	require.NoError(t, db.Create(&inactive).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", inactive.ID).Update("is_active", false).Error)

	members := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/members", td.CompanyID), testutil.AuthHeader(token)))
	names := map[string]bool{}
	for _, m := range members {
		names[m["username"].(string)] = true
	}
	assert.True(t, names["testadmin"], "активный привязанный пользователь должен быть в members")
	assert.False(t, names["comprespnotmember"], "ответственный без company_id не участник")
	assert.False(t, names["compinactivemember"], "неактивный участник исключён")
}

func TestCompanies_GetUsers_ExcludesArchived(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterUser(t, e, "archcompuser", "pass123", 1, td.OrgID, td.CompanyID)
	body := `{"users":[{"username":"archcompuser","is_primary":true}]}`
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), body, testutil.AuthHeader(token)).Code)

	// До архива ответственный в списке.
	users := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), testutil.AuthHeader(token)))
	require.Len(t, users, 1)

	// Архивируем юзера - должен исчезнуть из ответственных.
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, "/users/archcompuser", testutil.AuthHeader(token)).Code)

	users = testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", td.CompanyID), testutil.AuthHeader(token)))
	assert.Len(t, users, 0)
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

// TestCompanies_BulkOperations зеркалит TestOrganizations_BulkOperations: один
// SetupTestApp/RegisterAdmin (handlers-пакет на грани CI-timeout под -race, argon2
// в RegisterAdmin дорог - не плодим setup), подтесты изолированы созданием
// собственных компаний. bulkIDSet определён в organizations_test.go (тот же пакет).
func TestCompanies_BulkOperations(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	createCompany := func(t *testing.T, name string) int {
		return int(testutil.ParseMap(t, testutil.POST(t, e, "/companies", fmt.Sprintf(`{"name":%q,"type":"Организация"}`, name), h))["id"].(float64))
	}

	t.Run("type", func(t *testing.T) {
		a := createCompany(t, "Бул К А")
		b := createCompany(t, "Бул К Б")

		rec := testutil.POST(t, e, "/companies/bulk/type", fmt.Sprintf(`{"ids":[%d,%d],"type":"Подрядчик"}`, a, b), h)
		require.Equal(t, http.StatusOK, rec.Code)
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(2), res["success_count"])
		assert.Equal(t, float64(0), res["error_count"])

		// Оба реально сменили тип.
		types := map[int]string{}
		for _, o := range testutil.ParseSlice(t, testutil.GET(t, e, "/companies/with-users-extended", h)) {
			if tp, ok := o["type"].(string); ok {
				types[int(o["id"].(float64))] = tp
			}
		}
		assert.Equal(t, "Подрядчик", types[a])
		assert.Equal(t, "Подрядчик", types[b])

		// Назначение УЖЕ соответствующего типа - no-op успех БЕЗ записи в историю
		// (иначе переиспользуемый Update при неизменных name+type залогировал бы
		// дефолтный «renamed»).
		noop := createCompany(t, "Бул К Ноуп") // создаётся с типом "Организация"
		noopRes := testutil.ParseMap(t, testutil.POST(t, e, "/companies/bulk/type", fmt.Sprintf(`{"ids":[%d],"type":"Организация"}`, noop), h))
		assert.Equal(t, float64(1), noopRes["success_count"], "no-op смена типа - успех")
		hist := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/history", noop), h))
		require.Len(t, hist, 1, "no-op не должен добавлять запись в историю")
		assert.Equal(t, "created", hist[0]["action_type"])

		// Дубли id в наборе не раздувают success_count (dedup).
		dupRes := testutil.ParseMap(t, testutil.POST(t, e, "/companies/bulk/type", fmt.Sprintf(`{"ids":[%d,%d],"type":"Отдел"}`, noop, noop), h))
		assert.Equal(t, float64(1), dupRes["success_count"], "дубли id дедуплицируются")

		// Частичный успех: несуществующий id -> в errors, существующий проходит, статус 207.
		prec := testutil.POST(t, e, "/companies/bulk/type", fmt.Sprintf(`{"ids":[%d,999999],"type":"Отдел"}`, a), h)
		require.Equal(t, http.StatusMultiStatus, prec.Code)
		pres := testutil.ParseMap(t, prec)
		assert.Equal(t, float64(1), pres["success_count"])
		assert.Equal(t, float64(1), pres["error_count"])
		errs := pres["errors"].([]interface{})
		require.Len(t, errs, 1)
		assert.Equal(t, float64(999999), errs[0].(map[string]interface{})["id"])

		// Невалидный тип -> 400 на весь запрос (валидация до цикла).
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/companies/bulk/type", fmt.Sprintf(`{"ids":[%d],"type":"Ерунда"}`, a), h).Code)
		// Пустой список -> 400.
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/companies/bulk/type", `{"ids":[],"type":"Отдел"}`, h).Code)
	})

	t.Run("unload-places", func(t *testing.T) {
		a := createCompany(t, "Плейс К А")
		b := createCompany(t, "Плейс К Б")
		p1 := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Место К 1","description":"d","status":"active"}`, h))["id"].(float64))
		p2 := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Место К 2","description":"d","status":"active"}`, h))["id"].(float64))

		// replace: p1 обеим.
		rec := testutil.POST(t, e, "/companies/bulk/unload-places", fmt.Sprintf(`{"ids":[%d,%d],"unload_place_ids":[%d],"mode":"replace"}`, a, b, p1), h)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["success_count"])
		for _, id := range []int{a, b} {
			set := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/unload-places", id), h)))
			assert.True(t, set[p1] && len(set) == 1, "после replace у компании ровно p1")
		}

		// add: p2 обеим -> union {p1,p2}.
		rec2 := testutil.POST(t, e, "/companies/bulk/unload-places", fmt.Sprintf(`{"ids":[%d,%d],"unload_place_ids":[%d],"mode":"add"}`, a, b, p2), h)
		require.Equal(t, http.StatusOK, rec2.Code)
		for _, id := range []int{a, b} {
			set := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/unload-places", id), h)))
			assert.True(t, set[p1] && set[p2] && len(set) == 2, "после add у компании p1 и p2")
		}

		// Некорректный режим -> 400.
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/companies/bulk/unload-places", fmt.Sprintf(`{"ids":[%d],"unload_place_ids":[%d],"mode":"bogus"}`, a, p1), h).Code)
	})

	t.Run("tables", func(t *testing.T) {
		a := createCompany(t, "Табл К А")
		b := createCompany(t, "Табл К Б")
		t1 := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", `{"name":"bulk-ct1","display_name":"Bulk CT1","table_type":"cars"}`, h))["id"].(float64))
		t2 := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", `{"name":"bulk-ct2","display_name":"Bulk CT2","table_type":"cars"}`, h))["id"].(float64))

		// replace: t1 обеим.
		rec := testutil.POST(t, e, "/companies/bulk/tables", fmt.Sprintf(`{"ids":[%d,%d],"table_ids":[%d],"mode":"replace"}`, a, b, t1), h)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["success_count"])
		for _, id := range []int{a, b} {
			set := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/tables", id), h)))
			assert.True(t, set[t1] && len(set) == 1)
		}

		// add: t2 обеим -> union.
		rec2 := testutil.POST(t, e, "/companies/bulk/tables", fmt.Sprintf(`{"ids":[%d,%d],"table_ids":[%d],"mode":"add"}`, a, b, t2), h)
		require.Equal(t, http.StatusOK, rec2.Code)
		for _, id := range []int{a, b} {
			set := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/tables", id), h)))
			assert.True(t, set[t1] && set[t2] && len(set) == 2)
		}
	})

	t.Run("users", func(t *testing.T) {
		a := createCompany(t, "Юзер К А")
		b := createCompany(t, "Юзер К Б")
		testutil.RegisterUser(t, e, "bulkcresp1", "pass123", 1, td.OrgID, td.CompanyID)
		testutil.RegisterUser(t, e, "bulkcresp2", "pass123", 1, td.OrgID, td.CompanyID)

		// replace: resp1 обеим с required_approval=true; primary не назначается.
		rec := testutil.POST(t, e, "/companies/bulk/users", fmt.Sprintf(`{"ids":[%d,%d],"users":[{"username":"bulkcresp1","required_approval":true}],"mode":"replace"}`, a, b), h)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["success_count"])
		usersA := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", a), h))
		require.Len(t, usersA, 1)
		assert.Equal(t, "bulkcresp1", usersA[0]["username"])
		assert.Equal(t, true, usersA[0]["required_approval"])
		assert.Equal(t, false, usersA[0]["is_primary"])

		// add: resp2 обеим -> {resp1(required_approval сохранён true), resp2(false)}.
		rec2 := testutil.POST(t, e, "/companies/bulk/users", fmt.Sprintf(`{"ids":[%d,%d],"users":[{"username":"bulkcresp2","required_approval":false}],"mode":"add"}`, a, b), h)
		require.Equal(t, http.StatusOK, rec2.Code)
		byName := map[string]map[string]interface{}{}
		for _, u := range testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", a), h)) {
			byName[u["username"].(string)] = u
		}
		require.Len(t, byName, 2)
		assert.Equal(t, true, byName["bulkcresp1"]["required_approval"], "add сохраняет флаги существующего")
		assert.Equal(t, false, byName["bulkcresp2"]["required_approval"])

		// primary сохраняется в replace + required_approval ИНДИВИДУАЛЕН на каждого.
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", a),
			`{"users":[{"username":"bulkcresp1","is_primary":true},{"username":"bulkcresp2"}]}`, h).Code)
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/companies/bulk/users",
			fmt.Sprintf(`{"ids":[%d],"users":[{"username":"bulkcresp1","required_approval":true},{"username":"bulkcresp2","required_approval":false}],"mode":"replace"}`, a), h).Code)
		byName2 := map[string]map[string]interface{}{}
		for _, u := range testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/users", a), h)) {
			byName2[u["username"].(string)] = u
		}
		assert.Equal(t, true, byName2["bulkcresp1"]["is_primary"], "primary оставшегося сохраняется при replace")
		assert.Equal(t, false, byName2["bulkcresp2"]["is_primary"])
		assert.Equal(t, true, byName2["bulkcresp1"]["required_approval"], "required_approval индивидуален: resp1=true")
		assert.Equal(t, false, byName2["bulkcresp2"]["required_approval"], "required_approval индивидуален: resp2=false")
	})

	t.Run("archive-restore", func(t *testing.T) {
		// td.CompanyID активна и с пользователями (admin) -> архив заблокирован; пустая -> ок.
		empty := int(testutil.ParseMap(t, testutil.POST(t, e, "/companies", `{"name":"Пустая К для архива","type":"Отдел"}`, h))["id"].(float64))

		rec := testutil.POST(t, e, "/companies/bulk/archive", fmt.Sprintf(`{"ids":[%d,%d]}`, td.CompanyID, empty), h)
		require.Equal(t, http.StatusMultiStatus, rec.Code)
		res := testutil.ParseMap(t, rec)
		assert.Equal(t, float64(1), res["success_count"])
		assert.Equal(t, float64(1), res["error_count"])
		errs := res["errors"].([]interface{})
		require.Len(t, errs, 1)
		e0 := errs[0].(map[string]interface{})
		assert.Equal(t, float64(td.CompanyID), e0["id"])
		assert.Equal(t, "Test Company", e0["name"])
		assert.Contains(t, e0["error"].(string), "активными пользователями")

		// empty реально в архиве.
		archSet := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, "/companies/with-users?include_archived=true", h)))
		assert.True(t, archSet[empty])
		defSet := bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, "/companies/with-users", h)))
		assert.False(t, defSet[empty], "архивная не в списке по умолчанию")

		// bulk restore -> снова активна.
		rrec := testutil.POST(t, e, "/companies/bulk/restore", fmt.Sprintf(`{"ids":[%d]}`, empty), h)
		require.Equal(t, http.StatusOK, rrec.Code)
		assert.Equal(t, float64(1), testutil.ParseMap(t, rrec)["success_count"])
		assert.True(t, bulkIDSet(testutil.ParseSlice(t, testutil.GET(t, e, "/companies/with-users", h)))[empty])
	})
}

func TestCompanies_Bulk_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "regularuser", "pass123", 1, td.OrgID, td.CompanyID))

	// Гейт requireAdmin бьёт до цикла - 403 на весь запрос.
	assert.Equal(t, http.StatusForbidden,
		testutil.POST(t, e, "/companies/bulk/type", fmt.Sprintf(`{"ids":[%d],"type":"Отдел"}`, td.CompanyID), h).Code)
	assert.Equal(t, http.StatusForbidden,
		testutil.POST(t, e, "/companies/bulk/archive", fmt.Sprintf(`{"ids":[%d]}`, td.CompanyID), h).Code)
}

// TestCompanies_BulkAudit - зеркало TestOrganizations_BulkAudit: групповые и одиночные
// изменения привязок компании пишут в audit_log «было -> стало», no-op не пишет.
func TestCompanies_BulkAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	createCompany := func(name string) int {
		return int(testutil.ParseMap(t, testutil.POST(t, e, "/companies", fmt.Sprintf(`{"name":%q,"type":"Организация"}`, name), h))["id"].(float64))
	}
	history := func(id int) []map[string]interface{} {
		return testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/history", id), h))
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
		comp := createCompany("Аудит К Места")
		p1 := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Аудит К Место 1","description":"d","status":"active"}`, h))["id"].(float64))
		p2 := int(testutil.ParseMap(t, testutil.POST(t, e, "/unload-places", `{"name":"Аудит К Место 2","description":"d","status":"active"}`, h))["id"].(float64))

		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/companies/bulk/unload-places",
			fmt.Sprintf(`{"ids":[%d],"unload_place_ids":[%d,%d],"mode":"replace"}`, comp, p1, p2), h).Code)
		rec := findAction(history(comp), "unload_places_changed")
		require.NotNil(t, rec, "bulk назначение мест пишет запись в историю")
		assert.NotEmpty(t, rec["actor_name"])
		assert.ElementsMatch(t, []string{"Аудит К Место 1", "Аудит К Место 2"}, names(detailsOf(rec)["added"]))

		before := len(history(comp))
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d/unload-places", comp),
			fmt.Sprintf(`{"unload_place_ids":[%d,%d]}`, p1, p2), h).Code)
		assert.Len(t, history(comp), before, "неизменяющее обновление не пишет историю")

		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d/unload-places", comp),
			fmt.Sprintf(`{"unload_place_ids":[%d]}`, p1), h).Code)
		rec2 := findAction(history(comp), "unload_places_changed")
		require.NotNil(t, rec2)
		assert.ElementsMatch(t, []string{"Аудит К Место 2"}, names(detailsOf(rec2)["removed"]))
	})

	t.Run("tables added", func(t *testing.T) {
		comp := createCompany("Аудит К Таблицы")
		t1 := int(testutil.ParseMap(t, testutil.POST(t, e, "/system-tables", `{"name":"audit-ct1","display_name":"Аудит КТ1","table_type":"cars"}`, h))["id"].(float64))
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/companies/bulk/tables",
			fmt.Sprintf(`{"ids":[%d],"table_ids":[%d],"mode":"replace"}`, comp, t1), h).Code)
		rec := findAction(history(comp), "tables_changed")
		require.NotNil(t, rec, "bulk назначение таблиц пишет историю")
		assert.ElementsMatch(t, []string{"Аудит КТ1"}, names(detailsOf(rec)["added"]))
	})

	t.Run("responsibles added + approval_changed", func(t *testing.T) {
		comp := createCompany("Аудит К Ответственные")
		testutil.RegisterUser(t, e, "auditcresp1", "pass123", 1, td.OrgID, td.CompanyID)

		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/companies/bulk/users",
			fmt.Sprintf(`{"ids":[%d],"users":[{"username":"auditcresp1","required_approval":false}],"mode":"replace"}`, comp), h).Code)
		rec := findAction(history(comp), "responsibles_changed")
		require.NotNil(t, rec, "bulk назначение ответственных пишет историю")
		added, _ := detailsOf(rec)["added"].([]interface{})
		require.Len(t, added, 1)
		a0 := added[0].(map[string]interface{})
		assert.Equal(t, "auditcresp1", a0["username"])
		assert.Equal(t, false, a0["required_approval"])

		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/companies/bulk/users",
			fmt.Sprintf(`{"ids":[%d],"users":[{"username":"auditcresp1","required_approval":true}],"mode":"replace"}`, comp), h).Code)
		rec2 := findAction(history(comp), "responsibles_changed")
		require.NotNil(t, rec2)
		ch, _ := detailsOf(rec2)["approval_changed"].([]interface{})
		require.Len(t, ch, 1)
		c0 := ch[0].(map[string]interface{})
		assert.Equal(t, "auditcresp1", c0["username"])
		assert.Equal(t, false, c0["from"])
		assert.Equal(t, true, c0["to"])

		before := len(history(comp))
		require.Equal(t, http.StatusOK, testutil.POST(t, e, "/companies/bulk/users",
			fmt.Sprintf(`{"ids":[%d],"users":[{"username":"auditcresp1","required_approval":true}],"mode":"replace"}`, comp), h).Code)
		assert.Len(t, history(comp), before, "неизменяющее назначение ответственных не пишет историю")
	})

	t.Run("смена главного ответственного пишет primary_changed", func(t *testing.T) {
		comp := createCompany("Аудит К Главный")
		testutil.RegisterUser(t, e, "auditcprim1", "pass123", 1, td.OrgID, td.CompanyID)
		testutil.RegisterUser(t, e, "auditcprim2", "pass123", 1, td.OrgID, td.CompanyID)

		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", comp),
			`{"users":[{"username":"auditcprim1","is_primary":true},{"username":"auditcprim2"}]}`, h).Code)
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d/users", comp),
			`{"users":[{"username":"auditcprim1"},{"username":"auditcprim2","is_primary":true}]}`, h).Code)

		rec := findAction(history(comp), "responsibles_changed")
		require.NotNil(t, rec, "смена главного пишет историю")
		pc, _ := detailsOf(rec)["primary_changed"].(map[string]interface{})
		require.NotNil(t, pc, "смена главного пишет primary_changed")
		from, _ := pc["from"].(map[string]interface{})
		to, _ := pc["to"].(map[string]interface{})
		assert.Equal(t, "auditcprim1", from["username"])
		assert.Equal(t, "auditcprim2", to["username"])
		_, hasAdded := detailsOf(rec)["added"]
		assert.False(t, hasAdded, "при одной лишь смене главного added отсутствует")
	})

	t.Run("type no-op одиночный PUT не пишет историю", func(t *testing.T) {
		comp := createCompany("Аудит К Тип")
		before := len(history(comp))
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d", comp),
			`{"name":"Аудит К Тип","type":"Организация"}`, h).Code)
		assert.Len(t, history(comp), before, "неизменяющий PUT не добавляет запись")
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, fmt.Sprintf("/companies/%d", comp),
			`{"name":"Аудит К Тип","type":"Отдел"}`, h).Code)
		rec := findAction(history(comp), "retyped")
		require.NotNil(t, rec)
		from, _ := detailsOf(rec)["from"].(map[string]interface{})
		require.NotNil(t, from)
		assert.Equal(t, "Организация", from["type"])
	})
}

// createCompany заводит компанию через API и возвращает её ID (helper для reassign-тестов).
func createCompany(t *testing.T, e *echo.Echo, token, name string) int {
	t.Helper()
	rec := testutil.POST(t, e, "/companies", fmt.Sprintf(`{"name":%q,"type":"Отдел"}`, name), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "create company: %s", rec.Body.String())
	return int(testutil.ParseMap(t, rec)["id"].(float64))
}

// TestCompanies_BlockingUsersAndReassign - зеркало org-теста: список блокеров,
// перенос освобождает исходную, идемпотентность, аудит company_changed.
func TestCompanies_BlockingUsersAndReassign(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	srcID := createCompany(t, e, token, "Источник-К")
	tgtID := createCompany(t, e, token, "Цель-К")

	testutil.RegisterUser(t, e, "cblocker1", "pass123", 1, td.OrgID, srcID)
	testutil.RegisterUser(t, e, "cblocker2", "pass123", 1, td.OrgID, srcID)
	srcRef := srcID
	inactive := models.User{Username: "cblockerarchived", Password: "x", CompanyID: &srcRef, TypeID: 1}
	require.NoError(t, db.Create(&inactive).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", inactive.ID).Update("is_active", false).Error)

	blockers := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/members", srcID), testutil.AuthHeader(token)))
	assert.Len(t, blockers, 2, "только активные участники блокируют")

	// Пока есть активные - архивация запрещена.
	assert.Equal(t, http.StatusBadRequest, testutil.DELETE(t, e, fmt.Sprintf("/companies/%d", srcID), testutil.AuthHeader(token)).Code)

	rec := testutil.POST(t, e, fmt.Sprintf("/companies/%d/reassign-users", srcID), fmt.Sprintf(`{"target_id":%d}`, tgtID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, float64(2), testutil.ParseMap(t, rec)["reassigned"])

	tgtMembers := testutil.ParseSlice(t, testutil.GET(t, e, fmt.Sprintf("/companies/%d/members", tgtID), testutil.AuthHeader(token)))
	assert.Len(t, tgtMembers, 2, "оба перенесены в целевую")

	var auditCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND action = ?", models.AuditEntityUser, models.UserActionCompanyChanged).
		Count(&auditCount).Error)
	assert.EqualValues(t, 2, auditCount, "company_changed аудит на каждого перенесённого")

	// Исходную теперь можно архивировать.
	assert.Equal(t, http.StatusOK, testutil.DELETE(t, e, fmt.Sprintf("/companies/%d", srcID), testutil.AuthHeader(token)).Code)

	// Валидация: цель=источнику, несуществующая, несуществующий источник, гейт для не-админа.
	assert.Equal(t, http.StatusBadRequest, testutil.POST(t, e, fmt.Sprintf("/companies/%d/reassign-users", tgtID), fmt.Sprintf(`{"target_id":%d}`, tgtID), testutil.AuthHeader(token)).Code)
	assert.Equal(t, http.StatusBadRequest, testutil.POST(t, e, fmt.Sprintf("/companies/%d/reassign-users", tgtID), `{"target_id":999999}`, testutil.AuthHeader(token)).Code)
	assert.Equal(t, http.StatusNotFound, testutil.POST(t, e, "/companies/999999/reassign-users", fmt.Sprintf(`{"target_id":%d}`, tgtID), testutil.AuthHeader(token)).Code)
	userToken := testutil.RegisterAndLogin(t, e, "cplainuser", "pass123", 1, td.OrgID, td.CompanyID)
	assert.Equal(t, http.StatusForbidden, testutil.POST(t, e, fmt.Sprintf("/companies/%d/reassign-users", tgtID), fmt.Sprintf(`{"target_id":%d}`, srcID), testutil.AuthHeader(userToken)).Code)
	assert.Equal(t, http.StatusForbidden, testutil.GET(t, e, fmt.Sprintf("/companies/%d/members", tgtID), testutil.AuthHeader(userToken)).Code)
}
