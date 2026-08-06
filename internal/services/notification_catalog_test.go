package services

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestNotificationCatalogFieldsFilled(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{})
	for _, meta := range NotificationCatalog() {
		if _, dup := seen[meta.Code]; dup {
			t.Errorf("дубликат кода в каталоге уведомлений: %s", meta.Code)
		}
		seen[meta.Code] = struct{}{}
		if meta.Code == "" || meta.Label == "" || meta.Description == "" || meta.Category == "" {
			t.Errorf("узел каталога уведомлений с пустым полем: %+v", meta)
		}
	}
	if len(seen) == 0 {
		t.Fatal("каталог уведомлений пуст")
	}
}

func TestNotificationCatalogMandatoryOnlySecurity(t *testing.T) {
	t.Parallel()
	for _, meta := range NotificationCatalog() {
		isSecurity := meta.Category == NotificationCategorySecurity
		if meta.Mandatory != isSecurity {
			t.Errorf("%s: Mandatory=%v при категории %s (обязательными должны быть ровно security-типы)",
				meta.Code, meta.Mandatory, meta.Category)
		}
	}
}

func TestNotificationCatalogStableOrder(t *testing.T) {
	t.Parallel()
	first := NotificationCatalog()
	second := NotificationCatalog()
	if len(first) != len(second) {
		t.Fatalf("длина каталога различается между вызовами: %d != %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Code != second[i].Code {
			t.Fatalf("порядок каталога нестабилен на позиции %d: %s != %s", i, first[i].Code, second[i].Code)
		}
	}
	// В пределах категории коды идут по возрастанию, категории - в порядке
	// notificationCategoryOrder.
	rank := make(map[NotificationCategory]int, len(notificationCategoryOrder))
	for i, cat := range notificationCategoryOrder {
		rank[cat] = i
	}
	for i := 1; i < len(first); i++ {
		prev, cur := first[i-1], first[i]
		if rank[prev.Category] > rank[cur.Category] {
			t.Errorf("категория уехала не по порядку: %s (%s) после %s (%s)", cur.Code, cur.Category, prev.Code, prev.Category)
		}
		if prev.Category == cur.Category && prev.Code >= cur.Code {
			t.Errorf("коды внутри категории %s не по возрастанию: %s перед %s", cur.Category, prev.Code, cur.Code)
		}
	}
}

func TestNotificationTypeMeta(t *testing.T) {
	t.Parallel()
	meta, ok := NotificationTypeMeta(NotificationTypePasswordChanged)
	if !ok {
		t.Fatal("password_changed должен быть в каталоге")
	}
	if meta.Category != NotificationCategorySecurity || !meta.Mandatory {
		t.Errorf("password_changed: неожиданные метаданные %+v", meta)
	}
	if _, ok := NotificationTypeMeta("nonexistent_type"); ok {
		t.Error("несуществующий код не должен резолвиться каталогом")
	}
}

// TestNotificationCatalogTypesHaveTriggers -- замок против типа-призрака: кода,
// который лежит в каталоге, попадает переключателем на экран настроек, но не
// создаётся ни одним сервисом. Прежняя версия сверяла рукописный список с самим
// каталогом и поэтому не заметила ни одного из тринадцати типов, добавленных
// параллельными срезами #1748: список просто не пополняли. Теперь имена констант
// берутся из самого каталога, а наличие триггера проверяется по исходникам пакета.
//
// Обратное направление держит компилятор: код, переданный в CreateForUser мимо
// каталога, - это необъявленная константа либо голый литерал, и второе ловит
// TestNotificationCatalogNoStringLiterals ниже.
func TestNotificationCatalogTypesHaveTriggers(t *testing.T) {
	t.Parallel()
	names := catalogConstNames(t)
	if len(names) == 0 {
		t.Fatal("не удалось прочитать имена констант из notification_catalog.go")
	}

	sources := packageSources(t, "notification_catalog.go")
	for _, name := range names {
		used := false
		for _, src := range sources {
			if strings.Contains(src, name) {
				used = true
				break
			}
		}
		if !used {
			t.Errorf("тип уведомления %s есть в каталоге, но его не создаёт ни один сервис: "+
				"на экране настроек он станет переключателем, который ничего не делает", name)
		}
	}
}

// TestNotificationCatalogNoStringLiterals -- второй замок: код типа передаётся
// константой каталога, а не строкой на месте вызова. Литерал компилируется молча
// и обходит и каталог, и гейт подписок.
func TestNotificationCatalogNoStringLiterals(t *testing.T) {
	t.Parallel()
	codes := make([]string, 0, len(notificationCatalog))
	for code := range notificationCatalog {
		codes = append(codes, code)
	}
	for name, src := range packageSources(t, "notification_catalog.go") {
		for _, code := range codes {
			if strings.Contains(src, `"`+code+`"`) {
				t.Errorf("%s: код уведомления %q написан строкой - используй константу каталога", name, code)
			}
		}
	}
}

// catalogConstNames вытаскивает имена констант вида NotificationTypeXxx = "code"
// из самого каталога.
func catalogConstNames(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("notification_catalog.go")
	if err != nil {
		t.Fatalf("не удалось прочитать каталог: %v", err)
	}
	re := regexp.MustCompile(`(NotificationType[A-Za-z]+)\s*=\s*"`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	return names
}

// packageSources читает исходники пакета (без тестов и без указанных файлов) -
// по ним проверяется, что тип каталога где-то действительно создаётся.
func packageSources(t *testing.T, exclude ...string) map[string]string {
	t.Helper()
	skip := make(map[string]bool, len(exclude))
	for _, name := range exclude {
		skip[name] = true
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("не удалось прочитать пакет: %v", err)
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || skip[name] {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("не удалось прочитать %s: %v", name, err)
		}
		out[name] = string(data)
	}
	return out
}

func TestNotificationCategoriesOrder(t *testing.T) {
	t.Parallel()
	cats := NotificationCategories()
	want := []NotificationCategory{
		NotificationCategoryApplication,
		NotificationCategorySecurity,
		NotificationCategoryPassage,
		NotificationCategoryContent,
		NotificationCategorySystem,
	}
	if len(cats) != len(want) {
		t.Fatalf("ожидалось %d категорий, получено %d", len(want), len(cats))
	}
	for i, cat := range want {
		if cats[i] != cat {
			t.Errorf("категория на позиции %d: %s, ожидалось %s", i, cats[i], cat)
		}
	}
}
