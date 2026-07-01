package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithdrawApplication_BySender проверяет отзыв своей заявки отправителем (#951):
// статус -> "Отозвана", машины и вложения деактивируются, в audit_log[application]
// пишется action=withdraw с актором, событие видно в истории заявки.
func TestWithdrawApplication_BySender(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "wdsender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "wdsender")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_wd", "Cars WD")
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/withdraw", appID), "", testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Статус -> Отозвана.
	var status *string
	require.NoError(t, db.Raw("SELECT status FROM applications WHERE id = ?", appID).Scan(&status).Error)
	require.NotNil(t, status)
	assert.Equal(t, models.StatusWithdrawn, *status, "статус заявки стал Отозвана")

	// Вложения деактивированы.
	var activeAtt int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM attachments WHERE application_id = ? AND status = 1", appID).Scan(&activeAtt).Error)
	assert.Equal(t, int64(0), activeAtt, "все вложения деактивированы")

	// Машины вложений деактивированы.
	var activeCars int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM cars c JOIN attachments a ON a.id = c.attachment_id
		WHERE a.application_id = ? AND c.status = 1`, appID).Scan(&activeCars).Error)
	assert.Equal(t, int64(0), activeCars, "все машины заявки деактивированы")

	// Запись в audit_log[application] с актором.
	var entry models.AuditLog
	require.NoError(t, db.Where("entity_type = ? AND entity_id = ? AND action = ?",
		models.AuditEntityApplication, appID, "withdraw").Order("id DESC").First(&entry).Error)
	require.NotNil(t, entry.ActorUserID, "у записи отзыва есть актор")
	assert.Equal(t, senderID, *entry.ActorUserID, "актор отзыва = отправитель")

	// Видно в истории заявки.
	recHist := testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, recHist.Code, recHist.Body.String())
	found := false
	for _, it := range testutil.ParseSlice(t, recHist) {
		if it["action_type"] == "withdraw" {
			found = true
		}
	}
	assert.True(t, found, "отзыв виден в истории заявки")
}

// TestWithdrawApplication_OnlySender проверяет, что чужую заявку отозвать нельзя.
func TestWithdrawApplication_OnlySender(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "wdowner", "pass123", 1, td.OrgID, td.CompanyID)
	otherToken := testutil.RegisterAndLogin(t, e, "wdother", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_wd2", "Cars WD2")
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/withdraw", appID), "", testutil.AuthHeader(otherToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, "не-отправитель не может отозвать: %s", rec.Body.String())
}

// TestWithdrawApplication_BlocksReverseActions проверяет, что отозванную заявку
// нельзя принять в работу (checkNotWithdrawn -> 409).
func TestWithdrawApplication_BlocksReverseActions(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "wdsender3", "pass123", 1, td.OrgID, td.CompanyID)
	approverToken := testutil.RegisterAndLogin(t, e, "wdapprover3", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "wdapprover3")
	approverID := getUserID(t, db, "wdapprover3")
	uaID := seedUniqueAttachment(t, db, "cars", "cars_wd3", "Cars WD3")
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	recW := testutil.POST(t, e, fmt.Sprintf("/applications/%d/withdraw", appID), "", testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, recW.Code, recW.Body.String())

	takeBody := fmt.Sprintf(`{"user_id": %d, "action": "accept"}`, approverID)
	recTake := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), takeBody, testutil.AuthHeader(approverToken))
	assert.Equal(t, http.StatusConflict, recTake.Code, "отозванную заявку нельзя принять в работу: %s", recTake.Body.String())
}

// TestWithdrawApplication_DoubleWithdraw проверяет, что повторный отзыв уже
// отозванной заявки отбивается терминальным гейтом (409).
func TestWithdrawApplication_DoubleWithdraw(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "wdsender4", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_wd4", "Cars WD4")
	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)

	rec1 := testutil.POST(t, e, fmt.Sprintf("/applications/%d/withdraw", appID), "", testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec1.Code, rec1.Body.String())

	rec2 := testutil.POST(t, e, fmt.Sprintf("/applications/%d/withdraw", appID), "", testutil.AuthHeader(senderToken))
	assert.Equal(t, http.StatusConflict, rec2.Code, "повторный отзыв отозванной заявки -> 409: %s", rec2.Body.String())
}
