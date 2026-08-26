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

// toInt64 приводит числовое значение из []map (pgx отдаёт COUNT как int64) к int64.
func toInt64(t *testing.T, v any) int64 {
	t.Helper()
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case int:
		return int64(n)
	default:
		t.Fatalf("ожидалось числовое значение, got %T (%v)", v, v)
		return 0
	}
}

// TestRunReportList_WorkApplications проверяет реальный GORM-путь list-движка
// (B2b): сидирует сценарий «Заявка на работы» и убеждается, что плейсхолдер
// подписи кастомного поля (ILIKE ?) и аргумент базового фильтра типа вложения
// связываются в правильном порядке, а скан в []map отдаёт ожидаемые значения.
func TestRunReportList_WorkApplications(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Тест-орг B2b", IsActive: true}
	require.NoError(t, db.Create(&org).Error)

	last, first, phone := "Мякотных", "Сергей", "+79990001122"
	resp := models.User{Username: "b2b_resp", TypeID: 1, IsActive: true,
		LastName: &last, FirstName: &first, Phone: &phone}
	require.NoError(t, db.Create(&resp).Error)

	uaName, uaDisplay := "work", "Заявка на работы"
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &uaName, DisplayName: &uaDisplay, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	acf := models.AttachmentCustomField{UniqueAttachmentID: ua.ID, Label: "Наименование работ", IsActive: true}
	require.NoError(t, db.Create(&acf).Error)

	status := models.StatusInWork
	number := "TST/B2B/001"
	app := models.Application{ApplicationNumber: &number, OrganizationID: org.ID,
		SenderUserID: resp.ID, ResponsibleUserID: &resp.ID, Status: &status}
	require.NoError(t, db.Create(&app).Error)

	dateFrom, dateTo, timeFrom, timeTo := "2026-06-01", "2026-06-05", "09:00", "18:00"
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", UniqueAttachmentID: &ua.ID,
		EntryDateFrom: &dateFrom, EntryDateTo: &dateTo, EntryTimeFrom: &timeFrom, EntryTimeTo: &timeTo}
	require.NoError(t, db.Create(&att).Error)

	require.NoError(t, db.Create(&models.AttachmentCustomValue{
		AttachmentID: att.ID, CustomFieldID: acf.ID, Value: "Монтаж кровли"}).Error)

	// Двое сотрудников по этому вложению -> people_count = 2.
	for _, ln := range []string{"Кафанова", "Сидоров"} {
		name := ln
		require.NoError(t, db.Create(&models.Employee{AttachmentID: &att.ID, LastName: &name}).Error)
	}

	svc := services.NewStatisticsService(db, 0)
	res, err := svc.RunReportList(context.Background(), models.ReportRequest{Mode: "list", Entity: "work_applications"})
	require.NoError(t, err)
	require.Equal(t, "list", res.Mode)
	require.Equal(t, "work_applications", res.Entity)
	require.Len(t, res.Columns, 7)
	require.Len(t, res.Rows, 1, "ожидалась одна work-заявка")

	row := res.Rows[0]
	assert.Equal(t, number, row["number"])
	assert.Equal(t, "Тест-орг B2b", row["org_or_company"])
	assert.Equal(t, "Монтаж кровли", row["work_name"], "наименование работ через ILIKE ?-плейсхолдер")
	assert.Contains(t, row["responsible"], "Мякотных")
	assert.Contains(t, row["responsible"], phone)
	assert.Equal(t, "2026-06-01 - 2026-06-05", row["work_period"])
	assert.Equal(t, "09:00 - 18:00", row["work_time"])
	assert.Equal(t, int64(2), toInt64(t, row["people_count"]))
}

// TestRunReportList_AllEntitiesExecute убеждается, что SQL КАЖДОЙ сущности list-режима
// исполняется через GORM без рантайм-ошибки и возвращает столбцы по каталогу.
// Ловит ошибки сборки/связывания запроса, которые юнит-тест плана не видит.
func TestRunReportList_AllEntitiesExecute(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	svc := services.NewStatisticsService(db, 0)
	for _, entity := range []string{"work_applications", "applications", "cars", "people"} {
		t.Run(entity, func(t *testing.T) {
			res, err := svc.RunReportList(context.Background(), models.ReportRequest{Mode: "list", Entity: entity})
			require.NoError(t, err)
			assert.Equal(t, entity, res.Entity)
			assert.NotEmpty(t, res.Columns)
			assert.NotNil(t, res.Rows)
		})
	}
}
