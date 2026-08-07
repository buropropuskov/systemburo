package handlers_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// TestPushSummary_RequiresStatisticsPermission: сводка - НЕ личная настройка группы
// /notifications, а админский разрез (#974) - гейт page.statistics, как у остальной
// статистики: обычный пользователь получает 403, администратор - 200.
func TestPushSummary_RequiresStatisticsPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	regularToken := testutil.RegisterAndLogin(t, e, "push_summary_regular", "pass123", 1, td.OrgID, td.CompanyID)
	recRegular := testutil.GET(t, e, "/notifications/push/summary", testutil.AuthHeader(regularToken))
	assert.Equal(t, http.StatusForbidden, recRegular.Code)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	recAdmin := testutil.GET(t, e, "/notifications/push/summary", testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, recAdmin.Code)

	summary := testutil.ParseResponse[models.PushSummary](t, recAdmin)
	assert.GreaterOrEqual(t, summary.ActiveUsersTotal, int64(1))
}

// TestPushService_GetSummary_EmptyStateNoError: сводка не должна падать даже когда в
// базе нет ни одной подписки и ни одного собственного (не сидовского) события входа -
// итог обязан оставаться внутренне непротиворечивым.
func TestPushService_GetSummary_EmptyStateNoError(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	svc := services.NewPushService(db, "", "", "")
	summary, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	assert.Equal(t, summary.ActiveUsersTotal, summary.UsersWithPush+summary.UsersWithoutPush)
	assert.Equal(t, int64(0), summary.UsersWithPush, "на чистой базе живых подписок ещё нет")
}

// TestPushService_GetSummary_DeviceDedup: человек с двумя устройствами разных платформ
// считается один раз в UsersWithPush, но даёт две отдельные строки в разрезе подписок
// по платформам (#974).
func TestPushService_GetSummary_DeviceDedup(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := services.NewPushService(db, "", "", "")
	before, err := svc.GetSummary(context.Background())
	require.NoError(t, err)

	userID := seedPushUser(t, db, td, "push_summary_dedup")
	iphoneUA := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	androidUA := "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36"
	require.NoError(t, db.Create(&models.PushSubscription{
		UserID: userID, Endpoint: "https://push.example.com/ep-summary-1",
		P256dh: "k1", Auth: "a1", UserAgent: testutil.Ptr(iphoneUA),
	}).Error)
	require.NoError(t, db.Create(&models.PushSubscription{
		UserID: userID, Endpoint: "https://push.example.com/ep-summary-2",
		P256dh: "k2", Auth: "a2", UserAgent: testutil.Ptr(androidUA),
	}).Error)

	after, err := svc.GetSummary(context.Background())
	require.NoError(t, err)

	assert.Equal(t, before.ActiveUsersTotal+1, after.ActiveUsersTotal)
	assert.Equal(t, before.UsersWithPush+1, after.UsersWithPush, "два устройства одного человека - один пользователь")
	assert.Equal(t, before.SubscriptionsByPlatform.IOS+1, after.SubscriptionsByPlatform.IOS)
	assert.Equal(t, before.SubscriptionsByPlatform.Android+1, after.SubscriptionsByPlatform.Android)
}

// TestPushService_GetSummary_IPadNotDesktop: подписка с UA классического iPad должна
// попасть в разрез ios, а не desktop - ключевая ловушка платформенного разбора (#974).
func TestPushService_GetSummary_IPadNotDesktop(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := services.NewPushService(db, "", "", "")
	before, err := svc.GetSummary(context.Background())
	require.NoError(t, err)

	userID := seedPushUser(t, db, td, "push_summary_ipad")
	ipadUA := "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	require.NoError(t, db.Create(&models.PushSubscription{
		UserID: userID, Endpoint: "https://push.example.com/ep-summary-ipad",
		P256dh: "k", Auth: "a", UserAgent: testutil.Ptr(ipadUA),
	}).Error)

	after, err := svc.GetSummary(context.Background())
	require.NoError(t, err)

	assert.Equal(t, before.SubscriptionsByPlatform.IOS+1, after.SubscriptionsByPlatform.IOS)
	assert.Equal(t, before.SubscriptionsByPlatform.Desktop, after.SubscriptionsByPlatform.Desktop, "iPad не должен попасть в desktop")
}

// TestPushService_GetSummary_LastLoginPlatform_UnknownWithoutLogin: пользователь без
// единого события входа не пропадает из сводки - засчитывается активным, а платформа
// его последнего входа считается unknown.
func TestPushService_GetSummary_LastLoginPlatform_UnknownWithoutLogin(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := services.NewPushService(db, "", "", "")
	before, err := svc.GetSummary(context.Background())
	require.NoError(t, err)

	seedPushUser(t, db, td, "push_summary_nologin")

	after, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	assert.Equal(t, before.ActiveUsersTotal+1, after.ActiveUsersTotal)
	assert.Equal(t, before.UsersByLastLoginPlatform.Unknown+1, after.UsersByLastLoginPlatform.Unknown)
}

// TestPushService_GetSummary_LastLoginPlatform_UsesLatestEvent: платформа считается по
// ПОСЛЕДНЕМУ успешному входу, а не по первому - человек мог сменить устройство.
func TestPushService_GetSummary_LastLoginPlatform_UsesLatestEvent(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := services.NewPushService(db, "", "", "")
	before, err := svc.GetSummary(context.Background())
	require.NoError(t, err)

	userID := seedPushUser(t, db, td, "push_summary_lastlogin")
	windowsUA := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	iphoneUA := "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Minute)
	require.NoError(t, db.Create(&models.AuthEvent{
		UserID: &userID, Username: "push_summary_lastlogin", EventType: models.AuthEventLoginSuccess,
		Success: true, UserAgent: windowsUA, CreatedAt: older,
	}).Error)
	require.NoError(t, db.Create(&models.AuthEvent{
		UserID: &userID, Username: "push_summary_lastlogin", EventType: models.AuthEventLoginSuccess,
		Success: true, UserAgent: iphoneUA, CreatedAt: newer,
	}).Error)

	after, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	assert.Equal(t, before.UsersByLastLoginPlatform.IOS+1, after.UsersByLastLoginPlatform.IOS,
		"должна учитываться платформа последнего входа (iPhone), не первого (Windows)")
	assert.Equal(t, before.UsersByLastLoginPlatform.Desktop, after.UsersByLastLoginPlatform.Desktop)
}

// TestPushService_GetSummary_FailedLoginIgnored: неудачная попытка входа не должна
// становиться "последним входом" - засчитывается только login_success.
func TestPushService_GetSummary_FailedLoginIgnored(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc := services.NewPushService(db, "", "", "")
	before, err := svc.GetSummary(context.Background())
	require.NoError(t, err)

	userID := seedPushUser(t, db, td, "push_summary_failedlogin")
	androidUA := "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Mobile Safari/537.36"
	require.NoError(t, db.Create(&models.AuthEvent{
		UserID: &userID, Username: "push_summary_failedlogin", EventType: models.AuthEventLoginFailed,
		Success: false, UserAgent: androidUA, CreatedAt: time.Now(),
	}).Error)

	after, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	assert.Equal(t, before.UsersByLastLoginPlatform.Android, after.UsersByLastLoginPlatform.Android,
		"неудачная попытка входа не должна засчитываться как последний вход")
	assert.Equal(t, before.UsersByLastLoginPlatform.Unknown+1, after.UsersByLastLoginPlatform.Unknown,
		"без единого успешного входа платформа неизвестна")
}
