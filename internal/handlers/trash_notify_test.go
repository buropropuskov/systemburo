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

// Восстановление из корзины (#1748) уведомляет автора записи - заявителя, чья
// заявка привела машину/сотрудника, - а не того, кто нажал "восстановить".

func TestTrash_Notify_RestoreCarNotifiesAuthorNotRestorer(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorToken := testutil.RegisterAndLogin(t, e, "trashnotify_author", "password123", 1, td.OrgID, td.CompanyID)

	restorerToken := testutil.RegisterAndLogin(t, e, "trashnotify_restorer", "password123", 1, td.OrgID, td.CompanyID)
	var restorer models.User
	require.NoError(t, db.Where("username = ?", "trashnotify_restorer").First(&restorer).Error)

	dn := "Корзина уведомлений"
	tbl := models.SystemTable{Name: "trash_notify_cars", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	testutil.GrantTableVerb(t, restorer.ID, tbl.Name, "trash")

	appID, _, carID := seedCarViaCompleteApp(t, e, db, authorToken, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		fmt.Sprintf(`{"status":0,"user_id":%d,"table_id":%d}`, restorer.ID, tbl.ID), testutil.AuthHeader(restorerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/trash/restore", tbl.ID),
		fmt.Sprintf(`{"ids":[%d]}`, carID), testutil.AuthHeader(restorerToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, float64(1), testutil.ParseMap(t, rec)["restored"])

	// Заявитель уведомлён о восстановлении.
	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusOK, rec.Code)
	found := false
	for _, n := range testutil.ParseSlice(t, rec) {
		if n["type"] == "trash_restored" {
			found = true
			assert.Equal(t, "Запись восстановлена из корзины", n["title"])
		}
	}
	assert.True(t, found, "заявитель должен получить уведомление о восстановлении своей машины")

	// Восстанавливающий сам себе не шлёт.
	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(restorerToken))
	require.Equal(t, http.StatusOK, rec.Code)
	for _, n := range testutil.ParseSlice(t, rec) {
		assert.NotEqual(t, "trash_restored", n["type"], "восстанавливающий не должен уведомлять сам себя")
	}
}

func TestTrash_Notify_RestoreByAuthorItselfNotNotified(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "trashnotify_self", "password123", 1, td.OrgID, td.CompanyID)
	var u models.User
	require.NoError(t, db.Where("username = ?", "trashnotify_self").First(&u).Error)

	dn := "Корзина уведомлений (self)"
	tbl := models.SystemTable{Name: "trash_notify_self", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	testutil.GrantTableVerb(t, u.ID, tbl.Name, "trash")

	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		fmt.Sprintf(`{"status":0,"user_id":%d,"table_id":%d}`, u.ID, tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Тот же пользователь, что подавал заявку, восстанавливает машину сам.
	rec = testutil.POST(t, e, fmt.Sprintf("/system-tables/%d/trash/restore", tbl.ID),
		fmt.Sprintf(`{"ids":[%d]}`, carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, float64(1), testutil.ParseMap(t, rec)["restored"])

	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	for _, n := range testutil.ParseSlice(t, rec) {
		assert.NotEqual(t, "trash_restored", n["type"], "заявитель восстановил сам - уведомлять некого")
	}
}
