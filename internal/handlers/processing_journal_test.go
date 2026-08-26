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
	"gorm.io/gorm"
)

// Сквозная лента событий обработки (#1251 S4) на РЕАЛЬНОМ SQL: UNION голосов
// согласующих (aru.approval_datetime), принятий (первое take_to_work) и решений из
// audit_log — отказов принимающего и отзывов инициатором. Рабочая длительность
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

	// Роль и поиск складываются через И: Кузнецов только принимал, среди согласований
	// его нет. Если бы предикаты сложились через ИЛИ, тут пришли бы все согласования.
	rec = call("from=2026-06-01&to=2026-06-04&role=approval&q=" + url.QueryEscape("Кузнецов"))
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data, "согласований Кузнецова нет")
	assert.Zero(t, resp.Meta.Total)

	req := httptest.NewRequest(http.MethodGet, "/statistics/processing-journal?role=неизвестно", nil)
	err := h.GetProcessingJournal(e.NewContext(req, httptest.NewRecorder()))
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr, "неизвестная роль — HTTP-ошибка")
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}

// seedJournalDecisionEvents заводит события решений по заявкам за 05.06 (#1251 P7):
//
//	DEC/A — согласующий НЕ согласовал в 11:00 (назначен 10:00) -> несогласование, 1ч.
//	        Голос согласующего пишет в audit_log ту же action 'reject', что и отказ
//	        принимающего, но без new_value — эта запись в ленту попасть НЕ должна,
//	        иначе одно несогласование показалось бы дважды.
//	DEC/B — принимающий отказал в 13:00 (согласовано 10:00) -> отказ, 3ч.
//	DEC/C — инициатор отозвал заявку в 09:00 -> отзыв, длительности нет.
//	DEC/D — согласующий согласовал в 12:00 (назначен 10:00) -> согласование, 2ч.
func seedJournalDecisionEvents(t *testing.T, db *gorm.DB) {
	t.Helper()

	org := models.Organization{Name: "Орг-Решения", IsActive: true}
	require.NoError(t, db.Create(&org).Error)

	mkUser := func(username, last, first string) models.User {
		l, f := last, first
		u := models.User{Username: username, TypeID: 1, IsActive: true, LastName: &l, FirstName: &f}
		require.NoError(t, db.Create(&u).Error)
		return u
	}
	sender := mkUser("dec_sender", "Отправителев", "Олег")
	approver := mkUser("dec_approver", "Согласуев", "Семён")
	acceptor := mkUser("dec_acceptor", "Принимаев", "Пётр")

	mkApp := func(number string, confirmed *time.Time) int {
		n, status := number, models.StatusProcessing
		app := models.Application{
			ApplicationNumber:    &n,
			OrganizationID:       org.ID,
			SenderUserID:         sender.ID,
			Status:               &status,
			SendingDatetime:      mskTime(t, "2026-06-05 09:00"),
			ConfirmationDatetime: confirmed,
		}
		require.NoError(t, db.Create(&app).Error)
		return app.ID
	}
	vote := func(appID int, u models.User, status string, assigned, voted *time.Time) {
		s := status
		require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
			ApplicationID:    appID,
			UserID:           u.ID,
			CreatedAt:        *assigned,
			ApprovalStatus:   &s,
			ApprovalDatetime: voted,
		}).Error)
	}
	audit := func(appID int, action string, actor models.User, at *time.Time, details string) {
		id := appID
		row := models.AuditLog{
			EntityType:  models.AuditEntityApplication,
			EntityID:    &id,
			Action:      action,
			ActorUserID: &actor.ID,
			CreatedAt:   *at,
		}
		if details != "" {
			row.Details = json.RawMessage(details)
		}
		require.NoError(t, db.Create(&row).Error)
	}

	a := mkApp("DEC/A", mskTime(t, "2026-06-05 10:00"))
	vote(a, approver, "rejected", mskTime(t, "2026-06-05 10:00"), mskTime(t, "2026-06-05 11:00"))
	audit(a, models.AuditActionReject, approver, mskTime(t, "2026-06-05 11:00"), `{"comment":"не согласую"}`)

	b := mkApp("DEC/B", mskTime(t, "2026-06-05 10:00"))
	audit(b, models.AuditActionReject, acceptor, mskTime(t, "2026-06-05 13:00"),
		`{"old_value":"В обработке","new_value":"`+models.StatusRefused+`"}`)

	c := mkApp("DEC/C", nil)
	audit(c, models.AuditActionWithdraw, sender, mskTime(t, "2026-06-05 09:00"),
		`{"old_value":"В обработке","new_value":"`+models.StatusWithdrawn+`"}`)

	d := mkApp("DEC/D", mskTime(t, "2026-06-05 12:00"))
	vote(d, approver, "approved", mskTime(t, "2026-06-05 10:00"), mskTime(t, "2026-06-05 12:00"))
}

// TestGetProcessingJournal_DecisionRoles (#1251 P7) — лента разводит согласование и
// несогласование и показывает отказ принимающего и отзыв инициатором. До P7
// отрицательный голос ехал ролью «Согласование», а отказов и отзывов в ленте не было
// вовсе — по ней нельзя было понять, чем кончилась заявка.
func TestGetProcessingJournal_DecisionRoles(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seedJournalDecisionEvents(t, db)

	svc := services.NewStatisticsService(db, 0)
	from, _ := processingWindowArgs(t, "2026-06-05", "2026-06-05")
	_, to := processingWindowArgs(t, "2026-06-05", "2026-06-05")

	entries, total, err := svc.GetProcessingJournal(context.Background(), from, to, services.ProcessingJournalFilter{}, 0, 0)
	require.NoError(t, err, "SQL ленты должен исполняться")
	require.Len(t, entries, 4, "несогласование, отказ, отзыв и согласование; запись аудита о голосе согласующего не дублирует несогласование")
	assert.Equal(t, int64(4), total)

	byNumber := map[string]models.ProcessingJournalEntry{}
	for _, e := range entries {
		_, dup := byNumber[e.ApplicationNumber]
		require.False(t, dup, "заявка %s дала два события — ветки UNION пересеклись", e.ApplicationNumber)
		byNumber[e.ApplicationNumber] = e
	}

	notApproved := byNumber["DEC/A"]
	assert.Equal(t, models.ProcessingJournalRoleNotApproved, notApproved.Role, "отрицательный голос — не согласование")
	assert.Equal(t, "Согласуев Семён", notApproved.ActorName)
	require.NotNil(t, notApproved.WorkingSeconds)
	assert.Equal(t, int64(3600), *notApproved.WorkingSeconds, "назначен 10:00 -> голос 11:00 (календарный фолбэк)")

	rejection := byNumber["DEC/B"]
	assert.Equal(t, models.ProcessingJournalRoleRejection, rejection.Role)
	assert.Equal(t, "Принимаев Пётр", rejection.ActorName)
	require.NotNil(t, rejection.WorkingSeconds)
	assert.Equal(t, int64(3*3600), *rejection.WorkingSeconds, "согласовано 10:00 -> отказ 13:00")

	withdrawal := byNumber["DEC/C"]
	assert.Equal(t, models.ProcessingJournalRoleWithdrawal, withdrawal.Role)
	assert.Equal(t, "Отправителев Олег", withdrawal.ActorName, "отзыв показывает инициатора")
	assert.Nil(t, withdrawal.WorkingSeconds, "на отзыв инициатором рабочее время Бюро не тратится")

	assert.Equal(t, models.ProcessingJournalRoleApproval, byNumber["DEC/D"].Role, "положительный голос остаётся согласованием")

	// Каждая новая роль отбирается фильтром, и счётчик сходится с выдачей.
	for _, role := range []string{
		models.ProcessingJournalRoleApproval,
		models.ProcessingJournalRoleNotApproved,
		models.ProcessingJournalRoleRejection,
		models.ProcessingJournalRoleWithdrawal,
	} {
		filtered, filteredTotal, err := svc.GetProcessingJournal(context.Background(), from, to,
			services.ProcessingJournalFilter{Role: role}, 0, 0)
		require.NoError(t, err, "роль %q должна приниматься фильтром", role)
		require.Len(t, filtered, 1, "по роли %q ровно одно событие", role)
		assert.Equal(t, role, filtered[0].Role)
		assert.Equal(t, int64(1), filteredTotal, "total по роли %q", role)
	}
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
