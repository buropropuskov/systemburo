package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Тесты GET /applications/attachable (#1049 S11): список заявок для привязки ручного
// вложения. Ключевое отличие от GET /applications - НЕ скоупит по участию: super/admin
// видит ВСЕ активные согласованные заявки, даже чужие (иначе админ, не участвующий в
// заявке, не смог бы к ней привязать). Гейт page.admin на роуте.

// seedAttachableApp создаёт заявку с заданными номером/confirmation/status.
func seedAttachableApp(t *testing.T, db *gorm.DB, orgID, senderID int, number, confirmation, status string) int {
	t.Helper()
	num := number
	app := models.Application{
		ApplicationNumber: &num,
		Confirmation:      &confirmation,
		Status:            &status,
		OrganizationID:    orgID,
		SenderUserID:      senderID,
	}
	require.NoError(t, db.Create(&app).Error)
	return app.ID
}

func containsAppID(rows []map[string]interface{}, id int) bool {
	for _, r := range rows {
		if v, ok := r["id"].(float64); ok && int(v) == id {
			return true
		}
	}
	return false
}

func TestAttachable_AdminSeesNonParticipantActiveApp(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Заявка от ДРУГОГО пользователя: админ не автор/ответственный/наблюдатель/принимающий.
	senderID := seedAttachSender(t, db, td.OrgID)
	appID := seedAttachableApp(t, db, td.OrgID, senderID, "ATTACHABLE-ACTIVE-1", models.ConfirmationApproved, models.StatusInWork)

	// attachable: админ видит чужую активную согласованную заявку.
	rec := testutil.GET(t, e, "/applications/attachable", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows := testutil.ParseSlice(t, rec)
	assert.True(t, containsAppID(rows, appID), "админ видит чужую активную согласованную заявку в attachable")

	// Контраст: обычный скоупленный /applications ту же заявку НЕ отдаёт (админ не участник) -
	// именно поэтому нужен отдельный attachable-эндпоинт без access-фильтра.
	rec = testutil.GET(t, e, "/applications", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)
	scoped := testutil.ParseSlice(t, rec)
	assert.False(t, containsAppID(scoped, appID), "скоупленный /applications чужую заявку не показывает")
}

func TestAttachable_ExcludesNonActiveApproved(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	senderID := seedAttachSender(t, db, td.OrgID)

	activeApp := seedAttachableApp(t, db, td.OrgID, senderID, "ATTACHABLE-OK-2", models.ConfirmationApproved, models.StatusInWork)
	pendingApp := seedAttachableApp(t, db, td.OrgID, senderID, "ATTACHABLE-PENDING-2", "На согласовании", models.StatusInWork)
	completedApp := seedAttachableApp(t, db, td.OrgID, senderID, "ATTACHABLE-DONE-2", models.ConfirmationApproved, models.StatusCompleted)

	rec := testutil.GET(t, e, "/applications/attachable", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rows := testutil.ParseSlice(t, rec)

	assert.True(t, containsAppID(rows, activeApp), "активная согласованная видна")
	assert.False(t, containsAppID(rows, pendingApp), "не согласованная исключена")
	assert.False(t, containsAppID(rows, completedApp), "завершённая (не 'В работе') исключена")
}

func TestAttachable_NonAdminForbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Обычный пользователь (type_id=1, без page.admin) - роут гейтит requireAdmin.
	userToken := testutil.RegisterAndLogin(t, e, "plain_attachable", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/applications/attachable", testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, "не-админ получает 403")
}
