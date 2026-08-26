package handlers_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// fakePushSender подменяет доставку Web Push в тестах интеграции с
// notificationService (#974): фиксирует каждый вызов Send, не делает сетевых запросов.
type fakePushSender struct {
	mu    sync.Mutex
	calls []fakePushCall
}

type fakePushCall struct {
	userID  int
	payload services.PushPayload
}

func (f *fakePushSender) Send(_ context.Context, userID int, payload services.PushPayload) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakePushCall{userID: userID, payload: payload})
}

func (f *fakePushSender) Calls() []fakePushCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakePushCall(nil), f.calls...)
}

// TestCreateForUser_SendsPush защищает точку интеграции (#974): уведомление, прошедшее
// каталог и гейт подписки, доставляется и push-рассылкой - в тех же местах, где уже
// публикуется real-time сигнал notification.new.
func TestCreateForUser_SendsPush(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "push_user1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "push_user1")

	sender := &fakePushSender{}
	svc := services.NewNotificationService(db, services.WithNotificationPushSender(sender))
	ctx := context.Background()

	data := `{"application_id":701,"application_number":"№701"}`
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationStatusChanged,
		"Статус изменился", "Заявка согласована", &data))

	calls := sender.Calls()
	require.Len(t, calls, 1, "уведомление должно уйти в push ровно один раз")
	assert.Equal(t, userID, calls[0].userID)
	assert.Equal(t, "Статус изменился", calls[0].payload.Title)
	assert.Equal(t, "Заявка согласована", calls[0].payload.Message)
	assert.Equal(t, services.NotificationTypeApplicationStatusChanged, calls[0].payload.Type)
	require.NotNil(t, calls[0].payload.ApplicationID)
	assert.Equal(t, 701, *calls[0].payload.ApplicationID)
	assert.NotZero(t, calls[0].payload.NotificationID)
}

// TestCreateForUser_DisabledPreference_NoPush защищает порядок: точка вызова push стоит
// ПОСЛЕ гейта подписки (notificationAllowed), а не до него - пользователь, выключивший
// тип в настройках, не должен получать push, даже если запись в БД не создаётся.
func TestCreateForUser_DisabledPreference_NoPush(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "push_user2", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "push_user2")

	sender := &fakePushSender{}
	svc := services.NewNotificationService(db, services.WithNotificationPushSender(sender))
	ctx := context.Background()

	require.NoError(t, svc.UpdatePreferences(ctx, userID, []models.NotificationPreferenceItemUpdate{
		{TypeCode: services.NotificationTypeNewsPublished, Enabled: false},
	}))

	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeNewsPublished, "Новость", "Текст", nil))

	assert.Empty(t, sender.Calls(), "выключенный тип не должен доходить до push-рассылки")
}

// TestCreateForUser_Collapse_SendsPushForEachEvent защищает срез "схлопывание тоже
// событие для человека" (#974): второе событие той же группы обновляет существующую
// запись ленты, но push должно уйти и на него, с актуальным (обновлённым) текстом.
func TestCreateForUser_Collapse_SendsPushForEachEvent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "push_user3", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "push_user3")

	sender := &fakePushSender{}
	svc := services.NewNotificationService(db, services.WithNotificationPushSender(sender))
	ctx := context.Background()

	data := `{"application_id":702,"application_number":"№702"}`
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationApprovalRequired,
		"Заявка на согласование", "Первое сообщение", &data))
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationApprovalRequired,
		"Заявка на согласование", "Второе сообщение", &data))

	var stored models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", userID, services.NotificationTypeApplicationApprovalRequired).
		First(&stored).Error)
	require.Equal(t, 2, stored.Count, "второе событие должно схлопнуться в ту же запись")

	calls := sender.Calls()
	require.Len(t, calls, 2, "push должен уйти на оба события, включая схлопнутое")
	assert.Equal(t, "Первое сообщение", calls[0].payload.Message)
	assert.Equal(t, "Второе сообщение", calls[1].payload.Message)
	assert.Equal(t, stored.ID, calls[1].payload.NotificationID, "push схлопнутого события ссылается на ту же запись")
}

// TestNotificationsCreate_Admin_SendsPush: ручная рассылка администратора публикует тот
// же real-time сигнал, что и обычные уведомления, - push должен уйти и здесь (#974).
func TestNotificationsCreate_Admin_SendsPush(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "push_target", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "push_target")

	sender := &fakePushSender{}
	svc := services.NewNotificationService(db, services.WithNotificationPushSender(sender))
	ctx := context.Background()

	title := "Важное объявление"
	_, err := svc.Create(ctx, models.CreateNotificationRequest{UserID: userID, Title: &title})
	require.NoError(t, err)

	calls := sender.Calls()
	require.Len(t, calls, 1)
	assert.Equal(t, userID, calls[0].userID)
	assert.Equal(t, title, calls[0].payload.Title)
}
