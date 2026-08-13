package handlers_test

// Интеграционные тесты уведомлений категории «Проходы и сроки» (#1748, S4):
// истечение срока действия пропуска завтра, отзыв заявки, назначение принимающего,
// первый проход. Сервис-паттерн без CleanDB для прямых сервис-тестов (как в
// reminder_service_test.go) - переиспользует его хелперы (newReminderOrg,
// newReminderUser, newReminderApp, cleanupReminderFixture, uniqRem). HTTP-тесты
// (withdraw, take-to-work) идут по образцу application_withdraw_test.go с CleanDB.

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// --- Item 1: срок действия пропуска истекает завтра ---

func newExpiringAttachment(t *testing.T, db *gorm.DB, appID int, entryDateTo string) int {
	t.Helper()
	status := 1
	att := models.Attachment{ApplicationID: &appID, AttachmentType: "cars", EntryDateTo: &entryDateTo, Status: &status}
	require.NoError(t, db.Create(&att).Error)
	return att.ID
}

func countExpiryNotifications(t *testing.T, db *gorm.DB, userID int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", userID, services.NotificationTypeApplicationExpiring).
		Count(&count).Error)
	return count
}

// expiryMessages возвращает тексты предупреждений пользователя в порядке создания.
func expiryMessages(t *testing.T, db *gorm.DB, userID int) []string {
	t.Helper()
	var messages []string
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", userID, services.NotificationTypeApplicationExpiring).
		Order("id").Pluck("message", &messages).Error)
	return messages
}

// daysFromNow - дата через n дней в формате entry_date_to.
func daysFromNow(n int) string {
	return time.Now().AddDate(0, 0, n).Format("2006-01-02")
}

// TestExpiryNotify_OneNotificationPerApplication - заявка с двумя вложениями,
// истекающими завтра, должна дать ровно одно уведомление инициатору, а не по одному
// на вложение.
func TestExpiryNotify_OneNotificationPerApplication(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewExpiryNotifyService(db, services.NewNotificationService(db))

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusInWork, models.ConfirmationApproved)
	att1 := newExpiringAttachment(t, db, appID, daysFromNow(1))
	att2 := newExpiringAttachment(t, db, appID, daysFromNow(1))
	defer func() {
		db.Where("id IN ?", []int{att1, att2}).Delete(&models.Attachment{})
		cleanupReminderFixture(db, []int{appID}, []int{senderID}, orgID)
	}()

	require.NoError(t, svc.NotifyExpiringSoon(context.Background()))

	assert.EqualValues(t, 1, countExpiryNotifications(t, db, senderID),
		"заявка с двумя вложениями, истекающими завтра, должна дать одно уведомление на заявку")
	require.Len(t, expiryMessages(t, db, senderID), 1)
	assert.Contains(t, expiryMessages(t, db, senderID)[0], "завтра",
		"накануне человеку понятнее слово, а не «через 1 день»")
}

// TestExpiryNotify_ThreeDaysAhead - предупреждение за три дня приходит и называет срок
// словами, а не только датой.
func TestExpiryNotify_ThreeDaysAhead(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewExpiryNotifyService(db, services.NewNotificationService(db))

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusInWork, models.ConfirmationApproved)
	attID := newExpiringAttachment(t, db, appID, daysFromNow(3))
	defer func() {
		db.Where("id = ?", attID).Delete(&models.Attachment{})
		cleanupReminderFixture(db, []int{appID}, []int{senderID}, orgID)
	}()

	require.NoError(t, svc.NotifyExpiringSoon(context.Background()))

	messages := expiryMessages(t, db, senderID)
	require.Len(t, messages, 1, "за три дня до конца срока приходит одно предупреждение")
	assert.Contains(t, messages[0], "через 3 дня")
	assert.Contains(t, messages[0], time.Now().AddDate(0, 0, 3).Format("02.01.2006"))
}

// TestExpiryNotify_NoDuplicateOnRepeatRun - повторный прогон в те же сутки (задача
// крутится раз в сутки, но при рестарте сервиса может отработать чаще) не должен
// слать второе предупреждение по той же заявке.
func TestExpiryNotify_NoDuplicateOnRepeatRun(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewExpiryNotifyService(db, services.NewNotificationService(db))

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusInWork, models.ConfirmationApproved)
	attID := newExpiringAttachment(t, db, appID, daysFromNow(1))
	defer func() {
		db.Where("id = ?", attID).Delete(&models.Attachment{})
		cleanupReminderFixture(db, []int{appID}, []int{senderID}, orgID)
	}()

	require.NoError(t, svc.NotifyExpiringSoon(context.Background()))
	assert.EqualValues(t, 1, countExpiryNotifications(t, db, senderID))

	require.NoError(t, svc.NotifyExpiringSoon(context.Background()))
	assert.EqualValues(t, 1, countExpiryNotifications(t, db, senderID),
		"повторный прогон в те же сутки не должен дублировать предупреждение")
}

// TestExpiryNotify_SkipsOffThresholdDates - сроки, не попавшие в пороги (сегодня,
// послезавтра, через неделю), молчат: предупреждаем только за три дня и накануне.
func TestExpiryNotify_SkipsOffThresholdDates(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewExpiryNotifyService(db, services.NewNotificationService(db))

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	defer cleanupReminderFixture(db, nil, []int{senderID}, orgID)

	for _, days := range []int{0, 2, 7} {
		appID := newReminderApp(t, db, orgID, senderID, models.StatusInWork, models.ConfirmationApproved)
		attID := newExpiringAttachment(t, db, appID, daysFromNow(days))

		require.NoError(t, svc.NotifyExpiringSoon(context.Background()))
		assert.EqualValues(t, 0, countExpiryNotifications(t, db, senderID),
			"срок через %d дн. не попадает в пороги предупреждения", days)

		db.Where("id = ?", attID).Delete(&models.Attachment{})
		cleanupReminderFixture(db, []int{appID}, nil, 0)
	}
}

// TestExpiryNotify_UsesLatestAttachmentDate - срок заявки общий: он считается по самой
// поздней дате среди активных вложений. Заявка, где машины кончаются завтра, а люди
// через неделю, живёт до конца недели и предупреждения завтра не даёт.
func TestExpiryNotify_UsesLatestAttachmentDate(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewExpiryNotifyService(db, services.NewNotificationService(db))

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusInWork, models.ConfirmationApproved)
	soon := newExpiringAttachment(t, db, appID, daysFromNow(1))
	late := newExpiringAttachment(t, db, appID, daysFromNow(7))
	defer func() {
		db.Where("id IN ?", []int{soon, late}).Delete(&models.Attachment{})
		cleanupReminderFixture(db, []int{appID}, []int{senderID}, orgID)
	}()

	require.NoError(t, svc.NotifyExpiringSoon(context.Background()))
	assert.EqualValues(t, 0, countExpiryNotifications(t, db, senderID),
		"общий срок заявки - через неделю, накануне частичного окончания не предупреждаем")

	// Отодвигаем позднее вложение на три дня вперёд от сегодня: общий срок заявки
	// становится порогом, и предупреждение приходит.
	require.NoError(t, db.Model(&models.Attachment{}).Where("id = ?", late).
		Update("entry_date_to", daysFromNow(3)).Error)

	require.NoError(t, svc.NotifyExpiringSoon(context.Background()))
	messages := expiryMessages(t, db, senderID)
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], "через 3 дня", "срок считается по самой поздней дате вложений")
}

// TestExpiryNotify_IgnoresInactiveAttachments - погашенное вложение (status=0) в общий
// срок не входит: иначе заявка, у которой машины сняли досрочно, продолжала бы
// предупреждать по их старой дате.
func TestExpiryNotify_IgnoresInactiveAttachments(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewExpiryNotifyService(db, services.NewNotificationService(db))

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusInWork, models.ConfirmationApproved)
	active := newExpiringAttachment(t, db, appID, daysFromNow(1))
	cancelled := newExpiringAttachment(t, db, appID, daysFromNow(7))
	require.NoError(t, db.Model(&models.Attachment{}).Where("id = ?", cancelled).Update("status", 0).Error)
	defer func() {
		db.Where("id IN ?", []int{active, cancelled}).Delete(&models.Attachment{})
		cleanupReminderFixture(db, []int{appID}, []int{senderID}, orgID)
	}()

	require.NoError(t, svc.NotifyExpiringSoon(context.Background()))
	assert.EqualValues(t, 1, countExpiryNotifications(t, db, senderID),
		"снятое вложение срок заявки не продлевает")
}

// --- Item 2: заявка отозвана инициатором ---

// TestWithdrawApplication_NotifiesOnlyPendingApprovers - отзыв уведомляет только тех
// согласующих, чьё решение реально ждали (required, ещё не голосовал), а не тех, кто
// уже проголосовал, и не самого инициатора.
func TestWithdrawApplication_NotifiesOnlyPendingApprovers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "wdnsender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "wdnsender")
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	// Обязательный согласующий, ещё не проголосовавший - его решения ждут.
	testutil.RegisterUser(t, e, "wdnpending", "pass123", 1, td.OrgID, td.CompanyID)
	pendingID := getUserID(t, db, "wdnpending")
	fwdPending := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, pendingID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwdPending, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward pending: %s", rec.Body.String())

	// Необязательный согласующий, уже проголосовавший - решение принято, ждать нечего.
	testutil.RegisterAndLogin(t, e, "wdnvoted", "pass123", 1, td.OrgID, td.CompanyID)
	votedID := getUserID(t, db, "wdnvoted")
	fwdVoted := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":false,"can_view":false}]}`, votedID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwdVoted, testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "forward voted: %s", rec.Body.String())
	votedToken, _ := testutil.LoginUser(t, e, "wdnvoted", "pass123")
	ap := fmt.Sprintf(`{"user_id":%d,"status":"approved","comment":"ok"}`, votedID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), ap, testutil.AuthHeader(votedToken))
	require.Equal(t, http.StatusOK, rec.Code, "approve: %s", rec.Body.String())

	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/withdraw", appID), "", testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, "withdraw: %s", rec.Body.String())

	var pendingCount, votedCount, senderCount int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", pendingID, services.NotificationTypeApplicationWithdrawn).Count(&pendingCount).Error)
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", votedID, services.NotificationTypeApplicationWithdrawn).Count(&votedCount).Error)
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", senderID, services.NotificationTypeApplicationWithdrawn).Count(&senderCount).Error)

	assert.EqualValues(t, 1, pendingCount, "согласующий, чьего решения ждали, должен получить уведомление об отзыве")
	assert.EqualValues(t, 0, votedCount, "уже проголосовавший согласующий уведомление не получает")
	assert.EqualValues(t, 0, senderCount, "инициатор отзыва не уведомляет сам себя")
}

// --- Item 3: назначен принимающий ---

// TestTakeApplicationToWork_Accept_SetsResponsibleToActor фиксирует, почему уведомления
// "назначен принимающий" в системе нет: принимающим становится сам актор (self-accept),
// пути назначить принимающим ДРУГОГО человека у responsible_user_id сейчас не существует.
// Появится такой путь - уведомление станет осмысленным, и этот тест покажет, что изменилось.
func TestTakeApplicationToWork_Accept_SetsResponsibleToActor(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "acsender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	approverToken := testutil.RegisterAndLogin(t, e, "acapprover", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "acapprover")
	approverID := getUserID(t, db, "acapprover")

	accept := fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, approverID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), accept, testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "accept: %s", rec.Body.String())

	var responsibleID *int
	require.NoError(t, db.Raw("SELECT responsible_user_id FROM applications WHERE id = ?", appID).Scan(&responsibleID).Error)
	require.NotNil(t, responsibleID)
	assert.Equal(t, approverID, *responsibleID, "принявший в работу становится responsible_user")

}

// --- Item 4: первый проход по заявке ---

func newPassageApplication(t *testing.T, db *gorm.DB, orgID, senderID int) int {
	t.Helper()
	return newReminderApp(t, db, orgID, senderID, models.StatusInWork, models.ConfirmationApproved)
}

func newPassageAttachment(t *testing.T, db *gorm.DB, appID int, attachmentType string) int {
	t.Helper()
	status := 1
	att := models.Attachment{ApplicationID: &appID, AttachmentType: attachmentType, Status: &status}
	require.NoError(t, db.Create(&att).Error)
	return att.ID
}

