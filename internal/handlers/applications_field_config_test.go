package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedFieldConfig добавляет override настройки поля вложения (#529 H-9).
func seedFieldConfig(t *testing.T, db *gorm.DB, uaID int, key string, visible, required bool) {
	t.Helper()
	require.NoError(t, db.Create(&models.AttachmentFieldConfig{
		UniqueAttachmentID: uaID, FieldKey: key, Visible: visible, Required: required,
	}).Error)
}

// submitFC подаёт заявку с одним вложением заданного типа и data-объектом (сырой JSON).
func submitFC(t *testing.T, e *echo.Echo, token, org, attType string, uaID int, dataJSON string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{
		"message": "h9",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "%s",
			"attachment_name": "h9_tmpl",
			"attachment_display_name": "H9 Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": %s
		}]
	}`, org, attType, uaID, dataJSON)
	return testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
}

// submitByFactOneDay - подача с машиной «По факту»: срок такой заявки ограничен
// одним днём (#2320), поэтому общий хелпер с периодом до 2099 года ей не подходит.
func submitByFactOneDay(t *testing.T, e *echo.Echo, token, org string, uaID int, dataJSON string) *httptest.ResponseRecorder {
	t.Helper()
	day := time.Now().In(time.FixedZone("MSK", 3*60*60)).Format("2006-01-02")
	body := fmt.Sprintf(`{
		"message": "h9",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "h9_tmpl",
			"attachment_display_name": "H9 Template",
			"unique_attachment_id": %d,
			"entry_date_from": "%s",
			"entry_date_to": "%s",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": %s
		}]
	}`, org, uaID, day, day, dataJSON)
	return testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
}

// TestSubmitCompleteApplication_FieldConfigValidation проверяет config-only валидацию
// обязательных полей (#529 H-9): бэкенд требует поле только если админ явно настроил его
// required (override visible+required). Скрытые и ненастроенные поля не валидируются.
func TestSubmitCompleteApplication_FieldConfigValidation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const orgName = "Test Organization"
	token := testutil.RegisterAndLogin(t, e, "h9_user", "pass123", 1, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)

	t.Run("people: настроенное required middle_name - без отчества 400", func(t *testing.T) {
		uaID := seedUniqueAttachment(t, db, "people", "h9_mn_req", "tmpl")
		seedFieldConfig(t, db, uaID, "middle_name", true, true)
		data := fmt.Sprintf(`{"employees": [{"last_name": "Иванов", "first_name": "Иван", "citizenship_id": %d, "position": "Рабочий", "passport_series_number": "1234 567890", "target_tables": [%d]}]}`, citizenshipID, tableID)
		rec := submitFC(t, e, token, orgName, "people", uaID, data)
		require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Отчество")
	})

	t.Run("people: настроенное required middle_name - с отчеством 200", func(t *testing.T) {
		uaID := seedUniqueAttachment(t, db, "people", "h9_mn_ok", "tmpl")
		seedFieldConfig(t, db, uaID, "middle_name", true, true)
		data := fmt.Sprintf(`{"employees": [{"last_name": "Иванов", "first_name": "Иван", "middle_name": "Иванович", "citizenship_id": %d, "position": "Рабочий", "passport_series_number": "1234 567890", "target_tables": [%d]}]}`, citizenshipID, tableID)
		rec := submitFC(t, e, token, orgName, "people", uaID, data)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("people: скрытое поле (visible=false) не валидируется даже при required", func(t *testing.T) {
		uaID := seedUniqueAttachment(t, db, "people", "h9_hidden", "tmpl")
		seedFieldConfig(t, db, uaID, "passport", false, true) // скрыто -> не требуем
		data := fmt.Sprintf(`{"employees": [{"last_name": "Петров", "first_name": "Пётр", "citizenship_id": %d, "position": "Рабочий", "target_tables": [%d]}]}`, citizenshipID, tableID)
		rec := submitFC(t, e, token, orgName, "people", uaID, data)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("cars: настроенное required unloading_places - без мест 400", func(t *testing.T) {
		uaID := seedUniqueAttachment(t, db, "cars", "h9_unload_req", "tmpl")
		seedFieldConfig(t, db, uaID, "unloading_places", true, true)
		data := `{"vehicles": [{"car_number": "A123AA77", "car_brand": "Kamaz", "mark_id": null}]}`
		rec := submitFC(t, e, token, orgName, "cars", uaID, data)
		require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Места разгрузки")
	})

	t.Run("cars: настроенное required passage_tables - без проезда 400", func(t *testing.T) {
		uaID := seedUniqueAttachment(t, db, "cars", "h9_passage_req", "tmpl")
		seedFieldConfig(t, db, uaID, "passage_tables", true, true)
		data := `{"vehicles": [{"car_number": "A123AA77", "car_brand": "Kamaz", "mark_id": null}]}`
		rec := submitFC(t, e, token, orgName, "cars", uaID, data)
		require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Проезд")
	})

	t.Run("cars: by-fact 'По факту' проходит required number и mark", func(t *testing.T) {
		uaID := seedUniqueAttachment(t, db, "cars", "h9_byfact", "tmpl")
		seedFieldConfig(t, db, uaID, "number", true, true)
		seedFieldConfig(t, db, uaID, "mark", true, true)
		data := `{"vehicles": [{"car_number": "По факту", "car_brand": "По факту", "mark_id": null}]}`
		// Общий срок хелпера («по 2099 год») заявке «По факту» больше не разрешён
		// (#2320), а проверяется здесь не он, а обязательность номера и марки.
		rec := submitByFactOneDay(t, e, token, orgName, uaID, data)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("items: настроенное required quantity - count 0 -> 400", func(t *testing.T) {
		uaID := seedUniqueAttachment(t, db, "items", "h9_qty_req", "tmpl")
		seedFieldConfig(t, db, uaID, "quantity", true, true)
		data := `{"items": [{"name": "Ящик", "count": 0}]}`
		rec := submitFC(t, e, token, orgName, "items", uaID, data)
		require.Equal(t, http.StatusBadRequest, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Количество")
	})

	t.Run("items: настроенное required quantity - count>=1 -> 200", func(t *testing.T) {
		uaID := seedUniqueAttachment(t, db, "items", "h9_qty_ok", "tmpl")
		seedFieldConfig(t, db, uaID, "quantity", true, true)
		data := `{"items": [{"name": "Ящик", "count": 3}]}`
		rec := submitFC(t, e, token, orgName, "items", uaID, data)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("без override валидация не срабатывает (текущее поведение сохранено)", func(t *testing.T) {
		uaID := seedUniqueAttachment(t, db, "cars", "h9_no_override", "tmpl")
		// Нет ни одной строки конфига -> бэкенд не требует unload_places, как и раньше.
		data := `{"vehicles": [{"car_number": "B456BB77", "car_brand": "Kamaz", "mark_id": null}]}`
		rec := submitFC(t, e, token, orgName, "cars", uaID, data)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	})
}

// TestSubmitCompleteApplication_BlacklistHiddenFIO проверяет тихую деградацию ЧС (#529 H-9):
// при скрытом ФИО (пустые last_name+first_name) проверка чёрного списка пропускается -
// не падает и не матчит, даже если в ЧС есть запись с пустым ФИО.
func TestSubmitCompleteApplication_BlacklistHiddenFIO(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	token := testutil.RegisterAndLogin(t, e, "h9_bl_user", "pass123", 1, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)

	// Патологическая запись ЧС с пустым ФИО: без skip пустая подача сматчилась бы 409.
	require.NoError(t, db.WithContext(ctx).Create(&models.PersonBlacklist{
		LastName: "", FirstName: "", Reason: "пустой эталон", IsActive: true,
	}).Error)

	uaID := seedUniqueAttachment(t, db, "people", "h9_hidden_fio", "tmpl")
	// ФИО скрыто конфигом -> фронт шлёт пустые имена.
	seedFieldConfig(t, db, uaID, "last_name", false, false)
	seedFieldConfig(t, db, uaID, "first_name", false, false)
	data := fmt.Sprintf(`{"employees": [{"last_name": "", "first_name": "", "citizenship_id": %d, "position": "Рабочий", "passport_series_number": "1234 567890", "target_tables": [%d]}]}`, citizenshipID, tableID)
	rec := submitFC(t, e, token, orgName, "people", uaID, data)
	require.Equal(t, http.StatusOK, rec.Code, "пустое ФИО должно пропускать ЧС, а не падать/блокировать; body: %s", rec.Body.String())
}

// TestSubmitCompleteApplication_RoofParkingPersisted проверяет, что флаги
// roof_access/free_parking сохраняются на вложении при подаче (#529 - основа для тегов).
func TestSubmitCompleteApplication_RoofParkingPersisted(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const orgName = "Test Organization"
	token := testutil.RegisterAndLogin(t, e, "rp_user", "pass123", 1, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)
	uaID := seedUniqueAttachment(t, db, "people", "rp_tmpl", "tmpl")

	body := fmt.Sprintf(`{
		"message": "rp",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "people",
			"attachment_name": "rp_tmpl",
			"attachment_display_name": "RP",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"roof_access": true,
			"free_parking": true,
			"data": {"employees": [{"last_name": "Иванов", "first_name": "Иван", "citizenship_id": %d, "position": "Рабочий", "passport_series_number": "1234 567890", "target_tables": [%d]}]}
		}]
	}`, orgName, uaID, citizenshipID, tableID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var saved struct {
		RoofAccess  bool `gorm:"column:roof_access"`
		FreeParking bool `gorm:"column:free_parking"`
	}
	require.NoError(t, db.Raw(
		"SELECT roof_access, free_parking FROM attachments WHERE unique_attachment_id = ? ORDER BY id DESC LIMIT 1", uaID,
	).Scan(&saved).Error)
	assert.True(t, saved.RoofAccess, "roof_access должен сохраниться true")
	assert.True(t, saved.FreeParking, "free_parking должен сохраниться true")
}
