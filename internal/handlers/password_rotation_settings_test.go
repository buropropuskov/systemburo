package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"systemburo/internal/config"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRotationSettings_Validation: границы периодичности. Верхняя граница не
// косметическая - приказ ФСТЭК России N 21 требует смены не реже чем раз в 120
// суток, и «раз в три года» в системе, аттестуемой как ИСПДн, недопустимо.
func TestRotationSettings_Validation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	// Возвращаем значения обратно: настройки живут в кэше процесса и переживают
	// чистку базы, поэтому оставленный включённым режим красит соседние тесты.
	defer func() {
		testutil.PUT(t, e, "/api/settings/password.rotation_enabled", `{"value":"false"}`, h)
		testutil.PUT(t, e, "/api/settings/password.rotation_days", `{"value":"90"}`, h)
		testutil.PUT(t, e, "/api/settings/password.rotation_notify_days_before", `{"value":"7"}`, h)
	}()

	cases := []struct {
		key   string
		value string
		ok    bool
	}{
		{"password.rotation_days", "90", true},
		{"password.rotation_days", "30", true},
		{"password.rotation_days", "120", true},
		{"password.rotation_days", "29", false},
		{"password.rotation_days", "121", false},
		{"password.rotation_days", "неделя", false},
		{"password.rotation_notify_days_before", "0", true},
		{"password.rotation_notify_days_before", "30", true},
		{"password.rotation_notify_days_before", "31", false},
		{"password.rotation_enabled", "true", true},
		{"password.rotation_enabled", "да", false},
		{"password.force_change_on_next_login", "false", true},
	}
	for _, tc := range cases {
		t.Run(tc.key+"="+tc.value, func(t *testing.T) {
			rec := testutil.PUT(t, e, "/api/settings/"+tc.key, `{"value":"`+tc.value+`"}`, h)
			if tc.ok {
				assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			} else {
				assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestRotationSettings_InPolicyResponse: настройки ротации приезжают тем же
// ответом, что и требования к символам. Форма смены пароля показывает человеку не
// только «нужна цифра», но и то, что пароль придётся менять раз в N суток.
func TestRotationSettings_InPolicyResponse(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "rotation_policy_reader", "password12345678", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/api/settings/password-policy", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.Contains(t, body, "rotation_enabled")
	assert.Contains(t, body, "rotation_days")
	assert.Contains(t, body, "force_change_on_next_login")
}

// TestRotationStatus_CountsUsers: числа на экране считаются по тем же условиям, по
// которым будет отбирать работников сам прогон. Разойдутся - администратор увидит
// одно, а произойдёт другое.
//
// Считаем приращение, а не абсолютные значения: в базе уже живут сидовые учётные
// записи и администратор, заведённый самим тестом, и привязка к их числу сделала
// бы тест ложно-красным при любой правке сидов.
func TestRotationStatus_CountsUsers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(adminToken)

	readStatus := func() map[string]any {
		rec := testutil.GET(t, e, "/api/settings/password-rotation/status", h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return testutil.ParseMap(t, rec)
	}
	num := func(m map[string]any, key string) int {
		v, ok := m[key].(float64)
		require.True(t, ok, "поле %q отсутствует в ответе: %v", key, m)
		return int(v)
	}

	before := readStatus()

	longAgo := time.Now().AddDate(0, 0, -200)
	recent := time.Now().AddDate(0, 0, -1)
	mk := func(username, email string, changedAt time.Time, active, banned bool) {
		u := models.User{
			Username: username, Password: "x", TypeID: 1,
			OrganizationID: &td.OrgID, CompanyID: &td.CompanyID,
			IsActive: active, IsBanned: banned, PasswordChangedAt: &changedAt,
		}
		if email != "" {
			u.Email = &email
		}
		require.NoError(t, db.Create(&u).Error)
		// is_active=false - нулевое значение, а у поля стоит default:true, поэтому
		// gorm его при вставке пропускает и запись выходит активной. Дописываем
		// флаги отдельным обновлением, иначе «архивный» работник тихо считается
		// действующим и попадает в число затронутых сменой.
		require.NoError(t, db.Model(&models.User{}).Where("id = ?", u.ID).
			Updates(map[string]any{"is_active": active, "is_banned": banned}).Error)
	}

	mk("rot_expired_one", "one@example.org", longAgo, true, false)
	mk("rot_expired_two", "two@example.org", longAgo, true, false)
	mk("rot_fresh", "three@example.org", recent, true, false)
	mk("rot_no_email", "", longAgo, true, false)
	// Архивный и заблокированный не должны попасть никуда: плановая проверка их не
	// касается, им и входить некуда.
	mk("rot_archived", "four@example.org", longAgo, false, false)
	mk("rot_banned", "five@example.org", longAgo, true, true)

	after := readStatus()

	assert.Equal(t, 3, num(after, "expired")-num(before, "expired"),
		"срок вышел у троих действующих, адрес почты тут ни при чём - писем прогон не шлёт")
	assert.Equal(t, 1, num(after, "without_email")-num(before, "without_email"))
	assert.Equal(t, 3, num(after, "eligible")-num(before, "eligible"),
		"ручному обновлению доступны активные с почтой: два просроченных и один свежий")

	// Почта в тестовом приложении не настроена. Плановой проверке она не нужна, но
	// интерфейс обязан знать, что предупреждения заранее и ручное обновление
	// паролей сейчас недоступны.
	assert.Equal(t, false, after["mail_configured"])
	assert.NotEmpty(t, after["next_run_at"])
}

// TestRotationStatus_RequiresPermission: числа по учётным записям и состояние
// почты - административные сведения.
func TestRotationStatus_RequiresPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "rotation_plain_user", "password12345678", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/api/settings/password-rotation/status", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestRotationStatus_NextRunIsFuture: ближайший прогон всегда в будущем, иначе
// на экране висела бы прошедшая дата.
func TestRotationStatus_NextRunIsFuture(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	// Конфигурация нужна сервису настроек для дефолтов загрузки файлов; для
	// проверки сроков важны только ключи password.*, они не из конфигурации.
	cfg := &config.Config{UploadMaxFileSize: 10485760}
	svc := services.NewPasswordRotationStatusService(db, services.NewSettingsService(db, cfg), nil, time.UTC)
	status, err := svc.Get(t.Context())
	require.NoError(t, err)
	assert.True(t, status.NextRunAt.After(time.Now()), "ближайший прогон должен быть в будущем")
	assert.Equal(t, services.RotationRunHour, status.NextRunAt.Hour())
	assert.False(t, status.MailConfigured, "без почтового сервиса почта считается ненастроенной")
}
