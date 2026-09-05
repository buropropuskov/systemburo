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

// TestReportDataPeriod проверяет реальный GORM-путь границ дат (#2341): движок
// считает их по оси времени отчёта - той же, по которой сужает фильтр периода.
// Ими мастер разворачивает «Весь период» в конкретный диапазон, поэтому запрос
// обязан исполняться и отдавать даты в московской зоне.
func TestReportDataPeriod(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Тест-орг границы", IsActive: true}
	require.NoError(t, db.Create(&org).Error)

	sender := models.User{Username: "period_sender", TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&sender).Error)

	uaName, uaDisplay := "work", "Заявка на работы"
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &uaName, DisplayName: &uaDisplay, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	status := models.StatusInWork
	// Две заявки с разными датами подачи: границы должны совпасть с ними.
	for i, sent := range []time.Time{
		time.Date(2026, 4, 9, 8, 30, 0, 0, time.UTC),
		time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC),
	} {
		number := []string{"TST/PER/001", "TST/PER/002"}[i]
		sentAt := sent
		app := models.Application{ApplicationNumber: &number, OrganizationID: org.ID,
			SenderUserID: sender.ID, Status: &status, SendingDatetime: &sentAt}
		require.NoError(t, db.Create(&app).Error)

		att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", UniqueAttachmentID: &ua.ID}
		require.NoError(t, db.Create(&att).Error)
	}

	svc := services.NewStatisticsService(db, 0)

	agg, err := svc.ReportDataPeriod(context.Background(), models.ReportRequest{
		Mode: "aggregate", Metric: "applications_count"})
	require.NoError(t, err)
	assert.Equal(t, "2026-04-09", agg.From)
	assert.Equal(t, "2026-09-05", agg.To)

	list, err := svc.ReportDataPeriod(context.Background(), models.ReportRequest{
		Mode: "list", Entity: "work_applications"})
	require.NoError(t, err)
	assert.Equal(t, "2026-04-09", list.From, "у заявок на работы ось - дата подачи")
	assert.Equal(t, "2026-09-05", list.To)
}

// TestReportDataPeriod_NoAxisAndNoData: сущность без оси времени и пустая база дают
// пустые границы, а не ошибку - мастеру нечего подставить, и это нормальный исход.
func TestReportDataPeriod_NoAxisAndNoData(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	svc := services.NewStatisticsService(db, 0)

	cars, err := svc.ReportDataPeriod(context.Background(), models.ReportRequest{Mode: "list", Entity: "cars"})
	require.NoError(t, err)
	assert.Empty(t, cars.From)
	assert.Empty(t, cars.To)

	empty, err := svc.ReportDataPeriod(context.Background(), models.ReportRequest{
		Mode: "aggregate", Metric: "applications_count"})
	require.NoError(t, err)
	assert.Empty(t, empty.From)
	assert.Empty(t, empty.To)
}

// TestReportDataPeriod_InvalidRequest: неизвестные метрика и сущность отбиваются той
// же ошибкой, что и сам отчёт (400 в handler), а не считаются по чужой оси.
func TestReportDataPeriod_InvalidRequest(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	svc := services.NewStatisticsService(db, 0)

	_, err := svc.ReportDataPeriod(context.Background(), models.ReportRequest{
		Mode: "aggregate", Metric: "unknown_metric"})
	require.ErrorIs(t, err, services.ErrInvalidReportRequest)

	_, err = svc.ReportDataPeriod(context.Background(), models.ReportRequest{Mode: "list", Entity: "unknown"})
	require.ErrorIs(t, err, services.ErrInvalidReportRequest)
}
