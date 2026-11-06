package handlers_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type consentParallelKey struct{}

func waitConsentSubjectLock(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.Raw(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock' AND query LIKE '%"users"%' AND pid <> pg_backend_pid())`).Scan(&waiting).Error
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond, "operation did not wait for the subject row")
}

func TestConsentParallel_SubjectOperations(t *testing.T) {
	for _, tc := range []struct {
		first, second string
		active        bool
	}{
		{"edit", "revoke", false}, {"revoke", "edit", false},
		{"grant", "revoke", false}, {"accept", "revoke", false},
		{"revoke", "grant", true}, {"revoke", "accept", true},
	} {
		t.Run(tc.first+"_then_"+tc.second, func(t *testing.T) {
			_, db, cleanup := testutil.SetupTestApp(t)
			defer cleanup()
			testutil.CleanDB(t, db)
			td := testutil.SeedTestData(t, db)
			actor := s1FixtureActor(t, db)
			target := s1PolicyUser(t, db, td, "parallel_subject", false, false)
			grantConsent(t, db, target.Username)
			seedOptionalConsentDocument(t, db)
			settings := services.NewSettingsService(db, &config.Config{})
			shown, err := services.NewPDConsentGateService(services.NewConsentService(db), settings, time.Hour).CurrentRequirement(context.Background())
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ready, resume := make(chan struct{}), make(chan struct{})
			var pauseOnce, releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(resume) }) }
			defer release()
			hook := func(tx *gorm.DB) {
				table := "pd_consents"
				if tc.first == "edit" {
					table = "users"
				}
				if tx.Statement.Table == table && tx.Statement.Context.Value(consentParallelKey{}) == true {
					pauseOnce.Do(func() { close(ready); <-resume })
				}
			}
			require.NoError(t, db.Callback().Update().Before("gorm:update").Register("parallel:pause_update", hook))
			require.NoError(t, db.Callback().Create().Before("gorm:create").Register("parallel:pause_create", hook))
			defer db.Callback().Update().Remove("parallel:pause_update")
			defer db.Callback().Create().Remove("parallel:pause_create")
			invoke := func(ctx context.Context, op string) error {
				svc := services.NewConsentService(db)
				switch op {
				case "edit":
					position := "Updated position"
					return services.NewUserService(db, nil).UpdateInfo(ctx, actor, target.Username, models.UpdateUserInfoRequest{Position: &position})
				case "revoke":
					return svc.Revoke(ctx, target.ID, services.ConsentTypePDProcessing, target.ID)
				case "grant":
					_, err := svc.Grant(ctx, target.ID, models.GrantConsentRequest{ConsentType: services.ConsentTypePDProcessing}, "", "", shown.Version, shown.Hash)
					return err
				default:
					_, _, err := svc.AcceptProcessing(ctx, target.ID, models.PDConsentRevision{Version: shown.Version, Hash: shown.Hash}, "", "", false)
					return err
				}
			}
			first := make(chan error, 1)
			go func() { first <- invoke(context.WithValue(ctx, consentParallelKey{}, true), tc.first) }()
			select {
			case <-ready:
			case <-ctx.Done():
				t.Fatal("first operation did not reach barrier")
			}
			second := make(chan error, 1)
			go func() { second <- invoke(ctx, tc.second) }()
			waitConsentSubjectLock(t, db)
			select {
			case err := <-second:
				t.Fatalf("second operation completed before release: %v", err)
			default:
			}
			release()
			require.NoError(t, <-first)
			if tc.second == "edit" {
				s1Status(t, <-second, 409)
			} else {
				require.NoError(t, <-second)
			}
			active, err := services.NewConsentService(db).HasActive(ctx, target.ID, services.ConsentTypePDProcessing)
			require.NoError(t, err)
			require.Equal(t, tc.active, active)
			var after models.User
			require.NoError(t, db.First(&after, target.ID).Error)
			if tc.first == "edit" {
				require.NotNil(t, after.Position)
				require.Equal(t, "Updated position", *after.Position)
			} else {
				require.Equal(t, target.Position, after.Position)
			}
		})
	}
}

func TestConsentParallel_CancelAndOtherSubject(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	actor := s1FixtureActor(t, db)
	target := s1PolicyUser(t, db, td, "locked_subject", false, false)
	grantConsent(t, db, target.Username)
	other := s1PolicyUser(t, db, td, "other_subject", false, false)
	grantConsent(t, db, other.Username)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ready, resume := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(resume) }) }
	defer release()
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("parallel:pause", func(tx *gorm.DB) {
		if tx.Statement.Table == "users" && tx.Statement.Context.Value(consentParallelKey{}) == true {
			close(ready)
			<-resume
		}
	}))
	defer db.Callback().Update().Remove("parallel:pause")
	first := make(chan error, 1)
	position := "Saved before withdrawal"
	go func() {
		first <- services.NewUserService(db, nil).UpdateInfo(context.WithValue(ctx, consentParallelKey{}, true), actor, target.Username, models.UpdateUserInfoRequest{Position: &position})
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("edit did not reach barrier")
	}
	// A separate subject does not share the locked row or wait for the editor.
	require.NoError(t, services.NewConsentService(db).Revoke(ctx, other.ID, services.ConsentTypePDProcessing, other.ID))
	waitingCtx, abort := context.WithCancel(ctx)
	defer abort()
	second := make(chan error, 1)
	go func() {
		second <- services.NewConsentService(db).Revoke(waitingCtx, target.ID, services.ConsentTypePDProcessing, target.ID)
	}()
	waitConsentSubjectLock(t, db)
	abort()
	require.Error(t, <-second)
	active, err := services.NewConsentService(db).HasActive(ctx, target.ID, services.ConsentTypePDProcessing)
	require.NoError(t, err)
	require.True(t, active)
	var auditCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).Where("entity_id = ? AND action = ?", target.ID, models.UserActionConsentRevoked).Count(&auditCount).Error)
	require.Zero(t, auditCount, "canceled withdrawal must not create an audit event")
	release()
	require.NoError(t, <-first)
	require.NoError(t, services.NewConsentService(db).Revoke(ctx, target.ID, services.ConsentTypePDProcessing, target.ID))
	active, err = services.NewConsentService(db).HasActive(ctx, target.ID, services.ConsentTypePDProcessing)
	require.NoError(t, err)
	require.False(t, active)
}
