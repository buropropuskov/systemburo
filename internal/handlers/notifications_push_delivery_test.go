package handlers_test

import (
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webpush "github.com/SherClockHolmes/webpush-go"
	"gorm.io/gorm"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// Доставка Web Push (#974) через реальный webpush.SendNotificationWithContext -
// шифрование и подпись VAPID настоящие, конечная точка подменена httptest-сервером.
// WithPushSyncSend делает Send() синхронным, чтобы проверять результат сразу после
// вызова, без гонки с фоновой горутиной прод-режима. Пользователь заводится напрямую
// через db.Create, без HTTP-регистрации - эти тесты вообще не трогают Echo-приложение.

// fakeSubscriberKeys генерирует валидную пару ключей подписчика (P-256 публичный ключ +
// 16-байтный auth secret), как их отдал бы PushSubscription.getKey() в браузере -
// webpush-go проверяет, что p256dh лежит на кривой, и падает раньше HTTP-запроса на
// случайной строке.
func fakeSubscriberKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	curve := elliptic.P256()
	_, x, y, err := elliptic.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	pub := elliptic.Marshal(curve, x, y)
	secret := make([]byte, 16)
	_, err = rand.Read(secret)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(pub), base64.RawURLEncoding.EncodeToString(secret)
}

// newTestPushService поднимает pushService с настоящей VAPID-парой (иначе подпись
// запроса не соберётся) и синхронной отправкой, готовый писать в db из testutil.
func newTestPushService(t *testing.T, db *gorm.DB) services.PushService {
	t.Helper()
	priv, pub, err := webpush.GenerateVAPIDKeys()
	require.NoError(t, err)
	return services.NewPushService(db, pub, priv, "mailto:bureau@example.com", services.WithPushSyncSend())
}

// seedPushUser заводит пользователя напрямую через db.Create - этим тестам не нужна
// настоящая аутентификация, только валидный user_id для FK подписки.
func seedPushUser(t *testing.T, db *gorm.DB, td testutil.TestData, username string) int {
	t.Helper()
	u := models.User{
		Username:       username,
		Password:       "x",
		TypeID:         1,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&u).Error)
	return u.ID
}

func seedSubscription(t *testing.T, db *gorm.DB, userID int, endpoint string) {
	t.Helper()
	p256dh, auth := fakeSubscriberKeys(t)
	sub := models.PushSubscription{UserID: userID, Endpoint: endpoint, P256dh: p256dh, Auth: auth}
	require.NoError(t, db.Create(&sub).Error)
}

// TestPushService_Send_DeliversAndRecordsSuccess: push-сервис отвечает 201 - подписка
// получает LastSuccessAt, счётчик неудач остаётся нулевым.
func TestPushService_Send_DeliversAndRecordsSuccess(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_deliver")
	seedSubscription(t, db, userID, srv.URL)

	svc := newTestPushService(t, db)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	var sub models.PushSubscription
	require.NoError(t, db.Where("user_id = ?", userID).First(&sub).Error)
	assert.NotNil(t, sub.LastSuccessAt)
	assert.Equal(t, 0, sub.FailedCount)
}

// TestPushService_Send_410RemovesSubscription: push-сервис подтверждает 410 Gone -
// подписка удаляется сразу, без накопления счётчика неудач (#974).
func TestPushService_Send_410RemovesSubscription(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusGone)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_gone")
	seedSubscription(t, db, userID, srv.URL)

	svc := newTestPushService(t, db)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("user_id = ?", userID).Count(&count).Error)
	assert.Equal(t, int64(0), count, "410 должен удалить подписку немедленно")
}

// TestPushService_Send_404RemovesSubscription: то же самое для 404 - оба кода по RFC 8030
// значат "endpoint больше не существует".
func TestPushService_Send_404RemovesSubscription(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_404")
	seedSubscription(t, db, userID, srv.URL)

	svc := newTestPushService(t, db)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("user_id = ?", userID).Count(&count).Error)
	assert.Equal(t, int64(0), count, "404 должен удалить подписку немедленно")
}

// TestPushService_Send_TracksFailureAndRemovesAfterLimit: push-сервис стабильно отвечает
// 500 (не 404/410) - подписка не удаляется на первой неудаче, счётчик растёт, а после
// pushSubscriptionFailureLimit подряд идущих неудач строка убирается (#974).
func TestPushService_Send_TracksFailureAndRemovesAfterLimit(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_flaky")
	seedSubscription(t, db, userID, srv.URL)

	svc := newTestPushService(t, db)
	payload := services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1}

	// Первая неудача: подписка остаётся, счётчик = 1.
	svc.Send(context.Background(), userID, payload)
	var sub models.PushSubscription
	require.NoError(t, db.Where("user_id = ?", userID).First(&sub).Error)
	assert.Equal(t, 1, sub.FailedCount)
	require.NotNil(t, sub.LastError)

	// Ещё восемь неудач подряд (итого девять) - подписка ещё жива.
	for i := 0; i < 8; i++ {
		svc.Send(context.Background(), userID, payload)
	}
	require.NoError(t, db.Where("user_id = ?", userID).First(&sub).Error)
	assert.Equal(t, 9, sub.FailedCount)

	// Десятая неудача переваливает порог - подписка удаляется.
	svc.Send(context.Background(), userID, payload)
	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("user_id = ?", userID).Count(&count).Error)
	assert.Equal(t, int64(0), count, "подписка должна быть удалена после серии неудач")
	assert.Equal(t, int32(10), atomic.LoadInt32(&calls))
}

// TestPushService_Send_VAPIDDisabled_NoOp: пустые VAPID-ключи (штатный режим "push
// выключен") - Send() ничего не отправляет и не трогает подписку.
func TestPushService_Send_VAPIDDisabled_NoOp(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	userID := seedPushUser(t, db, td, "push_off")
	seedSubscription(t, db, userID, srv.URL)

	svc := services.NewPushService(db, "", "", "", services.WithPushSyncSend())
	assert.False(t, svc.Configured())
	assert.Empty(t, svc.PublicKey())

	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	assert.Equal(t, int32(0), atomic.LoadInt32(&calls), "выключенный push не должен делать сетевых запросов")
	var sub models.PushSubscription
	require.NoError(t, db.Where("user_id = ?", userID).First(&sub).Error)
	assert.Nil(t, sub.LastSuccessAt)
	assert.Equal(t, 0, sub.FailedCount)
}

// TestPushService_Send_NoSubscriptions_NoOp: пользователь без подписок - Send() не падает
// и не создаёт лишних запросов.
func TestPushService_Send_NoSubscriptions_NoOp(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userID := seedPushUser(t, db, td, "push_nosub")
	svc := newTestPushService(t, db)
	svc.Send(context.Background(), userID, services.PushPayload{Title: "T", Message: "M", Type: "application_created", NotificationID: 1})

	var count int64
	require.NoError(t, db.Model(&models.PushSubscription{}).Where("user_id = ?", userID).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

// TestPushService_ListDevices_ScopedToUser проверяет ListDevices напрямую (без HTTP) -
// сервисный уровень контракта, которым пользуется PushHandler.GetStatus.
func TestPushService_ListDevices_ScopedToUser(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userA := seedPushUser(t, db, td, "push_list_a")
	userB := seedPushUser(t, db, td, "push_list_b")
	for i := 0; i < 2; i++ {
		seedSubscription(t, db, userA, fmt.Sprintf("https://push.example.com/ep-list-a-%d", i))
	}
	seedSubscription(t, db, userB, "https://push.example.com/ep-list-b")

	svc := newTestPushService(t, db)
	devicesA, err := svc.ListDevices(context.Background(), userA)
	require.NoError(t, err)
	assert.Len(t, devicesA, 2)

	devicesB, err := svc.ListDevices(context.Background(), userB)
	require.NoError(t, err)
	assert.Len(t, devicesB, 1)
}
