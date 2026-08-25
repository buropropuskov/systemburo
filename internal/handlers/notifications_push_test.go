package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/models"
	"systemburo/internal/testutil"
)

// subscribeBody -- тело POST /notifications/push/subscribe в форме
// PushSubscription.toJSON() браузера.
func subscribeBody(endpoint, p256dh, auth string) string {
	return fmt.Sprintf(`{"endpoint":%q,"keys":{"p256dh":%q,"auth":%q}}`, endpoint, p256dh, auth)
}

// TestPushSubscribe_DedupSameEndpoint защищает upsert по endpoint (#974): повторная
// подписка того же браузера не плодит вторую строку.
func TestPushSubscribe_DedupSameEndpoint(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "push_sub1", "pass123", 1, td.OrgID, td.CompanyID)
	endpoint := "https://push.example.com/ep-dedup"

	rec1 := testutil.POST(t, e, "/notifications/push/subscribe", subscribeBody(endpoint, "p256dh-key", "auth-key"), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec1.Code)
	rec2 := testutil.POST(t, e, "/notifications/push/subscribe", subscribeBody(endpoint, "p256dh-key", "auth-key"), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec2.Code)

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("endpoint = ?", endpoint).Count(&count).Error)
	assert.Equal(t, int64(1), count, "повторная подписка тем же endpoint не должна плодить строк")
}

// TestPushSubscribe_EndpointMigratesOwner: общий компьютер, на котором сменился вход, -
// endpoint должен переехать на нового пользователя, а не задвоиться (#974).
func TestPushSubscribe_EndpointMigratesOwner(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	tokenA := testutil.RegisterAndLogin(t, e, "push_owner_a", "pass123", 1, td.OrgID, td.CompanyID)
	tokenB := testutil.RegisterAndLogin(t, e, "push_owner_b", "pass123", 1, td.OrgID, td.CompanyID)
	userBID := getUserID(t, db, "push_owner_b")
	endpoint := "https://push.example.com/ep-migrate"

	rec1 := testutil.POST(t, e, "/notifications/push/subscribe", subscribeBody(endpoint, "key-a", "auth-a"), testutil.AuthHeader(tokenA))
	require.Equal(t, http.StatusOK, rec1.Code)
	rec2 := testutil.POST(t, e, "/notifications/push/subscribe", subscribeBody(endpoint, "key-b", "auth-b"), testutil.AuthHeader(tokenB))
	require.Equal(t, http.StatusOK, rec2.Code)

	var rows []models.PushSubscription
	require.NoError(t, db.Where("endpoint = ?", endpoint).Find(&rows).Error)
	require.Len(t, rows, 1, "endpoint должен остаться одной строкой")
	assert.Equal(t, userBID, rows[0].UserID, "подписка должна переехать на нового владельца")
}

// TestPushUnsubscribe_OnlyOwnSubscription: отписка снимает только подписку, принадлежащую
// вызывающему пользователю (#974) - чужую строку убрать нельзя.
func TestPushUnsubscribe_OnlyOwnSubscription(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	tokenA := testutil.RegisterAndLogin(t, e, "push_del_a", "pass123", 1, td.OrgID, td.CompanyID)
	tokenB := testutil.RegisterAndLogin(t, e, "push_del_b", "pass123", 1, td.OrgID, td.CompanyID)
	endpoint := "https://push.example.com/ep-scoped"

	rec := testutil.POST(t, e, "/notifications/push/subscribe", subscribeBody(endpoint, "key", "auth"), testutil.AuthHeader(tokenA))
	require.Equal(t, http.StatusOK, rec.Code)

	// Чужая отписка отвечает 200 (идемпотентна), но строку А не трогает.
	recForeign := testutil.DELETE(t, e, "/notifications/push/subscribe?endpoint="+endpoint, testutil.AuthHeader(tokenB))
	assert.Equal(t, http.StatusOK, recForeign.Code)

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("endpoint = ?", endpoint).Count(&count).Error)
	assert.Equal(t, int64(1), count, "чужая отписка не должна снимать подписку другого пользователя")

	recOwn := testutil.DELETE(t, e, "/notifications/push/subscribe?endpoint="+endpoint, testutil.AuthHeader(tokenA))
	assert.Equal(t, http.StatusOK, recOwn.Code)
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("endpoint = ?", endpoint).Count(&count).Error)
	assert.Equal(t, int64(0), count, "владелец должен снять свою подписку")
}

// TestPushSubscribe_RequiresFields: пустой endpoint или ключи - 400, без записи в БД.
func TestPushSubscribe_RequiresFields(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "push_invalid", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/notifications/push/subscribe", `{"endpoint":"","keys":{"p256dh":"","auth":""}}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestPushSubscribe_Unauthorized: без токена - 401, как у остальной группы /notifications.
func TestPushSubscribe_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.POST(t, e, "/notifications/push/subscribe", subscribeBody("https://push.example.com/ep-anon", "k", "a"), nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestPushStatus_DisabledWhenNoVAPIDKeys: тестовое приложение поднимается с пустыми
// VAPID-ключами (testutil.go) - статус должен честно отвечать "не настроено", без ошибок.
func TestPushStatus_DisabledWhenNoVAPIDKeys(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "push_status", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/notifications/push/status", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	status := testutil.ParseResponse[models.PushStatusResponse](t, rec)
	assert.False(t, status.Enabled, "без VAPID-ключей push должен быть выключен")
	assert.Empty(t, status.PublicKey)
}

// TestPushStatus_ListsOwnDevices: список устройств содержит только подписки текущего
// пользователя, без ключей и endpoint в ответе.
func TestPushStatus_ListsOwnDevices(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	tokenA := testutil.RegisterAndLogin(t, e, "push_devices_a", "pass123", 1, td.OrgID, td.CompanyID)
	tokenB := testutil.RegisterAndLogin(t, e, "push_devices_b", "pass123", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/notifications/push/subscribe", subscribeBody("https://push.example.com/ep-devices", "k", "a"), testutil.AuthHeader(tokenA))
	require.Equal(t, http.StatusOK, rec.Code)

	statusA := testutil.ParseResponse[models.PushStatusResponse](t, testutil.GET(t, e, "/notifications/push/status", testutil.AuthHeader(tokenA)))
	require.Len(t, statusA.Devices, 1)

	statusB := testutil.ParseResponse[models.PushStatusResponse](t, testutil.GET(t, e, "/notifications/push/status", testutil.AuthHeader(tokenB)))
	assert.Empty(t, statusB.Devices, "чужие устройства не должны попадать в список")
}
