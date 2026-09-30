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

// TestReturnToProcessing_OnlyFromOwnStatus: принимающий возвращает заявку в обработку
// только из одного исходного статуса - отзыв из работы из "В работе", возврат из
// "Отказано". Раньше исходный статус не сверялся: завершённую или несогласованную заявку
// можно было вернуть в обработку и принять заново, а архивную - вытащить из архива.
func TestReturnToProcessing_OnlyFromOwnStatus(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "sg_author", "pass123", 1, td.OrgID, td.CompanyID)
	authorID := getUserID(t, db, "sg_author")
	accepterToken := testutil.RegisterAndLogin(t, e, "sg_accepter", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "sg_accepter")
	accepterID := getUserID(t, db, "sg_accepter")
	body := fmt.Sprintf(`{"user_id":%d}`, accepterID)

	cases := []struct {
		route    string
		from     string
		archived bool
		want     int
	}{
		{"revoke-from-work", models.StatusInWork, false, http.StatusOK},
		{"revoke-from-work", models.StatusUnread, false, http.StatusConflict},
		{"revoke-from-work", models.StatusProcessing, false, http.StatusConflict},
		{"revoke-from-work", models.StatusRefused, false, http.StatusConflict},
		{"revoke-from-work", models.StatusCompleted, false, http.StatusConflict},
		{"revoke-from-work", models.StatusRejected, false, http.StatusConflict},
		{"restore-to-work", models.StatusRefused, false, http.StatusOK},
		{"restore-to-work", models.StatusRefused, true, http.StatusForbidden},
		{"restore-to-work", models.StatusInWork, false, http.StatusConflict},
		{"restore-to-work", models.StatusProcessing, false, http.StatusConflict},
		{"restore-to-work", models.StatusCompleted, false, http.StatusConflict},
		{"restore-to-work", models.StatusRejected, false, http.StatusConflict},
	}
	for i, tc := range cases {
		name := fmt.Sprintf("%s/из %s", tc.route, tc.from)
		if tc.archived {
			name += " в архиве"
		}
		t.Run(name, func(t *testing.T) {
			app := suppApp(t, db, td.OrgID, authorID, fmt.Sprintf("SG-%d", i),
				models.ConfirmationApproved, tc.from)
			dateTo := "2099-12-31"
			if tc.archived {
				dateTo = "2025-01-01"
			}
			suppAttachment(t, db, app, "cars", dateTo)

			rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/%s", app, tc.route),
				body, testutil.AuthHeader(accepterToken))
			require.Equal(t, tc.want, rec.Code, rec.Body.String())

			got := suppReadApplication(t, db, app)
			if tc.want == http.StatusOK {
				require.Equal(t, models.StatusProcessing, *got.Status)
				return
			}
			require.Equal(t, tc.from, *got.Status, "отказ не должен двигать статус")
			require.Zero(t, applicationTrail(t, db, app), "отказ не должен оставлять записей в истории")
		})
	}
}

// TestApprovalOutcome_FrozenInWork: у заявки в работе итог согласования зафиксирован.
// Поздний голос необязательного согласующего, пересылка нового обязательного и отзыв
// своего голоса раньше пересчитывали confirmation и роняли его с "Согласовано", а с ним
// снимали с КПП уже допущенных. Контроль: на заявке в обработке отзыв голоса пересчитывает.
func TestApprovalOutcome_FrozenInWork(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorToken := testutil.RegisterAndLogin(t, e, "fz_author", "pass123", 1, td.OrgID, td.CompanyID)
	authorID := getUserID(t, db, "fz_author")
	voterToken := testutil.RegisterAndLogin(t, e, "fz_voter", "pass123", 1, td.OrgID, td.CompanyID)
	voterID := getUserID(t, db, "fz_voter")
	lateToken := testutil.RegisterAndLogin(t, e, "fz_late", "pass123", 1, td.OrgID, td.CompanyID)
	lateID := getUserID(t, db, "fz_late")
	testutil.RegisterUser(t, e, "fz_new", "pass123", 1, td.OrgID, td.CompanyID)
	newID := getUserID(t, db, "fz_new")

	newApp := func(t *testing.T, number, status string) int {
		app := suppApp(t, db, td.OrgID, authorID, number, models.ConfirmationApproved, status)
		suppResponsible(t, db, app, voterID, true, "approved")
		suppResponsible(t, db, app, lateID, false, "pending")
		return app
	}

	// Без обязательных кворум решают необязательные, и один отказ хоронит круг.
	t.Run("поздний отказ необязательного", func(t *testing.T) {
		app := suppApp(t, db, td.OrgID, authorID, "FZ-1", models.ConfirmationApproved, models.StatusInWork)
		suppResponsible(t, db, app, voterID, false, "approved")
		suppResponsible(t, db, app, lateID, false, "pending")
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", app),
			fmt.Sprintf(`{"user_id":%d,"status":"rejected"}`, lateID), testutil.AuthHeader(lateToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, models.ConfirmationApproved, *suppReadApplication(t, db, app).Confirmation)
	})

	t.Run("пересылка нового обязательного", func(t *testing.T) {
		app := newApp(t, "FZ-2", models.StatusInWork)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", app),
			forwardBody(newID, true, false), testutil.AuthHeader(authorToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var added int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM application_responsible_users
			WHERE application_id = ? AND user_id = ?`, app, newID).Scan(&added).Error)
		require.EqualValues(t, 1, added, "пересылка должна назначить получателя")
		require.Equal(t, models.ConfirmationApproved, *suppReadApplication(t, db, app).Confirmation)
	})

	t.Run("отзыв своего голоса", func(t *testing.T) {
		app := newApp(t, "FZ-3", models.StatusInWork)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-approval", app),
			`{}`, testutil.AuthHeader(voterToken))
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())

		var vote string
		require.NoError(t, db.Raw(`SELECT approval_status FROM application_responsible_users
			WHERE application_id = ? AND user_id = ?`, app, voterID).Scan(&vote).Error)
		require.Equal(t, "approved", vote)
		require.Equal(t, models.ConfirmationApproved, *suppReadApplication(t, db, app).Confirmation)
		require.Zero(t, applicationTrail(t, db, app))
	})

	t.Run("контроль: отзыв голоса до принятия пересчитывает", func(t *testing.T) {
		app := newApp(t, "FZ-4", models.StatusProcessing)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-approval", app),
			`{}`, testutil.AuthHeader(voterToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, models.ConfirmationPending, *suppReadApplication(t, db, app).Confirmation)
	})
}

// applicationTrail - число записей истории заявки.
func applicationTrail(t *testing.T, db *gorm.DB, appID int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM audit_log WHERE entity_type = ? AND entity_id = ?`,
		models.AuditEntityApplication, appID).Scan(&n).Error)
	return n
}
