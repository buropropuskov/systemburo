package handlers_test

import (
	"net/http"
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
	assert.Contains(t, after, "mask_sender", "вместо ФИО показывается логин")
}

// applicationSenderNames собирает имена подавших из списка Центра заявок одной строкой.
func applicationSenderNames(t *testing.T, e *echo.Echo, token string) string {
	t.Helper()
	rec := testutil.GET(t, e, "/applications", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return rec.Body.String()
}
