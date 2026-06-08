package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newVehicleBlacklistService(db *gorm.DB) services.VehicleBlacklistService {
	return services.NewVehicleBlacklistService(db, services.NewVehicleBlacklistHistoryService(db))
}

// seedMark пересоздаёт марку с заданным именем (marks не чистятся CleanDB - убираем
// возможный остаток от прошлого прогона) и регистрирует cleanup.
func seedMark(t *testing.T, db *gorm.DB, name string) models.Mark {
	t.Helper()
	db.Where("name = ?", name).Delete(&models.Mark{})
	mark := models.Mark{Name: name, IsActive: true}
	require.NoError(t, db.Create(&mark).Error)
	t.Cleanup(func() {
		db.Where("mark_id = ?", mark.ID).Delete(&models.MarkHistory{})
		db.Delete(&models.Mark{}, mark.ID)
	})
	return mark
}

func assertHTTPStatus(t *testing.T, err error, code int) {
	t.Helper()
	require.Error(t, err)
	var he *echo.HTTPError
	require.True(t, errors.As(err, &he), "expected *echo.HTTPError, got %T: %v", err, err)
	assert.Equal(t, code, he.Code)
}

// TestVehicleBlacklist_Lifecycle покрывает create/check/duplicate/archive/restore без cars.
func TestVehicleBlacklist_Lifecycle(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	mark := seedMark(t, db, "BL_Lifecycle")
	svc := newVehicleBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "A123BC799", MarkID: mark.ID, Reason: "тест",
	}, userID)
	require.NoError(t, err)
	require.NotZero(t, entry.ID)
	assert.Equal(t, "BL_Lifecycle", entry.MarkName, "должен снапшотить имя марки")

	checks := []struct {
		name      string
		number    string
		markID    int
		wantBlock bool
	}{
		{"matching number+mark", "A123BC799", mark.ID, true},
		{"case/space insensitive", "  a123bc799 ", mark.ID, true},
		{"other mark", "A123BC799", mark.ID + 999999, false},
		{"other number", "X000XX000", mark.ID, false},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			res, err := svc.Check(ctx, tc.number, tc.markID)
			require.NoError(t, err)
			assert.Equal(t, tc.wantBlock, res.IsBlacklisted)
			if tc.wantBlock {
				assert.Equal(t, "тест", res.Reason)
			}
		})
	}

	// Повторное добавление активной записи (даже в другом регистре) блокируется
	// partial unique index-ом по LOWER(TRIM(car_number)).
	_, err = svc.Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "a123bc799", MarkID: mark.ID, Reason: "дубль",
	}, userID)
	assertHTTPStatus(t, err, 409)

	// Несуществующая марка -> 400.
	_, err = svc.Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "Z999ZZ", MarkID: 99999999, Reason: "x",
	}, userID)
	assertHTTPStatus(t, err, 400)

	// Снятие (архивация) - больше не блокирует.
	require.NoError(t, svc.Archive(ctx, entry.ID, userID))
	res, err := svc.Check(ctx, "A123BC799", mark.ID)
	require.NoError(t, err)
	assert.False(t, res.IsBlacklisted)

	active, err := svc.GetAll(ctx, false)
	require.NoError(t, err)
	assert.Empty(t, active)
	all, err := svc.GetAll(ctx, true)
	require.NoError(t, err)
	assert.Len(t, all, 1)

	// После архивации ту же машину можно добавить заново (partial unique).
	entry2, err := svc.Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "A123BC799", MarkID: mark.ID, Reason: "снова",
	}, userID)
	require.NoError(t, err)
	assert.NotEqual(t, entry.ID, entry2.ID)

	// Restore архивной записи при наличии активного дубля -> 409.
	assertHTTPStatus(t, svc.Restore(ctx, entry.ID, userID), 409)

	// История содержит created + archived для первой записи.
	hist, err := svc.GetHistory(ctx, entry.ID)
	require.NoError(t, err)
	actions := make([]string, 0, len(hist))
	for _, h := range hist {
		actions = append(actions, h.ActionType)
	}
	assert.Contains(t, actions, models.BlacklistActionCreated)
	assert.Contains(t, actions, models.BlacklistActionArchived)
}

// TestVehicleBlacklist_CascadeDeactivatesActiveCar: добавление в ЧС гасит активную машину.
func TestVehicleBlacklist_CascadeDeactivatesActiveCar(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "blcasc1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "blcasc1")
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	var before models.Car
	require.NoError(t, db.First(&before, carID).Error)
	require.NotNil(t, before.Status)
	require.Equal(t, 1, *before.Status, "машина должна быть активна до ЧС")

	// car_brand="Kamaz" зашит в seedCarViaCompleteApp; марку называем так же, чтобы
	// каскад сматчил по имени (mark_id у тестовой машины пуст).
	mark := seedMark(t, db, "Kamaz")
	svc := newVehicleBlacklistService(db)
	entry, err := svc.Create(context.Background(), models.CreateVehicleBlacklistRequest{
		CarNumber: "B002BB799", MarkID: mark.ID, Reason: "угон",
	}, userID)
	require.NoError(t, err)

	var after models.Car
	require.NoError(t, db.First(&after, carID).Error)
	require.NotNil(t, after.Status)
	assert.Equal(t, 0, *after.Status, "машина должна деактивироваться")
	assert.NotNil(t, after.DateRemoved, "date_removed должен проставиться")

	var carHistCount int64
	db.Model(&models.CarHistory{}).Where("car_id = ? AND action_type = ?", carID, "blacklisted").Count(&carHistCount)
	assert.Equal(t, int64(1), carHistCount, "должна быть запись cars_history blacklisted")

	var blHistCount int64
	db.Model(&models.VehicleBlacklistHistory{}).Where("entity_id = ? AND action_type = ?", entry.ID, models.BlacklistActionCreated).Count(&blHistCount)
	assert.Equal(t, int64(1), blHistCount)
}

// TestVehicleBlacklist_UnblacklistRestoresActiveApplicationCar: снятие из ЧС возвращает
// status=1 машине с активной заявкой.
func TestVehicleBlacklist_UnblacklistRestoresActiveApplicationCar(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "blcasc2", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "blcasc2")
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	mark := seedMark(t, db, "Kamaz")
	svc := newVehicleBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "B002BB799", MarkID: mark.ID, Reason: "проверка",
	}, userID)
	require.NoError(t, err)

	var blacklisted models.Car
	require.NoError(t, db.First(&blacklisted, carID).Error)
	require.Equal(t, 0, *blacklisted.Status)

	require.NoError(t, svc.Archive(ctx, entry.ID, userID))

	var restored models.Car
	require.NoError(t, db.First(&restored, carID).Error)
	require.NotNil(t, restored.Status)
	assert.Equal(t, 1, *restored.Status, "машина с активной заявкой должна вернуться в status=1")
	assert.Nil(t, restored.DateRemoved, "date_removed должен очиститься")

	var carHistCount int64
	db.Model(&models.CarHistory{}).Where("car_id = ? AND action_type = ?", carID, "unblacklisted").Count(&carHistCount)
	assert.Equal(t, int64(1), carHistCount, "должна быть запись cars_history unblacklisted")
}

// TestVehicleBlacklist_UnblacklistSkipsExpiredPass: снятие из ЧС не возрождает машину,
// у которой за время блокировки истёк пропуск (дата неактуальна).
func TestVehicleBlacklist_UnblacklistSkipsExpiredPass(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "blexp1", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "blexp1")
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	mark := seedMark(t, db, "Kamaz")
	svc := newVehicleBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "B002BB799", MarkID: mark.ID, Reason: "проверка",
	}, userID)
	require.NoError(t, err)

	// Пропуск истёк, пока машина была в ЧС.
	require.NoError(t, db.Model(&models.Car{}).Where("id = ?", carID).Update("entry_date_to", "2000-01-01").Error)

	require.NoError(t, svc.Archive(ctx, entry.ID, userID))

	var after models.Car
	require.NoError(t, db.First(&after, carID).Error)
	require.NotNil(t, after.Status)
	assert.Equal(t, 0, *after.Status, "машина с истёкшим пропуском не должна возрождаться")
}

// TestVehicleBlacklist_UpdateReason покрывает редактирование причины + лог в историю.
func TestVehicleBlacklist_UpdateReason(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	mark := seedMark(t, db, "BL_UpdReason")
	svc := newVehicleBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "U111UU799", MarkID: mark.ID, Reason: "старая причина",
	}, userID)
	require.NoError(t, err)

	updated, err := svc.UpdateReason(ctx, entry.ID, "  новая причина  ", userID)
	require.NoError(t, err)
	assert.Equal(t, "новая причина", updated.Reason)

	stored, err := svc.GetByID(ctx, entry.ID)
	require.NoError(t, err)
	assert.Equal(t, "новая причина", stored.Reason)

	hist, err := svc.GetHistory(ctx, entry.ID)
	require.NoError(t, err)
	hasUpdated := false
	for _, h := range hist {
		if h.ActionType == models.BlacklistActionUpdated {
			hasUpdated = true
		}
	}
	assert.True(t, hasUpdated, "ожидали запись истории 'updated'")

	t.Run("пустая причина - 400", func(t *testing.T) {
		_, err := svc.UpdateReason(ctx, entry.ID, "   ", userID)
		assertHTTPStatus(t, err, http.StatusBadRequest)
	})

	t.Run("та же причина - без новой записи истории", func(t *testing.T) {
		before, _ := svc.GetHistory(ctx, entry.ID)
		_, err := svc.UpdateReason(ctx, entry.ID, "новая причина", userID)
		require.NoError(t, err)
		after, _ := svc.GetHistory(ctx, entry.ID)
		assert.Equal(t, len(before), len(after), "идентичная причина не должна писать историю")
	})

	t.Run("нельзя редактировать архивную запись - 400", func(t *testing.T) {
		require.NoError(t, svc.Archive(ctx, entry.ID, userID))
		_, err := svc.UpdateReason(ctx, entry.ID, "после архива", userID)
		assertHTTPStatus(t, err, http.StatusBadRequest)
	})
}

// TestVehicleBlacklist_Purge покрывает hard-delete архивной записи + запрет на активную.
func TestVehicleBlacklist_Purge(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	userID, _, userCleanup := setupMWUser(t, db, true, false)
	defer userCleanup()
	mark := seedMark(t, db, "BL_Purge")
	svc := newVehicleBlacklistService(db)
	ctx := context.Background()

	entry, err := svc.Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "P222PP799", MarkID: mark.ID, Reason: "к удалению",
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
		require.NoError(t, db.Model(&models.VehicleBlacklistHistory{}).Where("entity_id = ?", entry.ID).Count(&histCount).Error)
		assert.Zero(t, histCount, "история записи должна быть удалена")
	})
}
