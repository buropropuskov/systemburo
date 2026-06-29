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
	"gorm.io/gorm"
)

// seedEmployeeForStatsAudit создаёт сотрудника с вложением/заявкой/организацией и
// возвращает его + пользователя-актора. Нужен для проверки, что аналитика въездов
// людей разрешает ФИО/организацию из заявки через audit_log[employee] (до-cutover
// строки employees_history подняты backfill'ом).
func seedEmployeeForStatsAudit(t *testing.T, db *gorm.DB, orgName, username, num string) (models.Employee, models.User) {
	t.Helper()
	org := models.Organization{Name: orgName, IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: username, TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	status := models.StatusCompleted
	sent := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	app := models.Application{ApplicationNumber: &num, OrganizationID: org.ID,
		SenderUserID: user.ID, Status: &status, SendingDatetime: &sent}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: app.ID, AttachmentType: "people"}
	require.NoError(t, db.Create(&att).Error)
	ln, fn, mn, empStatus := "Иванов", "Иван", "Иванович", 1
	emp := models.Employee{AttachmentID: &att.ID, LastName: &ln, FirstName: &fn, MiddleName: &mn, Status: &empStatus}
	require.NoError(t, db.Create(&emp).Error)
	return emp, user
}

// TestStatistics_PeopleEntriesUnionAuditLog проверяет финал #870 (срез F.6):
// аналитика въездов людей читает audit_log[employee]-only, поэтому событие въезда из
// audit_log (как пишет recorder после 1.13b) учитывается в сводке (people_entered),
// таймлайне (people_entries) и живой ленте, а до-cutover legacy-строка
// employees_history попадает в ту же аналитику, будучи разово перенесённой в audit_log
// через BackfillAuditFromLegacy. Итог = legacy(перенесён) + audit = 2.
func TestStatistics_PeopleEntriesUnionAuditLog(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	emp, user := seedEmployeeForStatsAudit(t, db, "Орг-PplStats", "audit_ppl_sender", "AUDPPL/1")

	// 09:00 UTC = 12:00 МСК, 10:00 UTC = 13:00 МСК - обе в пределах суток 2026-06-15 МСК.
	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)

	// Legacy въезд - в замороженной employees_history (накоплен до cutover).
	require.NoError(t, db.Create(&models.EmployeeHistory{EmployeeID: emp.ID, ActionType: "entry", CreatedAt: day}).Error)
	// Новый въезд - напрямую в audit_log[employee] (как после переноса записи).
	require.NoError(t, db.Create(&models.AuditLog{
		EntityType:  models.AuditEntityEmployee,
		EntityID:    &emp.ID,
		Action:      "entry",
		ActorUserID: &user.ID,
		Details:     json.RawMessage(`{"comment":"Въезд через КПП"}`),
		CreatedAt:   day.Add(time.Hour),
	}).Error)

	// F.6: читатель audit_log-only, поэтому legacy-строку поднимаем в audit_log
	// backfill'ом (CleanDB->Seed заново ставит гард-флаг, снимаем перед переносом).
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityEmployee).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	h := handlers.NewStatisticsHandler(services.NewStatisticsService(db, 0))

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

	// 1. Сводка: people_entered учитывает обе записи (1 legacy + 1 audit) = 2.
	var summary models.StatsSummary
	callJSON(h.GetSummary, "/statistics/summary?from=2026-06-15&to=2026-06-15", &summary)
	assert.Equal(t, int64(2), summary.PeopleEntered, "backfill+audit: legacy(перенесён) + audit_log[employee] въезды")

	// 2. Таймлайн: точка дня содержит оба въезда.
	var points []models.StatsTimelinePoint
	callJSON(h.GetTimeline, "/statistics/timeline?metric=people_entries&granularity=day&from=2026-06-15&to=2026-06-15", &points)
	require.Len(t, points, 1, "один день в окне")
	assert.Equal(t, "2026-06-15", points[0].Date)
	assert.Equal(t, int64(2), points[0].Count, "backfill+audit: оба въезда в одной точке")

	// 3. Живая лента: оба прохода видны (legacy + audit), ФИО и организация из заявки.
	var passages models.RecentPassages
	callJSON(h.GetRecentPassages, "/statistics/recent-passages", &passages)
	require.Len(t, passages.People, 2, "backfill+audit: legacy(перенесён) + audit_log[employee] проходы в ленте")
	for _, p := range passages.People {
		assert.Equal(t, "entry", p.ActionType)
		assert.Equal(t, "Иванов Иван Иванович", p.Subject, "ФИО из employees")
		assert.Equal(t, "Орг-PplStats", p.Organization, "организация из заявки")
	}
}

// TestRunReport_PeopleEntriesCountUnionAuditLog проверяет, что конструктор отчётов (#632)
// после F.6 читает audit_log[employee]-only для метрики people_entries_count: движок
// собирает base как FROM (audit-source) eh, поэтому въезд из audit_log попадает в отчёт,
// а до-cutover legacy-въезд employees_history учитывается, будучи перенесённым backfill'ом.
func TestRunReport_PeopleEntriesCountUnionAuditLog(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	emp, user := seedEmployeeForStatsAudit(t, db, "Орг-PplRep", "audit_pplrep_sender", "AUDPPLREP/1")

	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	require.NoError(t, db.Create(&models.EmployeeHistory{EmployeeID: emp.ID, ActionType: "entry", CreatedAt: day}).Error)
	require.NoError(t, db.Create(&models.AuditLog{
		EntityType: models.AuditEntityEmployee, EntityID: &emp.ID, Action: "entry",
		ActorUserID: &user.ID, Details: json.RawMessage(`{}`), CreatedAt: day.Add(time.Hour),
	}).Error)

	// F.6: отчёт читает audit_log-only, legacy-въезд поднимаем backfill'ом (снять гард-флаг).
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityEmployee).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReport(context.Background(), models.ReportRequest{
		Mode:      "aggregate",
		Metrics:   []string{"people_entries_count"},
		Dimension: "none",
		Filters: []models.ReportFilterValue{
			{Key: "date_range", From: "2026-06-15", To: "2026-06-15"},
		},
	})
	require.NoError(t, err)
	require.Len(t, res.MetricRows, 1)
	assert.Equal(t, int64(2), res.MetricRows[0].Values["people_entries_count"], "backfill+audit: legacy(перенесён) + audit въезды в отчёте")
	assert.Equal(t, int64(2), res.Totals["people_entries_count"])
}
