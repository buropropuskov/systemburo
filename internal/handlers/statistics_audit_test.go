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

// TestStatistics_CarEntriesUnionAuditLog проверяет финал #870 (срез F.5):
// аналитика въездов машин читает audit_log[car]-only, поэтому события въезда из
// audit_log (как пишет recorder после 1.12c) учитываются в сводке (cars_entered),
// таймлайне (car_entries) и живой ленте проездов. Два события въезда -> итог 2.
func TestStatistics_CarEntriesUnionAuditLog(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	// Машина с вложением; организация - через вложение/заявку.
	org := models.Organization{Name: "Орг-AuditStats", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "audit_stats_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	status := models.StatusCompleted
	sent := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	n := "AUDST/1"
	app := models.Application{ApplicationNumber: &n, OrganizationID: org.ID,
		SenderUserID: user.ID, Status: &status, SendingDatetime: &sent}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars"}
	require.NoError(t, db.Create(&att).Error)
	num := "А777АА"
	car := models.Car{AttachmentID: att.ID, CarNumber: &num}
	require.NoError(t, db.Create(&car).Error)

	// 09:00 UTC = 12:00 МСК, 10:00 UTC = 13:00 МСК - обе в пределах суток 2026-06-15 МСК.
	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)

	// Два события въезда напрямую в audit_log[car] (как пишет recorder после cutover).
	require.NoError(t, db.Create(&models.AuditLog{
		EntityType: models.AuditEntityCar, EntityID: &car.ID, Action: "entry",
		ActorUserID: &user.ID, CreatedAt: day,
	}).Error)
	auditEntry := models.AuditLog{
		EntityType:  models.AuditEntityCar,
		EntityID:    &car.ID,
		Action:      "entry",
		ActorUserID: &user.ID,
		Details:     json.RawMessage(`{"comment":"Въезд через КПП"}`),
		CreatedAt:   day.Add(time.Hour),
	}
	require.NoError(t, db.Create(&auditEntry).Error)

	h := handlers.NewStatisticsHandler(services.NewStatisticsService(db, 0))

	// callJSON вызывает хендлер напрямую (минуя permission middleware) и распаковывает data.
	callJSON := func(handler echo.HandlerFunc, url string, out any) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		require.NoError(t, handler(c))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var resp struct {
			Success bool            `json:"success"`
			Data    json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.True(t, resp.Success)
		require.NoError(t, json.Unmarshal(resp.Data, out))
	}

	// 1. Сводка: cars_entered учитывает обе записи = 2.
	var summary models.StatsSummary
	callJSON(h.GetSummary, "/statistics/summary?from=2026-06-15&to=2026-06-15", &summary)
	assert.Equal(t, int64(2), summary.CarsEntered, "оба въезда из audit_log[car]")
	assert.Equal(t, 2.0, summary.AvgCarsPerDay, "среднее за день - оба въезда")

	// 2. Таймлайн: точка дня содержит оба въезда.
	var points []models.StatsTimelinePoint
	callJSON(h.GetTimeline, "/statistics/timeline?metric=car_entries&granularity=day&from=2026-06-15&to=2026-06-15", &points)
	require.Len(t, points, 1, "один день в окне")
	assert.Equal(t, "2026-06-15", points[0].Date)
	assert.Equal(t, int64(2), points[0].Count, "оба въезда в одной точке")

	// 3. Живая лента: оба проезда видны, организация из заявки.
	var passages models.RecentPassages
	callJSON(h.GetRecentPassages, "/statistics/recent-passages", &passages)
	require.Len(t, passages.Cars, 2, "оба въезда (audit_log[car]) в ленте")
	for _, p := range passages.Cars {
		assert.Equal(t, "entry", p.ActionType)
		assert.Equal(t, "А777АА", p.Subject, "гос. номер из cars")
		assert.Equal(t, "Орг-AuditStats", p.Organization, "организация из заявки")
	}
}

// TestRunReport_CarEntriesCountUnionAuditLog проверяет, что конструктор отчётов (#632)
// после F.5 читает audit_log[car]-only для метрики car_entries_count: движок собирает
// base как FROM (audit-source) ch, поэтому въезды из audit_log попадают в отчёт.
func TestRunReport_CarEntriesCountUnionAuditLog(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Орг-AuditRep", IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "audit_rep_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	status := models.StatusCompleted
	sent := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	n := "AUDREP/1"
	app := models.Application{ApplicationNumber: &n, OrganizationID: org.ID,
		SenderUserID: user.ID, Status: &status, SendingDatetime: &sent}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars"}
	require.NoError(t, db.Create(&att).Error)
	num := "В888ВВ"
	car := models.Car{AttachmentID: att.ID, CarNumber: &num}
	require.NoError(t, db.Create(&car).Error)

	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&models.AuditLog{
		EntityType: models.AuditEntityCar, EntityID: &car.ID, Action: "entry",
		ActorUserID: &user.ID, Details: json.RawMessage(`{}`), CreatedAt: day,
	}).Error)
	require.NoError(t, db.Create(&models.AuditLog{
		EntityType: models.AuditEntityCar, EntityID: &car.ID, Action: "entry",
		ActorUserID: &user.ID, Details: json.RawMessage(`{}`), CreatedAt: day.Add(time.Hour),
	}).Error)

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:      "aggregate",
		Metrics:   []string{"car_entries_count"},
		Dimension: "none",
		Filters: []models.ReportFilterValue{
			{Key: "date_range", From: "2026-06-15", To: "2026-06-15"},
		},
	})
	require.NoError(t, err)
	require.Len(t, res.MetricRows, 1)
	assert.Equal(t, int64(2), res.MetricRows[0].Values["car_entries_count"], "оба въезда из audit_log в отчёте")
	assert.Equal(t, int64(2), res.Totals["car_entries_count"])
}
