package handlers_test

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Всплеск серверных ошибок зовёт носителей права на раздел мониторинга (#2192).
// Проверка сквозная: считающий запрос, сбор аудитории по правам и созданная запись
// уведомления. Порог и пауза по отдельности закрыты чистыми тестами в services.
func TestErrorSpike_NotifiesMonitoringHolders(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "spikewatcher", "password123", 1, td.OrgID, td.CompanyID)
	watcherID := getUserID(t, db, "spikewatcher")
	testutil.GrantPermission(t, watcherID, services.KeyPageAdminMonitoring)

	testutil.RegisterAndLogin(t, e, "spikeoutsider", "password123", 1, td.OrgID, td.CompanyID)
	outsiderID := getUserID(t, db, "spikeoutsider")

	insertRequestLogs(t, db, 100, 200)
	insertRequestLogs(t, db, 20, 503)

	svc := services.NewErrorSpikeNotifyService(db, services.NewNotificationService(db),
		services.NewPermissionResolver(db))
	require.NoError(t, svc.CheckAndNotify(context.Background()))

	assert.Equal(t, int64(1), spikeNotificationCount(t, db, watcherID),
		"носитель права на раздел мониторинга должен получить уведомление")
	assert.Equal(t, int64(0), spikeNotificationCount(t, db, outsiderID),
		"работнику без права на раздел уведомление ни к чему: раздел ему всё равно закрыт")

	// Всплеск живёт минутами: вторая проверка внутри паузы не должна плодить записи.
	require.NoError(t, svc.CheckAndNotify(context.Background()))
	assert.Equal(t, int64(1), spikeNotificationCount(t, db, watcherID),
		"внутри паузы второе уведомление слаться не должно")
}

// Ночная тишина не всплеск: доля, посчитанная по десятку запросов, скачет от одной
// ошибки, и без нижней границы потока тревога приходила бы каждую проверку.
func TestErrorSpike_QuietTrafficStaysSilent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterAndLogin(t, e, "quietwatcher", "password123", 1, td.OrgID, td.CompanyID)
	watcherID := getUserID(t, db, "quietwatcher")
	testutil.GrantPermission(t, watcherID, services.KeyPageAdminMonitoring)

	// Две записи, обе с ошибкой: доля 100 процентов, а тревоги быть не должно.
	insertRequestLogs(t, db, 2, 500)

	svc := services.NewErrorSpikeNotifyService(db, services.NewNotificationService(db),
		services.NewPermissionResolver(db))
	require.NoError(t, svc.CheckAndNotify(context.Background()))

	assert.Equal(t, int64(0), spikeNotificationCount(t, db, watcherID),
		"при потоке ниже границы доля не считается вовсе")
}

// insertRequestLogs кладёт в журнал n записей с заданным кодом ответа, датированных
// текущей минутой: окно проверки - последние минуты, и запись во вчерашней партиции
// в него не попала бы.
func insertRequestLogs(t *testing.T, db *gorm.DB, n int, status int) {
	t.Helper()
	at := time.Now().UTC().Add(-time.Minute)
	for i := 0; i < n; i++ {
		require.NoError(t, db.Exec(
			`INSERT INTO request_logs (url, method, response_status, duration_ms, duration_us, created_at)
			 VALUES (?,?,?,?,?,?)`,
			"/api/spike-test", "GET", status, 1, 1000, at,
		).Error)
	}
}

// spikeNotificationCount - сколько уведомлений о всплеске лежит у работника.
func spikeNotificationCount(t *testing.T, db *gorm.DB, userID int) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("user_id = ? AND type = ?", userID, services.NotificationTypeErrorSpike).
		Count(&count).Error)
	return count
}
