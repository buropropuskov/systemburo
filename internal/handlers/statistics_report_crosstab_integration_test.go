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

// TestRunReport_CrossTabAttachmentType проверяет реальный GORM-путь cross-tab (G4):
// разрез period/week + pivot=attachment_type разворачивает типы вложений в колонки.
// Значения по бинам и pivot-колонки корректны; обычная метрика остаётся колонкой.
func TestRunReport_CrossTabAttachmentType(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-Кросс", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "ct_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	status := models.StatusCompleted
	// ISO-неделя: понедельник 2026-06-08 .. воскресенье 2026-06-14.
	week1 := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)  // вторник той же недели
	week1b := time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC) // среда той же недели
	week2 := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC) // следующая неделя

	// Хелпер создания заявки с одним вложением заданного display_name.
	mk := func(number string, sent time.Time, display string) {
		n := number
		s := status
		app := models.Application{ApplicationNumber: &n, OrganizationID: org.ID,
			SenderUserID: user.ID, Status: &s, SendingDatetime: &sent}
		require.NoError(t, db.Create(&app).Error)
		dn := display
		att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars", AttachmentDisplayName: &dn}
		require.NoError(t, db.Create(&att).Error)
	}

	// Неделя 1: 2 заявки "Машины" + 1 "Люди". Неделя 2: 1 "Машины".
	mk("CT/1", week1, "Машины")
	mk("CT/2", week1b, "Машины")
	mk("CT/3", week1, "Люди")
	mk("CT/4", week2, "Машины")

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:        "aggregate",
		Metrics:     []string{"applications_count"},
		Dimension:   "period",
		Granularity: "week",
		Pivot:       "attachment_type",
		Filters: []models.ReportFilterValue{
			{Key: "date_range", From: "2026-06-08", To: "2026-06-21"},
		},
	})
	require.NoError(t, err)

	// Колонки: обычная метрика + 2 pivot-колонки (Машины, Люди).
	var hasMetric, hasMachines, hasPeople bool
	for _, c := range res.Columns {
		switch {
		case c.Key == "applications_count" && c.Kind == "":
			hasMetric = true
		case c.Key == "pivot:Машины" && c.Kind == models.ReportColumnPivot:
			hasMachines = true
			assert.Equal(t, "Тип вложения: Машины", c.Label)
		case c.Key == "pivot:Люди" && c.Kind == models.ReportColumnPivot:
			hasPeople = true
		}
	}
	assert.True(t, hasMetric, "колонка метрики applications_count")
	assert.True(t, hasMachines, "pivot-колонка Машины")
	assert.True(t, hasPeople, "pivot-колонка Люди")

	// Две строки-бина (две недели), хронологически.
	require.Len(t, res.MetricRows, 2)
	w1 := res.MetricRows[0]
	w2 := res.MetricRows[1]
	assert.Less(t, w1.Label, w2.Label, "бины хронологически")

	// Неделя 1: всего 3 заявки, 2 машины, 1 человек.
	assert.Equal(t, int64(3), w1.Values["applications_count"])
	assert.Equal(t, int64(2), w1.Values["pivot:Машины"])
	assert.Equal(t, int64(1), w1.Values["pivot:Люди"])
	// Неделя 2: 1 заявка, 1 машина, 0 людей (явный нуль).
	assert.Equal(t, int64(1), w2.Values["applications_count"])
	assert.Equal(t, int64(1), w2.Values["pivot:Машины"])
	assert.Equal(t, int64(0), w2.Values["pivot:Люди"])
}

// TestRunReport_NoPivotUnchanged — regression: при пустом pivot ответ совпадает со
// старым поведением (никаких pivot-колонок, только метрика).
func TestRunReport_NoPivotUnchanged(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-NoPivot", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "np_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	status := models.StatusCompleted
	sent := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	n := "NP/1"
	app := models.Application{ApplicationNumber: &n, OrganizationID: org.ID,
		SenderUserID: user.ID, Status: &status, SendingDatetime: &sent}
	require.NoError(t, db.Create(&app).Error)
	dn := "Машины"
	require.NoError(t, db.Create(&models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars", AttachmentDisplayName: &dn}).Error)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:        "aggregate",
		Metrics:     []string{"applications_count"},
		Dimension:   "period",
		Granularity: "week",
		// Pivot пуст — обычный отчёт.
		Filters: []models.ReportFilterValue{
			{Key: "date_range", From: "2026-06-08", To: "2026-06-14"},
		},
	})
	require.NoError(t, err)

	require.Len(t, res.Columns, 1, "только колонка метрики")
	assert.Equal(t, "applications_count", res.Columns[0].Key)
	assert.Equal(t, models.ReportColumnKind(""), res.Columns[0].Kind)
	require.Len(t, res.MetricRows, 1)
	assert.Equal(t, int64(1), res.MetricRows[0].Values["applications_count"])
	// Никаких pivot-ключей в строке.
	for k := range res.MetricRows[0].Values {
		assert.NotContains(t, k, "pivot:", "не должно быть pivot-ключей")
	}
}

// TestRunReport_AvgCarsPerDayWeekly проверяет реальный GORM-путь метрики-среднего:
// въезды машин по неделям делятся на число дней бина (крайний неполный бин — на
// фактическое пересечение с окном). Значения дробные -> FloatValues, колонка Float.
func TestRunReport_AvgCarsPerDayWeekly(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	// Машина с вложением.
	org := models.Organization{Name: "Орг-Avg", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "avg_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	status := models.StatusCompleted
	sent := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	n := "AVG/1"
	app := models.Application{ApplicationNumber: &n, OrganizationID: org.ID,
		SenderUserID: user.ID, Status: &status, SendingDatetime: &sent}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars"}
	require.NoError(t, db.Create(&att).Error)
	num := "А001АА"
	car := models.Car{AttachmentID: att.ID, CarNumber: &num}
	require.NoError(t, db.Create(&car).Error)

	// Въезды: неделя 06-08..06-14 (понедельник..воскресенье) — 14 въездов -> 2/день.
	// Окно запроса 06-08..06-14 (полная неделя).
	week1Days := []int{8, 9, 10, 11, 12, 13, 14}
	for _, d := range week1Days {
		for i := 0; i < 2; i++ { // по 2 въезда в день -> 14 за неделю
			require.NoError(t, db.Create(&models.AuditLog{
				EntityType: models.AuditEntityCar, EntityID: &car.ID, Action: "entry",
				CreatedAt: time.Date(2026, 6, d, 10+i, 0, 0, 0, time.UTC),
			}).Error)
		}
	}

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:        "aggregate",
		Metrics:     []string{"avg_cars_per_day"},
		Dimension:   "period",
		Granularity: "week",
		Filters: []models.ReportFilterValue{
			{Key: "date_range", From: "2026-06-08", To: "2026-06-14"},
		},
	})
	require.NoError(t, err)

	require.Len(t, res.Columns, 1)
	assert.Equal(t, "avg_cars_per_day", res.Columns[0].Key)
	assert.True(t, res.Columns[0].Float, "колонка среднего помечена Float")

	require.Len(t, res.MetricRows, 1, "одна неделя в окне")
	row := res.MetricRows[0]
	// 14 въездов / 7 дней = 2.0; значение дробное -> FloatValues, не Values.
	require.NotNil(t, row.FloatValues)
	assert.Equal(t, 2.0, row.FloatValues["avg_cars_per_day"])
	_, intPresent := row.Values["avg_cars_per_day"]
	assert.False(t, intPresent, "среднее не должно лежать в целочисленных Values")

	// Итог-среднее: 14 въездов / 7 дней окна = 2.0.
	assert.Equal(t, 2.0, res.FloatTotals["avg_cars_per_day"])
}

// TestRunReport_AvgCarsPartialBin проверяет крайний неполный бин: окно начинается
// в среду, ведущая неделя делится на фактическое число дней пересечения, не на 7.
func TestRunReport_AvgCarsPartialBin(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-AvgP", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "avgp_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	status := models.StatusCompleted
	sent := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
	n := "AVGP/1"
	app := models.Application{ApplicationNumber: &n, OrganizationID: org.ID,
		SenderUserID: user.ID, Status: &status, SendingDatetime: &sent}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars"}
	require.NoError(t, db.Create(&att).Error)
	num := "В002ВВ"
	car := models.Car{AttachmentID: att.ID, CarNumber: &num}
	require.NoError(t, db.Create(&car).Error)

	// Окно 06-10 (среда) .. 06-14 (воскресенье) = ведущий неполный бин недели
	// 06-08..06-14, пересечение 06-10..06-14 = 5 дней. 10 въездов -> 10/5 = 2.0.
	for _, d := range []int{10, 11, 12, 13, 14} {
		for i := 0; i < 2; i++ {
			require.NoError(t, db.Create(&models.AuditLog{
				EntityType: models.AuditEntityCar, EntityID: &car.ID, Action: "entry",
				CreatedAt: time.Date(2026, 6, d, 10+i, 0, 0, 0, time.UTC),
			}).Error)
		}
	}

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:        "aggregate",
		Metrics:     []string{"avg_cars_per_day"},
		Dimension:   "period",
		Granularity: "week",
		Filters: []models.ReportFilterValue{
			{Key: "date_range", From: "2026-06-10", To: "2026-06-14"},
		},
	})
	require.NoError(t, err)

	require.Len(t, res.MetricRows, 1)
	// 10 въездов / 5 дней (фактическое пересечение неполного бина) = 2.0.
	assert.Equal(t, 2.0, res.MetricRows[0].FloatValues["avg_cars_per_day"])
	// Итог: 10 / 5 дней окна = 2.0.
	assert.Equal(t, 2.0, res.FloatTotals["avg_cars_per_day"])
}
