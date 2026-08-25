package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// Заблокированная и архивная учётные записи отключены от системы: войти по ним нельзя,
// а уведомления им создавались как обычным. Для ленты это мусор, который человек увидит
// через месяцы после разблокировки, а для web push - настоящая доставка: подписка
// браузера живёт независимо от сессии, и устройство продолжало получать пуши по заявкам
// после отключения учётки.

func setBanned(t *testing.T, db *gorm.DB, userID int, banned bool) {
	t.Helper()
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", userID).
		Update("is_banned", banned).Error)
}

func setActive(t *testing.T, db *gorm.DB, userID int, active bool) {
	t.Helper()
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", userID).
		Update("is_active", active).Error)
}

func countNotifications(t *testing.T, db *gorm.DB, userID int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.Notification{}).Where("user_id = ?", userID).Count(&n).Error)
	return n
}

func TestCreateForUser_SkipsBannedRecipient(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "ban_notify1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "ban_notify1")

	sender := &fakePushSender{}
	svc := services.NewNotificationService(db, services.WithNotificationPushSender(sender))
	ctx := context.Background()

	data := `{"application_id":801,"application_number":"№801"}`
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationStatusChanged,
		"Статус изменился", "До блокировки", &data))
	require.EqualValues(t, 1, countNotifications(t, db, userID), "до блокировки уведомления приходят как обычно")

	setBanned(t, db, userID, true)
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationStatusChanged,
		"Статус изменился", "После блокировки", &data))

	assert.EqualValues(t, 1, countNotifications(t, db, userID), "заблокированному новых уведомлений не создаём")
	// Push - главное: подписка браузера пережила блокировку и без гейта сработала бы.
	assert.Len(t, sender.Calls(), 1, "после блокировки push не уходит")
}

func TestCreateForUser_SkipsInactiveRecipient(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "ban_notify2", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "ban_notify2")

	svc := services.NewNotificationService(db)
	ctx := context.Background()

	setActive(t, db, userID, false)
	data := `{"application_id":802,"application_number":"№802"}`
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationStatusChanged,
		"Статус изменился", "Архивному", &data))

	assert.EqualValues(t, 0, countNotifications(t, db, userID), "архивной учётке уведомления не создаём")
}

func TestCreateForUser_DeliversBanNoticeToBannedUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "ban_notify3", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "ban_notify3")

	svc := services.NewNotificationService(db)
	ctx := context.Background()

	// UserBanService заводит это уведомление уже ПОСЛЕ проставленного флага - если бы
	// гейт срезал и его, человек не узнал бы причину, по которой его выкинуло.
	setBanned(t, db, userID, true)
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeUserBanned,
		"Учётная запись заблокирована", "Причина: нарушение регламента", nil))

	assert.EqualValues(t, 1, countNotifications(t, db, userID), "сообщение о самой блокировке доходит")
}

func TestCreateForUser_ResumesAfterUnban(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "ban_notify4", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "ban_notify4")

	svc := services.NewNotificationService(db)
	ctx := context.Background()

	setBanned(t, db, userID, true)
	setBanned(t, db, userID, false)

	data := `{"application_id":803,"application_number":"№803"}`
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationStatusChanged,
		"Статус изменился", "После разблокировки", &data))

	assert.EqualValues(t, 1, countNotifications(t, db, userID), "после снятия блокировки доставка возобновляется")
}

func TestCreateNotification_RefusesBannedRecipient(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "ban_notify5", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "ban_notify5")

	svc := services.NewNotificationService(db)
	ctx := context.Background()
	title := "Ручное уведомление"

	setBanned(t, db, userID, true)
	_, err := svc.Create(ctx, models.CreateNotificationRequest{UserID: userID, Title: &title})

	// Отправку руками отклоняем с ошибкой: администратор должен понять, почему не ушло.
	require.Error(t, err, "администратору не даём отправить уведомление заблокированному")
	assert.EqualValues(t, 0, countNotifications(t, db, userID))
}
