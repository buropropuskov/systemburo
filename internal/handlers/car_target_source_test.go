package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Тесты источника привязки car_target_tables (#1227, срез P1): submit заявки пишет
// source=application через дефолт колонки (сырой INSERT без явного source), bulk/ручные
// операции проставляют source=manual явно, а detail-путь проходной отдаёт список
// привязок {id,name,source} вместо голого счётчика.

// Подача заявки с выбранным «Проезд» пишет car_target_tables.source='application' -
// столбец получает значение от дефолта колонки, submit (сырой INSERT) его не проставляет.
func TestCarTargetSource_SubmitWritesApplicationSource(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	dn := "Проезд Src A"
	tbl := models.SystemTable{Name: "cars_src_a", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)

	token := testutil.RegisterAndLogin(t, e, "carsrc1", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "cars_src_tmpl", "Cars Src")

	body := fmt.Sprintf(`{
		"message":"source test","organization":"Test Organization",
		"responsible_person":"Test","contact_phone":"+79001234567","data_approval":true,
		"attachments":[{"attachment_type":"cars","attachment_name":"cars_tmpl",
			"attachment_display_name":"Cars Template","unique_attachment_id":%d,
			"entry_date_from":"2026-04-01","entry_date_to":"2099-12-31",
			"data":{"vehicles":[{"car_number":"E111EE177","car_brand":"Lada","passage_tables":[%d]}]}}]
	}`, uaID, tbl.ID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var source string
	require.NoError(t, db.Table("car_target_tables ctt").
		Select("ctt.source").
		Joins("JOIN cars c ON c.id = ctt.car_id").
		Where("ctt.table_id = ? AND c.car_number = ?", tbl.ID, "E111EE177").
		Scan(&source).Error)
	assert.Equal(t, "application", source, "submit должен получить source=application от дефолта колонки")
}

// Ручное добавление машины (POST /cars/manual, #1049) и bulk «Добавить» (POST
// /cars/bulk/add-table, #1194) - оба ручные действия, оба пишут source=manual.
func TestCarTargetSource_ManualAndBulkAddWriteManualSource(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	tableA := seedCarsTable(t, db, "src_manual_a", "Src Manual A")
	tableB := seedCarsTable(t, db, "src_manual_b", "Src Manual B")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [{"car_number": "F222FF177", "car_brand": "Test"}]}`,
		td.OrgID, tableA)
	rec := testutil.POST(t, e, "/cars/manual", body, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	carIDs := testutil.ParseMap(t, rec)["car_ids"].([]interface{})
	carID := int(carIDs[0].(float64))

	var sourceA string
	require.NoError(t, db.Table("car_target_tables").Select("source").
		Where("car_id = ? AND table_id = ?", carID, tableA).Scan(&sourceA).Error)
	assert.Equal(t, "manual", sourceA, "ручное добавление машины (#1049) проставляет source=manual")

	recAdd := testutil.POST(t, e, "/cars/bulk/add-table", fmt.Sprintf(`{"ids":[%d],"table_ids":[%d]}`, carID, tableB), h)
	require.Equal(t, http.StatusOK, recAdd.Code, recAdd.Body.String())

	var sourceB string
	require.NoError(t, db.Table("car_target_tables").Select("source").
		Where("car_id = ? AND table_id = ?", carID, tableB).Scan(&sourceB).Error)
	assert.Equal(t, "manual", sourceB, "BulkAddTable (#1194) проставляет source=manual")
}

// GET /cars/active-for-table(s) отдаёт target_tables[{id,name,source}] вместо голого
// счётчика (#1227) - карточка машины из контекста проходной сможет показать бейдж источника.
func TestCarTargetSource_DetailIncludesTargetTables(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "carsrcdet1", "pass123", 1, td.OrgID, td.CompanyID)
	appID, _, _ := seedCarViaCompleteApp(t, e, db, token, "Test Organization")
	activateCarViaApp(t, e, db, appID, td)

	// «Ключ есть, но пустой» смотрим на составе заявки: машина без выбранного «Проезда»
	// в адресную выдачу таблицы не попадает по определению, а фронт должен различать
	// «привязок нет» и «поле не пришло». Раньше проверка шла через /cars/active-for-tables -
	// тот путь снят вместе с техдолгом (#1050).
	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	atts := testutil.ParseSlice(t, rec)
	require.NotEmpty(t, atts)
	attID := int(atts[0]["id"].(float64))

	rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	cars := testutil.ParseSlice(t, rec)
	require.Len(t, cars, 1)

	tt, ok := cars[0]["target_tables"]
	require.True(t, ok, "target_tables должен присутствовать в составе заявки")
	assert.Empty(t, tt)

	dn := "Проезд Src Detail"
	tbl := models.SystemTable{Name: "cars_src_detail", DisplayName: &dn, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	carID := int(cars[0]["id"].(float64))
	require.NoError(t, db.Create(&models.CarTargetTable{CarID: carID, TableID: tbl.ID, Source: "manual"}).Error)

	rec = testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", tbl.ID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	scoped := testutil.ParseSlice(t, rec)
	require.Len(t, scoped, 1)

	targetTables, ok := scoped[0]["target_tables"].([]interface{})
	require.True(t, ok)
	require.Len(t, targetTables, 1)
	item, ok := targetTables[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(tbl.ID), item["id"])
	assert.Equal(t, "Проезд Src Detail", item["name"])
	assert.Equal(t, "manual", item["source"])
}
