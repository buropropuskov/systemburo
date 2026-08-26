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

// Бандл вкладки «Обработка заявок» (#1240, B4) на РЕАЛЬНОМ SQL: собирается из
// метрик движка (B2/B3), поэтому проверяем не только форму DTO, но и что цифры
// совпадают с посчитанными вручную по сидам — те же сиды, на которых проверялись
// сами метрики. Билд запросы не исполняет, разъехаться они могут только здесь.

// processingWindowArgs — границы периода в МСК, как их собирает parseDateRange.
func processingWindowArgs(t *testing.T, from, to string) (time.Time, time.Time) {
	t.Helper()
	loc := services.AnalyticsLocation()
	f, err := time.ParseInLocation("2006-01-02 15:04", from+" 00:00", loc)
	require.NoError(t, err)
	tt, err := time.ParseInLocation("2006-01-02 15:04", to+" 23:59", loc)
	require.NoError(t, err)
	return f, tt
}

func stageByKey(t *testing.T, stages []models.ProcessingStageKPI, key string) models.ProcessingStageKPI {
	t.Helper()
	for _, s := range stages {
		if s.Key == key {
			return s
		}
	}
	t.Fatalf("этап %q не найден в бандле", key)
	return models.ProcessingStageKPI{}
}

func qualityByKey(t *testing.T, rows []models.ProcessingQualityKPI, key string) models.ProcessingQualityKPI {
	t.Helper()
	for _, q := range rows {
		if q.Key == key {
			return q
		}
	}
	t.Fatalf("метрика качества %q не найдена в бандле", key)
	return models.ProcessingQualityKPI{}
}

// TestGetProcessingSummary_StagesAndBreakdown — KPI этапов на сиде длительностей
// (те же числа, что у метрик B2) и разбивка по организациям.
func TestGetProcessingSummary_StagesAndBreakdown(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedDurationApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, to := processingWindowArgs(t, "2026-06-01", "2026-06-03")

	got, err := svc.GetProcessingSummary(context.Background(), from, to)
	require.NoError(t, err, "SQL бандла должен исполняться")

	assert.Equal(t, "2026-06-01", got.From)
	assert.Equal(t, "2026-06-03", got.To)
	assert.Equal(t, int64(3), got.TotalApplications)

	// Этапы идут в порядке пути заявки — вкладка рисует их слева направо.
	require.Len(t, got.Stages, 4)
	assert.Equal(t, []string{"approval_time", "acceptance_time", "processing_time", "completion_time"},
		[]string{got.Stages[0].Key, got.Stages[1].Key, got.Stages[2].Key, got.Stages[3].Key})

	cases := []struct {
		key      string
		samples  int64
		avg, p90 int64
		why      string
	}{
		{"approval_time", 3, 19200, 31680, "согласование: 2ч, 4ч, 10ч"},
		{"acceptance_time", 2, 5400, 6840, "принятие: 1ч и 2ч (C не принята)"},
		{"processing_time", 2, 16200, 20520, "обработка: 3ч и 6ч"},
		{"completion_time", 1, 86400, 86400, "завершена только A: 24ч"},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			s := stageByKey(t, got.Stages, c.key)
			assert.Equal(t, c.samples, s.Samples, "заявок в выборке этапа: %s", c.why)
			require.NotNil(t, s.Avg, "этап прошли — значение есть")
			assert.Equal(t, c.avg, *s.Avg, "среднее, секунды: %s", c.why)
			require.NotNil(t, s.P90)
			assert.Equal(t, c.p90, *s.P90, "90-й перцентиль, секунды")
			assert.NotEmpty(t, s.Label)
		})
	}

	// Разбивка несёт ВСЕ этапы, а не только общее время обработки (#1251 polish, п.10).
	require.Len(t, got.ByOrganization, 1)
	org := got.ByOrganization[0]
	assert.Equal(t, "Орг-Длительности", org.Label)
	assert.Equal(t, int64(3), org.ApplicationsCount)
	require.NotNil(t, org.AvgProcessingTime)
	assert.Equal(t, int64(16200), *org.AvgProcessingTime, "обработка: 3ч и 6ч")
	require.NotNil(t, org.AvgApprovalTime)
	assert.Equal(t, int64(19200), *org.AvgApprovalTime, "согласование: 2ч, 4ч, 10ч")
	require.NotNil(t, org.AvgAcceptanceTime)
	assert.Equal(t, int64(5400), *org.AvgAcceptanceTime, "принятие: 1ч и 2ч")

	// Тот же набор колонок во втором разрезе. У заявок сида компании нет ->
	// движок группирует их в «(без компании)», строка всё равно должна прийти.
	require.Len(t, got.ByCompany, 1)
	comp := got.ByCompany[0]
	assert.Equal(t, "(без компании)", comp.Label)
	assert.Equal(t, int64(3), comp.ApplicationsCount)
	require.NotNil(t, comp.AvgProcessingTime)
	assert.Equal(t, int64(16200), *comp.AvgProcessingTime)
}

// TestGetProcessingSummary_Breakdown_SlowestFirst — заявленный порядок разбивки:
// дольше всего обрабатывающие сверху (в отличие от рейтингов людей, где сверху
// быстрые). Сид длительностей этого не ловит - там одна организация.
func TestGetProcessingSummary_Breakdown_SlowestFirst(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	user := models.User{Username: "brk_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	status := models.StatusInWork

	// Организация «Быстрая» обрабатывает 1ч, «Медленная» — 5ч.
	mkOrg := func(name, number string, sent, accepted *time.Time) {
		org := models.Organization{Name: name, IsActive: true}
		require.NoError(t, db.Create(&org).Error)
		n := number
		require.NoError(t, db.Create(&models.Application{
			ApplicationNumber: &n, OrganizationID: org.ID, SenderUserID: user.ID,
			Status: &status, SendingDatetime: sent, AcceptedAt: accepted,
		}).Error)
	}
	mkOrg("Быстрая", "B/1", mskTime(t, "2026-06-01 10:00"), mskTime(t, "2026-06-01 11:00"))
	mkOrg("Медленная", "M/1", mskTime(t, "2026-06-01 10:00"), mskTime(t, "2026-06-01 15:00"))

	svc := services.NewStatisticsService(db, 0)
	from, to := processingWindowArgs(t, "2026-06-01", "2026-06-03")
	got, err := svc.GetProcessingSummary(context.Background(), from, to)
	require.NoError(t, err)

	require.Len(t, got.ByOrganization, 2)
	assert.Equal(t, "Медленная", got.ByOrganization[0].Label, "дольше всего обрабатывающая — сверху")
	assert.Equal(t, int64(5*3600), *got.ByOrganization[0].AvgProcessingTime)
	assert.Equal(t, "Быстрая", got.ByOrganization[1].Label)
	assert.Equal(t, int64(3600), *got.ByOrganization[1].AvgProcessingTime)
}

// TestGetProcessingSummary_TrendAgainstPreviousPeriod — сравнение с предыдущим
// периодом равной длины: окно 02-03.06 против 31.05-01.06, куда попадает заявка A.
// Время согласования выросло с 2ч до (4ч+10ч)/2=7ч — рост времени это ухудшение.
func TestGetProcessingSummary_TrendAgainstPreviousPeriod(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedDurationApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, to := processingWindowArgs(t, "2026-06-02", "2026-06-03")

	got, err := svc.GetProcessingSummary(context.Background(), from, to)
	require.NoError(t, err)

	approval := stageByKey(t, got.Stages, "approval_time")
	assert.Equal(t, int64(2), approval.Samples, "в окне заявки B и C")
	require.NotNil(t, approval.Avg)
	assert.Equal(t, int64(25200), *approval.Avg, "(4ч + 10ч) / 2")

	require.NotNil(t, approval.PrevAvg, "в прошлом периоде согласовали заявку A")
	assert.Equal(t, int64(7200), *approval.PrevAvg, "2ч")

	require.NotNil(t, approval.Trend)
	assert.Equal(t, 250.0, approval.Trend.DeltaPct, "с 7200 до 25200")
	assert.Equal(t, models.ProcessingDirectionUp, approval.Trend.Direction)
	assert.Equal(t, models.ProcessingSentimentBad, approval.Trend.Sentiment,
		"согласовывать стали дольше — это ухудшение, хотя значение выросло")
}

// TestGetProcessingSummary_QualityAndSlowApprovers — доли качества и топ медленных
// согласующих на сиде качества (числа те же, что у метрик B3). Заодно замок: у
// этих заявок нет моментов согласования/принятия, значит этапы должны быть «нет
// данных», а не нулевой длительностью.
func TestGetProcessingSummary_QualityAndSlowApprovers(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, to := processingWindowArgs(t, "2026-06-01", "2026-06-04")

	got, err := svc.GetProcessingSummary(context.Background(), from, to)
	require.NoError(t, err)

	assert.Equal(t, int64(4), got.TotalApplications)

	// Вкладка показывает ветки отказа ПО ОТДЕЛЬНОСТИ (#1251 polish, п.8): по сводной
	// доле не понять, кто завернул заявку - согласующие или принимающий.
	rejected := qualityByKey(t, got.Quality, "rejected_rate")
	require.NotNil(t, rejected.Value)
	assert.Equal(t, 25.0, *rejected.Value, "отказ принимающего у 1 заявки из 4")
	assert.Equal(t, "%", rejected.Unit)

	notApproved := qualityByKey(t, got.Quality, "not_approved_rate")
	require.NotNil(t, notApproved.Value)
	assert.Equal(t, 25.0, *notApproved.Value, "несогласование у 1 заявки из 4")

	forwards := qualityByKey(t, got.Quality, "avg_forwards")
	require.NotNil(t, forwards.Value)
	assert.Equal(t, 1.5, *forwards.Value, "6 пересылок на 4 заявки")

	// Медленные сверху: Иванов 3ч, Петров 1ч. Сортировка своя (движок при
	// мультиметриках упорядочил бы по сумме секунд и штук).
	require.Len(t, got.SlowApprovers, 2)
	assert.Equal(t, "Иванов Иван", got.SlowApprovers[0].Name)
	require.NotNil(t, got.SlowApprovers[0].AvgResponseTime)
	assert.Equal(t, int64(10800), *got.SlowApprovers[0].AvgResponseTime, "(2ч + 4ч) / 2")
	assert.Equal(t, int64(2), got.SlowApprovers[0].VotesCount)

	assert.Equal(t, "Петров Пётр", got.SlowApprovers[1].Name)
	require.NotNil(t, got.SlowApprovers[1].AvgResponseTime)
	assert.Equal(t, int64(3600), *got.SlowApprovers[1].AvgResponseTime, "1ч; неотданный голос в среднее не входит")
	assert.Equal(t, int64(2), got.SlowApprovers[1].VotesCount, "нагрузка считает и неотданный голос")

	approval := stageByKey(t, got.Stages, "approval_time")
	assert.Zero(t, approval.Samples, "у этих заявок нет момента согласования")
	assert.Nil(t, approval.Avg, "этап никто не прошёл — прочерк, а не «0 секунд»")
	assert.Nil(t, approval.P90)
}

// TestGetProcessingSummary_ApproversAndAcceptorsRatings — полные рейтинги по скорости
// (#1251 S3): согласующие по времени реакции и принимающие по времени принятия,
// оба быстрые сверху. SlowApprovers остаётся топом медленных (сортировка обратная).
func TestGetProcessingSummary_ApproversAndAcceptorsRatings(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)
	seedAcceptedApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, to := processingWindowArgs(t, "2026-06-01", "2026-06-04")

	got, err := svc.GetProcessingSummary(context.Background(), from, to)
	require.NoError(t, err)

	// Approvers — полный рейтинг по скорости: быстрый (Петров 1ч) сверху, Иванов (3ч) ниже.
	require.Len(t, got.Approvers, 2)
	assert.Equal(t, "Петров Пётр", got.Approvers[0].Name, "быстрый согласующий сверху")
	require.NotNil(t, got.Approvers[0].AvgResponseTime)
	assert.Equal(t, int64(3600), *got.Approvers[0].AvgResponseTime)
	assert.Equal(t, "Иванов Иван", got.Approvers[1].Name)
	assert.Equal(t, int64(10800), *got.Approvers[1].AvgResponseTime)

	// SlowApprovers — та же пара, но медленный сверху (топ узких мест).
	require.NotEmpty(t, got.SlowApprovers)
	assert.Equal(t, "Иванов Иван", got.SlowApprovers[0].Name, "медленный согласующий сверху")

	// Acceptors — рейтинг принимающих по скорости: Кузнецов (2ч) выше Сидорова (3ч).
	require.Len(t, got.Acceptors, 2)
	assert.Equal(t, "Кузнецов Кузьма", got.Acceptors[0].Name, "быстрый принимающий сверху")
	require.NotNil(t, got.Acceptors[0].AvgAcceptanceTime)
	assert.Equal(t, int64(7200), *got.Acceptors[0].AvgAcceptanceTime)
	assert.Equal(t, int64(2), got.Acceptors[0].AcceptsCount)
	assert.Equal(t, "Сидоров Сидор", got.Acceptors[1].Name)
	assert.Equal(t, int64(10800), *got.Acceptors[1].AvgAcceptanceTime)
}

// TestGetProcessingSummary_EmptyPeriod_NoFakeZero — главный замок среза. Агрегат
// пустой выборки SQL приводит к нулю (гард скана NULL в int64), и без счётчиков
// выборки пустая вкладка отрапортовала бы «обработка за 0 минут, 0% отказов»
// вместо честного «нет данных».
func TestGetProcessingSummary_EmptyPeriod_NoFakeZero(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedDurationApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, to := processingWindowArgs(t, "2026-01-01", "2026-01-07") // до всех заявок сида

	got, err := svc.GetProcessingSummary(context.Background(), from, to)
	require.NoError(t, err)

	assert.Zero(t, got.TotalApplications)
	require.Len(t, got.Stages, 4)
	for _, s := range got.Stages {
		assert.Zero(t, s.Samples, "этап %s", s.Key)
		assert.Nil(t, s.Avg, "этап %s: нет данных, а не ноль", s.Key)
		assert.Nil(t, s.P90, "этап %s", s.Key)
		assert.Nil(t, s.Trend, "сравнивать не с чем")
	}
	for _, q := range got.Quality {
		assert.Nil(t, q.Value, "доля %s без заявок — нет данных, а не 0%%", q.Key)
	}
	assert.Empty(t, got.SlowApprovers)
	assert.Empty(t, got.ByOrganization)
	assert.Empty(t, got.ByCompany)
}

// TestGetProcessingSummary_HTTP — контракт эндпоинта: envelope, статус и имена
// полей JSON (фронт читает их as-is, разъехаться они не должны).
func TestGetProcessingSummary_HTTP(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedDurationApps(t, db)

	h := handlers.NewStatisticsHandler(services.NewStatisticsService(db, 0))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/statistics/processing-summary?from=2026-06-01&to=2026-06-03", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.GetProcessingSummary(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool                     `json:"success"`
		Data    models.ProcessingSummary `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	assert.Equal(t, int64(3), resp.Data.TotalApplications)
	require.Len(t, resp.Data.Stages, 4)

	// Имена полей в сыром JSON: фронт (api/statistics.js) читает именно их.
	var raw struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	for _, key := range []string{"from", "to", "total_applications", "stages", "quality", "slow_approvers", "by_organization", "by_company", "approvers", "acceptors"} {
		assert.Contains(t, raw.Data, key, "поле бандла %q", key)
	}

	var stages []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw.Data["stages"], &stages))
	require.NotEmpty(t, stages)
	for _, key := range []string{"key", "label", "samples", "avg", "p90", "prev_avg"} {
		assert.Contains(t, stages[0], key, "поле этапа %q", key)
	}
}
