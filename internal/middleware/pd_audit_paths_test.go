package middleware

import "testing"

// Перечень путей, ведущих к персональным данным, до сих пор не был закрыт ни одним
// тестом: строка сюда добавляется руками, а промах виден только тем, что журнал
// 152-ФЗ молча пустует - ровно так пропадали обращения до #1472.
//
// Тест чистый, без базы: проверяет разбор пути, а не запись строки. Запись сторожит
// DB-тест в internal/handlers.
func TestPDPaths(t *testing.T) {
	cases := []struct {
		path     string
		wantPD   bool
		resource string
	}{
		{"/api/applications/42/participants", true, "application_participants"},
		{"/api/applications/42/files", true, "application_file"},
		{"/api/applications/42/blank", true, "attachment_blank"},
		{"/api/applications/42/archive", true, "application_archive"},
		{"/api/employees", true, "employee"},
		{"/api/search?q=иванов", true, "search"},
		{"/api/file-archive/items", true, "file_archive"},
		{"/api/request-logs/export?from_date=2026-08-01", true, "request_logs_export"},
		// Остальные методы раздела мониторинга отдают показатели и строки журнала
		// на экран, а не файлом: под 152-ФЗ попадает вынос пачкой, а не просмотр.
		{"/api/request-logs", false, ""},
		{"/api/request-logs/stats", false, ""},
		// #2352: карточка заявки и её соседи персональные данные всё-таки отдают -
		// sender_full_name/responsible_full_name в детали, ФИО актора в истории, ФИО
		// и должность в составе ответственных. Раньше здесь стояло "false" - ровно та
		// дыра, которую закрывает эта задача.
		{"/api/applications", true, "application"},
		{"/api/applications/user", true, "application"},
		{"/api/applications/attachable", true, "application"},
		{"/api/applications/42", true, "application"},
		{"/api/applications/42/details", true, "application"},
		{"/api/applications/42/history", true, "application"},
		{"/api/applications/42/responsible-users", true, "application"},
		{"/api/applications/42/forward-messages", true, "application"},
		{"/api/applications/42/viewers", true, "application"},
		{"/api/applications/42/reads", true, "application"},
		{"/api/applications/42/questions", true, "application"},
		{"/api/applications/42/supplements", true, "application"},
		// Статические соседи на том же уровне маршрутизации персональных данных не
		// отдают: только счётчики/флаги, без единого ФИО.
		{"/api/applications/unread-count", false, ""},
		{"/api/applications/user/status-updates-count", false, ""},
		{"/api/applications/42/check-approval-status", false, ""},
		{"/api/applications/42/attachments", false, ""},
		// Действия (не просмотр) под теми же суффиксами-соседями персональных данных
		// в ответе не показывают - только статус операции.
		{"/api/applications/42/questions/seen", false, ""},
		{"/api/applications/42/supplements/3/approve", false, ""},
		// Список пользователей (#2352): доказано на стенде - счётчик журнала не
		// сдвигался на этом пути, хотя ответ несёт ФИО, должность, почту, телефон.
		{"/api/users/all", true, "user"},
		{"/api/users/all?include_archived=true", true, "user"},
		// История изменений учётки: ActorName - тоже ФИО, тот же принцип, что и
		// история пользователя-заявки.
		{"/api/users/ivanov/history", true, "user"},
		// Кандидаты в получатели заявки: models.RecipientCandidate несёт ФИО и
		// должность коллег, доступно любому авторизованному (не только page.admin.users).
		{"/api/users/recipient-candidates", true, "user"},
		// Соседи по управлению пользователями в ответе ФИО не показывают - только
		// статус операции (создание/смена типа/пароля/архивация).
		{"/api/users/me", false, ""},
		{"/api/users/me/theme", false, ""},
		{"/api/users/ivanov/type", false, ""},
		{"/api/users/ivanov/password", false, ""},
		{"/api/users/ivanov/unload-places", false, ""},
		{"/api/users/bulk/archive", false, ""},
		// Реестр автомобилей (#2352): зеркало /api/unique-employees - UserName в
		// ответе это ФИО владельца записи (см. maskCarOwners).
		{"/api/unique-cars", true, "unique_car"},
		{"/api/unique-cars/5/history", true, "unique_car"},
		// История машин (заявочных, не реестра): ФИО охранника, менявшего статус.
		{"/api/cars/42/history", true, "car"},
		{"/api/cars/history/all", true, "car"},
		{"/api/cars/history/table/5", true, "car"},
		{"/api/cars/history/unified", true, "car"},
		// Текущий статус и живая таблица поста персональных данных не показывают:
		// номер и марка машины субъекта не идентифицируют (см. UniqueCar.PDConsentAt).
		{"/api/cars/history/current-status", false, ""},
		{"/api/cars/active-for-table/5", false, ""},
		{"/api/cars/fact-for-table/5", false, ""},
		// Корзина и слепок таблицы поста (#186, #980): те же ФИО/номера машин, что
		// в основной таблице, только удалённые или зафиксированные версией. История
		// структуры таблицы (кто её настраивал) - тот же resource: UserName актора.
		{"/api/system-tables/5/trash", true, "system_table_content"},
		{"/api/system-tables/5/trash/history", true, "system_table_content"},
		{"/api/system-tables/5/snapshots/9", true, "system_table_content"},
		{"/api/system-tables/5/snapshots/9/export", true, "system_table_content"},
		{"/api/system-tables/5/history", true, "system_table_content"},
		// Список версий (без /{sid}) отдаёт только метаданные, восстановление и
		// очистка корзины - только счётчик; структура таблицы - конфигурация, не
		// содержимое.
		{"/api/system-tables/5/snapshots", false, ""},
		{"/api/system-tables/5/trash/restore", false, ""},
		{"/api/system-tables/5", false, ""},
		{"/api/organizations", false, ""},
	}

	for _, c := range cases {
		if got := isPDPath(c.path); got != c.wantPD {
			t.Errorf("isPDPath(%q) = %v, ожидалось %v", c.path, got, c.wantPD)
			continue
		}
		if !c.wantPD {
			continue
		}
		if got := pathToResource(c.path); got != c.resource {
			t.Errorf("pathToResource(%q) = %q, ожидалось %q", c.path, got, c.resource)
		}
	}
}

// Раздел журнала не должен оставаться «unknown»: по нему администратор фильтрует
// записи, и безымянный раздел делает фильтр бесполезным.
func TestPDResourceNamedForEveryPDPath(t *testing.T) {
	paths := []string{
		"/api/applications/7/participants",
		"/api/applications/7/files/3",
		"/api/unique-employees/5",
		"/api/attachments/1/employees",
		"/api/settings/pd-consent/collection",
		"/api/applications/export",
		"/api/request-logs/export",
		"/api/applications/7",
		"/api/applications/7/history",
		"/api/users/all",
		"/api/users/ivanov/history",
		"/api/unique-cars",
		"/api/cars/7/history",
		"/api/system-tables/7/trash",
		"/api/system-tables/7/snapshots/1",
	}
	for _, p := range paths {
		if !isPDPath(p) {
			t.Errorf("%q должен считаться обращением к персональным данным", p)
			continue
		}
		if r := pathToResource(p); r == "unknown" {
			t.Errorf("%q попал в журнал без имени раздела", p)
		}
	}
}
