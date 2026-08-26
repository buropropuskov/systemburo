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

// TestGetInsights_Structure проверяет реальный путь движка: все 4 блока инсайтов
// собираются вызовами RunReport (разрезы hour_of_day/none/unload_place/organization/
// period исполняются на postgres), структура согласована, заявка попадает в
// сравнение и топ организаций.
func TestGetInsights_Structure(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-Инсайт", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "ins_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)

	status := models.StatusCompleted
	number := "INS/001"
	// applications_count фильтрует период по sending_datetime — ставим дату в окне.
	sent := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)
	app := models.Application{
		ApplicationNumber: &number, OrganizationID: org.ID,
		SenderUserID: user.ID, Status: &status, SendingDatetime: &sent,
	}
	require.NoError(t, db.Create(&app).Error)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.GetInsights(context.Background(), "2026-06-01", "2026-06-30")
	require.NoError(t, err)

	// Сравнения и тренды — по 3 ключевым метрикам.
	require.Len(t, res.Comparisons, 3)
	require.Len(t, res.Trends, 3)

	// applications_count есть среди сравнений, направление заполнено.
	var apps *models.ComparisonInsight
	for i := range res.Comparisons {
		if res.Comparisons[i].Metric == "applications_count" {
			apps = &res.Comparisons[i]
		}
		assert.Contains(t, []string{"up", "down", "flat"}, res.Comparisons[i].Direction)
	}
	require.NotNil(t, apps)
	assert.Equal(t, int64(1), apps.Current, "одна заявка в текущем периоде")

	// Тренды несут направление.
	for _, tr := range res.Trends {
		assert.Contains(t, []string{"up", "down", "flat"}, tr.Direction)
	}

	// Срезы исполнились (не nil-слайсы), заявка дала топ организацию.
	assert.NotNil(t, res.PeakHours)
	assert.NotNil(t, res.TopPlaces)
	require.NotEmpty(t, res.TopOrgs)
	assert.Equal(t, "Орг-Инсайт", res.TopOrgs[0].Label)
	assert.Equal(t, int64(1), res.TopOrgs[0].Value)
}
