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

const historyRevokeMarker = "base_role_history_grants_revoked"

func grantHistory(t *testing.T, tx *gorm.DB, roleID int) {
	t.Helper()
	for _, k := range historyGrantKeys {
		require.NoError(t, tx.Where("role_id = ? AND permission_key = ?", roleID, k).
			FirstOrCreate(&models.RolePermissionGrant{RoleID: roleID, PermissionKey: k, Value: "allow"}).Error)
	}
}

func countHistory(t *testing.T, tx *gorm.DB, roleID int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, tx.Model(&models.RolePermissionGrant{}).
		Where("role_id = ? AND permission_key IN ?", roleID, historyGrantKeys).Count(&n).Error)
	return n
}

// TestBaseRole_HistoryRevokeOneTime: базовая роль "Пользователь" по итогу сида не имеет
// прав истории; сама revoke-функция разовая - снимает гранты и ставит маркер, а
// повторный прогон уже НЕ трогает роль (админ может вернуть права истории через UI, и
// они не сбросятся на следующем старте). Админ/супер видят историю по флагу, минуя роль.
func TestBaseRole_HistoryRevokeOneTime(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	var baseRole models.Role
	require.NoError(t, db.Where("code = ? AND is_system = ?", "user", true).First(&baseRole).Error)

	// После сида (seed без ключей + разовый revoke) базовая роль без грантов истории.
	assert.EqualValues(t, 0, countHistory(t, db, baseRole.ID), "после сида базовая роль без грантов истории")

	// Транзакция с откатом: глобальная роль расшарена между тестами (урок #706).
	rollback := errors.New("rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		// Симулируем "ещё не снимали": убираем маркер и возвращаем базовой роли гранты.
		require.NoError(t, tx.Where("key = ?", historyRevokeMarker).Delete(&models.SystemSetting{}).Error)
		grantHistory(t, tx, baseRole.ID)

		// Первый прогон: снимает гранты и ставит маркер разовости.
		require.NoError(t, database.RevokeBaseRoleHistoryGrants(tx))
		assert.EqualValues(t, 0, countHistory(t, tx, baseRole.ID), "revoke снял гранты истории с базовой роли")
		var marker int64
		require.NoError(t, tx.Model(&models.SystemSetting{}).Where("key = ?", historyRevokeMarker).Count(&marker).Error)
		assert.EqualValues(t, 1, marker, "маркер разовости поставлен")

		// detail.documents (прочий грант базовой роли) не тронут.
		var docs int64
		require.NoError(t, tx.Model(&models.RolePermissionGrant{}).
			Where("role_id = ? AND permission_key = ?", baseRole.ID, "detail.documents").Count(&docs).Error)
		assert.EqualValues(t, 1, docs, "detail.documents остаётся")

		// Разовость: админ вернул гранты роли - повторный прогон их НЕ сбрасывает.
		grantHistory(t, tx, baseRole.ID)
		require.NoError(t, database.RevokeBaseRoleHistoryGrants(tx))
		assert.EqualValues(t, 2, countHistory(t, tx, baseRole.ID), "повторный revoke не сбрасывает гранты (разовость)")

		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
