package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Возражение субъекта против обработки его данных (#2361).
//
// Это не отзыв согласия: согласия участник заявки не давал, его уведомили, а
// обработка идёт по законному интересу оператора. Возразить он вправе, и пока
// возражение стоит, персональные поля записи только читаются. Организация и привязка
// остаются изменяемыми - иначе возражение запирало бы и порядок в справочниках.

func objectionEmployeeID(t *testing.T, db *gorm.DB, lastName string) int {
	t.Helper()
	var id int
	require.NoError(t, db.Model(&models.UniqueEmployee{}).Where("last_name = ?", lastName).
		Select("id").Row().Scan(&id))
	return id
}

func TestPDObjection_LocksPersonalFieldsButNotBinding(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Возражаев","first_name":"Пётр","position":"Слесарь","passport_series_number":"4501 111111","pd_consent":true}`, h).Code)
	id := objectionEmployeeID(t, db, "Возражаев")
	path := "/unique-employees/" + itoa(id)

	// До возражения правится обычным порядком.
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, path,
		`{"last_name":"Возражаев","first_name":"Пётр","position":"Мастер","pd_consent":true}`, h).Code)

	rec := testutil.POST(t, e, path+"/objection", `{"source":"письмо на почту бюро 06.09"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Персональные поля заперты.
	rec = testutil.PUT(t, e, path, `{"last_name":"Подменённый","first_name":"Пётр","pd_consent":true}`, h)
	assert.Equal(t, http.StatusConflict, rec.Code, "правка имени после возражения отклоняется")

	var last string
	require.NoError(t, db.Model(&models.UniqueEmployee{}).Where("id = ?", id).
		Select("COALESCE(last_name, '')").Row().Scan(&last))
	assert.Equal(t, "Возражаев", last, "прежнее значение сохранено")

	// Привязка меняется и при возражении; полный запрос с прежними ПД допустим.
	rec = testutil.PUT(t, e, path,
		`{"last_name":"Возражаев","first_name":"Пётр","position":"Мастер","passport_series_number":"4501 111111","company_id":`+itoa(td.CompanyID)+`,"pd_consent":true}`, h)
	assert.Equal(t, http.StatusOK, rec.Code, "привязка меняется и при возражении: "+rec.Body.String())

	var before models.UniqueEmployee
	require.NoError(t, db.First(&before, id).Error)
	// Короткий запрос действительно меняет привязку, не повторяя персональные поля.
	rec = testutil.PUT(t, e, path, `{"organization_id":`+itoa(td.OrgID)+`}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var after models.UniqueEmployee
	require.NoError(t, db.First(&after, id).Error)
	assert.Equal(t, &td.OrgID, after.OrganizationID)
	assert.Nil(t, after.CompanyID)
	assert.Equal(t, before.LastName, after.LastName)
	assert.Equal(t, before.FirstName, after.FirstName)
	assert.Equal(t, before.Position, after.Position)
	assert.Equal(t, before.PassportSeriesNumber, after.PassportSeriesNumber)
	assert.Equal(t, before.PDObjectionAt, after.PDObjectionAt)
	assert.Equal(t, before.PDObjectionSource, after.PDObjectionSource)

	// JSON null имеет ту же семантику для ПД, что и отсутствующее поле.
	rec = testutil.PUT(t, e, path, `{"organization_id":`+itoa(td.OrgID)+`,"last_name":null,"citizenship_id":null}`, h)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// Запрещённое изменение не должно частично менять привязку.
	for _, body := range []string{
		`{"last_name":"","company_id":` + itoa(td.CompanyID) + `}`,
		`{"passport_series_number":"4501 999999","company_id":` + itoa(td.CompanyID) + `}`,
	} {
		rec = testutil.PUT(t, e, path, body, h)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	}
	var unchanged models.UniqueEmployee
	require.NoError(t, db.First(&unchanged, id).Error)
	assert.Equal(t, after.LastName, unchanged.LastName)
	assert.Equal(t, after.PassportSeriesNumber, unchanged.PassportSeriesNumber)
	assert.Equal(t, after.OrganizationID, unchanged.OrganizationID)
	assert.Nil(t, unchanged.CompanyID)
	assert.Equal(t, before.PDObjectionAt, unchanged.PDObjectionAt)
}

func TestPDObjection_StoresWhoWhenAndSource(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Обращенцев","first_name":"Иван","position":"Слесарь","passport_series_number":"4501 222222","pd_consent":true}`, h).Code)
	id := objectionEmployeeID(t, db, "Обращенцев")

	// Дату из запроса сервер игнорирует: её ставит он сам, иначе возражением можно
	// было бы задним числом прикрыть уже сделанную правку.
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees/"+itoa(id)+"/objection",
		`{"source":"заявление через работодателя","pd_objection_at":"2000-01-01T00:00:00Z"}`, h).Code)

	var row models.UniqueEmployee
	require.NoError(t, db.First(&row, id).Error)
	require.NotNil(t, row.PDObjectionAt, "время проставлено сервером")
	assert.True(t, row.PDObjectionAt.Year() >= 2026, "год из запроса не принят")
	require.NotNil(t, row.PDObjectionByUserID, "автор записан")
	require.NotNil(t, row.PDObjectionSource)
	assert.Equal(t, "заявление через работодателя", *row.PDObjectionSource)

	// Повторная отметка бессмысленна и отклоняется.
	assert.Equal(t, http.StatusConflict, testutil.POST(t, e, "/unique-employees/"+itoa(id)+"/objection",
		`{"source":"ещё раз"}`, h).Code)
}

func TestPDObjection_RequiresSource(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Безоснования","first_name":"Пётр","position":"Слесарь","passport_series_number":"4501 333333","pd_consent":true}`, h).Code)
	id := objectionEmployeeID(t, db, "Безоснования")

	// Отметка без указания, откуда обращение, через полгода неотличима от случайного
	// нажатия, а разбирать возражение будет человек.
	assert.Equal(t, http.StatusBadRequest, testutil.POST(t, e, "/unique-employees/"+itoa(id)+"/objection",
		`{"source":"   "}`, h).Code)
}

func TestPDObjection_ClearUnlocksRecord(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Отозвавший","first_name":"Пётр","position":"Слесарь","passport_series_number":"4501 444444","pd_consent":true}`, h).Code)
	id := objectionEmployeeID(t, db, "Отозвавший")
	path := "/unique-employees/" + itoa(id)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, path+"/objection", `{"source":"звонок в бюро"}`, h).Code)
	require.Equal(t, http.StatusConflict, testutil.PUT(t, e, path,
		`{"last_name":"Новый","pd_consent":true}`, h).Code)

	// Отметку ставят и ошибочно: запись не должна остаться запертой навсегда из-за описки.
	require.Equal(t, http.StatusOK, testutil.DELETE(t, e, path+"/objection", h).Code)
	assert.Equal(t, http.StatusOK, testutil.PUT(t, e, path,
		`{"last_name":"Отозвавший","first_name":"Пётр","position":"Бригадир","pd_consent":true}`, h).Code,
		"после снятия правка снова возможна")

	// Снимать нечего - отдельный случай, а не молчаливый успех.
	assert.Equal(t, http.StatusNotFound, testutil.DELETE(t, e, path+"/objection", h).Code)
}

// Снять отметку может только администратор бюро: возражение адресовано оператору,
// и решение по нему принимает он. Иначе заявитель снял бы чужое возражение и
// продолжил править данные человека, который этого не хотел.
func TestPDObjection_ClearIsAdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	applicantH := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "objection_applicant",
		"password123456789012345678901234", 1, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Заявителев","first_name":"Пётр","position":"Слесарь","passport_series_number":"4501 555555","pd_consent":true}`, applicantH).Code)
	id := objectionEmployeeID(t, db, "Заявителев")
	path := "/unique-employees/" + itoa(id)

	// Поставить отметку заявитель может: человек скажет о возражении работодателю.
	require.Equal(t, http.StatusOK, testutil.POST(t, e, path+"/objection",
		`{"source":"сказал лично"}`, applicantH).Code)

	// А снять - нет.
	assert.Equal(t, http.StatusForbidden, testutil.DELETE(t, e, path+"/objection", applicantH).Code,
		"заявитель не снимает возражение сам")

	assert.Equal(t, http.StatusOK, testutil.DELETE(t, e, path+"/objection", adminH).Code,
		"администратор бюро снимает по итогам рассмотрения")
}

func TestPDObjection_ClearDoesNotRevealEmployeeExistence(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	applicantH := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "objection_enumerator",
		"password123456789012345678901234", 1, td.OrgID, td.CompanyID))

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Скрытый","first_name":"Пётр","position":"Слесарь","passport_series_number":"4501 888888","pd_consent":true}`, adminH).Code)
	id := objectionEmployeeID(t, db, "Скрытый")
	path := "/unique-employees/" + itoa(id)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, path+"/objection", `{"source":"заявление"}`, adminH).Code)

	existing := testutil.DELETE(t, e, path+"/objection", applicantH)
	missing := testutil.DELETE(t, e, "/unique-employees/2000000000/objection", applicantH)
	assert.Equal(t, http.StatusForbidden, existing.Code)
	assert.Equal(t, existing.Code, missing.Code, "отказ не раскрывает существование записи")
	assert.Equal(t, existing.Body.String(), missing.Body.String(), "тело ответа одинаково для обоих ID")

	var row models.UniqueEmployee
	require.NoError(t, db.First(&row, id).Error)
	assert.NotNil(t, row.PDObjectionAt, "отказ не снял возражение")
	assert.NotNil(t, row.PDObjectionByUserID, "отказ не изменил автора возражения")
	assert.NotNil(t, row.PDObjectionSource, "отказ не изменил источник возражения")
	var cleared int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityUniqueEmployee, id, "pd_objection_cleared").
		Count(&cleared).Error)
	assert.Zero(t, cleared, "отказ не записал аудит снятия возражения")

	assert.Equal(t, http.StatusNotFound, testutil.DELETE(t, e, "/unique-employees/2000000000/objection", adminH).Code,
		"администратор по-прежнему получает 404 для неизвестного ID")
}

// Аннулирование действующих пропусков при возражении (#2361, решение владельца:
// аннулировать сразу). Человек возразил - его данные не используются больше ни для
// чего, включая проход. Строки убираются мягко: остаются в корзине и в истории,
// иначе порвалась бы связь с отметками прохода.
func TestPDObjection_RevokesActivePasses(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	const passport = "4501 777777"
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Проходов","first_name":"Семён","position":"Слесарь","passport_series_number":"`+passport+`","pd_consent":true}`, h).Code)
	id := objectionEmployeeID(t, db, "Проходов")

	// Действующая строка в заявке у того же человека: тот же документ, значит та же
	// свёртка, по ней его и находят.
	active := models.Employee{
		LastName:             ptr("Проходов"),
		FirstName:            ptr("Семён"),
		PassportSeriesNumber: ptr(passport),
		Status:               ptrInt(1),
	}
	require.NoError(t, db.Create(&active).Error)

	// Уже убранная строка повторно не трогается и в счёт не идёт.
	removedAt := time.Now().UTC().Add(-time.Hour)
	removed := models.Employee{
		LastName:             ptr("Проходов"),
		FirstName:            ptr("Семён"),
		PassportSeriesNumber: ptr(passport),
		Status:               ptrInt(0),
		DateDeleted:          &removedAt,
	}
	require.NoError(t, db.Create(&removed).Error)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees/"+itoa(id)+"/objection",
		`{"source":"письмо в бюро"}`, h).Code)

	var got models.Employee
	require.NoError(t, db.First(&got, active.ID).Error)
	require.NotNil(t, got.DateDeleted, "действующая строка убрана из заявки")
	require.NotNil(t, got.Status)
	assert.Equal(t, 0, *got.Status, "статус обнулён")

	// Мягко: строка на месте, а не удалена физически - иначе оборвалась бы связь
	// с отметками прохода.
	var alive int64
	require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", active.ID).Count(&alive).Error)
	assert.EqualValues(t, 1, alive, "строка осталась в базе")

	// Событие видно в истории строки: «кто убрал и почему» спрашивают именно про неё.
	var events int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityEmployee, active.ID, "delete").
		Count(&events).Error)
	assert.EqualValues(t, 1, events, "аннулирование записано в историю строки")
}

func ptr(s string) *string { return &s }
func ptrInt(i int) *int    { return &i }
