package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"
)

// roleAction - действие внутри заявки: кому оно положено и в каком состоянии заявки
// законный запрос проходит. Остальные роли получают на том же запросе 403.
type roleAction struct {
	name    string
	route   string // ключ реестра доступа
	seed    roleSeed
	allowed []roleActor
	url     func(f roleApp) string
	body    func(w *roleWorld, f roleApp, sender int) string
}

// roleNotInMatrix - изменяющие методы заявки вне матрицы и почему.
var roleNotInMatrix = map[string]string{
	"POST /api/applications/:id/read":                       "отметка прочтения своей строкой, чужой организации - замок IDOR",
	"POST /api/applications/:id/questions/seen":             "отметка просмотра своей строкой, чужой организации - замок IDOR",
	"POST /api/applications/:id/questions/:questionId/read": "отметка прочтения своей строкой, чужой организации - замок IDOR",
	"DELETE /api/applications/:id/files/:file_id":           "под правом page.admin, роль в заявке не участвует",
}

var (
	seedVoting = roleSeed{status: models.StatusProcessing, confirmation: models.ConfirmationPending, vote: "pending"}
	seedVoted  = roleSeed{status: models.StatusProcessing, confirmation: models.ConfirmationApproved, vote: "approved"}
	seedInWork = roleSeed{status: models.StatusInWork, confirmation: models.ConfirmationApproved, vote: "approved"}
)

func withSeed(base roleSeed, edit func(*roleSeed)) roleSeed {
	edit(&base)
	return base
}

func roleURL(suffix string) func(f roleApp) string {
	return func(f roleApp) string { return fmt.Sprintf("/api/applications/%d%s", f.app, suffix) }
}

func roleRoundURL(verb string) func(f roleApp) string {
	return func(f roleApp) string {
		return fmt.Sprintf("/api/applications/%d/supplements/%d/%s", f.app, f.supplement, verb)
	}
}

func roleBody(s string) func(*roleWorld, roleApp, int) string {
	return func(*roleWorld, roleApp, int) string { return s }
}

// participants - все, кому заявка открыта (CanAccessApplication).
var participants = []roleActor{roleAuthor, roleVoter, roleResponsible, roleReader, roleAccepter}

func roleActions() []roleAction {
	voters := []roleActor{roleVoter, roleResponsible}
	accepter := []roleActor{roleAccepter}
	forward := func(recipient func(w *roleWorld) int, required, canView bool) func(*roleWorld, roleApp, int) string {
		return func(w *roleWorld, _ roleApp, _ int) string {
			return forwardBody(recipient(w), required, canView)
		}
	}
	newcomer := func(w *roleWorld) int { return w.newcomer }
	flagBody := func(_ *roleWorld, f roleApp, _ int) string {
		return fmt.Sprintf(`{"flag_id":%d,"comment":"замок ролей"}`, f.flag)
	}
	roundPending := withSeed(seedVoted, func(s *roleSeed) { s.round, s.roundVote = models.SupplementPending, "pending" })

	return []roleAction{
		{name: "голос", route: "POST /api/applications/:id/approve", seed: seedVoting, allowed: voters,
			url: roleURL("/approve"),
			body: func(_ *roleWorld, _ roleApp, uid int) string {
				return fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, uid)
			}},
		{name: "отзыв голоса", route: "POST /api/applications/:id/revoke-approval", seed: seedVoted, allowed: voters,
			url: roleURL("/revoke-approval"), body: roleBody(`{"comment":"замок ролей"}`)},
		{name: "подтверждение пропуска ЧС", route: "POST /api/applications/:id/blacklist-overrides",
			seed: withSeed(seedVoting, func(s *roleSeed) { s.flag = true }), allowed: append(slices.Clone(voters), roleAccepter),
			url: roleURL("/blacklist-overrides"), body: flagBody},
		{name: "отмена пропуска ЧС", route: "DELETE /api/applications/:id/blacklist-overrides",
			seed:    withSeed(seedVoting, func(s *roleSeed) { s.flag, s.overridden = true, true }),
			allowed: append(slices.Clone(voters), roleAccepter),
			url: func(f roleApp) string {
				return fmt.Sprintf("/api/applications/%d/blacklist-overrides?flag_id=%d", f.app, f.flag)
			}},

		{name: "принятие в работу", route: "POST /api/applications/:id/take-to-work", seed: seedVoted, allowed: accepter,
			url: roleURL("/take-to-work"), body: roleBody(`{"user_id":1,"action":"accept"}`)},
		{name: "отзыв из работы", route: "POST /api/applications/:id/revoke-from-work", seed: seedInWork, allowed: accepter,
			url: roleURL("/revoke-from-work"), body: roleBody(`{"user_id":1,"comment":"замок ролей"}`)},
		{name: "возврат в обработку", route: "POST /api/applications/:id/restore-to-work",
			seed:    withSeed(seedVoted, func(s *roleSeed) { s.status = models.StatusRefused }),
			allowed: accepter, url: roleURL("/restore-to-work"), body: roleBody(`{"user_id":1,"comment":"замок ролей"}`)},
		{name: "срок", route: "PUT /api/applications/:id/dates", seed: seedVoting, allowed: accepter,
			url: roleURL("/dates"),
			body: roleBody(`{"entry_date_from":"2099-10-01","entry_date_to":"2099-10-02",` +
				`"entry_time_from":"08:00","entry_time_to":"20:00","reason":"замок ролей"}`)},
		{name: "посты элементов", route: "PUT /api/applications/:id/elements/tables", seed: seedVoting, allowed: accepter,
			url: roleURL("/elements/tables"),
			body: func(_ *roleWorld, f roleApp, _ int) string {
				return fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"table_ids":[],"mode":"replace"}`, f.car)
			}},
		{name: "места разгрузки", route: "PUT /api/applications/:id/elements/unload-places", seed: seedVoting, allowed: accepter,
			url: roleURL("/elements/unload-places"),
			body: func(_ *roleWorld, f roleApp, _ int) string {
				return fmt.Sprintf(`{"car_ids":[%d],"place_ids":[],"mode":"replace"}`, f.car)
			}},
		{name: "удаление элемента", route: "DELETE /api/applications/:id/elements", seed: seedVoting, allowed: accepter,
			url: roleURL("/elements"),
			body: func(_ *roleWorld, f roleApp, _ int) string {
				return fmt.Sprintf(`{"element_type":"cars","element_ids":[%d],"reason":"замок ролей"}`, f.car)
			}},
		{name: "заметка бюро", route: "PUT /api/applications/:id/bureau-note", seed: seedVoting, allowed: accepter,
			url: roleURL("/bureau-note"), body: roleBody(`{"note":"замок ролей"}`)},

		{name: "отзыв заявки", route: "POST /api/applications/:id/withdraw", seed: seedVoting,
			allowed: []roleActor{roleAuthor}, url: roleURL("/withdraw")},
		{name: "пересылка на просмотр", route: "POST /api/applications/:id/forward", seed: seedVoting,
			allowed: participants, url: roleURL("/forward"), body: forward(newcomer, false, true)},
		{name: "пересылка на согласование", route: "POST /api/applications/:id/forward", seed: seedVoting,
			allowed: []roleActor{roleAuthor, roleVoter, roleResponsible, roleAccepter},
			url:     roleURL("/forward"), body: forward(newcomer, true, false)},
		{name: "вопрос к заявке", route: "POST /api/applications/:id/questions", seed: seedVoting,
			allowed: participants, url: roleURL("/questions"), body: roleBody(`{"subject":"Тема","text":"Текст"}`)},
		{name: "ответ на вопрос", route: "POST /api/applications/:id/questions/:questionId/answers", seed: seedVoting,
			allowed: participants, body: roleBody(`{"text":"Ответ"}`),
			url: func(f roleApp) string {
				return fmt.Sprintf("/api/applications/%d/questions/%d/answers", f.app, f.question)
			}},

		{name: "подача дополнения", route: "POST /api/applications/:id/supplements", seed: seedVoted,
			allowed: []roleActor{roleAuthor}, url: roleURL("/supplements"),
			body: func(_ *roleWorld, f roleApp, _ int) string {
				return fmt.Sprintf(`{"additions":[{"attachment_id":%d,"items":[{"name":"Ящик","count":1,"order_index":1}]}]}`, f.itemsAtt)
			}},
		{name: "голос по дополнению", route: "POST /api/applications/:id/supplements/:sid/approve", seed: roundPending,
			allowed: voters, url: roleRoundURL("approve"), body: roleBody(`{"status":"approved"}`)},
		{name: "отзыв голоса по дополнению", route: "POST /api/applications/:id/supplements/:sid/revoke-approval",
			seed:    withSeed(roundPending, func(s *roleSeed) { s.roundVote = "approved" }),
			allowed: voters, url: roleRoundURL("revoke-approval"), body: roleBody(`{}`)},
		{name: "решение по дополнению", route: "POST /api/applications/:id/supplements/:sid/take-to-work",
			seed:    withSeed(roundPending, func(s *roleSeed) { s.round, s.roundVote = models.SupplementApproved, "approved" }),
			allowed: accepter, url: roleRoundURL("take-to-work"), body: roleBody(`{"action":"reject","comment":"замок ролей"}`)},
		{name: "снятие дополнения", route: "POST /api/applications/:id/supplements/:sid/cancel", seed: roundPending,
			allowed: []roleActor{roleAuthor}, url: roleRoundURL("cancel"), body: roleBody(`{}`)},
	}
}

// TestRouteAccess_ApplicationRoles - матрица ролей внутри заявки. Каждое изменяющее
// действие заявки шлют все шесть ролей, каждая на свежую заявку в состоянии, где
// законный запрос проходит: роль из allowed получает 2xx, остальные - 403 и не оставляют
// следа ни в заявке, ни в журнале. Полнота сверяется с реестром доступа: изменяющий
// метод заявки либо в матрице, либо в roleNotInMatrix с причиной.
func TestRouteAccess_ApplicationRoles(t *testing.T) {
	actions := roleActions()
	checkRoleMatrixCompleteness(t, actions)

	w := newRoleWorld(t)
	for _, a := range actions {
		method, _, _ := strings.Cut(a.route, " ")
		for _, actor := range roleActors {
			f := w.seed(t, a.seed)
			body := ""
			if a.body != nil {
				body = a.body(w, f, w.ids[actor])
			}
			before := w.trace(t, f, w.ids[actor])
			got := probeRoute(t, w.e, method, a.url(f), body, w.tokens[actor])
			if slices.Contains(a.allowed, actor) {
				if got.status < 200 || got.status > 299 {
					t.Errorf("%s: %s вправе, но получил %d: %s", a.name, actor, got.status, got.body)
				}
				continue
			}
			if got.status != http.StatusForbidden {
				t.Errorf("%s: %s не вправе, ждали 403, получили %d: %s", a.name, actor, got.status, got.body)
			}
			if after := w.trace(t, f, w.ids[actor]); after != before {
				t.Errorf("%s: отказ роли %s оставил след:\nбыло  %s\nстало %s", a.name, actor, before, after)
			}
		}
	}
}

// TestRouteAccess_BureauNoteReadOnlyAccepter - заметку бюро в детали заявки видит только
// принимающий: у остальных участников ключа нет вовсе, посторонний заявку не открывает.
func TestRouteAccess_BureauNoteReadOnlyAccepter(t *testing.T) {
	w := newRoleWorld(t)
	f := w.seed(t, seedVoting)
	note := "замок ролей"
	if err := w.db.Model(&models.Application{}).Where("id = ?", f.app).Update("bureau_note", note).Error; err != nil {
		t.Fatalf("заметка: %v", err)
	}
	for _, actor := range roleActors {
		got := probeRoute(t, w.e, http.MethodGet, fmt.Sprintf("/api/applications/%d/details", f.app), "", w.tokens[actor])
		if actor == roleStranger {
			if got.status != http.StatusForbidden {
				t.Errorf("деталь заявки: посторонний получил %d вместо 403", got.status)
			}
			continue
		}
		if got.status != http.StatusOK {
			t.Errorf("деталь заявки: %s получил %d: %s", actor, got.status, got.body)
			continue
		}
		rec := testutil.GET(t, w.e, fmt.Sprintf("/applications/%d/details", f.app), testutil.AuthHeader(w.tokens[actor]))
		var detail struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
			t.Fatalf("деталь заявки: %v", err)
		}
		raw, has := detail.Data["bureau_note"]
		switch {
		case actor == roleAccepter && !strings.Contains(string(raw), note):
			t.Errorf("принимающий не видит заметку бюро: %s", raw)
		case actor != roleAccepter && has:
			t.Errorf("%s видит ключ bureau_note в детали заявки: %s", actor, raw)
		}
	}
}

// checkRoleMatrixCompleteness - каждый изменяющий метод заявки из реестра разобран, и
// каждое действие матрицы указывает на метод реестра с совпадающим адресом.
func checkRoleMatrixCompleteness(t *testing.T, actions []roleAction) {
	t.Helper()
	inMatrix := map[string]bool{}
	sample := roleApp{app: 1, carsAtt: 2, itemsAtt: 3, car: 4, question: 5, flag: 6, supplement: 7}
	for _, a := range actions {
		inMatrix[a.route] = true
		if _, ok := routeAccessRegistry[a.route]; !ok {
			t.Errorf("%s (%s) нет в реестре доступа", a.route, a.name)
			continue
		}
		_, tmpl, _ := strings.Cut(a.route, " ")
		if !pathMatches(tmpl, a.url(sample)) {
			t.Errorf("%s: адрес %s не совпадает с шаблоном", a.name, a.url(sample))
		}
	}
	for key := range routeAccessRegistry {
		method, path, _ := strings.Cut(key, " ")
		if method == http.MethodGet || !strings.HasPrefix(path, "/api/applications/:id") {
			continue
		}
		if _, skipped := roleNotInMatrix[key]; !inMatrix[key] && !skipped {
			t.Errorf("%s меняет заявку, но не разобран: добавь действие в roleActions "+
				"или причину в roleNotInMatrix", key)
		}
	}
	for key := range roleNotInMatrix {
		if _, ok := routeAccessRegistry[key]; !ok {
			t.Errorf("%s есть в roleNotInMatrix, но не в реестре", key)
		}
	}
}
