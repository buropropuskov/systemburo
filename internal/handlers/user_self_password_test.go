package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	selfPassOld = "oldpassword12345"
	selfPassNew = "newpassword67890"
)

// TestChangeOwnPassword_HappyPath: обычный работник без административных прав
// меняет свой пароль и после этого входит только по новому. До #1915 такого пути
// не существовало вовсе - пароль менял лишь администратор.
func TestChangeOwnPassword_HappyPath(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "selfpass_user", selfPassOld, 1, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+selfPassOld+`","new_password":"`+selfPassNew+`"}`,
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Старый пароль перестал пускать, новый пускает.
	oldRec := testutil.POST(t, e, "/login", `{"username":"selfpass_user","password":"`+selfPassOld+`"}`, nil)
	assert.NotEqual(t, http.StatusOK, oldRec.Code, "старый пароль не должен пускать")

	newRec := testutil.POST(t, e, "/login", `{"username":"selfpass_user","password":"`+selfPassNew+`"}`, nil)
	assert.Equal(t, http.StatusOK, newRec.Code, newRec.Body.String())
}

// TestChangeOwnPassword_WrongCurrent: без верного текущего пароля смена не проходит.
// Это главный предохранитель ручки: перехваченная сессия иначе даёт захват учётной записи.
func TestChangeOwnPassword_WrongCurrent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "selfpass_wrong", selfPassOld, 1, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"definitely_not_it","new_password":"`+selfPassNew+`"}`,
		testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Пароль остался прежним.
	loginRec := testutil.POST(t, e, "/login", `{"username":"selfpass_wrong","password":"`+selfPassOld+`"}`, nil)
	assert.Equal(t, http.StatusOK, loginRec.Code)

	// Неудачная попытка попала в историю входов как отказ.
	var failed int64
	require.NoError(t, db.Model(&models.AuthEvent{}).
		Where("event_type = ? AND success = ?", models.AuthEventPasswordChanged, false).
		Count(&failed).Error)
	assert.EqualValues(t, 1, failed, "неудачная попытка смены пароля должна попасть в auth_events")
}

// TestChangeOwnPassword_SameAsCurrent: повтор текущего пароля отклоняется, иначе
// обязательная смена после плановой рассылки (#1911) обходится в один клик.
func TestChangeOwnPassword_SameAsCurrent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "selfpass_same", selfPassOld, 1, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+selfPassOld+`","new_password":"`+selfPassOld+`"}`,
		testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "совпадает")
}

// TestChangeOwnPassword_PolicyViolation: новый пароль проходит ту же политику,
// что и заданный администратором.
func TestChangeOwnPassword_PolicyViolation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "selfpass_policy", selfPassOld, 1, td.OrgID, td.CompanyID)

	// 4 символа - короче дефолтного минимума политики (8).
	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+selfPassOld+`","new_password":"ab1c"}`,
		testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestChangeOwnPassword_TouchesOnlySelf: ручка работает с учёткой из маркера
// доступа, чужой пароль через неё не сменить - имени пользователя в ней нет вовсе.
func TestChangeOwnPassword_TouchesOnlySelf(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "selfpass_owner", selfPassOld, 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "selfpass_neighbour", selfPassOld, 1, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+selfPassOld+`","new_password":"`+selfPassNew+`"}`,
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Пароль соседа не тронут.
	neighbourRec := testutil.POST(t, e, "/login",
		`{"username":"selfpass_neighbour","password":"`+selfPassOld+`"}`, nil)
	assert.Equal(t, http.StatusOK, neighbourRec.Code)
}

// TestChangeOwnPassword_RequiresAuth: без маркера доступа ручка недоступна.
func TestChangeOwnPassword_RequiresAuth(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"whatever","new_password":"`+selfPassNew+`"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestChangeOwnPassword_KeepsCurrentSession: человек, сменивший пароль сам, не
// должен вылетать из системы. Он только что подтвердил личность текущим паролем,
// и выкидывать его на форму входа - раздражение без выигрыша в безопасности.
// Остальные сессии при этом гаснут, поэтому угнанная на другом устройстве умирает.
func TestChangeOwnPassword_KeepsCurrentSession(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const old = "oldpassword12345"
	testutil.RegisterUser(t, e, "keepsess_user", old, 1, td.OrgID, td.CompanyID)
	access, refresh := testutil.LoginUser(t, e, "keepsess_user", old)
	require.NotEmpty(t, refresh, "вход должен положить маркер продления в cookie")

	// Вторая сессия того же работника - её смена пароля обязана погасить.
	_, otherRefresh := testutil.LoginUser(t, e, "keepsess_user", old)
	require.NotEmpty(t, otherRefresh)

	headers := testutil.AuthHeader(access)
	headers.Add("Cookie", "refresh_token="+refresh)

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+old+`","new_password":"freshpassword678"}`, headers)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Сессия, из которой меняли, продолжает продлеваться.
	refreshRec := testutil.POST(t, e, "/api/refresh-token", `{}`, func() http.Header {
		h := http.Header{}
		h.Set("Cookie", "refresh_token="+refresh)
		return h
	}())
	assert.Equal(t, http.StatusOK, refreshRec.Code, "текущая сессия должна пережить смену пароля: %s", refreshRec.Body.String())

	// А вторая - нет.
	otherRec := testutil.POST(t, e, "/api/refresh-token", `{}`, func() http.Header {
		h := http.Header{}
		h.Set("Cookie", "refresh_token="+otherRefresh)
		return h
	}())
	assert.NotEqual(t, http.StatusOK, otherRec.Code, "остальные сессии должны погаснуть")
}
