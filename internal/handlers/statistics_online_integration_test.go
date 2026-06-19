package handlers_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

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
	stale := now.Add(-30 * time.Minute) // вне окна (15 мин)
	edge := now.Add(-14 * time.Minute)  // на границе - в окне

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

	svc := services.NewStatisticsService(db)
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
	svc := services.NewStatisticsService(db)

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

	svc := services.NewStatisticsService(db)
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
