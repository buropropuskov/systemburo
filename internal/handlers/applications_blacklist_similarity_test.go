package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// latestElementID возвращает id последней вставленной строки таблицы по WHERE-условию.
func latestElementID(t *testing.T, db *gorm.DB, table, where string, args ...interface{}) int {
	t.Helper()
	var id int
	q := fmt.Sprintf("SELECT id FROM %s WHERE %s ORDER BY id DESC LIMIT 1", table, where)
	require.NoError(t, db.Raw(q, args...).Scan(&id).Error)
	require.NotZero(t, id, "элемент не найден: %s / %s", table, where)
	return id
}

// blacklistFlagFor возвращает per-element флаг обхода ЧС (#481), если он есть.
func blacklistFlagFor(t *testing.T, db *gorm.DB, elementType string, elementID int) (models.ApplicationBlacklistFlag, bool) {
	t.Helper()
	var f models.ApplicationBlacklistFlag
	err := db.Where("element_type = ? AND element_id = ?", elementType, elementID).First(&f).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return f, false
	}
	require.NoError(t, err)
	return f, true
}

// TestSubmitCompleteApplication_BlacklistSimilarity проверяет мягкий слой #481: похожий
// (не точный) элемент НЕ блокируется как точный 409, а помечается per-element флагом и
// отдаётся в детали заявки. Далёкие элементы остаются без флага.
func TestSubmitCompleteApplication_BlacklistSimilarity(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	token := testutil.RegisterAndLogin(t, e, "bl_sim_user", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "bl_sim_user")

	mark := seedMark(t, db, "BL_Sim_Mark")
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)

	_, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "C777CC799", MarkID: mark.ID, Reason: "угон",
	}, userID)
	require.NoError(t, err)
	_, err = newPersonBlacklistService(db).Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Petrov", FirstName: "Petr", MiddleName: "Petrovich", Reason: "нарушение",
	}, userID)
	require.NoError(t, err)

	t.Run("похожая машина (опечатка в номере) подаётся и помечается", func(t *testing.T) {
		rec := submitCarApp(t, e, db, token, orgName, "sim", "C777CC798", mark.ID)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		carID := latestElementID(t, db, "cars", "car_number = ?", "C777CC798")
		flag, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, carID)
		require.True(t, ok, "ожидался флаг похожести для машины")
		assert.GreaterOrEqual(t, flag.Similarity, 0.7)
		assert.Contains(t, flag.MatchedValue, "C777CC799")
		assert.Equal(t, "угон", flag.MatchedReason)

		var attID int
		require.NoError(t, db.Raw("SELECT attachment_id FROM cars WHERE id = ?", carID).Scan(&attID).Error)
		det := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, det.Code, "body: %s", det.Body.String())
		assert.Contains(t, det.Body.String(), "blacklist_similar")
		assert.Contains(t, det.Body.String(), "угон")
	})

	t.Run("точная машина по-прежнему 409 и не создаёт флаг", func(t *testing.T) {
		countCarFlags := func() int64 {
			var n int64
			require.NoError(t, db.Model(&models.ApplicationBlacklistFlag{}).
				Where("element_type = ?", models.BlacklistElementCar).Count(&n).Error)
			return n
		}
		before := countCarFlags()
		rec := submitCarApp(t, e, db, token, orgName, "exact", "C777CC799", mark.ID)
		require.Equal(t, http.StatusConflict, rec.Code, "body: %s", rec.Body.String())
		assert.Equal(t, before, countCarFlags(), "409 (точное совпадение) не должен создавать флаг")
	})

	t.Run("далёкая машина без флага", func(t *testing.T) {
		rec := submitCarApp(t, e, db, token, orgName, "far", "X123XX111", mark.ID)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		carID := latestElementID(t, db, "cars", "car_number = ?", "X123XX111")
		_, ok := blacklistFlagFor(t, db, models.BlacklistElementCar, carID)
		assert.False(t, ok, "далёкая машина не должна помечаться")
	})

	t.Run("похожий человек (без отчества) подаётся и помечается", func(t *testing.T) {
		rec := submitPersonApp(t, e, db, token, orgName, "sim", "Petrov", "Petr", "", citizenshipID, tableID)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		empID := latestElementID(t, db, "employees", "last_name = ? AND first_name = ?", "Petrov", "Petr")
		flag, ok := blacklistFlagFor(t, db, models.BlacklistElementEmployee, empID)
		require.True(t, ok, "ожидался флаг похожести для человека")
		assert.GreaterOrEqual(t, flag.Similarity, 0.7)
		assert.Contains(t, flag.MatchedValue, "Petrov")
		assert.Equal(t, "нарушение", flag.MatchedReason)

		var attID int
		require.NoError(t, db.Raw("SELECT attachment_id FROM employees WHERE id = ?", empID).Scan(&attID).Error)
		det := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/employees", attID), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, det.Code, "body: %s", det.Body.String())
		assert.Contains(t, det.Body.String(), "blacklist_similar")
	})

	t.Run("далёкий человек без флага", func(t *testing.T) {
		rec := submitPersonApp(t, e, db, token, orgName, "far", "Sidorov", "Sidr", "Sidorovich", citizenshipID, tableID)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		empID := latestElementID(t, db, "employees", "last_name = ? AND first_name = ?", "Sidorov", "Sidr")
		_, ok := blacklistFlagFor(t, db, models.BlacklistElementEmployee, empID)
		assert.False(t, ok, "далёкий человек не должен помечаться")
	})
}
