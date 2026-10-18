package handlers_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func s4WriterToken(t *testing.T, db *gorm.DB, owner int, raw, family string, now time.Time, revoked bool) models.RefreshToken {
	t.Helper()
	hash := sha256.Sum256([]byte(raw))
	token := models.RefreshToken{UserID: owner, FamilyID: family,
		TokenHash: fmt.Sprintf("sha256:%x", hash), ExpiresAt: now.Add(48 * time.Hour),
		CreatedAt: now, IsRevoked: revoked}
	if revoked {
		token.RevokedAt = &now
	}
	require.NoError(t, db.Create(&token).Error)
	return token
}

func s4WriterFailRevoke(t *testing.T, db *gorm.DB, userID int, deferred bool) {
	t.Helper()
	require.NoError(t, db.Exec(`CREATE FUNCTION s4_writer_fail_revoke() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.user_id = TG_ARGV[0]::integer AND NEW.is_revoked THEN
				RAISE EXCEPTION 'synthetic revoke failure';
			END IF;
			RETURN NEW;
		END $$`).Error)
	t.Cleanup(func() {
		assert.NoError(t, db.Exec("DROP TRIGGER IF EXISTS s4_writer_fail_revoke ON refresh_tokens").Error)
		assert.NoError(t, db.Exec("DROP FUNCTION IF EXISTS s4_writer_fail_revoke()").Error)
	})
	trigger := "CREATE TRIGGER s4_writer_fail_revoke BEFORE UPDATE ON refresh_tokens FOR EACH ROW EXECUTE FUNCTION s4_writer_fail_revoke('%d')"
	if deferred {
		trigger = "CREATE CONSTRAINT TRIGGER s4_writer_fail_revoke AFTER UPDATE ON refresh_tokens DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION s4_writer_fail_revoke('%d')"
	}
	require.NoError(t, db.Exec(fmt.Sprintf(trigger, userID)).Error)
}

func TestSessionWriters_OwnPasswordFamily(t *testing.T) {
	for _, cookieKind := range []string{"live", "foreign", "revoked", "expired", "legacy", "missing"} {
		t.Run(cookieKind, func(t *testing.T) {
			_, db, cleanup := testutil.SetupTestApp(t)
			defer cleanup()
			testutil.CleanDB(t, db)
			td := testutil.SeedTestData(t, db)
			owner := s1PolicyUser(t, db, td, "s4_writer_owner", false, false)
			other := s1PolicyUser(t, db, td, "s4_writer_other", false, false)
			now := time.Now().UTC()
			const currentFamily = "d4cbdcf8-52dd-45b1-b149-bae7cf2ff651"
			current := s4WriterToken(t, db, owner.ID, "current-cookie", currentFamily, now, false)
			// More than one active row in a family must survive together.
			sibling := s4WriterToken(t, db, owner.ID, "current-sibling", currentFamily, now, false)
			second := s4WriterToken(t, db, owner.ID, "second-cookie", "d4cbdcf8-52dd-45b1-b149-bae7cf2ff652", now, false)
			foreign := s4WriterToken(t, db, other.ID, "foreign-cookie", currentFamily, now, false)
			s4WriterToken(t, db, owner.ID, "revoked-cookie", currentFamily, now, true)
			expired := s4WriterToken(t, db, owner.ID, "expired-cookie", currentFamily, now, false)
			require.NoError(t, db.Model(&expired).Update("expires_at", now.Add(-time.Hour)).Error)
			s4WriterToken(t, db, owner.ID, "legacy-cookie", "", now, false)
			cookie := cookieKind + "-cookie"
			if cookieKind == "live" {
				cookie = "current-cookie"
			}
			require.NoError(t, services.NewUserService(db, nil).ChangeOwnPassword(context.Background(), owner.ID,
				models.ChangeOwnPasswordRequest{CurrentPassword: selfPassOld, NewPassword: selfPassNew}, nil, cookie))
			for _, token := range []models.RefreshToken{current, sibling, second, foreign} {
				var after models.RefreshToken
				require.NoError(t, db.First(&after, token.ID).Error)
				wantRevoked := token.UserID == owner.ID && (cookieKind != "live" || token.ID == second.ID)
				assert.Equal(t, wantRevoked, after.IsRevoked, "token %d, cookie %s", token.ID, cookieKind)
				if wantRevoked {
					require.NotNil(t, after.RevokedAt)
				}
			}
		})
	}
}

func TestSessionWriters_AdminPasswordCannotKeepSuppliedFamily(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	owner := s1PolicyUser(t, db, td, "s4_reset_owner", false, false)
	token := s4WriterToken(t, db, owner.ID, "reset-cookie", "d4cbdcf8-52dd-45b1-b149-bae7cf2ff654", time.Now().UTC(), false)
	require.NoError(t, services.NewUserService(db, nil).UpdatePasswordKeepingSession(context.Background(),
		s1FixtureActor(t, db), owner.Username, models.UpdatePasswordRequest{Password: selfPassNew}, nil, "reset-cookie"))
	var after models.RefreshToken
	require.NoError(t, db.First(&after, token.ID).Error)
	assert.True(t, after.IsRevoked)
	require.NotNil(t, after.RevokedAt)
}

// A deferred trigger fails at COMMIT, after the password, history and mail writes.
// The immediate variant fails at revocation. Both must leave the same old state.
func TestSessionWriters_PasswordRollbackOnRevokeOrCommitFailure(t *testing.T) {
	for _, scenario := range []struct {
		deferred bool
		rotation bool
	}{{false, false}, {true, false}, {false, true}, {true, true}} {
		t.Run(fmt.Sprintf("deferred_%t_rotation_%t", scenario.deferred, scenario.rotation), func(t *testing.T) {
			_, db, cleanup := testutil.SetupTestApp(t)
			defer cleanup()
			testutil.CleanDB(t, db)
			td := testutil.SeedTestData(t, db)
			owner := s1PolicyUser(t, db, td, "s4_rollback_owner", false, false)
			actorID := s1FixtureActor(t, db)
			token := s4WriterToken(t, db, owner.ID, "rollback-cookie", "d4cbdcf8-52dd-45b1-b149-bae7cf2ff653", time.Now().UTC(), false)
			s4WriterFailRevoke(t, db, owner.ID, scenario.deferred)
			var err error
			if scenario.rotation {
				rotation, _ := rotationEnv(t, db)
				err = rotation.RotateOne(context.Background(), owner.Username, actorID)
			} else {
				err = onboardingUserService(t, db, true).UpdatePassword(context.Background(), actorID, owner.Username,
					models.UpdatePasswordRequest{Password: selfPassNew}, nil)
			}
			s1Status(t, err, http.StatusServiceUnavailable)
			s1Unchanged(t, db, owner)
			var after models.RefreshToken
			require.NoError(t, db.First(&after, token.ID).Error)
			assert.False(t, after.IsRevoked)
			assert.Nil(t, after.RevokedAt)
			var count int64
			require.NoError(t, db.Model(&models.AuthEvent{}).Where("user_id = ? AND event_type = ? AND success = true", owner.ID, models.AuthEventPasswordChanged).Count(&count).Error)
			assert.Zero(t, count)
		})
	}
}

func TestSessionWriters_StateRollbackOnRevokeOrCommitFailure(t *testing.T) {
	for _, operation := range []string{"ban", "archive", "maintenance"} {
		for _, deferred := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_deferred_%t", operation, deferred), func(t *testing.T) {
				_, db, cleanup := testutil.SetupTestApp(t)
				defer cleanup()
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)
				owner := s1PolicyUser(t, db, td, "s4_state_owner", false, false)
				actor := s1PolicyUser(t, db, td, "s4_state_actor", true, true)
				token := s4WriterToken(t, db, owner.ID, "state-cookie", "d4cbdcf8-52dd-45b1-b149-bae7cf2ff655", time.Now().UTC(), false)
				s4WriterFailRevoke(t, db, owner.ID, deferred)
				maintenance := services.NewMaintenanceService(db)
				beforeStatus, err := maintenance.GetStatus(context.Background())
				require.NoError(t, err)
				require.False(t, beforeStatus.Enabled)
				var settingsBefore []models.SystemSetting
				require.NoError(t, db.Order("key").Find(&settingsBefore).Error)
				switch operation {
				case "ban":
					ban := services.NewUserBanService(db, services.NewPermissionResolver(db), nil, services.NewAuditRecorder(db))
					err = ban.Ban(context.Background(), owner.ID, actor.ID, "synthetic reason")
				case "archive":
					err = services.NewUserService(db, nil).Delete(context.Background(), actor.ID, owner.Username)
				case "maintenance":
					err = maintenance.Enable(context.Background(), actor.ID, actor.Username, services.MaintenanceParams{Message: "synthetic window"})
				}
				s1Status(t, err, http.StatusServiceUnavailable)
				after := fetchUser(t, db, owner.Username)
				assert.Equal(t, owner.IsActive, after.IsActive)
				assert.Equal(t, owner.IsBanned, after.IsBanned)
				assert.Equal(t, owner.BannedAt, after.BannedAt)
				assert.Equal(t, owner.BannedBy, after.BannedBy)
				assert.Equal(t, owner.BanReason, after.BanReason)
				var tokenAfter models.RefreshToken
				require.NoError(t, db.First(&tokenAfter, token.ID).Error)
				assert.False(t, tokenAfter.IsRevoked)
				assert.Nil(t, tokenAfter.RevokedAt)
				var settingsAfter []models.SystemSetting
				require.NoError(t, db.Order("key").Find(&settingsAfter).Error)
				assert.Equal(t, settingsBefore, settingsAfter)
				var count int64
				require.NoError(t, db.Model(&models.AuthEvent{}).Where("event_type = ? AND success = true", "maintenance_enabled").Count(&count).Error)
				assert.Zero(t, count)
				require.NoError(t, db.Model(&models.AuditLog{}).Where("entity_type = ? AND entity_id = ?", models.AuditEntityUser, owner.ID).Count(&count).Error)
				assert.Zero(t, count)
			})
		}
	}
}

func TestSessionWriters_ArchiveLookupAndUpdateFailuresKeepCause(t *testing.T) {
	for _, operation := range []string{"lookup", "update"} {
		t.Run(operation, func(t *testing.T) {
			_, db, cleanup := testutil.SetupTestApp(t)
			defer cleanup()
			testutil.CleanDB(t, db)
			td := testutil.SeedTestData(t, db)
			owner := s1PolicyUser(t, db, td, "s4_archive_error_owner", false, false)
			actorID := s1FixtureActor(t, db)
			token := s4WriterToken(t, db, owner.ID, "archive-error-cookie", "d4cbdcf8-52dd-45b1-b149-bae7cf2ff656", time.Now().UTC(), false)
			injected := errors.New("synthetic archive database error")
			callback := "s4_writer_archive_" + operation
			failUser := func(tx *gorm.DB) {
				if tx.Statement.Table == "users" {
					tx.AddError(injected)
				}
			}
			var remove func() error
			if operation == "lookup" {
				require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callback, failUser))
				remove = func() error { return db.Callback().Query().Remove(callback) }
			} else {
				require.NoError(t, db.Callback().Update().Before("gorm:update").Register(callback, failUser))
				remove = func() error { return db.Callback().Update().Remove(callback) }
			}
			removed := false
			t.Cleanup(func() {
				if !removed {
					assert.NoError(t, remove())
				}
			})
			err := services.NewUserService(db, nil).Delete(context.Background(), actorID, owner.Username)
			// Restore callbacks before assertions that themselves read users.
			require.NoError(t, remove())
			removed = true
			s1Status(t, err, http.StatusServiceUnavailable)
			require.ErrorIs(t, err, injected)
			after := fetchUser(t, db, owner.Username)
			assert.Equal(t, owner.IsActive, after.IsActive)
			var tokenAfter models.RefreshToken
			require.NoError(t, db.First(&tokenAfter, token.ID).Error)
			assert.False(t, tokenAfter.IsRevoked)
			assert.Nil(t, tokenAfter.RevokedAt)
		})
	}
}
