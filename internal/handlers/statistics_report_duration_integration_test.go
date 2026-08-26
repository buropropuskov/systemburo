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

// Метрики длительностей (#1240, B2) на РЕАЛЬНОМ SQL: агрегаты собираются из
// констант движка и исполняются GORM'ом против PG. Билд такие запросы не
// проверяет (EXTRACT/PERCENTILE_CONT/скан numeric в int64 падают только при
// исполнении), поэтому каждая метрика гоняется через RunReport на заведомых
// данных и сверяется с посчитанным вручную числом.
//
// Согласование/принятие/обработка с #1251 S2 считаются по рабочему времени Бюро
// (bureauWorkingDuration), но тесты значений ниже НЕ заводят график bureau_time_slots
// (CleanDB его чистит) -> метрика падает на календарный фолбэк, поэтому числа сидов
// равны календарной разнице и остались прежними. Сам вычет нерабочих часов и фолбэк
// проверяет TestRunReport_DurationMetrics_BureauWorkingTime.

// mskTime — момент в московской зоне (в ней же движок бьёт бины и границы окна).
func mskTime(t *testing.T, s string) *time.Time {
	t.Helper()
	ts, err := time.ParseInLocation("2006-01-02 15:04", s, services.AnalyticsLocation())
	require.NoError(t, err)
	return &ts
}

// seedDurationApps заводит три заявки с известными длительностями этапов:
//
//	A: согласование 2ч,  принятие 1ч,  обработка 3ч,  завершение 24ч
//	B: согласование 4ч,  принятие 2ч,  обработка 6ч,  не завершена
//	C: согласование 10ч, не принята,   не обработана, не завершена
//
// Незавершённые этапы (NULL-момент) в агрегат попадать не должны.
func seedDurationApps(t *testing.T, db *gorm.DB) int {
	t.Helper()

	org := models.Organization{Name: "Орг-Длительности", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "dur_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	status := models.StatusInWork
	mk := func(number string, sent, confirmed, accepted, completed *time.Time) {
		n := number
		app := models.Application{
			ApplicationNumber:    &n,
			OrganizationID:       org.ID,
			SenderUserID:         user.ID,
			Status:               &status,
			SendingDatetime:      sent,
			ConfirmationDatetime: confirmed,
			AcceptedAt:           accepted,
			CompletedAt:          completed,
		}
		require.NoError(t, db.Create(&app).Error)
	}

	mk("D/A", mskTime(t, "2026-06-01 10:00"), mskTime(t, "2026-06-01 12:00"),
		mskTime(t, "2026-06-01 13:00"), mskTime(t, "2026-06-02 10:00"))
	mk("D/B", mskTime(t, "2026-06-02 10:00"), mskTime(t, "2026-06-02 14:00"),
		mskTime(t, "2026-06-02 16:00"), nil)
	mk("D/C", mskTime(t, "2026-06-03 10:00"), mskTime(t, "2026-06-03 20:00"), nil, nil)

	return org.ID
}

func durationWindow() []models.ReportFilterValue {
	return []models.ReportFilterValue{{Key: "date_range", From: "2026-06-01", To: "2026-06-03"}}
}

// TestRunReport_DurationMetrics_Values гоняет КАЖДУЮ из 12 метрик длительностей и
// сверяет с числом, посчитанным вручную по данным сида. Заодно ловит,
// что заявки с непройденным этапом не считаются нулём.
func TestRunReport_DurationMetrics_Values(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedDurationApps(t, db)

	svc := services.NewStatisticsService(db, 0)

	cases := []struct {
		metric string
		want   int64
		why    string
	}{
		// Согласование: 2ч, 4ч, 10ч -> 7200, 14400, 36000.
		{"avg_approval_time", 19200, "(7200+14400+36000)/3"},
		{"p50_approval_time", 14400, "медиана трёх значений"},
		{"p90_approval_time", 31680, "интерполяция между 14400 и 36000"},
		// Принятие (только A и B прошли этап): 1ч и 2ч.
		{"avg_acceptance_time", 5400, "(3600+7200)/2"},
		{"p50_acceptance_time", 5400, "медиана двух значений"},
		{"p90_acceptance_time", 6840, "3600 + 0.9*(7200-3600)"},
		// Обработка (A и B): 3ч и 6ч.
		{"avg_processing_time", 16200, "(10800+21600)/2"},
		{"p50_processing_time", 16200, "медиана двух значений"},
		{"p90_processing_time", 20520, "10800 + 0.9*(21600-10800)"},
		// Завершение (только A): 24ч.
		{"avg_completion_time", 86400, "единственная завершённая заявка"},
		{"p50_completion_time", 86400, "единственное значение"},
		{"p90_completion_time", 86400, "единственное значение"},
	}

	for _, c := range cases {
		t.Run(c.metric, func(t *testing.T) {
			res, err := svc.RunReport(context.Background(), models.ReportRequest{
				Mode:      "aggregate",
				Metric:    c.metric,
				Dimension: "none",
				Filters:   durationWindow(),
			})
			require.NoError(t, err, "SQL метрики должен исполняться")

			require.Len(t, res.MetricRows, 1)
			assert.Equal(t, c.want, res.MetricRows[0].Values[c.metric], "секунды: %s", c.why)
			assert.Equal(t, c.want, res.Totals[c.metric], "итог без разреза = значение единственной строки")

			require.Len(t, res.Columns, 1)
			assert.Equal(t, models.ReportValueDuration, res.Columns[0].Type,
				"колонка должна быть помечена duration — по ней фронт выбирает формат")
			assert.Empty(t, res.Columns[0].Unit, "длительность форматируется целиком, суффикс единицы не нужен")
			assert.False(t, res.Columns[0].Float, "секунды целые, значение лежит в values")
		})
	}
}

// TestRunReport_DurationMetrics_PeriodTotal — итог по разрезу period считается по
// всему окну, а НЕ складыванием значений бинов: сумма средних не среднее, а
// перцентили не складываются вовсе.
func TestRunReport_DurationMetrics_PeriodTotal(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedDurationApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:        "aggregate",
		Metrics:     []string{"avg_approval_time", "p90_approval_time"},
		Dimension:   "period",
		Granularity: "day",
		Filters:     durationWindow(),
	})
	require.NoError(t, err)

	// По заявке в день: бины несут длительность своей заявки.
	require.Len(t, res.MetricRows, 3, "три дня подачи -> три бина")
	assert.Equal(t, "2026-06-01", res.MetricRows[0].Label)
	assert.Equal(t, int64(7200), res.MetricRows[0].Values["avg_approval_time"])
	assert.Equal(t, int64(14400), res.MetricRows[1].Values["avg_approval_time"])
	assert.Equal(t, int64(36000), res.MetricRows[2].Values["avg_approval_time"])

	assert.Equal(t, int64(19200), res.Totals["avg_approval_time"],
		"итог = среднее по всем заявкам окна, а не сумма средних по дням (57600)")
	assert.Equal(t, int64(31680), res.Totals["p90_approval_time"],
		"итог = перцентиль по всем заявкам окна, а не сумма перцентилей бинов")
	assert.Equal(t, int64(19200), res.Total, "legacy-итог первой метрики согласован с Totals")
}

// TestRunReport_DurationMetrics_MixedStagesNoFakeZero — в мультиметрике этапы
// имеют РАЗНОЕ покрытие (заявку согласовали, но ещё не завершили), и бин, где
// этап не пройден, обязан остаться БЕЗ значения. Ноль тут неотличим от «прошло
// мгновенно» и на графике рисует падение до нуля вместо разрыва.
func TestRunReport_DurationMetrics_MixedStagesNoFakeZero(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedDurationApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:        "aggregate",
		Metrics:     []string{"avg_approval_time", "avg_completion_time"},
		Dimension:   "period",
		Granularity: "day",
		Filters:     durationWindow(),
	})
	require.NoError(t, err)
	require.Len(t, res.MetricRows, 3)

	// 01.06 — заявка A: согласована и завершена, оба этапа пройдены.
	first := res.MetricRows[0]
	require.Equal(t, "2026-06-01", first.Label)
	assert.Equal(t, int64(7200), first.Values["avg_approval_time"])
	assert.Equal(t, int64(86400), first.Values["avg_completion_time"])

	// 02.06 и 03.06 — заявки B и C согласованы, но не завершены.
	for _, i := range []int{1, 2} {
		row := res.MetricRows[i]
		assert.Contains(t, row.Values, "avg_approval_time",
			"этап согласования пройден -> значение есть (%s)", row.Label)
		assert.NotContains(t, row.Values, "avg_completion_time",
			"этап не пройден -> ключа быть не должно, 0 читался бы как «завершено мгновенно» (%s)", row.Label)
	}
}

// TestRunReport_DurationMetrics_EmptyWindow — на окне без данных агрегат отдаёт 0,
// а не NULL: без COALESCE скан NULL в int64 падает в рантайме.
func TestRunReport_DurationMetrics_EmptyWindow(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedDurationApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	for _, metric := range []string{"avg_approval_time", "p50_approval_time", "p90_completion_time"} {
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
			assert.Equal(t, int64(0), res.MetricRows[0].Values[metric])
			assert.Equal(t, int64(0), res.Totals[metric])
		})
	}
}

// TestRunReport_DurationMetrics_Dimensions проверяет разрезы длительностей на
// реальных join'ах: организация (LEFT JOIN) и статус (поле заявки).
func TestRunReport_DurationMetrics_Dimensions(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedDurationApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	for _, dim := range []string{"organization", "company", "status"} {
		t.Run(dim, func(t *testing.T) {
			res, err := svc.RunReport(context.Background(), models.ReportRequest{
				Mode:      "aggregate",
				Metric:    "avg_approval_time",
				Dimension: dim,
				Filters:   durationWindow(),
			})
			require.NoError(t, err, "разрез %q должен исполняться", dim)
			require.Len(t, res.MetricRows, 1, "все заявки сида в одной группе")
			assert.Equal(t, int64(19200), res.MetricRows[0].Values["avg_approval_time"])
		})
	}
}

// TestRunReport_DurationMetrics_Filter — фильтр по организации применяется к
// длительностям (join не размножает строки и не искажает среднее).
func TestRunReport_DurationMetrics_Filter(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedDurationApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:      "aggregate",
		Metric:    "avg_approval_time",
		Dimension: "none",
		Filters: append(durationWindow(),
			models.ReportFilterValue{Key: "organization", Values: []string{"Орг-Длительности"}}),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(19200), res.MetricRows[0].Values["avg_approval_time"])

	res, err = svc.RunReport(context.Background(), models.ReportRequest{
		Mode:      "aggregate",
		Metric:    "avg_approval_time",
		Dimension: "none",
		Filters: append(durationWindow(),
			models.ReportFilterValue{Key: "organization", Values: []string{"Другая орг"}}),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(0), res.MetricRows[0].Values["avg_approval_time"], "нет заявок -> 0")
}

// optMskTime — момент из строки или nil для пустой (незаполненный этап).
func optMskTime(t *testing.T, s string) *time.Time {
	t.Helper()
	if s == "" {
		return nil
	}
	return mskTime(t, s)
}

// TestRunReport_DurationMetrics_ExcludesNegativePairs — у части исторических
// заявок конечный момент этапа раньше начального (напр. confirmation_datetime до
// sending_datetime), что даёт отрицательную длительность и утягивает среднее в
// минус. Такая пара обязана выпасть из выборки КАЖДОГО этапа, а не клампиться в
// ноль. Проверяем все четыре этапа: корректная заявка + битая пара, среднее = только
// корректная (sending всегда задан — по нему метрика фильтрует окно).
func TestRunReport_DurationMetrics_ExcludesNegativePairs(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	type appMoments struct{ sent, conf, acc, comp string }
	cases := []struct {
		metric  string
		wantSec int64
		ok, bad appMoments
	}{
		{
			"avg_approval_time", 7200, // согласование 2ч
			appMoments{sent: "2026-06-01 10:00", conf: "2026-06-01 12:00"},
			appMoments{sent: "2026-06-02 10:00", conf: "2026-06-02 09:00"}, // conf < sent
		},
		{
			"avg_acceptance_time", 3600, // принятие 1ч
			appMoments{sent: "2026-06-01 09:00", conf: "2026-06-01 10:00", acc: "2026-06-01 11:00"},
			appMoments{sent: "2026-06-02 09:00", conf: "2026-06-02 10:00", acc: "2026-06-02 09:30"}, // acc < conf
		},
		{
			"avg_processing_time", 10800, // обработка 3ч
			appMoments{sent: "2026-06-01 10:00", acc: "2026-06-01 13:00"},
			appMoments{sent: "2026-06-02 10:00", acc: "2026-06-02 09:00"}, // acc < sent
		},
		{
			"avg_completion_time", 86400, // до завершения 24ч
			appMoments{sent: "2026-06-01 10:00", comp: "2026-06-02 10:00"},
			appMoments{sent: "2026-06-03 10:00", comp: "2026-06-02 10:00"}, // comp < sent
		},
	}

	for _, c := range cases {
		t.Run(c.metric, func(t *testing.T) {
			testutil.CleanDB(t, db)
			org := models.Organization{Name: "Орг-Битая", IsActive: true}
			require.NoError(t, db.Create(&org).Error)
			user := models.User{Username: "neg_sender", TypeID: 1, IsActive: true}
			require.NoError(t, db.Create(&user).Error)

			status := models.StatusInWork
			mk := func(number string, m appMoments) {
				n := number
				app := models.Application{
					ApplicationNumber:    &n,
					OrganizationID:       org.ID,
					SenderUserID:         user.ID,
					Status:               &status,
					SendingDatetime:      optMskTime(t, m.sent),
					ConfirmationDatetime: optMskTime(t, m.conf),
					AcceptedAt:           optMskTime(t, m.acc),
					CompletedAt:          optMskTime(t, m.comp),
				}
				require.NoError(t, db.Create(&app).Error)
			}
			mk("N/OK", c.ok)
			mk("N/BAD", c.bad)

			svc := services.NewStatisticsService(db, 0)
			res, err := svc.RunReport(context.Background(), models.ReportRequest{
				Mode:      "aggregate",
				Metric:    c.metric,
				Dimension: "none",
				Filters:   durationWindow(),
			})
			require.NoError(t, err)
			require.Len(t, res.MetricRows, 1)
			// Только корректная заявка: без отсечения среднее уехало бы в минус.
			assert.Equal(t, c.wantSec, res.MetricRows[0].Values[c.metric],
				"битая пара (конец этапа < начало) исключена, среднее не уходит в минус")
		})
	}
}

// TestReportCatalog_DurationMetrics — метрики длительностей публикуются каталогом
// (иначе движок их исполняет, а гид не показывает) в своей группе и с разрезами,
// которые движок умеет резолвить.
func TestReportCatalog_DurationMetrics(t *testing.T) {
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

	for _, key := range []string{"avg_approval_time", "p50_acceptance_time", "p90_completion_time"} {
		m, ok := byKey[key]
		require.True(t, ok, "каталог должен публиковать метрику %q", key)
		assert.Equal(t, "Обработка заявок", m.Group)
		assert.NotEmpty(t, m.Label)
		assert.Contains(t, m.Dimensions, "period")
		assert.NotContains(t, m.Dimensions, "attachment_type",
			"тип вложения размножил бы заявку по числу вложений и взвесил бы среднее")
	}
	assert.Equal(t, "Среднее время согласования", byKey["avg_approval_time"].Label)
	assert.Equal(t, "90-й перцентиль времени до завершения", byKey["p90_completion_time"].Label)
}

// setBureauSchedule заменяет график Бюро слотами на дни недели dows (0=Пн..6=Вс) с
// общими часами. Тесты рабочего времени задают ограниченный график, чтобы вычет
// нерабочих часов был виден; БЕЗ вызова график пуст и bureauWorkingDuration падает
// на календарный фолбэк (по нему совпадают числа сидов длительностей выше).
func setBureauSchedule(t *testing.T, db *gorm.DB, dows []int, open, close string) {
	t.Helper()
	require.NoError(t, db.Exec("DELETE FROM bureau_time_slots").Error)
	for _, dow := range dows {
		slot := models.BureauTimeSlot{DayOfWeek: dow, OpenTime: open, CloseTime: close, IsActive: true}
		require.NoError(t, db.Create(&slot).Error)
	}
}

// TestRunReport_DurationMetrics_BureauWorkingTime — этапы работы людей Бюро
// (согласование/принятие/обработка) считаются по РАБОЧЕМУ времени (#1251 S2): при
// заведённом графике ночь и выходные вычитаются, при пустом — календарный фолбэк.
// «Время до завершения» остаётся календарным в обеих ветках (срок пропуска, а не
// работа человека). Одна заявка, поданная в пятницу вечером и принятая во вторник,
// показывает контраст. Дни недели сверены в TestBureauWorkingSeconds (19.06 Пт,
// 22.06 Пн, 23.06 Вт, 24.06 Ср).
func TestRunReport_DurationMetrics_BureauWorkingTime(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-Рабочее", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "work_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	status := models.StatusInWork
	number := "W/1"
	app := models.Application{
		ApplicationNumber:    &number,
		OrganizationID:       org.ID,
		SenderUserID:         user.ID,
		Status:               &status,
		SendingDatetime:      mskTime(t, "2026-06-19 16:00"),
		ConfirmationDatetime: mskTime(t, "2026-06-22 11:00"),
		AcceptedAt:           mskTime(t, "2026-06-23 10:00"),
		CompletedAt:          mskTime(t, "2026-06-24 16:00"),
	}
	require.NoError(t, db.Create(&app).Error)

	// Окно бьёт по дате ПОДАЧИ (sending_datetime), заявка подана 19.06; этапы
	// свободно дотягиваются в следующие дни.
	win := []models.ReportFilterValue{{Key: "date_range", From: "2026-06-19", To: "2026-06-19"}}
	svc := services.NewStatisticsService(db, 0)
	runAvg := func(metric string) int64 {
		res, err := svc.RunReport(context.Background(), models.ReportRequest{
			Mode: "aggregate", Metric: metric, Dimension: "none", Filters: win,
		})
		require.NoError(t, err, "SQL метрики должен исполняться")
		require.Len(t, res.MetricRows, 1)
		return res.MetricRows[0].Values[metric]
	}

	// --- Пустой график: календарный фолбэк ---
	t.Run("empty_schedule_calendar", func(t *testing.T) {
		// Пт16:00 -> Пн11:00 = 67ч календарно (график не задан -> вычитать нечего).
		assert.Equal(t, int64(67*3600), runAvg("avg_approval_time"))
		// Пт16:00 -> Ср16:00 = 5 суток = 120ч (завершение всегда календарное).
		assert.Equal(t, int64(120*3600), runAvg("avg_completion_time"))
	})

	// --- График Пн-Пт 09:00-18:00: вычитаем ночь и выходные ---
	t.Run("weekday_schedule_working", func(t *testing.T) {
		setBureauSchedule(t, db, []int{0, 1, 2, 3, 4}, "09:00", "18:00")

		// Согласование Пт16:00 -> Пн11:00: Пт 16-18 (2ч) + Пн 09-11 (2ч) = 4ч.
		assert.Equal(t, int64(4*3600), runAvg("avg_approval_time"))
		// Перцентиль идёт через тот же expr (durationPercentile): при единственной
		// заявке p90 = её рабочее время — заодно проверяем percentile-путь на рабочей ветке.
		assert.Equal(t, int64(4*3600), runAvg("p90_approval_time"))
		// Принятие Пн11:00 -> Вт10:00: Пн 11-18 (7ч) + Вт 09-10 (1ч) = 8ч.
		assert.Equal(t, int64(8*3600), runAvg("avg_acceptance_time"))
		// Обработка Пт16:00 -> Вт10:00: Пт 16-18 (2ч) + Пн 09-18 (9ч) + Вт 09-10 (1ч) = 12ч.
		assert.Equal(t, int64(12*3600), runAvg("avg_processing_time"))
		// Завершение остаётся календарным и при заведённом графике: 120ч.
		assert.Equal(t, int64(120*3600), runAvg("avg_completion_time"))
	})

	// --- Частичный график, не пересекающий интервал: снова календарь ---
	// Регрессия staging: график состоял из ОДНОГО дня недели, и согласующие с
	// десятками голосов показывали «время реакции 0 секунд» — их события просто не
	// попадали в рабочее окно. Ноль неотличим от «согласовал мгновенно», поэтому
	// нулевое пересечение обязано откатываться на календарное время.
	t.Run("partial_schedule_no_overlap_calendar", func(t *testing.T) {
		setBureauSchedule(t, db, []int{3}, "08:00", "22:00") // только четверг

		// Согласование Пт16:00 -> Пн11:00 четверга не задевает: рабочих секунд 0,
		// значит показываем календарные 67ч, а не ноль.
		assert.Equal(t, int64(67*3600), runAvg("avg_approval_time"))
		assert.Equal(t, int64(67*3600), runAvg("p90_approval_time"))
		// Принятие Пн11:00 -> Вт10:00 — тоже мимо четверга: 23ч календарных.
		assert.Equal(t, int64(23*3600), runAvg("avg_acceptance_time"))
	})
}
