package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// accessClass - кто получает доступ к методу API и где это решается.
type accessClass string

const (
	// accessPublic - вне JWT: без входа или со своей проверкой (билет, refresh-cookie).
	accessPublic accessClass = "public"
	// accessPerm - гейты в роутере по ключам права, по порядку срабатывания: гейт группы,
	// затем гейт метода. Без очередного ключа 403 с ним в required_permission.
	accessPerm accessClass = "perm"
	// accessRole - обработчик сам требует роль или флаг (супер, админ, принимающий).
	// На каждый метод заведён валидный запрос в roleProbes: без роли ответ 403.
	accessRole accessClass = "role"
	// accessScoped - обработчик или сервис проверяет доступ к конкретному объекту.
	accessScoped accessClass = "scoped"
	// accessSelf - только данные самого вызывающего, пользователь берётся из токена.
	accessSelf accessClass = "self"
	// accessOpen - любому вошедшему, так задумано; причина обязательна. Пользователю
	// без прав не 403.
	accessOpen accessClass = "open"
)

// routeAccess - решение о доступе к одному методу. Для accessPerm keys - цепочка ключей
// гейтов, где "{post}" заменяет имя поста в ключах table.<пост>.<глагол>; для остальных
// классов why объясняет решение человеку, который через год спросит «почему этот
// метод доступен».
type routeAccess struct {
	class accessClass
	keys  []string
	why   string
}

func perm(keys ...string) routeAccess     { return routeAccess{class: accessPerm, keys: keys} }
func role(why string) routeAccess         { return routeAccess{class: accessRole, why: why} }
func scoped(why string) routeAccess       { return routeAccess{class: accessScoped, why: why} }
func self(why string) routeAccess         { return routeAccess{class: accessSelf, why: why} }
func open(why string) routeAccess         { return routeAccess{class: accessOpen, why: why} }
func public(why string) routeAccess       { return routeAccess{class: accessPublic, why: why} }
func routeKey(method, path string) string { return method + " " + path }

// jwtMissingHeaderError - отказ JWT-прослойки запросу без токена. Публичные методы
// с собственной проверкой (билет потока событий, refresh-cookie) тоже отвечают 401,
// но своим текстом, поэтому «вне JWT» определяется по тексту отказа, а не по статусу.
const jwtMissingHeaderError = "Missing or invalid authorization header"

// routeAccessProbeTimeout ограничивает один запрос замка: поток событий иначе держал
// бы соединение до конца теста.
const routeAccessProbeTimeout = 5 * time.Second

// accessProbe - один запрос к приложению и то, что замку нужно из ответа.
type accessProbe struct {
	status int
	key    string
	body   string
}

// TestRouteAccess_Registry - замок реестра routeAccessRegistry в обе стороны:
// каждый зарегистрированный метод API разобран, каждая запись реестра указывает на
// существующий метод, и записанное решение совпадает с тем, что отвечает сервер.
// Сверка идёт двумя запросами на метод: без токена и от пользователя, у которого
// запрещены все ключи каталога прав. Этого хватает, чтобы отличить публичный метод,
// гейт по праву (ключ виден в required_permission) и решение в обработчике. Методам
// role тот же пользователь шлёт запрос из roleProbes, валидный по телу, и получает 403
// обработчика; снятые методы из removedRoutes не должны появиться снова.
func TestRouteAccess_Registry(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Гейты постов берут имя поста из базы по id из пути или тела: без поста они
	// отвечают 404 раньше проверки права, и ключ не виден.
	post := "route_access_post"
	dn := "Пост замка реестра"
	table := models.SystemTable{Name: post, DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&table).Error)

	tokens := map[string]string{}
	userIDs := map[string]int{}
	// tokenWith - пользователь, у которого есть только ключи granted, остальные ключи
	// каталога запрещены. Один пользователь на набор ключей: кэш прав резолвера живёт
	// до конца теста.
	tokenWith := func(granted []string) string {
		set := strings.Join(granted, "|")
		if tok, ok := tokens[set]; ok {
			return tok
		}
		name := fmt.Sprintf("route_access_%d", len(tokens))
		tok := testutil.RegisterAndLogin(t, e, name, "pass123", 1, td.OrgID, td.CompanyID)
		uid := getUserID(t, db, name)
		allowed := map[string]bool{}
		for _, k := range granted {
			allowed[k] = true
			testutil.GrantPermission(t, uid, k)
		}
		for _, k := range services.AllCatalogKeys() {
			if !allowed[k] {
				testutil.DenyPermission(t, uid, k)
			}
		}
		tokens[set] = tok
		userIDs[set] = uid
		return tok
	}
	noKeys := tokenWith(nil)
	probes := roleProbes(seedRoleFixture(t, db, td, userIDs[""]))

	routes := registeredRoutes(e.Routes())

	for key := range routes {
		if _, ok := routeAccessRegistry[key]; !ok {
			t.Errorf("%s не разобран: добавь в routeAccessRegistry решение о доступе "+
				"(perm с ключом гейта, либо role/scoped/self/open/public с причиной)", key)
		}
	}
	for key, ra := range routeAccessRegistry {
		if _, ok := routes[key]; !ok {
			t.Errorf("%s есть в routeAccessRegistry, но не в роутере: удали запись", key)
		}
		if ra.class == accessPerm && len(ra.keys) == 0 || ra.class != accessPerm && strings.TrimSpace(ra.why) == "" {
			t.Errorf("%s: у perm нужен ключ, у остальных классов причина", key)
		}
		if _, ok := probes[key]; ra.class == accessRole && !ok {
			t.Errorf("%s: у role нужен запрос в roleProbes, на котором без роли сервер отвечает 403", key)
		}
	}
	for key := range probes {
		if ra, ok := routeAccessRegistry[key]; !ok || ra.class != accessRole {
			t.Errorf("%s есть в roleProbes, но в реестре не role: удали запрос", key)
		}
	}
	for key, why := range removedRoutes {
		if routes[key] {
			t.Errorf("%s снова зарегистрирован, а был снят: %s", key, why)
		}
	}

	// Ролевой проход идёт до общего: общий шлёт на каждый :id один и тот же номер и
	// может снести объект фикстуры (владелец удаляет свою запись реестра).
	for _, key := range sortedKeys(routes) {
		rp, ok := probes[key]
		if !ok {
			continue
		}
		method, path, _ := strings.Cut(key, " ")
		id := table.ID
		if rp.id != 0 {
			id = rp.id
		}
		got := probeRoute(t, e, method, concreteRoutePath(path, id), rp.body, noKeys)
		if got.status != http.StatusForbidden || got.key != "" {
			t.Errorf("%s: без роли ждали 403 обработчика, получили %d с ключом %q: %s",
				key, got.status, got.key, got.body)
		}
	}

	body := fmt.Sprintf(`{"table_id":%d,"territory_status":1}`, table.ID)
	for _, key := range sortedKeys(routes) {
		ra, ok := routeAccessRegistry[key]
		if !ok {
			continue
		}
		method, path, _ := strings.Cut(key, " ")
		target := concreteRoutePath(path, table.ID)

		anon := probeRoute(t, e, method, target, body, "")
		if ra.class == accessPublic {
			if anon.status == http.StatusUnauthorized && strings.Contains(anon.body, jwtMissingHeaderError) {
				t.Errorf("%s: в реестре public, а роут стоит за JWT", key)
			}
			continue
		}
		if anon.status != http.StatusUnauthorized {
			t.Errorf("%s: без входа ждали 401, получили %d: %s", key, anon.status, anon.body)
		}

		if ra.class == accessPerm {
			checkPermChain(t, key, ra.keys, post, func(granted []string) accessProbe {
				return probeRoute(t, e, method, target, body, tokenWith(granted))
			})
			continue
		}
		if ra.class == accessRole {
			continue
		}
		// Статус scoped и self здесь не сверяется: объектные проверки у части методов
		// идут после разбора тела, и общий запрос получает 400 раньше отказа.
		got := probeRoute(t, e, method, target, body, noKeys)
		if got.key != "" {
			t.Errorf("%s: в реестре %s, а в роутере гейт %q - перенеси в perm", key, ra.class, got.key)
		}
		if ra.class == accessOpen && got.status == http.StatusForbidden {
			t.Errorf("%s: в реестре open, а пользователю без прав 403: %s", key, got.body)
		}
	}
}

// checkPermChain сверяет цепочку гейтов: с первыми i ключами ответ - 403 с (i+1)-м,
// а со всеми ключами ни один гейт роутера не отказывает. Вторая половина ловит гейт,
// которого нет в записи: пользователь без прав упирается во внешний гейт группы, и
// внутренний, более узкий, иначе не был бы виден вовсе.
func checkPermChain(t *testing.T, key string, keys []string, post string, probe func([]string) accessProbe) {
	t.Helper()
	chain := make([]string, len(keys))
	for i, k := range keys {
		chain[i] = strings.ReplaceAll(k, "{post}", post)
	}
	for i, want := range chain {
		got := probe(chain[:i])
		if got.status != http.StatusForbidden || got.key != want {
			t.Errorf("%s: с ключами %v ждали 403 с ключом %q, получили %d с ключом %q: %s",
				key, chain[:i], want, got.status, got.key, got.body)
		}
	}
	// Ключ только для супер-админа персонально не выдаётся: пользователя, прошедшего
	// такой гейт, в тесте не собрать, первая половина проверки его уже покрыла.
	if services.IsSuperOnly(chain[len(chain)-1]) {
		return
	}
	if got := probe(chain); got.key != "" {
		t.Errorf("%s: со всеми ключами из реестра %v гейт требует ещё %q - допиши его в цепочку",
			key, chain, got.key)
	}
}

// registeredRoutes - методы приложения в нотации "METHOD путь-шаблон". Служебный
// обработчик echo для несуществующих путей в реестр не входит.
func registeredRoutes(all []*echo.Route) map[string]bool {
	out := map[string]bool{}
	for _, r := range all {
		if r.Method == "echo_route_not_found" {
			continue
		}
		out[routeKey(r.Method, r.Path)] = true
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// concreteRoutePath подставляет id поста вместо каждого :param и хвоста "*" - гейтам постов
// нужен существующий пост, остальным обработчикам хватает любого числа.
func concreteRoutePath(tmpl string, id int) string {
	segments := strings.Split(tmpl, "/")
	for i, seg := range segments {
		switch {
		case strings.HasPrefix(seg, ":"):
			segments[i] = fmt.Sprint(id)
		case strings.HasSuffix(seg, "*"):
			segments[i] = strings.TrimSuffix(seg, "*") + fmt.Sprintf("/%d", id)
		}
	}
	return strings.Join(segments, "/")
}

func probeRoute(t *testing.T, e http.Handler, method, target, body, token string) accessProbe {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), routeAccessProbeTimeout)
	defer cancel()
	var req *http.Request
	if method == http.MethodGet || method == http.MethodHead {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	req = req.WithContext(ctx)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var parsed struct {
		Required string `json:"required_permission"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &parsed) // тело не обязано быть JSON (файлы, поток)
	b := rec.Body.String()
	if len(b) > 200 {
		b = b[:200]
	}
	return accessProbe{status: rec.Code, key: parsed.Required, body: b}
}
