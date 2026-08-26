package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunReport_MultiMetric проверяет реальный GORM-путь мультиметричного движка
// (GR0): две метрики с РАЗНЫМИ базовыми таблицами (applications -> COUNT,
// items -> SUM через join) сливаются по разрезу organization в колонки; legacy-поля
// (первая метрика) и totals согласованы.
func TestRunReport_MultiMetric(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-Мульти", IsActive: true}
	require.NoError(t, db.Create(&org).Error)

	user := models.User{Username: "mm_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	status := models.StatusCompleted
	number := "MM/001"
	app := models.Application{ApplicationNumber: &number, OrganizationID: org.ID,
		SenderUserID: user.ID, Status: &status}
	require.NoError(t, db.Create(&app).Error)

	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "items"}
	require.NoError(t, db.Create(&att).Error)

	cnt := 4
	require.NoError(t, db.Create(&models.Item{AttachmentID: att.ID, Count: &cnt}).Error)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:      "aggregate",
		Metrics:   []string{"applications_count", "items_sum"},
		Dimension: "organization",
	})
	require.NoError(t, err)

	require.Equal(t, "aggregate", res.Mode)
	require.Len(t, res.Columns, 2, "две колонки-метрики")
	assert.Equal(t, "applications_count", res.Columns[0].Key)
	assert.Equal(t, "items_sum", res.Columns[1].Key)

	require.Len(t, res.MetricRows, 1, "одна организация -> одна строка")
	row := res.MetricRows[0]
	assert.Equal(t, "Орг-Мульти", row.Label)
	assert.Equal(t, int64(1), row.Values["applications_count"])
	assert.Equal(t, int64(4), row.Values["items_sum"])

	assert.Equal(t, int64(1), res.Totals["applications_count"])
	assert.Equal(t, int64(4), res.Totals["items_sum"])

	// legacy: первая метрика как Rows/Total/Metric (обратная совместимость с FE).
	assert.Equal(t, "applications_count", res.Metric)
	require.Len(t, res.Rows, 1)
	assert.Equal(t, "Орг-Мульти", res.Rows[0].Label)
	assert.Equal(t, int64(1), res.Rows[0].Value)
	assert.Equal(t, int64(1), res.Total)
}

// TestRunReport_DimensionNone проверяет разрез "без разреза": один итоговый ряд с
// подписью "Итого" и значением метрики по всей выборке (без GROUP BY).
func TestRunReport_DimensionNone(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-Итого", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "none_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	status := models.StatusCompleted
	for _, num := range []string{"N/1", "N/2", "N/3"} {
		n := num
		app := models.Application{ApplicationNumber: &n, OrganizationID: org.ID,
			SenderUserID: user.ID, Status: &status}
		require.NoError(t, db.Create(&app).Error)
	}

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:      "aggregate",
		Metrics:   []string{"applications_count"},
		Dimension: "none",
	})
	require.NoError(t, err)

	require.Len(t, res.MetricRows, 1, "без разреза -> один ряд")
	assert.Equal(t, "Итого", res.MetricRows[0].Label)
	assert.Equal(t, int64(3), res.MetricRows[0].Values["applications_count"])
	assert.Equal(t, int64(3), res.Totals["applications_count"])
}
