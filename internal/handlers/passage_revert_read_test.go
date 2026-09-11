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

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedPassRevert кладёт запись отмены на отметку target (#2437). Ошибку вставки
// возвращает, а не валит тест: на ней же проверяется запрет отменить одно и то же
// дважды.
func seedPassRevert(db *gorm.DB, target models.AuditLog, actor *int, at time.Time) error {
	action := models.AuditActionEntryRevert
	if target.Action == "exit" {
		action = models.AuditActionExitRevert
	}
	return db.Create(&models.AuditLog{
		EntityType:  target.EntityType,
		EntityID:    target.EntityID,
		Action:      action,
		ActorUserID: actor,
		Details:     json.RawMessage(fmt.Sprintf(`{"reverts_id": %d, "comment": "ошибся строкой"}`, target.ID)),
		CreatedAt:   at,
	}).Error
}

// seedApplicationChain собирает цепочку организация-заявка-вложение: живая лента
// проходов и конструктор отчётов резолвят организацию через неё, и без цепочки
// проверять их бессмысленно. Возвращает вложение и автора заявки (он же играет
// охранника в отметках).
func seedApplicationChain(t *testing.T, db *gorm.DB, kind, login string) (int, int) {
	t.Helper()
	org := models.Organization{Name: "Орг-Сторно " + login, IsActive: true}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: login, TypeID: 1, IsActive: true}
	require.NoError(t, db.Create(&user).Error)
	status := models.StatusCompleted
	sent := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	n := "REV/" + login
	app := models.Application{ApplicationNumber: &n, OrganizationID: org.ID,
		SenderUserID: user.ID, Status: &status, SendingDatetime: &sent}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: kind}
	require.NoError(t, db.Create(&att).Error)
	return att.ID, user.ID
}

// seedCarWithApplication - машина на такой цепочке.
func seedCarWithApplication(t *testing.T, db *gorm.DB, number string) (models.Car, int) {
	t.Helper()
	attID, userID := seedApplicationChain(t, db, "cars", "revert_guard_"+number)
	// status=1 - машина в работе: текущий статус на КПП выбирает только такие строки.
	active := 1
	car := models.Car{AttachmentID: attID, CarNumber: &number, Status: &active}
	require.NoError(t, db.Create(&car).Error)
	return car, userID
}

// TestPassageRevert_MarkLeavesEveryCount - главная проверка среза: отменённая
// отметка перестаёт считаться ВЕЗДЕ, где считаются проходы.
//
// Замер идёт дважды, до отмены и после, и обе цифры проверяются. Тест, смотрящий
// только на «после», прошёл бы зелёным и на выключенном фильтре: единица в отчёте
// могла бы означать и «вторую отметку отбросили», и «второй отметки не было вовсе».
func TestPassageRevert_MarkLeavesEveryCount(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	car, userID := seedCarWithApplication(t, db, "А777АА")
	dn := "КПП Сторно"
	table := models.SystemTable{Name: "kpp_revert", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&table).Error)

	// 09:00 и 10:00 UTC = 12:00 и 13:00 МСК, обе внутри окна суточного отчёта 15.06.
	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	mark := func(at time.Time) models.AuditLog {
		row := models.AuditLog{
			EntityType:  models.AuditEntityCar,
			EntityID:    &car.ID,
			Action:      "entry",
			ActorUserID: &userID,
			Details:     json.RawMessage(fmt.Sprintf(`{"table_id": %d}`, table.ID)),
			CreatedAt:   at,
		}
		require.NoError(t, db.Create(&row).Error)
		return row
	}
	mark(day)
	second := mark(day.Add(time.Hour))

	stats := services.NewStatisticsService(db, 0)
	h := handlers.NewStatisticsHandler(stats)
	reports := services.NewDailyPassReportService(db)
	ctx := context.Background()

	callJSON := func(handler echo.HandlerFunc, url string, out any) {
		e := echo.New()
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, url, nil), rec)
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

	// counts снимает все пять цифр разом: расхождение между разделами на одних и
	// тех же данных как раз и есть то, что пропущенный фильтр даёт на экране.
	counts := func(stage string) (summary, timeline, feed, report, avgPerDay, daily int) {
		var s models.StatsSummary
		callJSON(h.GetSummary, "/statistics/summary?from=2026-06-15&to=2026-06-15", &s)

		var points []models.StatsTimelinePoint
		callJSON(h.GetTimeline,
			"/statistics/timeline?metric=car_entries&granularity=day&from=2026-06-15&to=2026-06-15", &points)
		tl := 0
		for _, p := range points {
			tl += int(p.Count)
		}

		var passages models.RecentPassages
		callJSON(h.GetRecentPassages, "/statistics/recent-passages", &passages)

		res, err := stats.RunReport(ctx, models.ReportRequest{
			Mode:      "aggregate",
			Metrics:   []string{"car_entries_count", "avg_cars_per_day"},
			Dimension: "none",
			Filters:   []models.ReportFilterValue{{Key: "date_range", From: "2026-06-15", To: "2026-06-15"}},
		})
		require.NoError(t, err, stage)

		require.NoError(t, reports.SaveDailyReports(ctx, day), stage)
		var saved []models.DailyPassReport
		require.NoError(t, db.Where("table_id = ?", table.ID).Find(&saved).Error)
		d := 0
		for _, r := range saved {
			d += r.CarEntries
		}

		return int(s.CarsEntered), tl, len(passages.Cars), int(res.Totals["car_entries_count"]),
			int(res.FloatTotals["avg_cars_per_day"]), d
	}

	sum, tl, feed, rep, avg, daily := counts("до отмены")
	require.Equal(t, [6]int{2, 2, 2, 2, 2, 2}, [6]int{sum, tl, feed, rep, avg, daily},
		"до отмены все разделы обязаны видеть оба въезда")

	require.NoError(t, seedPassRevert(db, second, &userID, day.Add(2*time.Hour)))

	sum, tl, feed, rep, avg, daily = counts("после отмены")
	assert.Equal(t, [6]int{1, 1, 1, 1, 1, 1}, [6]int{sum, tl, feed, rep, avg, daily},
		"отменённый въезд не считается ни в сводке, ни в таймлайне, ни в ленте, "+
			"ни в конструкторе отчётов, ни в среднем за день, ни в суточном отчёте")
}

// TestPassageRevert_MarkStaysInHistory - обратная сторона того же среза: из журнала
// отмена ничего не убирает. Охранник и администратор обязаны видеть и саму
// ошибочную отметку, и событие её отмены, иначе исправление превращается в
// бесследное стирание прохода.
func TestPassageRevert_MarkStaysInHistory(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	car, userID := seedCarWithApplication(t, db, "В888ВВ")
	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	entry := models.AuditLog{
		EntityType: models.AuditEntityCar, EntityID: &car.ID, Action: "entry",
		ActorUserID: &userID, Details: json.RawMessage(`{}`), CreatedAt: day,
	}
	require.NoError(t, db.Create(&entry).Error)
	require.NoError(t, seedPassRevert(db, entry, &userID, day.Add(time.Minute)))

	svc := services.NewCarService(db, nil)
	items, err := svc.GetCarHistory(context.Background(), car.ID)
	require.NoError(t, err)

	actions := make([]string, 0, len(items))
	for _, it := range items {
		actions = append(actions, it.ActionType)
	}
	assert.Contains(t, actions, "entry", "сама отметка обязана остаться в журнале")
	assert.Contains(t, actions, models.AuditActionEntryRevert, "событие отмены обязано быть видно рядом")
	assert.Len(t, items, 2, "самоджойн источника истории не должен ни размножать, ни терять строки: %v", actions)
}

// TestPassageRevert_SecondRevertRejected - одну отметку нельзя отменить дважды.
// Держит это не проверка в коде, а частичный уникальный индекс
// (database.createPassageRevertIndex): без него вторая запись отмены размножила бы
// строку в источнике истории, и один проход показался бы двумя.
func TestPassageRevert_SecondRevertRejected(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	car, userID := seedCarWithApplication(t, db, "С999СС")
	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	entry := models.AuditLog{
		EntityType: models.AuditEntityCar, EntityID: &car.ID, Action: "entry",
		ActorUserID: &userID, Details: json.RawMessage(`{}`), CreatedAt: day,
	}
	require.NoError(t, db.Create(&entry).Error)

	require.NoError(t, seedPassRevert(db, entry, &userID, day.Add(time.Minute)))
	err := seedPassRevert(db, entry, &userID, day.Add(2*time.Minute))
	require.Error(t, err, "вторая отмена той же отметки обязана отлететь на уникальном индексе")
}

// TestPassageRevert_LastExitIgnoresRevoked - «последний выход» в текущем статусе
// берётся из журнала подзапросом, и отменённый выезд не имеет права оказаться
// последним: иначе строка на КПП покажет время выезда, которого не было.
func TestPassageRevert_LastExitIgnoresRevoked(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	car, userID := seedCarWithApplication(t, db, "Е111ЕЕ")
	require.NoError(t, db.Model(&models.Car{}).Where("id = ?", car.ID).
		Update("territory_status", 2).Error)

	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	real := models.AuditLog{
		EntityType: models.AuditEntityCar, EntityID: &car.ID, Action: "exit",
		ActorUserID: &userID, Details: json.RawMessage(`{}`), CreatedAt: day,
	}
	require.NoError(t, db.Create(&real).Error)
	wrong := models.AuditLog{
		EntityType: models.AuditEntityCar, EntityID: &car.ID, Action: "exit",
		ActorUserID: &userID, Details: json.RawMessage(`{}`), CreatedAt: day.Add(time.Hour),
	}
	require.NoError(t, db.Create(&wrong).Error)

	svc := services.NewCarService(db, nil)
	ctx := context.Background()

	before, err := svc.GetCarsCurrentStatus(ctx, userID)
	require.NoError(t, err)
	require.Len(t, before, 1)
	require.Equal(t, services.FormatUTCPtr(&wrong.CreatedAt), before[0].LastExitTime,
		"до отмены последним выездом считается более поздняя запись")

	require.NoError(t, seedPassRevert(db, wrong, &userID, day.Add(90*time.Minute)))

	after, err := svc.GetCarsCurrentStatus(ctx, userID)
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.Equal(t, services.FormatUTCPtr(&real.CreatedAt), after[0].LastExitTime,
		"после отмены последним выездом становится предыдущий настоящий")
}

// TestPassageRevert_LastExitIgnoresRevokedForPeople - то же для человека: подзапрос
// последнего выхода у сотрудников свой, и машинный тест его не трогает.
func TestPassageRevert_LastExitIgnoresRevokedForPeople(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	attID, userID := seedApplicationChain(t, db, "people", "revert_last_exit")
	last, first := "Шумилин", "Кирилл"
	active, out := 1, 2
	emp := models.Employee{AttachmentID: &attID, LastName: &last, FirstName: &first,
		Status: &active, TerritoryStatus: &out}
	require.NoError(t, db.Create(&emp).Error)

	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	exitAt := func(at time.Time) models.AuditLog {
		row := models.AuditLog{
			EntityType: models.AuditEntityEmployee, EntityID: &emp.ID, Action: "exit",
			ActorUserID: &userID, Details: json.RawMessage(`{}`), CreatedAt: at,
		}
		require.NoError(t, db.Create(&row).Error)
		return row
	}
	real := exitAt(day)
	wrong := exitAt(day.Add(time.Hour))

	svc := services.NewEmployeesHistoryService(db)
	ctx := context.Background()

	before, err := svc.GetCurrentStatus(ctx, userID)
	require.NoError(t, err)
	require.Len(t, before, 1)
	require.Equal(t, services.FormatUTCPtr(&wrong.CreatedAt), before[0].LastExitTime,
		"до отмены последним выходом считается более поздняя запись")

	require.NoError(t, seedPassRevert(db, wrong, &userID, day.Add(90*time.Minute)))

	after, err := svc.GetCurrentStatus(ctx, userID)
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.Equal(t, services.FormatUTCPtr(&real.CreatedAt), after[0].LastExitTime,
		"после отмены последним выходом становится предыдущий настоящий")
}

// TestPassageRevert_PeopleCountsToo - у людей своя ветка SQL (employeesHistoryUnion,
// eh.*), симметричная машинной. Опечатка в ней не видна тесту про машины: там всё
// зелено, а вход человека продолжает считаться после отмены.
func TestPassageRevert_PeopleCountsToo(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	attID, userID := seedApplicationChain(t, db, "people", "revert_guard_people")
	last, first := "Роголев", "Иван"
	active := 1
	emp := models.Employee{AttachmentID: &attID, LastName: &last, FirstName: &first, Status: &active}
	require.NoError(t, db.Create(&emp).Error)

	dn := "КПП Люди"
	table := models.SystemTable{Name: "kpp_revert_people", DisplayName: &dn, TableType: "people", IsActive: true}
	require.NoError(t, db.Create(&table).Error)

	day := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	mark := func(at time.Time) models.AuditLog {
		row := models.AuditLog{
			EntityType:  models.AuditEntityEmployee,
			EntityID:    &emp.ID,
			Action:      "entry",
			ActorUserID: &userID,
			Details:     json.RawMessage(fmt.Sprintf(`{"table_id": %d}`, table.ID)),
			CreatedAt:   at,
		}
		require.NoError(t, db.Create(&row).Error)
		return row
	}
	mark(day)
	second := mark(day.Add(time.Hour))

	stats := services.NewStatisticsService(db, 0)
	h := handlers.NewStatisticsHandler(stats)
	reports := services.NewDailyPassReportService(db)
	ctx := context.Background()

	counts := func(stage string) [5]int {
		e := echo.New()
		get := func(handler echo.HandlerFunc, url string, out any) {
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, url, nil), rec)
			require.NoError(t, handler(c))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var resp struct {
				Success bool            `json:"success"`
				Data    json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
			require.NoError(t, json.Unmarshal(resp.Data, out))
		}

		var s models.StatsSummary
		get(h.GetSummary, "/statistics/summary?from=2026-06-15&to=2026-06-15", &s)

		var points []models.StatsTimelinePoint
		get(h.GetTimeline,
			"/statistics/timeline?metric=people_entries&granularity=day&from=2026-06-15&to=2026-06-15", &points)
		tl := 0
		for _, p := range points {
			tl += int(p.Count)
		}

		var passages models.RecentPassages
		get(h.GetRecentPassages, "/statistics/recent-passages", &passages)

		res, err := stats.RunReport(ctx, models.ReportRequest{
			Mode:      "aggregate",
			Metrics:   []string{"people_entries_count"},
			Dimension: "none",
			Filters:   []models.ReportFilterValue{{Key: "date_range", From: "2026-06-15", To: "2026-06-15"}},
		})
		require.NoError(t, err, stage)

		require.NoError(t, reports.SaveDailyReports(ctx, day), stage)
		var saved []models.DailyPassReport
		require.NoError(t, db.Where("table_id = ?", table.ID).Find(&saved).Error)
		d := 0
		for _, r := range saved {
			d += r.PeopleEntries
		}

		return [5]int{int(s.PeopleEntered), tl, len(passages.People), int(res.Totals["people_entries_count"]), d}
	}

	require.Equal(t, [5]int{2, 2, 2, 2, 2}, counts("до отмены"), "до отмены оба входа на месте")

	require.NoError(t, seedPassRevert(db, second, &userID, day.Add(2*time.Hour)))

	assert.Equal(t, [5]int{1, 1, 1, 1, 1}, counts("после отмены"),
		"отменённый вход человека не считается ни в сводке, ни в таймлайне, ни в ленте, "+
			"ни в конструкторе отчётов, ни в суточном отчёте")
}
