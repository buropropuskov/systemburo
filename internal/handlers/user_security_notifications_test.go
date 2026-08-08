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

// TestUserBan_Notification_WithReason проверяет, что блокировка учётки создаёт
// персистентное уведомление user_banned с причиной в тексте и в data (#1748 S3).
func TestUserBan_Notification_WithReason(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	actorToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "bannotiftarget", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "bannotiftarget")

	const reason = "Нарушение пропускного режима"
	rec := testutil.POST(t, e, fmt.Sprintf("/users/%d/ban", targetID), fmt.Sprintf(`{"reason":%q}`, reason), testutil.AuthHeader(actorToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var notifs []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", targetID, "user_banned").Find(&notifs).Error)
	require.Len(t, notifs, 1, "ожидается одно уведомление user_banned")

	n := notifs[0]
	require.NotNil(t, n.Title)
	assert.Equal(t, "Учётная запись заблокирована", *n.Title)
	require.NotNil(t, n.Message)
	assert.Contains(t, *n.Message, reason)
	require.NotNil(t, n.Data)
	assert.Contains(t, *n.Data, reason)
}

// TestUserBan_Notification_WithoutReason проверяет, что блокировка без причины
// не оставляет в тексте пустых кавычек/канцелярита.
func TestUserBan_Notification_WithoutReason(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	actorToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "bannoreasontarget", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "bannoreasontarget")

	rec := testutil.POST(t, e, fmt.Sprintf("/users/%d/ban", targetID), `{}`, testutil.AuthHeader(actorToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var notifs []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", targetID, "user_banned").Find(&notifs).Error)
	require.Len(t, notifs, 1)

	n := notifs[0]
	require.NotNil(t, n.Message)
	assert.NotContains(t, *n.Message, "Причина", "без причины формулировка не упоминает пустую причину")
	assert.NotContains(t, *n.Message, `""`, "без причины не должно быть пустых кавычек")
}

// TestUserUnban_Notification проверяет, что снятие блокировки создаёт отдельное
// уведомление user_unbanned с коротким текстом.
func TestUserUnban_Notification(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	actorToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "unbannotiftarget", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "unbannotiftarget")
	h := testutil.AuthHeader(actorToken)

	recBan := testutil.POST(t, e, fmt.Sprintf("/users/%d/ban", targetID), `{"reason":"Тест"}`, h)
	require.Equal(t, http.StatusOK, recBan.Code, recBan.Body.String())

	recUnban := testutil.POST(t, e, fmt.Sprintf("/users/%d/unban", targetID), `{}`, h)
	require.Equal(t, http.StatusOK, recUnban.Code, recUnban.Body.String())

	var notifs []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", targetID, "user_unbanned").Find(&notifs).Error)
	require.Len(t, notifs, 1, "ожидается одно уведомление user_unbanned")

	n := notifs[0]
	require.NotNil(t, n.Title)
	assert.Equal(t, "Учётная запись разблокирована", *n.Title)
	require.NotNil(t, n.Message)
	assert.Contains(t, *n.Message, "восстановлен")
}

// TestLoginBlocked_Notification_OnlyOnTransition проверяет, что уведомление
// login_blocked создаётся РОВНО ОДИН раз - в момент перехода учётки в блокировку,
// а не на каждой неудачной попытке (иначе спам ровно там, где его боятся).
func TestLoginBlocked_Notification_OnlyOnTransition(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "lockmenotif", "correctpass", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "lockmenotif")

	// Попытки 1..4: учётка ещё не заперта - уведомлений быть не должно.
	for i := 1; i <= 4; i++ {
		rec := testutil.POST(t, e, "/login", `{"username":"lockmenotif","password":"wrong"}`, nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code, "попытка %d", i)
	}

	var notifsBeforeLock []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", targetID, "login_blocked").Find(&notifsBeforeLock).Error)
	assert.Empty(t, notifsBeforeLock, "до блокировки уведомлений быть не должно")

	// 5-я попытка запирает учётку - ровно одно уведомление.
	rec := testutil.POST(t, e, "/login", `{"username":"lockmenotif","password":"wrong"}`, nil)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	var notifs []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", targetID, "login_blocked").Find(&notifs).Error)
	require.Len(t, notifs, 1, "ожидается ровно одно уведомление login_blocked")

	n := notifs[0]
	require.NotNil(t, n.Title)
	assert.Equal(t, "Вход временно заблокирован", *n.Title)
	require.NotNil(t, n.Message)
	assert.Contains(t, *n.Message, "5")
	require.NotNil(t, n.Data)
	assert.Contains(t, *n.Data, "attempts")
	assert.Contains(t, *n.Data, "locked_until")
}

// TestLoginBlocked_Notification_UnknownLogin_NoNotification проверяет, что для
// несуществующего логина уведомление не создаётся (создавать некому и нечего) -
// per-IP гвард запирает адрес так же, как и для существующего логина.
func TestLoginBlocked_Notification_UnknownLogin_NoNotification(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	testutil.SeedTestData(t, db)

	for i := 1; i <= 5; i++ {
		testutil.POST(t, e, "/login", `{"username":"ghost-login-does-not-exist","password":"wrong"}`, nil)
	}

	var notifs []models.Notification
	require.NoError(t, db.Where("type = ?", "login_blocked").Find(&notifs).Error)
	assert.Empty(t, notifs, "для несуществующего логина уведомление не создаётся")
}

// Уведомление о смене роли убрано по решению владельца (#974): человеку от строки
// «ваша роль изменена» толку нет - он всё равно видит доступные разделы сам, а в ленте
// это был шум. Тест держит отсутствие: смена роли обязана проходить, уведомление -
// не появляться.
func TestSetUserRole_DoesNotNotify(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	actorToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "rolenotiftarget", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "rolenotiftarget")

	role := models.Role{Code: "role_notif_test", Name: "Наблюдатель"}
	require.NoError(t, db.Create(&role).Error)

	rec := testutil.PUT(t, e, fmt.Sprintf("/users/%d/role", targetID), fmt.Sprintf(`{"role_id":%d}`, role.ID), testutil.AuthHeader(actorToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var updated models.User
	require.NoError(t, db.Select("role_id").First(&updated, targetID).Error)
	require.NotNil(t, updated.RoleID, "роль должна была назначиться")
	assert.Equal(t, role.ID, *updated.RoleID)

	var count int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", targetID, "role_changed").Count(&count).Error)
	assert.Zero(t, count, "уведомление о смене роли больше не создаётся")
}
