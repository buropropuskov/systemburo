package handlers_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	gatePath   = "/consents/gate"
	acceptPath = "/consents/accept"
)

// enableConsent задаёт текст согласия и включает его запрос от имени администратора.
func enableConsent(t *testing.T, e *echo.Echo, admin, html string) {
	t.Helper()
	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, html).Code)
	rec := testutil.PUT(t, e, pdConsentPath+"/required", `{"required":true}`, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func gateState(t *testing.T, e *echo.Echo, token string) *models.PDConsentGateState {
	t.Helper()
	rec := testutil.GET(t, e, gatePath, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return testutil.ParseResponse[*models.PDConsentGateState](t, rec)
}

func TestConsentGate_NotRequiredWhenDisabled(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	user := testutil.RegisterAndLogin(t, e, "gate_off", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	state := gateState(t, e, user)
	assert.False(t, state.Required, "по умолчанию согласие не запрашивается")
	assert.Equal(t, 1, state.Version)
}

// Включённый тумблер с пустым текстом - ошибка настройки, а не повод закрыть систему.
// Сервер такое включение отклоняет, но если ключ окажется выставлен иначе, гейт
// обязан пропускать: показать пользователю всё равно нечего.
func TestConsentGate_NotRequiredWhenTextEmpty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, "").Code)

	user := testutil.RegisterAndLogin(t, e, "gate_empty", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	assert.False(t, gateState(t, e, user).Required)
}

func TestConsentGate_RequiredForUserWithoutConsent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	const html = "<p>Я согласен на обработку персональных данных.</p>"
	enableConsent(t, e, admin, html)

	user := testutil.RegisterAndLogin(t, e, "gate_need", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	state := gateState(t, e, user)

	assert.True(t, state.Required)
	assert.Equal(t, html, state.Text, "фронт получает текст вместе со статусом")
	assert.Equal(t, 1, state.Version)
}

// Супер-администратор - аварийная дверь: даже при включённом запросе согласия он
// должен попадать в интерфейс, иначе битую настройку нечем починить.
func TestConsentGate_SuperAdminNeverRequired(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")

	assert.False(t, gateState(t, e, admin).Required)
}

func TestConsentGate_AcceptStampsVersionAndHash(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	const html = "<p>Редакция для подписи</p>"
	enableConsent(t, e, admin, html)
	user := testutil.RegisterAndLogin(t, e, "gate_accept", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.False(t, testutil.ParseResponse[*models.PDConsentGateState](t, rec).Required,
		"после подтверждения доступ открывается сразу, а не по истечении TTL кэша")

	var consent models.PDConsent
	require.NoError(t, db.Where("consent_type = ?", "pd_processing").Order("id DESC").First(&consent).Error)
	sum := sha256.Sum256([]byte(html))
	assert.Equal(t, 1, consent.DocumentVersion)
	assert.Equal(t, hex.EncodeToString(sum[:]), consent.DocumentHash, "хэш принятого текста")
	assert.NotEmpty(t, consent.IPAddress)

	assert.False(t, gateState(t, e, user).Required)
}

// Редакцию штампует сервер: присланное в теле число игнорируется, иначе клиент
// освободился бы от всех будущих переподтверждений.
func TestConsentGate_AcceptIgnoresClientVersion(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	user := testutil.RegisterAndLogin(t, e, "gate_forge", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, acceptPath, `{"document_version":999,"consent_type":"pd_transfer"}`, testutil.AuthHeader(user))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var consent models.PDConsent
	require.NoError(t, db.Order("id DESC").First(&consent).Error)
	assert.Equal(t, 1, consent.DocumentVersion, "версия из настроек, не из тела запроса")
	assert.Equal(t, "pd_processing", consent.ConsentType, "тип тоже не из тела")
}

func TestConsentGate_VersionBumpRequiresConsentAgain(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Первая редакция</p>")
	user := testutil.RegisterAndLogin(t, e, "gate_bump", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user)).Code)
	require.False(t, gateState(t, e, user).Required)

	bump := testutil.POST(t, e, pdConsentPath+"/require-again", "{}", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, bump.Code, bump.Body.String())

	state := gateState(t, e, user)
	assert.True(t, state.Required, "прежнее согласие больше не устраивает")
	assert.Equal(t, 2, state.Version)

	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user)).Code)
	assert.False(t, gateState(t, e, user).Required)

	var consent models.PDConsent
	require.NoError(t, db.Order("id DESC").First(&consent).Error)
	assert.Equal(t, 2, consent.DocumentVersion)
}

// Правка текста без подъёма редакции согласие не сбрасывает: опечатка не должна
// заставлять всех подтверждать заново.
func TestConsentGate_TextEditKeepsConsent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие с опечаткай</p>")
	user := testutil.RegisterAndLogin(t, e, "gate_typo", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user)).Code)

	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, "<p>Согласие с опечаткой</p>").Code)

	assert.False(t, gateState(t, e, user).Required)
}

// Отзыв согласия обязан закрыть доступ сразу, а не по истечении TTL кэша: без
// сброса кэша пользователь оставался бы "согласившимся" до истечения окна.
func TestConsentGate_RevokeRequiresConsentAgain(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	user := testutil.RegisterAndLogin(t, e, "gate_revoke", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user)).Code)
	require.False(t, gateState(t, e, user).Required)

	require.Equal(t, http.StatusOK,
		testutil.DELETE(t, e, "/consents/pd_processing", testutil.AuthHeader(user)).Code)

	assert.True(t, gateState(t, e, user).Required)
}

// Запрос согласия могли выключить, пока пользователь читал текст. Клик по кнопке в
// этот момент не должен давать ошибку: подтверждать нечего, окно просто закрывается,
// и записи о согласии на пустое требование не появляется.
func TestConsentGate_AcceptWhenNotRequestedIsNoop(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	user := testutil.RegisterAndLogin(t, e, "gate_noreq", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, acceptPath, `{}`, testutil.AuthHeader(user))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.False(t, testutil.ParseResponse[*models.PDConsentGateState](t, rec).Required)

	var count int64
	require.NoError(t, db.Model(&models.PDConsent{}).Count(&count).Error)
	assert.Zero(t, count, "согласие на невыставленное требование не записывается")
}

func TestConsentGate_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	assert.Equal(t, http.StatusUnauthorized, testutil.GET(t, e, gatePath, nil).Code)
	assert.Equal(t, http.StatusUnauthorized, testutil.POST(t, e, acceptPath, `{}`, nil).Code)
}

// Согласие на передачу данных редакцию текста обработки не несёт: у него своя
// семантика, и подмешивать чужую редакцию в запись нельзя.
func TestConsentGate_TransferConsentKeepsVersionOne(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Согласие</p>")
	require.Equal(t, http.StatusOK, testutil.POST(t, e, pdConsentPath+"/require-again", "{}", testutil.AuthHeader(admin)).Code)
	user := testutil.RegisterAndLogin(t, e, "gate_transfer", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/consents", `{"consent_type":"pd_transfer"}`, testutil.AuthHeader(user))
	require.Equal(t, http.StatusOK, rec.Code)

	var consent models.PDConsent
	require.NoError(t, db.Where("consent_type = ?", "pd_transfer").First(&consent).Error)
	assert.Equal(t, 1, consent.DocumentVersion)
	assert.Empty(t, consent.DocumentHash)

	// И согласие на передачу не закрывает требование по обработке.
	assert.True(t, gateState(t, e, user).Required)
}

// Изменённый текст с require_again -- новая редакция: согласие, данное прежней,
// перестаёт быть достаточным, и пользователю показывают именно новый текст.
func TestConsentGate_TextChangeWithRequireAgain_AsksAnew(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Редакция 1</p>")

	user := testutil.RegisterAndLogin(t, e, "gate_reconsent", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(user)).Code)
	require.False(t, gateState(t, e, user).Required, "после подтверждения окно не показывается")

	require.Equal(t, http.StatusOK, savePDConsentTextRequiringAgain(t, e, admin, "<p>Редакция 2</p>").Code)

	after := gateState(t, e, user)
	assert.True(t, after.Required, "изменённый текст спрашивают заново")
	assert.Equal(t, "<p>Редакция 2</p>", after.Text, "и показывают именно новую редакцию")
}

// Без require_again правка текста согласие не отменяет: опечатка не повод поднимать
// окно у всех.
func TestConsentGate_TextChangeWithoutRequireAgain_KeepsConsent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsent(t, e, admin, "<p>Редакция 1</p>")

	user := testutil.RegisterAndLogin(t, e, "gate_keepconsent", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.POST(t, e, acceptPath, "{}", testutil.AuthHeader(user)).Code)

	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, "<p>Редакция 1 с опечаткой</p>").Code)

	assert.False(t, gateState(t, e, user).Required, "правка без require_again согласие не отменяет")
}
