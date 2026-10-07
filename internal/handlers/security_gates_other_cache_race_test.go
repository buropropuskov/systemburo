package handlers_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type otherCachePauseKey struct{}

func TestSecurityGates_OtherInvalidationDuringRead(t *testing.T) {
	for _, kind := range []string{"password", "maintenance"} {
		t.Run(kind, func(t *testing.T) {
			w := newSecurityGateWorld(t)
			password := services.NewPasswordChangeGateService(w.db, time.Hour)
			w.gates.Maintenance.InvalidateCache()
			loaded, resume, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			var loadedOnce, releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(resume) }) }
			const callback = "security_other_cache_pause"
			pause := func(tx *gorm.DB) {
				if tx.Statement.Context.Value(otherCachePauseKey{}) == true {
					loadedOnce.Do(func() { close(loaded); <-resume })
				}
			}
			if kind == "password" {
				require.NoError(t, w.db.Callback().Row().After("gorm:row").Register(callback, pause))
			} else {
				require.NoError(t, w.db.Callback().Query().After("gorm:query").Register(callback, pause))
			}
			t.Cleanup(func() {
				release()
				if kind == "password" {
					_ = w.db.Callback().Row().Remove(callback)
				} else {
					_ = w.db.Callback().Query().Remove(callback)
				}
			})
			go func() {
				ctx := context.WithValue(context.Background(), otherCachePauseKey{}, true)
				if kind == "password" {
					_, err := password.Required(ctx, w.userID)
					finished <- err
				} else {
					w.gates.Maintenance.GetStatusCached(ctx)
					finished <- nil
				}
			}()
			select {
			case <-loaded:
			case <-time.After(5 * time.Second):
				t.Fatal("чтение не достигло контрольной точки")
			}
			if kind == "password" {
				w.change(t, "password")
				password.Invalidate(w.userID)
			} else {
				w.change(t, "maintenance")
			}
			release()
			select {
			case err := <-finished:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("чтение не завершилось")
			}
			if kind == "password" {
				required, err := password.Required(context.Background(), w.userID)
				require.NoError(t, err)
				require.True(t, required, "старое чтение восстановило отсутствие требования пароля")
			} else {
				rec := securityGateRequest(w.e, "PUT", "/api/users/me/theme", `{"theme":"light"}`, w.user)
				assertSecurityGateDenied(t, rec, "maintenance")
			}
		})
	}
}
