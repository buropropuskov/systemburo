package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Тесты правки срока заявки принимающим (#2575): пока заявка не принята и по ней нет
// итога согласования, принимающий задаёт новое окно всем вложениям и машинам, голоса
// согласующих сбрасываются, участники получают уведомление.

func datesPath(appID int) string {
	return fmt.Sprintf("/applications/%d/dates", appID)
}

func datesBody(from, to, timeFrom, timeTo, reason string) string {
	return fmt.Sprintf(`{"entry_date_from":%q,"entry_date_to":%q,"entry_time_from":%q,"entry_time_to":%q,"reason":%q}`,
		from, to, timeFrom, timeTo, reason)
}

// seedDatesApp создаёт заявку с cars-вложением и машиной. У машины свой срок, отличный
// от вложения: правка обязана выровнять и его.
func seedDatesApp(t *testing.T, db *gorm.DB, orgID, senderID int, status string, confirmation *string, plate string) (appID, attID, carID int) {
	t.Helper()
	num := "APP-DATES-" + plate
	app := models.Application{
		ApplicationNumber: &num,
		Confirmation:      confirmation,
		Status:            &status,
		OrganizationID:    orgID,
		SenderUserID:      senderID,
	}
	require.NoError(t, db.Create(&app).Error)

	st := 0
	from, to, tFrom, tTo := "2099-01-10", "2099-01-12", "09:00:00", "18:00:00"
	att := models.Attachment{
		ApplicationID: &app.ID, AttachmentType: "cars",
		EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &tFrom, EntryTimeTo: &tTo,
		Status: &st,
	}
	require.NoError(t, db.Create(&att).Error)

	carFrom, carTo := "2099-01-11", "2099-01-11"
	car := models.Car{AttachmentID: att.ID, CarNumber: &plate, Status: &st,
		EntryDateFrom: &carFrom, EntryDateTo: &carTo, EntryTimeFrom: &tFrom, EntryTimeTo: &tTo}
	require.NoError(t, db.Create(&car).Error)
	return app.ID, att.ID, car.ID
}

func seedDatesUser(t *testing.T, db *gorm.DB, username string, orgID int) int {
	t.Helper()
	u := models.User{Username: username, Password: "x", TypeID: 1, OrganizationID: &orgID}
	require.NoError(t, db.Create(&u).Error)
	return u.ID
}

func addResponsible(t *testing.T, db *gorm.DB, appID, userID int, vote string) {
	t.Helper()
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
		ApplicationID: appID, UserID: userID, RequiredApproval: true, ApprovalStatus: &vote,
	}).Error)
}

type storedPeriod struct {
	EntryDateFrom, EntryDateTo, EntryTimeFrom, EntryTimeTo string
}

func periodOf(t *testing.T, db *gorm.DB, table string, id int) storedPeriod {
	t.Helper()
	var p storedPeriod
	require.NoError(t, db.Raw(fmt.Sprintf(
		"SELECT entry_date_from, entry_date_to, entry_time_from, entry_time_to FROM %s WHERE id = ?", table), id).
		Scan(&p).Error)
	return p
}

func datesNotifCount(t *testing.T, db *gorm.DB, userID int) int {
	t.Helper()
	var n int
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM notifications WHERE user_id = ? AND type = 'application_dates_changed'", userID).
		Scan(&n).Error)
	return n
}

// Принимающий сдвигает срок: окно ложится на вложение и машину, в историю заявки пишется
// «было -> стало» с причиной, участники получают уведомление, сам принимающий - нет.
func TestChangeDates_ApproverChangesPeriod(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")
	actorID := getUserID(t, db, "testadmin")

	senderID := seedAttachSender(t, db, td.OrgID)
	appID, attID, carID := seedDatesApp(t, db, td.OrgID, senderID, models.StatusProcessing, nil, "D111DD777")

	rec := testutil.PUT(t, e, datesPath(appID),
		datesBody("2099-02-01", "2099-02-03", "08:30", "20:00", "заявитель ошибся на месяц"), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "правка срока: %s", rec.Body.String())

	var resp struct {
		Data struct {
			OldPeriod      string `json:"old_period"`
			NewPeriod      string `json:"new_period"`
			ApprovalsReset bool   `json:"approvals_reset"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "10.01.2099 09:00 - 12.01.2099 18:00", resp.Data.OldPeriod)
	assert.Equal(t, "01.02.2099 08:30 - 03.02.2099 20:00", resp.Data.NewPeriod)
	assert.False(t, resp.Data.ApprovalsReset, "голосов не было - сбрасывать нечего")

	want := storedPeriod{"2099-02-01", "2099-02-03", "08:30:00", "20:00:00"}
	assert.Equal(t, want, periodOf(t, db, "attachments", attID), "вложение получило новое окно")
	assert.Equal(t, want, periodOf(t, db, "cars", carID), "машина получила то же окно, а не свой прежний срок")

	var audit struct {
		ActorID *int
		Details string
	}
	require.NoError(t, db.Raw(`SELECT actor_user_id AS actor_id, details::text AS details FROM audit_log
		WHERE entity_type = ? AND entity_id = ? AND action = 'dates_changed'`, models.AuditEntityApplication, appID).
		Scan(&audit).Error)
	require.NotNil(t, audit.ActorID, "запись в истории заявки есть")
	assert.Equal(t, actorID, *audit.ActorID)
	assert.Contains(t, audit.Details, "10.01.2099 09:00 - 12.01.2099 18:00")
	assert.Contains(t, audit.Details, "01.02.2099 08:30 - 03.02.2099 20:00")
	assert.Contains(t, audit.Details, "заявитель ошибся на месяц")

	assert.Equal(t, 1, datesNotifCount(t, db, senderID), "заявитель уведомлён")
	assert.Equal(t, 0, datesNotifCount(t, db, actorID), "автору правки уведомление не шлём")
	var message string
	require.NoError(t, db.Raw("SELECT message FROM notifications WHERE user_id = ? AND type = 'application_dates_changed'", senderID).
		Scan(&message).Error)
	assert.Contains(t, message, "стало 01.02.2099 08:30 - 03.02.2099 20:00")
	assert.Contains(t, message, "Причина: заявитель ошибся на месяц")
}

// Уже поданный голос снимается: согласующий одобрял другое окно. Он же получает
// уведомление с пометкой о повторном согласовании.
func TestChangeDates_ResetsCastVotes(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	senderID := seedAttachSender(t, db, td.OrgID)
	voterA := seedDatesUser(t, db, "dates_voter_a", td.OrgID)
	voterB := seedDatesUser(t, db, "dates_voter_b", td.OrgID)
	pending := models.ConfirmationPending
	appID, _, _ := seedDatesApp(t, db, td.OrgID, senderID, models.StatusProcessing, &pending, "D222DD777")
	addResponsible(t, db, appID, voterA, "approved")
	addResponsible(t, db, appID, voterB, "pending")

	rec := testutil.PUT(t, e, datesPath(appID),
		datesBody("2099-03-01", "2099-03-01", "10:00", "12:00", "перенос по просьбе заявителя"), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "правка срока: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"approvals_reset":true`)

	var vote string
	require.NoError(t, db.Raw("SELECT approval_status FROM application_responsible_users WHERE application_id = ? AND user_id = ?",
		appID, voterA).Scan(&vote).Error)
	assert.Equal(t, "pending", vote, "голос согласующего снят")

	var confirmation string
	require.NoError(t, db.Raw("SELECT confirmation FROM applications WHERE id = ?", appID).Scan(&confirmation).Error)
	assert.Equal(t, models.ConfirmationPending, confirmation)

	assert.Equal(t, 1, datesNotifCount(t, db, voterA), "согласующий уведомлён")
	var message string
	require.NoError(t, db.Raw("SELECT message FROM notifications WHERE user_id = ? AND type = 'application_dates_changed'", voterA).
		Scan(&message).Error)
	assert.Contains(t, message, "Голоса согласующих сброшены")
}

// Не принимающий срок не меняет, даже будучи автором заявки.
func TestChangeDates_NonApproverForbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	senderToken := testutil.RegisterAndLogin(t, e, "dates_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "dates_sender")

	appID, attID, _ := seedDatesApp(t, db, td.OrgID, senderID, models.StatusUnread, nil, "D333DD777")

	rec := testutil.PUT(t, e, datesPath(appID),
		datesBody("2099-02-01", "2099-02-03", "08:30", "20:00", "хочу подольше"), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "автор заявки срок не правит: %s", rec.Body.String())
	assert.Equal(t, "2099-01-10", periodOf(t, db, "attachments", attID).EntryDateFrom, "срок не тронут")
}

// Принятая или получившая итог согласования заявка уже обещана охране: срок закрыт.
func TestChangeDates_ClosedStatesRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")
	senderID := seedAttachSender(t, db, td.OrgID)

	approved := models.ConfirmationApproved
	rejected := models.ConfirmationRejected
	cases := []struct {
		name         string
		status       string
		confirmation *string
		plate        string
	}{
		{"в работе", models.StatusInWork, nil, "D401DD777"},
		{"согласована", models.StatusProcessing, &approved, "D402DD777"},
		{"не согласована", models.StatusProcessing, &rejected, "D403DD777"},
		{"отказано", models.StatusRefused, nil, "D404DD777"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			appID, attID, _ := seedDatesApp(t, db, td.OrgID, senderID, tc.status, tc.confirmation, tc.plate)
			rec := testutil.PUT(t, e, datesPath(appID),
				datesBody("2099-02-01", "2099-02-03", "08:30", "20:00", "поздно"), testutil.AuthHeader(token))
			require.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", tc.name, rec.Body.String())
			assert.Equal(t, "2099-01-10", periodOf(t, db, "attachments", attID).EntryDateFrom, "срок не тронут")
		})
	}
}

// Окно проверяется как в форме подачи: без причины, наоборот, в прошлом и без изменений
// правка отклоняется и ничего не пишет.
func TestChangeDates_InvalidPeriodRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, attID, carID := seedDatesApp(t, db, td.OrgID, senderID, models.StatusProcessing, nil, "D555DD777")

	cases := []struct {
		name string
		body string
	}{
		{"без причины", datesBody("2099-02-01", "2099-02-03", "08:30", "20:00", "   ")},
		{"окончание раньше начала", datesBody("2099-02-03", "2099-02-01", "08:30", "20:00", "ошибка")},
		{"время наоборот в один день", datesBody("2099-02-01", "2099-02-01", "20:00", "08:30", "ошибка")},
		{"срок в прошлом", datesBody("2020-02-01", "2020-02-03", "08:30", "20:00", "ошибка")},
		{"неверная дата", datesBody("01.02.2099", "03.02.2099", "08:30", "20:00", "ошибка")},
		{"неверное время", datesBody("2099-02-01", "2099-02-03", "8 утра", "20:00", "ошибка")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := testutil.PUT(t, e, datesPath(appID), tc.body, testutil.AuthHeader(token))
			require.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", tc.name, rec.Body.String())
		})
	}
	assert.Equal(t, "2099-01-10", periodOf(t, db, "attachments", attID).EntryDateFrom, "срок не тронут")

	// Машину выравниваем на окно вложения: теперь ни вложение, ни машина не меняются.
	require.NoError(t, db.Exec("UPDATE cars SET entry_date_from = '2099-01-10', entry_date_to = '2099-01-12' WHERE id = ?", carID).Error)
	rec := testutil.PUT(t, e, datesPath(appID),
		datesBody("2099-01-10", "2099-01-12", "09:00", "18:00", "то же самое"), testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, "тот же срок: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Срок не изменился")
}

// Правило машины «По факту» (#2320) действует и здесь: срок дальше суток бюро не
// растягивает, иначе ограничение без обходов обходилось бы правкой после подачи.
func TestChangeDates_ByFactLimitApplies(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")
	senderID := seedAttachSender(t, db, td.OrgID)
	appID, attID, _ := seedDatesApp(t, db, td.OrgID, senderID, models.StatusProcessing, nil, "По факту")

	rec := testutil.PUT(t, e, datesPath(appID),
		datesBody("2099-02-01", "2099-02-03", "08:30", "20:00", "на неделю"), testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, "«По факту» на три дня: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "По факту")
	assert.Equal(t, "2099-01-10", periodOf(t, db, "attachments", attID).EntryDateFrom, "срок не тронут")
}
