package handlers_test

import (
	"errors"
	"testing"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestBackfillBaseRole_AssignsToRolelessNormalUsers: миграция выдаёт базовую роль
// обычным пользователям без роли (role_id IS NULL), супер-админа не трогает.
// В транзакции с откатом - глобальный UPDATE не персистится (урок #706).
func TestBackfillBaseRole_AssignsToRolelessNormalUsers(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	rollback := errors.New("rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		var baseRole models.Role
		require.NoError(t, tx.Where("code = ?", "user").First(&baseRole).Error)

		normal := models.User{Username: uniq("norole-normal"), TypeID: 1}
		super := models.User{Username: uniq("norole-super"), TypeID: 6, IsSuperAdmin: true}
		require.NoError(t, tx.Create(&normal).Error)
		require.NoError(t, tx.Create(&super).Error)
		require.Nil(t, normal.RoleID, "предусловие: обычный юзер без роли")

		require.NoError(t, database.BackfillBaseRole(tx))

		var gotNormal, gotSuper models.User
		require.NoError(t, tx.First(&gotNormal, normal.ID).Error)
		require.NoError(t, tx.First(&gotSuper, super.ID).Error)

		require.NotNil(t, gotNormal.RoleID, "обычному юзеру без роли назначена базовая роль")
		assert.Equal(t, baseRole.ID, *gotNormal.RoleID)
		assert.Nil(t, gotSuper.RoleID, "супер-админа бэкфилл не трогает")

		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
