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
)

// Согласие субъекта на обработку персональных данных (152-ФЗ). В карточке сотрудника
// вводят паспорт и патент, то есть данные третьего лица, поэтому запись реестра без
// отметки не создаётся. Дату и автора ставит сервер: запрос несёт только флаг, иначе
// датой согласия можно было бы прислать что угодно.
func TestPDConsent_RegistryRejectsEmployeeWithoutConsent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	rec := testutil.POST(t, e, "/unique-employees", `{"last_name":"Безсогласия","first_name":"Иван"}`, h)
	require.Equal(t, http.StatusBadRequest, rec.Code, "без отметки запись реестра не создаётся")
	assert.Contains(t, rec.Body.String(), "согласия субъекта")

	var count int64
	require.NoError(t, db.Model(&models.UniqueEmployee{}).Where("last_name = ?", "Безсогласия").Count(&count).Error)
	assert.Zero(t, count, "отказ не оставляет записи в базе")
}

func TestPDConsent_RegistryStoresWhoAndWhen(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Дату согласия из запроса сервер игнорирует - её ставит он сам.
	rec := testutil.POST(t, e, "/unique-employees",
		`{"pd_consent":true,"pd_consent_at":"2000-01-01T00:00:00Z","last_name":"Согласный","first_name":"Пётр"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	created := testutil.ParseResponse[services.UniqueEmployeeResponse](t, rec)
	require.NotNil(t, created.PDConsentAt, "ответ несёт дату согласия")
	assert.True(t, created.PDConsentAt.Year() >= 2026, "дату ставит сервер, а не запрос: %s", created.PDConsentAt)

	var stored models.UniqueEmployee
	require.NoError(t, db.First(&stored, created.ID).Error)
	require.NotNil(t, stored.PDConsentAt)
	require.NotNil(t, stored.PDConsentByUserID, "видно, кто подтвердил согласие")

	var author models.User
	require.NoError(t, db.First(&author, *stored.PDConsentByUserID).Error)
	assert.Equal(t, "testadmin", author.Username)
}

// Правка только ДОБАВЛЯЕТ отметку: у записи, заведённой до введения поля, согласие можно
// подтвердить, а снять снятой галочкой нельзя - иначе полученное согласие исчезало бы из
// базы вместе с исправлением опечатки в фамилии.
func TestPDConsent_UpdateAddsConsentButNeverRemovesIt(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	// Запись «из прошлого»: отметки нет.
	last, first := "Старый", "Иван"
	legacy := models.UniqueEmployee{LastName: &last, FirstName: &first, OrganizationID: &td.OrgID}
	require.NoError(t, db.Create(&legacy).Error)

	rec := testutil.PUT(t, e, fmt.Sprintf("/unique-employees/%d", legacy.ID),
		`{"pd_consent":true,"last_name":"Старый","first_name":"Иван"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var withConsent models.UniqueEmployee
	require.NoError(t, db.First(&withConsent, legacy.ID).Error)
	require.NotNil(t, withConsent.PDConsentAt, "правка поставила отметку")
	grantedAt := *withConsent.PDConsentAt

	rec = testutil.PUT(t, e, fmt.Sprintf("/unique-employees/%d", legacy.ID),
		`{"pd_consent":false,"last_name":"Старый","first_name":"Иоанн"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var afterUnchecked models.UniqueEmployee
	require.NoError(t, db.First(&afterUnchecked, legacy.ID).Error)
	require.NotNil(t, afterUnchecked.PDConsentAt, "снятая галочка не убирает полученное согласие")
	assert.WithinDuration(t, grantedAt, *afterUnchecked.PDConsentAt, 0, "дата согласия не переписывается")
}

// Подача заявки: отметка едет флагом и оседает на строке вложения. Строгость на сервере
// включает администратор настройкой поля - тем же порядком, что у прочих полей вложения
// (#529 H-9), а форма подачи требует галочку у каждого человека по умолчанию.
func TestPDConsent_SubmitStoresMarkOnEmployeeRow(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pdc_user", "pass123", 1, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)
	uaID := seedUniqueAttachment(t, db, "people", "pdc_tmpl", "tmpl")

	data := fmt.Sprintf(`{"employees": [
		{"pd_consent": true, "last_name": "Согласный", "first_name": "Пётр", "citizenship_id": %d, "position": "Монтажник", "passport_series_number": "4510 555666", "target_tables": [%d]},
		{"last_name": "Безотметки", "first_name": "Семён", "citizenship_id": %d, "position": "Слесарь", "passport_series_number": "4510 777888", "target_tables": [%d]}
	]}`, citizenshipID, tableID, citizenshipID, tableID)

	rec := submitFC(t, e, token, "Test Organization", "people", uaID, data)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var withMark models.Employee
	require.NoError(t, db.Where("last_name = ?", "Согласный").First(&withMark).Error)
	require.NotNil(t, withMark.PDConsentAt, "отметка сохранена на строке заявки")
	require.NotNil(t, withMark.PDConsentByUserID, "видно, кто подтвердил")

	var author models.User
	require.NoError(t, db.First(&author, *withMark.PDConsentByUserID).Error)
	assert.Equal(t, "pdc_user", author.Username)

	var withoutMark models.Employee
	require.NoError(t, db.Where("last_name = ?", "Безотметки").First(&withoutMark).Error)
	assert.Nil(t, withoutMark.PDConsentAt, "без флага отметки нет - пустое поле, а не сегодняшняя дата")
	assert.Nil(t, withoutMark.PDConsentByUserID)
}

// Администратор пометил поле согласия обязательным в шаблоне вложения - сервер начинает
// отклонять подачу без отметки. Мерка та же, что у прочих настроенных полей.
func TestPDConsent_ConfiguredRequiredRejectsSubmitWithoutMark(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "pdc_strict", "pass123", 1, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)
	uaID := seedUniqueAttachment(t, db, "people", "pdc_strict_tmpl", "tmpl")
	seedFieldConfig(t, db, uaID, services.PDConsentFieldKey, true, true)

	data := fmt.Sprintf(`{"employees": [{"last_name": "Отказной", "first_name": "Илья", "citizenship_id": %d, "position": "Монтажник", "passport_series_number": "4510 999000", "target_tables": [%d]}]}`, citizenshipID, tableID)
	rec := submitFC(t, e, token, "Test Organization", "people", uaID, data)
	require.Equal(t, http.StatusBadRequest, rec.Code, "настроенное обязательным поле требует отметки")

	var count int64
	require.NoError(t, db.Model(&models.Employee{}).Where("last_name = ?", "Отказной").Count(&count).Error)
	assert.Zero(t, count, "отказ до транзакции - строки не появились")

	// Контроль: та же подача с отметкой проходит.
	dataOK := fmt.Sprintf(`{"employees": [{"pd_consent": true, "last_name": "Проходной", "first_name": "Илья", "citizenship_id": %d, "position": "Монтажник", "passport_series_number": "4510 999111", "target_tables": [%d]}]}`, citizenshipID, tableID)
	rec = submitFC(t, e, token, "Test Organization", "people", uaID, dataOK)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
