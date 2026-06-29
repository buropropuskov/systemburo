package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatistics_CarEntriesUnionAuditLog проверяет финал #870 (срез F.5):
// аналитика въездов машин читает audit_log[car]-only, поэтому событие въезда из
// audit_log (как пишет recorder после 1.12c) учитывается в сводке (cars_entered),
// таймлайне (car_entries) и живой ленте проездов, а до-cutover legacy-строка
// cars_history попадает в ту же аналитику, будучи разово перенесённой в audit_log
// через BackfillAuditFromLegacy. Итог = legacy(перенесён) + audit = 2.
func TestStatistics_CarEntriesUnionAuditLog(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	// Машина с вложением: cars_history.car_id -> cars, организация - через вложение/заявку.
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
	att := models.Attachment{ApplicationID: app.ID, AttachmentType: "cars"}
	require.NoError(t, db.Create(&att).Error)
	num := "А777АА"
	car := models.Car{AttachmentID: att.ID, CarNumber: &num}
	require.NoError(t, db.Create(&car).Error)

	// 09:00 UTC = 12:00 МСК, 10:00 UTC = 13:00 МСК - обе в пределах суток 2026-06-15 МСК.
	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)

	// Legacy въезд - в замороженной cars_history (накоплен до cutover).
	legacy := models.CarHistory{CarID: car.ID, ActionType: "entry", CreatedAt: day}
	require.NoError(t, db.Create(&legacy).Error)

	// Новый въезд - напрямую в audit_log[car] (как после переноса записи).
	auditEntry := models.AuditLog{
		EntityType:  models.AuditEntityCar,
		EntityID:    &car.ID,
		Action:      "entry",
		ActorUserID: &user.ID,
		Details:     json.RawMessage(`{"comment":"Въезд через КПП"}`),
		CreatedAt:   day.Add(time.Hour),
	}
	require.NoError(t, db.Create(&auditEntry).Error)

	// F.5: читатель audit_log-only, поэтому legacy-строку поднимаем в audit_log
	// backfill'ом (флаг audit_backfilled:car снят CleanDB, снимаем повторно defensively).
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityCar).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

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

	// 1. Сводка: cars_entered учитывает обе записи (1 legacy + 1 audit) = 2.
	var summary models.StatsSummary
	callJSON(h.GetSummary, "/statistics/summary?from=2026-06-15&to=2026-06-15", &summary)
	assert.Equal(t, int64(2), summary.CarsEntered, "union: legacy + audit_log[car] въезды")
	assert.Equal(t, 2.0, summary.AvgCarsPerDay, "среднее за день - оба въезда")

	// 2. Таймлайн: точка дня содержит оба въезда.
	var points []models.StatsTimelinePoint
	callJSON(h.GetTimeline, "/statistics/timeline?metric=car_entries&granularity=day&from=2026-06-15&to=2026-06-15", &points)
	require.Len(t, points, 1, "один день в окне")
	assert.Equal(t, "2026-06-15", points[0].Date)
	assert.Equal(t, int64(2), points[0].Count, "union: оба въезда в одной точке")

	// 3. Живая лента: оба проезда видны (legacy + audit), организация из заявки.
	var passages models.RecentPassages
	callJSON(h.GetRecentPassages, "/statistics/recent-passages", &passages)
	require.Len(t, passages.Cars, 2, "union: legacy + audit_log[car] проезды в ленте")
	for _, p := range passages.Cars {
		assert.Equal(t, "entry", p.ActionType)
		assert.Equal(t, "А777АА", p.Subject, "гос. номер из cars")
		assert.Equal(t, "Орг-AuditStats", p.Organization, "организация из заявки")
	}
}

// TestRunReport_CarEntriesCountUnionAuditLog проверяет, что конструктор отчётов (#632)
// после F.5 читает audit_log[car]-only для метрики car_entries_count: движок собирает
// base как FROM (audit-source) ch, поэтому въезд из audit_log попадает в отчёт, а
// до-cutover legacy-въезд cars_history учитывается, будучи перенесённым backfill'ом.
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
	att := models.Attachment{ApplicationID: app.ID, AttachmentType: "cars"}
	require.NoError(t, db.Create(&att).Error)
	num := "В888ВВ"
	car := models.Car{AttachmentID: att.ID, CarNumber: &num}
	require.NoError(t, db.Create(&car).Error)

	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&models.CarHistory{CarID: car.ID, ActionType: "entry", CreatedAt: day}).Error)
	require.NoError(t, db.Create(&models.AuditLog{
		EntityType: models.AuditEntityCar, EntityID: &car.ID, Action: "entry",
		ActorUserID: &user.ID, Details: json.RawMessage(`{}`), CreatedAt: day.Add(time.Hour),
	}).Error)

	// F.5: отчёт читает audit_log-only, legacy-въезд поднимаем в audit_log backfill'ом.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityCar).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

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
	assert.Equal(t, int64(2), res.MetricRows[0].Values["car_entries_count"], "union: legacy + audit въезды в отчёте")
	assert.Equal(t, int64(2), res.Totals["car_entries_count"])
}
