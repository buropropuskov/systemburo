package handlers_test

import (
	"fmt"
	"net/http"
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

// GET /applications/:id/participants - все участники заявки одним списком: отправитель,
// принявший в работу, согласующие, ответственные и читатели.

const participantPassword = "password123456789012345678901234"

// registerParticipant заводит работника с заполненной карточкой: регистрация ФИО и
// контактов не просит, а именно их и отдаёт метод участников.
func registerParticipant(t *testing.T, e *echo.Echo, db *gorm.DB, username, last, first, middle, email, phone string, orgID, companyID int) (string, int) {
	t.Helper()
	token := testutil.RegisterAndLogin(t, e, username, participantPassword, 1, orgID, companyID)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", username).
		Updates(map[string]any{
			"last_name":   last,
			"first_name":  first,
			"middle_name": middle,
			"position":    "Инженер " + username,
			"email":       email,
			"phone":       phone,
		}).Error)
	return token, getUserID(t, db, username)
}

// participantsByID читает список участников так, как его получит фронт, и раскладывает
// по идентификатору работника.
func participantsByID(t *testing.T, e *echo.Echo, token string, appID int) map[int]services.ApplicationParticipant {
	t.Helper()
	list := participantsList(t, e, token, appID)
	out := make(map[int]services.ApplicationParticipant, len(list))
	for _, p := range list {
		out[p.UserID] = p
	}
	return out
}

func participantsList(t *testing.T, e *echo.Echo, token string, appID int) []services.ApplicationParticipant {
	t.Helper()
	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/participants", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return testutil.ParseResponse[[]services.ApplicationParticipant](t, rec)
}

// Каждая роль на своём месте, контакты и названия организации с компанией на месте,
// состояние голоса - только у согласующего.
func TestApplicationParticipants_RolesContactsAndOrganizations(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken, senderID := registerParticipant(t, e, db, "pt_sender",
		"Отправителев", "Олег", "Олегович", "sender@example.com", "+7 900 000 00 01", td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	_, acceptorID := registerParticipant(t, e, db, "pt_acceptor",
		"Принимаев", "Пётр", "Петрович", "acceptor@example.com", "+7 900 000 00 02", td.OrgID, td.CompanyID)
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).
		Update("responsible_user_id", acceptorID).Error)

	_, approverID := registerParticipant(t, e, db, "pt_approver",
		"Согласуев", "Семён", "Семёнович", "approver@example.com", "+7 900 000 00 03", td.OrgID, td.CompanyID)
	approvedAt := time.Date(2026, 5, 12, 9, 30, 0, 0, time.UTC)
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
		ApplicationID:    appID,
		UserID:           approverID,
		RequiredApproval: true,
		ApprovalStatus:   strPtr("approved"),
		ApprovalComment:  strPtr("Согласовано без замечаний"),
		ApprovalDatetime: &approvedAt,
	}).Error)

	_, responsibleID := registerParticipant(t, e, db, "pt_responsible",
		"Ответов", "Роман", "Романович", "responsible@example.com", "+7 900 000 00 04", td.OrgID, td.CompanyID)
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
		ApplicationID:    appID,
		UserID:           responsibleID,
		RequiredApproval: false,
	}).Error)

	_, readerID := registerParticipant(t, e, db, "pt_reader",
		"Читаев", "Charlie", "Чарльзович", "reader@example.com", "+7 900 000 00 05", td.OrgID, td.CompanyID)
	require.NoError(t, db.Create(&models.ApplicationViewer{ApplicationID: appID, UserID: readerID}).Error)

	list := participantsList(t, e, senderToken, appID)
	require.Len(t, list, 5, "в списке ровно пятеро участников: %+v", list)
	byID := participantsByID(t, e, senderToken, appID)

	assert.Equal(t, []string{services.ParticipantRoleSender}, byID[senderID].Roles, "подавший - отправитель")
	assert.Equal(t, []string{services.ParticipantRoleAcceptor}, byID[acceptorID].Roles, "взявший в работу - принимающий")
	assert.Equal(t, []string{services.ParticipantRoleApprover}, byID[approverID].Roles, "required_approval=true - согласующий")
	assert.Equal(t, []string{services.ParticipantRoleApprover}, byID[responsibleID].Roles, "required_approval=false - тоже согласующий, просто необязательный")
	assert.True(t, byID[approverID].RequiredApproval, "обязательность голоса - признак, а не отдельная роль")
	assert.False(t, byID[responsibleID].RequiredApproval)
	assert.Equal(t, []string{services.ParticipantRoleReader}, byID[readerID].Roles, "application_viewers - читатель")

	approver := byID[approverID]
	assert.Equal(t, "Согласуев", deref(approver.LastName))
	assert.Equal(t, "Семён", deref(approver.FirstName))
	assert.Equal(t, "Семёнович", deref(approver.MiddleName))
	assert.Equal(t, "Согласуев Семён Семёнович", approver.FullName)
	assert.Equal(t, "Инженер pt_approver", deref(approver.Position))
	assert.Equal(t, "approver@example.com", deref(approver.Email), "ради контактов метод и заводился")
	assert.Equal(t, "+7 900 000 00 03", deref(approver.Phone))
	assert.Equal(t, "Test Organization", deref(approver.OrganizationName), "организация названием, не только идентификатором")
	assert.Equal(t, "Test Company", deref(approver.CompanyName))
	require.NotNil(t, approver.OrganizationID)
	assert.Equal(t, td.OrgID, *approver.OrganizationID)
	require.NotNil(t, approver.CompanyID)
	assert.Equal(t, td.CompanyID, *approver.CompanyID)

	assert.Equal(t, "approved", deref(approver.ApprovalStatus))
	assert.Equal(t, "Согласовано без замечаний", deref(approver.ApprovalComment))
	require.NotNil(t, approver.ApprovalDatetime)
	assert.True(t, approvedAt.Equal(*approver.ApprovalDatetime), "дата голоса отдана как есть: %v", approver.ApprovalDatetime)

	// Голосуют все, у кого есть строка в application_responsible_users: голос
	// необязательного на исход не влияет, но карточка заявки его показывает, и список
	// участников обязан говорить то же самое.
	var storedStatus string
	require.NoError(t, db.Model(&models.ApplicationResponsibleUser{}).
		Where("application_id = ? AND user_id = ?", appID, responsibleID).
		Select("COALESCE(approval_status, '')").Row().Scan(&storedStatus))
	require.Equal(t, "pending", storedStatus, "в базе у необязательного согласующего дефолтный pending")
	assert.Equal(t, "pending", deref(byID[responsibleID].ApprovalStatus), "его состояние голоса тоже видно")
	assert.Nil(t, byID[senderID].ApprovalStatus, "у отправителя голоса нет вовсе")

	// Порядок списка: автор, принявший, согласующие, читатель. Согласующие идут
	// одной группой и внутри неё по видимому имени: «Ответов» раньше «Согласуева».
	assert.Equal(t, []int{senderID, acceptorID, responsibleID, approverID, readerID},
		[]int{list[0].UserID, list[1].UserID, list[2].UserID, list[3].UserID, list[4].UserID},
		"порядок от автора к читателю, согласующие по алфавиту")
}

// Посторонний список участников не получает: контакты половины бюро - не публичные данные.
func TestApplicationParticipants_OutsiderForbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "pt_own_sender", participantPassword, 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	outsider := testutil.RegisterAndLogin(t, e, "pt_outsider", participantPassword, 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/participants", appID), testutil.AuthHeader(outsider))
	assert.Equal(t, http.StatusForbidden, rec.Code, "посторонний не участник заявки: %s", rec.Body.String())

	// Читателя пускаем: он участник, пусть и самый бесправный.
	readerToken := testutil.RegisterAndLogin(t, e, "pt_reader_access", participantPassword, 1, td.OrgID, td.CompanyID)
	readerID := getUserID(t, db, "pt_reader_access")
	require.NoError(t, db.Create(&models.ApplicationViewer{ApplicationID: appID, UserID: readerID}).Error)
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/participants", appID), testutil.AuthHeader(readerToken))
	assert.Equal(t, http.StatusOK, rec.Code, "читатель заявки список участников видит")
}

// Один человек в нескольких ролях приходит одной записью: панель рисует по одному
// бейджу на участника, и два одинаковых ФИО подряд читались бы как ошибка данных.
func TestApplicationParticipants_SamePersonInSeveralRoles(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken, senderID := registerParticipant(t, e, db, "pt_multi",
		"Многоролев", "Михаил", "Михайлович", "multi@example.com", "+7 900 000 00 06", td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	// Тот же человек: сам подал, сам взял в работу, сам же согласующий.
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).
		Update("responsible_user_id", senderID).Error)
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
		ApplicationID:    appID,
		UserID:           senderID,
		RequiredApproval: true,
		ApprovalStatus:   strPtr("rejected"),
		ApprovalComment:  strPtr("Передумал"),
	}).Error)
	require.NoError(t, db.Create(&models.ApplicationViewer{ApplicationID: appID, UserID: senderID}).Error)

	list := participantsList(t, e, senderToken, appID)
	require.Len(t, list, 1, "один человек - одна запись: %+v", list)

	me := list[0]
	assert.Equal(t, []string{
		services.ParticipantRoleSender,
		services.ParticipantRoleAcceptor,
		services.ParticipantRoleApprover,
		services.ParticipantRoleReader,
	}, me.Roles, "роли перечислены по старшинству")
	assert.Equal(t, services.ParticipantRoleSender, me.PrimaryRole, "бейдж - старшая роль")
	assert.Equal(t, "rejected", deref(me.ApprovalStatus), "состояние голоса не теряется при склейке ролей")
	assert.Equal(t, "Передумал", deref(me.ApprovalComment))
}

// Работник без согласия на обработку персональных данных (#1567) приходит скрытым:
// ни ФИО, ни контактов. Почта здесь не менее чувствительна, чем фамилия - рабочий
// адрес её и содержит.
func TestApplicationParticipants_PDConsentHidesNameAndContacts(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	senderToken := testutil.RegisterAndLogin(t, e, "pt_pd_sender", participantPassword, 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	approverToken, approverID := registerParticipant(t, e, db, "pt_pd_approver",
		"Молчанов", "Максим", "Максимович", "silent@example.com", "+7 900 000 00 07", td.OrgID, td.CompanyID)
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
		ApplicationID: appID, UserID: approverID, RequiredApproval: true,
	}).Error)

	before := participantsByID(t, e, senderToken, appID)[approverID]
	require.Equal(t, "Молчанов", deref(before.LastName), "пока согласие не запрашивают, всё видно как раньше")
	require.Equal(t, "silent@example.com", deref(before.Email))
	require.False(t, before.PDHidden)

	enableConsent(t, e, admin, "<p>Согласие</p>")

	hidden := participantsByID(t, e, senderToken, appID)[approverID]
	assert.Nil(t, hidden.LastName, "фамилия скрыта")
	assert.Nil(t, hidden.FirstName)
	assert.Nil(t, hidden.MiddleName)
	assert.Empty(t, hidden.FullName, "собранное ФИО тоже скрыто - иначе маскировка обходится одним полем")
	assert.Nil(t, hidden.Email, "почта скрыта: рабочий адрес выдаёт человека не хуже фамилии")
	assert.Nil(t, hidden.Phone, "телефон скрыт")
	assert.True(t, hidden.PDHidden, "признак отличает скрытое от незаполненного")
	assert.Equal(t, "pt_pd_approver", hidden.Username, "логин остаётся - им интерфейс подписывает скрытого")
	assert.Equal(t, []string{services.ParticipantRoleApprover}, hidden.Roles, "роль скрытие не трогает")
	assert.Equal(t, "Test Organization", deref(hidden.OrganizationName), "организация - не персональные данные")

	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(approverToken)).Code)

	shown := participantsByID(t, e, senderToken, appID)[approverID]
	assert.Equal(t, "Молчанов", deref(shown.LastName), "после подтверждения согласия ФИО снова видно")
	assert.Equal(t, "silent@example.com", deref(shown.Email), "и контакты тоже")
	assert.False(t, shown.PDHidden)
}

// Принимающему администратор может задать отображаемое имя - заявитель не должен знать,
// кто именно взял заявку. Список участников обязан эту маску держать: настоящее ФИО и
// личная почта сняли бы её в один клик.
func TestApplicationParticipants_AcceptorDisplayNameMaskHoldsContacts(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "pt_mask_sender", participantPassword, 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	_, acceptorID := registerParticipant(t, e, db, "pt_mask_acceptor",
		"Тайнов", "Тимофей", "Тимофеевич", "secret@example.com", "+7 900 000 00 08", td.OrgID, td.CompanyID)
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).
		Update("responsible_user_id", acceptorID).Error)
	require.NoError(t, db.Create(&models.ApplicationApprover{
		UserID: acceptorID, DisplayName: strPtr("Бюро пропусков"),
	}).Error)

	masked := participantsByID(t, e, senderToken, appID)[acceptorID]
	assert.Equal(t, "Бюро пропусков", masked.FullName, "показываем заданную маску")
	assert.Nil(t, masked.LastName, "настоящая фамилия наружу не идёт")
	assert.Nil(t, masked.FirstName)
	assert.Nil(t, masked.MiddleName)
	assert.Nil(t, masked.Email, "личная почта сняла бы маску")
	assert.Nil(t, masked.Phone)
	assert.Equal(t, []string{services.ParticipantRoleAcceptor}, masked.Roles)
}

// Несуществующая заявка - 404 супер-администратору (обычного отбивает гейт доступа),
// а не пустой список: пустых участников у заявки не бывает, её всегда кто-то подал.
func TestApplicationParticipants_UnknownApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/applications/999999/participants", testutil.AuthHeader(admin))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}
