package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettings_GetAll_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	rec := testutil.GET(t, e, "/settings", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSettings_GetAll_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regular_user", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/settings", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestSettings_GetAll_Admin: RegisterAdmin заводит буропропускова (type_id=6,
// is_super_admin=true) - тест проверяет ветку "супер-админ проходит" (#7).
func TestSettings_GetAll_Admin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/settings", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	data := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(data), 6)
}

// TestSettings_GetAll_PlainAdmin: обычный администратор (is_admin, НЕ супер)
// получает page.admin.settings через adminAll - ключ не super-only (#7). До этой
// правки читал бы 403 (checkSuper в settings_service.go требовал именно супера).
func TestSettings_GetAll_PlainAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterManager(t, e, "settings_plain_admin", td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/settings", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	data := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(data), 6)
}

// TestSettings_Update_PlainAdminAllowed: тот же обычный администратор пишет
// настройку, не только читает (#7).
func TestSettings_Update_PlainAdminAllowed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterManager(t, e, "settings_plain_writer", td.OrgID, td.CompanyID)
	rec := testutil.PUT(t, e, "/settings/upload.max_file_size", `{"value":"5242880"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// TestSettings_DeniedByPersonalOverride: точечный смысл задачи #7 - у КОНКРЕТНОГО
// администратора можно отобрать доступ личным deny-override, хотя adminAll по
// умолчанию page.admin.settings выдаёт. Гейт снимает и чтение, и запись.
func TestSettings_DeniedByPersonalOverride(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterManager(t, e, "settings_denied_admin", td.OrgID, td.CompanyID)
	userID := userIDByName(t, db, "settings_denied_admin")
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        userID,
		PermissionKey: "page.admin.settings",
		Value:         "deny",
	}).Error)

	rec := testutil.GET(t, e, "/settings", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	rec = testutil.PUT(t, e, "/settings/upload.max_file_size", `{"value":"5242880"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestSettings_Update_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.PUT(t, e, "/settings/upload.max_file_size", `{"value":"5242880"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestSettings_Update_InvalidKey(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.PUT(t, e, "/settings/nonexistent.key", `{"value":"test"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSettings_Update_InvalidValue(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.PUT(t, e, "/settings/upload.max_file_size", `{"value":"not-a-number"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestSettings_PublicContacts_NoAuth: контакты Бюро доступны публично (без JWT),
// дефолт пустой, после установки супер-админом отдаются актуальные значения.
func TestSettings_PublicContacts_NoAuth(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	rec := testutil.GET(t, e, "/settings/contacts", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, "", resp["phone"], "дефолт телефона пустой")
	assert.Equal(t, "", resp["email"], "дефолт почты пустой")

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, "/settings/contacts.bureau_phone", `{"value":"+7 (495) 123-45-67"}`, testutil.AuthHeader(token)).Code)
	require.Equal(t, http.StatusOK, testutil.PUT(t, e, "/settings/contacts.bureau_email", `{"value":"bureau@example.com"}`, testutil.AuthHeader(token)).Code)

	rec = testutil.GET(t, e, "/settings/contacts", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	resp = testutil.ParseMap(t, rec)
	assert.Equal(t, "+7 (495) 123-45-67", resp["phone"])
	assert.Equal(t, "bureau@example.com", resp["email"])
}

// TestSettings_UpdateContacts_Validation: некорректный email/слишком короткий
// телефон -> 400; корректные значения -> 200.
func TestSettings_UpdateContacts_Validation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	assert.Equal(t, http.StatusBadRequest, testutil.PUT(t, e, "/settings/contacts.bureau_email", `{"value":"not-an-email"}`, testutil.AuthHeader(token)).Code)
	assert.Equal(t, http.StatusBadRequest, testutil.PUT(t, e, "/settings/contacts.bureau_phone", `{"value":"123"}`, testutil.AuthHeader(token)).Code)
	assert.Equal(t, http.StatusOK, testutil.PUT(t, e, "/settings/contacts.bureau_email", `{"value":"ok@example.com"}`, testutil.AuthHeader(token)).Code)
	assert.Equal(t, http.StatusOK, testutil.PUT(t, e, "/settings/contacts.bureau_phone", `{"value":"+7 495 123 45 67"}`, testutil.AuthHeader(token)).Code)
}

func TestSettings_GetNotifications_ReturnsDurations(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	// Доступно любому авторизованному (не только супер-админу).
	token := testutil.RegisterAndLogin(t, e, "notif_reader", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/settings/notifications", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(10), resp["delete_duration"], "дефолт длительности удаления - 10 сек")
	assert.Equal(t, float64(5), resp["restore_duration"], "дефолт длительности восстановления - 5 сек")
}

func TestSettings_UpdateNotificationDuration_ValidPersists(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/settings/notifications.delete_duration", `{"value":"15"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = testutil.PUT(t, e, "/settings/notifications.restore_duration", `{"value":"3"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// GET отражает сохранённые значения.
	rec = testutil.GET(t, e, "/settings/notifications", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseMap(t, rec)
	assert.Equal(t, float64(15), resp["delete_duration"])
	assert.Equal(t, float64(3), resp["restore_duration"])
}

func TestSettings_UpdateNotificationDuration_OutOfRangeRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	for _, v := range []string{"2", "61", "abc"} {
		rec := testutil.PUT(t, e, "/settings/notifications.delete_duration", `{"value":"`+v+`"}`, testutil.AuthHeader(token))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "значение %q должно отклоняться (диапазон 3-60)", v)
	}
}

func TestSettings_GetUploadSettings_AuthUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regular", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/settings/upload", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	resp := testutil.ParseMap(t, rec)
	assert.NotNil(t, resp["max_file_size"])
	// Перечни типов из ответа убраны (#2000): настройкой они не задаются, формат
	// проверяется по сигнатуре файла. Замок держит и это, иначе поле вернётся молча.
	assert.NotContains(t, resp, "allowed_image_types")
	assert.NotContains(t, resp, "allowed_doc_types")
}

func TestGetPasswordPolicy(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "policy_reader", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/settings/password-policy", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)

	policy := testutil.ParseResponse[models.PasswordPolicy](t, rec)
	assert.Equal(t, 8, policy.MinLength)
	assert.True(t, policy.RequireDigit)
}

func TestSettings_PasswordPolicyValidation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// min_length вне диапазона -> 400
	rec := testutil.PUT(t, e, "/settings/password.min_length", `{"value":"3"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// валидный min_length -> 200
	rec = testutil.PUT(t, e, "/settings/password.min_length", `{"value":"10"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	// bool-ключ с мусором -> 400
	rec = testutil.PUT(t, e, "/settings/password.require_digit", `{"value":"maybe"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// валидный bool -> 200
	rec = testutil.PUT(t, e, "/settings/password.require_special", `{"value":"true"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
}
