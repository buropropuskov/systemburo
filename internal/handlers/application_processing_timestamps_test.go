package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Таймстампы воронки обработки заявки (#1240, срез B1): accepted_at (T2) и completed_at
// (T3). Раньше моменты жили только в audit_log (принятие) либо нигде (завершение) - без
// колонок аналитика сроков обработки не считается.

func readApplicationTimestamps(t *testing.T, db *gorm.DB, appID int) (acceptedAt, completedAt *time.Time, status string) {
	t.Helper()
	var row struct {
		AcceptedAt  *time.Time
		CompletedAt *time.Time
		Status      *string
	}
	err := db.Raw("SELECT accepted_at, completed_at, status FROM applications WHERE id = ?", appID).Scan(&row).Error
	require.NoError(t, err)
	if row.Status != nil {
		status = *row.Status
	}
	return row.AcceptedAt, row.CompletedAt, status
}

// newWorkflowService собирает applicationService на тестовой БД: CheckExpiredAttachments
// зовётся кроном (cmd/server/main.go), HTTP-роута у него нет.
func newWorkflowService(db *gorm.DB) services.ApplicationService {
	recorder := services.NewAuditRecorder(db)
	return services.NewApplicationService(
		db,
		services.NewPermissionService(db),
		services.NewNotificationService(db),
		services.NewVehicleBlacklistService(db, recorder),
		services.NewPersonBlacklistService(db, recorder),
		recorder,
	)
}

func takeToWork(t *testing.T, e *echo.Echo, token string, appID, approverID int) {
	t.Helper()
	body := fmt.Sprintf(`{"user_id": %d, "action": "accept"}`, approverID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// TestApplicationAcceptedAt_FirstTakeWins: принятие в работу проставляет accepted_at, а
// повторное принятие после revoke/restore НЕ перетирает первый момент - иначе срок
// обработки считался бы от последней попытки, а не от реального начала работы.
func TestApplicationAcceptedAt_FirstTakeWins(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "apasender1", "pass123", 1, td.OrgID, td.CompanyID)
	approverToken := testutil.RegisterAndLogin(t, e, "apaapprover1", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "apaapprover1")
	approverID := getUserID(t, db, "apaapprover1")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_apa1", "Cars APA1")
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	accepted, _, _ := readApplicationTimestamps(t, db, appID)
	require.Nil(t, accepted, "до принятия в работу accepted_at пуст")

	beforeTake := time.Now()
	takeToWork(t, e, approverToken, appID, approverID)

	first, _, status := readApplicationTimestamps(t, db, appID)
	require.NotNil(t, first, "принятие в работу должно проставить accepted_at")
	assert.Equal(t, models.StatusInWork, status)
	assert.WithinDuration(t, beforeTake, *first, time.Minute, "accepted_at = момент принятия")

	// Отзыв из работы и возврат: момент первого принятия обязан пережить оба.
	recRevoke := testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-from-work", appID),
		fmt.Sprintf(`{"user_id": %d, "comment": "revoke"}`, approverID), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, recRevoke.Code, recRevoke.Body.String())

	afterRevoke, _, revokedStatus := readApplicationTimestamps(t, db, appID)
	require.NotNil(t, afterRevoke, "отзыв из работы не стирает accepted_at")
	assert.Equal(t, models.StatusProcessing, revokedStatus)
	assert.WithinDuration(t, *first, *afterRevoke, time.Millisecond)

	recRestore := testutil.POST(t, e, fmt.Sprintf("/applications/%d/restore-to-work", appID),
		fmt.Sprintf(`{"user_id": %d, "comment": "restore"}`, approverID), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, recRestore.Code, recRestore.Body.String())

	takeToWork(t, e, approverToken, appID, approverID)

	second, _, _ := readApplicationTimestamps(t, db, appID)
	require.NotNil(t, second)
	assert.WithinDuration(t, *first, *second, time.Millisecond,
		"повторное принятие после revoke/restore не перетирает первый момент")
}

// TestApplicationCompletedAt_ExpiryWritesSystemAudit: истечение срока вложений завершает
// заявку, проставляет completed_at и пишет системное событие completed - оно должно
// доехать до истории заявки (актор NULL -> user_id 0 -> FE рисует «Система»).
func TestApplicationCompletedAt_ExpiryWritesSystemAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "apasender2", "pass123", 1, td.OrgID, td.CompanyID)
	approverToken := testutil.RegisterAndLogin(t, e, "apaapprover2", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "apaapprover2")
	approverID := getUserID(t, db, "apaapprover2")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_apa2", "Cars APA2")
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)
	takeToWork(t, e, approverToken, appID, approverID)

	require.NoError(t, db.Exec("UPDATE attachments SET entry_date_to = '2020-01-01' WHERE application_id = ?", appID).Error)

	svc := newWorkflowService(db)
	beforeRun := time.Now()
	require.NoError(t, svc.CheckExpiredAttachments(context.Background()))

	_, completed, status := readApplicationTimestamps(t, db, appID)
	require.NotNil(t, completed, "истёкшая заявка должна получить completed_at")
	assert.Equal(t, models.StatusCompleted, status)
	assert.WithinDuration(t, beforeRun, *completed, time.Minute)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)

	entry := findHistoryEntry(t, hist, "completed")
	assert.Equal(t, float64(0), entry["user_id"], "актора нет: завершил крон, FE рисует «Система»")
	assert.Equal(t, models.StatusCompleted, entry["new_value"])
	assert.Equal(t, models.StatusInWork, entry["old_value"], "old_value = статус до завершения")

	// Повторный прогон по уже завершённой заявке: возвращаем вложение в активные, чтобы
	// крон снова дошёл до неё, - гард по статусу не должен ни сдвинуть момент, ни задвоить событие.
	require.NoError(t, db.Exec("UPDATE attachments SET status = 1 WHERE application_id = ?", appID).Error)
	require.NoError(t, svc.CheckExpiredAttachments(context.Background()))

	_, completedAgain, _ := readApplicationTimestamps(t, db, appID)
	require.NotNil(t, completedAgain)
	assert.WithinDuration(t, *completed, *completedAgain, time.Millisecond, "повторный прогон не сдвигает completed_at")

	var completedEvents int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM audit_log WHERE entity_type = ? AND entity_id = ? AND action = 'completed'",
		models.AuditEntityApplication, appID).Scan(&completedEvents).Error)
	assert.Equal(t, int64(1), completedEvents, "событие завершения пишется один раз")
}

// TestApplicationCompletedAt_RefusedApplicationNotCompleted: у отказа решение принято
// человеком, и истёкший срок вложений его не отменяет. Ловушка: reject гасит только
// машины/людей, а attachments остаются активными (в отличие от withdraw, который гасит их
// сам) - значит крон до отказанной заявки доходит и без белого списка статусов переписал
// бы "Отказано" на "Завершено" с фальшивым completed_at и событием в истории.
func TestApplicationCompletedAt_RefusedApplicationNotCompleted(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "apasender4", "pass123", 1, td.OrgID, td.CompanyID)
	approverToken := testutil.RegisterAndLogin(t, e, "apaapprover4", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "apaapprover4")
	approverID := getUserID(t, db, "apaapprover4")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_apa4", "Cars APA4")
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	rejectBody := fmt.Sprintf(`{"user_id": %d, "action": "reject", "comment": "не пропускаем"}`, approverID)
	recReject := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), rejectBody, testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, recReject.Code, recReject.Body.String())

	var activeAttachments int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM attachments WHERE application_id = ? AND status = 1", appID).Scan(&activeAttachments).Error)
	require.Equal(t, int64(1), activeAttachments, "предпосылка теста: reject оставляет вложения активными")

	require.NoError(t, db.Exec("UPDATE attachments SET entry_date_to = '2020-01-01' WHERE application_id = ?", appID).Error)
	require.NoError(t, newWorkflowService(db).CheckExpiredAttachments(context.Background()))

	_, completed, status := readApplicationTimestamps(t, db, appID)
	assert.Equal(t, models.StatusRefused, status, "отказ не должен превращаться в завершение по сроку")
	assert.Nil(t, completed, "у отказанной заявки нет момента завершения")

	var completedEvents int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM audit_log WHERE entity_type = ? AND entity_id = ? AND action = 'completed'",
		models.AuditEntityApplication, appID).Scan(&completedEvents).Error)
	assert.Equal(t, int64(0), completedEvents, "фальшивое событие завершения поверх отказа не пишется")
}

// TestBackfillApplicationAcceptedAt_FromAuditLog: заявкам, принятым до появления колонки,
// accepted_at восстанавливается из первой записи take_to_work в audit_log.
func TestBackfillApplicationAcceptedAt_FromAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "apasender3", "pass123", 1, td.OrgID, td.CompanyID)
	approverToken := testutil.RegisterAndLogin(t, e, "apaapprover3", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "apaapprover3")
	approverID := getUserID(t, db, "apaapprover3")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_apa3", "Cars APA3")

	acceptedApp := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)
	takeToWork(t, e, approverToken, acceptedApp, approverID)
	untouchedApp := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	// Состояние до-колоночной эры: событие в аудите есть, колонка пуста.
	require.NoError(t, db.Exec("UPDATE applications SET accepted_at = NULL WHERE id = ?", acceptedApp).Error)

	require.NoError(t, database.BackfillApplicationAcceptedAt(db))

	restored, _, _ := readApplicationTimestamps(t, db, acceptedApp)
	require.NotNil(t, restored, "accepted_at восстановлен из audit_log take_to_work")

	var auditAt time.Time
	require.NoError(t, db.Raw(`SELECT MIN(created_at) FROM audit_log
		WHERE entity_type = ? AND entity_id = ? AND action = 'take_to_work'`,
		models.AuditEntityApplication, acceptedApp).Scan(&auditAt).Error)
	assert.WithinDuration(t, auditAt, *restored, time.Second, "момент = первое take_to_work")

	neverAccepted, _, _ := readApplicationTimestamps(t, db, untouchedApp)
	assert.Nil(t, neverAccepted, "заявка без take_to_work остаётся с пустым accepted_at")

	// Идемпотентность: повторный прогон AutoMigrate не переписывает уже проставленный момент.
	require.NoError(t, database.BackfillApplicationAcceptedAt(db))
	afterSecondRun, _, _ := readApplicationTimestamps(t, db, acceptedApp)
	require.NotNil(t, afterSecondRun)
	assert.WithinDuration(t, *restored, *afterSecondRun, time.Millisecond)
}
