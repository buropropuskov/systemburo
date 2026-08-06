package middleware

import "testing"

// Файлы заявки попадают в журнал 152-ФЗ: поле общее, «приложите документы», и что
// именно там лежит, система заранее не знает. Разрешение на работу, а то и скан
// паспорта вопреки подписи поля - значит обращения к ним считаются просмотром
// персональных данных, как поштучный бланк и выгрузка архива.
func TestIsPDPath_ApplicationFiles(t *testing.T) {
	cases := []struct {
		path     string
		want     bool
		resource string
	}{
		{"/api/applications/12/files", true, "application_file"},
		{"/api/applications/12/files/34", true, "application_file"},
		// Загрузка черновика идёт без номера заявки и тоже несёт документ.
		{"/api/applications/files", false, ""},
		// Сама заявка персональных данных в этом смысле не отдаёт: состав вложений
		// уже закрыт другими правилами перечня.
		{"/api/applications/12", false, ""},
		{"/api/applications", false, ""},
		// Соседние маршруты заявки не должны попасть под правило целиком.
		{"/api/applications/12/attachments", false, ""},
	}

	for _, c := range cases {
		if got := isPDPath(c.path); got != c.want {
			t.Errorf("isPDPath(%q) = %v, ожидалось %v", c.path, got, c.want)
		}
		if !c.want {
			continue
		}
		if got := pathToResource(c.path); got != c.resource {
			t.Errorf("pathToResource(%q) = %q, ожидалось %q", c.path, got, c.resource)
		}
	}
}
