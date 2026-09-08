package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Частичная правка записи реестра (#2400).
//
// Карта обновления собиралась из всех персональных полей разом, и поле, которого в
// теле нет, уходило в базу как NULL: запрос «поменяй компанию» стирал фамилию и
// должность. Через форму не воспроизводится - она шлёт все поля, - поэтому дефект
// прожил незамеченным и вскрылся частичным запросом на стенде.

func registryFieldsOf(t *testing.T, db *gorm.DB, id int) (last, first, position string) {
	t.Helper()
	require.NoError(t, db.Model(&models.UniqueEmployee{}).Where("id = ?", id).
		Select("COALESCE(last_name, ''), COALESCE(first_name, ''), COALESCE(position, '')").
		Row().Scan(&last, &first, &position))
	return
}

func TestRegistryPartialUpdate_KeepsFieldsNotSent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Стираев","first_name":"Иван","position":"Электрик","passport_series_number":"4588 010101","pd_consent":true}`, h).Code)
	id := objectionEmployeeID(t, db, "Стираев")

	// Ровно тот запрос, что стирал данные на стенде.
	rec := testutil.PUT(t, e, "/unique-employees/"+itoa(id),
		`{"company_id":`+itoa(td.CompanyID)+`}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	last, first, position := registryFieldsOf(t, db, id)
	assert.Equal(t, "Стираев", last, "фамилия не стёрта запросом, где её не было")
	assert.Equal(t, "Иван", first, "имя не стёрто")
	assert.Equal(t, "Электрик", position, "должность не стёрта")

	var companyID *int
	require.NoError(t, db.Model(&models.UniqueEmployee{}).Where("id = ?", id).
		Select("company_id").Row().Scan(&companyID))
	require.NotNil(t, companyID, "привязка при этом применилась")
	assert.Equal(t, td.CompanyID, *companyID)
}

// Пустая строка - другое дело: администратор вправе стереть ошибочно введённое, и
// «не прислали» с «прислали пусто» смешивать нельзя.
func TestRegistryPartialUpdate_EmptyStringStillClears(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Очищаев","first_name":"Пётр","middle_name":"Петрович","position":"Слесарь","passport_series_number":"4588 020202","pd_consent":true}`, h).Code)
	id := objectionEmployeeID(t, db, "Очищаев")

	rec := testutil.PUT(t, e, "/unique-employees/"+itoa(id),
		`{"last_name":"Очищаев","first_name":"Пётр","middle_name":"","position":"Слесарь"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var middle string
	require.NoError(t, db.Model(&models.UniqueEmployee{}).Where("id = ?", id).
		Select("COALESCE(middle_name, '')").Row().Scan(&middle))
	assert.Empty(t, middle, "явно переданная пустая строка очищает отчество")

	last, _, position := registryFieldsOf(t, db, id)
	assert.Equal(t, "Очищаев", last)
	assert.Equal(t, "Слесарь", position)
}

// Тот же дефект в реестре машин: запрос с одним номером обнулял марку и формат.
// Проверено на стенде вживую - «Проверка2400» превратилась в пустую строку.
func carFieldsOf(t *testing.T, db *gorm.DB, id int) (mark string, formatID *int) {
	t.Helper()
	require.NoError(t, db.Model(&models.UniqueCar{}).Where("id = ?", id).
		Select("COALESCE(mark, ''), format_id").Row().Scan(&mark, &formatID))
	return
}

func createFormat(t *testing.T, db *gorm.DB) int {
	t.Helper()
	format := models.LicensePlateFormat{Name: "Проверочный формат"}
	require.NoError(t, db.Create(&format).Error)
	return format.ID
}

func carIDByNumber(t *testing.T, db *gorm.DB, number string) int {
	t.Helper()
	var id int
	require.NoError(t, db.Model(&models.UniqueCar{}).Where("number = ?", number).
		Select("id").Row().Scan(&id))
	return id
}

func TestRegistryPartialUpdate_CarKeepsMarkAndFormat(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	formatID := createFormat(t, db)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars",
		`{"number":"Х 999 ХХ 199","mark":"Проверка2400","format_id":`+itoa(formatID)+`}`, h).Code)
	id := carIDByNumber(t, db, "Х 999 ХХ 199")

	rec := testutil.PUT(t, e, "/unique-cars/"+itoa(id), `{"number":"Х 999 ХХ 199"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	mark, format := carFieldsOf(t, db, id)
	assert.Equal(t, "Проверка2400", mark, "марка не стёрта запросом, где её не было")
	require.NotNil(t, format, "формат не стёрт запросом, где его не было")
	assert.Equal(t, formatID, *format)
}

// Пустая марка по-прежнему очищает: администратор вправе стереть ошибочный ввод.
func TestRegistryPartialUpdate_CarEmptyMarkClears(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-cars",
		`{"number":"У 888 УУ 188","mark":"Ошибочная"}`, h).Code)
	id := carIDByNumber(t, db, "У 888 УУ 188")

	rec := testutil.PUT(t, e, "/unique-cars/"+itoa(id), `{"number":"У 888 УУ 188","mark":""}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	mark, _ := carFieldsOf(t, db, id)
	assert.Empty(t, mark, "явно переданная пустая марка очищает значение")
}
