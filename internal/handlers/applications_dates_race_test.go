package handlers_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// waitLockWaiters ждёт, пока столько запросов к заявкам встанет в очередь за блокировкой.
// Так порядок входа в гонку задаётся тестом, а не планировщиком горутин; фильтр по таблице
// отсекает ожидания других DB-пакетов, если они делят тест-БД.
func waitLockWaiters(t *testing.T, db *gorm.DB, want int) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		var waiting int
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock' AND query ~* '\mapplications\M'`).Scan(&waiting).Error)
		if waiting >= want {
			return
		}
		require.True(t, time.Now().Before(deadline), "в очереди за блокировкой %d из %d запросов", waiting, want)
		time.Sleep(10 * time.Millisecond)
	}
}

// holdApplicationRow держит строку заявки, пока тест выстраивает очередь. Откат в Cleanup
// нужен на случай падения до отпускания: иначе ждущие запросы вешают весь пакет.
func holdApplicationRow(t *testing.T, db *gorm.DB, appID int) *gorm.DB {
	t.Helper()
	holder := db.Begin()
	require.NoError(t, holder.Error)
	t.Cleanup(func() { holder.Rollback() })
	require.NoError(t, holder.Exec("SELECT id FROM applications WHERE id = ? FOR UPDATE", appID).Error)
	return holder
}

func auditID(t *testing.T, db *gorm.DB, appID int, action string) int {
	t.Helper()
	var id int
	require.NoError(t, db.Raw(`SELECT COALESCE(MAX(id), 0) FROM audit_log
		WHERE entity_type = ? AND entity_id = ? AND action = ?`, models.AuditEntityApplication, appID, action).
		Scan(&id).Error)
	return id
}

// Гонка голоса с правкой срока (#2575). Принимающий правит срок, а согласующий в тот же
// момент голосует по старому окну. Правка встаёт в очередь за строкой заявки первой, голос
// вторым. Голос, записанный до правки, обязан быть сброшен ею: согласующий одобрял другое
// окно. Если кто-то уже голосовал, правка сбрасывает раунд - и встречный порядок
// блокировок у голоса давал дедлок, один из запросов падал с 500.
func TestChangeDates_RaceWithVote(t *testing.T) {
	cases := []struct {
		name      string
		priorVote bool
	}{
		{"голосов ещё нет", false},
		{"другой согласующий уже проголосовал", true},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, db, cleanup := testutil.SetupTestApp(t)
			defer cleanup()
			testutil.CleanDB(t, db)
			td := testutil.SeedTestData(t, db)
			token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
			makeApprover(t, db, "testadmin")

			senderID := seedAttachSender(t, db, td.OrgID)
			voterID, voterToken := suppVoteUser(t, e, db, fmt.Sprintf("race_voter_%d", i), td.OrgID, td.CompanyID)
			pending := models.ConfirmationPending
			appID, attID, _ := seedDatesApp(t, db, td.OrgID, senderID, models.StatusProcessing, &pending, fmt.Sprintf("R%d00RR777", i))
			addResponsible(t, db, appID, voterID, "pending")
			if tc.priorVote {
				otherID := seedDatesUser(t, db, fmt.Sprintf("race_other_%d", i), td.OrgID)
				addResponsible(t, db, appID, otherID, "approved")
			}

			holder := holdApplicationRow(t, db, appID)

			var wg sync.WaitGroup
			var datesCode, voteCode int
			var datesBodyText, voteBodyText string
			wg.Add(1)
			go func() {
				defer wg.Done()
				rec := testutil.PUT(t, e, datesPath(appID),
					datesBody("2099-04-01", "2099-04-02", "09:00", "18:00", "перенос"), testutil.AuthHeader(token))
				datesCode, datesBodyText = rec.Code, rec.Body.String()
			}()
			waitLockWaiters(t, db, 1)

			wg.Add(1)
			go func() {
				defer wg.Done()
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID),
					fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, voterID), testutil.AuthHeader(voterToken))
				voteCode, voteBodyText = rec.Code, rec.Body.String()
			}()
			waitLockWaiters(t, db, 2)

			require.NoError(t, holder.Rollback().Error)
			wg.Wait()

			require.Equal(t, http.StatusOK, datesCode, "правка срока: %s", datesBodyText)
			require.Equal(t, http.StatusOK, voteCode, "голос: %s", voteBodyText)
			assert.Equal(t, "2099-04-01", periodOf(t, db, "attachments", attID).EntryDateFrom, "новое окно легло на вложение")

			var vote string
			require.NoError(t, db.Raw("SELECT approval_status FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
				appID, voterID).Scan(&vote).Error)
			if vote == "approved" {
				assert.Greater(t, auditID(t, db, appID, "approve"), auditID(t, db, appID, models.AuditActionDatesChanged),
					"голос, поданный до правки срока, пережил её: согласующий одобрил окно, которого больше нет")
			}
		})
	}
}

// Отзыв голоса в гонке с правкой срока: правка сбрасывает голос первой, отзыв приходит
// следом. Прочитав свой голос до блокировки заявки, отзыв видел уже снятое «согласовано»
// и писал в историю заявки отзыв голоса, которого к тому моменту не было.
func TestChangeDates_RaceWithRevokeApproval(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	senderID := seedAttachSender(t, db, td.OrgID)
	voterID, voterToken := suppVoteUser(t, e, db, "race_revoker", td.OrgID, td.CompanyID)
	pending := models.ConfirmationPending
	appID, _, _ := seedDatesApp(t, db, td.OrgID, senderID, models.StatusProcessing, &pending, "R900RR777")
	addResponsible(t, db, appID, voterID, "approved")

	holder := holdApplicationRow(t, db, appID)

	var wg sync.WaitGroup
	var datesCode, revokeCode int
	var datesBodyText string
	wg.Add(1)
	go func() {
		defer wg.Done()
		rec := testutil.PUT(t, e, datesPath(appID),
			datesBody("2099-05-01", "2099-05-02", "09:00", "18:00", "перенос"), testutil.AuthHeader(token))
		datesCode, datesBodyText = rec.Code, rec.Body.String()
	}()
	waitLockWaiters(t, db, 1)

	wg.Add(1)
	go func() {
		defer wg.Done()
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-approval", appID), `{}`, testutil.AuthHeader(voterToken))
		revokeCode = rec.Code
	}()
	waitLockWaiters(t, db, 2)

	require.NoError(t, holder.Rollback().Error)
	wg.Wait()

	require.Equal(t, http.StatusOK, datesCode, "правка срока: %s", datesBodyText)
	assert.Contains(t, datesBodyText, `"approvals_reset":true`, "правка застала голос и сбросила его")
	assert.Equal(t, http.StatusBadRequest, revokeCode, "после сброса отзывать нечего")
	assert.Zero(t, auditID(t, db, appID, "revoke_approval"), "отзыв снятого голоса не попадает в историю заявки")
}
