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
	"gorm.io/gorm"
)

// Метрики принимающих (#1251 S3) на РЕАЛЬНОМ SQL: агрегаты собираются из констант
// движка и исполняются GORM'ом против PG. База — подзапрос первого take_to_work на
// заявку (DISTINCT ON), билд его не проверяет; каждая метрика гоняется через
// RunReport на заведомых данных и сверяется с посчитанным вручную числом.
//
// Время принятия с #1251 S2 считается по рабочему времени Бюро, но тест НЕ заводит
// график bureau_time_slots -> метрика падает на календарный фолбэк, поэтому часы
// равны календарной разнице. Сам вычет нерабочих часов проверен на этапах заявки в
// TestRunReport_DurationMetrics_BureauWorkingTime (тот же bureauWorkingDuration).

// seedAcceptedApps заводит заявки с известным принятием в работу:
//
//	Сидоров:   AC/A принял 2ч, AC/B принял 4ч            -> реакция 3ч, нагрузка 2
//	Кузнецов:  AC/C принял 1ч, AC/D принял 3ч            -> реакция 2ч, нагрузка 2
//
// AC/D принимали дважды: сначала Кузнецов, позже повторно Сидоров (заявку могли
// отозвать из работы и принять снова). Принимающим считается ПЕРВЫЙ (Кузнецов),
// accepted_at = первое принятие — повторное принятие Сидорова в его рейтинг не идёт.
func seedAcceptedApps(t *testing.T, db *gorm.DB) {
	t.Helper()

	org := models.Organization{Name: "Орг-Принятие", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	sender := models.User{Username: "acc_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&sender).Error)

	mkUser := func(username, last, first string) models.User {
		l, f := last, first
		u := models.User{Username: username, TypeID: 1, IsActive: true, LastName: &l, FirstName: &f}
		require.NoError(t, db.Create(&u).Error)
		return u
	}
	sidorov := mkUser("acc_sidorov", "Сидоров", "Сидор")
	kuznetsov := mkUser("acc_kuznetsov", "Кузнецов", "Кузьма")

	status := models.StatusInWork
	mkApp := func(number string, sent, confirmed, accepted *time.Time) int {
		n := number
		app := models.Application{
			ApplicationNumber:    &n,
			OrganizationID:       org.ID,
			SenderUserID:         sender.ID,
			Status:               &status,
			SendingDatetime:      sent,
			ConfirmationDatetime: confirmed,
			AcceptedAt:           accepted,
		}
		require.NoError(t, db.Create(&app).Error)
		return app.ID
	}
	// take_to_work в audit_log с явным CreatedAt: порядок определяет, кто ПЕРВЫЙ
	// принял (DISTINCT ON ... ORDER BY created_at ASC).
	takeToWork := func(appID int, acceptor models.User, at *time.Time) {
		id := appID
		require.NoError(t, db.Create(&models.AuditLog{
			EntityType:  models.AuditEntityApplication,
			EntityID:    &id,
			Action:      "take_to_work",
			ActorUserID: &acceptor.ID,
			CreatedAt:   *at,
		}).Error)
	}

	a := mkApp("AC/A", mskTime(t, "2026-06-01 09:00"), mskTime(t, "2026-06-01 10:00"), mskTime(t, "2026-06-01 12:00"))
	takeToWork(a, sidorov, mskTime(t, "2026-06-01 12:00"))
	b := mkApp("AC/B", mskTime(t, "2026-06-02 09:00"), mskTime(t, "2026-06-02 10:00"), mskTime(t, "2026-06-02 14:00"))
	takeToWork(b, sidorov, mskTime(t, "2026-06-02 14:00"))
	c := mkApp("AC/C", mskTime(t, "2026-06-03 09:00"), mskTime(t, "2026-06-03 10:00"), mskTime(t, "2026-06-03 11:00"))
	takeToWork(c, kuznetsov, mskTime(t, "2026-06-03 11:00"))
	d := mkApp("AC/D", mskTime(t, "2026-06-04 09:00"), mskTime(t, "2026-06-04 10:00"), mskTime(t, "2026-06-04 13:00"))
	takeToWork(d, kuznetsov, mskTime(t, "2026-06-04 13:00")) // первое принятие
	takeToWork(d, sidorov, mskTime(t, "2026-06-04 15:00"))   // повторное, позже -> не принимающий
}

func acceptorWindow() []models.ReportFilterValue {
	return []models.ReportFilterValue{{Key: "date_range", From: "2026-06-01", To: "2026-06-04"}}
}

// TestRunReport_AcceptorMetrics_ByAcceptor — разрез по принимающему на реальных
// join'ах и подзапросе первого принятия: строка на принимающего с его временем
// реакции и нагрузкой. Ловит, что повторное принятие в другого принимающего не
// утекает (DISTINCT ON берёт первого).
func TestRunReport_AcceptorMetrics_ByAcceptor(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedAcceptedApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:      "aggregate",
		Metrics:   []string{"avg_acceptor_response_time", "acceptor_accepts_count"},
		Dimension: "by_acceptor",
		Filters:   acceptorWindow(),
	})
	require.NoError(t, err, "SQL разреза по принимающему должен исполняться")

	require.Len(t, res.MetricRows, 2, "два принимающих -> две строки")
	byName := make(map[string]models.ReportMetricRow, 2)
	for _, r := range res.MetricRows {
		byName[r.Label] = r
	}

	sidorov, ok := byName["Сидоров Сидор"]
	require.True(t, ok, "подпись строки — ФИО принимающего, получено: %v", byName)
	assert.Equal(t, int64(3*3600), sidorov.Values["avg_acceptor_response_time"], "(2ч+4ч)/2")
	assert.Equal(t, int64(2), sidorov.Values["acceptor_accepts_count"], "принял AC/A и AC/B")

	kuznetsov, ok := byName["Кузнецов Кузьма"]
	require.True(t, ok)
	assert.Equal(t, int64(2*3600), kuznetsov.Values["avg_acceptor_response_time"], "(1ч+3ч)/2")
	assert.Equal(t, int64(2), kuznetsov.Values["acceptor_accepts_count"],
		"AC/D первым принял Кузнецов; повторное принятие Сидоровым не в счёт")

	assert.Equal(t, models.ReportValueDuration, res.Columns[0].Type,
		"время принятия — длительность, по типу колонки фронт выбирает формат")
	assert.Equal(t, int64(4), res.Totals["acceptor_accepts_count"], "итог счётчика — сумма строк")
}

// TestRunReport_AcceptorResponseTime_ExcludesNegativePairs — как у этапов заявки: на
// исторических данных принятие может стоять раньше согласования (accepted_at <
// confirmation_datetime), давая отрицательную длительность. Битую пару отсекаем.
func TestRunReport_AcceptorResponseTime_ExcludesNegativePairs(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-Принятие-Битая", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	sender := models.User{Username: "an_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&sender).Error)
	l, f := "Принимающий", "Тест"
	acc := models.User{Username: "an_acc", TypeID: 1, IsActive: true, LastName: &l, FirstName: &f}
	require.NoError(t, db.Create(&acc).Error)

	status := models.StatusInWork
	mkApp := func(number string, sent, confirmed, accepted *time.Time) int {
		n := number
		app := models.Application{
			ApplicationNumber:    &n,
			OrganizationID:       org.ID,
			SenderUserID:         sender.ID,
			Status:               &status,
			SendingDatetime:      sent,
			ConfirmationDatetime: confirmed,
			AcceptedAt:           accepted,
		}
		require.NoError(t, db.Create(&app).Error)
		return app.ID
	}
	take := func(appID int, at *time.Time) {
		id := appID
		require.NoError(t, db.Create(&models.AuditLog{
			EntityType: models.AuditEntityApplication, EntityID: &id,
			Action: "take_to_work", ActorUserID: &acc.ID, CreatedAt: *at,
		}).Error)
	}

	// Корректная: принятие 2ч (7200с).
	okApp := mkApp("AN/OK", mskTime(t, "2026-06-01 09:00"), mskTime(t, "2026-06-01 10:00"), mskTime(t, "2026-06-01 12:00"))
	take(okApp, mskTime(t, "2026-06-01 12:00"))
	// Битая: принятие РАНЬШЕ согласования на час.
	badApp := mkApp("AN/BAD", mskTime(t, "2026-06-02 09:00"), mskTime(t, "2026-06-02 10:00"), mskTime(t, "2026-06-02 09:00"))
	take(badApp, mskTime(t, "2026-06-02 09:00"))

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:      "aggregate",
		Metric:    "avg_acceptor_response_time",
		Dimension: "none",
		Filters:   acceptorWindow(),
	})
	require.NoError(t, err)
	require.Len(t, res.MetricRows, 1)
	assert.Equal(t, int64(7200), res.MetricRows[0].Values["avg_acceptor_response_time"],
		"битая пара (принятие < согласование) исключена, среднее не уходит в минус")
}

// TestRunReport_ByAcceptor_RejectedForForeignMetrics — разрез по принимающему даётся
// только метрикам принимающих. Каталог его не публикует для чужих метрик -> движок
// отбивает (как by_approver для метрик заявки).
func TestRunReport_ByAcceptor_RejectedForForeignMetrics(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	svc := services.NewStatisticsService(db, 0)
	for _, metric := range []string{"applications_count", "avg_approval_time", "avg_approver_response_time"} {
		t.Run(metric, func(t *testing.T) {
			_, err := svc.RunReport(context.Background(), models.ReportRequest{
				Mode:      "aggregate",
				Metric:    metric,
				Dimension: "by_acceptor",
				Filters:   acceptorWindow(),
			})
			require.ErrorIs(t, err, services.ErrInvalidReportRequest)
		})
	}
}

// TestReportCatalog_AcceptorMetrics — метрики принимающих публикуются каталогом в
// своей группе и с разрезом by_acceptor, который движок умеет резолвить.
func TestReportCatalog_AcceptorMetrics(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	svc := services.NewStatisticsService(db, 0)
	cat, err := svc.GetReportCatalog(context.Background())
	require.NoError(t, err)

	byKey := make(map[string]models.ReportMetricInfo, len(cat.Metrics))
	for _, m := range cat.Metrics {
		byKey[m.Key] = m
	}

	for _, key := range []string{"avg_acceptor_response_time", "acceptor_accepts_count"} {
		m, ok := byKey[key]
		require.True(t, ok, "каталог должен публиковать метрику %q", key)
		assert.Equal(t, "Принимающие", m.Group)
		assert.Contains(t, m.Dimensions, "by_acceptor")
		assert.NotEmpty(t, m.Label)
	}

	dims := make(map[string]string, len(cat.Dimensions))
	for _, d := range cat.Dimensions {
		dims[d.Key] = d.Label
	}
	assert.Equal(t, "Принимающий", dims["by_acceptor"], "разрез должен быть в каталоге с подписью")
}
