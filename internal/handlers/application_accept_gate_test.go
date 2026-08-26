package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTakeToWork_RequiresApproval: принять заявку в работу можно только когда согласование
// завершено (confirmation='Согласовано' = все обязательные approved / при отсутствии
// обязательных хотя бы один approved). Заявка вообще без согласующих принимается без
// согласования (согласовывать нечего). Барьер authoritative на бэке - FE-гейт обходится.
func TestTakeToWork_RequiresApproval(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	t.Run("mandatory_approver_pending_blocks_accept", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "agsender", "pass123", 1, td.OrgID, td.CompanyID)
		appID := createSimpleApplication(t, e, senderToken, td.OrgID)

		// принимающий (isApprover)
		approverToken := testutil.RegisterAndLogin(t, e, "agapprover", "pass123", 1, td.OrgID, td.CompanyID)
		makeApprover(t, db, "agapprover")
		approverID := getUserID(t, db, "agapprover")

		// обязательный согласующий (ещё не голосовал)
		testutil.RegisterUser(t, e, "agreq", "pass123", 1, td.OrgID, td.CompanyID)
		reqID := getUserID(t, db, "agreq")
		fwd := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, reqID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), fwd, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		// принять в работу до согласования обязательного -> 400
		accept := fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, approverID)
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), accept, testutil.AuthHeader(approverToken))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "принять до согласования обязательного нельзя: %s", rec.Body.String())

		// обязательный согласует -> confirmation='Согласовано'
		reqToken, _ := testutil.LoginUser(t, e, "agreq", "pass123")
		ap := fmt.Sprintf(`{"user_id":%d,"status":"approved","comment":"ok"}`, reqID)
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), ap, testutil.AuthHeader(reqToken))
		require.Equal(t, http.StatusOK, rec.Code, "approve: %s", rec.Body.String())

		// теперь принять можно
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), accept, testutil.AuthHeader(approverToken))
		assert.Equal(t, http.StatusOK, rec.Code, "после согласования обязательного принять можно: %s", rec.Body.String())
	})

	t.Run("no_approvers_accept_allowed", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "agsender2", "pass123", 1, td.OrgID, td.CompanyID)
		appID := createSimpleApplication(t, e, senderToken, td.OrgID)

		approverToken := testutil.RegisterAndLogin(t, e, "agapprover2", "pass123", 1, td.OrgID, td.CompanyID)
		makeApprover(t, db, "agapprover2")
		approverID := getUserID(t, db, "agapprover2")

		// заявка без согласующих - принять можно без согласования
		accept := fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, approverID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), accept, testutil.AuthHeader(approverToken))
		assert.Equal(t, http.StatusOK, rec.Code, "заявку без согласующих принять можно: %s", rec.Body.String())
	})
}
