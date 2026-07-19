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

// Метрики качества и согласующих (#1240, B3) на РЕАЛЬНОМ SQL. Агрегаты собраны из
// констант движка, но исполняет их PG: COUNT(*) FILTER, коррелированный подзапрос
// внутри AVG, деление numeric и скан результата в int64 билд не проверяет — они
// падают только на исполнении. Поэтому каждая метрика гоняется через RunReport на
// заведомых данных и сверяется с числом, посчитанным вручную.

// seedQualityApps заводит четыре заявки с известным исходом и числом пересылок:
//
//	A: 01.06, В работе,    Согласовано,     3 пересылки
//	B: 02.06, Отказано,    Согласовано,     2 пересылки  -> отказ (статус)
//	C: 03.06, В обработке, Не согласовано,  1 пересылка  -> отказ (согласование)
//	D: 04.06, Завершено,   Согласовано,     0 пересылок
//
// Итого по окну: доля отказов 2/4 = 50.0%, среднее число пересылок 6/4 = 1.5.
// Отказ намеренно приходит двумя ветками (статус и confirmation) — метрика обязана
// считать обе.
func seedQualityApps(t *testing.T, db *gorm.DB) {
	t.Helper()

	org := models.Organization{Name: "Орг-Качество", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	sender := models.User{Username: "qual_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&sender).Error)

	mk := func(number, status, confirmation string, sent *time.Time, forwards int) {
		n, s, c := number, status, confirmation
		app := models.Application{
			ApplicationNumber: &n,
			OrganizationID:    org.ID,
			SenderUserID:      sender.ID,
			Status:            &s,
			Confirmation:      &c,
			SendingDatetime:   sent,
		}
		require.NoError(t, db.Create(&app).Error)

		// Пересылка = сводная запись audit_log (одна на действие), её и считает
		// avg_forwards. Записи по-получательские (assigned_*) метрика игнорирует.
		for i := 0; i < forwards; i++ {
			id := app.ID
			require.NoError(t, db.Create(&models.AuditLog{
				EntityType:  models.AuditEntityApplication,
				EntityID:    &id,
				Action:      models.AuditActionForwarded,
				ActorUserID: &sender.ID,
			}).Error)
		}
	}

	mk("Q/A", models.StatusInWork, models.ConfirmationApproved, mskTime(t, "2026-06-01 10:00"), 3)
	mk("Q/B", models.StatusRefused, models.ConfirmationApproved, mskTime(t, "2026-06-02 10:00"), 2)
	mk("Q/C", models.StatusProcessing, models.ConfirmationRejected, mskTime(t, "2026-06-03 10:00"), 1)
	mk("Q/D", models.StatusCompleted, models.ConfirmationApproved, mskTime(t, "2026-06-04 10:00"), 0)
}

// seedApproverVotes раздаёт голоса согласующих по заявкам сида качества:
//
//	Иванов:  A назначен 10:00 -> голос 12:00 (2ч), B назначен 10:00 -> голос 14:00 (4ч)
//	Петров:  C назначен 10:00 -> голос 11:00 (1ч), D назначен, но ещё НЕ ответил
//
// Ожидаемо: Иванов - реакция 3ч, нагрузка 2; Петров - реакция 1ч, нагрузка 2
// (неотданный голос считается нагрузкой, но в среднее время реакции не входит).
//
// Время реакции с #1251 S2 считается по рабочему времени Бюро, но тесты голосов не
// заводят график bureau_time_slots -> метрика падает на календарный фолбэк, поэтому
// часы реакции равны календарной разнице. Вычет нерабочих часов и фолбэк проверяет
// TestRunReport_ApproverResponseTime_BureauWorkingTime.
func seedApproverVotes(t *testing.T, db *gorm.DB) {
	t.Helper()

	mkUser := func(username, last, first string) models.User {
		l, f := last, first
		u := models.User{Username: username, TypeID: 1, IsActive: true, LastName: &l, FirstName: &f}
		require.NoError(t, db.Create(&u).Error)
		return u
	}
	ivanov := mkUser("appr_ivanov", "Иванов", "Иван")
	petrov := mkUser("appr_petrov", "Петров", "Пётр")

	appID := func(number string) int {
		var app models.Application
		require.NoError(t, db.Where("application_number = ?", number).First(&app).Error)
		return app.ID
	}

	vote := func(number string, u models.User, assigned, approved *time.Time) {
		row := models.ApplicationResponsibleUser{
			ApplicationID:    appID(number),
			UserID:           u.ID,
			CreatedAt:        *assigned, // GORM не перетирает заданный CreatedAt
			ApprovalDatetime: approved,
		}
		require.NoError(t, db.Create(&row).Error)
	}

	vote("Q/A", ivanov, mskTime(t, "2026-06-01 10:00"), mskTime(t, "2026-06-01 12:00"))
	vote("Q/B", ivanov, mskTime(t, "2026-06-02 10:00"), mskTime(t, "2026-06-02 14:00"))
	vote("Q/C", petrov, mskTime(t, "2026-06-03 10:00"), mskTime(t, "2026-06-03 11:00"))
	vote("Q/D", petrov, mskTime(t, "2026-06-04 10:00"), nil)
}

func qualityWindow() []models.ReportFilterValue {
	return []models.ReportFilterValue{{Key: "date_range", From: "2026-06-01", To: "2026-06-04"}}
}

// TestRunReport_QualityMetrics_Values — доля отказов и среднее число пересылок на
// заведомых данных. Дробное значение обязано приехать в FloatValues: SQL везёт его
// целым (домноженным), и без обратного деления клиент увидел бы 500 вместо 50%.
func TestRunReport_QualityMetrics_Values(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)

	svc := services.NewStatisticsService(db, 0)

	cases := []struct {
		metric string
		want   float64
		unit   string
		why    string
	}{
		{"refusal_rate", 50, "%", "2 отказа (статус + несогласование) из 4 заявок"},
		// Те же данные по отдельности (#1251 polish, п.8): отказал принимающий у B,
		// не согласовали у C. Ветки не взаимоисключающие, сумма долей совпала с
		// объединённой здесь случайно - на пересекающихся данных совпадать не обязана.
		{"rejected_rate", 25, "%", "статус «Отказано» у одной заявки из 4"},
		{"not_approved_rate", 25, "%", "confirmation «Не согласовано» у одной заявки из 4"},
		{"avg_forwards", 1.5, "раз/заявку", "(3+2+1+0)/4 пересылок на заявку"},
	}

	for _, c := range cases {
		t.Run(c.metric, func(t *testing.T) {
			res, err := svc.RunReport(context.Background(), models.ReportRequest{
				Mode:      "aggregate",
				Metric:    c.metric,
				Dimension: "none",
				Filters:   qualityWindow(),
			})
			require.NoError(t, err, "SQL метрики должен исполняться")

			require.Len(t, res.MetricRows, 1)
			assert.Equal(t, c.want, res.MetricRows[0].FloatValues[c.metric], c.why)
			assert.Equal(t, c.want, res.FloatTotals[c.metric], "итог без разреза = значение единственной строки")

			require.Len(t, res.Columns, 1)
			assert.True(t, res.Columns[0].Float, "метрика дробная -> значение в float_values")
			assert.Equal(t, c.unit, res.Columns[0].Unit)
			assert.NotContains(t, res.MetricRows[0].Values, c.metric,
				"домноженное целое — деталь транспорта, наружу выходить не должно")
		})
	}
}

// TestRunReport_RefusalBranches_NotAdditiveOnOverlap — ради чего ветки разводили:
// заявку могли И не согласовать, И получить отказ принимающего. На таких данных
// сумма двух долей НЕ равна объединённой refusal_rate (та считает заявку один раз).
// Общий сид качества этого не ловит: там ветки лежат на разных заявках.
func TestRunReport_RefusalBranches_NotAdditiveOnOverlap(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-Пересечение", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	sender := models.User{Username: "ovl_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&sender).Error)

	mk := func(number, status, confirmation string, sent *time.Time) {
		n, s, c := number, status, confirmation
		require.NoError(t, db.Create(&models.Application{
			ApplicationNumber: &n, OrganizationID: org.ID, SenderUserID: sender.ID,
			Status: &s, Confirmation: &c, SendingDatetime: sent,
		}).Error)
	}
	// Одна заявка сразу с обеими ветками, одна чистая.
	mk("OVL/BOTH", models.StatusRefused, models.ConfirmationRejected, mskTime(t, "2026-06-01 10:00"))
	mk("OVL/OK", models.StatusInWork, models.ConfirmationApproved, mskTime(t, "2026-06-02 10:00"))

	svc := services.NewStatisticsService(db, 0)
	rate := func(metric string) float64 {
		res, err := svc.RunReport(context.Background(), models.ReportRequest{
			Mode: "aggregate", Metric: metric, Dimension: "none", Filters: qualityWindow(),
		})
		require.NoError(t, err)
		require.Len(t, res.MetricRows, 1)
		return res.MetricRows[0].FloatValues[metric]
	}

	rejected := rate("rejected_rate")
	notApproved := rate("not_approved_rate")
	combined := rate("refusal_rate")

	assert.Equal(t, float64(50), rejected, "отказ принимающего у 1 заявки из 2")
	assert.Equal(t, float64(50), notApproved, "несогласование у той же 1 заявки из 2")
	assert.Equal(t, float64(50), combined, "объединённая считает эту заявку ОДИН раз")
	assert.NotEqual(t, rejected+notApproved, combined,
		"ветки пересекаются - складывать доли нельзя, ради этого их и развели")
}

// TestRunReport_QualityMetrics_PeriodTotal — итог по разрезу period считается по
// всему окну, а НЕ сложением бинов. Для доли это принципиально: дни дают 0/100/100/0,
// и сумма (200%) была бы бессмыслицей вместо честных 50%.
func TestRunReport_QualityMetrics_PeriodTotal(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:        "aggregate",
		Metrics:     []string{"refusal_rate", "avg_forwards"},
		Dimension:   "period",
		Granularity: "day",
		Filters:     qualityWindow(),
	})
	require.NoError(t, err)

	require.Len(t, res.MetricRows, 4, "четыре дня подачи -> четыре бина")
	wantRate := []float64{0, 100, 100, 0}
	wantFwd := []float64{3, 2, 1, 0}
	for i, row := range res.MetricRows {
		assert.Equal(t, wantRate[i], row.FloatValues["refusal_rate"], "доля отказов %s", row.Label)
		assert.Equal(t, wantFwd[i], row.FloatValues["avg_forwards"], "пересылки %s", row.Label)
	}

	assert.Equal(t, float64(50), res.FloatTotals["refusal_rate"],
		"итог = доля по всем заявкам окна, а не сумма долей по дням (200%)")
	assert.Equal(t, 1.5, res.FloatTotals["avg_forwards"],
		"итог = среднее по всем заявкам окна, а не сумма средних по дням (6)")
}

// TestRunReport_QualityMetrics_EmptyWindow — на окне без заявок агрегат отдаёт 0,
// а не NULL: без COALESCE скан NULL в int64 падает в рантайме.
func TestRunReport_QualityMetrics_EmptyWindow(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	for _, metric := range []string{"refusal_rate", "avg_forwards"} {
		t.Run(metric, func(t *testing.T) {
			res, err := svc.RunReport(context.Background(), models.ReportRequest{
				Mode:      "aggregate",
				Metric:    metric,
				Dimension: "none",
				Filters: []models.ReportFilterValue{
					{Key: "date_range", From: "2030-01-01", To: "2030-01-31"},
				},
			})
			require.NoError(t, err, "пустая выборка не должна ронять скан")
			require.Len(t, res.MetricRows, 1)
			assert.Equal(t, float64(0), res.MetricRows[0].FloatValues[metric])
		})
	}
}

// TestRunReport_QualityMetrics_NoFakeZeroInForeignBin — доля не счётчик: в бине,
// где заявок не было вовсе, она неопределена, а не «0% отказов». Бины здесь бьёт
// метрика другой базы (въезды машин идут по своим датам), и дорисованный ноль
// врал бы на графике рядом с ней.
func TestRunReport_QualityMetrics_NoFakeZeroInForeignBin(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:        "aggregate",
		Metrics:     []string{"applications_count", "refusal_rate"},
		Dimension:   "period",
		Granularity: "day",
		Filters:     qualityWindow(),
	})
	require.NoError(t, err)
	require.Len(t, res.MetricRows, 4)

	// Все бины пришли от заявок -> доля определена в каждом (в т.ч. честный 0).
	for _, row := range res.MetricRows {
		assert.Contains(t, row.FloatValues, "refusal_rate",
			"в бине есть заявки -> доля определена (%s)", row.Label)
	}
	assert.Equal(t, float64(0), res.MetricRows[0].FloatValues["refusal_rate"],
		"01.06: заявка есть, отказов нет -> честный ноль")
	assert.Equal(t, int64(1), res.MetricRows[0].Values["applications_count"])
}

// TestRunReport_ApproverMetrics_ByApprover — разрез по согласующему на реальных
// join'ах: строка на согласующего с его временем реакции и нагрузкой.
func TestRunReport_ApproverMetrics_ByApprover(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:      "aggregate",
		Metrics:   []string{"avg_approver_response_time", "approver_votes_count"},
		Dimension: "by_approver",
		Filters:   qualityWindow(),
	})
	require.NoError(t, err, "SQL разреза по согласующему должен исполняться")

	require.Len(t, res.MetricRows, 2, "два согласующих -> две строки")
	byName := make(map[string]models.ReportMetricRow, 2)
	for _, r := range res.MetricRows {
		byName[r.Label] = r
	}

	ivanov, ok := byName["Иванов Иван"]
	require.True(t, ok, "подпись строки — ФИО согласующего, получено: %v", byName)
	assert.Equal(t, int64(10800), ivanov.Values["avg_approver_response_time"], "(2ч+4ч)/2")
	assert.Equal(t, int64(2), ivanov.Values["approver_votes_count"])

	petrov, ok := byName["Петров Пётр"]
	require.True(t, ok)
	assert.Equal(t, int64(3600), petrov.Values["avg_approver_response_time"],
		"неотданный голос в среднее время реакции не входит — иначе «ещё думает» = мгновенный ответ")
	assert.Equal(t, int64(2), petrov.Values["approver_votes_count"],
		"нагрузка считает и неотданный голос: заявку на согласующего завели")

	assert.Equal(t, models.ReportValueDuration, res.Columns[0].Type,
		"время реакции — длительность, по типу колонки фронт выбирает формат")
	assert.Equal(t, int64(4), res.Totals["approver_votes_count"], "итог счётчика — сумма строк")
	assert.Equal(t, int64(8400), res.Totals["avg_approver_response_time"],
		"итог пересчитан по всем голосам окна ((2ч+4ч+1ч)/3), а не сложен из строк (14400) "+
			"и не усреднён по строкам (7200)")
}

// TestRunReport_ApproverResponseTime_ExcludesNegativePairs — тот же класс, что у
// длительностей этапов: на исторических данных голос согласующего может стоять
// раньше его назначения (approval_datetime < created_at), давая отрицательное
// время реакции. Битый голос обязан выпасть из среднего, а не тянуть его в минус.
func TestRunReport_ApproverResponseTime_ExcludesNegativePairs(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-Апрувер", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	sender := models.User{Username: "ar_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&sender).Error)
	l, f := "Реакция", "Тест"
	appr := models.User{Username: "ar_appr", TypeID: 1, IsActive: true, LastName: &l, FirstName: &f}
	require.NoError(t, db.Create(&appr).Error)

	status := models.StatusInWork
	mkApp := func(number string, sent *time.Time) int {
		n := number
		app := models.Application{
			ApplicationNumber: &n,
			OrganizationID:    org.ID,
			SenderUserID:      sender.ID,
			Status:            &status,
			SendingDatetime:   sent,
		}
		require.NoError(t, db.Create(&app).Error)
		return app.ID
	}
	vote := func(appID int, assigned, approved *time.Time) {
		row := models.ApplicationResponsibleUser{
			ApplicationID:    appID,
			UserID:           appr.ID,
			CreatedAt:        *assigned,
			ApprovalDatetime: approved,
		}
		require.NoError(t, db.Create(&row).Error)
	}

	win := []models.ReportFilterValue{{Key: "date_range", From: "2026-06-01", To: "2026-06-03"}}
	// Корректный голос: реакция 2ч (7200с).
	okApp := mkApp("AR/OK", mskTime(t, "2026-06-01 10:00"))
	vote(okApp, mskTime(t, "2026-06-01 10:00"), mskTime(t, "2026-06-01 12:00"))
	// Битый: голос РАНЬШЕ назначения на час — длительность была бы -3600.
	badApp := mkApp("AR/BAD", mskTime(t, "2026-06-02 10:00"))
	vote(badApp, mskTime(t, "2026-06-02 10:00"), mskTime(t, "2026-06-02 09:00"))

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:      "aggregate",
		Metric:    "avg_approver_response_time",
		Dimension: "none",
		Filters:   win,
	})
	require.NoError(t, err)
	require.Len(t, res.MetricRows, 1)
	assert.Equal(t, int64(7200), res.MetricRows[0].Values["avg_approver_response_time"],
		"битый голос (approval<created) исключён, среднее не уходит в минус")
}

// TestRunReport_ApproverMetrics_Dimensions — прочие разрезы согласующих идут через
// тот же join к заявке (окно, статус, организация) и должны исполняться.
func TestRunReport_ApproverMetrics_Dimensions(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)

	svc := services.NewStatisticsService(db, 0)
	for _, dim := range []string{"organization", "company", "status", "period", "none"} {
		t.Run(dim, func(t *testing.T) {
			res, err := svc.RunReport(context.Background(), models.ReportRequest{
				Mode:        "aggregate",
				Metric:      "approver_votes_count",
				Dimension:   dim,
				Granularity: "day",
				Filters:     qualityWindow(),
			})
			require.NoError(t, err, "разрез %q должен исполняться", dim)
			assert.Equal(t, int64(4), res.Totals["approver_votes_count"],
				"все четыре голоса окна на месте при разрезе %q", dim)
		})
	}
}

// TestRunReport_ByApprover_RejectedForApplicationMetrics — разрез по согласующему
// метрикам заявки не даётся: 1 заявка : N согласующих размножила бы её по числу
// голосов и взвесила бы среднее. Каталог его не публикует -> движок отбивает.
func TestRunReport_ByApprover_RejectedForApplicationMetrics(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	svc := services.NewStatisticsService(db, 0)
	for _, metric := range []string{"applications_count", "refusal_rate", "avg_approval_time"} {
		t.Run(metric, func(t *testing.T) {
			_, err := svc.RunReport(context.Background(), models.ReportRequest{
				Mode:      "aggregate",
				Metric:    metric,
				Dimension: "by_approver",
				Filters:   qualityWindow(),
			})
			require.ErrorIs(t, err, services.ErrInvalidReportRequest)
		})
	}
}

// TestReportCatalog_QualityAndApproverMetrics — метрики публикуются каталогом
// (иначе движок их исполняет, а гид не показывает) в своих группах и с разрезами,
// которые движок умеет резолвить.
func TestReportCatalog_QualityAndApproverMetrics(t *testing.T) {
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

	rate, ok := byKey["refusal_rate"]
	require.True(t, ok, "каталог должен публиковать долю отказов")
	assert.Equal(t, "Обработка заявок", rate.Group)
	assert.Equal(t, "Доля отказов и несогласований", rate.Label,
		"объединённая доля осталась в каталоге, но подпись уточнена - рядом живут её ветки по отдельности")
	assert.NotContains(t, rate.Dimensions, "status",
		"разрез доли отказов по статусу тавтологичен: группа «Отказано» дала бы 100%")
	assert.Contains(t, rate.Filters, "status", "как фильтр статус остаётся")

	// Ветки отказа по отдельности: разрезы у них РАЗНЫЕ (у отказа принимающего
	// статус тавтологичен и убран, у несогласования — осмыслен и оставлен).
	rejected, ok := byKey["rejected_rate"]
	require.True(t, ok, "каталог должен публиковать долю отказов принимающего")
	assert.Equal(t, "Доля отказов принимающего", rejected.Label)
	assert.Equal(t, "Обработка заявок", rejected.Group)
	assert.NotContains(t, rejected.Dimensions, "status",
		"группировка отказа принимающего по статусу тавтологична: группа «Отказано» даст 100%")

	notApproved, ok := byKey["not_approved_rate"]
	require.True(t, ok, "каталог должен публиковать долю несогласованных")
	assert.Equal(t, "Доля несогласованных", notApproved.Label)
	assert.Contains(t, notApproved.Dimensions, "status",
		"несогласование фильтрует confirmation, разрез по статусу здесь не тавтологичен")

	fwd, ok := byKey["avg_forwards"]
	require.True(t, ok)
	assert.Equal(t, "Обработка заявок", fwd.Group)
	assert.Contains(t, fwd.Dimensions, "status")

	for _, key := range []string{"avg_approver_response_time", "approver_votes_count"} {
		m, ok := byKey[key]
		require.True(t, ok, "каталог должен публиковать метрику %q", key)
		assert.Equal(t, "Согласующие", m.Group)
		assert.Contains(t, m.Dimensions, "by_approver")
		assert.NotEmpty(t, m.Label)
	}

	dims := make(map[string]string, len(cat.Dimensions))
	for _, d := range cat.Dimensions {
		dims[d.Key] = d.Label
	}
	assert.Equal(t, "Согласующий", dims["by_approver"], "разрез должен быть в каталоге с подписью")
}

// TestRunReport_ApproverResponseTime_BureauWorkingTime — время реакции согласующего
// считается по РАБОЧЕМУ времени Бюро (#1251 S2), как и этапы заявки: при графике
// ночь и выходные вычитаются, при пустом графике — календарный фолбэк. Голос,
// назначенный в пятницу вечером и отданный в понедельник, показывает контраст.
// setBureauSchedule живёт в statistics_report_duration_integration_test.go (тот же
// пакет handlers_test).
func TestRunReport_ApproverResponseTime_BureauWorkingTime(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-Реакция-Раб", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	sender := models.User{Username: "wr_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&sender).Error)
	l, f := "Реакция", "Рабочая"
	appr := models.User{Username: "wr_appr", TypeID: 1, IsActive: true, LastName: &l, FirstName: &f}
	require.NoError(t, db.Create(&appr).Error)

	status := models.StatusInWork
	number := "WR/1"
	app := models.Application{
		ApplicationNumber: &number,
		OrganizationID:    org.ID,
		SenderUserID:      sender.ID,
		Status:            &status,
		SendingDatetime:   mskTime(t, "2026-06-19 16:00"),
	}
	require.NoError(t, db.Create(&app).Error)
	// Назначен Пт 19.06 16:00, проголосовал Пн 22.06 11:00.
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
		ApplicationID:    app.ID,
		UserID:           appr.ID,
		CreatedAt:        *mskTime(t, "2026-06-19 16:00"),
		ApprovalDatetime: mskTime(t, "2026-06-22 11:00"),
	}).Error)

	win := []models.ReportFilterValue{{Key: "date_range", From: "2026-06-19", To: "2026-06-19"}}
	svc := services.NewStatisticsService(db, 0)
	runAvg := func() int64 {
		res, err := svc.RunReport(context.Background(), models.ReportRequest{
			Mode: "aggregate", Metric: "avg_approver_response_time", Dimension: "none", Filters: win,
		})
		require.NoError(t, err, "SQL времени реакции должен исполняться")
		require.Len(t, res.MetricRows, 1)
		return res.MetricRows[0].Values["avg_approver_response_time"]
	}

	// Пустой график: Пт16:00 -> Пн11:00 = 67ч календарно.
	assert.Equal(t, int64(67*3600), runAvg(), "пустой график -> календарный фолбэк")

	// График Пн-Пт 09:00-18:00: Пт 16-18 (2ч) + Пн 09-11 (2ч) = 4ч.
	setBureauSchedule(t, db, []int{0, 1, 2, 3, 4}, "09:00", "18:00")
	assert.Equal(t, int64(4*3600), runAvg(), "рабочее время: ночь и выходные вычтены")
}
