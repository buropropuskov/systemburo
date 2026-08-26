package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pdConsentPath = "/settings/pd-consent"

// textBody собирает тело запроса на сохранение текста согласия с корректным
// экранированием: текст содержит HTML и кавычки.
func textBody(t *testing.T, html string) string {
	t.Helper()
	payload, err := json.Marshal(models.UpdatePDConsentTextRequest{Text: html})
	require.NoError(t, err)
	return string(payload)
}

// savePDConsentText сохраняет текст согласия от имени token, не двигая редакцию.
func savePDConsentText(t *testing.T, e *echo.Echo, token, html string) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.PUT(t, e, pdConsentPath+"/text", textBody(t, html), testutil.AuthHeader(token))
}

// savePDConsentTextRequiringAgain сохраняет текст и тем же запросом поднимает редакцию.
func savePDConsentTextRequiringAgain(t *testing.T, e *echo.Echo, token, html string) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(models.UpdatePDConsentTextRequest{Text: html, RequireAgain: true})
	require.NoError(t, err)
	return testutil.PUT(t, e, pdConsentPath+"/text", string(payload), testutil.AuthHeader(token))
}

func TestPDConsent_Get_DefaultsWhenNothingSet(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, pdConsentPath, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	got := testutil.ParseResponse[*models.PDConsentSettings](t, rec)
	require.NotNil(t, got)
	assert.Empty(t, got.Text)
	assert.Equal(t, 1, got.Version, "версия по умолчанию 1")
	assert.False(t, got.Required, "запрос согласия по умолчанию выключен")
}

// Ключевая проверка: секция настроек открыта по page.admin, поэтому обычный
// администратор (is_admin, НЕ супер) обязан сохранять текст без 403. Сохранение
// идёт через отдельный SetPDConsentText, а не через общий settingsService.Update
// (тот до #7 требовал супер-админа через checkSuper, сейчас гейтится точечным
// правом page.admin.settings на уровне роутера).
func TestPDConsent_SaveText_PlainAdminAllowed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterManager(t, e, "pdc_admin", td.OrgID, td.CompanyID)

	const html = "<p>Я согласен на обработку моих персональных данных &laquo;Бюро пропусков&raquo;.</p>"
	rec := savePDConsentText(t, e, admin, html)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	saved := testutil.ParseResponse[*models.PDConsentSettings](t, rec)
	require.NotNil(t, saved)
	assert.Equal(t, html, saved.Text)

	// Перечитываем отдельным запросом - значение действительно записано.
	reread := testutil.GET(t, e, pdConsentPath, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, reread.Code)
	assert.Equal(t, html, testutil.ParseResponse[*models.PDConsentSettings](t, reread).Text)
}

func TestPDConsent_RegularUserForbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	user := testutil.RegisterAndLogin(t, e, "pdc_plain", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	assert.Equal(t, http.StatusForbidden,
		testutil.GET(t, e, pdConsentPath, testutil.AuthHeader(user)).Code)
	assert.Equal(t, http.StatusForbidden,
		savePDConsentText(t, e, user, "<p>текст</p>").Code)
	assert.Equal(t, http.StatusForbidden,
		testutil.POST(t, e, pdConsentPath+"/require-again", "{}", testutil.AuthHeader(user)).Code)
}

// Включить запрос согласия с пустым текстом нельзя: настройка выглядела бы рабочей,
// а показать пользователю было бы нечего. Пустой абзац от редактора - тоже пусто.
func TestPDConsent_EnableRequired_RejectsEmptyText(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	for _, text := range []string{"", "<p></p>", "<p>   </p>", "<p>&nbsp;</p>"} {
		require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, text).Code)

		rec := testutil.PUT(t, e, pdConsentPath+"/required", `{"required":true}`, testutil.AuthHeader(admin))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "текст %q не должен позволять включение", text)

		state := testutil.GET(t, e, pdConsentPath, testutil.AuthHeader(admin))
		assert.False(t, testutil.ParseResponse[*models.PDConsentSettings](t, state).Required)
	}
}

func TestPDConsent_EnableAndDisableRequired(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, "<p>Согласие</p>").Code)

	on := testutil.PUT(t, e, pdConsentPath+"/required", `{"required":true}`, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, on.Code, on.Body.String())
	assert.True(t, testutil.ParseResponse[*models.PDConsentSettings](t, on).Required)

	off := testutil.PUT(t, e, pdConsentPath+"/required", `{"required":false}`, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, off.Code, off.Body.String())
	assert.False(t, testutil.ParseResponse[*models.PDConsentSettings](t, off).Required)
}

// Выключить запрос согласия должно быть можно всегда, в том числе когда текст уже
// стёрт: иначе настройку не снять после очистки текста.
func TestPDConsent_DisableRequired_WorksWithEmptyText(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, "<p>Согласие</p>").Code)
	require.Equal(t, http.StatusOK,
		testutil.PUT(t, e, pdConsentPath+"/required", `{"required":true}`, testutil.AuthHeader(admin)).Code)
	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, "").Code)

	rec := testutil.PUT(t, e, pdConsentPath+"/required", `{"required":false}`, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.False(t, testutil.ParseResponse[*models.PDConsentSettings](t, rec).Required)
}

func TestPDConsent_Required_MissingFieldRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, pdConsentPath+"/required", `{}`, testutil.AuthHeader(admin))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "поле required обязательно")
}

func TestPDConsent_SaveText_RejectsTooLarge(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	oversized := "<p>" + strings.Repeat("я", services.PDConsentTextMaxBytes) + "</p>"
	rec := savePDConsentText(t, e, admin, oversized)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "текст сверх лимита не должен сохраняться")
}

// Подъём версии - отдельное действие: правка опечатки в тексте не должна заставлять
// всех соглашаться заново, а кнопка "требовать повторно" должна.
func TestPDConsent_VersionMovesOnlyOnRequireAgain(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, "<p>Редакция 1</p>").Code)
	afterEdit := savePDConsentText(t, e, admin, "<p>Редакция 1 с опечаткой</p>")
	require.Equal(t, http.StatusOK, afterEdit.Code)
	assert.Equal(t, 1, testutil.ParseResponse[*models.PDConsentSettings](t, afterEdit).Version,
		"правка текста версию не двигает")

	bumped := testutil.POST(t, e, pdConsentPath+"/require-again", "{}", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, bumped.Code, bumped.Body.String())
	state := testutil.ParseResponse[*models.PDConsentSettings](t, bumped)
	assert.Equal(t, 2, state.Version)
	assert.Equal(t, "<p>Редакция 1 с опечаткой</p>", state.Text, "текст подъём версии не трогает")

	again := testutil.POST(t, e, pdConsentPath+"/require-again", "{}", testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, again.Code)
	assert.Equal(t, 3, testutil.ParseResponse[*models.PDConsentSettings](t, again).Version)
}

// Смена текста с require_again -- новая редакция того же согласия: текст и номер
// редакции обязаны меняться ОДНИМ запросом, иначе между двумя вызовами существует
// состояние «новый текст со старой редакцией», в котором система считает
// достаточным согласие, данное другому тексту.
func TestPDConsent_SaveText_RequireAgain_BumpsVersion(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, "<p>Редакция 1</p>").Code)

	rec := savePDConsentTextRequiringAgain(t, e, admin, "<p>Редакция 2</p>")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	state := testutil.ParseResponse[*models.PDConsentSettings](t, rec)
	assert.Equal(t, "<p>Редакция 2</p>", state.Text)
	assert.Equal(t, 2, state.Version, "редакция поднялась тем же запросом")
	assert.NotEmpty(t, state.VersionAt, "дата редакции проставлена")
}

// Слишком большой текст отвергается ДО подъёма редакции: иначе отказ сохранить
// оставлял бы всех переподтверждать прежний текст без причины.
func TestPDConsent_SaveText_RequireAgain_TooLargeKeepsVersion(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, savePDConsentText(t, e, admin, "<p>Редакция 1</p>").Code)

	oversized := "<p>" + strings.Repeat("я", services.PDConsentTextMaxBytes) + "</p>"
	require.Equal(t, http.StatusBadRequest, savePDConsentTextRequiringAgain(t, e, admin, oversized).Code)

	state := testutil.ParseResponse[*models.PDConsentSettings](t,
		testutil.GET(t, e, pdConsentPath, testutil.AuthHeader(admin)))
	assert.Equal(t, 1, state.Version, "отвергнутое сохранение редакцию не двигает")
	assert.Equal(t, "<p>Редакция 1</p>", state.Text)
}
