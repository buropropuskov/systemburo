package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Решение по раунду дополнения (#1685, срез S4): принятие и отказ принимающего, снятие
// раунда автором.
//
// Стережётся здесь две вещи. Первая - принятие поднимает на КПП ТОЛЬКО состав своего
// раунда: исходные строки уже активны, и повторный проход написал бы им вторую историю
// «Добавлен в таблицу проходной» - попадание, которого не было. Вторая - принятие не
// воскрешает строки, уехавшие в корзину или погашенные чёрным списком: они лежат ровно с
// тем же status = 0, что и непринятая добавка, и предикат статуса сам по себе их не
// отличает.
//
// Секциями на одном поднятом приложении: отдельные SetupTestApp с CleanDB на каждую секцию
// перебивают границу go test -timeout у пакета handlers (та же причина, что в
// application_supplement_vote_test).

// suppDecTable - пост проходной, к которому привязываются строки: без привязки история
// попадания в таблицу не пишется вовсе, и дубли было бы не на чем ловить.
func suppDecTable(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	st := models.SystemTable{Name: name, TableType: "people", IsActive: true}
	require.NoError(t, db.Create(&st).Error)
	return st.ID
}

// suppDecEmployee создаёт сотрудника вложения с заданным статусом и привязкой к посту.
// supplementID nil - исходный состав подачи.
func suppDecEmployee(t *testing.T, db *gorm.DB, attID, tableID int, lastName string, supplementID *int, status int) int {
	t.Helper()
	ln, st := lastName, status
	emp := models.Employee{
		AttachmentID: &attID,
		SupplementID: supplementID,
		LastName:     &ln,
		Status:       &st,
	}
	require.NoError(t, db.Create(&emp).Error)
	require.NoError(t, db.Create(&models.EmployeeTargetTable{EmployeeID: emp.ID, TableID: tableID}).Error)
	return emp.ID
}

func suppDecCar(t *testing.T, db *gorm.DB, attID int, number string, supplementID *int, status int) int {
	t.Helper()
	num, st := number, status
	car := models.Car{
		AttachmentID: attID,
		SupplementID: supplementID,
		CarNumber:    &num,
		Status:       &st,
	}
	require.NoError(t, db.Create(&car).Error)
	return car.ID
}

func suppDecEmployeeStatus(t *testing.T, db *gorm.DB, id int) int {
	t.Helper()
	var emp models.Employee
	require.NoError(t, db.First(&emp, id).Error)
	require.NotNil(t, emp.Status, "статус сотрудника не проставлен")
	return *emp.Status
}

func suppDecCarStatus(t *testing.T, db *gorm.DB, id int) int {
	t.Helper()
	var car models.Car
	require.NoError(t, db.First(&car, id).Error)
	require.NotNil(t, car.Status, "статус машины не проставлен")
	return *car.Status
}

// suppDecAddedToTableCount - сколько записей «Добавлен в таблицу проходной» у сущности.
// Именно этим счётчиком ловится повторная активация уже стоящих на КПП строк.
func suppDecAddedToTableCount(t *testing.T, db *gorm.DB, entityType string, entityID int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", entityType, entityID, models.AuditActionAddedToTable).
		Count(&n).Error)
	return n
}

// suppDecAssertApplicationIntact - вердикт заявки на месте. Ради этого дополнение и заведено
// отдельной сущностью: откат confirmation/status снял бы уже выданные пропуска, поэтому
// проверка стоит после КАЖДОГО решения по раунду.
func suppDecAssertApplicationIntact(t *testing.T, db *gorm.DB, appID int) {
	t.Helper()
	app := suppReadApplication(t, db, appID)
	require.NotNil(t, app.Confirmation)
	require.NotNil(t, app.Status)
	assert.Equal(t, models.ConfirmationApproved, *app.Confirmation, "решение по раунду не двигает согласование заявки")
	assert.Equal(t, models.StatusInWork, *app.Status, "решение по раунду не двигает статус заявки")
}

func TestSupplementDecision(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, authorToken := suppVoteUser(t, e, db, "dec_author", td.OrgID, td.CompanyID)

	// Принимающий - глобальная роль: тот же, кто принимает саму заявку.
	acceptorID, acceptorToken := suppVoteUser(t, e, db, "dec_acceptor", td.OrgID, td.CompanyID)
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: acceptorID}).Error)

	decide := func(token string, appID, roundID int, body string) *httptest.ResponseRecorder {
		return testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements/%d/take-to-work", appID, roundID),
			body, testutil.AuthHeader(token))
	}
	cancel := func(token string, appID, roundID int, body string) *httptest.ResponseRecorder {
		return testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements/%d/cancel", appID, roundID),
			body, testutil.AuthHeader(token))
	}

	t.Run("принятие поднимает только строки своего раунда", func(t *testing.T) {
		table := suppDecTable(t, db, "dec-accept-post")
		appID := suppApp(t, db, td.OrgID, authorID, "DEC-OK-1", models.ConfirmationApproved, models.StatusInWork)
		attID := suppAttachment(t, db, appID, "people", "2030-01-01")

		// Исходный состав уже стоит на КПП - принятие добавки его трогать не должно.
		originalID := suppDecEmployee(t, db, attID, table, "Исходный", nil, 1)
		// И прошлый принятый раунд тоже: его строки активны с момента своего принятия.
		prevSup := models.ApplicationSupplement{
			ApplicationID: appID, Number: 1, Status: models.SupplementAccepted, CreatedByUserID: authorID,
		}
		require.NoError(t, db.Create(&prevSup).Error)
		prevID := suppDecEmployee(t, db, attID, table, "Прошлый", &prevSup.ID, 1)

		// А отклонённый раунд лежит со status = 0 навсегда: принятие СЛЕДУЮЩЕГО раунда не
		// должно поднять заодно и то, что круг однажды забраковал.
		refusedSup := models.ApplicationSupplement{
			ApplicationID: appID, Number: 2, Status: models.SupplementRefused, CreatedByUserID: authorID,
		}
		require.NoError(t, db.Create(&refusedSup).Error)
		refusedID := suppDecEmployee(t, db, attID, table, "Отклонённый", &refusedSup.ID, 0)

		round := models.ApplicationSupplement{
			ApplicationID: appID, Number: 3, Status: models.SupplementApproved, CreatedByUserID: authorID,
		}
		require.NoError(t, db.Create(&round).Error)
		newEmpID := suppDecEmployee(t, db, attID, table, "Добавленный", &round.ID, 0)
		newCarID := suppDecCar(t, db, attID, "X777XX177", &round.ID, 0)

		rec := decide(acceptorToken, appID, round.ID, `{"action":"accept","comment":"состав проверен"}`)
		require.Equal(t, http.StatusOK, rec.Code, "принятие раунда: %s", rec.Body.String())

		resp := testutil.ParseResponse[services.SupplementDecisionResponse](t, rec)
		assert.Equal(t, models.SupplementAccepted, resp.Status)
		assert.Equal(t, round.ID, resp.SupplementID)
		assert.Equal(t, 2, resp.Activated, "на КПП встали сотрудник и машина раунда")

		var saved models.ApplicationSupplement
		require.NoError(t, db.First(&saved, round.ID).Error)
		assert.Equal(t, models.SupplementAccepted, saved.Status)
		require.NotNil(t, saved.DecidedByUserID)
		assert.Equal(t, acceptorID, *saved.DecidedByUserID, "решение подписано принимающим")
		assert.NotNil(t, saved.DecidedAt)
		require.NotNil(t, saved.DecisionComment)
		assert.Equal(t, "состав проверен", *saved.DecisionComment)

		assert.Equal(t, 1, suppDecEmployeeStatus(t, db, newEmpID), "сотрудник раунда встал на КПП")
		assert.Equal(t, 1, suppDecCarStatus(t, db, newCarID), "машина раунда встала на КПП")
		assert.Equal(t, 1, suppDecEmployeeStatus(t, db, originalID), "исходный состав остался активным")
		assert.Equal(t, 1, suppDecEmployeeStatus(t, db, prevID), "состав прошлого раунда остался активным")
		assert.Equal(t, 0, suppDecEmployeeStatus(t, db, refusedID),
			"строку отклонённого раунда принятие соседнего не поднимает")
		assert.EqualValues(t, 0, suppDecAddedToTableCount(t, db, models.AuditEntityEmployee, refusedID),
			"отклонённой строке попадание в таблицу не пишется")

		// Главное следствие скоупа по supplement_id: у уже стоявших строк не появилось
		// второй записи о попадании в таблицу - события, которого не было.
		assert.EqualValues(t, 1, suppDecAddedToTableCount(t, db, models.AuditEntityEmployee, newEmpID),
			"попадание в таблицу записано ровно один раз")
		assert.EqualValues(t, 0, suppDecAddedToTableCount(t, db, models.AuditEntityEmployee, originalID),
			"исходной строке повторное попадание в таблицу не пишется")
		assert.EqualValues(t, 0, suppDecAddedToTableCount(t, db, models.AuditEntityEmployee, prevID),
			"строке прошлого раунда повторное попадание в таблицу не пишется")

		var audits int64
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?",
				models.AuditEntityApplication, appID, models.AuditActionSupplementAccepted).
			Count(&audits).Error)
		assert.EqualValues(t, 1, audits, "принятие записано в историю заявки")

		var notifications int64
		require.NoError(t, db.Model(&models.Notification{}).
			Where("user_id = ? AND type = ?", authorID, "application_supplement_decided").
			Count(&notifications).Error)
		assert.EqualValues(t, 1, notifications, "автор уведомлён о решении по своему дополнению")

		suppDecAssertApplicationIntact(t, db, appID)
	})

	// Главный риск среза. Строка в корзине и строка, погашенная чёрным списком, лежат с тем
	// же status = 0, что и непринятая добавка: один предикат статуса поднял бы обе обратно
	// на проходную - удалённого человека и того, кому вход запрещён.
	t.Run("принятие не воскрешает корзину и чёрный список", func(t *testing.T) {
		table := suppDecTable(t, db, "dec-trash-post")
		appID := suppApp(t, db, td.OrgID, authorID, "DEC-TRASH-1", models.ConfirmationApproved, models.StatusInWork)
		attID := suppAttachment(t, db, appID, "people", "2030-01-01")

		round := models.ApplicationSupplement{
			ApplicationID: appID, Number: 1, Status: models.SupplementApproved, CreatedByUserID: authorID,
		}
		require.NoError(t, db.Create(&round).Error)

		liveID := suppDecEmployee(t, db, attID, table, "Живой", &round.ID, 0)

		// Корзина: soft-delete помечает date_deleted, статус при этом тот же 0.
		trashedID := suppDecEmployee(t, db, attID, table, "Удалённый", &round.ID, 0)
		require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", trashedID).
			Update("date_deleted", time.Now().UTC()).Error)

		// Окончательно удалённый из корзины.
		purgedID := suppDecEmployee(t, db, attID, table, "Вычищенный", &round.ID, 0)
		require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", purgedID).
			Updates(map[string]any{"date_deleted": time.Now().UTC(), "is_purged": true}).Error)

		// Чёрный список гасит той же меткой, что и корзина (deactivateMatchingEmployees /
		// deactivateMatchingCars): status = 0 плюс date_deleted/date_removed.
		blacklistedID := suppDecEmployee(t, db, attID, table, "Запрещённый", &round.ID, 0)
		require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", blacklistedID).
			Update("date_deleted", time.Now().UTC()).Error)
		blCarID := suppDecCar(t, db, attID, "B666BB177", &round.ID, 0)
		require.NoError(t, db.Model(&models.Car{}).Where("id = ?", blCarID).
			Update("date_removed", time.Now().UTC()).Error)

		rec := decide(acceptorToken, appID, round.ID, `{"action":"accept"}`)
		require.Equal(t, http.StatusOK, rec.Code, "принятие раунда: %s", rec.Body.String())

		resp := testutil.ParseResponse[services.SupplementDecisionResponse](t, rec)
		assert.Equal(t, 1, resp.Activated, "на КПП встала только живая строка раунда")

		assert.Equal(t, 1, suppDecEmployeeStatus(t, db, liveID), "живая строка раунда активирована")
		assert.Equal(t, 0, suppDecEmployeeStatus(t, db, trashedID), "строка из корзины не воскресает принятием")
		assert.Equal(t, 0, suppDecEmployeeStatus(t, db, purgedID), "вычищенная строка не воскресает принятием")
		assert.Equal(t, 0, suppDecEmployeeStatus(t, db, blacklistedID), "погашенный чёрным списком не воскресает")
		assert.Equal(t, 0, suppDecCarStatus(t, db, blCarID), "машина из чёрного списка не воскресает")

		// И истории попадания на проходную у невоскрешённых тоже нет: она пишется ровно тем,
		// кто реально перешёл в активный статус.
		assert.EqualValues(t, 0, suppDecAddedToTableCount(t, db, models.AuditEntityEmployee, trashedID))
		assert.EqualValues(t, 0, suppDecAddedToTableCount(t, db, models.AuditEntityEmployee, blacklistedID))

		suppDecAssertApplicationIntact(t, db, appID)
	})

	t.Run("несогласованный раунд решения не принимает", func(t *testing.T) {
		appID := suppApp(t, db, td.OrgID, authorID, "DEC-409-1", models.ConfirmationApproved, models.StatusInWork)
		attID := suppAttachment(t, db, appID, "people", "2030-01-01")
		table := suppDecTable(t, db, "dec-409-post")

		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		empID := suppDecEmployee(t, db, attID, table, "Ожидающий", &roundID, 0)

		rec := decide(acceptorToken, appID, roundID, `{"action":"accept"}`)
		assert.Equal(t, http.StatusConflict, rec.Code, "раунд, не прошедший круг согласования, принять нельзя")

		assert.Equal(t, models.SupplementPending, suppVoteReadRoundByID(t, db, roundID).Status,
			"отбитое решение статус раунда не сдвинуло")
		assert.Equal(t, 0, suppDecEmployeeStatus(t, db, empID), "строки несогласованного раунда на КПП не попали")
		suppDecAssertApplicationIntact(t, db, appID)
	})

	t.Run("решение принимает только принимающий", func(t *testing.T) {
		_, strangerToken := suppVoteUser(t, e, db, "dec_stranger", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "DEC-403-1", models.ConfirmationApproved, models.StatusInWork)
		attID := suppAttachment(t, db, appID, "people", "2030-01-01")
		table := suppDecTable(t, db, "dec-403-post")

		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementApproved)
		empID := suppDecEmployee(t, db, attID, table, "Спорный", &roundID, 0)

		rec := decide(strangerToken, appID, roundID, `{"action":"accept"}`)
		assert.Equal(t, http.StatusForbidden, rec.Code, "посторонний решение по раунду не принимает")

		// Автор заявки принимающим тоже не становится: подать добавку и самому её пропустить
		// на КПП - обход круга согласования.
		rec = decide(authorToken, appID, roundID, `{"action":"accept"}`)
		assert.Equal(t, http.StatusForbidden, rec.Code, "автор заявки сам своё дополнение не принимает")

		assert.Equal(t, models.SupplementApproved, suppVoteReadRoundByID(t, db, roundID).Status)
		assert.Equal(t, 0, suppDecEmployeeStatus(t, db, empID), "строки раунда на КПП не попали")
		suppDecAssertApplicationIntact(t, db, appID)
	})

	t.Run("отказ оставляет строки раунда неактивными", func(t *testing.T) {
		table := suppDecTable(t, db, "dec-reject-post")
		appID := suppApp(t, db, td.OrgID, authorID, "DEC-REJ-1", models.ConfirmationApproved, models.StatusInWork)
		attID := suppAttachment(t, db, appID, "people", "2030-01-01")

		originalID := suppDecEmployee(t, db, attID, table, "Исходный", nil, 1)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementApproved)
		empID := suppDecEmployee(t, db, attID, table, "Отклонённый", &roundID, 0)

		rec := decide(acceptorToken, appID, roundID, `{"action":"reject","comment":"состав не подтверждён"}`)
		require.Equal(t, http.StatusOK, rec.Code, "отказ по раунду: %s", rec.Body.String())

		resp := testutil.ParseResponse[services.SupplementDecisionResponse](t, rec)
		assert.Equal(t, models.SupplementRefused, resp.Status)
		assert.Equal(t, 0, resp.Activated, "отказ никого на КПП не выпускает")

		var saved models.ApplicationSupplement
		require.NoError(t, db.First(&saved, roundID).Error)
		assert.Equal(t, models.SupplementRefused, saved.Status)
		require.NotNil(t, saved.DecidedByUserID)
		assert.Equal(t, acceptorID, *saved.DecidedByUserID)

		assert.Equal(t, 0, suppDecEmployeeStatus(t, db, empID), "отклонённая строка на КПП не попадает")
		assert.EqualValues(t, 0, suppDecAddedToTableCount(t, db, models.AuditEntityEmployee, empID))
		assert.Equal(t, 1, suppDecEmployeeStatus(t, db, originalID), "допущенный состав отказом не задет")

		var audits int64
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?",
				models.AuditEntityApplication, appID, models.AuditActionSupplementRefused).
			Count(&audits).Error)
		assert.EqualValues(t, 1, audits, "отказ записан в историю заявки")

		suppDecAssertApplicationIntact(t, db, appID)
	})

	t.Run("снять раунд может только автор заявки", func(t *testing.T) {
		table := suppDecTable(t, db, "dec-cancel-post")
		_, strangerToken := suppVoteUser(t, e, db, "dec_cancel_stranger", td.OrgID, td.CompanyID)

		appID := suppApp(t, db, td.OrgID, authorID, "DEC-CANCEL-1", models.ConfirmationApproved, models.StatusInWork)
		attID := suppAttachment(t, db, appID, "people", "2030-01-01")

		originalID := suppDecEmployee(t, db, attID, table, "Исходный", nil, 1)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementPending)
		empID := suppDecEmployee(t, db, attID, table, "Снимаемый", &roundID, 0)

		rec := cancel(strangerToken, appID, roundID, `{}`)
		assert.Equal(t, http.StatusForbidden, rec.Code, "чужой раунд посторонний не снимает")
		assert.Equal(t, models.SupplementPending, suppVoteReadRoundByID(t, db, roundID).Status)

		// Принимающий - тоже не автор: снятие это воля подавшего, а не решение по раунду.
		rec = cancel(acceptorToken, appID, roundID, `{}`)
		assert.Equal(t, http.StatusForbidden, rec.Code, "принимающий раунд не снимает - у него отказ")

		rec = cancel(authorToken, appID, roundID, `{"comment":"передумал"}`)
		require.Equal(t, http.StatusOK, rec.Code, "снятие раунда автором: %s", rec.Body.String())

		resp := testutil.ParseResponse[services.SupplementDecisionResponse](t, rec)
		assert.Equal(t, models.SupplementCancelled, resp.Status)

		var saved models.ApplicationSupplement
		require.NoError(t, db.First(&saved, roundID).Error)
		assert.Equal(t, models.SupplementCancelled, saved.Status)
		require.NotNil(t, saved.DecidedByUserID)
		assert.Equal(t, authorID, *saved.DecidedByUserID, "снятие подписано автором")
		require.NotNil(t, saved.DecisionComment)
		assert.Equal(t, "передумал", *saved.DecisionComment)

		assert.Equal(t, 0, suppDecEmployeeStatus(t, db, empID), "снятая строка на КПП не попадает")
		assert.Equal(t, 1, suppDecEmployeeStatus(t, db, originalID), "допущенный состав снятием не задет")

		var audits int64
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?",
				models.AuditEntityApplication, appID, models.AuditActionSupplementCancelledByAuthor).
			Count(&audits).Error)
		assert.EqualValues(t, 1, audits, "снятие записано в историю заявки")

		// Снятый раунд закрыт - повторное снятие отбивается.
		rec = cancel(authorToken, appID, roundID, `{}`)
		assert.Equal(t, http.StatusConflict, rec.Code, "закрытый раунд снять уже нельзя")

		suppDecAssertApplicationIntact(t, db, appID)
	})

	// Заявку могли вывести из работы, пока раунд шёл круг: её строки при этом погашены, и
	// состав раунда встал бы на КПП поверх снятых пропусков.
	t.Run("принятие у заявки не в работе отбивается", func(t *testing.T) {
		table := suppDecTable(t, db, "dec-notwork-post")
		appID := suppApp(t, db, td.OrgID, authorID, "DEC-NOTWORK-1", models.ConfirmationApproved, models.StatusProcessing)
		attID := suppAttachment(t, db, appID, "people", "2030-01-01")

		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementApproved)
		empID := suppDecEmployee(t, db, attID, table, "Преждевременный", &roundID, 0)

		rec := decide(acceptorToken, appID, roundID, `{"action":"accept"}`)
		assert.Equal(t, http.StatusConflict, rec.Code, "у заявки не в работе принимать дополнение некуда")

		assert.Equal(t, models.SupplementApproved, suppVoteReadRoundByID(t, db, roundID).Status,
			"отбитое принятие статус раунда не сдвинуло")
		assert.Equal(t, 0, suppDecEmployeeStatus(t, db, empID), "строки раунда на КПП не попали")

		app := suppReadApplication(t, db, appID)
		require.NotNil(t, app.Status)
		assert.Equal(t, models.StatusProcessing, *app.Status, "статус заявки отбитым принятием не задет")
	})

	t.Run("недопустимое решение отбивается", func(t *testing.T) {
		appID := suppApp(t, db, td.OrgID, authorID, "DEC-BAD-1", models.ConfirmationApproved, models.StatusInWork)
		roundID := suppNewSupplement(t, db, appID, authorID, models.SupplementApproved)

		rec := decide(acceptorToken, appID, roundID, `{"action":"maybe"}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "решение бывает только accept или reject")
		assert.Equal(t, models.SupplementApproved, suppVoteReadRoundByID(t, db, roundID).Status)
		suppDecAssertApplicationIntact(t, db, appID)
	})
}

// Гонка решения принимающего с закрытием заявки: принятие раунда и вывод заявки из работы
// стартуют одновременно. Проверяем не «кто победил» (это выбор планировщика), а что исход
// согласован: раунд не может остаться cancelled со строками, стоящими на КПП, - именно
// такое расхождение прячет строки от всех читателей состава, оставляя их физически на посту.
func TestSupplementDecision_RaceWithApplicationClose(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, _ := suppVoteUser(t, e, db, "race_author", td.OrgID, td.CompanyID)
	acceptorID, acceptorToken := suppVoteUser(t, e, db, "race_acceptor", td.OrgID, td.CompanyID)
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: acceptorID}).Error)

	table := suppDecTable(t, db, "race-post")
	appID := suppApp(t, db, td.OrgID, authorID, "DEC-RACE-1", models.ConfirmationApproved, models.StatusInWork)
	attID := suppAttachment(t, db, appID, "people", "2030-01-01")

	round := models.ApplicationSupplement{
		ApplicationID: appID, Number: 1, Status: models.SupplementApproved, CreatedByUserID: authorID,
	}
	require.NoError(t, db.Create(&round).Error)
	empID := suppDecEmployee(t, db, attID, table, "Гоночный", &round.ID, 0)

	svc := services.NewApplicationService(db, nil, nil, nil, nil, services.NewAuditRecorder(db))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements/%d/take-to-work", appID, round.ID),
			`{"action":"accept"}`, testutil.AuthHeader(acceptorToken))
	}()
	go func() {
		defer wg.Done()
		_ = svc.RevokeApplicationFromWork(context.Background(), "race_acceptor", appID,
			services.RevokeFromWorkRequest{Comment: nil})
	}()
	wg.Wait()

	var final models.ApplicationSupplement
	require.NoError(t, db.First(&final, round.ID).Error)
	status := suppDecEmployeeStatus(t, db, empID)

	if final.Status == models.SupplementAccepted {
		// Принятие успело раньше - вывод из работы обязан был погасить строку следом,
		// раз он гасит весь состав заявки.
		require.Equal(t, 0, status, "после вывода заявки из работы строка принятого раунда не может остаться на КПП")
	} else {
		require.Equal(t, models.SupplementCancelled, final.Status,
			"раунд закрытой заявки либо принят, либо снят - других исходов нет")
		require.Equal(t, 0, status, "строка снятого раунда на КПП не попадает")
	}
}
