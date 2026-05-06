package handlers_test

// Resolver интеграционные тесты. Лежат в handlers_test, чтобы идти в одной
// тестовой бинарке с другими handler tests -- это даёт sequential выполнение
// (go test -p 4 параллелит между пакетами, но не внутри пакета). На общей
// auto_registry_test БД параллельный CleanDB из соседнего пакета сносил наши
// записи и ломал FK на user_types. Здесь race снят пока тесты сосуществуют.

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"gorm.io/gorm"
)

// uniqCounter гарантирует уникальность кодов/usernames между параллельными
// тестами этого пакета и параллельными запусками с другими пакетами на одной БД.
var uniqCounter int64

func uniq(prefix string) string {
	n := atomic.AddInt64(&uniqCounter, 1)
	return fmt.Sprintf("%s-%d", prefix, n)
}

// createOwnUserType создаёт изолированный user_type, чтобы не race-иться
// с CleanDB других тестов, которые чистят таблицу user_types.
func createOwnUserType(t *testing.T, db *gorm.DB) models.UserType {
	t.Helper()
	ut := models.UserType{Name: uniq("test_type"), Code: uniq("tt")}
	if err := db.Create(&ut).Error; err != nil {
		t.Fatalf("create user_type: %v", err)
	}
	return ut
}

// seedRoleGroupUser создаёт изолированный set записей с уникальными
// идентификаторами. CleanDB не вызывается -- мы работаем поверх существующих
// данных, чтобы не race-иться с другими пакетами.
// Возвращает userID, group1ID, group2ID + cleanup-функцию.
func seedRoleGroupUser(t *testing.T, db *gorm.DB) (int, int, int, func()) {
	t.Helper()

	ut := createOwnUserType(t, db)

	g1 := models.PermissionGroup{Name: uniq("Базовый доступ")}
	if err := db.Create(&g1).Error; err != nil {
		t.Fatalf("create group1: %v", err)
	}
	if err := db.Create(&[]models.PermissionGroupGrant{
		{GroupID: g1.ID, PermissionKey: "page.center", Value: "allow"},
		{GroupID: g1.ID, PermissionKey: "page.cars", Value: "allow"},
	}).Error; err != nil {
		t.Fatalf("create grants1: %v", err)
	}

	g2 := models.PermissionGroup{Name: uniq("Удаление сотрудников")}
	if err := db.Create(&g2).Error; err != nil {
		t.Fatalf("create group2: %v", err)
	}
	if err := db.Create(&[]models.PermissionGroupGrant{
		{GroupID: g2.ID, PermissionKey: "action.delete.employee", Value: "allow"},
	}).Error; err != nil {
		t.Fatalf("create grants2: %v", err)
	}

	role := models.Role{Code: uniq("tenant"), Name: "Арендатор"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := db.Create(&models.RoleDefaultGroup{RoleID: role.ID, GroupID: g1.ID}).Error; err != nil {
		t.Fatalf("create role default: %v", err)
	}

	roleID := role.ID
	user := models.User{Username: uniq("tenant"), Password: "x", TypeID: ut.ID, RoleID: &roleID}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	cleanup := func() {
		// Удаляем строго свои записи в FK-safe порядке.
		db.Where("user_id = ?", user.ID).Delete(&models.UserPermissionOverride{})
		db.Where("user_id = ?", user.ID).Delete(&models.UserGroup{})
		db.Delete(&user)
		db.Where("role_id = ?", role.ID).Delete(&models.RoleDefaultGroup{})
		db.Delete(&role)
		db.Where("group_id IN ?", []int{g1.ID, g2.ID}).Delete(&models.PermissionGroupGrant{})
		db.Delete(&models.PermissionGroup{}, []int{g1.ID, g2.ID})
		db.Delete(&ut)
	}

	return user.ID, g1.ID, g2.ID, cleanup
}

func TestPermissionResolver_RoleDefaultGroupOnly(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)

	userID, _, _, cleanup := seedRoleGroupUser(t, db)
	defer cleanup()

	resolver := services.NewPermissionResolver(db)
	set, err := resolver.Resolve(context.Background(), userID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !set.Has("page.center") {
		t.Errorf("expected page.center allowed via role default group")
	}
	if !set.Has("page.cars") {
		t.Errorf("expected page.cars allowed via role default group")
	}
	if set.Has("action.delete.employee") {
		t.Errorf("expected delete denied (not in default group)")
	}
}

func TestPermissionResolver_AdditionalUserGroupAdds(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)

	userID, _, g2ID, cleanup := seedRoleGroupUser(t, db)
	defer cleanup()
	if err := db.Create(&models.UserGroup{UserID: userID, GroupID: g2ID}).Error; err != nil {
		t.Fatalf("assign g2: %v", err)
	}

	resolver := services.NewPermissionResolver(db)
	set, err := resolver.Resolve(context.Background(), userID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !set.Has("page.center") || !set.Has("action.delete.employee") {
		t.Errorf("expected union of role + extra group, got allows=%v", set.Keys())
	}
}

func TestPermissionResolver_OverrideDenyWins(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)

	userID, _, _, cleanup := seedRoleGroupUser(t, db)
	defer cleanup()
	if err := db.Create(&models.UserPermissionOverride{
		UserID: userID, PermissionKey: "page.center", Value: "deny",
	}).Error; err != nil {
		t.Fatalf("override: %v", err)
	}

	resolver := services.NewPermissionResolver(db)
	set, err := resolver.Resolve(context.Background(), userID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if set.Has("page.center") {
		t.Errorf("expected deny override to win over role allow")
	}
	if !set.Has("page.cars") {
		t.Errorf("expected page.cars still allowed (only page.center overridden)")
	}
}

func TestPermissionResolver_SuperAdminBypass(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)

	ut := createOwnUserType(t, db)
	defer db.Delete(&ut)

	user := models.User{Username: uniq("super"), Password: "x", TypeID: ut.ID, IsSuperAdmin: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create super: %v", err)
	}
	defer db.Delete(&user)

	resolver := services.NewPermissionResolver(db)
	set, err := resolver.Resolve(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !set.IsSuperAdmin() {
		t.Errorf("expected super-admin set")
	}
	if !set.Has("any.random.key") {
		t.Errorf("super-admin should have any key")
	}
}

func TestPermissionResolver_CacheInvalidation(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)

	userID, _, g2ID, cleanup := seedRoleGroupUser(t, db)
	defer cleanup()
	resolver := services.NewPermissionResolver(db)

	set, _ := resolver.Resolve(context.Background(), userID)
	if set.Has("action.delete.employee") {
		t.Fatalf("baseline expected no delete permission")
	}

	if err := db.Create(&models.UserGroup{UserID: userID, GroupID: g2ID}).Error; err != nil {
		t.Fatalf("assign: %v", err)
	}

	set, _ = resolver.Resolve(context.Background(), userID)
	if set.Has("action.delete.employee") {
		t.Errorf("expected stale cache to hide new permission")
	}

	resolver.Invalidate(userID)
	set, _ = resolver.Resolve(context.Background(), userID)
	if !set.Has("action.delete.employee") {
		t.Errorf("expected fresh cache to show new permission")
	}
}
