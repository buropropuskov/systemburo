package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	entries, total, err := svc.GetProcessingJournal(context.Background(), from, to, services.ProcessingJournalFilter{}, 0, 0)
	require.NoError(t, err, "SQL ленты должен исполняться")

	require.Len(t, entries, 7, "3 согласования (голос отдан) + 4 принятия")
	assert.Equal(t, int64(7), total, "всего событий периода — столько же, страница вмещает все")

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

	entries, total, err := svc.GetProcessingJournal(context.Background(), from, to, services.ProcessingJournalFilter{}, 2, 0)
	require.NoError(t, err)
	require.Len(t, entries, 2, "лимит глубины отдаёт только 2 события")
	assert.Equal(t, int64(7), total, "total считает ВСЕ события периода, не размер страницы")

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

	entries, total, err := svc.GetProcessingJournal(context.Background(), from, to, services.ProcessingJournalFilter{}, 0, 0)
	require.NoError(t, err)
	assert.Empty(t, entries, "в 2030 событий обработки нет")
	assert.Zero(t, total, "пустое окно — нечего листать")
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
		Meta    models.PaginationMeta           `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Success)
	require.Len(t, resp.Data, 3, "limit=3 из query")
	assert.Equal(t, int64(7), resp.Meta.Total, "meta.total — все события периода")
	assert.Equal(t, 1, resp.Meta.Page)
	assert.Equal(t, 3, resp.Meta.PerPage, "per_page = применённый limit")

	// Имена полей в сыром JSON: фронт (api/statistics.js) читает именно их.
	var raw struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	require.NotEmpty(t, raw.Data)
	for _, key := range []string{"application_id", "application_number", "actor_name", "role", "occurred_at", "working_seconds"} {
		assert.Contains(t, raw.Data[0], key, "поле события %q", key)
	}
	assert.NotContains(t, raw.Data[0], "event_id", "тай-брейк сортировки наружу не течёт")
}

// TestGetProcessingJournal_RoleFilter — фильтр роли (#1251 P5c) оставляет только свою
// ветку UNION, и total считает отфильтрованные события, а не все за период (иначе
// пейджер обещал бы страницы, которых нет).
func TestGetProcessingJournal_RoleFilter(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)
	seedAcceptedApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, _ := processingWindowArgs(t, "2026-06-01", "2026-06-01")
	_, to := processingWindowArgs(t, "2026-06-04", "2026-06-04")

	acceptances, total, err := svc.GetProcessingJournal(context.Background(), from, to,
		services.ProcessingJournalFilter{Role: models.ProcessingJournalRoleAcceptance}, 0, 0)
	require.NoError(t, err)
	require.Len(t, acceptances, 4, "принятий в окне четыре")
	assert.Equal(t, int64(4), total, "total считает только принятия")
	for _, e := range acceptances {
		assert.Equal(t, models.ProcessingJournalRoleAcceptance, e.Role)
	}

	approvals, total, err := svc.GetProcessingJournal(context.Background(), from, to,
		services.ProcessingJournalFilter{Role: models.ProcessingJournalRoleApproval}, 0, 0)
	require.NoError(t, err)
	require.Len(t, approvals, 3, "согласований с отданным голосом три")
	assert.Equal(t, int64(3), total)
	for _, e := range approvals {
		assert.Equal(t, models.ProcessingJournalRoleApproval, e.Role)
	}

	_, _, err = svc.GetProcessingJournal(context.Background(), from, to,
		services.ProcessingJournalFilter{Role: "нет такой"}, 0, 0)
	assert.Error(t, err, "неизвестная роль — ошибка, а не тихая выдача всех событий")
}

// TestGetProcessingJournal_Search — поиск (#1251 P5c) бьёт и по номеру заявки, и по
// подписи актора, регистр не важен, а спецсимволы LIKE экранируются.
func TestGetProcessingJournal_Search(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)
	seedAcceptedApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, _ := processingWindowArgs(t, "2026-06-01", "2026-06-01")
	_, to := processingWindowArgs(t, "2026-06-04", "2026-06-04")

	search := func(q string) ([]models.ProcessingJournalEntry, int64) {
		entries, total, err := svc.GetProcessingJournal(context.Background(), from, to,
			services.ProcessingJournalFilter{Search: q}, 0, 0)
		require.NoError(t, err)
		return entries, total
	}

	byNumber, total := search("AC/D")
	require.Len(t, byNumber, 1, "по номеру находится ровно принятие этой заявки")
	assert.Equal(t, int64(1), total, "total сходится с числом найденного")
	assert.Equal(t, "AC/D", byNumber[0].ApplicationNumber)

	byActor, total := search("кузнецов")
	require.Len(t, byActor, 2, "Кузнецов принял две заявки; регистр запроса не важен")
	assert.Equal(t, int64(2), total)
	for _, e := range byActor {
		assert.Equal(t, "Кузнецов Кузьма", e.ActorName)
	}

	wildcard, total := search("%")
	assert.Empty(t, wildcard, "процент — обычный символ поиска, а не «показать всё»")
	assert.Zero(t, total)

	underscore, _ := search("_")
	assert.Empty(t, underscore, "подчёркивание не должно матчить любой символ")

	nothing, total := search("такого нет")
	assert.Empty(t, nothing)
	assert.Zero(t, total)
}

// TestGetProcessingJournal_FilterHTTP — фильтры доезжают из query (role, q), meta
// считает отфильтрованное, а неизвестная роль отбивается 400.
func TestGetProcessingJournal_FilterHTTP(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)
	seedAcceptedApps(t, db)

	h := handlers.NewStatisticsHandler(services.NewStatisticsService(db, 0))
	e := echo.New()

	call := func(query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/statistics/processing-journal?"+query, nil)
		rec := httptest.NewRecorder()
		require.NoError(t, h.GetProcessingJournal(e.NewContext(req, rec)))
		return rec
	}

	rec := call("from=2026-06-01&to=2026-06-04&role=acceptance&q=" + url.QueryEscape("Кузнецов"))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Data []models.ProcessingJournalEntry `json:"data"`
		Meta models.PaginationMeta           `json:"meta"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 2, "принятия Кузнецова")
	assert.Equal(t, int64(2), resp.Meta.Total, "meta.total учитывает фильтры")

	req := httptest.NewRequest(http.MethodGet, "/statistics/processing-journal?role=неизвестно", nil)
	err := h.GetProcessingJournal(e.NewContext(req, httptest.NewRecorder()))
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr, "неизвестная роль — HTTP-ошибка")
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}

// TestGetProcessingJournal_Pages — страницы не пересекаются и не теряют событий:
// две страницы по 3 подряд дают те же 6 событий, что первые 6 одной страницей.
func TestGetProcessingJournal_Pages(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedQualityApps(t, db)
	seedApproverVotes(t, db)
	seedAcceptedApps(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, _ := processingWindowArgs(t, "2026-06-01", "2026-06-01")
	_, to := processingWindowArgs(t, "2026-06-04", "2026-06-04")

	all, total, err := svc.GetProcessingJournal(context.Background(), from, to, services.ProcessingJournalFilter{}, 6, 0)
	require.NoError(t, err)
	require.Len(t, all, 6)
	assert.Equal(t, int64(7), total)

	key := func(e models.ProcessingJournalEntry) string {
		return fmt.Sprintf("%d|%s|%s|%s", e.ApplicationID, e.Role, e.ActorName, e.OccurredAt.Format(time.RFC3339Nano))
	}

	page1, _, err := svc.GetProcessingJournal(context.Background(), from, to, services.ProcessingJournalFilter{}, 3, 0)
	require.NoError(t, err)
	page2, total2, err := svc.GetProcessingJournal(context.Background(), from, to, services.ProcessingJournalFilter{}, 3, 3)
	require.NoError(t, err)
	require.Len(t, page1, 3)
	require.Len(t, page2, 3, "вторая страница из 7 событий полная")
	assert.Equal(t, int64(7), total2, "total не зависит от страницы")

	var paged []string
	for _, e := range append(append([]models.ProcessingJournalEntry{}, page1...), page2...) {
		paged = append(paged, key(e))
	}
	var expected []string
	for _, e := range all {
		expected = append(expected, key(e))
	}
	assert.Equal(t, expected, paged, "склейка страниц совпадает с одной выборкой: ни дублей, ни пропусков")

	// Последняя страница — хвост, за ней пусто.
	tail, _, err := svc.GetProcessingJournal(context.Background(), from, to, services.ProcessingJournalFilter{}, 3, 6)
	require.NoError(t, err)
	assert.Len(t, tail, 1, "7-е событие на третьей странице")

	beyond, _, err := svc.GetProcessingJournal(context.Background(), from, to, services.ProcessingJournalFilter{}, 3, 99)
	require.NoError(t, err)
	assert.Empty(t, beyond, "за хвостом событий нет")
}
