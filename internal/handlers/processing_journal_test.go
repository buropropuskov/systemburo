package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/handlers"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сквозная лента событий обработки (#1251 S4) на РЕАЛЬНОМ SQL: UNION согласований
// (aru.approval_datetime) и принятий (первое take_to_work). Рабочая длительность
// считается по рабочему времени Бюро, но тест не заводит график bureau_time_slots
// -> календарный фолбэк, поэтому часы = календарной разнице.
//
// Данные берём из сидов согласующих (seedQualityApps + seedApproverVotes: голоса с
// approval_datetime) и принимающих (seedAcceptedApps: accepted_at + take_to_work).
// Согласований с проставленным голосом — 3 (Иванов A/B, Петров C; голос Петрова по D
// не отдан). Принятий — 4 (AC/A..AC/D). Итого 7 событий в окне.

func journalWindow() []models.ReportFilterValue {
	return []models.ReportFilterValue{{Key: "date_range", From: "2026-06-01", To: "2026-06-04"}}
}

// TestGetProcessingJournal_MixedEvents — лента объединяет согласования и принятия,
// сортирует по времени убыванием, считает рабочую длительность и заполняет роли.
func TestGetProcessingJournal_MixedEvents(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)
	seedAcceptedApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, _ := processingWindowArgs(t, "2026-06-01", "2026-06-01")
	_, to := processingWindowArgs(t, "2026-06-04", "2026-06-04")

	entries, err := svc.GetProcessingJournal(context.Background(), from, to, 0)
	require.NoError(t, err, "SQL ленты должен исполняться")

	require.Len(t, entries, 7, "3 согласования (голос отдан) + 4 принятия")

	// Отсортировано по времени убыванием: свежее сверху.
	for i := 1; i < len(entries); i++ {
		assert.False(t, entries[i-1].OccurredAt.Before(entries[i].OccurredAt),
			"событие %d раньше предыдущего — лента не по убыванию времени", i)
	}

	// Самое свежее — принятие AC/D Кузнецовым 04.06 13:00 (позже всех остальных).
	top := entries[0]
	assert.Equal(t, models.ProcessingJournalRoleAcceptance, top.Role)
	assert.Equal(t, "Кузнецов Кузьма", top.ActorName)
	require.NotNil(t, top.WorkingSeconds)
	assert.Equal(t, int64(3*3600), *top.WorkingSeconds, "согласование 10:00 -> принятие 13:00 = 3ч (календарный фолбэк)")

	// Роли: 3 согласования, 4 принятия.
	var approvals, acceptances int
	for _, e := range entries {
		switch e.Role {
		case models.ProcessingJournalRoleApproval:
			approvals++
		case models.ProcessingJournalRoleAcceptance:
			acceptances++
		default:
			t.Fatalf("неизвестная роль события: %q", e.Role)
		}
		assert.NotZero(t, e.ApplicationID, "у события должна быть заявка")
		assert.NotEmpty(t, e.ActorName, "у события должен быть актор")
	}
	assert.Equal(t, 3, approvals, "согласований с отданным голосом")
	assert.Equal(t, 4, acceptances, "принятий в работу")

	// Согласование Иванова по заявке B: назначен 10:00 -> голос 14:00 = 4ч.
	var ivanovB *models.ProcessingJournalEntry
	for i := range entries {
		e := entries[i]
		if e.Role == models.ProcessingJournalRoleApproval && e.ActorName == "Иванов Иван" && e.WorkingSeconds != nil && *e.WorkingSeconds == int64(4*3600) {
			ivanovB = &entries[i]
		}
	}
	require.NotNil(t, ivanovB, "должно быть согласование Иванова длительностью 4ч")
}

// TestGetProcessingJournal_DepthLimit — глубина ограничена лимитом: возвращаются
// только самые свежие N событий.
func TestGetProcessingJournal_DepthLimit(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)
	seedAcceptedApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, _ := processingWindowArgs(t, "2026-06-01", "2026-06-01")
	_, to := processingWindowArgs(t, "2026-06-04", "2026-06-04")

	entries, err := svc.GetProcessingJournal(context.Background(), from, to, 2)
	require.NoError(t, err)
	require.Len(t, entries, 2, "лимит глубины отдаёт только 2 события")

	// Первое — самое свежее (принятие AC/D 04.06 13:00).
	assert.Equal(t, models.ProcessingJournalRoleAcceptance, entries[0].Role)
	assert.Equal(t, "Кузнецов Кузьма", entries[0].ActorName)
	assert.False(t, entries[0].OccurredAt.Before(entries[1].OccurredAt), "первое свежее второго")
}

// TestGetProcessingJournal_EmptyWindow — вне периода событий нет (окно бьёт по
// времени самого события, не по дате подачи).
func TestGetProcessingJournal_EmptyWindow(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)
	seedAcceptedApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, to := processingWindowArgs(t, "2030-01-01", "2030-01-31")

	entries, err := svc.GetProcessingJournal(context.Background(), from, to, 0)
	require.NoError(t, err)
	assert.Empty(t, entries, "в 2030 событий обработки нет")
}

// TestGetProcessingJournal_HTTP — контракт эндпоинта: envelope, статус, имена полей
// JSON (фронт читает их as-is).
func TestGetProcessingJournal_HTTP(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)
	seedAcceptedApps(t, db)

	h := handlers.NewStatisticsHandler(services.NewStatisticsService(db, 0))
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/statistics/processing-journal?from=2026-06-01&to=2026-06-04&limit=3", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, h.GetProcessingJournal(c))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Success bool                            `json:"success"`
		Data    []models.ProcessingJournalEntry `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	require.Len(t, resp.Data, 3, "limit=3 из query")

	// Имена полей в сыром JSON: фронт (api/statistics.js) читает именно их.
	var raw struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.NotEmpty(t, raw.Data)
	for _, key := range []string{"application_id", "application_number", "actor_name", "role", "occurred_at", "working_seconds"} {
		assert.Contains(t, raw.Data[0], key, "поле события %q", key)
	}
}
