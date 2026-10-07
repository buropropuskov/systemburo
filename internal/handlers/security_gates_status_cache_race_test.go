package handlers_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type statusCachePauseKey struct{}

func TestSecurityGates_StatusInvalidationDuringRead(t *testing.T) {
	for _, state := range []string{"ban", "archive"} {
		t.Run(state, func(t *testing.T) {
			w := newSecurityGateWorld(t)
			cache := w.gates.BanCache
			cache.Invalidate(w.userID)
			loaded, resume := make(chan struct{}), make(chan struct{})
			var loadedOnce, resumeOnce sync.Once
			release := func() { resumeOnce.Do(func() { close(resume) }) }
			const callback = "security_status_cache_pause"
			require.NoError(t, w.db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Table == "users" && tx.Statement.Context.Value(statusCachePauseKey{}) == true {
					loadedOnce.Do(func() {
						close(loaded)
						<-resume
					})
				}
			}))
			t.Cleanup(func() {
				release()
				_ = w.db.Callback().Query().Remove(callback)
			})
			finished := make(chan error, 1)
			go func() {
				_, _, err := cache.Status(context.WithValue(context.Background(), statusCachePauseKey{}, true), w.userID)
				finished <- err
			}()
			select {
			case <-loaded:
			case <-time.After(5 * time.Second):
				t.Fatal("чтение статуса не достигло SELECT пользователя")
			}
			w.change(t, state)
			release()
			select {
			case err := <-finished:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("чтение статуса не завершилось")
			}
			before := w.snapshot(t)
			rec := securityGateRequest(w.e, "PUT", "/api/users/me/theme", `{"theme":"light"}`, w.user)
			assertSecurityGateDenied(t, rec, state)
			require.Equal(t, before, w.snapshot(t), "запрос после блокировки изменил бизнес-данные")
		})
	}
}
