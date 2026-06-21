package services

import "testing"

// Юнит-тесты чистой логики PermissionSet (без БД).

func TestPermissionSetHasSuper(t *testing.T) {
	t.Parallel()
	s := PermissionSet{allowAll: true}
	if !s.Has(KeyPageCenter) {
		t.Error("super должен иметь обычное право")
	}
	if !s.Has(KeyPageSystemControl) {
		t.Error("super должен иметь super-only право (техработы)")
	}
	if !s.Has(KeyActionGrantAdmin) {
		t.Error("super должен иметь выдачу админки")
	}
	if s.Mode() != "super" {
		t.Errorf("Mode = %q, ожидалось super", s.Mode())
	}
}

func TestPermissionSetHasAdmin(t *testing.T) {
	t.Parallel()
	s := PermissionSet{adminAll: true, denies: map[string]struct{}{KeyPageAdminUsers: {}}}
	if !s.Has(KeyPageCenter) {
		t.Error("admin должен иметь обычное право")
	}
	if s.Has(KeyPageAdminUsers) {
		t.Error("admin НЕ должен иметь право из личных deny")
	}
	if s.Has(KeyPageSystemControl) {
		t.Error("admin НЕ должен иметь super-only (техработы)")
	}
	if s.Has(KeyActionGrantAdmin) {
		t.Error("admin НЕ должен выдавать админки (super-only)")
	}
	if s.Mode() != "admin" {
		t.Errorf("Mode = %q, ожидалось admin", s.Mode())
	}
	if len(s.Denies()) != 1 || s.Denies()[0] != KeyPageAdminUsers {
		t.Errorf("Denies = %v, ожидался [%s]", s.Denies(), KeyPageAdminUsers)
	}
}

func TestPermissionSetHasNormal(t *testing.T) {
	t.Parallel()
	s := PermissionSet{allows: map[string]string{
		KeyPageCenter:        SourceRole,
		KeyPageCars:          SourceGroup,
		KeyPageSystemControl: SourceOverride, // даже если как-то попало -- super-only режется
	}}
	if !s.Has(KeyPageCenter) {
		t.Error("normal должен иметь выданное право")
	}
	if s.Has(KeyPageEmployees) {
		t.Error("normal НЕ должен иметь невыданное право")
	}
	if s.Has(KeyPageSystemControl) {
		t.Error("normal НЕ должен иметь super-only даже при allow в наборе")
	}
	if s.Source(KeyPageCenter) != SourceRole {
		t.Errorf("Source(center) = %q, ожидалось role", s.Source(KeyPageCenter))
	}
	if s.Source(KeyPageCars) != SourceGroup {
		t.Errorf("Source(cars) = %q, ожидалось group", s.Source(KeyPageCars))
	}
	if s.Mode() != "normal" {
		t.Errorf("Mode = %q, ожидалось normal", s.Mode())
	}
}

func TestPermissionSetBanned(t *testing.T) {
	t.Parallel()
	s := PermissionSet{banned: true, banReason: "нарушение"}
	if s.Has(KeyPageCenter) || s.Has(KeyPagePersonal) {
		t.Error("забаненный не имеет прав")
	}
	if !s.IsBanned() {
		t.Error("IsBanned должен быть true")
	}
	if s.BanReason() != "нарушение" {
		t.Errorf("BanReason = %q", s.BanReason())
	}
	if s.Mode() != "banned" {
		t.Errorf("Mode = %q, ожидалось banned", s.Mode())
	}
}

func TestPermissionSetKeys(t *testing.T) {
	t.Parallel()
	// super/admin не перечисляют ключи
	super := PermissionSet{allowAll: true}
	if super.Keys() != nil {
		t.Error("super.Keys должен быть nil")
	}
	admin := PermissionSet{adminAll: true}
	if admin.Keys() != nil {
		t.Error("admin.Keys должен быть nil")
	}
	normal := PermissionSet{allows: map[string]string{KeyPageCars: SourceRole, KeyPageCenter: SourceRole}}
	keys := normal.Keys()
	if len(keys) != 2 {
		t.Fatalf("Keys len = %d, ожидалось 2", len(keys))
	}
	// отсортированы
	if keys[0] != KeyPageCars || keys[1] != KeyPageCenter {
		t.Errorf("Keys не отсортированы: %v", keys)
	}
}
