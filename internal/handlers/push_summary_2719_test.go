package handlers_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestPushSummary2719_SubscriptionDeviceAndLastLoginAreDifferent(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	svc := services.NewPushService(db, "", "", "")
	before, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	id := seedPushUser(t, db, td, "push2719_devices")
	point := time.Date(2026, time.October, 20, 12, 0, 0, 0, time.UTC)
	ios := "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X)"
	desktop := "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"
	require.NoError(t, db.Create(&models.PushSubscription{UserID: id, Endpoint: "https://push.example.com/2719-ios", P256dh: "fixture", Auth: "fixture", UserAgent: &ios}).Error)
	require.NoError(t, db.Create(&models.AuthEvent{UserID: &id, Username: "push2719_devices", EventType: models.AuthEventLoginSuccess, Success: true, UserAgent: ios, CreatedAt: point}).Error)
	require.NoError(t, db.Create(&models.AuthEvent{UserID: &id, Username: "push2719_devices", EventType: models.AuthEventLoginSuccess, Success: true, UserAgent: desktop, CreatedAt: point.Add(time.Minute)}).Error)
	after, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, before.SubscriptionsByPlatform.IOS+1, after.SubscriptionsByPlatform.IOS)
	require.Equal(t, before.UsersByLastLoginPlatform.IOS, after.UsersByLastLoginPlatform.IOS)
	require.Equal(t, before.UsersByLastLoginPlatform.Desktop+1, after.UsersByLastLoginPlatform.Desktop)
	require.Equal(t, before.UsersWithPush+1, after.UsersWithPush)
	require.Equal(t, before.UsersWithPushByPlatform.IOS+1, after.UsersWithPushByPlatform.IOS)
	require.Equal(t, after.ActiveUsersTotal, after.UsersWithPush+after.UsersWithoutPush)

	require.NoError(t, db.Create(&models.PushSubscription{UserID: id, Endpoint: "https://push.example.com/2719-ipad", P256dh: "fixture", Auth: "fixture", UserAgent: &ios}).Error)
	duplicate, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, after.UsersWithPushByPlatform.IOS, duplicate.UsersWithPushByPlatform.IOS)
	require.Equal(t, after.SubscriptionsByPlatform.IOS+1, duplicate.SubscriptionsByPlatform.IOS)

	android := "Mozilla/5.0 (Linux; Android 14)"
	require.NoError(t, db.Create(&models.PushSubscription{UserID: id, Endpoint: "https://push.example.com/2719-android", P256dh: "fixture", Auth: "fixture", UserAgent: &android}).Error)
	twoDevices, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, after.UsersWithPush, twoDevices.UsersWithPush)
	require.Equal(t, before.UsersWithPushByPlatform.Android+1, twoDevices.UsersWithPushByPlatform.Android)
	require.Equal(t, after.UsersWithPushByPlatform.IOS, twoDevices.UsersWithPushByPlatform.IOS)
	require.Equal(t, after.SubscriptionsByPlatform.Android+1, twoDevices.SubscriptionsByPlatform.Android)

	require.NoError(t, db.Model(&models.User{}).Where("id = ?", id).Update("is_active", false).Error)
	archived, err := svc.GetSummary(context.Background())
	require.NoError(t, err)
	require.Equal(t, before.ActiveUsersTotal, archived.ActiveUsersTotal)
	require.Equal(t, before.UsersWithPush, archived.UsersWithPush)
	require.Equal(t, before.SubscriptionsByPlatform, archived.SubscriptionsByPlatform)
	require.Equal(t, before.UsersWithPushByPlatform, archived.UsersWithPushByPlatform)
	require.Equal(t, before.UsersByLastLoginPlatform, archived.UsersByLastLoginPlatform)
}
