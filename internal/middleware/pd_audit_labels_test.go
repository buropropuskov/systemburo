package middleware

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Раздел, который вернул pathToResource, обязан иметь подпись на экране журнала
// обращений: без неё он не появляется в фильтре, а в строке показывается служебным
// кодом вроде «applications_export». Перечень на фронте вели руками, и он отстал -
// из двенадцати разделов подписаны были пять.
//
// Замок сверяет два файла напрямую, без списка внутри теста: список тоже пришлось бы
// вести руками, и он отстал бы третьим.
func TestPDResourceLabeledOnScreen(t *testing.T) {
	resources := resourcesFromPathToResource(t)
	if len(resources) < 10 {
		t.Fatalf("из pathToResource разобрано %d разделов - похоже, изменилась форма функции", len(resources))
	}

	view, err := os.ReadFile("../../frontend/src/views/admin/PdAuditLog.vue")
	if err != nil {
		t.Fatalf("не прочитан экран журнала обращений: %v", err)
	}
	labels := string(view)

	for _, r := range resources {
		if !strings.Contains(labels, r+":") {
			t.Errorf("раздел %q попадает в журнал, но подписи на экране у него нет "+
				"(RESOURCE_LABELS в PdAuditLog.vue) - в фильтре он отсутствует, "+
				"а в строке виден служебным кодом", r)
		}
	}
}

// resourcesFromPathToResource собирает имена разделов из самой функции разбора.
func resourcesFromPathToResource(t *testing.T) []string {
	t.Helper()

	src, err := os.ReadFile("pd_audit.go")
	if err != nil {
		t.Fatalf("не прочитан pd_audit.go: %v", err)
	}

	body := string(src)
	start := strings.Index(body, "func pathToResource(")
	if start < 0 {
		t.Fatal("в pd_audit.go не найдена функция pathToResource")
	}
	end := strings.Index(body[start:], "\n}")
	if end < 0 {
		t.Fatal("не найден конец функции pathToResource")
	}

	var out []string
	for _, m := range regexp.MustCompile(`return "([a-z_]+)"`).FindAllStringSubmatch(body[start:start+end], -1) {
		if m[1] != "unknown" {
			out = append(out, m[1])
		}
	}
	return out
}
