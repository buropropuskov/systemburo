package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Два гейта вместе. По отдельности каждый проверен давно, а вместе они не стояли
// ни в одном тесте - и ровно там пряталась запертая с двух сторон дверь: смену
// пароля закрывал гейт согласия, принятие согласия закрывал парольный гейт.
// Работник с паролем из письма не попадал в систему вообще.
//
// Всплыло это, когда заведение учётной записи стало поднимать признак смены всем
// подряд: до того новичок упирался только в согласие, и связка не встречалась.
//
// Порядок для человека: сначала согласие, потом свой пароль.

// setupBothGates поднимает приложение с обоими гейтами и отдаёт вошедшего
// работника, которому нужно и согласиться, и сменить пароль.
func setupBothGates(t *testing.T, username string) (e2eGates, func()) {
	t.Helper()
	e, db, cleanup := testutil.SetupTestAppWithBothGates(t)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	enableConsentSettings(t, e, admin, "<p>Согласие на обработку данных</p>")
	token := testutil.RegisterAndLogin(t, e, username, mcpOldPassword, 1, td.OrgID, td.CompanyID)
	setPasswordFlag(t, db, username, true)
	return e2eGates{e: e, token: token, username: username}, cleanup
}

type e2eGates struct {
	e        *echo.Echo
	token    string
	username string
}

// Полный путь новичка: согласие, затем свой пароль, затем обычная работа. Каждый
// шаг обязан быть доступен в тот момент, когда до него доходит очередь.
func TestGates_NewUserPassesConsentThenPassword(t *testing.T) {
	g, cleanup := setupBothGates(t, "gates_newcomer")
	defer cleanup()

	// Пока не сделано ни то, ни другое, обычная ручка закрыта - это норма.
	require.Equal(t, http.StatusForbidden,
		testutil.GET(t, g.e, mcpGuardedPath, testutil.AuthHeader(g.token)).Code)

	// Шаг первый: окно согласия. Парольный гейт не должен его перебивать, иначе
	// человек уйдёт на форму пароля, которую закроет гейт согласия.
	assert.Equal(t, http.StatusOK,
		testutil.GET(t, g.e, gatePath, testutil.AuthHeader(g.token)).Code,
		"текст согласия обязан читаться: без него окну нечего показать")

	rec := testutil.POST(t, g.e, acceptPath, `{}`, testutil.AuthHeader(g.token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Шаг второй: свой пароль. Согласие уже дано, гейт согласия пропускает.
	rec = testutil.PUT(t, g.e, "/users/me/password",
		`{"current_password":"`+mcpOldPassword+`","new_password":"`+mcpNewPassword+`"}`,
		testutil.AuthHeader(g.token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Смена пароля отзывает сессии - работник входит заново уже своим паролем.
	token, _ := testutil.LoginUser(t, g.e, g.username, mcpNewPassword)
	assert.Equal(t, http.StatusOK,
		testutil.GET(t, g.e, mcpGuardedPath, testutil.AuthHeader(token)).Code,
		"после согласия и смены пароля система обязана открыться")
}

// Обратный порядок тоже не должен быть тупиком: если человек первым делом попал
// на форму пароля, отказ обязан объяснить, что сперва согласие, а не молча
// закрыться. Проверяем, что маркер отказа - именно согласия.
func TestGates_PasswordChangeBeforeConsentAsksForConsent(t *testing.T) {
	g, cleanup := setupBothGates(t, "gates_pwd_first")
	defer cleanup()

	rec := testutil.PUT(t, g.e, "/users/me/password",
		`{"current_password":"`+mcpOldPassword+`","new_password":"`+mcpNewPassword+`"}`,
		testutil.AuthHeader(g.token))
	assertConsentBlocked(t, rec)
}
