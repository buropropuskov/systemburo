package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaintenance_PublicStatus_Default(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	// Публичный endpoint без auth.
	rec := testutil.GET(t, e, "/settings/maintenance", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	m := testutil.ParseMap(t, rec)
	assert.Equal(t, false, m["enabled"])
}

func TestMaintenance_AdminGet_Forbidden_ForRegularUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "mt_regular", "pass12345", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/admin/maintenance", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestMaintenance_Toggle_AdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	start := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(3 * time.Hour).Format(time.RFC3339)

	// Enable с объявленным окном и контактами поддержки.
	body := fmt.Sprintf(`{"enabled":true,"message":"БД-миграция 1.5.0",
		"planned_start":%q,"planned_end":%q,
		"support_email":"support@buropropuskov.ru","support_phone":"+7 495 123-45-67"}`, start, end)
	rec := testutil.PUT(t, e, "/admin/maintenance", body, h)
	require.Equal(t, http.StatusOK, rec.Code)
	m := testutil.ParseMap(t, rec)
	assert.Equal(t, true, m["enabled"])
	assert.Equal(t, "БД-миграция 1.5.0", m["message"])
	assert.Equal(t, start, m["planned_start"])
	assert.Equal(t, end, m["planned_end"])

	// Публичный endpoint отдаёт окно и оба контакта - страница техработ
	// строит из них срок работ и ссылки на поддержку.
	rec = testutil.GET(t, e, "/settings/maintenance", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	m = testutil.ParseMap(t, rec)
	assert.Equal(t, true, m["enabled"])
	assert.Equal(t, "support@buropropuskov.ru", m["support_email"])
	assert.Equal(t, "+7 495 123-45-67", m["support_phone"])
	assert.Equal(t, end, m["planned_end"])

	// Disable
	body = `{"enabled":false}`
	rec = testutil.PUT(t, e, "/admin/maintenance", body, h)
	require.Equal(t, http.StatusOK, rec.Code)
	m = testutil.ParseMap(t, rec)
	assert.Equal(t, false, m["enabled"])
}

// Кривое окно не должно сохраняться: иначе страница техработ показала бы
// пользователям срок окончания раньше начала или прогресс, который не считается.
func TestMaintenance_Toggle_RejectsInvalidWindow(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	start := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(3 * time.Hour).Format(time.RFC3339)

	cases := map[string]string{
		"окончание раньше начала": fmt.Sprintf(`{"enabled":true,"planned_start":%q,"planned_end":%q}`, end, start),
		"окончание равно началу":  fmt.Sprintf(`{"enabled":true,"planned_start":%q,"planned_end":%q}`, start, start),
		"задана только половина":  fmt.Sprintf(`{"enabled":true,"planned_start":%q}`, start),
		"не дата": `{"enabled":true,"planned_start":"вчера","planned_end":"завтра"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			rec := testutil.PUT(t, e, "/admin/maintenance", body, h)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}

	// Ни одна из отклонённых попыток не включила режим.
	rec := testutil.GET(t, e, "/settings/maintenance", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, false, testutil.ParseMap(t, rec)["enabled"])
}

// Режим снимается сам, когда объявленное окно закончилось: страховка на случай
// потери доступа супер-админом. Событие аудита пишется один раз.
func TestMaintenance_AutoDisabled_AfterWindowExpired(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	start := time.Now().UTC().Add(-3 * time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	body := fmt.Sprintf(`{"enabled":true,"message":"вчерашние работы","planned_start":%q,"planned_end":%q}`, start, end)

	rec := testutil.PUT(t, e, "/admin/maintenance", body, h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, false, testutil.ParseMap(t, rec)["enabled"],
		"истёкшее окно не должно оставлять режим включённым")

	// Повторные обращения к публичному статусу не должны плодить события.
	for i := 0; i < 3; i++ {
		rec = testutil.GET(t, e, "/settings/maintenance", nil)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, false, testutil.ParseMap(t, rec)["enabled"])
	}

	var flag string
	db.Raw(`SELECT value FROM system_settings WHERE key = 'maintenance.enabled'`).Scan(&flag)
	assert.Equal(t, "false", flag, "флаг режима должен быть погашен в БД")

	var autoDisabledEvents int64
	db.Raw(`SELECT COUNT(*) FROM auth_events WHERE event_type = 'maintenance_auto_disabled'`).Scan(&autoDisabledEvents)
	assert.Equal(t, int64(1), autoDisabledEvents, "авто-снятие пишется в аудит ровно один раз")
}

// Enable maintenance должен revoke refresh_tokens всех не-админов. После этого
// их /refresh-token будет 401, они попадут на /login -> 503.
func TestMaintenance_Enable_RevokesNonAdminRefreshTokens(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Обычный юзер - логинимся, получаем refresh.
	testutil.RegisterAndLogin(t, e, "mt_regular2", "pass12345", 1, td.OrgID, td.CompanyID)
	// Супер-админ - включает maintenance.
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	body := `{"enabled":true,"message":"test"}`
	rec := testutil.PUT(t, e, "/admin/maintenance", body, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	// Проверяем что refresh_tokens обычного юзера помечены revoked.
	var nonAdminRevoked int64
	db.Raw(`
		SELECT COUNT(*) FROM refresh_tokens rt
		JOIN users u ON rt.user_id = u.id
		WHERE u.type_id != 6 AND rt.is_revoked = true
	`).Scan(&nonAdminRevoked)
	assert.Greater(t, nonAdminRevoked, int64(0), "не-админские refresh_tokens должны быть revoked")

	// Админские refresh_tokens должны остаться активными.
	var adminActive int64
	db.Raw(`
		SELECT COUNT(*) FROM refresh_tokens rt
		JOIN users u ON rt.user_id = u.id
		WHERE u.type_id = 6 AND rt.is_revoked = false
	`).Scan(&adminActive)
	assert.Greater(t, adminActive, int64(0), "админские refresh_tokens не должны быть revoked")
}

// При включённом maintenance обычный юзер получает 503 на /login даже с
// правильными credentials. Super-admin (type_id=6) проходит.
func TestMaintenance_Login_BlockedForRegular_AllowedForAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Создаём обычного юзера и супер-админа, но не логинимся сразу -
	// делаем это после включения режима.
	testutil.RegisterUser(t, e, "mt_user", "pass12345", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "mt_admin", "pass12345", 6, td.OrgID, td.CompanyID)

	// Ещё один админ для включения maintenance (текущему mt_admin тоже пойдёт).
	initialAdminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	body := `{"enabled":true,"message":"work"}`
	rec := testutil.PUT(t, e, "/admin/maintenance", body, testutil.AuthHeader(initialAdminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	// Обычный юзер пытается залогиниться - 503
	loginReq := `{"username":"mt_user","password":"pass12345"}`
	rec = testutil.POST(t, e, "/login", loginReq, nil)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code,
		"обычный юзер должен получать 503 на /login во время maintenance")

	// Super-admin логинится успешно
	loginReq = `{"username":"mt_admin","password":"pass12345"}`
	rec = testutil.POST(t, e, "/login", loginReq, nil)
	assert.Equal(t, http.StatusOK, rec.Code, "super-admin должен логиниться даже во время maintenance")
}
