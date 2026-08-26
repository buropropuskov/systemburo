package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// TestCreateForUser_CollapsesRepeatedEventIntoSameGroup защищает схлопывание повторов
// (#1748): второе событие того же типа и группы в пределах окна обновляет существующую
// непрочитанную запись вместо новой строки ленты, message берётся от последнего события.
func TestCreateForUser_CollapsesRepeatedEventIntoSameGroup(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "agg_user1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "agg_user1")

	svc := services.NewNotificationService(db)
	ctx := context.Background()

	data1 := `{"application_id":501,"application_number":"№501"}`
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationApprovalRequired,
		"Заявка на согласование", "Первое сообщение", &data1))

	data2 := `{"application_id":501,"application_number":"№501"}`
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationApprovalRequired,
		"Заявка на согласование", "Второе сообщение", &data2))

	var notifications []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", userID, services.NotificationTypeApplicationApprovalRequired).
		Find(&notifications).Error)

	require.Len(t, notifications, 1, "два события одной группы должны схлопнуться в одну запись")
	assert.Equal(t, 2, notifications[0].Count)
	require.NotNil(t, notifications[0].Message)
	assert.Equal(t, "Второе сообщение", *notifications[0].Message)
	require.NotNil(t, notifications[0].GroupKey)
	assert.Equal(t, "app:501", *notifications[0].GroupKey)
	assert.NotNil(t, notifications[0].LastEventAt)
}

// TestCreateForUser_ReadNotificationNotCollapsed: если единственная запись группы уже
// прочитана, новое событие не должно снова прятать её от пользователя молчаливым
// обновлением - оно заводит новую запись (#1748).
func TestCreateForUser_ReadNotificationNotCollapsed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "agg_user2", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "agg_user2")

	svc := services.NewNotificationService(db)
	ctx := context.Background()

	data := `{"application_id":502,"application_number":"№502"}`
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationApprovalRequired,
		"Заявка на согласование", "Первое", &data))

	var first models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", userID, services.NotificationTypeApplicationApprovalRequired).
		First(&first).Error)
	require.NoError(t, db.Model(&first).Update("is_read", true).Error)

	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationApprovalRequired,
		"Заявка на согласование", "Второе", &data))

	var notifications []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", userID, services.NotificationTypeApplicationApprovalRequired).
		Find(&notifications).Error)
	assert.Len(t, notifications, 2, "прочитанное уведомление не должно схлопывать новое событие")
}

// TestCreateForUser_DifferentGroupKeysDoNotCollapse: события по разным заявкам того же
// типа не должны схлопываться друг с другом - у каждой заявки своя запись.
func TestCreateForUser_DifferentGroupKeysDoNotCollapse(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "agg_user3", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "agg_user3")

	svc := services.NewNotificationService(db)
	ctx := context.Background()

	data1 := `{"application_id":601}`
	data2 := `{"application_id":602}`
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationApprovalRequired, "T", "M1", &data1))
	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeApplicationApprovalRequired, "T", "M2", &data2))

	var notifications []models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", userID, services.NotificationTypeApplicationApprovalRequired).
		Find(&notifications).Error)
	assert.Len(t, notifications, 2, "разные заявки не должны схлопываться в одну запись")
}

// TestCreateForUser_DisabledTypeSkipsCreation: пользователь выключил конкретный
// не-mandatory тип - уведомление не создаётся вообще (#1748).
func TestCreateForUser_DisabledTypeSkipsCreation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "agg_user4", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "agg_user4")

	svc := services.NewNotificationService(db)
	ctx := context.Background()

	require.NoError(t, svc.UpdatePreferences(ctx, userID, []models.NotificationPreferenceItemUpdate{
		{TypeCode: services.NotificationTypeNewsPublished, Enabled: false},
	}))

	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypeNewsPublished, "Новость", "Текст", nil))

	var count int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", userID, services.NotificationTypeNewsPublished).
		Count(&count).Error)
	assert.Equal(t, int64(0), count, "выключенный тип не должен создавать запись")
}

// TestCreateForUser_MandatoryTypeAlwaysCreated: даже когда пользователь выключил все
// типы, которые смог (все не-mandatory), уведомление безопасности всё равно доставляется -
// иначе человек не узнает о блокировке собственной учётной записи (#1748).
func TestCreateForUser_MandatoryTypeAlwaysCreated(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "agg_user5", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "agg_user5")

	svc := services.NewNotificationService(db)
	ctx := context.Background()

	var items []models.NotificationPreferenceItemUpdate
	for _, meta := range services.NotificationCatalog() {
		if !meta.Mandatory {
			items = append(items, models.NotificationPreferenceItemUpdate{TypeCode: meta.Code, Enabled: false})
		}
	}
	require.NoError(t, svc.UpdatePreferences(ctx, userID, items))

	require.NoError(t, svc.CreateForUser(ctx, userID, services.NotificationTypePasswordChanged,
		"Пароль изменён", "Пароль вашей учётной записи был изменён.", nil))

	var count int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", userID, services.NotificationTypePasswordChanged).
		Count(&count).Error)
	assert.Equal(t, int64(1), count, "mandatory-тип обязан создаваться независимо от подписки")
}
