package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Пути, дописанные в pdPaths задачей #2352 (internal/middleware/pd_audit.go): до
// этой задачи обращение к ним не двигало счётчик журнала - ровно так и было
// обнаружено на стенде (issue #2352, GET /api/users/all не сдвигал журнал, а
// GET /api/unique-employees сдвигал). Здесь тот же способ проверки, но сквозь
// реальный HTTP-стек, а не только разбор пути (тот - в internal/middleware).

// waitPDAuditCount ждёт, пока число строк журнала с данным resource не станет >= min:
// запись идёт из middleware в отдельной горутине.
func waitPDAuditCount(t *testing.T, db *gorm.DB, resource string, min int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		var n int64
		db.Model(&models.PDAuditLog{}).Where("resource = ?", resource).Count(&n)
		return n >= min
	}, 3*time.Second, 20*time.Millisecond, "запись журнала для resource=%s не появилась", resource)
}

func TestPDAudit_NewPathsRecordAccess(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	var before int64
	require.NoError(t, db.Model(&models.PDAuditLog{}).Count(&before).Error)

	t.Run("список пользователей", func(t *testing.T) {
		rec := testutil.GET(t, e, "/users/all", testutil.AuthHeader(adminToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		waitPDAuditCount(t, db, "user", 1)

		var row models.PDAuditLog
		require.NoError(t, db.Where("path = ?", "/api/users/all").Order("id DESC").First(&row).Error)
		require.Equal(t, "user", row.Resource)
		require.Equal(t, "view", row.Action)
		require.Equal(t, http.StatusOK, row.StatusCode)
	})

	t.Run("реестр автомобилей", func(t *testing.T) {
		rec := testutil.GET(t, e, "/unique-cars", testutil.AuthHeader(adminToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		waitPDAuditCount(t, db, "unique_car", 1)

		var row models.PDAuditLog
		require.NoError(t, db.Where("path = ?", "/api/unique-cars").Order("id DESC").First(&row).Error)
		require.Equal(t, "unique_car", row.Resource)
		require.Equal(t, "view", row.Action)
	})

	// Обращение без персональных данных (справочник) журнал по-прежнему не трогает -
	// новые пути не превратили middleware в блокирующий вход.
	rec := testutil.GET(t, e, "/citizenships", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	var after int64
	require.NoError(t, db.Model(&models.PDAuditLog{}).Count(&after).Error)
	require.Greater(t, after, before, "обращения к новым путям обязаны были прирастить журнал")
}
