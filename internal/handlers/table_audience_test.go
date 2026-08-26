package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

// TestTableAudience_MirrorsTableViewPermission защищает контракт аудитории
// real-time сигнала проходной (#840): TableAudience обязан зеркалить authoritative-
// гейт видимости таблицы (право table.<name>.view, по которому фронт показывает
// таблицу), а не подмножество ролей. Проверяем через реальный резолвер: юзер с
// правом и super попадают, без права - нет, banned и неактивный исключены.
func TestTableAudience_MirrorsTableViewPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	displayName := "КПП RT"
	table := models.SystemTable{Name: "kpp_rt", DisplayName: &displayName, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&table).Error)
	key := "table.kpp_rt.view"

	testutil.RegisterUser(t, e, "rt_granted", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "rt_nogrant", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "rt_banned", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "rt_inactive", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID) // testadmin: is_super_admin

	grantedID := getUserID(t, db, "rt_granted")
	nograntID := getUserID(t, db, "rt_nogrant")
	bannedID := getUserID(t, db, "rt_banned")
	inactiveID := getUserID(t, db, "rt_inactive")
	superID := getUserID(t, db, "testadmin")

	// Право table.<name>.view выдаём троим через явный allow-override. banned и
	// inactive исключаются потом флагами - именно чтобы доказать, что их отсекает
	// НЕ отсутствие права, а флаги (резолвер по бану, SQL по is_active).
	for _, uid := range []int{grantedID, bannedID, inactiveID} {
		require.NoError(t, db.Create(&models.UserPermissionOverride{
			UserID:        uid,
			PermissionKey: key,
			Value:         "allow",
			GrantedAt:     time.Now(),
		}).Error)
	}
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", bannedID).Update("is_banned", true).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", inactiveID).Update("is_active", false).Error)

	resolver := services.NewPermissionResolver(db)
	audience, err := services.TableAudience(context.Background(), db, resolver, table.ID)
	require.NoError(t, err)

	assert.Contains(t, audience, grantedID, "юзер с правом table.<name>.view должен быть в аудитории")
	assert.Contains(t, audience, superID, "super-admin видит все таблицы -> в аудитории")
	assert.NotContains(t, audience, nograntID, "юзер без права не должен попасть в аудиторию")
	assert.NotContains(t, audience, bannedID, "banned исключается резолвером, несмотря на право")
	assert.NotContains(t, audience, inactiveID, "неактивный (архивный) аккаунт исключается")
}

// TestTableAudience_UnknownTable: несуществующая таблица -> пустая аудитория без
// ошибки (сигнал best-effort, публиковать некому).
func TestTableAudience_UnknownTable(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	resolver := services.NewPermissionResolver(db)
	audience, err := services.TableAudience(context.Background(), db, resolver, 999999)
	require.NoError(t, err)
	assert.Empty(t, audience)
}
