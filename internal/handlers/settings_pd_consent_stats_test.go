package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сводка по сбору согласий (#1567). Главное требование: считать ТОЙ ЖЕ меркой, что и
// гейт, иначе администратор увидит число, которому нельзя верить - «согласились все»
// при том, что часть людей система не пускает.

const collectionPath = pdConsentPath + "/collection"

func collection(t *testing.T, e *echo.Echo, token string) *models.PDConsentCollection {
	t.Helper()
	return collectionAt(t, e, token, collectionPath)
}

func collectionAt(t *testing.T, e *echo.Echo, token, path string) *models.PDConsentCollection {
	t.Helper()
	rec := testutil.GET(t, e, path, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return testutil.ParseResponse[*models.PDConsentCollection](t, rec)
}

// Список не подтвердивших урезается: сразу после подъёма редакции в него попадают
// все работники, и на крупной установке полный список - тысячи строк в разметке.
// Урезание обязано быть видимым, а полный список - доступным выгрузке.
func TestPDConsentCollection_PendingListTruncatedAndFull(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")

	const extra = 3
	for i := 0; i < services.PendingListLimit+extra; i++ {
		testutil.RegisterAndLogin(t, e,
			fmt.Sprintf("coll_many_%02d", i), "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	}

	got := collection(t, e, admin)
	assert.Equal(t, services.PendingListLimit+extra, got.Pending, "счётчик считает всех, а не показанных")
	assert.Len(t, got.PendingUsers, services.PendingListLimit)
	assert.True(t, got.Truncated, "урезание обязано быть видимым")

	full := collectionAt(t, e, admin, collectionPath+"?full=1")
	assert.Len(t, full.PendingUsers, services.PendingListLimit+extra)
	assert.False(t, full.Truncated)
}

func TestPDConsentCollection_CountsOnlyGatedUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")

	user := testutil.RegisterAndLogin(t, e, "coll_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	got := collection(t, e, admin)
	// Супер-администратор в знаменатель не входит: гейт его не закрывает.
	assert.Equal(t, 1, got.Total)
	assert.Equal(t, 0, got.Accepted)
	assert.Equal(t, 1, got.Pending)
	require.Len(t, got.PendingUsers, 1)
	assert.Equal(t, "coll_user", got.PendingUsers[0].Username)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user)).Code)

	got = collection(t, e, admin)
	assert.Equal(t, 1, got.Accepted)
	assert.Equal(t, 0, got.Pending)
	assert.Empty(t, got.PendingUsers)
}

// Подъём редакции возвращает человека в список не подтвердивших - ровно так же, как
// он возвращает ему окно согласия.
func TestPDConsentCollection_VersionBumpReturnsUserToPending(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	user := testutil.RegisterAndLogin(t, e, "coll_bump", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user)).Code)
	require.Equal(t, 1, collection(t, e, admin).Accepted)

	require.Equal(t, http.StatusOK,
		testutil.POST(t, e, pdConsentPath+"/require-again", `{}`, testutil.AuthHeader(admin)).Code)

	got := collection(t, e, admin)
	assert.Equal(t, 2, got.Version)
	assert.Equal(t, 0, got.Accepted)
	assert.Equal(t, 1, got.Pending)
	require.Len(t, got.PendingUsers, 1)
	assert.Equal(t, "coll_bump", got.PendingUsers[0].Username)
}

// Архивных и заблокированных гейт не закрывает (их раньше отбивает проверка
// блокировки), поэтому в знаменателе им не место - иначе сводка никогда не дойдёт
// до 100%, сколько работников ни собери.
func TestPDConsentCollection_ExcludesArchivedAndBanned(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	testutil.RegisterAndLogin(t, e, "coll_active", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "coll_archived", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "coll_banned", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	require.NoError(t, db.Exec("UPDATE users SET is_active = false WHERE username = ?", "coll_archived").Error)
	require.NoError(t, db.Exec("UPDATE users SET is_banned = true WHERE username = ?", "coll_banned").Error)

	got := collection(t, e, admin)
	assert.Equal(t, 1, got.Total)
	require.Len(t, got.PendingUsers, 1)
	assert.Equal(t, "coll_active", got.PendingUsers[0].Username)
}

// Отозванное согласие перестаёт считаться - та же формула, что у ActiveVersion.
func TestPDConsentCollection_RevokedConsentCountsAsPending(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	user := testutil.RegisterAndLogin(t, e, "coll_revoke", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user)).Code)
	require.Equal(t, 1, collection(t, e, admin).Accepted)

	require.Equal(t, http.StatusOK,
		testutil.DELETE(t, e, "/consents/pd_processing", testutil.AuthHeader(user)).Code)

	got := collection(t, e, admin)
	assert.Equal(t, 0, got.Accepted)
	assert.Equal(t, 1, got.Pending)
}

func TestPDConsentCollection_ShowsNameAndOrganization(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	testutil.RegisterAndLogin(t, e, "coll_named", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.NoError(t, db.Exec(
		"UPDATE users SET last_name = ?, first_name = ? WHERE username = ?",
		"Петров", "Пётр", "coll_named").Error)

	got := collection(t, e, admin)
	require.Len(t, got.PendingUsers, 1)
	assert.Equal(t, "Петров Пётр", got.PendingUsers[0].FullName)
	assert.NotEmpty(t, got.PendingUsers[0].Organization, "организация нужна, чтобы найти человека")
}

// Пока запрос согласия выключен, гейт не закрывает никого, а подтвердить согласие
// нельзя в принципе (Accept при выключенном требовании ничего не пишет). Счёт в этот
// момент всегда «0 из N», и выдавать его за ход сбора значит врать администратору -
// поэтому сводка обязана честно сказать, что она неактивна.
func TestPDConsentCollection_InactiveWhenDisabled(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "coll_off", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	got := collection(t, e, admin)
	assert.False(t, got.Active, "тумблер выключен - сбор не идёт")

	enableConsent(t, e, admin, "<p>Согласие</p>")
	assert.True(t, collection(t, e, admin).Active)
}

// Включённый тумблер с пустым текстом - ошибка настройки: гейт в этом состоянии
// пропускает всех, значит и сбор не идёт.
func TestPDConsentCollection_InactiveWhenTextEmpty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, "").Code)

	assert.False(t, collection(t, e, admin).Active)
}

// Сводка отдаёт поимённый список работников с организациями, то есть это просмотр
// персональных данных - обращение к ней обязано попадать в журнал 152-ФЗ наравне с
// прочими. Без записи в журнале обещание, данное заказчику в документации, не
// выполняется, а сама правка списка путей никак иначе не проверяется.
func TestPDConsentCollection_WritesPDAuditRow(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")

	require.NotNil(t, collection(t, e, admin))

	row := waitPDAuditRow(t, db, "/api"+collectionPath)
	assert.Equal(t, "pd_consent_collection", row.Resource)
	assert.Equal(t, "view", row.Action)
	assert.Equal(t, http.StatusOK, row.StatusCode)
	assert.NotNil(t, row.UserID)
}

// Сводка - административные данные о людях, обычному пользователю она закрыта.
func TestPDConsentCollection_RequiresAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	user := testutil.RegisterAndLogin(t, e, "coll_plain", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, collectionPath, testutil.AuthHeader(user))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}
