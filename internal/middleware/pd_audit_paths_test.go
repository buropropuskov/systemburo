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
		// Соседи по карточке заявки персональных данных не отдают: состав ответственных
		// приходит без контактов, история - записями о действиях.
		{"/api/applications/42/responsible-users", false, ""},
		{"/api/applications/42/history", false, ""},
		{"/api/applications/42", false, ""},
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
