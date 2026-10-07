package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQueryInput_MoscowDayAcrossApplicationReportsStatisticsTrash(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	userID := userIDByName(t, db, "testadmin")
	day := time.Date(2026, 6, 15, 0, 0, 0, 0, services.MoscowLocation())
	end := day.AddDate(0, 0, 1)
	ua := models.UniqueAttachment{AttachmentType: "people", Name: searchStrPtr("work"), DisplayName: searchStrPtr("Заявка на работы"), IsActive: true}
	require.NoError(t, db.Create(&ua).Error)
	table := models.SystemTable{Name: "s6_bounds", TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&table).Error)
	var included []int
	for i, at := range []time.Time{day.Add(-time.Microsecond), day, day.Add(2*time.Hour + 59*time.Minute), end.Add(-500 * time.Millisecond), end} {
		id := seedSearchApplication(t, db, fmt.Sprintf("S6/BOUND/%d", i), userID, td.OrgID)
		require.NoError(t, db.Model(&models.Application{}).Where("id = ?", id).Update("sending_datetime", at).Error)
		if i > 0 && i < 4 {
			included = append(included, id)
		}
		att := models.Attachment{ApplicationID: &id, AttachmentType: "people", UniqueAttachmentID: &ua.ID, AttachmentDisplayName: ua.DisplayName}
		require.NoError(t, db.Create(&att).Error)
		zero := 0
		car := models.Car{AttachmentID: att.ID, CarNumber: searchStrPtr(fmt.Sprintf("S6-%d", i)), Status: &zero, DateRemoved: &at}
		require.NoError(t, db.Create(&car).Error)
		emp := models.Employee{AttachmentID: &att.ID, LastName: searchStrPtr("Synthetic"), Status: &zero, DateDeleted: &at}
		require.NoError(t, db.Create(&emp).Error)
		for _, entity := range []struct {
			kind string
			id   int
		}{{models.AuditEntityCar, car.ID}, {models.AuditEntityEmployee, emp.ID}} {
			require.NoError(t, db.Create(&models.AuditLog{EntityType: entity.kind, EntityID: &entity.id, ActorUserID: &userID,
				Action: "delete", Details: json.RawMessage(fmt.Sprintf(`{"table_id":%d}`, table.ID)), CreatedAt: at}).Error)
			require.NoError(t, db.Create(&models.AuditLog{EntityType: entity.kind, EntityID: &entity.id, ActorUserID: &userID,
				Action: "entry", Details: json.RawMessage(fmt.Sprintf(`{"table_id":%d}`, table.ID)), CreatedAt: at}).Error)
		}
		require.NoError(t, db.Create(&models.AuditLog{EntityType: models.AuditEntityApplication, EntityID: &id, ActorUserID: &userID,
			Action: models.AuditActionWithdraw, Details: json.RawMessage(`{}`), CreatedAt: at}).Error)
	}

	filter := services.ApplicationFilter{DateFrom: searchStrPtr("2026-06-15"), DateTo: searchStrPtr("2026-06-15")}
	apps := services.NewApplicationService(db, nil, nil, nil, nil, nil)
	rows, total, err := apps.GetApplicationsPaginated(context.Background(), "testadmin", filter, 1, 20)
	require.NoError(t, err)
	require.Equal(t, int64(3), total)
	var ids []int
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	assert.ElementsMatch(t, included, ids, "00:00, 02:59 and fractional final second included; next midnight excluded")

	stats := services.NewStatisticsService(db, 0)
	filters := []models.ReportFilterValue{{Key: "date_range", From: "2026-06-15", To: "2026-06-15"}}
	for _, pivot := range []string{"", "attachment_type"} {
		res, err := stats.RunReport(context.Background(), models.ReportRequest{Mode: "aggregate", Metric: "applications_count", Dimension: "period", Granularity: "day", Pivot: pivot, Filters: filters})
		require.NoError(t, err)
		assert.Equal(t, int64(3), res.Totals["applications_count"], "pivot=%s", pivot)
	}
	list, err := stats.RunReportList(context.Background(), models.ReportRequest{Mode: "list", Entity: "work_applications", Filters: filters})
	require.NoError(t, err)
	assert.Equal(t, 3, list.Total)
	summary, err := stats.GetSummary(context.Background(), day, end)
	require.NoError(t, err)
	assert.Equal(t, int64(3), summary.TotalApplications)
	assert.Equal(t, int64(3), summary.CarsEntered)
	assert.Equal(t, int64(3), summary.PeopleEntered)
	assert.Equal(t, float64(3), summary.AvgCarsPerDay, "one day must not be divided by two")
	avg, err := stats.RunReport(context.Background(), models.ReportRequest{Mode: "aggregate", Metric: "avg_cars_per_day", Dimension: "period", Granularity: "week", Filters: filters})
	require.NoError(t, err)
	assert.Equal(t, float64(3), avg.FloatTotals["avg_cars_per_day"], "partial week denominator is one day")
	timeline, err := stats.GetTimeline(context.Background(), day, end, "applications", "day")
	require.NoError(t, err)
	require.Len(t, timeline, 1)
	assert.EqualValues(t, 3, timeline[0].Count)
	processing, err := stats.GetProcessingSummary(context.Background(), day, end)
	require.NoError(t, err)
	assert.EqualValues(t, 3, processing.TotalApplications)
	assert.Equal(t, "2026-06-15", processing.To)
	journal, journalTotal, err := stats.GetProcessingJournal(context.Background(), day, end, services.ProcessingJournalFilter{Role: models.ProcessingJournalRoleWithdrawal}, 20, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(3), journalTotal)
	assert.Len(t, journal, 3)

	trash := services.NewTrashService(db, nil)
	trashFilter := models.TrashFilter{DateFrom: "2026-06-15", DateTo: "2026-06-15"}
	cars, err := trash.ListCarsTrash(context.Background(), table.ID, trashFilter)
	require.NoError(t, err)
	assert.Len(t, cars, 3)
	people, err := trash.ListEmployeesTrash(context.Background(), table.ID, trashFilter)
	require.NoError(t, err)
	assert.Len(t, people, 3)

	rec := testutil.GET(t, e, "/applications?date_from=2026-06-15&date_to=2026-06-15&per_page=20", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Len(t, testutil.ParseSlice(t, rec), 3)

	// The shared test app omits statistics routes; exercise the production handler
	// with a real HTTP query and the same isolated DB-backed statistics service.
	insightsRec := httptest.NewRecorder()
	insightsContext := e.NewContext(httptest.NewRequest(http.MethodGet, "/statistics/insights?from=2026-06-15&to=2026-06-15", nil), insightsRec)
	require.NoError(t, handlers.NewStatisticsHandler(stats).GetInsights(insightsContext))
	require.Equal(t, http.StatusOK, insightsRec.Code, insightsRec.Body.String())
	var insightsEnvelope struct {
		Data models.InsightsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(insightsRec.Body.Bytes(), &insightsEnvelope))
	found := false
	for _, comparison := range insightsEnvelope.Data.Comparisons {
		if comparison.Metric == "applications_count" {
			found = true
			assert.EqualValues(t, 3, comparison.Current, "insights keeps the requested final day and excludes next midnight")
		}
	}
	require.True(t, found, "applications comparison must be present")
}

func TestApplication_CenterStablePagesWithEqualSendingDatetime(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	userID := userIDByName(t, db, "testadmin")
	at := time.Date(2026, 6, 15, 12, 0, 0, 0, services.MoscowLocation())
	var want []int
	for i := 0; i < 5; i++ {
		id := seedSearchApplication(t, db, fmt.Sprintf("S6/TIE/%d", i), userID, td.OrgID)
		require.NoError(t, db.Model(&models.Application{}).Where("id = ?", id).Update("sending_datetime", at).Error)
		want = append([]int{id}, want...)
	}
	apps := services.NewApplicationService(db, nil, nil, nil, nil, nil)
	var got []int
	for page := 1; page <= 3; page++ {
		rows, total, err := apps.GetApplicationsPaginated(context.Background(), "testadmin", services.ApplicationFilter{}, page, 2)
		require.NoError(t, err)
		require.Equal(t, int64(5), total)
		for _, row := range rows {
			got = append(got, row.ID)
		}
	}
	assert.Equal(t, want, got)
	legacy, err := apps.GetApplications(context.Background(), "testadmin", services.ApplicationFilter{})
	require.NoError(t, err)
	got = nil
	for _, row := range legacy {
		got = append(got, row.ID)
	}
	assert.Equal(t, want, got, "legacy center uses the same stable order")
}
