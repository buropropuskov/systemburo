package handlers_test

import (
	"context"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestSecurityGates_ArchivePermissions(t *testing.T) {
	for _, admin := range []bool{false, true} {
		role := "regular"
		if admin {
			role = "admin"
		}
		for _, warm := range []bool{false, true} {
			cache := "cold"
			if warm {
				cache = "warm"
			}
			t.Run(role+"/"+cache, func(t *testing.T) {
				w := newSecurityGateWorld(t)
				testutil.GrantPermission(t, w.userID, "page.admin.file_archive")
				require.NoError(t, w.db.Model(&models.User{}).Where("id = ?", w.userID).Update("is_admin", admin).Error)
				if warm {
					rec := testutil.GET(t, w.e, "/file-archive/settings", testutil.AuthHeader(w.user))
					require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				}
				w.change(t, "archive")
				rec := testutil.GET(t, w.e, "/users/me", testutil.AuthHeader(w.user))
				require.Equal(t, http.StatusOK, rec.Code, "read-only cabinet: %s", rec.Body.String())
				rec = testutil.GET(t, w.e, "/file-archive/settings", testutil.AuthHeader(w.user))
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				// Новый resolver проверяет предикат без возможности попадания в старый кэш.
				set, err := services.NewPermissionResolver(w.db).Resolve(context.Background(), w.userID)
				require.NoError(t, err)
				require.Empty(t, set.Keys())
				require.False(t, set.Has("page.admin.file_archive"))
				require.False(t, set.IsAdmin())
				require.False(t, set.IsSuperAdmin())
				require.False(t, set.IsBanned(), "архив не должен превращаться в бан")
				require.NoError(t, w.gates.Users.Restore(context.Background(), w.adminID, "security_gate_user"))
				rec = testutil.GET(t, w.e, "/file-archive/settings", testutil.AuthHeader(w.user))
				require.Equal(t, http.StatusOK, rec.Code, "restore must invalidate empty permission cache: %s", rec.Body.String())
			})
		}
	}
}
