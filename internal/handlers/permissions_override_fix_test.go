package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermissions_CatalogOverride_PersistsAndApplies покрывает #867: точечное
// право на КАТАЛОЖНЫЙ ключ (page.center - Go-константа, строкой в permissions её
// нет) должно (1) сохраняться в read-back, чтобы тумблер не слетал после F5, и
// (2) реально действовать - резолвер обязан увидеть его в /permissions/my.
// Раньше запись шла в legacy user_permissions (резолвер её не читает), а read-back
// делал INNER JOIN с permissions и выбрасывал каталожные ключи.
func TestPermissions_CatalogOverride_PersistsAndApplies(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	userToken := testutil.RegisterAndLogin(t, e, "permtarget", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "permtarget").Row().Scan(&userID))
	admin := testutil.AuthHeader(adminToken)

	// Админ выдаёт каталожное право page.center.
	body := `{"permissions":[{"key":"page.center","value":"allow"}]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, admin)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Read-back сохраняет каталожный ключ (раньше INNER JOIN его терял -> тумблер слетал).
	rec = testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d", userID), admin)
	require.Equal(t, http.StatusOK, rec.Code)
	perms := testutil.ParseResponse[[]models.UserPermissionResponse](t, rec)
	var got *models.UserPermissionResponse
	for i := range perms {
		if perms[i].Key == "page.center" {
			got = &perms[i]
		}
	}
	require.NotNil(t, got, "page.center должен сохраниться в read-back")
	assert.Equal(t, "allow", got.Value)

	// Право реально действует: резолвер видит page.center allow в /permissions/my юзера.
	rec = testutil.GET(t, e, "/permissions/my", testutil.AuthHeader(userToken))
	require.Equal(t, http.StatusOK, rec.Code)
	my := testutil.ParseResponse[models.MyPermissionsResponse](t, rec)
	has := false
	for _, it := range my.Permissions {
		if it.Key == "page.center" && it.Value == "allow" {
			has = true
		}
	}
	assert.True(t, has, "выданное право page.center должно действовать (резолвер видит его)")
}

// TestPermissions_CatalogOverride_ReadbackMetadata закрывает #887 (единый SoT
// каталога): read-back каталожного ключа обязан вернуть category/display_name из
// Go-каталога, а не пусто. В таблице permissions каталожных ключей нет, поэтому
// LEFT JOIN дал бы NULL -> модалка прав теряла бы группировку/подпись.
func TestPermissions_CatalogOverride_ReadbackMetadata(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "permmeta", "password123", 1, td.OrgID, td.CompanyID)
	var userID int
	require.NoError(t, db.Table("users").Select("id").Where("username = ?", "permmeta").Row().Scan(&userID))
	admin := testutil.AuthHeader(adminToken)

	body := `{"permissions":[{"key":"page.center","value":"allow"}]}`
	rec := testutil.PUT(t, e, fmt.Sprintf("/permissions/user/%d", userID), body, admin)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.GET(t, e, fmt.Sprintf("/permissions/user/%d", userID), admin)
	require.Equal(t, http.StatusOK, rec.Code)
	perms := testutil.ParseResponse[[]models.UserPermissionResponse](t, rec)
	var got *models.UserPermissionResponse
	for i := range perms {
		if perms[i].Key == "page.center" {
			got = &perms[i]
		}
	}
	require.NotNil(t, got, "page.center должен быть в read-back")
	assert.Equal(t, "Центр заявок", got.DisplayName, "display_name каталожного ключа - из Go-каталога")
	assert.Equal(t, "Навигация", got.Category, "category каталожного ключа - из Go-каталога")
}
