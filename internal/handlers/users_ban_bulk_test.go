package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Групповая блокировка/разблокировка на уровне сервиса (лёгкий тест без CleanDB/Seed
// - пакет handlers на грани CI-таймаута под -race). Реюзает единожды засеянную БД,
// создаёт целевых пользователей через setupMWUser и чистит их за собой.
func TestUserBanService_BulkBanUnban(t *testing.T) {
	_, db, _ := testutil.SetupTestApp(t)
	resolver := services.NewPermissionResolver(db)
	banSvc := services.NewUserBanService(db, resolver, nil, services.NewAuditRecorder(db))
	ctx := context.Background()

	actorID, _, ca := setupMWUser(t, db, true, false)
	defer ca()
	t1, _, c1 := setupMWUser(t, db, false, false)
	defer c1()
	t2, _, c2 := setupMWUser(t, db, false, false)
	defer c2()
	superID, _, cs := setupMWUser(t, db, true, false) // isSuperAdmin=true -> нельзя банить
	defer cs()

	uname := func(id int) string {
		var u models.User
		require.NoError(t, db.Select("username").First(&u, id).Error)
		return u.Username
	}
	banned := func(id int) bool {
		var u models.User
		require.NoError(t, db.Select("is_banned").First(&u, id).Error)
		return u.IsBanned
	}
	reasonOf := func(id int) *string {
		var u models.User
		require.NoError(t, db.Select("ban_reason").First(&u, id).Error)
		return u.BanReason
	}
	n1, n2, actorName, superName := uname(t1), uname(t2), uname(actorID), uname(superID)

	t.Run("ban полный успех с причиной", func(t *testing.T) {
		res, err := banSvc.BulkBan(ctx, actorID, []string{n1, n2}, "нарушение")
		require.NoError(t, err)
		assert.Equal(t, 2, res.SuccessCount)
		assert.Equal(t, 0, res.ErrorCount)
		assert.True(t, banned(t1))
		assert.True(t, banned(t2))
		require.NotNil(t, reasonOf(t1))
		assert.Equal(t, "нарушение", *reasonOf(t1))
	})

	t.Run("unban полный успех очищает причину; дедуп", func(t *testing.T) {
		res, err := banSvc.BulkUnban(ctx, actorID, []string{n1, n1})
		require.NoError(t, err)
		assert.Equal(t, 1, res.SuccessCount, "дубли username дедуплицируются")
		assert.False(t, banned(t1))
		assert.Nil(t, reasonOf(t1))
	})

	t.Run("самобан актора -> в errors", func(t *testing.T) {
		res, err := banSvc.BulkBan(ctx, actorID, []string{n2, actorName}, "")
		require.NoError(t, err)
		assert.Equal(t, 1, res.SuccessCount)
		require.Len(t, res.Errors, 1)
		assert.Equal(t, actorName, res.Errors[0].Name)
		assert.Contains(t, res.Errors[0].Error, "самого себя")
		banSvc.BulkUnban(ctx, actorID, []string{n2}) //nolint:errcheck // откат для чистоты
	})

	t.Run("супер-админ -> в errors, не валит пачку", func(t *testing.T) {
		res, err := banSvc.BulkBan(ctx, actorID, []string{n2, superName}, "")
		require.NoError(t, err)
		assert.Equal(t, 1, res.SuccessCount)
		require.Len(t, res.Errors, 1)
		assert.Equal(t, superName, res.Errors[0].Name)
		assert.Contains(t, res.Errors[0].Error, "супер-администратор")
		banSvc.BulkUnban(ctx, actorID, []string{n2}) //nolint:errcheck
	})

	t.Run("несуществующий username -> в errors", func(t *testing.T) {
		res, err := banSvc.BulkBan(ctx, actorID, []string{n1, "nouser_zzz_9999"}, "")
		require.NoError(t, err)
		assert.Equal(t, 1, res.SuccessCount)
		assert.Equal(t, 1, res.ErrorCount)
		require.Len(t, res.Errors, 1)
		assert.Equal(t, "nouser_zzz_9999", res.Errors[0].Name)
		banSvc.BulkUnban(ctx, actorID, []string{n1}) //nolint:errcheck
	})

	t.Run("пустой список -> пустой результат", func(t *testing.T) {
		res, err := banSvc.BulkBan(ctx, actorID, []string{}, "")
		require.NoError(t, err)
		assert.Equal(t, 0, res.SuccessCount)
		assert.Equal(t, 0, res.ErrorCount)
	})
}
