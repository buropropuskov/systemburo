package handlers_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type authIntegrityLoginResult struct {
	response *models.LoginResponse
	err      error
}

func authIntegrityDB(t *testing.T, base *gorm.DB, connection gorm.ConnPool) *gorm.DB {
	t.Helper()
	if connection == nil {
		pool, err := base.DB()
		require.NoError(t, err)
		connection = pool
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: connection}), &gorm.Config{Logger: base.Logger})
	require.NoError(t, err)
	return db
}

func authIntegrityService(db *gorm.DB) services.AuthService {
	return services.NewAuthService(db, testutil.TestJWTSecret, testutil.TestJWTRefreshSecret, 15*time.Minute, 168*time.Hour,
		services.WithAuthNotifications(services.NewNotificationService(db)))
}

func authIntegrityAwait(t *testing.T, channel <-chan struct{}) {
	t.Helper()
	select {
	case <-channel:
	case <-time.After(30 * time.Second):
		t.Fatal("authentication operation did not reach its controlled barrier")
	}
}

func authIntegrityReadUser(t *testing.T, db *gorm.DB, id int) models.User {
	t.Helper()
	var user models.User
	require.NoError(t, db.First(&user, id).Error)
	return user
}

func authIntegrityCount(t *testing.T, db *gorm.DB, table, condition string, args ...interface{}) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Where(condition, args...).Count(&count).Error)
	return count
}

func TestPasswordAdmission_HTTPBusyLeavesCredentialsUntouched(t *testing.T) {
	e, base, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, base)
	td := testutil.SeedTestData(t, base)
	const username = "password_admission_user"
	token := testutil.RegisterAndLogin(t, e, username, selfPassOld, 1, td.OrgID, td.CompanyID)
	var before models.User
	require.NoError(t, base.Where("username = ?", username).First(&before).Error)
	events := authIntegrityCount(t, base, "auth_events", "username = ?", username)
	history := authIntegrityCount(t, base, "used_passwords", "user_id = ?", before.ID)
	sessions := authIntegrityCount(t, base, "refresh_tokens", "user_id = ? AND is_revoked = false", before.ID)
	services.SetArgon2Concurrency(1)
	t.Cleanup(func() { services.SetArgon2Concurrency(0) })

	// Hold the sole slot with an actual login on a separate callback registry.
	// The synthetic account is missing, so releasing the holder creates no session.
	db := authIntegrityDB(t, base, nil)
	ctx, cancel := context.WithCancel(context.Background())
	entered, done := make(chan struct{}), make(chan struct{})
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:hold_password_slot", func(tx *gorm.DB) {
		close(entered)
		<-ctx.Done()
		tx.AddError(ctx.Err())
	}))
	holder := authIntegrityService(db)
	go func() {
		defer close(done)
		_, _ = holder.Login(ctx, models.LoginRequest{Username: "password_admission_holder"}, nil)
	}()
	defer func() { cancel(); authIntegrityAwait(t, done) }()
	authIntegrityAwait(t, entered)

	read := testutil.GET(t, e, "/user-types", nil)
	require.Equal(t, http.StatusOK, read.Code)
	login := testutil.POST(t, e, "/login", `{"username":"password_admission_user","password":"wrong-password"}`, nil)
	require.Equal(t, http.StatusServiceUnavailable, login.Code)
	require.Equal(t, "2", login.Header().Get("Retry-After"))
	change := testutil.PUT(t, e, "/api/users/me/password", `{"current_password":"oldpassword12345","new_password":"newpassword67890"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusServiceUnavailable, change.Code)
	require.Equal(t, "2", change.Header().Get("Retry-After"))
	after := authIntegrityReadUser(t, base, before.ID)
	require.Equal(t, before.Password, after.Password)
	require.Equal(t, before.PasswordChangedAt, after.PasswordChangedAt)
	require.Equal(t, before.FailedLoginCount, after.FailedLoginCount)
	require.Equal(t, events, authIntegrityCount(t, base, "auth_events", "username = ?", username))
	require.Equal(t, history, authIntegrityCount(t, base, "used_passwords", "user_id = ?", before.ID))
	require.Equal(t, sessions, authIntegrityCount(t, base, "refresh_tokens", "user_id = ? AND is_revoked = false", before.ID))

	cancel()
	authIntegrityAwait(t, done)
	rec := testutil.POST(t, e, "/login", `{"username":"password_admission_user","password":"oldpassword12345"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestAuthLockoutIntegrity_ConcurrentWrongAttemptsAndSingleTransition(t *testing.T) {
	_, base, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, base)
	db := authIntegrityDB(t, base, nil)
	td := testutil.SeedTestData(t, db)
	user := mkBareUser(t, db, td, "auth_atomic_wrong", "correct-password123")
	const attempts = 8
	// This test needs all eight snapshots at the barrier before any can proceed.
	// Saturation of the password queue is covered separately.
	services.SetArgon2Concurrency(attempts)
	t.Cleanup(func() { services.SetArgon2Concurrency(0) })
	ready, release := make(chan struct{}, attempts), make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	var initialReads atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := "test:auth_initial_snapshots"
	require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table != "users" || !strings.Contains(tx.Statement.SQL.String(), "username =") {
			return
		}
		if initialReads.Add(1) > attempts {
			return
		}
		ready <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
			tx.AddError(ctx.Err())
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Query().Remove(name)) })
	svc := authIntegrityService(db)
	results := make(chan authIntegrityLoginResult, attempts)
	var workers sync.WaitGroup
	defer func() { releaseOnce.Do(func() { close(release) }); cancel(); workers.Wait() }()
	for i := 0; i < attempts; i++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			response, err := svc.Login(ctx, models.LoginRequest{Username: user.Username, Password: "wrong-password"},
				&services.RequestMeta{IPAddress: fmt.Sprintf("198.51.100.%d", index+1), UserAgent: "integrity-test"})
			results <- authIntegrityLoginResult{response, err}
		}(i)
	}
	// All callers hold the same pre-transition snapshot; atomicity cannot be
	// supplied accidentally by sequential initial reads or an IP-only guard.
	for i := 0; i < attempts; i++ {
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatal("initial reads did not rendezvous")
		}
	}
	releaseOnce.Do(func() { close(release) })
	unauthorized, locked := 0, 0
	for i := 0; i < attempts; i++ {
		result := <-results
		require.Nil(t, result.response)
		var domain *apperr.Error
		require.ErrorAs(t, result.err, &domain)
		switch domain.Code {
		case http.StatusUnauthorized:
			unauthorized++
		case http.StatusTooManyRequests:
			locked++
		default:
			t.Fatalf("unexpected authentication status %d", domain.Code)
		}
	}
	require.Equal(t, 4, unauthorized)
	require.Equal(t, 4, locked)
	current := authIntegrityReadUser(t, db, user.ID)
	require.Zero(t, current.FailedLoginCount)
	require.Equal(t, 1, current.LockoutLevel)
	require.NotNil(t, current.LastFailedLoginAt)
	require.NotNil(t, current.LockedUntil)
	require.Equal(t, time.Minute, current.LockedUntil.Sub(*current.LastFailedLoginAt))
	require.EqualValues(t, 5, authIntegrityCount(t, db, "auth_events", "username = ? AND event_type = ?", user.Username, models.AuthEventLoginFailed), "only the five committed failures are failed-login events")
	require.EqualValues(t, 3, authIntegrityCount(t, db, "auth_events", "username = ? AND event_type = ?", user.Username, models.AuthEventLoginLocked), "the three stale initial snapshots must observe the fresh lock without another penalty")
	require.EqualValues(t, 1, authIntegrityCount(t, db, "auth_events", "username = ? AND event_type = ?", user.Username, models.AuthEventAccountLocked))
	require.EqualValues(t, 1, authIntegrityCount(t, db, "notifications", "user_id = ? AND type = ?", user.ID, services.NotificationTypeLoginBlocked))
}

func TestAuthLockoutIntegrity_HashChangeDoesNotPenalizeStaleVerification(t *testing.T) {
	_, base, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, base)
	td := testutil.SeedTestData(t, base)
	for _, password := range []string{"old-correct-password", "wrong-password"} {
		t.Run(password, func(t *testing.T) {
			db := authIntegrityDB(t, base, nil)
			user := mkBareUser(t, db, td, "stale_"+password, "old-correct-password")
			newHash := services.HashPassword("new-correct-password")
			loaded, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			first := true
			require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:stale_hash", func(tx *gorm.DB) {
				if first && tx.Statement.Table == "users" && strings.Contains(tx.Statement.SQL.String(), "username =") {
					first = false
					close(loaded)
					<-release
				}
			}))
			t.Cleanup(func() { require.NoError(t, db.Callback().Query().Remove("test:stale_hash")) })
			result := make(chan authIntegrityLoginResult, 1)
			workerDone := make(chan struct{})
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			defer func() { once.Do(func() { close(release) }); cancel(); <-workerDone }()
			svc := authIntegrityService(db)
			go func() {
				defer close(workerDone)
				response, err := svc.Login(ctx, models.LoginRequest{Username: user.Username, Password: password}, &services.RequestMeta{IPAddress: "203.0.113.21"})
				result <- authIntegrityLoginResult{response, err}
			}()
			authIntegrityAwait(t, loaded)
			// Change only the password; leave every lockout field untouched.
			require.NoError(t, base.Model(&models.User{}).Where("id = ?", user.ID).Update("password", newHash).Error)
			once.Do(func() { close(release) })
			done := <-result
			require.Nil(t, done.response)
			var domain *apperr.Error
			require.ErrorAs(t, done.err, &domain)
			require.Equal(t, http.StatusUnauthorized, domain.Code)
			current := authIntegrityReadUser(t, base, user.ID)
			require.Equal(t, newHash, current.Password)
			require.Zero(t, current.FailedLoginCount)
			require.Zero(t, current.LockoutLevel)
			require.Nil(t, current.LastFailedLoginAt)
			require.Nil(t, current.LockedUntil)
			require.Zero(t, authIntegrityCount(t, base, "refresh_tokens", "user_id = ?", user.ID))
			require.Zero(t, authIntegrityCount(t, base, "auth_events", "username = ? AND event_type = ?", user.Username, models.AuthEventAccountLocked))
		})
	}
}

func TestAuthLockoutIntegrity_FailedStateWriteCannotAnnounceLock(t *testing.T) {
	_, base, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, base)
	db := authIntegrityDB(t, base, nil)
	td := testutil.SeedTestData(t, db)
	user := mkBareUser(t, db, td, "failed_lock_write", "correct-password")
	// One coherent test point, far from both the five-minute and 24-hour edges.
	point := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]interface{}{"failed_login_count": 4, "last_failed_login_at": point}).Error)
	before := authIntegrityReadUser(t, db, user.ID)
	hits := 0
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:lock_write", func(tx *gorm.DB) {
		if tx.Statement.Table != "users" {
			return
		}
		updates, ok := tx.Statement.Dest.(map[string]interface{})
		if !ok {
			return
		}
		if _, exists := updates["failed_login_count"]; exists {
			hits++
			tx.AddError(errors.New("synthetic lock-state write failure"))
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Update().Remove("test:lock_write")) })
	response, err := authIntegrityService(db).Login(context.Background(), models.LoginRequest{Username: user.Username, Password: "wrong"}, &services.RequestMeta{IPAddress: "203.0.113.22"})
	require.Nil(t, response)
	var domain *apperr.Error
	require.ErrorAs(t, err, &domain)
	require.Equal(t, http.StatusInternalServerError, domain.Code)
	require.Equal(t, 1, hits)
	require.Equal(t, before, authIntegrityReadUser(t, db, user.ID))
	require.Zero(t, authIntegrityCount(t, db, "auth_events", "username = ? AND event_type = ?", user.Username, models.AuthEventAccountLocked))
	require.Zero(t, authIntegrityCount(t, db, "notifications", "user_id = ? AND type = ?", user.ID, services.NotificationTypeLoginBlocked))
}

func TestAuthLockoutIntegrity_UsedPasswordCancellationStopsBeforeWrite(t *testing.T) {
	_, base, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, base)
	db := authIntegrityDB(t, base, nil)
	td := testutil.SeedTestData(t, db)
	user := mkUsedPwdUser(t, db, td, "used_password_cancel", "original-password123")
	actorID := s1FixtureActor(t, db)
	before, history := authIntegrityReadUser(t, db, user.ID), usedPasswordRows(t, db, user.ID)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reads, writes := 0, 0
	require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:cancel_history", func(tx *gorm.DB) {
		if tx.Statement.Table == "used_passwords" {
			reads++
			cancel()
		}
	}))
	require.NoError(t, db.Callback().Update().Before("gorm:update").Register("test:password_write_attempt", func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			writes++
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Query().Remove("test:cancel_history"))
		require.NoError(t, db.Callback().Update().Remove("test:password_write_attempt"))
	})
	err := usedPwdSvc(db).UpdatePassword(ctx, actorID, user.Username, models.UpdatePasswordRequest{Password: "different-password456"}, nil)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, reads)
	require.Zero(t, writes, "cancellation must escape history verification, not continue to a cancelled UPDATE")
	require.Equal(t, before, authIntegrityReadUser(t, db, user.ID))
	require.Equal(t, history, usedPasswordRows(t, base, user.ID))
}

type authIntegrityCommitKey struct{}
type authIntegrityPool struct {
	*sql.DB
	committed, release chan struct{}
	once               sync.Once
}
type authIntegrityTx struct {
	*sql.Tx
	owner *authIntegrityPool
	ctx   context.Context
}

func (p *authIntegrityPool) BeginTx(ctx context.Context, options *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.DB.BeginTx(ctx, options)
	if err != nil {
		return nil, err
	}
	return &authIntegrityTx{tx, p, ctx}, nil
}

func (tx *authIntegrityTx) Commit() error {
	err := tx.Tx.Commit()
	if err == nil && tx.ctx.Value(authIntegrityCommitKey{}) == true {
		tx.owner.once.Do(func() { close(tx.owner.committed) })
		select {
		case <-tx.owner.release:
		case <-tx.ctx.Done():
		}
	}
	return err
}

func TestAuthLockoutIntegrity_PostcommitSuccessAndResetKeepNextFailure(t *testing.T) {
	_, base, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, base)
	td := testutil.SeedTestData(t, base)
	pool, err := base.DB()
	require.NoError(t, err)
	for _, operation := range []string{"success", "reset"} {
		t.Run(operation, func(t *testing.T) {
			wrapped := &authIntegrityPool{DB: pool, committed: make(chan struct{}), release: make(chan struct{})}
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(wrapped.release) })
			db := authIntegrityDB(t, base, wrapped)
			user := mkBareUser(t, db, td, "postcommit_"+operation, "correct-password123")
			svc := authIntegrityService(db)
			meta := &services.RequestMeta{IPAddress: "198.51.100.40"}
			_, err := svc.Login(context.Background(), models.LoginRequest{Username: user.Username, Password: "wrong-before"}, meta)
			require.Error(t, err) // Seed DB and RAM through the same ordinary wrong attempt.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var workers sync.WaitGroup
			defer func() { releaseOnce.Do(func() { close(wrapped.release) }); cancel(); workers.Wait() }()
			firstDone := make(chan error, 1)
			workers.Add(1)
			go func() {
				defer workers.Done()
				marked := context.WithValue(ctx, authIntegrityCommitKey{}, true)
				if operation == "success" {
					_, err := svc.Login(marked, models.LoginRequest{Username: user.Username, Password: "correct-password123"}, meta)
					firstDone <- err
				} else {
					_, err := svc.ResetLoginLockout(marked, user.Username, user.ID)
					firstDone <- err
				}
			}()
			authIntegrityAwait(t, wrapped.committed)
			current := authIntegrityReadUser(t, base, user.ID)
			require.Zero(t, current.FailedLoginCount, "the first real commit happened before its RAM reset was allowed to finish")
			started := make(chan struct{})
			secondDone := make(chan authIntegrityLoginResult, 1)
			workers.Add(1)
			go func() {
				defer workers.Done()
				close(started)
				response, err := svc.Login(ctx, models.LoginRequest{Username: user.Username, Password: "wrong-after"}, meta)
				secondDone <- authIntegrityLoginResult{response, err}
			}()
			authIntegrityAwait(t, started)
			// This proves a postcommit interleaving and its final property, not
			// the instant the second caller reaches the private transition gate.
			releaseOnce.Do(func() { close(wrapped.release) })
			require.NoError(t, <-firstDone)
			second := <-secondDone
			require.Nil(t, second.response)
			var domain *apperr.Error
			require.ErrorAs(t, second.err, &domain)
			require.Equal(t, http.StatusUnauthorized, domain.Code)
			require.Equal(t, "4", domain.Headers["X-Auth-Attempts-Remaining"])
			current = authIntegrityReadUser(t, base, user.ID)
			require.Equal(t, 1, current.FailedLoginCount)
			require.Nil(t, current.LockedUntil)
		})
	}
}
