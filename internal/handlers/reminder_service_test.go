package handlers_test

// Интеграционные тесты ReminderService (#1315 S1) на реальном SQL: отбор зависших
// согласующих зеркалит кворум updateConfirmationBasedOnApprovals (application_helpers.go)
// и предикат активности заявки (application_archive.go). Сервис-паттерн без CleanDB
// (internal/handlers на грани лимита go test -timeout 300s) - данные создаются и
// удаляются точечно, как в marks_integration_test.go.

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func uniqRem(prefix string) string {
	return prefix + "-rem-" + intStr(int(time.Now().UnixNano()%100000))
}

func newReminderOrg(t *testing.T, db *gorm.DB) int {
	t.Helper()
	org := models.Organization{Name: uniqRem("org"), IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	return org.ID
}

func newReminderUser(t *testing.T, db *gorm.DB, prefix string) int {
	t.Helper()
	l, f := "Тест", prefix
	u := models.User{Username: uniqRem(prefix), TypeID: 1, IsActive: true, LastName: &l, FirstName: &f}
	require.NoError(t, db.Create(&u).Error)
	return u.ID
}

// newReminderApp создаёт заявку с указанным статусом/подтверждением и отправителем.
func newReminderApp(t *testing.T, db *gorm.DB, orgID, senderID int, status, confirmation string) int {
	t.Helper()
	n, s, c := uniqRem("APP"), status, confirmation
	sending := time.Now().Add(-10 * 24 * time.Hour)
	app := models.Application{
		ApplicationNumber: &n,
		OrganizationID:    orgID,
		SenderUserID:      senderID,
		Status:            &s,
		Confirmation:      &c,
		SendingDatetime:   &sending,
	}
	require.NoError(t, db.Create(&app).Error)
	return app.ID
}

func pendingStatus() *string {
	s := "pending"
	return &s
}

func approvedStatus() *string {
	s := "approved"
	return &s
}

// newResponsible заводит строку application_responsible_users с явным created_at
// (момент назначения согласующего, а не подачи заявки) и статусом голоса.
func newResponsible(t *testing.T, db *gorm.DB, appID, userID int, required bool, approvalStatus *string, createdDaysAgo int) int {
	t.Helper()
	aru := models.ApplicationResponsibleUser{
		ApplicationID:    appID,
		UserID:           userID,
		CreatedAt:        time.Now().Add(-time.Duration(createdDaysAgo) * 24 * time.Hour),
		RequiredApproval: required,
		ApprovalStatus:   approvalStatus,
	}
	require.NoError(t, db.Create(&aru).Error)
	return aru.ID
}

func newReminderServices(db *gorm.DB) (services.ReminderService, services.SettingsService) {
	settingsSvc := services.NewSettingsService(db, &config.Config{UploadMaxFileSize: 10485760, PaginationMaxLimit: 100})
	notifSvc := services.NewNotificationService(db)
	return services.NewReminderService(db, notifSvc, settingsSvc), settingsSvc
}

func countReminderNotifications(t *testing.T, db *gorm.DB, userID int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", userID, services.NotificationTypeApprovalReminder).
		Count(&count).Error)
	return count
}

func loadResponsible(t *testing.T, db *gorm.DB, id int) models.ApplicationResponsibleUser {
	t.Helper()
	var r models.ApplicationResponsibleUser
	require.NoError(t, db.First(&r, id).Error)
	return r
}

func cleanupReminderFixture(db *gorm.DB, appIDs, userIDs []int, orgID int) {
	if len(appIDs) > 0 {
		db.Where("application_id IN ?", appIDs).Delete(&models.ApplicationResponsibleUser{})
		db.Where("id IN ?", appIDs).Delete(&models.Application{})
	}
	if len(userIDs) > 0 {
		db.Where("user_id IN ?", userIDs).Delete(&models.Notification{})
		db.Where("id IN ?", userIDs).Delete(&models.User{})
	}
	if orgID > 0 {
		db.Delete(&models.Organization{}, orgID)
	}
}

// TestReminderService_RequiredApproverPastDeadline_SendsReminder — обязательный
// согласующий молчит дольше first_days (дефолт 3) - приходит напоминание, отметки
// last_reminder_at/reminder_count проставляются на его строке.
func TestReminderService_RequiredApproverPastDeadline_SendsReminder(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc, _ := newReminderServices(db)

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	approverID := newReminderUser(t, db, "approver")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusProcessing, models.ConfirmationPending)
	respID := newResponsible(t, db, appID, approverID, true, pendingStatus(), 4)
	defer cleanupReminderFixture(db, []int{appID}, []int{senderID, approverID}, orgID)

	require.NoError(t, svc.SendPendingReminders(context.Background()))

	assert.EqualValues(t, 1, countReminderNotifications(t, db, approverID),
		"согласующий молчит 4 дня при пороге 3 - должно прийти напоминание")

	r := loadResponsible(t, db, respID)
	require.NotNil(t, r.LastReminderAt, "last_reminder_at должен проставиться")
	assert.WithinDuration(t, time.Now(), *r.LastReminderAt, time.Minute)
	assert.Equal(t, 1, r.ReminderCount)
}

// TestReminderService_NoDoubleReminder_ThenRepeatsAfterInterval — повторный прогон
// сразу после первого не дублирует напоминание; после сдвига last_reminder_at за
// repeat_days (дефолт 3) уходит второе.
func TestReminderService_NoDoubleReminder_ThenRepeatsAfterInterval(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc, _ := newReminderServices(db)
	ctx := context.Background()

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	approverID := newReminderUser(t, db, "approver")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusProcessing, models.ConfirmationPending)
	respID := newResponsible(t, db, appID, approverID, true, pendingStatus(), 4)
	defer cleanupReminderFixture(db, []int{appID}, []int{senderID, approverID}, orgID)

	require.NoError(t, svc.SendPendingReminders(ctx))
	assert.EqualValues(t, 1, countReminderNotifications(t, db, approverID))

	require.NoError(t, svc.SendPendingReminders(ctx))
	assert.EqualValues(t, 1, countReminderNotifications(t, db, approverID),
		"повторный прогон сразу после первого не должен слать второе напоминание")

	past := time.Now().Add(-4 * 24 * time.Hour)
	require.NoError(t, db.Model(&models.ApplicationResponsibleUser{}).Where("id = ?", respID).
		Update("last_reminder_at", past).Error)
	require.NoError(t, svc.SendPendingReminders(ctx))
	assert.EqualValues(t, 2, countReminderNotifications(t, db, approverID),
		"после истечения repeat_days должно уйти повторное напоминание")

	r := loadResponsible(t, db, respID)
	assert.Equal(t, 2, r.ReminderCount)
}

// TestReminderService_NonRequiredApproverSkipped_WhenRequiredExists — если у заявки
// есть обязательный согласующий, необязательный напоминания не получает независимо
// от того, сколько он молчит (его голос на исход не влияет).
func TestReminderService_NonRequiredApproverSkipped_WhenRequiredExists(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc, _ := newReminderServices(db)

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	requiredID := newReminderUser(t, db, "required")
	optionalID := newReminderUser(t, db, "optional")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusProcessing, models.ConfirmationPending)
	newResponsible(t, db, appID, requiredID, true, pendingStatus(), 4)
	optRespID := newResponsible(t, db, appID, optionalID, false, pendingStatus(), 4)
	defer cleanupReminderFixture(db, []int{appID}, []int{senderID, requiredID, optionalID}, orgID)

	require.NoError(t, svc.SendPendingReminders(context.Background()))

	assert.EqualValues(t, 1, countReminderNotifications(t, db, requiredID))
	assert.EqualValues(t, 0, countReminderNotifications(t, db, optionalID),
		"необязательный согласующий не должен получать напоминание, когда есть обязательные")

	r := loadResponsible(t, db, optRespID)
	assert.Nil(t, r.LastReminderAt)
	assert.Zero(t, r.ReminderCount)
}

// TestReminderService_NoRequired_AlreadyApproved_OthersSkipped — обязательных нет,
// один уже согласовал (confirmation уже "Согласовано") - остальные pending
// напоминаний не получают, заявка уже решена.
func TestReminderService_NoRequired_AlreadyApproved_OthersSkipped(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc, _ := newReminderServices(db)

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	approvedID := newReminderUser(t, db, "approved")
	pendingID := newReminderUser(t, db, "pending")
	// confirmation "Согласовано" - ровно так updateConfirmationBasedOnApprovals
	// проставляет его синхронно с первым положительным голосом при отсутствии
	// обязательных согласующих.
	appID := newReminderApp(t, db, orgID, senderID, models.StatusInWork, models.ConfirmationApproved)
	votedAt := time.Now().Add(-4 * 24 * time.Hour)
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
		ApplicationID: appID, UserID: approvedID, CreatedAt: votedAt,
		RequiredApproval: false, ApprovalStatus: approvedStatus(), ApprovalDatetime: &votedAt,
	}).Error)
	pendingRespID := newResponsible(t, db, appID, pendingID, false, pendingStatus(), 4)
	defer cleanupReminderFixture(db, []int{appID}, []int{senderID, approvedID, pendingID}, orgID)

	require.NoError(t, svc.SendPendingReminders(context.Background()))

	assert.EqualValues(t, 0, countReminderNotifications(t, db, pendingID),
		"заявка уже согласована - остальные напоминаний не получают")
	r := loadResponsible(t, db, pendingRespID)
	assert.Nil(t, r.LastReminderAt)
}

// TestReminderService_WithdrawnApplication_NoReminder — отзыв заявки меняет только
// application.status (WithdrawApplication), confirmation остаётся "Согласование" -
// отбор обязан проверять статус отдельно, иначе отозванная заявка продолжит слать
// напоминания до архивации через месяц.
func TestReminderService_WithdrawnApplication_NoReminder(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc, _ := newReminderServices(db)

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	approverID := newReminderUser(t, db, "approver")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusWithdrawn, models.ConfirmationPending)
	newResponsible(t, db, appID, approverID, true, pendingStatus(), 4)
	defer cleanupReminderFixture(db, []int{appID}, []int{senderID, approverID}, orgID)

	require.NoError(t, svc.SendPendingReminders(context.Background()))

	assert.EqualValues(t, 0, countReminderNotifications(t, db, approverID),
		"отозванная заявка не должна получать напоминаний, даже если confirmation ещё не сброшен")
}

// TestReminderService_DisabledSetting_NoReminders — выключенный
// approval.reminder_enabled останавливает прогон целиком, даже при явно зависшем
// обязательном согласующем.
func TestReminderService_DisabledSetting_NoReminders(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc, settingsSvc := newReminderServices(db)
	ctx := context.Background()

	_, err := settingsSvc.Update(ctx, "approval.reminder_enabled", "false")
	require.NoError(t, err)
	defer func() {
		_, _ = settingsSvc.Update(ctx, "approval.reminder_enabled", "true")
	}()

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	approverID := newReminderUser(t, db, "approver")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusProcessing, models.ConfirmationPending)
	newResponsible(t, db, appID, approverID, true, pendingStatus(), 4)
	defer cleanupReminderFixture(db, []int{appID}, []int{senderID, approverID}, orgID)

	require.NoError(t, svc.SendPendingReminders(ctx))

	assert.EqualValues(t, 0, countReminderNotifications(t, db, approverID),
		"выключенная настройка - прогон должен молчать")
}
