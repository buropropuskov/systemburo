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

var historyGrantKeys = []string{"detail.full_history", "detail.entry_exit_history"}

// TestBaseRole_NoHistoryGrants: базовая роль "Пользователь" не выдаёт detail.full_history
// и detail.entry_exit_history - рядовой юзер не видит "Полную историю"/"Историю проходов"
// (админ/супер видят по флагу adminAll/allowAll, минуя гранты роли). Проверяем и итог
// сида, и что revoke снимает гранты со старой БД, не трогая прочие права базовой роли.
func TestBaseRole_NoHistoryGrants(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	var baseRole models.Role
	require.NoError(t, db.Where("code = ? AND is_system = ?", "user", true).First(&baseRole).Error)

	// После сидирования (seed без ключей + revoke) базовая роль истории не имеет.
	var seeded int64
	require.NoError(t, db.Model(&models.RolePermissionGrant{}).
		Where("role_id = ? AND permission_key IN ?", baseRole.ID, historyGrantKeys).Count(&seeded).Error)
	assert.EqualValues(t, 0, seeded, "после сида базовая роль без грантов истории")

	// Транзакция с откатом: глобальная роль расшарена между тестами (урок #706) -
	// вставленные/удалённые гранты не должны персистить.
	rollback := errors.New("rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		// Симулируем старую БД: возвращаем базовой роли гранты истории.
		for _, k := range historyGrantKeys {
			require.NoError(t, tx.Where("role_id = ? AND permission_key = ?", baseRole.ID, k).
				FirstOrCreate(&models.RolePermissionGrant{RoleID: baseRole.ID, PermissionKey: k, Value: "allow"}).Error)
		}

		require.NoError(t, database.RevokeBaseRoleHistoryGrants(tx))

		var after int64
		require.NoError(t, tx.Model(&models.RolePermissionGrant{}).
			Where("role_id = ? AND permission_key IN ?", baseRole.ID, historyGrantKeys).Count(&after).Error)
		assert.EqualValues(t, 0, after, "revoke снял гранты истории с базовой роли")

		// Прочие гранты базовой роли не тронуты.
		var docs int64
		require.NoError(t, tx.Model(&models.RolePermissionGrant{}).
			Where("role_id = ? AND permission_key = ?", baseRole.ID, "detail.documents").Count(&docs).Error)
		assert.EqualValues(t, 1, docs, "detail.documents у базовой роли остаётся")

		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
