package handlers_test

import (
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

// passagePage - ответ журнала проходов со страницей и счётчиком.
type passagePage struct {
	Success bool                     `json:"success"`
	Data    []map[string]interface{} `json:"data"`
	Meta    models.PaginationMeta    `json:"meta"`
}

func parsePassagePage(t *testing.T, rec interface{ Bytes() []byte }) passagePage {
	t.Helper()
	var page passagePage
	require.NoError(t, json.Unmarshal(rec.Bytes(), &page))
	require.True(t, page.Success)
	return page
}

// writeCarPassageAt пишет отметку проезда с заданным временем и автором. Отдельно от
// writeCarPassageAudit: проверкам периода и фильтра по пользователю нужно управлять
// и моментом, и тем, кто отметил.
func writeCarPassageAt(t *testing.T, db *gorm.DB, carID int, action string, at time.Time, actorID *int, tableID *int) int {
	t.Helper()
	details := map[string]any{"comment": "тестовый проезд"}
	if tableID != nil {
		details["table_id"] = *tableID
	}
	raw, err := json.Marshal(details)
	require.NoError(t, err)
	entry := models.AuditLog{
		EntityType:  models.AuditEntityCar,
		EntityID:    &carID,
		Action:      action,
		ActorUserID: actorID,
		Details:     raw,
		CreatedAt:   at,
	}
	require.NoError(t, db.Create(&entry).Error)
	return entry.ID
}

// Журнал проходов отдаётся страницами (#2469): до этого оба метода выгружали всю
// историю, и фронт искал по загруженному массиву. Страница не больше запрошенной,
// meta.total считает все подходящие строки, а перебор per_page срезается до предела -
// иначе один запрос снова вытащил бы журнал целиком.
func TestCarsHistory_PagesAndTotal(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	table := seedCarsTable(t, db, "page_tbl", "Таблица страниц")
	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": "P001AA777", "car_brand": "Test"}]}`, td.OrgID, table)
	rec := testutil.POST(t, e, "/cars/manual", body, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	carID := int(testutil.ParseMap(t, rec)["car_ids"].([]interface{})[0].(float64))

	base := time.Now().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		writeCarPassageAt(t, db, carID, "entry", base.Add(time.Duration(i)*time.Minute), nil, &table)
	}

	first := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all?per_page=2", h).Body)
	assert.Len(t, first.Data, 2, "страница не больше запрошенной")
	assert.EqualValues(t, 5, first.Meta.Total, "счётчик считает все подходящие строки, а не страницу")
	assert.Equal(t, 1, first.Meta.Page)
	assert.Equal(t, 2, first.Meta.PerPage)

	last := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all?per_page=2&page=3", h).Body)
	assert.Len(t, last.Data, 1, "последняя страница отдаёт остаток")

	overflow := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all?per_page=100000", h).Body)
	assert.Equal(t, models.PassageHistoryMaxPerPage, overflow.Meta.PerPage,
		"перебор per_page срезается до предела, иначе журнал снова выгружается целиком")
}

// Период фильтра - московские сутки, а не сутки по UTC. Соединение с базой открыто в
// UTC (#184), поэтому отметка в 00:30 МСК лежит в базе вчерашним днём, и наивная
// граница отдавала её «вчера»: ночная смена уезжала не в тот день.
func TestCarsHistory_MoscowDayBounds(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	table := seedCarsTable(t, db, "tz_tbl", "Таблица зоны")
	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": "T001AA777", "car_brand": "Test"}]}`, td.OrgID, table)
	rec := testutil.POST(t, e, "/cars/manual", body, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	carID := int(testutil.ParseMap(t, rec)["car_ids"].([]interface{})[0].(float64))

	moscow := services.MoscowLocation()
	nightShift := time.Date(2026, 3, 12, 0, 30, 0, 0, moscow)
	writeCarPassageAt(t, db, carID, "entry", nightShift, nil, &table)

	sameDay := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all?date_from=2026-03-12&date_to=2026-03-12", h).Body)
	assert.EqualValues(t, 1, sameDay.Meta.Total, "отметка 00:30 МСК принадлежит 12 марта по Москве")

	dayBefore := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all?date_from=2026-03-11&date_to=2026-03-11", h).Body)
	assert.EqualValues(t, 0, dayBefore.Meta.Total, "по UTC та же отметка попадала в 11 марта - этого быть не должно")
}

// Фильтры журнала работают на сервере: поиск по данным отметки, конкретный
// отметивший, конкретная машина. Это и есть смысл среза - до него фронт фильтровал
// то, что успел загрузить.
func TestCarsHistory_ServerSideFilters(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	adminID := userIDByName(t, db, "testadmin")

	table := seedCarsTable(t, db, "flt_tbl", "Таблица фильтров")
	createCar := func(number string) int {
		body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": %q, "car_brand": "Volvo"}]}`, td.OrgID, table, number)
		rec := testutil.POST(t, e, "/cars/manual", body, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return int(testutil.ParseMap(t, rec)["car_ids"].([]interface{})[0].(float64))
	}

	volvo := createCar("F001AA777")
	scania := createCar("F002BB777")
	require.NoError(t, db.Exec("UPDATE cars SET car_brand = 'Scania' WHERE id = ?", scania).Error)

	now := time.Now()
	writeCarPassageAt(t, db, volvo, "entry", now.Add(-3*time.Minute), &adminID, &table)
	writeCarPassageAt(t, db, scania, "entry", now.Add(-2*time.Minute), nil, &table)
	writeCarPassageAt(t, db, scania, "exit", now.Add(-time.Minute), nil, &table)

	byNumber := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all?search=F001AA", h).Body)
	assert.EqualValues(t, 1, byNumber.Meta.Total, "поиск по номеру машины")

	byBrand := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all?search=scania", h).Body)
	assert.EqualValues(t, 2, byBrand.Meta.Total, "поиск по марке, регистр не важен")

	byUser := parsePassagePage(t, testutil.GET(t, e, fmt.Sprintf("/cars/history/all?user_id=%d", adminID), h).Body)
	assert.EqualValues(t, 1, byUser.Meta.Total, "фильтр по отметившему")

	byCar := parsePassagePage(t, testutil.GET(t, e, fmt.Sprintf("/cars/history/all?car_id=%d", scania), h).Body)
	assert.EqualValues(t, 2, byCar.Meta.Total, "фильтр по машине")

	wildcard := parsePassagePage(t, testutil.GET(t, e, "/cars/history/all?search=%25", h).Body)
	assert.EqualValues(t, 0, wildcard.Meta.Total, "процент в поиске - символ, а не маска на весь журнал")

	badDate := testutil.GET(t, e, "/cars/history/all?date_from=12.03.2026", h)
	assert.Equal(t, http.StatusBadRequest, badDate.Code, "битая дата - ошибка, а не тихая выдача всего журнала")
}

// Постраничное чтение не теряет и не повторяет строки, даже когда отметки попали в
// одну секунду. Что порядок при этом опирается на id вторым ключом, стережёт
// TestPassageHistoryOrderSQL_HasIDTiebreaker: на живой базе один и тот же план обычно
// отдаёт устойчивый порядок и без ключа, поэтому здесь это не проверить.
func TestCarsHistory_StableOrderWithinSameSecond(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	table := seedCarsTable(t, db, "ord_tbl", "Таблица порядка")
	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": "O001AA777", "car_brand": "Test"}]}`, td.OrgID, table)
	rec := testutil.POST(t, e, "/cars/manual", body, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	carID := int(testutil.ParseMap(t, rec)["car_ids"].([]interface{})[0].(float64))

	sameMoment := time.Now().Truncate(time.Second).Add(-time.Minute)
	want := make([]int, 0, 3)
	for i := 0; i < 3; i++ {
		want = append(want, writeCarPassageAt(t, db, carID, "entry", sameMoment, nil, &table))
	}

	seen := make(map[int]bool, 3)
	for page := 1; page <= 3; page++ {
		got := parsePassagePage(t, testutil.GET(t, e, fmt.Sprintf("/cars/history/all?per_page=1&page=%d", page), h).Body)
		require.Len(t, got.Data, 1, "страница %d", page)
		id := int(got.Data[0]["id"].(float64))
		assert.False(t, seen[id], "строка %d пришла дважды при постраничном чтении", id)
		seen[id] = true
	}
	assert.Len(t, seen, 3, "три отметки одной секунды прочитаны без потерь")
	for _, id := range want {
		assert.True(t, seen[id], "отметка %d не попала ни в одну страницу", id)
	}
}

// Значения выпадающих списков берутся из журнала, а не из справочника: предлагать
// всех заведённых пользователей в фильтре журнала проходов незачем, отмечают единицы.
func TestCarsHistory_FilterOptions(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)
	adminID := userIDByName(t, db, "testadmin")

	tableOwn := seedCarsTable(t, db, "opt_own", "Своя таблица")
	tableOther := seedCarsTable(t, db, "opt_other", "Чужая таблица")
	createCar := func(number string, tableID int) int {
		body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": %q, "car_brand": "Test"}]}`, td.OrgID, tableID, number)
		rec := testutil.POST(t, e, "/cars/manual", body, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return int(testutil.ParseMap(t, rec)["car_ids"].([]interface{})[0].(float64))
	}

	own := createCar("C001AA777", tableOwn)
	other := createCar("C002BB777", tableOther)

	now := time.Now()
	writeCarPassageAt(t, db, own, "entry", now.Add(-2*time.Minute), &adminID, &tableOwn)
	writeCarPassageAt(t, db, other, "entry", now.Add(-time.Minute), nil, &tableOther)

	all := testutil.ParseMap(t, testutil.GET(t, e, "/cars/history/filter-options", h))
	users := all["users"].([]interface{})
	require.Len(t, users, 1, "в списке только тот, кто отмечал проход")
	assert.Equal(t, float64(adminID), users[0].(map[string]interface{})["id"])

	// Скоуп таблицы: отметку без автора делали на чужом посту, поэтому в своей
	// таблице отметивших нет вовсе.
	scoped := testutil.ParseMap(t, testutil.GET(t, e, fmt.Sprintf("/cars/history/filter-options?table_id=%d", tableOther), h))
	assert.Empty(t, scoped["users"], "список сужается до своей таблицы проходной")

	bad := testutil.GET(t, e, "/cars/history/filter-options?table_id=abc", h)
	assert.Equal(t, http.StatusBadRequest, bad.Code, "мусор в table_id не должен молча отдавать весь журнал")
}
