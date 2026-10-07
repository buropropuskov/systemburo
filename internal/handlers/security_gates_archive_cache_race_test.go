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

type archiveCachePauseKey struct{}

// Проверка логической гонки: ранее начатое вычисление не должно заново
// опубликовать старые права в кэше после завершённого архивирования.
func TestSecurityGates_ArchiveInvalidationDuringResolve(t *testing.T) {
	for _, invalidation := range []string{"archive", "all"} {
		t.Run(invalidation, func(t *testing.T) {
			w := newSecurityGateWorld(t)
			resolver := services.NewPermissionResolver(w.db)
			w.gates.Users.SetPermissionResolver(resolver)
			loaded, resume := make(chan struct{}), make(chan struct{})
			var loadedOnce, resumeOnce sync.Once
			release := func() { resumeOnce.Do(func() { close(resume) }) }
			const callback = "security_archive_resolver_pause"
			require.NoError(t, w.db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
				if tx.Statement.Table == "users" && tx.Statement.Context.Value(archiveCachePauseKey{}) == true {
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
			// Администратор получает эффективные права без отдельных грантов.
			require.NoError(t, w.db.Exec("UPDATE users SET is_admin = true WHERE id = ?", w.userID).Error)
			finished := make(chan error, 1)
			go func() {
				_, err := resolver.Resolve(context.WithValue(context.Background(), archiveCachePauseKey{}, true), w.userID)
				finished <- err
			}()
			select {
			case <-loaded:
			case <-time.After(5 * time.Second):
				t.Fatal("вычисление прав не достигло чтения пользователя")
			}
			if invalidation == "archive" {
				w.change(t, "archive")
			} else {
				require.NoError(t, w.db.Exec("UPDATE users SET is_admin = false WHERE id = ?", w.userID).Error)
				resolver.InvalidateAll()
			}
			release()
			select {
			case err := <-finished:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("вычисление прав не завершилось")
			}
			// Это новый запрос, начатый после возврата штатного archive и Resolve.
			set, err := resolver.Resolve(context.Background(), w.userID)
			require.NoError(t, err)
			require.False(t, set.Has("page.admin.file_archive"), "старое вычисление вернуло архивному пользователю административные права через кэш")
		})
	}
}
