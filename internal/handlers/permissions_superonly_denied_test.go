package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #1997: обычному администратору (is_admin, не супер) бэкенд отказывает в
// super-only ключах (PermissionSet.Has режет их для всех кроме супера), но
// GetMyPermissions раньше не сообщал об этом фронту -- Denied содержал только
// личные deny-override. Фронтовый стор в admin-режиме считает право выданным,
// если ключа нет в denied, поэтому интерфейс показывал тумблер/пункт доступным,
// а сервер отказывал на сохранении.
func TestPermissions_GetMy_Admin_DeniedIncludesSuperOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// RegisterManager -> is_admin=true, не супер (см. testutil/auth.go).
	adminToken := testutil.RegisterManager(t, e, "permadmin_superonly", td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/permissions/my", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseResponse[models.MyPermissionsResponse](t, rec)
	assert.Equal(t, "admin", resp.Mode)
	assert.Contains(t, resp.Denied, "action.grant.admin", "выдача админки -- super-only, должна быть в denied для обычного admin")
	assert.Contains(t, resp.Denied, "page.admin.system_control", "режим техработ -- super-only, должен быть в denied для обычного admin")
}

// Супер-админу super-only ключи доступны -- Denied не должен их содержать.
func TestPermissions_GetMy_Super_DeniedExcludesSuperOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	superToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/permissions/my", testutil.AuthHeader(superToken))
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseResponse[models.MyPermissionsResponse](t, rec)
	assert.Equal(t, "super", resp.Mode)
	assert.NotContains(t, resp.Denied, "action.grant.admin")
	assert.NotContains(t, resp.Denied, "page.admin.system_control")
}
