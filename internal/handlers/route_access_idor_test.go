package handlers_test

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"systemburo/internal/models"
)

// idorProbe - запрос к объекту организации A. Законный участник actor получает 2xx,
// чужак на тот же адрес с тем же телом - 403 или 404. Контроль у участника доказывает,
// что отказ чужаку дан по владению, а не потому что объекта нет или тело невалидно.
type idorProbe struct {
	actor idorActor
	// noRound - заявка без открытого раунда дополнения (подать новое при открытом нельзя).
	noRound bool
	url     func(f idorFixture) string
	// body получает id отправителя: голос принимается только за себя, и чужак должен
	// упереться в состав согласующих, а не в «голос не за себя».
	body func(f idorFixture, senderID int) string
}

// idorSwap - чужак идёт в СВОЮ заявку с дочерним объектом заявки A. Своя заявка ему
// открыта, поэтому отказ обязан прийти из сверки дочернего объекта с заявкой.
type idorSwap struct {
	method string
	url    func(f idorFixture) string
	body   func(f idorFixture) string
}

// idorCoveredElsewhere - методы с id объекта, которые замок не шлёт сам: либо чужак
// проверен отдельным тестом (имя теста сверяется с исходниками пакета), либо объект
// не принадлежит организации.
var idorCoveredElsewhere = map[string]string{
	"GET /api/applications/:id/archive": "TestFileArchiveDownload_Application: посторонний 404, отправитель 200; " +
		"нужен слепок архива на диске",
	"GET /api/uploads*": "TestApplicationScans_StaticGate: чужой скан и несуществующий отвечают одинаково 404",
	"GET /api/documents/:id/download": "документ бюро общий для всех организаций, скрытый - только " +
		"с page.admin.directories (#2621)",
	"GET /api/guide/sections/:role/download": ":role - раздел руководства, доступ по праву guide.<роль>",
	"DELETE /api/consents/:type":             ":type - вид согласия, запись берётся по user_id из токена",
	"GET /api/consents/check/:type":          ":type - вид согласия, запись берётся по user_id из токена",
}

func appURL(suffix string) func(f idorFixture) string {
	return func(f idorFixture) string { return fmt.Sprintf("/api/applications/%d%s", f.app, suffix) }
}

func constBody(s string) func(idorFixture, int) string {
	return func(idorFixture, int) string { return s }
}

// idorProbes - по запросу на каждый метод с id объекта организации.
func idorProbes() map[string]idorProbe {
	owner := func(suffix string) idorProbe { return idorProbe{actor: idorOwner, url: appURL(suffix)} }
	ownerBody := func(suffix, body string) idorProbe {
		return idorProbe{actor: idorOwner, url: appURL(suffix), body: constBody(body)}
	}
	sup := func(verb string) func(f idorFixture) string {
		return func(f idorFixture) string {
			return fmt.Sprintf("/api/applications/%d/supplements/%d/%s", f.app, f.supplement, verb)
		}
	}
	q := func(verb string) func(f idorFixture) string {
		return func(f idorFixture) string {
			return fmt.Sprintf("/api/applications/%d/questions/%d/%s", f.app, f.question, verb)
		}
	}
	att := func(kind string) idorProbe {
		return idorProbe{actor: idorOwner, url: func(f idorFixture) string {
			id := map[string]int{"cars": f.carsAtt, "employees": f.peopleAtt, "items": f.itemsAtt}[kind]
			return fmt.Sprintf("/api/attachments/%d/%s", id, kind)
		}}
	}
	reg := func(format string, id func(f idorFixture) int, body string) idorProbe {
		return idorProbe{actor: idorOwner, body: constBody(body),
			url: func(f idorFixture) string { return fmt.Sprintf(format, id(f)) }}
	}
	uCar := func(f idorFixture) int { return f.uniqueCar }
	uEmp := func(f idorFixture) int { return f.uniqueEmployee }
	note := func(f idorFixture) int { return f.notification }

	return map[string]idorProbe{
		"GET /api/applications/:id":                       owner(""),
		"PUT /api/applications/:id":                       ownerBody("", `{"responsible_comment":"замок"}`),
		"GET /api/applications/:id/attachments":           owner("/attachments"),
		"GET /api/applications/:id/check-approval-status": owner("/check-approval-status"),
		"GET /api/applications/:id/details":               owner("/details"),
		"GET /api/applications/:id/files":                 owner("/files"),
		"GET /api/applications/:id/forward-messages":      owner("/forward-messages"),
		"GET /api/applications/:id/history":               owner("/history"),
		"GET /api/applications/:id/participants":          owner("/participants"),
		"GET /api/applications/:id/questions":             owner("/questions"),
		"GET /api/applications/:id/reads":                 owner("/reads"),
		"GET /api/applications/:id/responsible-users":     owner("/responsible-users"),
		"GET /api/applications/:id/supplements":           owner("/supplements"),
		"GET /api/applications/:id/viewers":               owner("/viewers"),
		"POST /api/applications/:id/read":                 owner("/read"),
		"POST /api/applications/:id/questions/seen":       owner("/questions/seen"),
		"POST /api/applications/:id/withdraw":             owner("/withdraw"),
		"POST /api/applications/:id/forward":              ownerBody("/forward", `{"users":[]}`),
		"POST /api/applications/:id/questions":            ownerBody("/questions", `{"subject":"Тема","text":"Текст"}`),
		"GET /api/applications/:id/files/:file_id": {actor: idorOwner, url: func(f idorFixture) string {
			return fmt.Sprintf("/api/applications/%d/files/%d", f.app, f.file)
		}},
		"GET /api/applications/:id/blank": {actor: idorOwner, url: func(f idorFixture) string {
			return fmt.Sprintf("/api/applications/%d/blank?attachment_id=%d", f.blankApp, f.blankAtt)
		}},
		"POST /api/applications/:id/questions/:questionId/answers": {actor: idorOwner, url: q("answers"), body: constBody(`{"text":"Ответ"}`)},
		"POST /api/applications/:id/questions/:questionId/read":    {actor: idorOwner, url: q("read")},

		"POST /api/applications/:id/approve": {actor: idorVoter, url: appURL("/approve"),
			body: func(_ idorFixture, uid int) string { return fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, uid) }},
		"POST /api/applications/:id/revoke-approval": {actor: idorVoted, url: appURL("/revoke-approval"), body: constBody(`{}`)},
		"POST /api/applications/:id/blacklist-overrides": {actor: idorVoter, url: appURL("/blacklist-overrides"),
			body: func(f idorFixture, _ int) string { return fmt.Sprintf(`{"flag_id":%d,"comment":"замок"}`, f.flag) }},
		"DELETE /api/applications/:id/blacklist-overrides": {actor: idorVoter, url: func(f idorFixture) string {
			return fmt.Sprintf("/api/applications/%d/blacklist-overrides?flag_id=%d", f.app, f.flag)
		}},
		"POST /api/applications/:id/supplements": {actor: idorOwner, noRound: true, url: appURL("/supplements"),
			body: func(f idorFixture, _ int) string {
				return fmt.Sprintf(`{"additions":[{"attachment_id":%d,"items":[{"name":"Ящик","count":1,"order_index":1}]}]}`, f.itemsAtt)
			}},
		"POST /api/applications/:id/supplements/:sid/approve":         {actor: idorVoter, url: sup("approve"), body: constBody(`{"status":"approved"}`)},
		"POST /api/applications/:id/supplements/:sid/revoke-approval": {actor: idorVoted, url: sup("revoke-approval"), body: constBody(`{}`)},
		"POST /api/applications/:id/supplements/:sid/cancel":          {actor: idorOwner, url: sup("cancel"), body: constBody(`{}`)},

		"POST /api/applications/history": {actor: idorOwner, url: func(idorFixture) string { return "/api/applications/history" },
			body: func(f idorFixture, _ int) string {
				return fmt.Sprintf(`{"application_id":%d,"user_id":1,"action_type":"comment"}`, f.app)
			}},
		"DELETE /api/applications/files/:id": {actor: idorOwner, url: func(f idorFixture) string {
			return fmt.Sprintf("/api/applications/files/%d", f.draft)
		}},

		"GET /api/attachments/:id/cars":      att("cars"),
		"GET /api/attachments/:id/employees": att("employees"),
		"GET /api/attachments/:id/items":     att("items"),
		"GET /api/cars/:id/history":          reg("/api/cars/%d/history", func(f idorFixture) int { return f.car }, ""),
		"GET /api/employees/:id/history":     reg("/api/employees/%d/history", func(f idorFixture) int { return f.employee }, ""),

		"PUT /api/unique-cars/:id": {actor: idorOwner, url: func(f idorFixture) string { return fmt.Sprintf("/api/unique-cars/%d", f.uniqueCar) },
			body: func(f idorFixture, _ int) string { return fmt.Sprintf(`{"number":%q,"mark":"DAF"}`, f.plate) }},
		"DELETE /api/unique-cars/:id":              reg("/api/unique-cars/%d", uCar, ""),
		"GET /api/unique-cars/:id/history":         reg("/api/unique-cars/%d/history", uCar, ""),
		"PUT /api/unique-employees/:id":            reg("/api/unique-employees/%d", uEmp, `{"pd_consent":true,"last_name":"Реестров","first_name":"Павел"}`),
		"DELETE /api/unique-employees/:id":         reg("/api/unique-employees/%d", uEmp, ""),
		"GET /api/unique-employees/:id/history":    reg("/api/unique-employees/%d/history", uEmp, ""),
		"POST /api/unique-employees/:id/objection": reg("/api/unique-employees/%d/objection", uEmp, `{"source":"письмо"}`),

		"PUT /api/notifications/:id/read": reg("/api/notifications/%d/read", note, `{"is_read":true}`),
		"DELETE /api/notifications/:id":   reg("/api/notifications/%d", note, ""),
	}
}

// idorSwaps - подмена дочернего объекта: заявка чужака, объект заявки A.
func idorSwaps() map[string]idorSwap {
	child := func(method, format string, id func(f idorFixture) int, body string) idorSwap {
		return idorSwap{method: method, body: func(idorFixture) string { return body },
			url: func(f idorFixture) string { return fmt.Sprintf(format, f.strangerApp, id(f)) }}
	}
	file := func(f idorFixture) int { return f.file }
	question := func(f idorFixture) int { return f.question }
	round := func(f idorFixture) int { return f.supplement }
	flag := func(f idorFixture) int { return f.flag }
	blank := func(f idorFixture) int { return f.blankAtt }
	get, post := http.MethodGet, http.MethodPost
	return map[string]idorSwap{
		"файл заявки":               child(get, "/api/applications/%d/files/%d", file, ""),
		"бланк вложения":            child(get, "/api/applications/%d/blank?attachment_id=%d", blank, ""),
		"ответ на вопрос":           child(post, "/api/applications/%d/questions/%d/answers", question, `{"text":"Ответ"}`),
		"прочтение вопроса":         child(post, "/api/applications/%d/questions/%d/read", question, ""),
		"голос в дополнении":        child(post, "/api/applications/%d/supplements/%d/approve", round, `{"status":"approved"}`),
		"отзыв голоса в дополнении": child(post, "/api/applications/%d/supplements/%d/revoke-approval", round, `{}`),
		"снятие дополнения":         child(post, "/api/applications/%d/supplements/%d/cancel", round, `{}`),
		"снятие подтверждения ЧС":   child(http.MethodDelete, "/api/applications/%d/blacklist-overrides?flag_id=%d", flag, ""),
		"подтверждение ЧС": {method: post,
			url: func(f idorFixture) string {
				return fmt.Sprintf("/api/applications/%d/blacklist-overrides", f.strangerApp)
			},
			body: func(f idorFixture) string { return fmt.Sprintf(`{"flag_id":%d,"comment":"замок"}`, f.flag) }},
	}
}

// TestRouteAccess_IDOR - замок чужой организации. Каждый метод реестра классов scoped и
// self, адресующий объект по id, разобран: либо запрос в idorProbes, либо причина в
// idorCoveredElsewhere. По каждому запросу чужак получает 403 или 404 и не оставляет
// следа - ни записи журнала от своего имени, ни сдвига статуса заявки (так прятался
// перевод «Непрочитано» до проверки доступа, #2608); законный участник на том же
// адресе получает 2xx. Подмена дочернего объекта в своей заявке тоже отказывает.
func TestRouteAccess_IDOR(t *testing.T) {
	probes := idorProbes()
	checkIDORCompleteness(t, probes)

	w := newIDORWorld(t)
	stranger := w.ids[idorStranger]
	for _, key := range keysOf(probes) {
		p := probes[key]
		method, _, _ := strings.Cut(key, " ")
		fx := w.seed(t, !p.noRound)
		target := p.url(fx)
		body := func(uid int) string {
			if p.body == nil {
				return ""
			}
			return p.body(fx, uid)
		}

		before := w.trace(t, fx.app, stranger)
		got := probeRoute(t, w.e, method, target, body(stranger), w.tokens[idorStranger])
		if got.status != http.StatusForbidden && got.status != http.StatusNotFound {
			t.Errorf("%s: чужая организация получила %d вместо 403/404: %s", key, got.status, got.body)
		}
		if after := w.trace(t, fx.app, stranger); after != before {
			t.Errorf("%s: отказ чужаку оставил след: было %s, стало %s", key, before, after)
		}

		ctl := probeRoute(t, w.e, method, target, body(w.ids[p.actor]), w.tokens[p.actor])
		if ctl.status < 200 || ctl.status > 299 {
			t.Errorf("%s: законный участник (%s) получил %d - отказ чужаку ничего не доказывает: %s",
				key, p.actor, ctl.status, ctl.body)
		}
	}

	for _, name := range keysOf(idorSwaps()) {
		s := idorSwaps()[name]
		fx := w.seed(t, true)
		got := probeRoute(t, w.e, s.method, s.url(fx), s.body(fx), w.tokens[idorStranger])
		if got.status != http.StatusForbidden && got.status != http.StatusNotFound {
			t.Errorf("своя заявка, чужой объект (%s): получили %d вместо 403/404: %s", name, got.status, got.body)
		}
	}
}

// trace - след чужака: записи журнала от его имени и состояние заявки A.
func (w *idorWorld) trace(t *testing.T, appID, actorID int) string {
	t.Helper()
	var audit int64
	if err := w.db.Model(&models.AuditLog{}).Where("actor_user_id = ?", actorID).Count(&audit).Error; err != nil {
		t.Fatalf("журнал: %v", err)
	}
	var app models.Application
	if err := w.db.Select("status, confirmation").First(&app, appID).Error; err != nil {
		t.Fatalf("заявка %d: %v", appID, err)
	}
	return fmt.Sprintf("журнал=%d статус=%s подтверждение=%s", audit, deref(app.Status), deref(app.Confirmation))
}

// checkIDORCompleteness - обе стороны полноты: каждый метод scoped/self с параметром
// пути разобран, каждый запрос и исключение указывают на метод реестра, адрес запроса
// совпадает с шаблоном ключа, тест из исключения существует.
func checkIDORCompleteness(t *testing.T, probes map[string]idorProbe) {
	t.Helper()
	for key, ra := range routeAccessRegistry {
		if ra.class != accessScoped && ra.class != accessSelf {
			continue
		}
		_, path, _ := strings.Cut(key, " ")
		if !strings.ContainsAny(path, ":*") {
			continue
		}
		_, probed := probes[key]
		_, covered := idorCoveredElsewhere[key]
		if !probed && !covered {
			t.Errorf("%s адресует объект по id, но не разобран: добавь запрос в idorProbes "+
				"или причину в idorCoveredElsewhere", key)
		}
	}
	sample := idorFixture{app: 1, carsAtt: 2, peopleAtt: 3, itemsAtt: 4, car: 5, employee: 6, file: 7, draft: 8,
		question: 9, supplement: 10, flag: 11, uniqueCar: 12, uniqueEmployee: 13, notification: 14, blankApp: 15, blankAtt: 16}
	for key, p := range probes {
		if _, ok := routeAccessRegistry[key]; !ok {
			t.Errorf("%s есть в idorProbes, но не в реестре", key)
			continue
		}
		_, tmpl, _ := strings.Cut(key, " ")
		if !pathMatches(tmpl, p.url(sample)) {
			t.Errorf("%s: адрес запроса %s не совпадает с шаблоном", key, p.url(sample))
		}
	}
	sources := packageTestSources(t)
	for key, why := range idorCoveredElsewhere {
		if _, ok := routeAccessRegistry[key]; !ok {
			t.Errorf("%s есть в idorCoveredElsewhere, но не в реестре", key)
		}
		if name, _, found := strings.Cut(why, ":"); found && strings.HasPrefix(name, "Test") &&
			!strings.Contains(sources, "func "+name+"(") {
			t.Errorf("%s ссылается на %s, а такого теста в пакете нет", key, name)
		}
	}
}

// pathMatches сверяет адрес с шаблоном echo: ":param" - любой сегмент, "*" - любой хвост.
func pathMatches(tmpl, url string) bool {
	url, _, _ = strings.Cut(url, "?")
	ts, us := strings.Split(tmpl, "/"), strings.Split(url, "/")
	if len(ts) != len(us) {
		return false
	}
	for i := range ts {
		if !strings.HasPrefix(ts[i], ":") && ts[i] != us[i] {
			return false
		}
	}
	return true
}

func packageTestSources(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("исходники пакета: %v", err)
	}
	var sb strings.Builder
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_test.go") {
			b, err := os.ReadFile(e.Name())
			if err != nil {
				t.Fatalf("%s: %v", e.Name(), err)
			}
			sb.Write(b)
		}
	}
	return sb.String()
}
