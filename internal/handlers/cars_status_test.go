package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCarTerritoryStatus_EntryThenExitWithHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carentry1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// Entry: territory_status = 1
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/territory-status", carID),
		`{"territory_status": 1}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Car territory status updated successfully", testutil.ParseMessage(t, rec))

	// Verify car has territory_status=1 in DB
	var car models.Car
	require.NoError(t, db.First(&car, carID).Error)
	require.NotNil(t, car.TerritoryStatus)
	assert.Equal(t, 1, *car.TerritoryStatus)
	assert.NotNil(t, car.TerritoryEntryTime, "entry time should be set after territory_status=1")

	// Exit: territory_status = 2
	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/territory-status", carID),
		`{"territory_status": 2}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Car territory status updated successfully", testutil.ParseMessage(t, rec))

	// Verify car has territory_status=2 in DB
	require.NoError(t, db.First(&car, carID).Error)
	require.NotNil(t, car.TerritoryStatus)
	assert.Equal(t, 2, *car.TerritoryStatus)

	// Verify history records: at least entry + exit
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	history := testutil.ParseSlice(t, rec)

	actionTypes := make([]string, 0, len(history))
	for _, h := range history {
		if at, ok := h["action_type"].(string); ok {
			actionTypes = append(actionTypes, at)
		}
	}
	assert.Contains(t, actionTypes, "entry", "history should contain entry record")
	assert.Contains(t, actionTypes, "exit", "history should contain exit record")
}

// TestCarTerritoryStatus_RecordsTableInHistory фиксирует регрессию: въезд/выезд
// должны сохранять table_id таблицы (КПП), из которой отмечены, и история должна
// отдавать table_name. Раньше table_id не писался и read-запрос не джойнил system_tables.
func TestCarTerritoryStatus_RecordsTableInHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carentrytbl1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	tableID := seedSystemTable(t, db)

	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/territory-status", carID),
		fmt.Sprintf(`{"territory_status": 1, "table_id": %d}`, tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	history := testutil.ParseSlice(t, rec)

	var entry map[string]interface{}
	for _, h := range history {
		if h["action_type"] == "entry" {
			entry = h
			break
		}
	}
	require.NotNil(t, entry, "history should contain entry record")
	require.NotNil(t, entry["table_id"], "entry record should carry table_id")
	assert.Equal(t, float64(tableID), entry["table_id"])
	assert.Equal(t, "Test Table", entry["table_name"], "entry record should resolve table_name from system_tables")
}

func TestCarTerritoryStatus_DeactivateSetsDateRemoved(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "cardeact2", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// Car is now active (status=1)
	var carBefore models.Car
	require.NoError(t, db.First(&carBefore, carID).Error)
	require.NotNil(t, carBefore.Status)
	assert.Equal(t, 1, *carBefore.Status, "car should be active before deactivation")
	assert.Nil(t, carBefore.DateRemoved, "date_removed should be nil before deactivation")

	// Deactivate
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		`{"status": 2}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Car deactivated successfully", testutil.ParseMessage(t, rec))

	// Verify DB state
	var carAfter models.Car
	require.NoError(t, db.First(&carAfter, carID).Error)
	require.NotNil(t, carAfter.Status)
	assert.Equal(t, 2, *carAfter.Status, "status should change to 2 after deactivation")
	assert.NotNil(t, carAfter.DateRemoved, "date_removed should be set after deactivation")

	// Verify history has a "delete" entry
	var historyCount int64
	db.Model(&models.CarHistory{}).Where("car_id = ? AND action_type = ?", carID, "delete").Count(&historyCount)
	assert.Equal(t, int64(1), historyCount, "should have exactly one delete history entry")
}

func TestCarTerritoryStatus_ActivateAfterDeactivateClearsDateRemoved(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "caract2", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, carID := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// Deactivate first
	rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/deactivate", carID),
		`{"status": 2}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	// Confirm deactivated state
	var carDeactivated models.Car
	require.NoError(t, db.First(&carDeactivated, carID).Error)
	assert.NotNil(t, carDeactivated.DateRemoved, "date_removed should be set after deactivation")
	require.NotNil(t, carDeactivated.Status)
	assert.Equal(t, 2, *carDeactivated.Status)

	// Activate
	rec = testutil.PUT(t, e, fmt.Sprintf("/cars/%d/activate", carID),
		`{}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Car activated successfully", testutil.ParseMessage(t, rec))

	// Verify date_removed cleared and status=1
	var carActivated models.Car
	require.NoError(t, db.First(&carActivated, carID).Error)
	require.NotNil(t, carActivated.Status)
	assert.Equal(t, 1, *carActivated.Status, "status should be 1 after activation")
	assert.Nil(t, carActivated.DateRemoved, "date_removed should be cleared after activation")

	// Verify history has both delete and activate entries
	var deleteCount, activateCount int64
	db.Model(&models.CarHistory{}).Where("car_id = ? AND action_type = ?", carID, "delete").Count(&deleteCount)
	db.Model(&models.CarHistory{}).Where("car_id = ? AND action_type = ?", carID, "activate").Count(&activateCount)
	assert.Equal(t, int64(1), deleteCount, "should have one delete history entry")
	assert.Equal(t, int64(1), activateCount, "should have one activate history entry")
}
