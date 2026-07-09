package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedCarsTable создаёт активную cars-таблицу «Проезда» и возвращает её ID.
func seedCarsTable(t *testing.T, db *gorm.DB, name, display string) int {
	t.Helper()
	tbl := models.SystemTable{Name: name, DisplayName: &display, TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&tbl).Error)
	return tbl.ID
}

// POST /cars/manual (#1049, режим-1): super-админ добавляет машину без заявки. Проверяем
// весь путь persist -> scoped-показ: вложение-сирота (application_id NULL, is_manual),
// машина активна (status=1), привязана к целевой таблице и видна в ней с меткой
// «добавлено вручную» (application_id пустой), но не видна в чужой таблице.
func TestCreateManualCars_PersistsAndScoped(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	tableID := seedCarsTable(t, db, "cars_manual_a", "Проезд A")
	otherTableID := seedCarsTable(t, db, "cars_manual_b", "Проезд B")

	body := fmt.Sprintf(`{
		"organization_id": %d,
		"company_id": %d,
		"table_id": %d,
		"entry_date_from": "2026-07-01",
		"entry_date_to": "2099-12-31",
		"entry_time_from": "08:00",
		"entry_time_to": "18:00",
		"vehicles": [{"car_number": "M100MM777", "car_brand": "Ford", "unload_places": []}]
	}`, td.OrgID, td.CompanyID, tableID)

	rec := testutil.POST(t, e, "/cars/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	resp := testutil.ParseResponse[services.ManualCarResponse](t, rec)
	require.Len(t, resp.CarIDs, 1)
	require.NotZero(t, resp.AttachmentID)

	// Вложение-сирота: application_id NULL, is_manual, org/company на вложении.
	var att models.Attachment
	require.NoError(t, db.First(&att, resp.AttachmentID).Error)
	assert.Nil(t, att.ApplicationID, "ручное вложение без заявки")
	assert.True(t, att.IsManual)
	require.NotNil(t, att.OrganizationID)
	assert.Equal(t, td.OrgID, *att.OrganizationID)

	// Машина активна и привязана к целевой таблице.
	var car models.Car
	require.NoError(t, db.First(&car, resp.CarIDs[0]).Error)
	require.NotNil(t, car.Status)
	assert.Equal(t, 1, *car.Status, "ручная машина сразу активна (одобрения нет)")
	var linkCount int64
	require.NoError(t, db.Table("car_target_tables").
		Where("car_id = ? AND table_id = ?", car.ID, tableID).Count(&linkCount).Error)
	assert.EqualValues(t, 1, linkCount, "машина привязана к таблице со страницы")

	// Видна в целевой таблице с меткой «добавлено вручную» (application_id пустой).
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	rows := testutil.ParseSlice(t, rec)
	require.Len(t, rows, 1, "ручная машина видна в целевой таблице")
	assert.Equal(t, "M100MM777", rows[0]["car_number"])
	assert.Nil(t, rows[0]["application_id"], "у ручной машины нет заявки (метка «добавлено вручную»)")
	assert.Equal(t, "Test Organization", rows[0]["organization"], "org резолвится с вложения через COALESCE")

	// Не видна в чужой таблице - scope, а не «во всех сразу».
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", otherTableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, testutil.ParseSlice(t, rec), "ручная машина не видна в непривязанной таблице")
}

// Долг ревью S1: app-detail путь к сироте (#1049) закрыт даже для super - вложение
// ручной машины не принадлежит заявке, GET /attachments/:id/cars отдаёт 403 (без гейта
// super байпасил бы CanAccessApplication на appID 0).
func TestGetAttachmentCars_ManualOrphan_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	tableID := seedCarsTable(t, db, "cars_manual_orphan", "Проезд O")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d,
		"vehicles": [{"car_number": "O200OO777", "car_brand": "Lada"}]}`, td.OrgID, tableID)
	rec := testutil.POST(t, e, "/cars/manual", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "manual add: %s", rec.Body.String())
	attID := testutil.ParseResponse[services.ManualCarResponse](t, rec).AttachmentID

	rec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code, "сирота ручного добавления недоступна через app-detail даже super")
}

// Гейт entity.cars.manual_add: обычный пользователь без права не может добавить вручную.
func TestCreateManualCars_ForbiddenWithoutPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "manualnoperm", "pass123", 1, td.OrgID, td.CompanyID)
	tableID := seedCarsTable(t, db, "cars_manual_gate", "Проезд G")

	body := fmt.Sprintf(`{"organization_id": %d, "table_id": %d,
		"vehicles": [{"car_number": "G300GG777", "car_brand": "UAZ"}]}`, td.OrgID, tableID)
	rec := testutil.POST(t, e, "/cars/manual", body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code, "без права entity.cars.manual_add - 403")
}

// Валидация: без организации / без таблицы / без машин - 400.
func TestCreateManualCars_Validation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	tableID := seedCarsTable(t, db, "cars_manual_val", "Проезд V")

	cases := map[string]string{
		"без организации": fmt.Sprintf(`{"table_id": %d, "vehicles":[{"car_number":"V1V"}]}`, tableID),
		"без таблицы":     fmt.Sprintf(`{"organization_id": %d, "vehicles":[{"car_number":"V1V"}]}`, td.OrgID),
		"без машин":       fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles":[]}`, td.OrgID, tableID),
		"пустой номер":    fmt.Sprintf(`{"organization_id": %d, "table_id": %d, "vehicles":[{"car_number":"  "}]}`, td.OrgID, tableID),
	}
	for name, body := range cases {
		rec := testutil.POST(t, e, "/cars/manual", body, testutil.AuthHeader(token))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s -> 400 (%s)", name, rec.Body.String())
	}
}
