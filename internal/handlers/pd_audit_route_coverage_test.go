package handlers_test

import (
	"net/http"
	"strings"
	"testing"

	mw "systemburo/internal/middleware"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
)

// pdAuditDomainPrefixes - разделы, для которых задача #2352 разобрала КАЖДЫЙ
// обработчик (проверено по коду - отдаёт ли он ФИО/документы/контакты/номера
// машин). Список сознательно уже, чем весь /api: сделать такую сверку для
// произвольного нового раздела нельзя без чтения его кода (см. пакет
// TestPDAudit_NoUntriagedGetRoute) - a вот внутри уже разобранных разделов новый
// GET-роут обязан явно попасть либо в pdPaths/хелперы pd_audit.go, либо сюда с
// причиной, иначе тест падает.
var pdAuditDomainPrefixes = []string{
	"/api/applications", "/api/users", "/api/cars", "/api/unique-cars", "/api/system-tables",
}

// pdAuditKnownNonPDGetRoutes - GET-роуты внутри pdAuditDomainPrefixes, для которых
// код обработчика проверен и персональных данных не отдаёт (#2352). Причина - не
// формальность: через полгода по ней видно, что решение принято осознанно, а не
// забыто. Ключ - "METHOD path" в нотации echo (":param", не конкретное число).
var pdAuditKnownNonPDGetRoutes = map[string]string{
	// applications: списки/детали персональные данные отдают (см. pd_audit.go),
	// но эти соседи - только счётчики, флаги или метаданные вложения.
	"GET /api/applications/unread-count":              "счётчик, без ФИО",
	"GET /api/applications/user/status-updates-count": "счётчик, без ФИО",
	"GET /api/applications/available-attachments":     "перечень доступных вложений (номера/статус), сами сотрудники/машины - за /api/attachments",
	"GET /api/applications/:id/check-approval-status": "статус-enum согласования, без ФИО",
	"GET /api/applications/:id/attachments":           "метаданные вложения (тип/имя/счётчик), сотрудники/машины вложения - за /api/attachments (уже в pdPaths)",
	// users: /me и /current показывают человеку его же учётку - это не обращение к
	// чужим персональным данным, а не просмотр в смысле 152-ФЗ; остальные -
	// операции без ФИО в ответе (только статус/список назначений).
	"GET /api/users/me":                         "пользователь смотрит свою же учётку",
	"GET /api/users/current":                    "алиас /me",
	"GET /api/users/me/theme":                   "настройка интерфейса, не ПД",
	"GET /api/users/:username/unload-places":    "назначенные места разгрузки охранника, без ФИО в ответе",
	"GET /api/users/:username/tables":           "назначенные таблицы охранника, без ФИО в ответе",
	"GET /api/users/:username/auth-events":      "models.AuthEventResponse без Username - только IP/UA/событие (см. auth_event.go)",
	"GET /api/users/:user_id/permission-groups": "PermissionGroupResponse - имя группы и её права, без ФИО (см. models/permission.go)",
	// cars: живая таблица поста и текущий статус не идентифицируют субъекта - номер
	// и марка машины персональными данными не считаются (см. UniqueCar.PDConsentAt);
	// история машин (ФИО охранника) уже в pd_audit.go.
	"GET /api/cars/active-for-table/:table_id": "TableCarResponse - номер/марка/организация, без ФИО",
	"GET /api/cars/fact-for-table/:table_id":   "то же самое для фактовой таблицы",
	"GET /api/cars/unload-places":              "назначенные места разгрузки, без ФИО",
	"GET /api/cars/fact-unload-places":         "то же самое для фактовой таблицы",
	"GET /api/cars/check-active":               "булев признак, без ФИО",
	"GET /api/cars/history/current-status":     "CarCurrentStatus - car_id/статус/время, без единого имени",
	// system-tables: конфигурация таблицы (структура, слайты, права) - не её
	// содержимое; содержимое (корзина/слепок/история конфигурации) уже в pd_audit.go.
	"GET /api/system-tables":                      "список таблиц - конфигурация, не содержимое",
	"GET /api/system-tables/:id":                  "конфигурация одной таблицы",
	"GET /api/system-tables/name/:name":           "то же самое по имени",
	"GET /api/system-tables/:id/usage":            "счётчики организаций/компаний, привязанных к таблице",
	"GET /api/system-tables/:id/time-slots":       "конфигурация окон времени",
	"GET /api/system-tables/:id/warning-windows":  "конфигурация предупреждающих окон",
	"GET /api/system-tables/:id/snapshots":        "метаданные версий (дата/автор/агрегаты) без payload - см. isSystemTableSnapshotPayloadPath",
	"GET /api/system-tables/:id/pass-report/live": "агрегированные счётчики событий по охранникам, без ФИО проходящих",
	"GET /api/system-tables/:id/pass-reports":     "то же самое, история дней",
}

// concretePath подставляет вместо каждого :param правдоподобное числовое значение -
// так же, как выглядит реальный путь запроса, который видит isPDPath.
func concretePath(tmpl string) string {
	segments := strings.Split(tmpl, "/")
	for i, seg := range segments {
		if strings.HasPrefix(seg, ":") {
			segments[i] = "1"
		}
	}
	return strings.Join(segments, "/")
}

// TestPDAudit_NoUntriagedGetRoute - замок против дыры #2352: новый GET-роут внутри
// уже разобранных разделов (pdAuditDomainPrefixes) обязан быть либо покрыт
// pdPaths/хелперами pd_audit.go, либо явно занесён в pdAuditKnownNonPDGetRoutes с
// причиной. Молчаливого варианта нет - тест падает на любом непокрытом роуте.
//
// Это НЕ универсальная защита на весь /api: решить автоматически, отдаёт ли
// произвольный обработчик персональные данные, нельзя - для этого пришлось бы
// разбирать тело ответа по коду, а не по пути (собственно то, чем и была эта
// задача). Реалистичный компромисс - держать список разобранных разделов
// (pdAuditDomainPrefixes) в актуальном состоянии: разбирая новый раздел на
// персональные данные, дописывать его сюда, и тест начнёт сторожить его тоже.
func TestPDAudit_NoUntriagedGetRoute(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	_ = db

	checked := 0
	for _, r := range e.Routes() {
		if r.Method != http.MethodGet {
			continue
		}
		inDomain := false
		for _, prefix := range pdAuditDomainPrefixes {
			if strings.HasPrefix(r.Path, prefix) {
				inDomain = true
				break
			}
		}
		if !inDomain {
			continue
		}
		checked++

		if mw.IsPDPathForRouteCoverage(concretePath(r.Path)) {
			continue
		}
		key := "GET " + r.Path
		if _, ok := pdAuditKnownNonPDGetRoutes[key]; ok {
			continue
		}
		t.Errorf("%s не триажен: разбери обработчик (отдаёт ли ФИО/документы/контакты/номера) "+
			"и добавь путь либо в pdPaths/хелперы internal/middleware/pd_audit.go, "+
			"либо в pdAuditKnownNonPDGetRoutes с причиной", key)
	}
	assert.Greater(t, checked, 20, "маловато роутов разобрано - похоже, изменился роутер или список префиксов")
}
