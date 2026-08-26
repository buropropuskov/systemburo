package handlers_test

import (
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Скрытие персональных данных работника, не давшего согласия на их обработку (#1567
// S10): вместо фамилии, имени и отчества другим показывается логин.

// setUserName проставляет ФИО пользователя напрямую в базе: регистрация его не просит.
func setUserName(t *testing.T, db *gorm.DB, username, last, first, middle string) int {
	t.Helper()
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", username).
		Updates(map[string]any{"last_name": last, "first_name": first, "middle_name": middle}).Error)
	var id int
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", username).
		Select("id").Row().Scan(&id))
	return id
}

// orgUserNames возвращает ФИО и логины участников организации так, как их видит фронт.
func orgUserNames(t *testing.T, e *echo.Echo, token string, orgID int) map[string]string {
	t.Helper()
	rec := testutil.GET(t, e, "/organizations/"+strconv.Itoa(orgID)+"/members", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	type member struct {
		Username string  `json:"username"`
		LastName *string `json:"last_name"`
	}
	members := testutil.ParseResponse[[]member](t, rec)
	out := make(map[string]string, len(members))
	for _, m := range members {
		if m.LastName == nil {
			out[m.Username] = ""
			continue
		}
		out[m.Username] = *m.LastName
	}
	return out
}

// Пока запрос согласия выключен, согласия нет ни у кого - и маскировать в этот момент
// значит обезличить всю систему на установке, где согласие не спрашивают вовсе.
func TestPDConsentMask_DisabledRequirement_KeepsNames(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "mask_off", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_off", "Иванов", "Иван", "Иванович")

	assert.Equal(t, "Иванов", orgUserNames(t, e, admin, td.OrgID)["mask_off"],
		"без включённого запроса согласия ФИО показывается как раньше")
}

// Основной сценарий пользователя: согласующий не дал согласия - его ФИО не видно.
func TestPDConsentMask_NoConsent_HidesNameParts(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")

	testutil.RegisterAndLogin(t, e, "mask_silent", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_silent", "Молчанов", "Пётр", "Петрович")

	names := orgUserNames(t, e, admin, td.OrgID)
	assert.Equal(t, "", names["mask_silent"],
		"ФИО не давшего согласия скрыто - интерфейс покажет логин")
}

// Подтвердивший согласие виден по имени: скрытие снимается сразу, а не по TTL.
func TestPDConsentMask_AfterAccept_ShowsName(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")

	user := testutil.RegisterAndLogin(t, e, "mask_agreed", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_agreed", "Согласнов", "Семён", "Семёнович")
	require.Equal(t, "", orgUserNames(t, e, admin, td.OrgID)["mask_agreed"])

	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(user)).Code)

	assert.Equal(t, "Согласнов", orgUserNames(t, e, admin, td.OrgID)["mask_agreed"],
		"после подтверждения ФИО снова видно")
}

// Ловушка, ради которой признак именно «никогда не давал согласия»: повышение
// редакции не должно разом обезличивать всех, кто согласился с прежней.
func TestPDConsentMask_VersionBump_KeepsNamesOfPastConsent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Редакция 1</p>")

	user := testutil.RegisterAndLogin(t, e, "mask_v1", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_v1", "Староверов", "Игорь", "Игоревич")
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(user)).Code)

	require.Equal(t, http.StatusOK, savePDConsentTextRequiringAgain(t, e, admin, "<p>Редакция 2</p>").Code)

	assert.Equal(t, "Староверов", orgUserNames(t, e, admin, td.OrgID)["mask_v1"],
		"подъём редакции ФИО не скрывает: человек согласие давал, просто просим подтвердить снова")
}

// Супер-администратора гейт не закрывает, значит и скрывать его имя не за что:
// иначе единственная учётная запись, которой чинят настройку, стала бы безымянной.
func TestPDConsentMask_SuperAdmin_NotMasked(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	setUserName(t, db, "testadmin", "Главный", "Админ", "Админович")

	assert.Equal(t, "Главный", orgUserNames(t, e, admin, td.OrgID)["testadmin"])
}

// Заявка показывает подавшего: пока он не дал согласия, вместо ФИО стоит логин.
func TestPDConsentMask_ApplicationSender_MaskedByLogin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Принимающий видит все заявки центра - иначе список пуст и проверять нечего.
	makeApprover(t, db, "testadmin")
	sender := testutil.RegisterAndLogin(t, e, "mask_sender", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_sender", "Заявкин", "Афанасий", "Афанасьевич")
	createSimpleApplication(t, e, sender, td.OrgID)

	before := applicationSenderNames(t, e, admin)
	require.Contains(t, before, "Заявкин", "до включения запроса согласия ФИО на месте")

	enableConsent(t, e, admin, "<p>Согласие</p>")

	after := applicationSenderNames(t, e, admin)
	assert.NotContains(t, after, "Заявкин", "ФИО подавшего скрыто")
	assert.Contains(t, after, "@mask_sender", "вместо ФИО показывается логин с собачкой")
}

// applicationSenderNames собирает имена подавших из списка Центра заявок одной строкой.
func applicationSenderNames(t *testing.T, e *echo.Echo, token string) string {
	t.Helper()
	rec := testutil.GET(t, e, "/applications", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return rec.Body.String()
}

// Маскировка - состояние показа, а не команда стереть данные. Форма редактирования
// работника скрытое ФИО не присылает, и сохранение соседнего поля (должность, почта)
// не должно затирать настоящую фамилию в базе.
func TestPDConsentMask_UpdateUserInfoWithoutNames_KeepsRealName(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")

	testutil.RegisterAndLogin(t, e, "mask_edit", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_edit", "Скрытов", "Кузьма", "Кузьмич")
	require.Equal(t, "", orgUserNames(t, e, admin, td.OrgID)["mask_edit"], "ФИО скрыто")

	// Тело без ключей ФИО - ровно то, что шлёт форма для скрытого работника.
	rec := testutil.PUT(t, e, "/users/mask_edit/info",
		`{"position":"Главный инженер","email":"k@example.com","phone":null,"is_important":false}`,
		testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var last string
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "mask_edit").
		Select("COALESCE(last_name, '')").Row().Scan(&last))
	assert.Equal(t, "Скрытов", last, "правка должности не стирает настоящую фамилию")
}

// Явно переданная пустая строка по-прежнему очищает ФИО: администратор должен мочь
// стереть ошибочно введённые данные.
func TestPDConsentMask_UpdateUserInfoWithEmptyNames_ClearsThem(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "mask_clear", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_clear", "Ошибкин", "Иван", "Иванович")

	rec := testutil.PUT(t, e, "/users/mask_clear/info",
		`{"last_name":"","first_name":"","middle_name":"","position":null,"email":null,"phone":null}`,
		testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var last string
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "mask_clear").
		Select("COALESCE(last_name, '')").Row().Scan(&last))
	assert.Equal(t, "", last, "пустая строка очищает поле, как и раньше")
}

// Скрытое ФИО в списке помечено флагом: без него форма не отличит «скрыто» от
// «не заполнено» и затрёт настоящие данные.
func TestPDConsentMask_UsersList_MarksHiddenNames(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	user := testutil.RegisterAndLogin(t, e, "mask_flag", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_flag", "Флагов", "Фёдор", "Фёдорович")

	require.True(t, userPDHidden(t, e, admin, "mask_flag"), "скрытое ФИО помечено")
	require.False(t, userPDHidden(t, e, admin, "testadmin"), "у супер-администратора ничего не скрыто")

	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(user)).Code)
	assert.False(t, userPDHidden(t, e, admin, "mask_flag"), "после согласия пометка снимается")
}

// userPDHidden читает признак скрытого ФИО из списка работников.
func userPDHidden(t *testing.T, e *echo.Echo, token, username string) bool {
	t.Helper()
	rec := testutil.GET(t, e, "/users/all", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	type row struct {
		Username string `json:"username"`
		PDHidden bool   `json:"pd_hidden"`
	}
	for _, r := range testutil.ParseResponse[[]row](t, rec) {
		if r.Username == username {
			return r.PDHidden
		}
	}
	t.Fatalf("работник %s не найден в списке", username)
	return false
}

// Журналы справочников: актор без согласия подписан логином. Проверяем на истории
// марки - она читает audit_log тем же путём, что и остальные справочники.
func TestPDConsentMask_DirectoryHistory_ActorMaskedByLogin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Актор - обычный администратор (не супер): гейт согласия его закрывает.
	actor := testutil.RegisterManager(t, e, "mask_dir", td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_dir", "Справочников", "Тимур", "Тимурович")

	// Действие делаем над уже существующей маркой: имя справочника уникально, а
	// CleanDB марки не чистит - создание второй раз упиралось бы в дубликат.
	markID := seedMarkForMask(t, db)
	require.Equal(t, http.StatusOK,
		testutil.POST(t, e, "/marks/"+strconv.Itoa(markID)+"/archive", "{}", testutil.AuthHeader(actor)).Code)

	before := markHistoryBody(t, e, admin, markID)
	require.Contains(t, before, "Справочников", "до включения запроса согласия ФИО актора на месте")

	enableConsent(t, e, admin, "<p>Согласие</p>")

	after := markHistoryBody(t, e, admin, markID)
	assert.NotContains(t, after, "Справочников", "ФИО актора скрыто")
	assert.Contains(t, after, "mask_dir", "вместо ФИО подставлен логин")
}

// seedMarkForMask заводит марку прямо в базе и возвращает её id.
func seedMarkForMask(t *testing.T, db *gorm.DB) int {
	t.Helper()
	mark := models.Mark{Name: "Маска-Тест-Марка", IsActive: true}
	require.NoError(t, db.Where("name = ?", mark.Name).FirstOrCreate(&mark).Error)
	require.NoError(t, db.Model(&models.Mark{}).Where("id = ?", mark.ID).
		Update("is_active", true).Error)
	return mark.ID
}

// markHistoryBody возвращает тело истории марки одной строкой.
func markHistoryBody(t *testing.T, e *echo.Echo, token string, markID int) string {
	t.Helper()
	rec := testutil.GET(t, e, "/marks/"+strconv.Itoa(markID)+"/history", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return rec.Body.String()
}

// Пул принимающих - список, из которого заявитель узнаёт, кому уйдёт заявка.
func TestPDConsentMask_ApproversPool_HidesName(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "mask_pool", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_pool", "Приёмов", "Олег", "Олегович")
	makeApprover(t, db, "mask_pool")
	enableConsent(t, e, admin, "<p>Согласие</p>")

	rec := testutil.GET(t, e, "/application-approvers", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.NotContains(t, body, "Приёмов", "ФИО принимающего без согласия скрыто")
	assert.Contains(t, body, "mask_pool", "логин остаётся - иначе строку не опознать")
}

// В истории принимающих ФИО двое: кто действовал и кого добавили или сняли. Второе
// ревью нашло незакрытым - оба должны прятаться.
func TestPDConsentMask_ApproverHistory_HidesBothNames(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterAndLogin(t, e, "mask_subject", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	subjectID := setUserName(t, db, "mask_subject", "Назначенов", "Борис", "Борисович")

	added := testutil.POST(t, e, "/application-approvers",
		`{"user_id":`+strconv.Itoa(subjectID)+`}`, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusCreated, added.Code, added.Body.String())

	rec := testutil.GET(t, e, "/application-approvers/history", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "Назначенов", "до включения запроса согласия ФИО на месте")

	enableConsent(t, e, admin, "<p>Согласие</p>")

	after := testutil.GET(t, e, "/application-approvers/history", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, after.Code)
	assert.NotContains(t, after.Body.String(), "Назначенов", "ФИО назначенного принимающего скрыто")
	assert.Contains(t, after.Body.String(), "mask_subject", "вместо ФИО подставлен логин")
}

// Признак согласия в списке работников: администратор должен видеть, дал ли человек
// согласие, прямо в разделе «Работники», а не догадываться по скрытому ФИО.
func TestPDConsentMask_UsersList_ShowsConsentState(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	user := testutil.RegisterAndLogin(t, e, "consent_state", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	before := userConsentRow(t, e, admin, "consent_state")
	assert.False(t, before.ConsentGranted, "согласия ещё нет")
	assert.False(t, before.ConsentRequired, "запрос выключен - «не дано» ничего не значит")

	enableConsent(t, e, admin, "<p>Согласие</p>")
	during := userConsentRow(t, e, admin, "consent_state")
	assert.True(t, during.ConsentRequired, "запрос включён и этого работника касается")
	assert.False(t, during.ConsentGranted)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(user)).Code)
	after := userConsentRow(t, e, admin, "consent_state")
	assert.True(t, after.ConsentGranted, "после подтверждения согласие видно")
	require.NotNil(t, after.ConsentAt, "дата согласия проставлена")

	// Кого запрос не касается: супер-администратор, архивный и заблокированный.
	assert.False(t, userConsentRow(t, e, admin, "testadmin").ConsentRequired, "супер-администратор")

	testutil.RegisterAndLogin(t, e, "consent_archived", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "consent_archived").
		Update("is_active", false).Error)
	assert.False(t, userConsentRowArchived(t, e, admin, "consent_archived").ConsentRequired, "архивный")

	testutil.RegisterAndLogin(t, e, "consent_banned", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "consent_banned").
		Update("is_banned", true).Error)
	assert.False(t, userConsentRow(t, e, admin, "consent_banned").ConsentRequired, "заблокированный")
}

type consentRow struct {
	Username        string  `json:"username"`
	ConsentGranted  bool    `json:"consent_granted"`
	ConsentAt       *string `json:"consent_at"`
	ConsentRequired bool    `json:"consent_required"`
}

func userConsentRow(t *testing.T, e *echo.Echo, token, username string) consentRow {
	t.Helper()
	return findConsentRow(t, e, token, username, "/users/all")
}

// userConsentRowArchived - тот же запрос, но с архивными: по умолчанию их в списке нет.
func userConsentRowArchived(t *testing.T, e *echo.Echo, token, username string) consentRow {
	t.Helper()
	return findConsentRow(t, e, token, username, "/users/all?include_archived=true")
}

func findConsentRow(t *testing.T, e *echo.Echo, token, username, path string) consentRow {
	t.Helper()
	rec := testutil.GET(t, e, path, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, r := range testutil.ParseResponse[[]consentRow](t, rec) {
		if r.Username == username {
			return r
		}
	}
	t.Fatalf("работник %s не найден", username)
	return consentRow{}
}

// Согласие - факт о самом работнике, и в истории его учётной записи он должен быть
// виден администратору наравне с остальными событиями.
func TestPDConsent_Accept_WritesUserHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")

	user := testutil.RegisterAndLogin(t, e, "hist_consent", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(user)).Code)

	rec := testutil.GET(t, e, "/users/hist_consent/history", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, "consent_granted", "выдача согласия попала в историю учётной записи")
	assert.Contains(t, body, "hist_consent", "актор - сам работник, а не администратор")

	// Отзыв тоже записывается: без него в истории остаётся только половина правды.
	require.Equal(t, http.StatusOK,
		testutil.DELETE(t, e, "/consents/pd_processing", testutil.AuthHeader(user)).Code)
	after := testutil.GET(t, e, "/users/hist_consent/history", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, after.Code)
	assert.Contains(t, after.Body.String(), "consent_revoked")
}

// Отзыв согласия за работника: своей кнопки отзыва у него нет, и без этой ручки
// его письменную просьбу администратору нечем исполнить.
func TestPDConsent_AdminRevokesForUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")

	user := testutil.RegisterAndLogin(t, e, "revoke_target", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(user)).Code)
	require.False(t, gateState(t, e, user).Required, "после подтверждения окно снято")

	rec := testutil.DELETE(t, e, "/users/revoke_target/consent", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.True(t, gateState(t, e, user).Required, "после отзыва система снова спрашивает согласие")
	assert.False(t, userConsentRow(t, e, admin, "revoke_target").ConsentGranted,
		"в разделе работников согласие больше не числится")

	// В истории учётной записи отзыв записан на администратора, а не на работника:
	// человек не передумал, отозвали за него.
	hist := testutil.GET(t, e, "/users/revoke_target/history", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, hist.Code)
	body := hist.Body.String()
	assert.Contains(t, body, "consent_revoked")
	assert.Contains(t, body, "by_admin")
}

// Отзывать чужое согласие может только тот, кому доступен раздел работников.
func TestPDConsent_RevokeForUser_RequiresPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	testutil.RegisterAndLogin(t, e, "revoke_victim", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	outsider := testutil.RegisterAndLogin(t, e, "revoke_outsider", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	assert.Equal(t, http.StatusForbidden,
		testutil.DELETE(t, e, "/users/revoke_victim/consent", testutil.AuthHeader(outsider)).Code)
}

// Несуществующий работник - 404, а не молчаливый успех.
func TestPDConsent_RevokeForUser_UnknownUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	assert.Equal(t, http.StatusNotFound,
		testutil.DELETE(t, e, "/users/no_such_person/consent", testutil.AuthHeader(admin)).Code)
}

// Почта и телефон - такие же персональные данные, как фамилия: до согласия их не
// показывают и не дают затереть правкой соседнего поля.
func TestPDConsentMask_HidesContacts(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	user := testutil.RegisterAndLogin(t, e, "mask_contacts", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_contacts", "Контактов", "Кирилл", "Кириллович")
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "mask_contacts").
		Updates(map[string]any{"email": "k@example.com", "phone": "+7 999 111 22-33"}).Error)

	before := userContactsRow(t, e, admin, "mask_contacts")
	require.Equal(t, "k@example.com", before.Email, "до включения запроса согласия контакты видны")

	enableConsent(t, e, admin, "<p>Согласие</p>")

	hidden := userContactsRow(t, e, admin, "mask_contacts")
	assert.Empty(t, hidden.Email, "почта скрыта")
	assert.Empty(t, hidden.Phone, "телефон скрыт")
	assert.True(t, hidden.PDHidden, "признак скрытых данных выставлен")

	// Правка должности без полей контактов настоящие значения не трогает.
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, "/users/mask_contacts/info",
		`{"position":"Инженер","is_important":false}`, testutil.AuthHeader(admin)).Code)
	var email string
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "mask_contacts").
		Select("COALESCE(email, '')").Row().Scan(&email))
	assert.Equal(t, "k@example.com", email, "почта в базе осталась")

	// После подтверждения контакты возвращаются.
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(user)).Code)
	after := userContactsRow(t, e, admin, "mask_contacts")
	assert.Equal(t, "k@example.com", after.Email)
	assert.Equal(t, "+7 999 111 22-33", after.Phone)
}

type contactsRow struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	PDHidden bool   `json:"pd_hidden"`
}

func userContactsRow(t *testing.T, e *echo.Echo, token, username string) contactsRow {
	t.Helper()
	rec := testutil.GET(t, e, "/users/all", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, r := range testutil.ParseResponse[[]contactsRow](t, rec) {
		if r.Username == username {
			return r
		}
	}
	t.Fatalf("работник %s не найден", username)
	return contactsRow{}
}

// Сквозной поиск не должен ни находить по скрытым контактам, ни показывать скрытое ФИО.
func TestPDConsentMask_Search_HidesPD(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	testutil.RegisterAndLogin(t, e, "mask_search", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "mask_search", "Поисков", "Павел", "Павлович")
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "mask_search").
		Update("email", "hidden_addr@example.com").Error)

	require.NotEmpty(t, searchTitles(t, e, admin, "hidden_addr"),
		"до включения запроса согласия поиск по почте работает")

	enableConsent(t, e, admin, "<p>Согласие</p>")

	// Запросы намеренно разные: ответы поиска живут в кэше 10 секунд, и повтор той
	// же строки вернул бы прежнюю выдачу вместо новой.
	assert.Empty(t, searchTitles(t, e, admin, "hidden_addr@example"),
		"по скрытой почте больше не находится")
	assert.Equal(t, []string{"@mask_search"}, searchTitles(t, e, admin, "Поисков"),
		"вместо скрытого ФИО в подсказке логин")
}

// searchTitles возвращает заголовки найденных учётных записей.
func searchTitles(t *testing.T, e *echo.Echo, token, query string) []string {
	t.Helper()
	rec := testutil.GET(t, e, "/search?q="+url.QueryEscape(query), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	type group struct {
		Type  string `json:"type"`
		Items []struct {
			Title string `json:"title"`
		} `json:"items"`
	}
	res := testutil.ParseResponse[struct {
		Groups []group `json:"groups"`
	}](t, rec)
	titles := []string{}
	for _, g := range res.Groups {
		if g.Type != "users" {
			continue
		}
		for _, it := range g.Items {
			titles = append(titles, it.Title)
		}
	}
	return titles
}
