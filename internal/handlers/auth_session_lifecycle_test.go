package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/entityarchive"
	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sessionLoginFallback struct {
	services.AuthService
	revokeError error
}

func (s sessionLoginFallback) Login(context.Context, models.LoginRequest, *services.RequestMeta) (*models.LoginResponse, error) {
	return &models.LoginResponse{RefreshToken: "synthetic-cookie"}, nil
}
func (s sessionLoginFallback) Logout(context.Context, string, models.LogoutRequest, *services.RequestMeta) error {
	return s.revokeError
}

type sessionMaintenanceFallback struct{ services.MaintenanceService }

func (sessionMaintenanceFallback) GetStatusCached(context.Context) *services.MaintenanceStatus {
	return &services.MaintenanceStatus{Enabled: true}
}

func TestSessionLifecycle_MaintenanceFallbackPropagatesRevokeFailureAndClearsCookies(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	cause := errors.New("synthetic revoke failure")
	failure := apperr.New(http.StatusServiceUnavailable, "session unavailable", cause)
	h := handlers.NewAuthHandler(sessionLoginFallback{revokeError: failure}, sessionMaintenanceFallback{}, false, time.Hour)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"synthetic","password":"correctpass"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	err := h.Login(e.NewContext(req, rec))
	require.ErrorIs(t, err, cause)
	var unavailable *apperr.Error
	require.ErrorAs(t, err, &unavailable)
	assert.Equal(t, http.StatusServiceUnavailable, unavailable.Code)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 2)
	for _, cookie := range cookies {
		assert.Equal(t, -1, cookie.MaxAge)
		assert.Empty(t, cookie.Value)
	}
}

// Each wrapper shares the real PostgreSQL pool but has its own callbacks.
// The channel is reached only by an actual users FOR UPDATE query.
func sessionLockBarrier(t *testing.T, base *gorm.DB, after bool) (*gorm.DB, <-chan struct{}, func()) {
	t.Helper()
	pool, err := base.DB()
	require.NoError(t, err)
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	reached, release := make(chan struct{}), make(chan struct{})
	var once, releaseOnce sync.Once
	open := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(open)
	callback := func(tx *gorm.DB) {
		_, locked := tx.Statement.Clauses["FOR"]
		rawUsersLock := strings.Contains(tx.Statement.SQL.String(), "FROM users") && strings.Contains(tx.Statement.SQL.String(), "FOR UPDATE")
		if !(tx.Statement.Table == "users" && locked) && !rawUsersLock {
			return
		}
		once.Do(func() {
			close(reached)
			select {
			case <-release:
			case <-tx.Statement.Context.Done():
				tx.AddError(tx.Statement.Context.Err())
			}
		})
	}
	if after {
		err = db.Callback().Query().After("gorm:query").Register("test:session_lock_barrier", callback)
		require.NoError(t, db.Callback().Row().After("gorm:row").Register("test:session_raw_lock_barrier", callback))
	} else {
		err = db.Callback().Query().Before("gorm:query").Register("test:session_lock_barrier", callback)
		require.NoError(t, db.Callback().Row().Before("gorm:row").Register("test:session_raw_lock_barrier", callback))
	}
	require.NoError(t, err)
	return db, reached, open
}

func TestSessionLifecycle_MaintenanceRefreshBothOrders(t *testing.T) {
	for _, maintenanceFirst := range []bool{false, true} {
		name := "refresh_first"
		if maintenanceFirst {
			name = "maintenance_first"
		}
		t.Run(name, func(t *testing.T) {
			e, db, cleanup := testutil.SetupTestApp(t)
			defer cleanup()
			testutil.CleanDB(t, db)
			raw, owner := loginForRefreshTests(t, e, db, "session_maintenance")
			// The existing super administrator exemption must survive revocation.
			var regular models.User
			require.NoError(t, db.First(&regular, owner).Error)
			testutil.RegisterAdmin(t, e, *regular.OrganizationID, *regular.CompanyID)
			var admin models.User
			require.NoError(t, db.Where("is_super_admin = true").First(&admin).Error)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			firstDB, held, releaseFirst := sessionLockBarrier(t, db, true)
			secondDB, arrived, releaseSecond := sessionLockBarrier(t, db, false)
			first, second := make(chan error, 1), make(chan error, 1)
			var workers sync.WaitGroup
			defer func() { releaseFirst(); releaseSecond(); cancel(); workers.Wait() }()
			start := func(result chan<- error, call func() error) {
				workers.Add(1)
				go func() { defer workers.Done(); result <- call() }()
			}
			maintenance := func(db *gorm.DB) error {
				return services.NewMaintenanceService(db).Enable(ctx, admin.ID, admin.Username, services.MaintenanceParams{Message: "synthetic test"})
			}
			refresh := func(db *gorm.DB) error {
				_, err := newRefreshService(db).RefreshToken(ctx, models.RefreshTokenRequest{RefreshToken: raw}, nil)
				return err
			}
			a, b := refresh, maintenance
			if maintenanceFirst {
				a, b = maintenance, refresh
			}
			start(first, func() error { return a(firstDB) })
			awaitSessionBarrier(t, ctx, held)
			start(second, func() error { return b(secondDB) })
			awaitSessionBarrier(t, ctx, arrived)
			releaseSecond()
			releaseFirst()
			require.NoError(t, <-first)
			if maintenanceFirst {
				var unavailable *apperr.Error
				require.ErrorAs(t, <-second, &unavailable)
				assert.Equal(t, http.StatusServiceUnavailable, unavailable.Code)
			} else {
				require.NoError(t, <-second)
			}
			assert.Zero(t, liveSessionCount(t, db, owner))
			assert.Positive(t, liveSessionCount(t, db, admin.ID))
		})
	}
}

func awaitSessionBarrier(t *testing.T, ctx context.Context, reached <-chan struct{}) {
	t.Helper()
	select {
	case <-reached:
	case <-ctx.Done():
		t.Fatal("session row-lock barrier was not reached: ", ctx.Err())
	}
}

func liveSessionCount(t *testing.T, db *gorm.DB, userID int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.RefreshToken{}).Where("user_id = ? AND is_revoked = false AND expires_at > ?", userID, time.Now().UTC()).Count(&n).Error)
	return n
}

func TestSessionLifecycle_ConcurrentRefreshCreatesOneDescendant(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	raw, owner := loginForRefreshTests(t, e, db, "session_race")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	firstDB, held, releaseFirst := sessionLockBarrier(t, db, true)
	secondDB, arrived, releaseSecond := sessionLockBarrier(t, db, false)
	first, second := make(chan error, 1), make(chan error, 1)
	var workers sync.WaitGroup
	defer func() { releaseFirst(); releaseSecond(); cancel(); workers.Wait() }()
	start := func(result chan<- error, call func() error) {
		workers.Add(1)
		go func() { defer workers.Done(); result <- call() }()
	}
	start(first, func() error {
		_, err := newRefreshService(firstDB).RefreshToken(ctx, models.RefreshTokenRequest{RefreshToken: raw}, nil)
		return err
	})
	awaitSessionBarrier(t, ctx, held)
	start(second, func() error {
		_, err := newRefreshService(secondDB).RefreshToken(ctx, models.RefreshTokenRequest{RefreshToken: raw}, nil)
		return err
	})
	awaitSessionBarrier(t, ctx, arrived)
	releaseSecond()
	releaseFirst()
	require.NoError(t, <-first)
	var denied *apperr.Error
	require.ErrorAs(t, <-second, &denied)
	assert.Equal(t, http.StatusUnauthorized, denied.Code)
	assert.Contains(t, denied.Message, "recently rotated")
	assert.EqualValues(t, 1, liveSessionCount(t, db, owner))
	var total int64
	require.NoError(t, db.Model(&models.RefreshToken{}).Where("user_id = ?", owner).Count(&total).Error)
	assert.EqualValues(t, 2, total)
}

func TestSessionLifecycle_RefreshLogoutBothOrders(t *testing.T) {
	for _, logoutFirst := range []bool{false, true} {
		name := "refresh_first"
		if logoutFirst {
			name = "logout_first"
		}
		t.Run(name, func(t *testing.T) {
			e, db, cleanup := testutil.SetupTestApp(t)
			defer cleanup()
			testutil.CleanDB(t, db)
			raw, owner := loginForRefreshTests(t, e, db, "session_logout")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			firstDB, held, releaseFirst := sessionLockBarrier(t, db, true)
			secondDB, arrived, releaseSecond := sessionLockBarrier(t, db, false)
			first, second := make(chan error, 1), make(chan error, 1)
			var workers sync.WaitGroup
			defer func() { releaseFirst(); releaseSecond(); cancel(); workers.Wait() }()
			start := func(result chan<- error, call func() error) {
				workers.Add(1)
				go func() { defer workers.Done(); result <- call() }()
			}
			logout := func(db *gorm.DB) error {
				return newRefreshService(db).Logout(ctx, "session_logout", models.LogoutRequest{RefreshToken: raw}, nil)
			}
			refresh := func(db *gorm.DB) error {
				_, err := newRefreshService(db).RefreshToken(ctx, models.RefreshTokenRequest{RefreshToken: raw}, nil)
				return err
			}
			a, b := refresh, logout
			if logoutFirst {
				a, b = logout, refresh
			}
			start(first, func() error { return a(firstDB) })
			awaitSessionBarrier(t, ctx, held)
			start(second, func() error { return b(secondDB) })
			awaitSessionBarrier(t, ctx, arrived)
			releaseSecond()
			releaseFirst()
			require.NoError(t, <-first)
			if logoutFirst {
				require.Error(t, <-second)
			} else {
				require.NoError(t, <-second)
			}
			assert.Zero(t, liveSessionCount(t, db, owner))
		})
	}
}

func TestSessionLifecycle_LoginRechecksVerifiedPasswordAfterReset(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	_, owner := loginForRefreshTests(t, e, db, "session_reset")
	actorID := s1FixtureActor(t, db)
	var before models.User
	require.NoError(t, db.First(&before, owner).Error)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	wrapped, beforeLock, release := sessionLockBarrier(t, db, false)
	var worker sync.WaitGroup
	defer func() { release(); cancel(); worker.Wait() }()
	result := make(chan error, 1)
	worker.Add(1)
	go func() {
		defer worker.Done()
		_, err := newRefreshService(wrapped).Login(ctx, models.LoginRequest{Username: "session_reset", Password: "correctpass"}, nil)
		result <- err
	}()
	awaitSessionBarrier(t, ctx, beforeLock) // Argon2 verification has finished.
	// Execute the real administrative reset while Login holds its verified hash
	// but has not yet acquired the user row lock.
	require.NoError(t, services.NewUserService(db, nil).UpdatePassword(ctx, actorID,
		"session_reset", models.UpdatePasswordRequest{Password: selfPassNew}, nil))
	release()
	var denied *apperr.Error
	require.ErrorAs(t, <-result, &denied)
	assert.Equal(t, http.StatusUnauthorized, denied.Code)
	assert.Zero(t, liveSessionCount(t, db, owner))
	var after models.User
	require.NoError(t, db.First(&after, owner).Error)
	assert.NotEqual(t, before.Password, after.Password)
}

func TestSessionLifecycle_LegacyAndForeignCookiesAreIsolated(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	raw, owner := loginForRefreshTests(t, e, db, "session_legacy")
	svc := newRefreshService(db)
	other, err := svc.Login(context.Background(), models.LoginRequest{Username: "session_legacy", Password: "correctpass"}, nil)
	require.NoError(t, err)
	var regular models.User
	require.NoError(t, db.First(&regular, owner).Error)
	testutil.RegisterUser(t, e, "another_owner", "correctpass", 1, *regular.OrganizationID, *regular.CompanyID)
	require.NoError(t, db.Model(&models.RefreshToken{}).Where("user_id = ?", owner).Update("family_id", "").Error)
	_, err = svc.RefreshToken(context.Background(), models.RefreshTokenRequest{RefreshToken: raw}, nil)
	require.Error(t, err)
	require.NoError(t, svc.Logout(context.Background(), "session_legacy", models.LogoutRequest{RefreshToken: raw}, nil))
	assert.EqualValues(t, 1, liveSessionCount(t, db, owner), "empty families must not be grouped")
	require.NoError(t, svc.Logout(context.Background(), "session_legacy", models.LogoutRequest{RefreshToken: raw}, nil))
	require.NoError(t, svc.Logout(context.Background(), "session_legacy", models.LogoutRequest{}, nil))
	assert.EqualValues(t, 1, liveSessionCount(t, db, owner))
	require.NoError(t, svc.Logout(context.Background(), "another_owner", models.LogoutRequest{RefreshToken: other.RefreshToken}, nil))
	assert.EqualValues(t, 1, liveSessionCount(t, db, owner))
}

func TestSessionLifecycle_LogoutRotatedFamilyPreservesOtherDevice(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	raw, owner := loginForRefreshTests(t, e, db, "session_devices")
	svc := newRefreshService(db)
	other, err := svc.Login(context.Background(), models.LoginRequest{Username: "session_devices", Password: "correctpass"}, nil)
	require.NoError(t, err)
	rotated, err := svc.RefreshToken(context.Background(), models.RefreshTokenRequest{RefreshToken: raw}, nil)
	require.NoError(t, err)
	require.NoError(t, svc.Logout(context.Background(), "session_devices", models.LogoutRequest{RefreshToken: raw}, nil))
	assert.EqualValues(t, 1, liveSessionCount(t, db, owner))
	_, err = svc.RefreshToken(context.Background(), models.RefreshTokenRequest{RefreshToken: rotated.RefreshToken}, nil)
	require.Error(t, err)
	_, err = svc.RefreshToken(context.Background(), models.RefreshTokenRequest{RefreshToken: other.RefreshToken}, nil)
	require.NoError(t, err)
}

// Every writer is called through its production entry point. The first service
// holds a real PostgreSQL user lock before the competing service reaches it.
func TestSessionLifecycle_RefreshRevocationWritersBothOrders(t *testing.T) {
	for _, kind := range []string{"reset", "rotation", "ban", "archive"} {
		for _, writerFirst := range []bool{false, true} {
			order := "refresh_first"
			if writerFirst {
				order = "writer_first"
			}
			t.Run(kind+"/"+order, func(t *testing.T) {
				e, db, cleanup := testutil.SetupTestApp(t)
				defer cleanup()
				testutil.CleanDB(t, db)
				raw, ownerID := loginForRefreshTests(t, e, db, "session_writer_race")
				actorID := s1FixtureActor(t, db)
				var owner models.User
				require.NoError(t, db.First(&owner, ownerID).Error)
				// RotateOne requires a deliverable address; Enqueue writes only to
				// the local queue and no mail worker is started by this test.
				require.NoError(t, db.Model(&owner).Update("email", "session-race@example.org").Error)
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				firstDB, held, releaseFirst := sessionLockBarrier(t, db, true)
				secondDB, arrived, releaseSecond := sessionLockBarrier(t, db, false)
				var workers sync.WaitGroup
				// Join workers before SetupTestApp releases its database lease,
				// including when a barrier/assertion fails.
				defer func() { releaseFirst(); releaseSecond(); cancel(); workers.Wait() }()
				writerDB, refreshDB := secondDB, firstDB
				if writerFirst {
					writerDB, refreshDB = firstDB, secondDB
				}
				var write func() error
				switch kind {
				case "reset":
					userService := services.NewUserService(writerDB, nil)
					write = func() error {
						return userService.UpdatePassword(ctx, actorID, owner.Username, models.UpdatePasswordRequest{Password: selfPassNew}, nil)
					}
				case "rotation":
					rotation, _ := rotationEnv(t, writerDB)
					write = func() error { return rotation.RotateOne(ctx, owner.Username, actorID) }
				case "ban":
					ban := services.NewUserBanService(writerDB, services.NewPermissionResolver(writerDB), nil, services.NewAuditRecorder(writerDB))
					write = func() error { return ban.Ban(ctx, ownerID, actorID, "synthetic race") }
				case "archive":
					recorder := services.NewAuditRecorder(writerDB)
					write = func() error {
						_, err := entityarchive.Retire(ctx, writerDB, recorder, entityarchive.TypeOrganization, *owner.OrganizationID, &actorID, true)
						return err
					}
				}
				var child string
				refreshService := newRefreshService(refreshDB)
				refresh := func() error {
					response, err := refreshService.RefreshToken(ctx, models.RefreshTokenRequest{RefreshToken: raw}, nil)
					if response != nil {
						child = response.RefreshToken
					}
					return err
				}
				firstCall, secondCall := refresh, write
				if writerFirst {
					firstCall, secondCall = write, refresh
				}
				first, second := make(chan error, 1), make(chan error, 1)
				workers.Add(1)
				go func() { defer workers.Done(); first <- firstCall() }()
				awaitSessionBarrier(t, ctx, held)
				workers.Add(1)
				go func() { defer workers.Done(); second <- secondCall() }()
				awaitSessionBarrier(t, ctx, arrived)
				releaseSecond()
				releaseFirst()
				require.NoError(t, <-first)
				if writerFirst {
					require.Error(t, <-second)
					assert.Empty(t, child, "refresh after a committed writer cannot create a descendant")
				} else {
					require.NoError(t, <-second)
					require.NotEmpty(t, child)
				}
				workers.Wait()
				var after models.User
				require.NoError(t, db.First(&after, ownerID).Error)
				switch kind {
				case "reset", "rotation":
					assert.NotEqual(t, owner.Password, after.Password)
				case "ban":
					assert.True(t, after.IsBanned)
				case "archive":
					assert.False(t, after.IsActive)
				}
				assert.Zero(t, liveSessionCount(t, db, ownerID), "the committed writer must revoke every family descendant")
				var rows []models.RefreshToken
				require.NoError(t, db.Where("user_id = ?", ownerID).Find(&rows).Error)
				expected := 2
				if writerFirst {
					expected = 1
				}
				require.Len(t, rows, expected)
				for _, row := range rows {
					assert.True(t, row.IsRevoked)
					require.NotNil(t, row.RevokedAt)
				}
				for _, token := range []string{raw, child} {
					if token == "" {
						continue
					}
					_, err := newRefreshService(db).RefreshToken(ctx, models.RefreshTokenRequest{RefreshToken: token}, nil)
					require.Error(t, err, "no issued family member may extend after the writer commit")
				}
				assert.Zero(t, liveSessionCount(t, db, ownerID))
			})
		}
	}
}
