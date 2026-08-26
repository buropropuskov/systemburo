package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBureau_TimeSlots_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/bureau/time-slots", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestBureau_TimeSlots_CRUD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Изначально расписание пустое.
	rec := testutil.GET(t, e, "/bureau/time-slots", h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, testutil.ParseSlice(t, rec), 0)

	// Add
	rec = testutil.POST(t, e, "/bureau/time-slots", `{"day_of_week":0,"open_time":"08:00","close_time":"17:00"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	slotID := int(testutil.ParseMap(t, rec)["id"].(float64))
	assert.Greater(t, slotID, 0)

	// Get
	rec = testutil.GET(t, e, "/bureau/time-slots", h)
	require.Equal(t, http.StatusOK, rec.Code)
	slots := testutil.ParseSlice(t, rec)
	require.Len(t, slots, 1)
	assert.Equal(t, float64(0), slots[0]["day_of_week"])
	assert.Equal(t, "08:00", slots[0]["open_time"])
	assert.Equal(t, "17:00", slots[0]["close_time"])

	// Update
	rec = testutil.PUT(t, e, fmt.Sprintf("/bureau/time-slots/%d", slotID), `{"open_time":"09:00","close_time":"18:00"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, "/bureau/time-slots", h)
	require.Equal(t, http.StatusOK, rec.Code)
	slots = testutil.ParseSlice(t, rec)
	require.Len(t, slots, 1)
	assert.Equal(t, "09:00", slots[0]["open_time"])
	assert.Equal(t, "18:00", slots[0]["close_time"])

	// Delete
	rec = testutil.DELETE(t, e, fmt.Sprintf("/bureau/time-slots/%d", slotID), h)
	require.Equal(t, http.StatusOK, rec.Code)

	rec = testutil.GET(t, e, "/bureau/time-slots", h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, testutil.ParseSlice(t, rec), 0)
}

func TestBureau_TimeSlots_InvalidTime(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Неверный формат времени.
	rec := testutil.POST(t, e, "/bureau/time-slots", `{"day_of_week":0,"open_time":"invalid","close_time":"17:00"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// День недели вне диапазона.
	rec = testutil.POST(t, e, "/bureau/time-slots", `{"day_of_week":7,"open_time":"08:00","close_time":"17:00"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// Запись расписания Бюро гейтится requireAdmin: обычный пользователь получает 403.
func TestBureau_TimeSlots_NonAdminForbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "bureau_regular", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/bureau/time-slots", `{"day_of_week":0,"open_time":"08:00","close_time":"17:00"}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// 404 при обновлении/удалении несуществующего слота.
func TestBureau_TimeSlots_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.PUT(t, e, "/bureau/time-slots/99999", `{"open_time":"09:00"}`, h)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = testutil.DELETE(t, e, "/bureau/time-slots/99999", h)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
