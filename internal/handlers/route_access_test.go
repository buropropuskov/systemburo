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
	// accessPerm - гейт в роутере по ключу права; без ключа 403 с required_permission.
	accessPerm accessClass = "perm"
	// accessRole - обработчик сам требует роль или флаг (супер, админ, принимающий).
	accessRole accessClass = "role"
	// accessScoped - обработчик или сервис проверяет доступ к конкретному объекту.
	accessScoped accessClass = "scoped"
	// accessSelf - только данные самого вызывающего, пользователь берётся из токена.
	accessSelf accessClass = "self"
	// accessOpen - любому вошедшему, так задумано; причина обязательна.
	accessOpen accessClass = "open"
)

// routeAccess - решение о доступе к одному методу. Для accessPerm key - ключ гейта,
// где "{post}" заменяет имя поста в ключах table.<пост>.<глагол>; для остальных
// классов why объясняет решение человеку, который через год спросит «почему этот
// метод доступен».
type routeAccess struct {
	class accessClass
	key   string
	why   string
}

func perm(key string) routeAccess         { return routeAccess{class: accessPerm, key: key} }
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
// гейт по праву (ключ виден в required_permission) и решение в обработчике.
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

	token := testutil.RegisterAndLogin(t, e, "route_access_noperm", "pass123", 1, td.OrgID, td.CompanyID)
	uid := getUserID(t, db, "route_access_noperm")
	for _, k := range services.AllCatalogKeys() {
		testutil.DenyPermission(t, uid, k)
	}

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
		if ra.class == accessPerm && ra.key == "" || ra.class != accessPerm && strings.TrimSpace(ra.why) == "" {
			t.Errorf("%s: у perm нужен ключ, у остальных классов причина", key)
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

		got := probeRoute(t, e, method, target, body, token)
		switch ra.class {
		case accessPerm:
			want := strings.ReplaceAll(ra.key, "{post}", post)
			if got.status != http.StatusForbidden || got.key != want {
				t.Errorf("%s: в реестре гейт %q, а без прав ответ %d с ключом %q: %s",
					key, want, got.status, got.key, got.body)
			}
		default:
			// Статус здесь не сверяется: ролевые и объектные проверки у части методов
			// идут после разбора тела, и пустой запрос получает 400 раньше отказа.
			if got.key != "" {
				t.Errorf("%s: в реестре %s, а в роутере гейт %q - перенеси в perm", key, ra.class, got.key)
			}
		}
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

// concreteRoutePath подставляет id поста вместо каждого :param и "*" - гейтам постов
// нужен существующий пост, остальным обработчикам хватает любого числа.
func concreteRoutePath(tmpl string, id int) string {
	segments := strings.Split(tmpl, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") {
			segments[i] = fmt.Sprint(id)
		}
	}
	return strings.ReplaceAll(strings.Join(segments, "/"), "*", fmt.Sprint(id))
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
