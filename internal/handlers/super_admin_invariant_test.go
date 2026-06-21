package handlers_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestEnforceSingleSuperAdmin_KeepsOnlyCanonical проверяет инвариант "ровно один
// супер-админ": миграция оставляет супером только системный аккаунт
// (username='buropropuskov'), а лишних разжалует в обычные администраторы.
// Выполняется в транзакции с откатом -- глобальный UPDATE миграции не
// персистится и не задевает параллельные пакеты на общей тест-БД (урок #706).
func TestEnforceSingleSuperAdmin_KeepsOnlyCanonical(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	rollback := errors.New("rollback")
	err := db.Transaction(func(tx *gorm.DB) error {
		// Возможный закоммиченный остаток системного аккаунта убираем в рамках
		// транзакции (откатится), чтобы uniqueIndex по username не помешал INSERT.
		if err := tx.Where("username = ?", "buropropuskov").Delete(&models.User{}).Error; err != nil {
			return err
		}

		canon := models.User{Username: "buropropuskov", TypeID: 6, IsSuperAdmin: true}
		extra1 := models.User{Username: uniq("extra-super"), TypeID: 6, IsSuperAdmin: true}
		extra2 := models.User{Username: uniq("extra-super"), TypeID: 6, IsSuperAdmin: true}
		require.NoError(t, tx.Create(&canon).Error)
		require.NoError(t, tx.Create(&extra1).Error)
		require.NoError(t, tx.Create(&extra2).Error)

		require.NoError(t, database.EnforceSingleSuperAdmin(tx))

		// Канонический остаётся супером и получает имя пустого системного аккаунта.
		var got models.User
		require.NoError(t, tx.First(&got, canon.ID).Error)
		assert.True(t, got.IsSuperAdmin, "канонический аккаунт остаётся супер-админом")
		require.NotNil(t, got.LastName)
		assert.Equal(t, "Администратор", *got.LastName)

		// Лишние супера разжалованы в обычных администраторов (доступ сохранён).
		for _, id := range []int{extra1.ID, extra2.ID} {
			var u models.User
			require.NoError(t, tx.First(&u, id).Error)
			assert.False(t, u.IsSuperAdmin, "лишний супер снят")
			assert.True(t, u.IsAdmin, "снятый супер сохраняет admin-доступ")
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// TestBanSuperAdmin_Forbidden: супер-админа нельзя заблокировать -> 403.
func TestBanSuperAdmin_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// Цель -- ещё один супер-админ (type 6 -> IsSuperAdmin=true).
	testutil.RegisterUser(t, e, "supertarget", "password123", 6, td.OrgID, td.CompanyID)
	var target models.User
	require.NoError(t, db.Where("username = ?", "supertarget").First(&target).Error)

	rec := testutil.POST(t, e, fmt.Sprintf("/users/%d/ban", target.ID), `{}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// TestArchiveSuperAdmin_Forbidden: супер-админа нельзя архивировать (soft-delete) -> 403.
func TestArchiveSuperAdmin_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	testutil.RegisterUser(t, e, "supertarget", "password123", 6, td.OrgID, td.CompanyID)

	rec := testutil.DELETE(t, e, "/users/supertarget", h)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

// TestCreateUser_CannotSetSuperAdmin: API создания пользователя не позволяет
// выставить is_super_admin/is_admin (полей нет в RegisterRequest) -- второго
// супер-админа через API не создать.
func TestCreateUser_CannotSetSuperAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// Пытаемся "протащить" флаги через тело запроса -- они игнорируются биндингом.
	body := fmt.Sprintf(
		`{"username":"sneaky","password":"password123","is_super_admin":true,"is_admin":true,"organization_id":%d}`,
		td.OrgID,
	)
	rec := testutil.POST(t, e, "/users", body, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var created models.User
	require.NoError(t, db.Where("username = ?", "sneaky").First(&created).Error)
	assert.False(t, created.IsSuperAdmin, "API не должен позволять выставить is_super_admin")
	assert.False(t, created.IsAdmin, "API не должен позволять выставить is_admin при создании")
}
