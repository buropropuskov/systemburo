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

// #1238: у машины, добавленной вручную, детальная карточка показывала «Места
// разгрузки не указаны», хотя в строке таблицы место было видно. Строка берёт
// ИМЕНА мест отдельным запросом без заявки, а карточка строит секцию по
// unload_place_ids из GET /cars/unload-places - и тот запрос джойнил заявку
// внутренним джойном, выкидывая ручные машины (у них вложение без заявки).
func TestCarUnloadPlaces_ManualCarLinkIsReturned(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	tableID := seedCarsTable(t, db, "cup_manual_tbl", "Место разгрузки: ручная машина")

	place := models.UnloadPlace{Name: "ПОСТ №61 (ручная)", Status: "active"}
	require.NoError(t, db.Create(&place).Error)

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles": [
		{"car_number": "Т999ТТ999", "car_brand": "Тестовая", "unload_places": [%d]}
	]}`, td.OrgID, tableID, place.ID)
	rec := testutil.POST(t, e, "/cars/manual", body, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	carIDs := testutil.ParseMap(t, rec)["car_ids"].([]interface{})
	require.Len(t, carIDs, 1)
	carID := int(carIDs[0].(float64))

	// Связь в БД есть - ручное добавление её пишет; проверяем именно выдачу.
	var linkCount int64
	require.NoError(t, db.Table("car_unload_places").
		Where("car_id = ? AND unload_place_id = ?", carID, place.ID).Count(&linkCount).Error)
	require.EqualValues(t, 1, linkCount, "ручное добавление обязано создать связь машины с местом")

	recPlaces := testutil.GET(t, e, "/cars/unload-places", h)
	require.Equal(t, http.StatusOK, recPlaces.Code, recPlaces.Body.String())

	places := testutil.ParseSlice(t, recPlaces)
	found := false
	for _, row := range places {
		if int(row["car_id"].(float64)) == carID {
			found = true
			assert.Equal(t, float64(place.ID), row["unload_place_id"], "должен приехать id выбранного места")
			assert.Equal(t, place.Name, row["unload_place_name"])
		}
	}
	assert.True(t, found, "связь ручной машины с местом разгрузки должна попадать в /cars/unload-places: карточка строит секцию по ней")
}

// Заявочная ветка не должна пострадать: машина несогласованной заявки в выдачу
// по-прежнему не попадает (иначе фикс превратился бы в дыру видимости).
func TestCarUnloadPlaces_UnapprovedApplicationCarStaysHidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	place := models.UnloadPlace{Name: "ПОСТ №62 (заявочная)", Status: "active"}
	require.NoError(t, db.Create(&place).Error)

	// Вложение с заявкой, которая ещё НЕ согласована.
	num := "APP-CUP-PENDING"
	confirmation, appStatus := "Согласование", "Непрочитано"
	senderID := getUserID(t, db, "testadmin")
	app := models.Application{
		ApplicationNumber: &num,
		Confirmation:      &confirmation,
		Status:            &appStatus,
		OrganizationID:    td.OrgID,
		SenderUserID:      senderID,
	}
	require.NoError(t, db.Create(&app).Error)

	attStatus := 1
	from, to := "2026-07-01", "2099-12-31"
	att := models.Attachment{
		ApplicationID:  &app.ID,
		AttachmentType: "cars",
		EntryDateFrom:  &from,
		EntryDateTo:    &to,
		Status:         &attStatus,
	}
	require.NoError(t, db.Create(&att).Error)

	status := 1
	number, brand := "У555УУ555", "Заявочная"
	car := models.Car{AttachmentID: att.ID, CarNumber: &number, CarBrand: &brand, Status: &status}
	require.NoError(t, db.Create(&car).Error)
	orderIdx := 1
	require.NoError(t, db.Create(&models.CarUnloadPlace{CarID: car.ID, UnloadPlaceID: place.ID, OrderIndex: &orderIdx}).Error)

	rec := testutil.GET(t, e, "/cars/unload-places", h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	for _, row := range testutil.ParseSlice(t, rec) {
		assert.NotEqual(t, float64(car.ID), row["car_id"],
			"машина несогласованной заявки в выдачу мест попадать не должна")
	}
}
