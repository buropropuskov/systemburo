package handlers_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Тесты привязки ручного вложения-сироты к заявке (#1049 S7, режим-2). Два пути: усыновление
// (application_id) и перевешивание на существующее вложение заявки (target_attachment_id).
// Проверяем оба через реальный HTTP-эндпоинт (SQL исполняется), валидацию времени и статуса
// заявки, гейт super/admin.

// seedAttachSender создаёт пользователя-отправителя для заявки (FK sender_user_id).
func seedAttachSender(t *testing.T, db *gorm.DB, orgID int) int {
	t.Helper()
	u := models.User{Username: "attach_sender", Password: "x", TypeID: 1, OrganizationID: &orgID}
	require.NoError(t, db.Create(&u).Error)
	return u.ID
}

// seedActiveApp создаёт заявку с указанными confirmation/status.
func seedActiveApp(t *testing.T, db *gorm.DB, orgID, senderID int, confirmation, status string) int {
	t.Helper()
	num := "APP-ATTACH-1"
	app := models.Application{
		ApplicationNumber: &num,
		Confirmation:      &confirmation,
		Status:            &status,
		OrganizationID:    orgID,
		SenderUserID:      senderID,
	}
	require.NoError(t, db.Create(&app).Error)
	return app.ID
}

// seedAppAttachment создаёт вложение, принадлежащее заявке (не ручное).
func seedAppAttachment(t *testing.T, db *gorm.DB, appID int, atype, from, to string) int {
	t.Helper()
	st := 1
	att := models.Attachment{
		ApplicationID:  &appID,
		AttachmentType: atype,
		EntryDateFrom:  &from,
		EntryDateTo:    &to,
		Status:         &st,
	}
	require.NoError(t, db.Create(&att).Error)
	return att.ID
}

// seedOrphanCar создаёт ручное cars-вложение-сироту с одной машиной. Даты вложения и машины
// задаются раздельно, чтобы проверять валидацию «сущность ⊂ вложение».
func seedOrphanCar(t *testing.T, db *gorm.DB, orgID int, attFrom, attTo, carFrom, carTo, number string, tableID int) (attID, carID int) {
	t.Helper()
	st := 1
	att := models.Attachment{
		AttachmentType: "cars",
		IsManual:       true,
		OrganizationID: &orgID,
		EntryDateFrom:  &attFrom,
		EntryDateTo:    &attTo,
		Status:         &st,
	}
	require.NoError(t, db.Create(&att).Error)
	car := models.Car{
		AttachmentID:  att.ID,
		CarNumber:     &number,
		EntryDateFrom: &carFrom,
		EntryDateTo:   &carTo,
		Status:        &st,
	}
	require.NoError(t, db.Create(&car).Error)
	if tableID > 0 {
		require.NoError(t, db.Create(&models.CarTargetTable{CarID: car.ID, TableID: tableID}).Error)
	}
	return att.ID, car.ID
}

// seedOrphanPeople создаёт ручное people-вложение-сироту с одним сотрудником.
func seedOrphanPeople(t *testing.T, db *gorm.DB, orgID int, from, to, lastName string) (attID, empID int) {
	t.Helper()
	st := 1
	att := models.Attachment{
		AttachmentType: "people",
		IsManual:       true,
		OrganizationID: &orgID,
		EntryDateFrom:  &from,
		EntryDateTo:    &to,
		Status:         &st,
	}
	require.NoError(t, db.Create(&att).Error)
	ln := lastName
	emp := models.Employee{AttachmentID: &att.ID, LastName: &ln, Status: &st}
	require.NoError(t, db.Create(&emp).Error)
	return att.ID, emp.ID
}

func seedPlace(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	p := models.UnloadPlace{Name: name, IsActive: true, Status: "active"}
	require.NoError(t, db.Create(&p).Error)
	return p.ID
}

// Усыновление cars-сироты: вложение становится частью заявки (application_id проставлен,
// org/company/is_manual обнулены), машина видна в целевой таблице уже как заявочная.
func TestAttach_AdoptCars_LinksAndBecomesApplicationScoped(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	tableID := seedCarsTable(t, db, "attach_adopt_a", "Проезд A")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID := seedActiveApp(t, db, td.OrgID, senderID, models.ConfirmationApproved, models.StatusInWork)
	orphanAtt, carID := seedOrphanCar(t, db, td.OrgID, "2026-07-01", "2099-12-31", "2026-07-01", "2099-12-31", "A100AA777", tableID)

	body := fmt.Sprintf(`{"application_id": %d}`, appID)
	rec := testutil.POST(t, e, fmt.Sprintf("/attachments/%d/attach-to-application", orphanAtt), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "adopt: %s", rec.Body.String())
	resp := testutil.ParseResponse[services.AttachToApplicationResponse](t, rec)
	assert.Equal(t, appID, resp.ApplicationID)

	// Вложение усыновлено: application_id проставлен, метки ручного сняты.
	var att models.Attachment
	require.NoError(t, db.First(&att, orphanAtt).Error)
	require.NotNil(t, att.ApplicationID)
	assert.Equal(t, appID, *att.ApplicationID)
	assert.False(t, att.IsManual, "is_manual снят")
	assert.Nil(t, att.OrganizationID, "org обнулён - берётся из заявки")
	assert.Nil(t, att.CompanyID)

	// Машина осталась в том же вложении и теперь показывается как заявочная (application_id != null).
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/active-for-table/%d", tableID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	rows := testutil.ParseSlice(t, rec)
	require.Len(t, rows, 1, "машина видна в таблице после привязки")
	assert.NotNil(t, rows[0]["application_id"], "метка «добавлено вручную» ушла - машина теперь заявочная")

	var car models.Car
	require.NoError(t, db.First(&car, carID).Error)
	assert.Equal(t, orphanAtt, car.AttachmentID, "машина осталась в том же вложении")

	// В истории машины появилась запись о привязке с осмысленным комментарием.
	rec = testutil.GET(t, e, fmt.Sprintf("/cars/%d/history", carID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	history := testutil.ParseSlice(t, rec)
	foundLink := false
	for _, h := range history {
		if c, ok := h["comment"].(string); ok && strings.Contains(c, "Привязано к заявке") {
			foundLink = true
			break
		}
	}
	assert.True(t, foundLink, "история машины содержит запись о привязке к заявке")
}

// Перевешивание cars: машины сироты переезжают в существующее вложение заявки, места
// разгрузки переносятся на целевое вложение, опустевшая сирота удаляется.
func TestAttach_ReattachCars_MovesEntitiesAndDeletesOrphan(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	tableID := seedCarsTable(t, db, "attach_reatt_a", "Проезд A")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID := seedActiveApp(t, db, td.OrgID, senderID, models.ConfirmationApproved, models.StatusInWork)
	targetAtt := seedAppAttachment(t, db, appID, "cars", "2026-07-01", "2026-07-31")

	orphanAtt, carID := seedOrphanCar(t, db, td.OrgID, "2026-07-01", "2026-07-31", "2026-07-05", "2026-07-20", "B200BB777", tableID)
	place := seedPlace(t, db, "Склад А")
	require.NoError(t, db.Create(&models.AttachmentUnloadPlace{AttachmentID: orphanAtt, UnloadPlaceID: place}).Error)

	body := fmt.Sprintf(`{"target_attachment_id": %d}`, targetAtt)
	rec := testutil.POST(t, e, fmt.Sprintf("/attachments/%d/attach-to-application", orphanAtt), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "reattach: %s", rec.Body.String())

	// Машина перевешена на целевое вложение.
	var car models.Car
	require.NoError(t, db.First(&car, carID).Error)
	assert.Equal(t, targetAtt, car.AttachmentID, "машина переехала в вложение заявки")

	// Сирота удалена.
	var cnt int64
	require.NoError(t, db.Model(&models.Attachment{}).Where("id = ?", orphanAtt).Count(&cnt).Error)
	assert.EqualValues(t, 0, cnt, "опустевшая сирота удалена")

	// Место перенесено на целевое вложение (видимость охраны не теряется).
	require.NoError(t, db.Table("attachment_unload_places").
		Where("attachment_id = ? AND unload_place_id = ?", targetAtt, place).Count(&cnt).Error)
	assert.EqualValues(t, 1, cnt, "место разгрузки перенесено на вложение заявки")
}

// Перевешивание с датами машины за окном целевого вложения - 422 (сущность ⊄ вложение).
func TestAttach_ReattachCars_TimeOutOfWindow422(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	senderID := seedAttachSender(t, db, td.OrgID)
	appID := seedActiveApp(t, db, td.OrgID, senderID, models.ConfirmationApproved, models.StatusInWork)
	targetAtt := seedAppAttachment(t, db, appID, "cars", "2026-07-01", "2026-07-10")
	// Машина действует до 2026-07-20 - выходит за окно целевого вложения (до 07-10).
	orphanAtt, _ := seedOrphanCar(t, db, td.OrgID, "2026-06-20", "2026-07-20", "2026-06-20", "2026-07-20", "C300CC777", 0)

	body := fmt.Sprintf(`{"target_attachment_id": %d}`, targetAtt)
	rec := testutil.POST(t, e, fmt.Sprintf("/attachments/%d/attach-to-application", orphanAtt), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, "период машины шире вложения -> 422")

	// Сирота не тронута (транзакция не выполнялась).
	var cnt int64
	require.NoError(t, db.Model(&models.Attachment{}).Where("id = ?", orphanAtt).Count(&cnt).Error)
	assert.EqualValues(t, 1, cnt, "при ошибке валидации сирота остаётся")
}

// Перевешивание people: сотрудник переезжает в вложение заявки, сирота удаляется. У людей
// нет своего времени - валидация «сущность ⊂ вложение» не применяется.
func TestAttach_ReattachPeople_MovesAndDeletesOrphan(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	senderID := seedAttachSender(t, db, td.OrgID)
	appID := seedActiveApp(t, db, td.OrgID, senderID, models.ConfirmationApproved, models.StatusInWork)
	targetAtt := seedAppAttachment(t, db, appID, "people", "2026-07-01", "2026-07-31")
	orphanAtt, empID := seedOrphanPeople(t, db, td.OrgID, "2026-07-01", "2026-07-31", "Иванов")

	body := fmt.Sprintf(`{"target_attachment_id": %d}`, targetAtt)
	rec := testutil.POST(t, e, fmt.Sprintf("/attachments/%d/attach-to-application", orphanAtt), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "reattach people: %s", rec.Body.String())

	var emp models.Employee
	require.NoError(t, db.First(&emp, empID).Error)
	require.NotNil(t, emp.AttachmentID)
	assert.Equal(t, targetAtt, *emp.AttachmentID, "сотрудник переехал в вложение заявки")

	var cnt int64
	require.NoError(t, db.Model(&models.Attachment{}).Where("id = ?", orphanAtt).Count(&cnt).Error)
	assert.EqualValues(t, 0, cnt, "опустевшая сирота удалена")
}

// Тип целевого вложения не совпадает с типом сироты - 422.
func TestAttach_TargetTypeMismatch422(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	senderID := seedAttachSender(t, db, td.OrgID)
	appID := seedActiveApp(t, db, td.OrgID, senderID, models.ConfirmationApproved, models.StatusInWork)
	peopleAtt := seedAppAttachment(t, db, appID, "people", "2026-07-01", "2026-07-31")
	orphanAtt, _ := seedOrphanCar(t, db, td.OrgID, "2026-07-01", "2026-07-31", "2026-07-01", "2026-07-31", "D400DD777", 0)

	body := fmt.Sprintf(`{"target_attachment_id": %d}`, peopleAtt)
	rec := testutil.POST(t, e, fmt.Sprintf("/attachments/%d/attach-to-application", orphanAtt), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, "cars-сироту нельзя перевесить на people-вложение")
}

// Привязка к несогласованной/неактивной заявке - 422 (иначе запись пропала бы из таблиц).
func TestAttach_InactiveApp422(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	senderID := seedAttachSender(t, db, td.OrgID)
	// Заявка на согласовании (не активна) - привязка запрещена.
	appID := seedActiveApp(t, db, td.OrgID, senderID, models.ConfirmationPending, models.StatusApproval)
	orphanAtt, _ := seedOrphanCar(t, db, td.OrgID, "2026-07-01", "2026-07-31", "2026-07-01", "2026-07-31", "E500EE777", 0)

	body := fmt.Sprintf(`{"application_id": %d}`, appID)
	rec := testutil.POST(t, e, fmt.Sprintf("/attachments/%d/attach-to-application", orphanAtt), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, "привязка к несогласованной заявке -> 422")

	var att models.Attachment
	require.NoError(t, db.First(&att, orphanAtt).Error)
	assert.Nil(t, att.ApplicationID, "вложение осталось сиротой")
	assert.True(t, att.IsManual)
}

// Ровно одно из application_id / target_attachment_id обязательно.
func TestAttach_XorRequired400(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	orphanAtt, _ := seedOrphanCar(t, db, td.OrgID, "2026-07-01", "2026-07-31", "2026-07-01", "2026-07-31", "F600FF777", 0)
	path := fmt.Sprintf("/attachments/%d/attach-to-application", orphanAtt)

	rec := testutil.POST(t, e, path, `{}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, "ни одного поля -> 400")

	rec = testutil.POST(t, e, path, `{"application_id": 1, "target_attachment_id": 2}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, "оба поля -> 400")
}

// Привязка возможна только для ручного вложения-сироты; заявочное -> 400.
func TestAttach_NotManual400(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	senderID := seedAttachSender(t, db, td.OrgID)
	appID := seedActiveApp(t, db, td.OrgID, senderID, models.ConfirmationApproved, models.StatusInWork)
	appAtt := seedAppAttachment(t, db, appID, "cars", "2026-07-01", "2026-07-31")
	otherApp := seedActiveApp(t, db, td.OrgID, senderID, models.ConfirmationApproved, models.StatusInWork)

	body := fmt.Sprintf(`{"application_id": %d}`, otherApp)
	rec := testutil.POST(t, e, fmt.Sprintf("/attachments/%d/attach-to-application", appAtt), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, "заявочное вложение нельзя привязать заново")
}

// Повторная привязка того же вложения запрещена: после усыновления оно уже не ручное.
func TestAttach_DoubleAttachRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	senderID := seedAttachSender(t, db, td.OrgID)
	appID := seedActiveApp(t, db, td.OrgID, senderID, models.ConfirmationApproved, models.StatusInWork)
	orphanAtt, _ := seedOrphanCar(t, db, td.OrgID, "2026-07-01", "2026-07-31", "2026-07-01", "2026-07-31", "H800HH777", 0)
	path := fmt.Sprintf("/attachments/%d/attach-to-application", orphanAtt)
	body := fmt.Sprintf(`{"application_id": %d}`, appID)

	rec := testutil.POST(t, e, path, body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "первая привязка: %s", rec.Body.String())

	rec = testutil.POST(t, e, path, body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, "повторная привязка уже заявочного вложения отклонена")
}

// Гейт super/admin: обычный пользователь -> 403.
func TestAttach_Forbidden403(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	userToken := testutil.RegisterAndLogin(t, e, "attach_regular", "regularpass_long_enough_1234567!", 1, td.OrgID, td.CompanyID)

	orphanAtt, _ := seedOrphanCar(t, db, td.OrgID, "2026-07-01", "2026-07-31", "2026-07-01", "2026-07-31", "G700GG777", 0)

	body := `{"application_id": 1}`
	rec := testutil.POST(t, e, fmt.Sprintf("/attachments/%d/attach-to-application", orphanAtt), body, testutil.AuthHeader(userToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "обычный пользователь не может привязывать (super/admin only)")
}
