package services_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"gorm.io/gorm"
)

// seedRoleGroupUser -- хелпер для подготовки минимального state.
// Возвращает userID, group1ID, group2ID.
func seedRoleGroupUser(t *testing.T, db *gorm.DB) (int, int, int) {
	t.Helper()

	g1 := models.PermissionGroup{Name: "Базовый доступ"}
	if err := db.Create(&g1).Error; err != nil {
		t.Fatalf("create group1: %v", err)
	}
	if err := db.Create(&[]models.PermissionGroupGrant{
		{GroupID: g1.ID, PermissionKey: "page.center", Value: "allow"},
		{GroupID: g1.ID, PermissionKey: "page.cars", Value: "allow"},
	}).Error; err != nil {
		t.Fatalf("create grants1: %v", err)
	}

	g2 := models.PermissionGroup{Name: "Удаление сотрудников"}
	if err := db.Create(&g2).Error; err != nil {
		t.Fatalf("create group2: %v", err)
	}
	if err := db.Create(&[]models.PermissionGroupGrant{
		{GroupID: g2.ID, PermissionKey: "action.delete.employee", Value: "allow"},
	}).Error; err != nil {
		t.Fatalf("create grants2: %v", err)
	}

	role := models.Role{Code: "tenant", Name: "Арендатор"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	if err := db.Create(&models.RoleDefaultGroup{RoleID: role.ID, GroupID: g1.ID}).Error; err != nil {
		t.Fatalf("create role default: %v", err)
	}

	roleID := role.ID
	user := models.User{Username: "tenant1", Password: "x", TypeID: 1, RoleID: &roleID}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	return user.ID, g1.ID, g2.ID
}

func TestPermissionResolver_RoleDefaultGroupOnly(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, _ := seedRoleGroupUser(t, db)

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
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, g2ID := seedRoleGroupUser(t, db)
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
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, _ := seedRoleGroupUser(t, db)
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
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	user := models.User{Username: "super", Password: "x", TypeID: 1, IsSuperAdmin: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create super: %v", err)
	}

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
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, g2ID := seedRoleGroupUser(t, db)
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
