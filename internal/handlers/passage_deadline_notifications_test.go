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

// TestExpiryNotify_OneNotificationPerApplication - заявка с двумя вложениями,
// истекающими завтра, должна дать ровно одно уведомление инициатору, а не по одному
// на вложение.
func TestExpiryNotify_OneNotificationPerApplication(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewExpiryNotifyService(db, services.NewNotificationService(db))

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusInWork, models.ConfirmationApproved)
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	att1 := newExpiringAttachment(t, db, appID, tomorrow)
	att2 := newExpiringAttachment(t, db, appID, tomorrow)
	defer func() {
		db.Where("id IN ?", []int{att1, att2}).Delete(&models.Attachment{})
		cleanupReminderFixture(db, []int{appID}, []int{senderID}, orgID)
	}()

	require.NoError(t, svc.NotifyExpiringTomorrow(context.Background()))

	assert.EqualValues(t, 1, countExpiryNotifications(t, db, senderID),
		"заявка с двумя вложениями, истекающими завтра, должна дать одно уведомление на заявку")
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
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	attID := newExpiringAttachment(t, db, appID, tomorrow)
	defer func() {
		db.Where("id = ?", attID).Delete(&models.Attachment{})
		cleanupReminderFixture(db, []int{appID}, []int{senderID}, orgID)
	}()

	require.NoError(t, svc.NotifyExpiringTomorrow(context.Background()))
	assert.EqualValues(t, 1, countExpiryNotifications(t, db, senderID))

	require.NoError(t, svc.NotifyExpiringTomorrow(context.Background()))
	assert.EqualValues(t, 1, countExpiryNotifications(t, db, senderID),
		"повторный прогон в те же сутки не должен дублировать предупреждение")
}

// TestExpiryNotify_SkipsFarDates - вложение, истекающее не завтра (ни сегодня, ни
// послезавтра), уведомления не даёт.
func TestExpiryNotify_SkipsFarDates(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	svc := services.NewExpiryNotifyService(db, services.NewNotificationService(db))

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	appID := newReminderApp(t, db, orgID, senderID, models.StatusInWork, models.ConfirmationApproved)
	today := time.Now().Format("2006-01-02")
	nextWeek := time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02")
	att1 := newExpiringAttachment(t, db, appID, today)
	att2 := newExpiringAttachment(t, db, appID, nextWeek)
	defer func() {
		db.Where("id IN ?", []int{att1, att2}).Delete(&models.Attachment{})
		cleanupReminderFixture(db, []int{appID}, []int{senderID}, orgID)
	}()

	require.NoError(t, svc.NotifyExpiringTomorrow(context.Background()))
	assert.EqualValues(t, 0, countExpiryNotifications(t, db, senderID),
		"вложения, истекающие сегодня или через неделю - не завтра, уведомления не дают")
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

func countPassageFirstNotifications(t *testing.T, db *gorm.DB, userID int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", userID, services.NotificationTypeApplicationPassageFirst).
		Count(&count).Error)
	return count
}

// TestFirstPassage_CarEntry_SecondEntryDoesNotDuplicate - первый въезд машины по
// заявке уведомляет инициатора, второй въезд (другой машины той же заявки) новую
// запись не создаёт.
func TestFirstPassage_CarEntry_SecondEntryDoesNotDuplicate(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	notifSvc := services.NewNotificationService(db)
	recorder := services.NewAuditRecorder(db)
	carSvc := services.NewCarService(db, recorder, services.WithCarNotifications(notifSvc))

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	appID := newPassageApplication(t, db, orgID, senderID)
	attID := newPassageAttachment(t, db, appID, "cars")

	statusZero := 0
	car1 := models.Car{AttachmentID: attID, Status: &statusZero, TerritoryStatus: &statusZero}
	require.NoError(t, db.Create(&car1).Error)
	car2 := models.Car{AttachmentID: attID, Status: &statusZero, TerritoryStatus: &statusZero}
	require.NoError(t, db.Create(&car2).Error)

	defer func() {
		db.Where("attachment_id = ?", attID).Delete(&models.Car{})
		db.Delete(&models.Attachment{}, attID)
		cleanupReminderFixture(db, []int{appID}, []int{senderID}, orgID)
	}()

	ctx := context.Background()
	require.NoError(t, carSvc.UpdateCarTerritoryStatus(ctx, car1.ID, services.UpdateCarTerritoryStatusRequest{
		UpdateTerritoryStatusRequest: services.UpdateTerritoryStatusRequest{TerritoryStatus: 1, UserID: &senderID},
	}))
	assert.EqualValues(t, 1, countPassageFirstNotifications(t, db, senderID), "первый въезд должен уведомить инициатора")

	require.NoError(t, carSvc.UpdateCarTerritoryStatus(ctx, car2.ID, services.UpdateCarTerritoryStatusRequest{
		UpdateTerritoryStatusRequest: services.UpdateTerritoryStatusRequest{TerritoryStatus: 1, UserID: &senderID},
	}))
	assert.EqualValues(t, 1, countPassageFirstNotifications(t, db, senderID),
		"второй въезд по той же заявке не должен создавать новую запись уведомления")
}

// TestFirstPassage_MixedCarAndEmployee_ShareFirstCounter - "первый проход" общий на
// заявку между машинами и людьми: вход сотрудника ПОСЛЕ въезда машины той же заявки
// не считается первым.
func TestFirstPassage_MixedCarAndEmployee_ShareFirstCounter(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	notifSvc := services.NewNotificationService(db)
	recorder := services.NewAuditRecorder(db)
	carSvc := services.NewCarService(db, recorder, services.WithCarNotifications(notifSvc))
	empSvc := services.NewEmployeeService(db, recorder, services.WithEmployeeNotifications(notifSvc))

	orgID := newReminderOrg(t, db)
	senderID := newReminderUser(t, db, "sender")
	appID := newPassageApplication(t, db, orgID, senderID)
	attID := newPassageAttachment(t, db, appID, "cars")

	statusZero := 0
	car := models.Car{AttachmentID: attID, Status: &statusZero, TerritoryStatus: &statusZero}
	require.NoError(t, db.Create(&car).Error)
	lastName, firstName := "Тестов", "Тест"
	emp := models.Employee{AttachmentID: &attID, LastName: &lastName, FirstName: &firstName, Status: &statusZero, TerritoryStatus: &statusZero}
	require.NoError(t, db.Create(&emp).Error)

	defer func() {
		db.Where("id = ?", emp.ID).Delete(&models.Employee{})
		db.Where("id = ?", car.ID).Delete(&models.Car{})
		db.Delete(&models.Attachment{}, attID)
		cleanupReminderFixture(db, []int{appID}, []int{senderID}, orgID)
	}()

	ctx := context.Background()
	require.NoError(t, carSvc.UpdateCarTerritoryStatus(ctx, car.ID, services.UpdateCarTerritoryStatusRequest{
		UpdateTerritoryStatusRequest: services.UpdateTerritoryStatusRequest{TerritoryStatus: 1, UserID: &senderID},
	}))
	assert.EqualValues(t, 1, countPassageFirstNotifications(t, db, senderID), "первый проход (машина) должен уведомить инициатора")

	require.NoError(t, empSvc.UpdateEmployeeTerritoryStatus(ctx, emp.ID, services.UpdateTerritoryStatusRequest{TerritoryStatus: 1, UserID: &senderID}))
	assert.EqualValues(t, 1, countPassageFirstNotifications(t, db, senderID),
		"вход сотрудника по той же заявке после машины не создаёт новую запись - первый проход уже был")
}

// TestFirstPassage_ManualAttachment_NoApplication_NoNotification - вложение без
// заявки (ручное добавление в таблицу проходной, #1049) уведомлять некого:
// applications.sender_user_id для него не существует.
func TestFirstPassage_ManualAttachment_NoApplication_NoNotification(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	notifSvc := services.NewNotificationService(db)
	recorder := services.NewAuditRecorder(db)
	carSvc := services.NewCarService(db, recorder, services.WithCarNotifications(notifSvc))

	status := 1
	att := models.Attachment{AttachmentType: "cars", IsManual: true, Status: &status}
	require.NoError(t, db.Create(&att).Error)
	statusZero := 0
	car := models.Car{AttachmentID: att.ID, Status: &statusZero, TerritoryStatus: &statusZero}
	require.NoError(t, db.Create(&car).Error)
	defer func() {
		db.Delete(&models.Car{}, car.ID)
		db.Delete(&models.Attachment{}, att.ID)
	}()

	var before int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("type = ?", services.NotificationTypeApplicationPassageFirst).Count(&before).Error)

	userID := 0
	require.NoError(t, carSvc.UpdateCarTerritoryStatus(context.Background(), car.ID, services.UpdateCarTerritoryStatusRequest{
		UpdateTerritoryStatusRequest: services.UpdateTerritoryStatusRequest{TerritoryStatus: 1, UserID: &userID},
	}))

	var after int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("type = ?", services.NotificationTypeApplicationPassageFirst).Count(&after).Error)
	assert.Equal(t, before, after, "въезд машины без заявки (ручное добавление) не должен падать и не создаёт уведомление")
}
