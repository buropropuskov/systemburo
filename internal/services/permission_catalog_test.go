package services

import "testing"

func TestCatalogNoDuplicateKeys(t *testing.T) {
	t.Parallel()
	seen := make(map[string]struct{})
	for _, n := range Catalog() {
		if _, dup := seen[n.Key]; dup {
			t.Errorf("дубликат ключа в каталоге: %s", n.Key)
		}
		seen[n.Key] = struct{}{}
		if n.Key == "" || n.DisplayName == "" || n.Category == "" {
			t.Errorf("узел каталога с пустым полем: %+v", n)
		}
	}
	if len(seen) == 0 {
		t.Fatal("каталог пуст")
	}
}

func TestIsSuperOnly(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		KeyPageSystemControl: true,
		KeyActionGrantAdmin:  true,
		KeyPageCenter:        false,
		KeyPageAdminUsers:    false,
		"table.kpp4.view":    false,
	}
	for key, want := range cases {
		if got := IsSuperOnly(key); got != want {
			t.Errorf("IsSuperOnly(%q) = %v, ожидалось %v", key, got, want)
		}
	}
}

func TestSuperOnlyKeysPresentInCatalog(t *testing.T) {
	t.Parallel()
	for k := range superOnlyKeys {
		if !IsCatalogKey(k) {
			t.Errorf("super-only ключ %q отсутствует в каталоге", k)
		}
	}
}

func TestIsCatalogKey(t *testing.T) {
	t.Parallel()
	if !IsCatalogKey(KeyPageCenter) {
		t.Error("KeyPageCenter должен быть в каталоге")
	}
	if !IsCatalogKey(KeyDetailFullHistory) {
		t.Error("KeyDetailFullHistory должен быть в каталоге")
	}
	if IsCatalogKey("detail.blacklist") {
		t.Error("detail.blacklist убран из каталога (ЧС гейтится page.admin.blacklist)")
	}
	if IsCatalogKey("nonexistent.key") {
		t.Error("несуществующий ключ не должен быть в каталоге")
	}
	if IsCatalogKey("table.kpp4.view") {
		t.Error("динамический table.* не входит в статический каталог")
	}
}

func TestIsValidKey(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		KeyPageCenter:        true,
		KeyDetailDocuments:   true,
		"table.kpp4.view":    true, // динамический префикс
		"table.post72.entry": true,
		"garbage":            false,
		"page.unknown":       false,
	}
	for key, want := range cases {
		if got := IsValidKey(key); got != want {
			t.Errorf("IsValidKey(%q) = %v, ожидалось %v", key, got, want)
		}
	}
}

// Гигиена каталога (#permission-gating срез 1): «Аналитика» гейтится реально
// enforced ключом page.statistics, а не orphan page.analytics; управление новостями
// живёт в Администрировании (/admin/news), отдельного тумблера news.manage на
// «Обзор и новости» нет.
func TestCatalogHygiene(t *testing.T) {
	t.Parallel()
	if !IsCatalogKey(KeyPageStatistics) {
		t.Error("Аналитика должна гейтиться page.statistics (ключ обязан быть в каталоге)")
	}
	if IsCatalogKey("page.analytics") {
		t.Error("orphan page.analytics не должен остаться в каталоге")
	}
	if IsCatalogKey("news.manage") {
		t.Error("news.manage убран с «Обзор и новости» (управление новостями -- в Администрировании)")
	}
}

func TestAllCatalogKeysMatchesSet(t *testing.T) {
	t.Parallel()
	keys := AllCatalogKeys()
	if len(keys) == 0 {
		t.Fatal("AllCatalogKeys пуст")
	}
	for _, k := range keys {
		if !IsCatalogKey(k) {
			t.Errorf("AllCatalogKeys вернул ключ %q вне каталога", k)
		}
	}
}
