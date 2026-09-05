package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ограничения на заявку с машиной «По факту» (#2320).
//
// Такой пропуск не опознаёт конкретную машину: по нему на территорию заезжает кто
// угодно. До этих правил организация могла держать сколько угодно таких заявок и на
// любой срок - то есть постоянный безымянный пропуск.
//
// Отказ проверяем не только по коду 400, но и по тексту: требование прямо просит
// понятную ошибку на форме, а не общее «не удалось отправить».

// byFactBody собирает тело подачи с одной машиной «По факту» на заданный срок.
//
// Окно пребывания - до конца суток. Фиксированное «до 18:00» делало тесты
// непроходимыми после шести вечера по Москве: первая заявка успевала истечь, место
// освобождалось, и «вторая заявка отклоняется» падало на вечернем прогоне CI.
func byFactBody(uaID int, dateFrom, dateTo string) string {
	return fmt.Sprintf(`{
		"message": "по факту",
		"organization": "Test Organization",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "fact_tmpl",
			"attachment_display_name": "Fact Template",
			"unique_attachment_id": %d,
			"entry_date_from": "%s",
			"entry_date_to": "%s",
			"entry_time_from": "00:01",
			"entry_time_to": "23:59",
			"data": { "vehicles": [{ "car_number": "По факту", "car_brand": "Kamaz" }] }
		}]
	}`, uaID, dateFrom, dateTo)
}

func todayMSK() string {
	return time.Now().In(time.FixedZone("MSK", 3*60*60)).Format("2006-01-02")
}

func TestSubmitByFact_SecondApplicationRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "byfact1", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "fact_tmpl_"+t.Name(), "Fact Template")

	today := todayMSK()
	rec := testutil.POST(t, e, "/applications/submit-complete-application", byFactBody(uaID, today, today), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "первая заявка «По факту» должна проходить: %s", rec.Body.String())

	rec = testutil.POST(t, e, "/applications/submit-complete-application", byFactBody(uaID, today, today), testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code,
		"вторая заявка «По факту» при живой первой должна отклоняться: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "действующая заявка",
		"отказ обязан объяснять причину - на форме показывается именно этот текст")
}

func TestSubmitByFact_AllowedAfterFirstExpired(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "byfact2", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "fact_tmpl_"+t.Name(), "Fact Template")

	today := todayMSK()
	rec := testutil.POST(t, e, "/applications/submit-complete-application", byFactBody(uaID, today, today), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Срок первой закончился минуту назад по Москве - место освободилось.
	msk := time.Now().In(time.FixedZone("MSK", 3*60*60)).Add(-time.Minute)
	require.NoError(t, db.Exec(
		`UPDATE attachments SET entry_date_to = ?, entry_time_to = ?`,
		msk.Format("2006-01-02"), msk.Format("15:04:05"),
	).Error)

	rec = testutil.POST(t, e, "/applications/submit-complete-application", byFactBody(uaID, today, today), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code,
		"после истечения срока первой заявки вторую подать можно: %s", rec.Body.String())
}

func TestSubmitByFact_ClosedApplicationDoesNotBlock(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "byfact3", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "fact_tmpl_"+t.Name(), "Fact Template")

	today := todayMSK()
	rec := testutil.POST(t, e, "/applications/submit-complete-application", byFactBody(uaID, today, today), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Отозванная заявка место не занимает: срок у неё ещё «живой», но сама она закрыта.
	require.NoError(t, db.Exec(`UPDATE applications SET status = ?`, models.StatusWithdrawn).Error)

	rec = testutil.POST(t, e, "/applications/submit-complete-application", byFactBody(uaID, today, today), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code,
		"закрытая заявка не должна блокировать новую: %s", rec.Body.String())
}

// Крайняя дата - конец суток, в которые попадает «сейчас плюс 24 часа»: в 17:38
// пятого числа заявку можно оформить по шестое включительно. Округление вверх и
// есть тот запас, без которого предел сползал бы каждую минуту.
func TestByFactMaxDate_RoundsUpToEndOfDay(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)

	// Вечер: сутки попадают на следующий день, значит крайняя дата - он.
	assert.Equal(t, "2026-09-06",
		services.ByFactMaxDate(time.Date(2026, 9, 5, 17, 38, 0, 0, msk)))

	// Минутой позже предел тот же - за это и держались.
	assert.Equal(t, "2026-09-06",
		services.ByFactMaxDate(time.Date(2026, 9, 5, 17, 39, 0, 0, msk)))

	// Ранним утром сутки всё ещё уводят в следующий день.
	assert.Equal(t, "2026-09-06",
		services.ByFactMaxDate(time.Date(2026, 9, 5, 0, 30, 0, 0, msk)))

	// Момент задан в UTC - предел считается по московскому календарю.
	assert.Equal(t, "2026-09-06",
		services.ByFactMaxDate(time.Date(2026, 9, 4, 22, 0, 0, 0, time.UTC)))
}

func TestSubmitByFact_TomorrowAllowed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "byfacttomorrow", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "fact_tmpl_"+t.Name(), "Fact Template")

	msk := time.Now().In(time.FixedZone("MSK", 3*60*60))
	today := msk.Format("2006-01-02")
	tomorrow := msk.AddDate(0, 0, 1).Format("2006-01-02")

	rec := testutil.POST(t, e, "/applications/submit-complete-application",
		byFactBody(uaID, today, tomorrow), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code,
		"срок до конца завтрашнего дня укладывается в сутки с запасом: %s", rec.Body.String())
}

func TestSubmitByFact_MultiDayPeriodRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "byfact4", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "fact_tmpl_"+t.Name(), "Fact Template")

	msk := time.Now().In(time.FixedZone("MSK", 3*60*60))
	today := msk.Format("2006-01-02")
	afterTomorrow := msk.AddDate(0, 0, 2).Format("2006-01-02")

	rec := testutil.POST(t, e, "/applications/submit-complete-application",
		byFactBody(uaID, today, afterTomorrow), testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code,
		"срок «По факту» дальше суток с запасом не пускаем: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "не позже",
		"отказ обязан называть крайнюю дату, а не только запрещать")
}

func TestSubmitByFact_SecondVehicleInSameApplicationRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "byfact5", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "fact_tmpl_"+t.Name(), "Fact Template")

	today := todayMSK()
	body := fmt.Sprintf(`{
		"message": "две по факту",
		"organization": "Test Organization",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "fact_tmpl",
			"attachment_display_name": "Fact Template",
			"unique_attachment_id": %d,
			"entry_date_from": "%s",
			"entry_date_to": "%s",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": { "vehicles": [
				{ "car_number": "По факту", "car_brand": "Kamaz" },
				{ "car_number": "по факту", "car_brand": "Gazel" }
			] }
		}]
	}`, uaID, today, today)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code,
		"две машины «По факту» в одной заявке обходили бы ограничение: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "только одна машина")
}

// Дополнение уже поданной заявки идёт по тем же правилам: иначе ограничение
// обходится в два шага - подать заявку с номером, а «По факту» дописать потом.
func TestSupplementByFact_SecondVehicleRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, token := suppAuthor(t, e, db, "byfact_supp_author", td.OrgID, td.CompanyID)
	approverID, _ := suppAuthor(t, e, db, "byfact_supp_approver", td.OrgID, td.CompanyID)

	appID := suppApp(t, db, td.OrgID, authorID, "BYFACT-SUPP-1", models.ConfirmationApproved, models.StatusProcessing)
	suppResponsible(t, db, appID, approverID, true, "approved")
	// Два вложения в одной заявке: «По факту» уже лежит в первом, добавляют во
	// второе. Существующая проверка дублей смотрит внутри одного вложения и такую
	// добавку пропускает - правило про заявку целиком нужно отдельно.
	firstAttID := suppAttachment(t, db, appID, "cars", "2099-12-31")
	attID := suppAttachment(t, db, appID, "cars", "2099-12-31")
	tableID := seedCarsTable(t, db, "byfact_supp_pass", "Проезд по факту")
	placeID := seedPlace(t, db, "Склад по факту")

	require.NoError(t, db.Exec(
		`INSERT INTO cars (attachment_id, car_number, car_brand, status) VALUES (?, 'По факту', 'Kamaz', 1)`,
		firstAttID,
	).Error)

	body := fmt.Sprintf(`{
		"comment": "вторая по факту",
		"additions": [{
			"attachment_id": %d,
			"vehicles": [{"car_number":"По факту","car_brand":"Gazel","unload_places":[%d],"passage_tables":[%d]}]
		}]
	}`, attID, placeID, tableID)

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code,
		"вторая машина «По факту» через дополнение обходила бы ограничение: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "только одна машина")
}

// Машину с номером дополнением добавляют как обычно - правило её не касается.
func TestSupplementByFact_NamedVehicleAllowed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, token := suppAuthor(t, e, db, "byfact_supp2_author", td.OrgID, td.CompanyID)
	approverID, _ := suppAuthor(t, e, db, "byfact_supp2_approver", td.OrgID, td.CompanyID)

	appID := suppApp(t, db, td.OrgID, authorID, "BYFACT-SUPP-2", models.ConfirmationApproved, models.StatusProcessing)
	suppResponsible(t, db, appID, approverID, true, "approved")
	attID := suppAttachment(t, db, appID, "cars", "2099-12-31")
	tableID := seedCarsTable(t, db, "byfact_supp2_pass", "Проезд обычный")
	placeID := seedPlace(t, db, "Склад обычный")

	require.NoError(t, db.Exec(
		`INSERT INTO cars (attachment_id, car_number, car_brand, status) VALUES (?, 'По факту', 'Kamaz', 1)`,
		attID,
	).Error)

	body := fmt.Sprintf(`{
		"comment": "обычная машина",
		"additions": [{
			"attachment_id": %d,
			"vehicles": [{"car_number":"X777XX777","car_brand":"Gazel","unload_places":[%d],"passage_tables":[%d]}]
		}]
	}`, attID, placeID, tableID)

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code,
		"машина с номером дополнением добавляется как раньше: %s", rec.Body.String())
}

func TestSubmitByFact_NamedVehiclesUnaffected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "byfact6", "pass123", 1, td.OrgID, td.CompanyID)
	uaID := seedUniqueAttachment(t, db, "cars", "fact_tmpl_"+t.Name(), "Fact Template")

	// Обычные заявки с номерами - на любой срок и в любом количестве, как раньше.
	body := fmt.Sprintf(`{
		"message": "обычная",
		"organization": "Test Organization",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "fact_tmpl",
			"attachment_display_name": "Fact Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": { "vehicles": [{ "car_number": "A111AA777", "car_brand": "Kamaz" }] }
		}]
	}`, uaID)

	for i := 0; i < 2; i++ {
		rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code,
			"ограничение не должно задевать заявки с номерами: %s", rec.Body.String())
	}
}
