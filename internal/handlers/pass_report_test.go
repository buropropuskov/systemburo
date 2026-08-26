package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// grantTableReport выдаёт право table.<name>.report через allow-override
// (каталожная строка permissions не нужна: override резолвится по ключу).
func grantTableReport(t *testing.T, db *gorm.DB, userID int, tableName string) {
	t.Helper()
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        userID,
		PermissionKey: fmt.Sprintf("table.%s.report", tableName),
		Value:         "allow",
		GrantedAt:     time.Now(),
	}).Error)
}

// seedPassAudit вставляет событие прохода в audit_log с заданным моментом.
// actor=nil - отметка без автора; tableID=nil - легаси-запись без поста.
func seedPassAudit(t *testing.T, db *gorm.DB, entityType, action string, actor *int, tableID *int, at time.Time) {
	t.Helper()
	details := json.RawMessage(`{}`)
	if tableID != nil {
		details = json.RawMessage(fmt.Sprintf(`{"table_id": %d}`, *tableID))
	}
	require.NoError(t, db.Create(&models.AuditLog{
		EntityType:  entityType,
		Action:      action,
		ActorUserID: actor,
		Details:     details,
		CreatedAt:   at,
	}).Error)
}

// passCounts достаёт счётчики из map-а строки/итога ответа.
func passCounts(m map[string]interface{}) (carE, carX, pplE, pplX int) {
	toInt := func(v interface{}) int {
		f, _ := v.(float64)
		return int(f)
	}
	return toInt(m["car_entries"]), toInt(m["car_exits"]), toInt(m["people_entries"]), toInt(m["people_exits"])
}

// TestPassReport_LiveScopeAndGate: живой отчёт через реальный флоу отметок.
// Охранник с правом видит СВОИ строки + итог по таблице, админ - разбивку по
// всем, юзер без права - 403 с required_permission (гейт RequireTableVerb).
func TestPassReport_LiveScopeAndGate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	dn := "КПП Отчёт"
	table := models.SystemTable{Name: "kpp_pass", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&table).Error)

	guardToken := testutil.RegisterAndLogin(t, e, "prguard", "pass123", 1, td.OrgID, td.CompanyID)
	guardID := getUserID(t, db, "prguard")
	guard2Token := testutil.RegisterAndLogin(t, e, "prguard2", "pass123", 1, td.OrgID, td.CompanyID)
	guard2ID := getUserID(t, db, "prguard2")
	adminToken := testutil.RegisterAndLogin(t, e, "pradmin", "pass123", 6, td.OrgID, td.CompanyID)
	grantTableReport(t, db, guardID, "kpp_pass")
	testutil.GrantTableVerb(t, guardID, "kpp_pass", "entry")
	testutil.GrantTableVerb(t, guardID, "kpp_pass", "exit")

	// Машина через реальный флоу: заявка -> активация -> отметки въезда/выезда
	// охранником guard (ловит реальный SQL агрегата, а не только билд).
	appID, _, carID := seedCarViaCompleteApp(t, e, db, guardToken, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)
	for _, status := range []int{1, 2} {
		rec := testutil.PUT(t, e, fmt.Sprintf("/cars/%d/territory-status", carID),
			fmt.Sprintf(`{"territory_status": %d, "user_id": %d, "table_id": %d}`, status, guardID, table.ID),
			testutil.AuthHeader(guardToken))
		require.Equal(t, http.StatusOK, rec.Code)
	}
	// Проходы людей вторым охранником - прямые записи audit_log в текущем окне.
	nowUTC := time.Now().UTC()
	seedPassAudit(t, db, models.AuditEntityEmployee, "entry", &guard2ID, &table.ID, nowUTC)
	seedPassAudit(t, db, models.AuditEntityEmployee, "exit", &guard2ID, &table.ID, nowUTC)

	// Охранник: только своя строка, итог - по всей таблице.
	rec := testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/pass-report/live", table.ID), testutil.AuthHeader(guardToken))
	require.Equal(t, http.StatusOK, rec.Code)
	data := testutil.ParseMap(t, rec)
	assert.NotEmpty(t, data["period_start"])
	rows, ok := data["rows"].([]interface{})
	require.True(t, ok)
	require.Len(t, rows, 1, "охранник видит только свои строки")
	own := rows[0].(map[string]interface{})
	assert.Equal(t, float64(guardID), own["user_id"])
	carE, carX, pplE, pplX := passCounts(own)
	assert.Equal(t, []int{1, 1, 0, 0}, []int{carE, carX, pplE, pplX})
	totals := data["totals"].(map[string]interface{})
	carE, carX, pplE, pplX = passCounts(totals)
	assert.Equal(t, []int{1, 1, 1, 1}, []int{carE, carX, pplE, pplX}, "итог включает события всех охранников")

	// Супер-админ: разбивка по обоим охранникам (allowAll, без гранта).
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/pass-report/live", table.ID), testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)
	data = testutil.ParseMap(t, rec)
	rows = data["rows"].([]interface{})
	assert.Len(t, rows, 2, "супер-админ видит разбивку по всем")

	// Обычный админ (is_admin, ветка set.IsAdmin в scopeFor): тоже полная разбивка.
	managerToken := testutil.RegisterManager(t, e, "prmanager", td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/pass-report/live", table.ID), testutil.AuthHeader(managerToken))
	require.Equal(t, http.StatusOK, rec.Code)
	rows = testutil.ParseMap(t, rec)["rows"].([]interface{})
	assert.Len(t, rows, 2, "админ (не супер) видит разбивку по всем")

	// Без права - 403 с required_permission (FE-гейт и BE-гейт - один ключ).
	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/pass-report/live", table.ID), testutil.AuthHeader(guard2Token))
	require.Equal(t, http.StatusForbidden, rec.Code)
	var denied map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &denied))
	assert.Equal(t, "table.kpp_pass.report", denied["required_permission"])

	// Несуществующая таблица - 404 ещё в гейте.
	rec = testutil.GET(t, e, "/system-tables/999999/pass-report/live", testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestPassReport_WindowBoundariesIdempotencyBackfill: границы окна [D-1 21:30,
// D 21:30) МСК (21:30:00 - уже новое окно), идемпотентность upsert-а, полный
// backfill при пустой таблице и catch-up пропущенных дней. Момент «сейчас»
// инъектирован - тест детерминирован в любое время суток.
func TestPassReport_WindowBoundariesIdempotencyBackfill(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	testutil.SeedTestData(t, db)
	ctx := context.Background()

	tableID := seedSystemTable(t, db)
	loc := services.AnalyticsLocation()

	// Окно report_date=20.07: [19.07 21:30, 20.07 21:30).
	seedPassAudit(t, db, models.AuditEntityEmployee, "entry", nil, &tableID, time.Date(2026, 7, 19, 21, 30, 0, 0, loc)) // нижняя граница включена
	seedPassAudit(t, db, models.AuditEntityEmployee, "exit", nil, &tableID, time.Date(2026, 7, 20, 21, 29, 59, 0, loc)) // внутри окна
	seedPassAudit(t, db, models.AuditEntityEmployee, "entry", nil, &tableID, time.Date(2026, 7, 20, 21, 30, 0, 0, loc)) // верхняя граница -> окно 21.07
	seedPassAudit(t, db, models.AuditEntityCar, "entry", nil, &tableID, time.Date(2026, 7, 18, 15, 0, 0, 0, loc))       // окно 18.07
	seedPassAudit(t, db, models.AuditEntityEmployee, "entry", nil, nil, time.Date(2026, 7, 20, 10, 0, 0, 0, loc))       // без table_id - исключается

	fixedNow := time.Date(2026, 7, 21, 12, 0, 0, 0, loc) // окно 21.07 ещё не закрыто
	svc := services.NewDailyPassReportServiceAt(db, func() time.Time { return fixedNow })

	assertDay := func(day time.Time, wantRows int, check func(models.DailyPassReport)) {
		t.Helper()
		var recs []models.DailyPassReport
		require.NoError(t, db.Where("table_id = ? AND report_date = ?", tableID, day.Format("2006-01-02")).Find(&recs).Error)
		require.Len(t, recs, wantRows)
		if wantRows > 0 && check != nil {
			check(recs[0])
		}
	}

	require.NoError(t, svc.SaveDailyReports(ctx, time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)))
	assertDay(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), 1, func(r models.DailyPassReport) {
		assert.Equal(t, 0, r.UserID, "отметки без автора схлопываются в user_id=0")
		assert.Equal(t, 1, r.PeopleEntries, "21:30:00 не входит в закрывшееся окно")
		assert.Equal(t, 1, r.PeopleExits)
		assert.Equal(t, 0, r.CarEntries)
	})

	// Идемпотентность: повторный прогон не плодит строк и не меняет значений.
	require.NoError(t, svc.SaveDailyReports(ctx, time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)))
	assertDay(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), 1, func(r models.DailyPassReport) {
		assert.Equal(t, 1, r.PeopleEntries)
	})

	// Полный backfill с нуля: пустая таблица -> все закрытые окна из audit_log.
	require.NoError(t, db.Exec("DELETE FROM daily_pass_reports").Error)
	require.NoError(t, svc.CatchUp(ctx))
	assertDay(time.Date(2026, 7, 18, 0, 0, 0, 0, time.UTC), 1, func(r models.DailyPassReport) {
		assert.Equal(t, 1, r.CarEntries)
	})
	assertDay(time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC), 1, nil)
	assertDay(time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC), 0, nil) // окно ещё открыто - не фиксируется
	var total int64
	require.NoError(t, db.Model(&models.DailyPassReport{}).Count(&total).Error)
	assert.EqualValues(t, 2, total, "пустые дни (19.07) строк не создают")

	// Catch-up после «даунтайма»: назавтра окно 21.07 закрылось - дозаписывается.
	nextNow := time.Date(2026, 7, 22, 12, 0, 0, 0, loc)
	svcNext := services.NewDailyPassReportServiceAt(db, func() time.Time { return nextNow })
	require.NoError(t, svcNext.CatchUp(ctx))
	assertDay(time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC), 1, func(r models.DailyPassReport) {
		assert.Equal(t, 1, r.PeopleEntries, "событие 20.07 21:30:00 легло в окно 21.07")
	})
}

// TestPassReport_ListDaysScope: история дней через HTTP - охранник получает свои
// строки + общий итог дня, админ - разбивку с именами; кривая дата фильтра - 400.
func TestPassReport_ListDaysScope(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	dn := "КПП История"
	table := models.SystemTable{Name: "kpp_list", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&table).Error)

	guardToken := testutil.RegisterAndLogin(t, e, "prlista", "pass123", 1, td.OrgID, td.CompanyID)
	guardID := getUserID(t, db, "prlista")
	testutil.RegisterAndLogin(t, e, "prlistb", "pass123", 1, td.OrgID, td.CompanyID)
	otherID := getUserID(t, db, "prlistb")
	adminToken := testutil.RegisterAndLogin(t, e, "prlistadm", "pass123", 6, td.OrgID, td.CompanyID)
	grantTableReport(t, db, guardID, "kpp_list")

	loc := services.AnalyticsLocation()
	at := time.Date(2026, 7, 10, 12, 0, 0, 0, loc) // окно report_date=10.07
	seedPassAudit(t, db, models.AuditEntityCar, "entry", &guardID, &table.ID, at)
	seedPassAudit(t, db, models.AuditEntityCar, "exit", &otherID, &table.ID, at.Add(time.Hour))

	svc := services.NewDailyPassReportService(db)
	require.NoError(t, svc.SaveDailyReports(ctx, time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)))

	url := fmt.Sprintf("/system-tables/%d/pass-reports?from=2026-07-10&to=2026-07-10", table.ID)

	rec := testutil.GET(t, e, url, testutil.AuthHeader(guardToken))
	require.Equal(t, http.StatusOK, rec.Code)
	days := testutil.ParseMap(t, rec)["days"].([]interface{})
	require.Len(t, days, 1)
	day := days[0].(map[string]interface{})
	assert.Equal(t, "2026-07-10", day["report_date"])
	rows := day["rows"].([]interface{})
	require.Len(t, rows, 1, "охранник видит только свою строку дня")
	assert.Equal(t, float64(guardID), rows[0].(map[string]interface{})["user_id"])
	carE, carX, _, _ := passCounts(day["totals"].(map[string]interface{}))
	assert.Equal(t, []int{1, 1}, []int{carE, carX}, "итог дня - по всем охранникам")

	rec = testutil.GET(t, e, url, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)
	days = testutil.ParseMap(t, rec)["days"].([]interface{})
	require.Len(t, days, 1)
	rows = days[0].(map[string]interface{})["rows"].([]interface{})
	require.Len(t, rows, 2, "админ видит разбивку по всем")
	for _, r := range rows {
		assert.NotEmpty(t, r.(map[string]interface{})["user_name"], "имя резолвится (ФИО или username)")
	}

	rec = testutil.GET(t, e, fmt.Sprintf("/system-tables/%d/pass-reports?from=oops", table.ID), testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestPassReport_LiveGracePeriod: первые 15 минут после 21:30 живой отчёт
// показывает ТОЛЬКО ЧТО ЗАКРЫТУЮ смену (кейс «открыл в 21:31»), после - новую
// пустую. Момент инъектирован, тест детерминирован в любое время суток.
func TestPassReport_LiveGracePeriod(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	testutil.SeedTestData(t, db)
	ctx := context.Background()

	tableID := seedSystemTable(t, db)
	loc := services.AnalyticsLocation()

	// Проход в смене [23.07 21:30, 24.07 21:30) - днём 24.07.
	seedPassAudit(t, db, models.AuditEntityCar, "entry", nil, &tableID, time.Date(2026, 7, 24, 15, 0, 0, 0, loc))

	scope := services.PassReportScope{AllUsers: true}

	// Внутри grace (21:35, +5 мин): видим закрытую смену, окно закрыто границей.
	svcGrace := services.NewDailyPassReportServiceAt(db, func() time.Time { return time.Date(2026, 7, 24, 21, 35, 0, 0, loc) })
	grace, err := svcGrace.Live(ctx, tableID, scope)
	require.NoError(t, err)
	assert.Equal(t, 1, grace.Totals.CarEntries, "в grace показываем отработанную смену")
	assert.Equal(t, time.Date(2026, 7, 24, 21, 30, 0, 0, loc).UTC(), grace.PeriodEnd.UTC(), "окно закрыто границей 21:30, не now")

	// После grace (21:50, +20 мин): новая смена [24.07 21:30, now), проход туда не входит.
	svcAfter := services.NewDailyPassReportServiceAt(db, func() time.Time { return time.Date(2026, 7, 24, 21, 50, 0, 0, loc) })
	after, err := svcAfter.Live(ctx, tableID, scope)
	require.NoError(t, err)
	assert.Equal(t, 0, after.Totals.CarEntries, "после grace новая смена пуста")
	assert.Equal(t, time.Date(2026, 7, 24, 21, 30, 0, 0, loc).UTC(), after.PeriodStart.UTC(), "новая смена начинается с границы 21:30")
}
