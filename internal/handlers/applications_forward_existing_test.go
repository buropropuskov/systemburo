package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// TestForward_ExistingResponsibleUntouched: пересылка уже назначенному ответственному не
// меняет его обязательность голоса. Раньше повторная пересылка делала UPDATE
// required_approval: отправитель снимал обязательность с ожидающего согласующего, кворум
// считал только оставшихся, и заявка становилась Согласовано без его голоса. Обязательность
// задаёт справочник согласующих при подаче (#2037), а не тот, кто пересылает. Форма
// пересылки уже назначенных не предлагает, так что штатный сценарий не страдает.
func TestForward_ExistingResponsibleUntouched(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorToken := testutil.RegisterAndLogin(t, e, "fwdex_author", "pass123", 1, td.OrgID, td.CompanyID)
	authorID := getUserID(t, db, "fwdex_author")
	voterToken := testutil.RegisterAndLogin(t, e, "fwdex_voter", "pass123", 1, td.OrgID, td.CompanyID)
	voterID := getUserID(t, db, "fwdex_voter")
	accepterToken := testutil.RegisterAndLogin(t, e, "fwdex_accepter", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "fwdex_accepter")
	superToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "fwdex_pending", "pass123", 1, td.OrgID, td.CompanyID)
	pendingID := getUserID(t, db, "fwdex_pending")
	testutil.RegisterUser(t, e, "fwdex_optional", "pass123", 1, td.OrgID, td.CompanyID)
	optionalID := getUserID(t, db, "fwdex_optional")

	actors := []struct {
		name, token string
	}{
		{"заявитель", authorToken},
		{"согласующий", voterToken},
		{"принимающий", accepterToken},
		{"супер-админ", superToken},
	}
	cases := []struct {
		name         string
		target       func() int
		wantRequired bool
		body         func(id int) string
	}{
		{"понижение ожидающего согласующего", func() int { return pendingID }, true,
			func(id int) string { return forwardBody(id, false, false) }},
		{"повышение ответственного без голоса", func() int { return optionalID }, false,
			func(id int) string { return forwardBody(id, true, false) }},
	}

	n := 0
	for _, actor := range actors {
		for _, tc := range cases {
			t.Run(actor.name+"/"+tc.name, func(t *testing.T) {
				n++
				app := suppApp(t, db, td.OrgID, authorID, fmt.Sprintf("FWDEX-%d", n),
					models.ConfirmationPending, models.StatusProcessing)
				suppResponsible(t, db, app, voterID, true, "approved")
				suppResponsible(t, db, app, pendingID, true, "pending")
				suppResponsible(t, db, app, optionalID, false, "pending")

				target := tc.target()
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", app),
					tc.body(target), testutil.AuthHeader(actor.token))
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

				var required bool
				require.NoError(t, db.Raw(`SELECT required_approval FROM application_responsible_users
					WHERE application_id = ? AND user_id = ?`, app, target).Scan(&required).Error)
				require.Equal(t, tc.wantRequired, required, "обязательность голоса изменена пересылкой")

				got := suppReadApplication(t, db, app)
				require.Equal(t, models.ConfirmationPending, *got.Confirmation,
					"согласование заявки сдвинуто пересылкой уже назначенного")

				var trail int64
				require.NoError(t, db.Raw(`SELECT COUNT(*) FROM audit_log
					WHERE entity_type = ? AND entity_id = ?`, models.AuditEntityApplication, app).Scan(&trail).Error)
				require.Zero(t, trail, "пересылка без новых получателей не должна оставлять записей в истории")
			})
		}
	}
}
