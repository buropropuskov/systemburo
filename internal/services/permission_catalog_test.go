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
	if !IsCatalogKey(KeyDetailBlacklist) {
		t.Error("KeyDetailBlacklist должен быть в каталоге")
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
