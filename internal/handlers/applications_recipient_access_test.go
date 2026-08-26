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

// TestCanAccess_RecipientOpensAnyApplication: принимающий (application_approvers,
// admin-раздел "Принимающие") может открывать детали любой заявки, даже не будучи
// отправителем/ответственным/viewer. Список центра показывает принимающему все
// заявки, а детальный гейт CanAccessApplication раньше его не пускал -> 403
// "вижу в списке, но не могу открыть". Посторонний по-прежнему получает 403
// (гейт security-аудита для обычных юзеров сохранён).
func TestCanAccess_RecipientOpensAnyApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Отправитель и его заявка.
	senderToken := testutil.RegisterAndLogin(t, e, "ra_sender", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	// Принимающий: отдельный юзер в application_approvers, НЕ участник заявки.
	recipientToken := testutil.RegisterAndLogin(t, e, "ra_recipient", "pass123", 1, td.OrgID, td.CompanyID)
	recipientID := getUserID(t, db, "ra_recipient")
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: recipientID}).Error)

	// Принимающий открывает детали и историю чужой заявки -> 200 (до фикса 403).
	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", appID), testutil.AuthHeader(recipientToken))
	assert.Equal(t, http.StatusOK, rec.Code, "принимающий должен открывать детали любой заявки: %s", rec.Body.String())

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(recipientToken))
	assert.Equal(t, http.StatusOK, rec.Code, "принимающий должен видеть историю любой заявки")

	// Контроль безопасности: посторонний (не принимающий, не участник) -> 403.
	outsiderToken := testutil.RegisterAndLogin(t, e, "ra_outsider", "pass123", 1, td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/details", appID), testutil.AuthHeader(outsiderToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, "посторонний не должен открывать чужую заявку (гейт аудита цел)")
}
