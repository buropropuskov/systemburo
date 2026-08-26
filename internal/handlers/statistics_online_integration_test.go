package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUsersOnline_CountsByLastSeenWindow проверяет, что users_online считает только
// пользователей с last_seen внутри окна онлайна, а старые/нулевые не попадают.
func TestUsersOnline_CountsByLastSeenWindow(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	recent := now.Add(-2 * time.Minute) // в окне
	stale := now.Add(-30 * time.Minute) // вне окна (5 мин)
	edge := now.Add(-4 * time.Minute)   // на границе - в окне

	mk := func(username string, ls *time.Time) {
		u := models.User{Username: username, TypeID: 1, IsActive: true}
		require.NoError(t, db.Create(&u).Error)
		if ls != nil {
			require.NoError(t, db.Model(&models.User{}).Where("id = ?", u.ID).
				Update("last_seen", *ls).Error)
		}
	}

	mk("online_recent", &recent)
	mk("online_edge", &edge)
	mk("offline_stale", &stale)
	mk("never_seen", nil)

	svc := services.NewStatisticsService(db, 0)
	summary, err := svc.GetSummary(context.Background(), now.Add(-24*time.Hour), now)
	require.NoError(t, err)

	assert.Equal(t, int64(2), summary.UsersOnline, "онлайн = recent + edge, без stale и never")
}

// TestSnapshotOnlinePeak_MaxAndUpsert проверяет: повторные снимки за день не плодят
// строки (UNIQUE по date) и peak_count монотонно растёт (MAX старого и текущего).
func TestSnapshotOnlinePeak_MaxAndUpsert(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	svc := services.NewStatisticsService(db, 0)

	// Снимок 1: 3 пользователя онлайн.
	for _, name := range []string{"p1", "p2", "p3"} {
		u := models.User{Username: name, TypeID: 1, IsActive: true}
		require.NoError(t, db.Create(&u).Error)
		require.NoError(t, db.Model(&models.User{}).Where("id = ?", u.ID).
			Update("last_seen", now).Error)
	}
	require.NoError(t, svc.SnapshotOnlinePeak(context.Background()))

	today := now.Format("2006-01-02")
	var rowCount int64
	require.NoError(t, db.Table("user_online_peaks").Where("date = ?", today).Count(&rowCount).Error)
	assert.Equal(t, int64(1), rowCount, "одна строка за дату")

	var peak int
	require.NoError(t, db.Table("user_online_peaks").Where("date = ?", today).
		Select("peak_count").Scan(&peak).Error)
	assert.Equal(t, 3, peak, "пик после первого снимка")

	// Снимок 2: онлайн упал до 1 (двое "ушли") - пик не должен уменьшиться.
	require.NoError(t, db.Model(&models.User{}).Where("username IN ?", []string{"p2", "p3"}).
		Update("last_seen", now.Add(-1*time.Hour)).Error)
	require.NoError(t, svc.SnapshotOnlinePeak(context.Background()))

	require.NoError(t, db.Table("user_online_peaks").Where("date = ?", today).Count(&rowCount).Error)
	assert.Equal(t, int64(1), rowCount, "upsert не плодит дубли по дате")
	require.NoError(t, db.Table("user_online_peaks").Where("date = ?", today).
		Select("peak_count").Scan(&peak).Error)
	assert.Equal(t, 3, peak, "пик не убывает при снижении онлайна")

	// Снимок 3: онлайн вырос до 5 - пик растёт.
	for _, name := range []string{"p4", "p5", "p6", "p7"} {
		u := models.User{Username: name, TypeID: 1, IsActive: true}
		require.NoError(t, db.Create(&u).Error)
		require.NoError(t, db.Model(&models.User{}).Where("id = ?", u.ID).
			Update("last_seen", now).Error)
	}
	// p1 ещё онлайн (now), p4..p7 онлайн -> 5.
	require.NoError(t, svc.SnapshotOnlinePeak(context.Background()))
	require.NoError(t, db.Table("user_online_peaks").Where("date = ?", today).
		Select("peak_count").Scan(&peak).Error)
	assert.Equal(t, 5, peak, "пик вырос до нового максимума")

	// Summary отдаёт пик за сегодня.
	summary, err := svc.GetSummary(context.Background(), now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	assert.Equal(t, int64(5), summary.UsersOnlinePeakToday)
}

// TestGetOnlineUsers_WindowSortAndFields проверяет список «кто онлайн»: только в окне,
// по убыванию last_seen, ФИО собрано из частей, роль/тип подтянуты из справочников.
func TestGetOnlineUsers_WindowSortAndFields(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	recent := now.Add(-1 * time.Minute) // в окне, свежее
	edge := now.Add(-4 * time.Minute)   // в окне, граница
	stale := now.Add(-30 * time.Minute) // вне окна (5 мин)

	role := models.Role{Code: "role_test_g7", Name: "РольТест"}
	require.NoError(t, db.Create(&role).Error)
	utype := models.UserType{Code: "type_test_g7", Name: "ТипТест"}
	require.NoError(t, db.Create(&utype).Error)

	str := func(s string) *string { return &s }
	mk := func(username string, last, first, middle *string, roleID *int, ls *time.Time) {
		u := models.User{Username: username, TypeID: utype.ID, IsActive: true,
			LastName: last, FirstName: first, MiddleName: middle, RoleID: roleID}
		require.NoError(t, db.Create(&u).Error)
		if ls != nil {
			require.NoError(t, db.Model(&models.User{}).Where("id = ?", u.ID).
				Update("last_seen", *ls).Error)
		}
	}

	mk("ivanov", str("Иванов"), str("Иван"), str("Иванович"), &role.ID, &recent)
	mk("petr", nil, str("Пётр"), nil, nil, &edge)
	mk("stale_user", str("Старый"), nil, nil, nil, &stale)
	mk("never_user", str("Никогда"), nil, nil, nil, nil)

	// Забаненный и архивный со свежим last_seen НЕ должны попадать ни в список, ни в счётчик.
	banned := models.User{Username: "banned_fresh", TypeID: utype.ID, IsActive: true, IsBanned: true}
	require.NoError(t, db.Create(&banned).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", banned.ID).
		Update("last_seen", recent).Error)
	// is_active=false выставляем через Updates(map), иначе gorm с default:true опустит zero-value.
	inactive := models.User{Username: "inactive_fresh", TypeID: utype.ID}
	require.NoError(t, db.Create(&inactive).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", inactive.ID).
		Updates(map[string]any{"is_active": false, "last_seen": recent}).Error)

	svc := services.NewStatisticsService(db, 0)
	users, err := svc.GetOnlineUsers(context.Background())
	require.NoError(t, err)

	require.Len(t, users, 2, "только активные не забаненные в окне онлайна")
	// По убыванию last_seen: recent (ivanov) раньше edge (petr).
	assert.Equal(t, "ivanov", users[0].Login)
	assert.Equal(t, "Иванов Иван Иванович", users[0].FullName)
	assert.Equal(t, "РольТест", users[0].Role)
	assert.Equal(t, "ТипТест", users[0].UserType)
	assert.False(t, users[0].LastSeen.IsZero())

	assert.Equal(t, "petr", users[1].Login)
	assert.Equal(t, "Пётр", users[1].FullName, "ФИО из доступных частей")
	assert.Empty(t, users[1].Role, "без роли -> пусто")

	// Счётчик плитки (users_online) использует тот же предикат -> совпадает с длиной списка.
	summary, err := svc.GetSummary(context.Background(), now.Add(-24*time.Hour), now)
	require.NoError(t, err)
	assert.Equal(t, int64(2), summary.UsersOnline, "счётчик исключает забаненных/архивных, как и список")
}

// TestGetOnlineUsersHandler_HTTP проверяет, что эндпоинт проводит список через сервис
// и отдаёт его в envelope.
func TestGetOnlineUsersHandler_HTTP(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	u := models.User{Username: "online_one", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&u).Error)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", u.ID).
		Update("last_seen", now.Add(-1*time.Minute)).Error)

	h := handlers.NewStatisticsHandler(services.NewStatisticsService(db, 0))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/statistics/online-users", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.GetOnlineUsers(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool                `json:"success"`
		Data    []models.OnlineUser `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	require.Len(t, resp.Data, 1)
	assert.Equal(t, "online_one", resp.Data[0].Login)
}

// TestGetOnlinePeaks_Series проверяет серию пиков за период: фильтр по датам и порядок.
func TestGetOnlinePeaks_Series(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	mkPeak := func(date time.Time, peak int) {
		require.NoError(t, db.Exec(`
			INSERT INTO user_online_peaks (date, peak_count, created_at, updated_at)
			VALUES (?, ?, ?, ?)`,
			date.Format("2006-01-02"), peak, now, now).Error)
	}

	d2 := now.Add(-48 * time.Hour)
	d1 := now.Add(-24 * time.Hour)
	d0 := now
	old := now.Add(-10 * 24 * time.Hour) // вне периода

	mkPeak(old, 99)
	mkPeak(d0, 7)
	mkPeak(d2, 3)
	mkPeak(d1, 5)

	svc := services.NewStatisticsService(db, 0)
	points, err := svc.GetOnlinePeaks(context.Background(), now.Add(-3*24*time.Hour), now)
	require.NoError(t, err)

	require.Len(t, points, 3, "только дни внутри периода")
	assert.Equal(t, d2.Format("2006-01-02"), points[0].Date, "по возрастанию даты")
	assert.Equal(t, 3, points[0].Peak)
	assert.Equal(t, d1.Format("2006-01-02"), points[1].Date)
	assert.Equal(t, 5, points[1].Peak)
	assert.Equal(t, d0.Format("2006-01-02"), points[2].Date)
	assert.Equal(t, 7, points[2].Peak)
}

// TestGetOnlinePeaksHandler_HTTP проверяет, что эндпоинт GET /statistics/online-peaks
// проводит серию пиков через сервис и отдаёт её в envelope, отфильтрованную по периоду.
func TestGetOnlinePeaksHandler_HTTP(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	mkPeak := func(date time.Time, peak int) {
		require.NoError(t, db.Exec(`
			INSERT INTO user_online_peaks (date, peak_count, created_at, updated_at)
			VALUES (?, ?, ?, ?)`,
			date.Format("2006-01-02"), peak, now, now).Error)
	}
	d1 := now.Add(-24 * time.Hour)
	mkPeak(d1, 4)
	mkPeak(now, 9)

	h := handlers.NewStatisticsHandler(services.NewStatisticsService(db, 0))

	e := echo.New()
	from := now.Add(-72 * time.Hour).Format("2006-01-02")
	to := now.Format("2006-01-02")
	req := httptest.NewRequest(http.MethodGet, "/statistics/online-peaks?from="+from+"&to="+to, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.GetOnlinePeaks(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool                     `json:"success"`
		Data    []models.OnlinePeakPoint `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	require.Len(t, resp.Data, 2, "оба дня внутри периода")
	assert.Equal(t, d1.Format("2006-01-02"), resp.Data[0].Date, "по возрастанию даты")
	assert.Equal(t, 4, resp.Data[0].Peak)
	assert.Equal(t, now.Format("2006-01-02"), resp.Data[1].Date)
	assert.Equal(t, 9, resp.Data[1].Peak)
}

// TestGetOnlinePeaksHandler_InvalidRange проверяет, что from позже to даёт 400.
func TestGetOnlinePeaksHandler_InvalidRange(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	h := handlers.NewStatisticsHandler(services.NewStatisticsService(db, 0))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/statistics/online-peaks?from=2026-06-10&to=2026-06-01", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.GetOnlinePeaks(c)
	require.Error(t, err)
	he, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, he.Code)
}
