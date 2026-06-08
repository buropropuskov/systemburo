package handlers_test

import (
	"context"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newPersonBlacklistService(db *gorm.DB) services.PersonBlacklistService {
	return services.NewPersonBlacklistService(db, services.NewPersonBlacklistHistoryService(db))
}

// TestPersonBlacklist_Lifecycle: create/check/строгое-ФИО/дубль/archive/restore без employees.
func TestPersonBlacklist_Lifecycle(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	withMiddle, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Петров", FirstName: "Пётр", MiddleName: "Петрович", Reason: "тест",
	}, userID)
	require.NoError(t, err)
	require.NotZero(t, withMiddle.ID)

	noMiddle, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Сидоров", FirstName: "Сидор", Reason: "тест2",
	}, userID)
	require.NoError(t, err)
	require.Nil(t, noMiddle.MiddleName, "пустое отчество должно храниться как NULL")

	checks := []struct {
		name                string
		last, first, middle string
		wantBlock           bool
	}{
		{"полное совпадение с отчеством", "Петров", "Пётр", "Петрович", true},
		{"регистр/пробелы", " петров ", "ПЁТР", "петрович", true},
		{"есть отчество в ЧС - без отчества не матчит", "Петров", "Пётр", "", false},
		{"есть отчество в ЧС - другое отчество не матчит", "Петров", "Пётр", "Другое", false},
		{"нет отчества в ЧС - без отчества матчит", "Сидоров", "Сидор", "", true},
		{"нет отчества в ЧС - с отчеством не матчит", "Сидоров", "Сидор", "Иванович", false},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			res, err := svc.Check(ctx, tc.last, tc.first, tc.middle)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBlock, res.IsBlacklisted)
		})
	}

	// Дубль (даже в другом регистре/с пробелами) блокируется partial unique index-ом.
	_, err = svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "ПЕТРОВ", FirstName: "пётр", MiddleName: " Петрович ", Reason: "дубль",
	}, userID)
	assertHTTPStatus(t, err, 409)

	// После снятия можно добавить заново.
	require.NoError(t, svc.Archive(ctx, withMiddle.ID, userID))
	res, err := svc.Check(ctx, "Петров", "Пётр", "Петрович")
	require.NoError(t, err)
	assert.False(t, res.IsBlacklisted)

	again, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Петров", FirstName: "Пётр", MiddleName: "Петрович", Reason: "снова",
	}, userID)
	require.NoError(t, err)
	assert.NotEqual(t, withMiddle.ID, again.ID)

	// Restore архивной при активном дубле -> 409.
	assertHTTPStatus(t, svc.Restore(ctx, withMiddle.ID, userID), 409)
}

// TestPersonBlacklist_CascadeDeactivatesActiveEmployee: добавление в ЧС гасит активного сотрудника.
func TestPersonBlacklist_CascadeDeactivatesActiveEmployee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pblcasc1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "pblcasc1")
	appID, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	var before models.Employee
	require.NoError(t, db.First(&before, empID).Error)
	require.NotNil(t, before.Status)
	require.Equal(t, 1, *before.Status, "сотрудник должен быть активен до ЧС")

	svc := newPersonBlacklistService(db)
	entry, err := svc.Create(context.Background(), models.CreatePersonBlacklistRequest{
		LastName: "Ivanov", FirstName: "Ivan", MiddleName: "Ivanovich", Reason: "запрет",
	}, userID)
	require.NoError(t, err)

	var after models.Employee
	require.NoError(t, db.First(&after, empID).Error)
	require.NotNil(t, after.Status)
	assert.Equal(t, 0, *after.Status, "сотрудник должен деактивироваться")
	assert.NotNil(t, after.DateDeleted, "date_deleted должен проставиться")

	var empHistCount int64
	db.Model(&models.EmployeeHistory{}).Where("employee_id = ? AND action_type = ?", empID, "blacklisted").Count(&empHistCount)
	assert.Equal(t, int64(1), empHistCount, "должна быть запись employees_history blacklisted")

	var blHistCount int64
	db.Model(&models.PersonBlacklistHistory{}).Where("entity_id = ? AND action_type = ?", entry.ID, models.BlacklistActionCreated).Count(&blHistCount)
	assert.Equal(t, int64(1), blHistCount)
}

// TestPersonBlacklist_UnblacklistRestoresActiveApplicationEmployee: снятие возвращает status=1.
func TestPersonBlacklist_UnblacklistRestoresActiveApplicationEmployee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pblcasc2", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "pblcasc2")
	appID, _, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Ivanov", FirstName: "Ivan", MiddleName: "Ivanovich", Reason: "проверка",
	}, userID)
	require.NoError(t, err)

	var blacklisted models.Employee
	require.NoError(t, db.First(&blacklisted, empID).Error)
	require.Equal(t, 0, *blacklisted.Status)

	require.NoError(t, svc.Archive(ctx, entry.ID, userID))

	var restored models.Employee
	require.NoError(t, db.First(&restored, empID).Error)
	require.NotNil(t, restored.Status)
	assert.Equal(t, 1, *restored.Status, "сотрудник с активной заявкой должен вернуться в status=1")
	assert.Nil(t, restored.DateDeleted, "date_deleted должен очиститься")

	var empHistCount int64
	db.Model(&models.EmployeeHistory{}).Where("employee_id = ? AND action_type = ?", empID, "unblacklisted").Count(&empHistCount)
	assert.Equal(t, int64(1), empHistCount)
}

// TestPersonBlacklist_UnblacklistSkipsExpiredPass: просроченный пропуск не возрождается
// (дата берётся из attachments.entry_date_to - у employees своего поля нет).
func TestPersonBlacklist_UnblacklistSkipsExpiredPass(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pblexp1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "pblexp1")
	appID, attID, empID := seedEmployeeViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Ivanov", FirstName: "Ivan", MiddleName: "Ivanovich", Reason: "проверка",
	}, userID)
	require.NoError(t, err)

	require.NoError(t, db.Exec("UPDATE attachments SET entry_date_to = ? WHERE id = ?", "2000-01-01", attID).Error)

	require.NoError(t, svc.Archive(ctx, entry.ID, userID))

	var after models.Employee
	require.NoError(t, db.First(&after, empID).Error)
	require.NotNil(t, after.Status)
	assert.Equal(t, 0, *after.Status, "сотрудник с истёкшим пропуском не должен возрождаться")
}

// TestPersonBlacklist_Purge покрывает hard-delete архивной записи + запрет на активную.
func TestPersonBlacklist_Purge(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	svc := newPersonBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Удаляев", FirstName: "Удал", MiddleName: "Удалович", Reason: "к удалению",
	}, userID)
	require.NoError(t, err)

	t.Run("активную удалять нельзя - 400", func(t *testing.T) {
		assertHTTPStatus(t, svc.Purge(ctx, entry.ID), http.StatusBadRequest)
	})

	require.NoError(t, svc.Archive(ctx, entry.ID, userID))

	t.Run("архивную удаляет вместе с историей", func(t *testing.T) {
		require.NoError(t, svc.Purge(ctx, entry.ID))

		_, err := svc.GetByID(ctx, entry.ID)
		assertHTTPStatus(t, err, http.StatusNotFound)

		var histCount int64
		require.NoError(t, db.Model(&models.PersonBlacklistHistory{}).Where("entity_id = ?", entry.ID).Count(&histCount).Error)
		assert.Zero(t, histCount, "история записи должна быть удалена")
	})
}
