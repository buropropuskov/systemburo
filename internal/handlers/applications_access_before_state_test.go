package handlers_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplication_AccessBeforeBusinessState(t *testing.T) {
	w := newRoleWorld(t)
	selected := map[string]bool{"голос": true, "отзыв голоса": true, "принятие в работу": true,
		"заметка бюро": true}
	for _, action := range roleActions() {
		if !selected[action.name] && !strings.Contains(action.route, "/supplements/:sid/") && !strings.Contains(action.route, "/supplements/:supplementId/") && !strings.HasSuffix(action.route, "/forward") {
			continue
		}
		// Только изменённые методы раундов, создание/решение не входят в этот diff.
		if strings.Contains(action.route, "/supplements/") && !strings.HasSuffix(action.route, "/approve") && !strings.HasSuffix(action.route, "/revoke-approval") && !strings.HasSuffix(action.route, "/cancel") {
			continue
		}
		for _, archived := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/archive=%v", action.name, archived), func(t *testing.T) {
				f := w.seed(t, action.seed)
				status := models.StatusWithdrawn
				if archived {
					status = models.StatusCompleted
					past := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
					require.NoError(t, w.db.Table("attachments").Where("application_id = ?", f.app).Updates(map[string]any{"entry_date_from": past, "entry_date_to": past}).Error)
				}
				require.NoError(t, w.db.Table("applications").Where("id = ?", f.app).Update("status", status).Error)
				actor := roleStranger
				before := w.trace(t, f, w.ids[actor])
				method, _, _ := strings.Cut(action.route, " ")
				got := probeRoute(t, w.e, method, action.url(f), action.body(w, f, w.ids[actor]), w.tokens[actor])
				require.Equal(t, http.StatusForbidden, got.status, got.body)
				assert.NotContains(t, got.body, "Архивная")
				assert.NotContains(t, got.body, "отозвана")
				assert.Equal(t, before, w.trace(t, f, w.ids[actor]), "отказ не меняет данные и историю")
			})
		}
	}
}

func TestApplication_CompletedCannotBeAcceptedAgain(t *testing.T) {
	w := newRoleWorld(t)
	f := w.seed(t, withSeed(seedVoted, func(s *roleSeed) { s.status = models.StatusCompleted }))
	before := w.trace(t, f, w.ids[roleAccepter])
	got := probeRoute(t, w.e, http.MethodPost, roleURL("/take-to-work")(f), `{"action":"accept"}`, w.tokens[roleAccepter])
	require.Equal(t, http.StatusBadRequest, got.status, got.body)
	assert.Contains(t, got.body, "Завершённую")
	assert.Equal(t, before, w.trace(t, f, w.ids[roleAccepter]))
}

func TestSupplement_WrongApplicationBeforeArchive(t *testing.T) {
	w := newRoleWorld(t)
	own := w.seed(t, withSeed(seedVoted, func(s *roleSeed) { s.round = models.SupplementPending; s.roundVote = "pending" }))
	for _, archived := range []bool{false, true} {
		other := w.seed(t, seedVoted)
		if archived {
			past := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
			require.NoError(t, w.db.Table("applications").Where("id = ?", other.app).Update("status", models.StatusCompleted).Error)
			require.NoError(t, w.db.Table("attachments").Where("application_id = ?", other.app).Update("entry_date_to", past).Error)
		}
		before := w.trace(t, other, w.ids[roleVoter])
		for _, verb := range []string{"approve", "revoke-approval"} {
			got := probeRoute(t, w.e, http.MethodPost, fmt.Sprintf("/api/applications/%d/supplements/%d/%s", other.app, own.supplement, verb), `{"status":"approved"}`, w.tokens[roleVoter])
			require.Equal(t, http.StatusNotFound, got.status, got.body)
			assert.NotContains(t, got.body, "Архивная")
		}
		assert.Equal(t, before, w.trace(t, other, w.ids[roleVoter]))
	}
}

func TestApplication_StateGuardsUseTransactionConnection(t *testing.T) {
	w := newRoleWorld(t)
	sqlDB, err := w.db.DB()
	require.NoError(t, err)
	oldMax := sqlDB.Stats().MaxOpenConnections
	defer sqlDB.SetMaxOpenConns(oldMax)
	for _, action := range roleActions() {
		if action.name != "голос" && action.name != "отзыв голоса" && action.name != "голос по дополнению" && action.name != "отзыв голоса по дополнению" {
			continue
		}
		t.Run(action.name, func(t *testing.T) {
			f := w.seed(t, action.seed)
			past := time.Now().AddDate(-1, 0, 0).Format("2006-01-02")
			require.NoError(t, w.db.Table("applications").Where("id = ?", f.app).Update("status", models.StatusCompleted).Error)
			require.NoError(t, w.db.Table("attachments").Where("application_id = ?", f.app).Update("entry_date_to", past).Error)
			sqlDB.SetMaxOpenConns(1)
			defer sqlDB.SetMaxOpenConns(oldMax)
			method, _, _ := strings.Cut(action.route, " ")
			got := probeRoute(t, w.e, method, action.url(f), action.body(w, f, w.ids[roleVoter]), w.tokens[roleVoter])
			require.Equal(t, http.StatusForbidden, got.status, got.body)
			assert.Contains(t, got.body, "Архивная")
		})
	}
}
