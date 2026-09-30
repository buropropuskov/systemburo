package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestWorkflowActor_FromToken: голос, принятие в работу, отзыв из работы и возврат в
// работу делает тот, чей токен. Раньше все четыре требовали user_id в теле: голос с
// чужим id отвечал 403, остальные без поля - 400 и молча игнорировали значение. Теперь
// поле не читается: без него действие проходит, чужой id не делает автором другого.
func TestWorkflowActor_FromToken(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "at_author", "pass123", 1, td.OrgID, td.CompanyID)
	authorID := getUserID(t, db, "at_author")
	voterToken := testutil.RegisterAndLogin(t, e, "at_voter", "pass123", 1, td.OrgID, td.CompanyID)
	voterID := getUserID(t, db, "at_voter")
	testutil.RegisterUser(t, e, "at_other", "pass123", 1, td.OrgID, td.CompanyID)
	otherID := getUserID(t, db, "at_other")
	accepterToken := testutil.RegisterAndLogin(t, e, "at_accepter", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "at_accepter")
	accepterID := getUserID(t, db, "at_accepter")

	bodies := map[string]string{
		"без user_id":   `{%s}`,
		"чужой user_id": fmt.Sprintf(`{"user_id":%d,%%s}`, otherID),
	}
	n := 0
	newApp := func(t *testing.T, confirmation, status string) int {
		n++
		app := suppApp(t, db, td.OrgID, authorID, fmt.Sprintf("AT-%d", n), confirmation, status)
		suppAttachment(t, db, app, "cars", "2099-12-31")
		return app
	}

	for name, tmpl := range bodies {
		t.Run("голос/"+name, func(t *testing.T) {
			app := newApp(t, models.ConfirmationPending, models.StatusProcessing)
			suppResponsible(t, db, app, voterID, true, "pending")
			suppResponsible(t, db, app, otherID, true, "pending")

			rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", app),
				fmt.Sprintf(tmpl, `"status":"approved"`), testutil.AuthHeader(voterToken))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, "approved", voteOf(t, db, app, voterID), "голос ложится на владельца токена")
			require.Equal(t, "pending", voteOf(t, db, app, otherID), "id из тела не голосует за другого")
		})

		t.Run("принятие в работу/"+name, func(t *testing.T) {
			app := newApp(t, models.ConfirmationApproved, models.StatusProcessing)
			rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", app),
				fmt.Sprintf(tmpl, `"action":"accept"`), testutil.AuthHeader(accepterToken))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			got := suppReadApplication(t, db, app)
			require.Equal(t, models.StatusInWork, *got.Status)
			require.NotNil(t, got.ResponsibleUserID)
			require.Equal(t, accepterID, *got.ResponsibleUserID)
			require.Equal(t, accepterID, lastActor(t, db, app, models.AuditActionTakeToWork))
		})

		for route, from := range map[string]string{
			"revoke-from-work": models.StatusInWork,
			"restore-to-work":  models.StatusRefused,
		} {
			action := map[string]string{"revoke-from-work": "revoke_from_work", "restore-to-work": "restore_to_work"}[route]
			t.Run(route+"/"+name, func(t *testing.T) {
				app := newApp(t, models.ConfirmationApproved, from)
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/%s", app, route),
					fmt.Sprintf(tmpl, `"comment":"проверка"`), testutil.AuthHeader(accepterToken))
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				require.Equal(t, models.StatusProcessing, *suppReadApplication(t, db, app).Status)
				require.Equal(t, accepterID, lastActor(t, db, app, action))
			})
		}
	}
}

func voteOf(t *testing.T, db *gorm.DB, appID, userID int) string {
	t.Helper()
	var status string
	require.NoError(t, db.Raw(`SELECT approval_status FROM application_responsible_users
		WHERE application_id = ? AND user_id = ?`, appID, userID).Scan(&status).Error)
	return status
}

func lastActor(t *testing.T, db *gorm.DB, appID int, action string) int {
	t.Helper()
	var actor *int
	require.NoError(t, db.Raw(`SELECT actor_user_id FROM audit_log
		WHERE entity_type = ? AND entity_id = ? AND action = ? ORDER BY id DESC LIMIT 1`,
		models.AuditEntityApplication, appID, action).Scan(&actor).Error)
	require.NotNil(t, actor, "в журнале нет записи %s", action)
	return *actor
}
