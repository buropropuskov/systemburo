package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/config"
	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Existing service tests also use a real active administrator, rather than a
// nonexistent actor or ID 0. Reuse within a test keeps the initiating ID stable.
func s1FixtureActor(t *testing.T, db *gorm.DB) int {
	t.Helper()
	var actor models.User
	require.NoError(t, db.Where("username = ?", "s1_fixture_admin").Find(&actor).Error)
	if actor.ID == 0 {
		actor = models.User{Username: "s1_fixture_admin", Password: services.HashPassword(selfPassOld), TypeID: 1, IsAdmin: true, IsActive: true}
		require.NoError(t, db.Create(&actor).Error)
	}
	return actor.ID
}

func s1PolicyUser(t *testing.T, db *gorm.DB, td testutil.TestData, name string, admin, super bool) models.User {
	t.Helper()
	u := models.User{Username: name, Password: services.HashPassword(selfPassOld), TypeID: 1,
		OrganizationID: &td.OrgID, CompanyID: &td.CompanyID, IsActive: true,
		IsAdmin: admin, IsSuperAdmin: super, Email: strPtr(name + "@example.org")}
	require.NoError(t, db.Create(&u).Error)
	return fetchUser(t, db, name)
}

func s1Status(t *testing.T, err error, want int) {
	t.Helper()
	require.Error(t, err)
	var domain *apperr.Error
	require.True(t, errors.As(err, &domain), "expected domain error, got %T: %v", err, err)
	assert.Equal(t, want, domain.Code)
	assert.NotContains(t, err.Error(), selfPassNew)
}

func s1Unchanged(t *testing.T, db *gorm.DB, before models.User) {
	t.Helper()
	after := fetchUser(t, db, before.Username)
	assert.Equal(t, before.Password, after.Password)
	assert.Equal(t, before.Email, after.Email)
	assert.Equal(t, before.Position, after.Position)
	assert.Equal(t, before.MustChangePassword, after.MustChangePassword)
	var count int64
	require.NoError(t, db.Model(&models.EmailMessage{}).Where("user_id = ?", before.ID).Count(&count).Error)
	assert.Zero(t, count, "no password delivery after rejection")
	require.NoError(t, db.Model(&models.UsedPassword{}).Where("user_id = ?", before.ID).Count(&count).Error)
	assert.Zero(t, count, "no password history mutation after rejection")
}

func TestAdminTargetPolicy_PasswordRankAndCaller(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor := s1PolicyUser(t, db, td, "s1_admin", true, false)
	super := s1PolicyUser(t, db, td, "s1_super", false, true)
	svc := onboardingUserService(t, db, true)
	for _, tc := range []struct {
		name                           string
		admin, super, archived, banned bool
	}{
		{name: "peer", admin: true}, {name: "super", super: true},
		{name: "archivedsuper", super: true, archived: true}, {name: "bannedsuper", super: true, banned: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := s1PolicyUser(t, db, td, "s1_target_"+tc.name, tc.admin, tc.super)
			if tc.archived {
				require.NoError(t, db.Model(&models.User{}).Where("id = ?", target.ID).Update("is_active", false).Error)
			}
			if tc.banned {
				require.NoError(t, db.Model(&models.User{}).Where("id = ?", target.ID).Update("is_banned", true).Error)
			}
			s1Status(t, svc.UpdatePassword(context.Background(), actor.ID, target.Username, models.UpdatePasswordRequest{Password: selfPassNew}, nil), http.StatusForbidden)
			s1Unchanged(t, db, target)
			// A different super can manage protected targets, including archived rank.
			require.NoError(t, svc.UpdatePassword(context.Background(), super.ID, target.Username, models.UpdatePasswordRequest{Password: selfPassNew}, nil))
			assert.NotEqual(t, target.Password, fetchUser(t, db, target.Username).Password)
			require.Len(t, userLetters(t, db, target.ID), 1)
		})
	}
	ordinary := s1PolicyUser(t, db, td, "s1_ordinary", false, false)
	require.NoError(t, svc.UpdatePassword(context.Background(), actor.ID, ordinary.Username, models.UpdatePasswordRequest{Password: selfPassNew}, nil))
	require.Len(t, userLetters(t, db, ordinary.ID), 1)

	for _, kind := range []string{"banned", "deny", "no_permission"} {
		t.Run(kind, func(t *testing.T) {
			caller := s1PolicyUser(t, db, td, "s1_caller_"+kind, kind != "no_permission", false)
			target := s1PolicyUser(t, db, td, "s1_caller_target_"+kind, false, false)
			if kind == "banned" {
				require.NoError(t, db.Model(&caller).Update("is_banned", true).Error)
			}
			if kind == "deny" {
				testutil.DenyPermission(t, caller.ID, services.KeyPageAdminUsers)
			}
			s1Status(t, svc.UpdatePassword(context.Background(), caller.ID, target.Username, models.UpdatePasswordRequest{Password: selfPassNew}, nil), http.StatusForbidden)
			s1Unchanged(t, db, target)
		})
	}
	s1Status(t, svc.UpdatePassword(context.Background(), actor.ID, "s1_missing", models.UpdatePasswordRequest{Password: selfPassNew}, nil), http.StatusNotFound)
}

func TestAdminTargetPolicy_SelfPasswordRequiresCurrent(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	svc := onboardingUserService(t, db, true)
	for _, super := range []bool{false, true} {
		name := "s1_self_admin"
		if super {
			name = "s1_self_super"
		}
		user := s1PolicyUser(t, db, td, name, !super, super)
		s1Status(t, svc.UpdatePassword(context.Background(), user.ID, user.Username, models.UpdatePasswordRequest{Password: selfPassNew}, nil), http.StatusForbidden)
		s1Status(t, svc.UpdatePasswordKeepingSession(context.Background(), user.ID, user.Username, models.UpdatePasswordRequest{Password: selfPassNew}, nil, "supplied-cookie"), http.StatusForbidden)
		s1Unchanged(t, db, user)
		require.Error(t, svc.ChangeOwnPassword(context.Background(), user.ID, models.ChangeOwnPasswordRequest{CurrentPassword: "wrong", NewPassword: selfPassNew}, nil, ""))
		s1Unchanged(t, db, user)
		require.NoError(t, svc.ChangeOwnPassword(context.Background(), user.ID, models.ChangeOwnPasswordRequest{CurrentPassword: selfPassOld, NewPassword: selfPassNew}, nil, ""))
		after := fetchUser(t, db, user.Username)
		assert.NotEqual(t, user.Password, after.Password)
		assert.False(t, after.MustChangePassword)
		assert.Empty(t, userLetters(t, db, user.ID))
	}
}

func TestAdminTargetPolicy_EmailAtomicAndUnchanged(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	svc := onboardingUserService(t, db, true)
	actor := s1PolicyUser(t, db, td, "s1_email_actor", true, false)
	super := s1PolicyUser(t, db, td, "s1_email_super", false, true)
	for _, admin := range []bool{false, true} {
		name := "s1_email_target_super"
		if admin {
			name = "s1_email_target_admin"
		}
		target := s1PolicyUser(t, db, td, name, admin, !admin)
		for _, email := range []string{"replaced@example.org", ""} {
			s1Status(t, svc.UpdateInfo(context.Background(), actor.ID, target.Username, models.UpdateUserInfoRequest{Email: &email, Position: strPtr("must not save")}), http.StatusForbidden)
			s1Unchanged(t, db, target)
		}
		require.NoError(t, svc.UpdateInfo(context.Background(), actor.ID, target.Username, models.UpdateUserInfoRequest{Email: strPtr("  " + *target.Email + "  "), Position: strPtr("allowed position")}))
		after := fetchUser(t, db, target.Username)
		assert.Equal(t, target.Email, after.Email)
		assert.Equal(t, "allowed position", *after.Position)
		require.NoError(t, svc.UpdateInfo(context.Background(), super.ID, target.Username, models.UpdateUserInfoRequest{Email: strPtr(name + ".new@example.org")}))
		assert.Equal(t, name+".new@example.org", *fetchUser(t, db, target.Username).Email)
	}
	s1Status(t, svc.UpdateInfo(context.Background(), actor.ID, actor.Username, models.UpdateUserInfoRequest{Email: strPtr("admin.self@example.org")}), http.StatusForbidden)
	require.NoError(t, svc.UpdateInfo(context.Background(), super.ID, super.Username, models.UpdateUserInfoRequest{Email: strPtr("super.self@example.org")}))
	ordinary := s1PolicyUser(t, db, td, "s1_email_granted_self", false, false)
	testutil.GrantPermission(t, ordinary.ID, services.KeyPageAdminUsers)
	require.NoError(t, svc.UpdateInfo(context.Background(), ordinary.ID, ordinary.Username, models.UpdateUserInfoRequest{Email: strPtr("granted.self@example.org")}))
}

func TestAdminTargetPolicy_RotationPartialAndRechecks(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	// One controlled timestamp for all fixtures; this manual flow has no expiry gate.
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	actor := s1PolicyUser(t, db, td, "s1_rot_actor", true, false)
	protected := s1PolicyUser(t, db, td, "s1_rot_super", false, true)
	noMailProtected := s1PolicyUser(t, db, td, "s1_rot_admin_nomail", true, false)
	require.NoError(t, db.Model(&noMailProtected).Update("email", nil).Error)
	noMailProtected = fetchUser(t, db, noMailProtected.Username)
	allowed := mkRotationUser(t, db, td, "s1_rot_allowed", "allowed@example.org", now)
	allowed = fetchUser(t, db, allowed.Username)
	noMail := mkRotationUser(t, db, td, "s1_rot_nomail", "", now)
	svc, _ := rotationEnv(t, db)
	for _, user := range []models.User{actor, protected, noMailProtected} {
		s1Status(t, svc.RotateOne(context.Background(), user.Username, actor.ID), http.StatusForbidden)
		s1Unchanged(t, db, user)
	}
	require.NoError(t, svc.AuthorizeRun(context.Background(), actor.ID))
	testutil.DenyPermission(t, actor.ID, services.KeyActionRotatePasswords)
	_, err := svc.Run(context.Background(), actor.ID)
	s1Status(t, err, http.StatusForbidden)
	s1Unchanged(t, db, allowed)
	require.NoError(t, db.Where("user_id = ? AND permission_key = ?", actor.ID, services.KeyActionRotatePasswords).Delete(&models.UserPermissionOverride{}).Error)
	result, err := svc.Run(context.Background(), actor.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Changed)
	assert.Equal(t, 3, result.Failed)
	assert.Contains(t, result.NoMailLogins, noMail.Username)
	assert.NotContains(t, result.NoMailLogins, noMailProtected.Username)
	assert.GreaterOrEqual(t, result.SkippedNoMail, 1)
	for _, user := range []models.User{actor, protected, noMailProtected} {
		s1Unchanged(t, db, user)
	}
	assert.NotEqual(t, allowed.Password, fetchUser(t, db, allowed.Username).Password)
	require.Len(t, userLetters(t, db, allowed.ID), 1)
}

func TestAdminTargetPolicy_HandlerForbiddenBeforeAccepted(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor := s1PolicyUser(t, db, td, "s1_handler_actor", true, false)
	target := s1PolicyUser(t, db, td, "s1_handler_super", false, true)
	rotation, settings := rotationEnv(t, db)
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodPost, "/users/s1_handler_super/rotate-password", nil), rec)
	c.Set("user_id", actor.ID)
	c.SetParamNames("username")
	c.SetParamValues(target.Username)
	users := handlers.NewUsersHandler(nil)
	users.SetRotationService(rotation)
	s1Status(t, users.RotatePassword(c), http.StatusForbidden)
	s1Unchanged(t, db, target)

	testutil.DenyPermission(t, actor.ID, services.KeyActionRotatePasswords)
	handler := handlers.NewSettingsHandler(settings, nil, 0, nil, nil)
	handler.SetRotationService(rotation)
	handler.SetMailSender(services.NewMailService(db, &config.Config{SMTPHost: "127.0.0.1", SMTPPort: 2525, SMTPFrom: "bureau@example.org"}))
	rec = httptest.NewRecorder()
	c = e.NewContext(httptest.NewRequest(http.MethodPost, "/settings/password-rotation/run", nil), rec)
	c.Set("user_id", actor.ID)
	s1Status(t, handler.RunPasswordRotation(c), http.StatusForbidden)
	assert.NotEqual(t, http.StatusAccepted, rec.Code)
	assert.Empty(t, rec.Body.String(), "no accepted response before actor authorization")
	assert.False(t, rotation.IsRunning())
	assert.Nil(t, rotation.LastResult())
	s1Unchanged(t, db, target)
}

func TestAdminTargetPolicy_TargetRecheckReadError(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor := s1PolicyUser(t, db, td, "s1_read_actor", true, false)
	target := s1PolicyUser(t, db, td, "s1_read_target", false, false)
	svc := onboardingUserService(t, db, true)
	const callback = "s1:test_target_recheck_error"
	reads := 0
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			reads++
			if reads == 2 {
				tx.AddError(errors.New("synthetic user read failure"))
			}
		}
	}))
	t.Cleanup(func() { db.Callback().Query().Remove(callback) })
	err := svc.UpdatePassword(context.Background(), actor.ID, target.Username, models.UpdatePasswordRequest{Password: selfPassNew}, nil)
	require.NoError(t, db.Callback().Query().Remove(callback))
	s1Status(t, err, http.StatusServiceUnavailable)
	assert.Equal(t, 2, reads, "failure must occur during locked target recheck")
	s1Unchanged(t, db, target)
}
