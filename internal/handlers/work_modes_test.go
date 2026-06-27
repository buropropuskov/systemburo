package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// todayDOW -- текущий день недели в формате слотов (0=Пн..6=Вс) в МСК,
// как и сервис (computeWorkModeStatus считает в Europe/Moscow).
func todayDOW() int {
	return int(time.Now().In(time.FixedZone("MSK", 3*60*60)).Weekday()+6) % 7
}

func TestWorkModes_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.GET(t, e, "/work-modes", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// Агрегатор приводит три типа к единой форме слота: Бюро, места разгрузки и
// места прохода. Архивные исключаются, операционно неактивные включаются с status.
func TestWorkModes_Aggregates(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "modes_user", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	today := todayDOW()

	// Бюро: круглосуточный слот на сегодня -> current_status open.
	require.NoError(t, db.Create(&models.BureauTimeSlot{
		DayOfWeek: today, OpenTime: "00:00", CloseTime: "23:59", IsNextDay: false, IsActive: true,
	}).Error)

	// Место разгрузки активное, открыто сейчас.
	active := models.UnloadPlace{Name: "Склад активный", Status: "active", IsActive: true}
	require.NoError(t, db.Create(&active).Error)
	require.NoError(t, db.Create(&models.UnloadPlaceTimeSlot{
		UnloadPlaceID: active.ID, DayOfWeek: today, OpenTime: "00:00", CloseTime: "23:59", IsActive: true,
	}).Error)

	// Место разгрузки операционно неактивное -- включается с флагом status.
	inactive := models.UnloadPlace{Name: "Склад неактивный", Status: "inactive", IsActive: true}
	require.NoError(t, db.Create(&inactive).Error)

	// Архивное (soft-delete, is_active=false) -- НЕ должно попасть в выдачу.
	// is_active имеет gorm default:true, поэтому zero-value при Create опускается и
	// БД ставит true -- форсируем false явным Update.
	archived := models.UnloadPlace{Name: "Склад архивный", Status: "active"}
	require.NoError(t, db.Create(&archived).Error)
	require.NoError(t, db.Model(&archived).Update("is_active", false).Error)

	// Место прохода (пост) -- системная таблица с display_name и слотом.
	display := "КПП-1 - главный въезд"
	table := models.SystemTable{Name: "kpp1", DisplayName: &display, Status: "active", IsActive: true}
	require.NoError(t, db.Create(&table).Error)
	require.NoError(t, db.Create(&models.SystemTableTimeSlot{
		TableID: table.ID, DayOfWeek: today, OpenTime: "08:00", CloseTime: "20:00", IsActive: true,
	}).Error)

	rec := testutil.GET(t, e, "/work-modes", h)
	require.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseResponse[services.WorkModesResponse](t, rec)

	// Бюро всегда присутствует, открыто по круглосуточному слоту.
	assert.Equal(t, "bureau", resp.Bureau.Kind)
	assert.Equal(t, "active", resp.Bureau.Status)
	assert.Equal(t, "open", resp.Bureau.CurrentStatus)
	require.Len(t, resp.Bureau.TimeSlots, 1)
	assert.Equal(t, today, resp.Bureau.TimeSlots[0].DayOfWeek)
	assert.Equal(t, "00:00", resp.Bureau.TimeSlots[0].OpenTime)
	assert.Equal(t, "23:59", resp.Bureau.TimeSlots[0].CloseTime)

	// Места разгрузки: активное + неактивное, БЕЗ архивного.
	require.Len(t, resp.UnloadPlaces, 2)
	byName := map[string]services.WorkModeEntity{}
	for _, p := range resp.UnloadPlaces {
		byName[p.Name] = p
		assert.Equal(t, "unload_place", p.Kind)
	}
	require.Contains(t, byName, "Склад активный")
	require.Contains(t, byName, "Склад неактивный")
	assert.NotContains(t, byName, "Склад архивный")
	assert.Equal(t, "active", byName["Склад активный"].Status)
	assert.Equal(t, "open", byName["Склад активный"].CurrentStatus)
	assert.Equal(t, "inactive", byName["Склад неактивный"].Status)
	// Операционно неактивное место всегда closed (canonical computeUnloadPlaceStatus
	// при status != active) -- проверяем, что семантика пробросилась через агрегатор.
	assert.Equal(t, "closed", byName["Склад неактивный"].CurrentStatus)

	// Слот активного места в единой форме {day_of_week,open_time,close_time,is_next_day,is_active}.
	require.Len(t, byName["Склад активный"].TimeSlots, 1)
	slot := byName["Склад активный"].TimeSlots[0]
	assert.Equal(t, today, slot.DayOfWeek)
	assert.Equal(t, "00:00", slot.OpenTime)
	assert.Equal(t, "23:59", slot.CloseTime)
	assert.False(t, slot.IsNextDay)
	assert.True(t, slot.IsActive)

	// Места прохода: пост с display_name, единые слоты. current_status поста
	// (слот 08:00-20:00) зависит от времени прогона -- детерминированно его логику
	// покрывает unit-тест computeWorkModeStatus; здесь проверяем проброс формы слота.
	require.Len(t, resp.Checkpoints, 1)
	cp := resp.Checkpoints[0]
	assert.Equal(t, "checkpoint", cp.Kind)
	assert.Equal(t, display, cp.Name)
	assert.Equal(t, "active", cp.Status)
	require.Len(t, cp.TimeSlots, 1)
	assert.Equal(t, "08:00", cp.TimeSlots[0].OpenTime)
	assert.Equal(t, "20:00", cp.TimeSlots[0].CloseTime)
}

// Пустая система: Бюро присутствует со статусом closed и пустыми слотами, списки пусты.
func TestWorkModes_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "modes_empty", "pass123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.GET(t, e, "/work-modes", h)
	require.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseResponse[services.WorkModesResponse](t, rec)

	assert.Equal(t, "bureau", resp.Bureau.Kind)
	assert.Equal(t, "closed", resp.Bureau.CurrentStatus)
	assert.Empty(t, resp.Bureau.TimeSlots)
	assert.Empty(t, resp.UnloadPlaces)
	assert.Empty(t, resp.Checkpoints)
}
