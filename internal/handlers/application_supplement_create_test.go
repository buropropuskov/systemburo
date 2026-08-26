package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Создание дополнения заявки (#1685, срез S2). Главное, что здесь стережётся: у заявки
// В РАБОТЕ дополнение НЕ откатывает confirmation и status. От этой пары производен допуск
// строки на КПП, и откат снял бы с проходной всех, кого уже пустили - поэтому повторный
// круг живёт отдельным раундом, а не сбросом голосов основного.

// suppAuthor - пользователь с правом дополнять заявки. Право выдаём личным override ДО
// логина: резолвер кэширует набор при первом защищённом запросе.
func suppAuthor(t *testing.T, e *echo.Echo, db *gorm.DB, username string, orgID, companyID int) (int, string) {
	t.Helper()
	testutil.RegisterUser(t, e, username, "pass123", 1, orgID, companyID)

	var user models.User
	require.NoError(t, db.Where("username = ?", username).First(&user).Error)
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        user.ID,
		PermissionKey: services.KeyActionSupplementApplication,
		Value:         "allow",
	}).Error)

	token, _ := testutil.LoginUser(t, e, username, "pass123")
	return user.ID, token
}

func suppApp(t *testing.T, db *gorm.DB, orgID, senderID int, number, confirmation, status string) int {
	t.Helper()
	num, conf, st := number, confirmation, status
	app := models.Application{
		ApplicationNumber: &num,
		Confirmation:      &conf,
		Status:            &st,
		OrganizationID:    orgID,
		SenderUserID:      senderID,
	}
	require.NoError(t, db.Create(&app).Error)
	return app.ID
}

// suppAttachment - активное вложение заявки с длинным сроком действия.
func suppAttachment(t *testing.T, db *gorm.DB, appID int, atype, dateTo string) int {
	t.Helper()
	st := 1
	from, to := "2026-01-01", dateTo
	timeFrom, timeTo := "08:00:00", "20:00:00"
	att := models.Attachment{
		ApplicationID:  &appID,
		AttachmentType: atype,
		EntryDateFrom:  &from,
		EntryDateTo:    &to,
		EntryTimeFrom:  &timeFrom,
		EntryTimeTo:    &timeTo,
		Status:         &st,
	}
	require.NoError(t, db.Create(&att).Error)
	return att.ID
}

// suppResponsible - согласующий заявки с уже отданным голосом: ветка А обязана его сбросить,
// ветка Б - оставить нетронутым.
func suppResponsible(t *testing.T, db *gorm.DB, appID, userID int, required bool, status string) {
	t.Helper()
	st := status
	comment := "согласовано ранее"
	when := testutil.Ptr(time.Now().UTC())
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
		ApplicationID:    appID,
		UserID:           userID,
		RequiredApproval: required,
		ApprovalStatus:   &st,
		ApprovalComment:  &comment,
		ApprovalDatetime: when,
	}).Error)
}

func suppReadApplication(t *testing.T, db *gorm.DB, appID int) models.Application {
	t.Helper()
	var app models.Application
	require.NoError(t, db.First(&app, appID).Error)
	return app
}

func suppReadRound(t *testing.T, db *gorm.DB, appID int) models.ApplicationSupplement {
	t.Helper()
	var round models.ApplicationSupplement
	require.NoError(t, db.Where("application_id = ?", appID).Order("number DESC").First(&round).Error)
	return round
}

// Ветка А: заявка ещё не в работе - её сущности не активированы, на КПП терять нечего,
// поэтому добавка вливается в текущий круг согласования (раунд merged), голоса сбрасываются
// и confirmation пересчитывается.
func TestSupplementCreate_NotInWork_MergesIntoCurrentRound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, token := suppAuthor(t, e, db, "supp_merge_author", td.OrgID, td.CompanyID)
	approverID, _ := suppAuthor(t, e, db, "supp_merge_approver", td.OrgID, td.CompanyID)

	appID := suppApp(t, db, td.OrgID, authorID, "SUPP-MERGE-1", models.ConfirmationApproved, models.StatusProcessing)
	suppResponsible(t, db, appID, approverID, true, "approved")
	attID := suppAttachment(t, db, appID, "cars", "2099-12-31")
	tableID := seedCarsTable(t, db, "supp_merge_pass", "Проезд дополнения")
	placeID := seedPlace(t, db, "Склад дополнения")

	body := fmt.Sprintf(`{
		"comment": "  Добавляем монтажников  ",
		"additions": [{
			"attachment_id": %d,
			"vehicles": [{"car_number":"X001XX777","car_brand":"КамАЗ","unload_places":[%d],"passage_tables":[%d]}]
		}]
	}`, attID, placeID, tableID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "создание дополнения: %s", rec.Body.String())

	resp := testutil.ParseResponse[services.CreateSupplementResponse](t, rec)
	assert.Equal(t, models.SupplementMerged, resp.Status, "заявка не в работе - добавка вливается в текущий круг")
	assert.Equal(t, 1, resp.Number)
	assert.Equal(t, 1, resp.Counts.Vehicles)

	round := suppReadRound(t, db, appID)
	assert.Equal(t, models.SupplementMerged, round.Status)
	assert.Equal(t, authorID, round.CreatedByUserID)
	require.NotNil(t, round.Comment)
	assert.Equal(t, "Добавляем монтажников", *round.Comment, "комментарий сохраняется без окружающих пробелов")

	// Голос согласующего сброшен: он согласовывал другой состав.
	var responsible models.ApplicationResponsibleUser
	require.NoError(t, db.Where("application_id = ? AND user_id = ?", appID, approverID).First(&responsible).Error)
	require.NotNil(t, responsible.ApprovalStatus)
	assert.Equal(t, "pending", *responsible.ApprovalStatus)
	assert.Nil(t, responsible.ApprovalComment, "комментарий прежнего голоса снят")
	assert.Nil(t, responsible.ApprovalDatetime, "дата прежнего голоса снята")

	// Пересчёт confirmation: обязательный согласующий снова pending -> заявка на согласовании.
	app := suppReadApplication(t, db, appID)
	require.NotNil(t, app.Confirmation)
	assert.Equal(t, models.ConfirmationPending, *app.Confirmation)

	// Отдельный раунд согласования не заводится - голосовать будут в основном круге.
	var votes int64
	require.NoError(t, db.Model(&models.ApplicationSupplementApproval{}).Where("supplement_id = ?", round.ID).Count(&votes).Error)
	assert.EqualValues(t, 0, votes, "у влитого дополнения своих голосующих нет")

	// Машина заведена неактивной, помечена раундом и унаследовала окно вложения.
	var car models.Car
	require.NoError(t, db.Where("attachment_id = ?", attID).First(&car).Error)
	require.NotNil(t, car.Status)
	assert.Equal(t, 0, *car.Status, "строки дополнения неактивны до принятия")
	require.NotNil(t, car.SupplementID)
	assert.Equal(t, round.ID, *car.SupplementID)
	require.NotNil(t, car.EntryDateTo)
	assert.Equal(t, "2099-12-31", *car.EntryDateTo, "даты наследуются от вложения")
	require.NotNil(t, car.EntryTimeFrom)
	assert.Equal(t, "08:00:00", *car.EntryTimeFrom)

	// Привязки прохода и разгрузки записаны сразу, как при подаче.
	var links int64
	require.NoError(t, db.Table("car_target_tables").Where("car_id = ? AND table_id = ?", car.ID, tableID).Count(&links).Error)
	assert.EqualValues(t, 1, links, "машина привязана к таблице проезда")
	require.NoError(t, db.Table("car_unload_places").Where("car_id = ? AND unload_place_id = ?", car.ID, placeID).Count(&links).Error)
	assert.EqualValues(t, 1, links, "место разгрузки машины записано")
	require.NoError(t, db.Table("attachment_unload_places").Where("attachment_id = ? AND unload_place_id = ?", attID, placeID).Count(&links).Error)
	assert.EqualValues(t, 1, links, "место попало в union вложения - иначе охрана его не увидит")

	// Событие видно в ленте истории заявки.
	var audits int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, "supplement_created").
		Count(&audits).Error)
	assert.EqualValues(t, 1, audits, "создание дополнения записано в историю заявки")
}

// Ветка Б и ядро требования: у заявки В РАБОТЕ дополнение заводит отдельный раунд со своим
// снимком голосующих, а confirmation и status заявки остаются нетронутыми - иначе с КПП
// слетели бы уже допущенные люди и машины.
func TestSupplementCreate_InWork_KeepsApplicationVerdictIntact(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, token := suppAuthor(t, e, db, "supp_work_author", td.OrgID, td.CompanyID)
	requiredID, _ := suppAuthor(t, e, db, "supp_work_required", td.OrgID, td.CompanyID)
	optionalID, _ := suppAuthor(t, e, db, "supp_work_optional", td.OrgID, td.CompanyID)

	appID := suppApp(t, db, td.OrgID, authorID, "SUPP-WORK-1", models.ConfirmationApproved, models.StatusInWork)
	suppResponsible(t, db, appID, requiredID, true, "approved")
	suppResponsible(t, db, appID, optionalID, false, "approved")
	attID := suppAttachment(t, db, appID, "people", "2099-12-31")
	tableID := seedCarsTable(t, db, "supp_work_pass", "Проход дополнения")
	citizenship := models.Citizenship{Name: "Российская Федерация", IsActive: true}
	require.NoError(t, db.Create(&citizenship).Error)

	body := fmt.Sprintf(`{
		"additions": [{
			"attachment_id": %d,
			"employees": [{"last_name":"Иванов","first_name":"Пётр","citizenship_id":%d,"position":"монтажник","passport_series_number":"1234 567890","target_tables":[%d]}]
		}]
	}`, attID, citizenship.ID, tableID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "создание дополнения: %s", rec.Body.String())

	resp := testutil.ParseResponse[services.CreateSupplementResponse](t, rec)
	assert.Equal(t, models.SupplementPending, resp.Status, "заявка в работе - добавка идёт отдельным раундом")
	assert.Equal(t, 1, resp.Counts.Employees)

	// Вердикт заявки не сдвинулся ни на йоту - на этом держится допуск на КПП.
	app := suppReadApplication(t, db, appID)
	require.NotNil(t, app.Confirmation)
	require.NotNil(t, app.Status)
	assert.Equal(t, models.ConfirmationApproved, *app.Confirmation, "согласование заявки не откатывается")
	assert.Equal(t, models.StatusInWork, *app.Status, "статус заявки не откатывается")

	// Голоса основного круга тоже нетронуты.
	var responsible models.ApplicationResponsibleUser
	require.NoError(t, db.Where("application_id = ? AND user_id = ?", appID, requiredID).First(&responsible).Error)
	require.NotNil(t, responsible.ApprovalStatus)
	assert.Equal(t, "approved", *responsible.ApprovalStatus, "голос основного круга не сбрасывается")
	assert.NotNil(t, responsible.ApprovalDatetime)

	// Снимок голосующих раунда: те же пользователи и та же обязательность, но голоса пустые.
	round := suppReadRound(t, db, appID)
	assert.Equal(t, models.SupplementPending, round.Status)
	var votes []models.ApplicationSupplementApproval
	require.NoError(t, db.Where("supplement_id = ?", round.ID).Order("user_id").Find(&votes).Error)
	require.Len(t, votes, 2, "в снимок попали оба ответственных заявки")
	byUser := map[int]models.ApplicationSupplementApproval{}
	for _, v := range votes {
		byUser[v.UserID] = v
		require.NotNil(t, v.ApprovalStatus)
		assert.Equal(t, "pending", *v.ApprovalStatus, "голоса раунда стартуют пустыми")
	}
	assert.True(t, byUser[requiredID].RequiredApproval, "обязательность перенесена в снимок")
	assert.False(t, byUser[optionalID].RequiredApproval)

	// Сотрудник заведён неактивным с меткой раунда и привязкой к посту.
	var emp models.Employee
	require.NoError(t, db.Where("attachment_id = ?", attID).First(&emp).Error)
	require.NotNil(t, emp.Status)
	assert.Equal(t, 0, *emp.Status, "сотрудник дополнения неактивен до принятия раунда")
	require.NotNil(t, emp.SupplementID)
	assert.Equal(t, round.ID, *emp.SupplementID)
	var links int64
	require.NoError(t, db.Table("employee_target_tables").Where("employee_id = ? AND table_id = ?", emp.ID, tableID).Count(&links).Error)
	assert.EqualValues(t, 1, links, "сотрудник привязан к месту прохода")
}

// Гарды создания: кто, когда и во что вправе добавлять строки.
func TestSupplementCreate_Guards(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, token := suppAuthor(t, e, db, "supp_guard_author", td.OrgID, td.CompanyID)
	// Посторонний с тем же правом: 403 должен приходить от проверки владения, а не от гейта прав.
	strangerID, strangerToken := suppAuthor(t, e, db, "supp_guard_stranger", td.OrgID, td.CompanyID)

	appID := suppApp(t, db, td.OrgID, authorID, "SUPP-GUARD-1", models.ConfirmationApproved, models.StatusInWork)
	attID := suppAttachment(t, db, appID, "cars", "2099-12-31")

	carBody := func(attachmentID int, number string) string {
		return fmt.Sprintf(`{"additions":[{"attachment_id":%d,"vehicles":[{"car_number":"%s","car_brand":"КамАЗ"}]}]}`, attachmentID, number)
	}
	post := func(token string, targetApp int, body string) *httptest.ResponseRecorder {
		return testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", targetApp), body, testutil.AuthHeader(token))
	}

	t.Run("пустой additions - 400", func(t *testing.T) {
		rec := post(token, appID, `{"additions":[]}`)
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("вложение без строк - 400", func(t *testing.T) {
		rec := post(token, appID, fmt.Sprintf(`{"additions":[{"attachment_id":%d}]}`, attID))
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("содержимое не того типа - 400", func(t *testing.T) {
		body := fmt.Sprintf(`{"additions":[{"attachment_id":%d,"employees":[{"last_name":"Сидоров","first_name":"Иван"}]}]}`, attID)
		rec := post(token, appID, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("вложение чужой заявки - 400", func(t *testing.T) {
		otherAppID := suppApp(t, db, td.OrgID, strangerID, "SUPP-GUARD-OTHER", models.ConfirmationApproved, models.StatusInWork)
		otherAttID := suppAttachment(t, db, otherAppID, "cars", "2099-12-31")

		rec := post(token, appID, carBody(otherAttID, "Y002YY777"))
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("истёкшее вложение - 400", func(t *testing.T) {
		expiredAppID := suppApp(t, db, td.OrgID, authorID, "SUPP-GUARD-EXPIRED", models.ConfirmationApproved, models.StatusInWork)
		expiredAttID := suppAttachment(t, db, expiredAppID, "cars", "2020-01-31")

		rec := post(token, expiredAppID, carBody(expiredAttID, "Z003ZZ777"))
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("посторонний - 403", func(t *testing.T) {
		rec := post(strangerToken, appID, carBody(attID, "A004AA777"))
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("завершённая заявка - 409", func(t *testing.T) {
		doneAppID := suppApp(t, db, td.OrgID, authorID, "SUPP-GUARD-DONE", models.ConfirmationApproved, models.StatusCompleted)
		doneAttID := suppAttachment(t, db, doneAppID, "cars", "2099-12-31")

		rec := post(token, doneAppID, carBody(doneAttID, "B005BB777"))
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	})

	t.Run("отозванная заявка - 409", func(t *testing.T) {
		withdrawnAppID := suppApp(t, db, td.OrgID, authorID, "SUPP-GUARD-WITHDRAWN", models.ConfirmationApproved, models.StatusWithdrawn)
		withdrawnAttID := suppAttachment(t, db, withdrawnAppID, "cars", "2099-12-31")

		rec := post(token, withdrawnAppID, carBody(withdrawnAttID, "C006CC777"))
		assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	})

	t.Run("второе незакрытое дополнение - 409", func(t *testing.T) {
		rec := post(token, appID, carBody(attID, "D007DD777"))
		require.Equal(t, http.StatusOK, rec.Code, "первый раунд: %s", rec.Body.String())

		rec = post(token, appID, carBody(attID, "E008EE777"))
		assert.Equal(t, http.StatusConflict, rec.Code, "второй открытый раунд на заявке недопустим: %s", rec.Body.String())

		// Отказ второго раунда не оставил после себя строк.
		var cars int64
		require.NoError(t, db.Model(&models.Car{}).Where("attachment_id = ? AND car_number = ?", attID, "E008EE777").Count(&cars).Error)
		assert.EqualValues(t, 0, cars, "отклонённое дополнение не должно оставлять машину")
	})
}

// Чтение раундов: доступно тому, кому видна заявка, и закрыто постороннему.
func TestSupplementList_VisibleToParticipantsOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, token := suppAuthor(t, e, db, "supp_list_author", td.OrgID, td.CompanyID)
	approverID, approverToken := suppAuthor(t, e, db, "supp_list_approver", td.OrgID, td.CompanyID)
	_, strangerToken := suppAuthor(t, e, db, "supp_list_stranger", td.OrgID, td.CompanyID)

	appID := suppApp(t, db, td.OrgID, authorID, "SUPP-LIST-1", models.ConfirmationApproved, models.StatusInWork)
	suppResponsible(t, db, appID, approverID, true, "approved")
	attID := suppAttachment(t, db, appID, "items", "2099-12-31")

	body := fmt.Sprintf(`{"comment":"добавили инструмент","additions":[{"attachment_id":%d,"items":[{"name":"Перфоратор","count":2,"order_index":1}]}]}`, attID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/supplements", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "создание дополнения: %s", rec.Body.String())

	// ТМЦ помечены раундом: у них нет status, и провенанс - единственное, чем их отличают.
	round := suppReadRound(t, db, appID)
	var item models.Item
	require.NoError(t, db.Where("attachment_id = ?", attID).First(&item).Error)
	require.NotNil(t, item.SupplementID)
	assert.Equal(t, round.ID, *item.SupplementID)

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/supplements", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rounds := testutil.ParseResponse[[]services.SupplementInfo](t, rec)
	require.Len(t, rounds, 1)
	assert.Equal(t, models.SupplementPending, rounds[0].Status)
	assert.Equal(t, 1, rounds[0].Counts.Items)
	require.NotNil(t, rounds[0].Comment)
	assert.Equal(t, "добавили инструмент", *rounds[0].Comment)
	require.Len(t, rounds[0].Approvals, 1, "голосующие раунда отдаются вместе с ним")
	assert.Equal(t, approverID, rounds[0].Approvals[0].UserID)

	// Согласующий заявки видит раунд - ему по нему голосовать.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/supplements", appID), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Len(t, testutil.ParseResponse[[]services.SupplementInfo](t, rec), 1)

	// Посторонний не видит ни раундов, ни самого факта их наличия.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/supplements", appID), testutil.AuthHeader(strangerToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}
