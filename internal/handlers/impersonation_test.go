package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// impersonate дёргает вход в режим и возвращает выданный маркер доступа.
func impersonate(t *testing.T, e *echo.Echo, actorToken string, targetID int) (string, *httptest.ResponseRecorder) {
	t.Helper()
	rec := testutil.POST(t, e, fmt.Sprintf("/users/%d/impersonate", targetID), "", testutil.AuthHeader(actorToken))
	if rec.Code != http.StatusOK {
		return "", rec
	}
	resp := testutil.ParseMap(t, rec)
	token, _ := resp["token"].(string)
	require.NotEmpty(t, token, "маркер доступа не выдан: %s", rec.Body.String())
	return token, rec
}

// auditRows считает записи журнала по паре (тип сущности, действие).
func auditRows(t *testing.T, db *gorm.DB, entityID int, action string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityUser, entityID, action).
		Count(&n).Error)
	return n
}

// TestImpersonate_RequiresPermission: без права user.impersonate вход от чужого
// имени отклоняется. Право - единственный ключ к режиму, поэтому проверка первая.
func TestImpersonate_RequiresPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	plainToken := testutil.RegisterAndLogin(t, e, "impnoright", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "imptarget1", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "imptarget1")

	_, rec := impersonate(t, e, plainToken, targetID)
	assert.Equal(t, http.StatusForbidden, rec.Code, "без права режим должен быть закрыт: %s", rec.Body.String())
	assert.Zero(t, auditRows(t, db, targetID, models.AuditActionImpersonateStart),
		"отклонённая попытка не должна оставлять запись о входе в режим")
}

// TestImpersonate_TokenCarriesInitiatorAndTarget: выданный маркер работает от имени
// цели, а инициатор в нём назван отдельно. Личность запроса обязана быть целевой -
// иначе доступ считался бы по администратору, и смотреть чужими глазами не вышло бы.
func TestImpersonate_TokenCarriesInitiatorAndTarget(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterManager(t, e, "impadmin2", td.OrgID, td.CompanyID)
	adminID := getUserID(t, db, "impadmin2")
	testutil.RegisterUser(t, e, "imptarget2", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "imptarget2")

	token, rec := impersonate(t, e, adminToken, targetID)
	require.Equal(t, http.StatusOK, rec.Code, "администратор должен входить от имени рядового пользователя: %s", rec.Body.String())

	claims, err := services.DecodeAccessToken(token, []byte(testutil.TestJWTSecret))
	require.NoError(t, err, "маркер режима должен проверяться тем же секретом, что и обычный")
	assert.Equal(t, targetID, claims.UserID, "личность маркера - цель, а не инициатор")
	require.NotNil(t, claims.ImpersonatedBy, "в маркере должен быть назван инициатор")
	assert.Equal(t, adminID, *claims.ImpersonatedBy)

	subject, _ := claims.GetSubject()
	assert.Equal(t, "imptarget2", subject)

	// Система под этим маркером отвечает как целевому пользователю.
	rec = testutil.GET(t, e, "/users/me", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "чтение под режимом должно работать: %s", rec.Body.String())
	me := testutil.ParseMap(t, rec)
	assert.Equal(t, "imptarget2", me["username"], "под режимом система должна отвечать как целевой учётной записи")
}

// TestImpersonate_NormalLoginTokenUnchanged: маркер обычного входа не приобретает
// признака режима. Отдельная проверка, потому что поле добавлено в общую структуру
// разбора: молчаливое появление инициатора у обычного маркера открыло бы гейт
// опасных действий там, где никакого режима нет.
func TestImpersonate_NormalLoginTokenUnchanged(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "impplain", "pass123", 1, td.OrgID, td.CompanyID)
	claims, err := services.DecodeAccessToken(token, []byte(testutil.TestJWTSecret))
	require.NoError(t, err)
	assert.Nil(t, claims.ImpersonatedBy, "обычный вход не должен помечаться режимом")
}

// TestImpersonate_MorePrivilegedTargetRejected: войти от имени более полномочного
// нельзя. Без этого барьера режим - готовый способ поднять себе полномочия.
func TestImpersonate_MorePrivilegedTargetRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	t.Run("super_admin_target", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		adminToken := testutil.RegisterManager(t, e, "imprank1", td.OrgID, td.CompanyID)
		testutil.RegisterUser(t, e, "impsuper", "pass123", 6, td.OrgID, td.CompanyID)
		superID := getUserID(t, db, "impsuper")

		_, rec := impersonate(t, e, adminToken, superID)
		assert.Equal(t, http.StatusForbidden, rec.Code,
			"от имени супер-администратора не входит никто: %s", rec.Body.String())
	})

	t.Run("admin_target_for_non_super_actor", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		actorToken := testutil.RegisterAndLogin(t, e, "imprank2", "pass123", 1, td.OrgID, td.CompanyID)
		testutil.GrantPermission(t, getUserID(t, db, "imprank2"), services.KeyUserImpersonate)

		testutil.RegisterManager(t, e, "imprank2admin", td.OrgID, td.CompanyID)
		adminID := getUserID(t, db, "imprank2admin")

		_, rec := impersonate(t, e, actorToken, adminID)
		assert.Equal(t, http.StatusForbidden, rec.Code,
			"войти от имени администратора может только супер-администратор: %s", rec.Body.String())
	})

	t.Run("target_with_extra_permission", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		actorToken := testutil.RegisterAndLogin(t, e, "imprank3", "pass123", 1, td.OrgID, td.CompanyID)
		testutil.GrantPermission(t, getUserID(t, db, "imprank3"), services.KeyUserImpersonate)

		testutil.RegisterUser(t, e, "imprank3target", "pass123", 1, td.OrgID, td.CompanyID)
		targetID := getUserID(t, db, "imprank3target")
		// У цели есть право, которого нет у инициатора: рангами они равны, но набор
		// прав у цели шире - вход дал бы инициатору чужой доступ.
		testutil.GrantPermission(t, targetID, services.KeyPageAdminFeedback)

		_, rec := impersonate(t, e, actorToken, targetID)
		assert.Equal(t, http.StatusForbidden, rec.Code,
			"цель с лишним правом закрыта: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), services.KeyPageAdminFeedback,
			"отказ должен называть право, из-за которого закрыт вход")
	})

	t.Run("self_rejected", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		adminToken := testutil.RegisterManager(t, e, "imprank4", td.OrgID, td.CompanyID)
		adminID := getUserID(t, db, "imprank4")

		_, rec := impersonate(t, e, adminToken, adminID)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "вход от собственного имени бессмыслен: %s", rec.Body.String())
	})
}

// TestImpersonate_DisabledAndBannedTargetRejected: войти от имени отключённой,
// заблокированной или обязанной сменить пароль учётной записи нельзя - она не
// пускает и собственного владельца, а сеанс вышел бы тупиком.
func TestImpersonate_DisabledAndBannedTargetRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterManager(t, e, "impstate", td.OrgID, td.CompanyID)

	testutil.RegisterUser(t, e, "impdisabled", "pass123", 1, td.OrgID, td.CompanyID)
	disabledID := getUserID(t, db, "impdisabled")
	require.NoError(t, db.Table("users").Where("id = ?", disabledID).Update("is_active", false).Error)

	testutil.RegisterUser(t, e, "impbanned", "pass123", 1, td.OrgID, td.CompanyID)
	bannedID := getUserID(t, db, "impbanned")
	require.NoError(t, db.Table("users").Where("id = ?", bannedID).Update("is_banned", true).Error)

	_, rec := impersonate(t, e, adminToken, disabledID)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "отключённая учётная запись закрыта: %s", rec.Body.String())

	_, rec = impersonate(t, e, adminToken, bannedID)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "заблокированная учётная запись закрыта: %s", rec.Body.String())

	testutil.RegisterUser(t, e, "impmustchange", "pass123", 1, td.OrgID, td.CompanyID)
	mustChangeID := getUserID(t, db, "impmustchange")
	require.NoError(t, db.Table("users").Where("id = ?", mustChangeID).Update("must_change_password", true).Error)

	_, rec = impersonate(t, e, adminToken, mustChangeID)
	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"учётная запись с назначенной сменой пароля закрыта - сеанс был бы тупиком: %s", rec.Body.String())
}

// TestImpersonate_DangerousActionsBlocked: под режимом закрыты действия, меняющие
// судьбу учётной записи. Смена своего пароля взята основной проверкой намеренно:
// на неё у целевого пользователя есть полное право, поэтому отказ доказывает
// работу гейта, а не отсутствие права.
func TestImpersonate_DangerousActionsBlocked(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterManager(t, e, "impblock", td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "impblocktgt", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "impblocktgt")

	token, rec := impersonate(t, e, adminToken, targetID)
	require.Equal(t, http.StatusOK, rec.Code, "вход в режим: %s", rec.Body.String())

	// Свой пароль вне режима меняется без всяких прав - значит отказ ниже даёт
	// именно гейт режима.
	ownPassword := `{"current_password":"pass123","new_password":"another_pass_456"}`
	rec = testutil.PUT(t, e, "/users/me/password", ownPassword, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code, "смена своего пароля под режимом закрыта: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Вернитесь в свою учётную запись")

	ownToken, _ := testutil.LoginUser(t, e, "impblocktgt", "pass123")
	require.NotEmpty(t, ownToken, "пароль не должен был смениться: владелец обязан входить прежним")

	blocked := []struct {
		name, method, path, body string
	}{
		{"чужой пароль", http.MethodPut, "/users/impblocktgt/password", `{"password":"whatever_123456"}`},
		{"удаление учётной записи", http.MethodDelete, "/users/impblocktgt", ""},
		{"признак администратора", http.MethodPut, fmt.Sprintf("/users/%d/admin", targetID), `{"is_admin":true}`},
		{"роль", http.MethodPut, fmt.Sprintf("/users/%d/role", targetID), `{"role_id":1}`},
		{"правка прав", http.MethodPut, fmt.Sprintf("/permissions/user/%d", targetID), `{"permissions":[]}`},
		{"снятие блокировки входа", http.MethodPost, "/users/impblocktgt/reset-lockout", ""},
		{"согласие на обработку данных", http.MethodPost, "/consents/accept", ""},
		{"рассылка нового пароля", http.MethodPost, "/users/impblocktgt/rotate-password", ""},
		{"смена паролей всем", http.MethodPost, "/settings/password-rotation/run", ""},
		{"цепочка режимов", http.MethodPost, fmt.Sprintf("/users/%d/impersonate", targetID), ""},
	}
	for _, tc := range blocked {
		t.Run(tc.name, func(t *testing.T) {
			var r *httptest.ResponseRecorder
			switch tc.method {
			case http.MethodPut:
				r = testutil.PUT(t, e, tc.path, tc.body, testutil.AuthHeader(token))
			case http.MethodPost:
				r = testutil.POST(t, e, tc.path, tc.body, testutil.AuthHeader(token))
			default:
				r = testutil.DELETE(t, e, tc.path, testutil.AuthHeader(token))
			}
			assert.Equal(t, http.StatusForbidden, r.Code, "%s под режимом закрыт: %s", tc.name, r.Body.String())
			assert.Contains(t, r.Body.String(), "Вернитесь в свою учётную запись",
				"отказ должен приходить от гейта режима, а не от проверки прав")
		})
	}

	// Гейт не должен быть глухой стеной: обычная запись, не касающаяся судьбы
	// учётной записи, под режимом проходит.
	rec = testutil.PUT(t, e, "/users/me/theme", `{"theme":"dark"}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code, "безобидная запись под режимом должна проходить: %s", rec.Body.String())
}

// TestImpersonate_AuditTrail: журнал знает и о входе в режим, и о выходе, и об
// инициаторе действий внутри окна. Ради этого режим и заводится взамен практики
// «администратор знает пароль работника».
func TestImpersonate_AuditTrail(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterManager(t, e, "impaudit", td.OrgID, td.CompanyID)
	adminID := getUserID(t, db, "impaudit")

	testutil.RegisterUser(t, e, "impaudittgt", "pass123", 1, td.OrgID, td.CompanyID)
	targetID := getUserID(t, db, "impaudittgt")
	makeApprover(t, db, "impaudittgt")

	senderToken := testutil.RegisterAndLogin(t, e, "impauditsnd", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	token, rec := impersonate(t, e, adminToken, targetID)
	require.Equal(t, http.StatusOK, rec.Code, "вход в режим: %s", rec.Body.String())

	// Вход записан на цель, актором назван инициатор.
	var start models.AuditLog
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityUser, targetID, models.AuditActionImpersonateStart).
		First(&start).Error, "записи о входе в режим нет")
	require.NotNil(t, start.ActorUserID)
	assert.Equal(t, adminID, *start.ActorUserID, "актором входа в режим должен быть инициатор")
	assert.Contains(t, string(start.Details), "impaudittgt", "запись должна называть, от чьего имени открыт сеанс")

	// Действие внутри окна: заявку принимает целевой пользователь, но запись
	// помнит инициатора.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID),
		fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, targetID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "принять заявку под режимом: %s", rec.Body.String())

	var inside models.AuditLog
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?",
			models.AuditEntityApplication, appID, models.AuditActionTakeToWork).
		First(&inside).Error, "действие внутри режима не попало в журнал")
	require.NotNil(t, inside.ActorUserID)
	assert.Equal(t, targetID, *inside.ActorUserID,
		"актором остаётся тот, под чьей учётной записью сделано действие")
	var details map[string]any
	require.NoError(t, json.Unmarshal(inside.Details, &details))
	assert.EqualValues(t, adminID, details["impersonated_by"],
		"действие внутри режима обязано называть инициатора")

	// Выход из режима.
	rec = testutil.POST(t, e, "/impersonation/stop", "", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "выход из режима: %s", rec.Body.String())
	assert.Equal(t, int64(1), auditRows(t, db, targetID, models.AuditActionImpersonateStop),
		"выход из режима должен быть записан")

	// Обычный маркер выйти из режима не может: выходить не из чего.
	rec = testutil.POST(t, e, "/impersonation/stop", "", testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, rec.Code,
		"вне режима выход бессмыслен и не должен плодить записи: %s", rec.Body.String())
	assert.Equal(t, int64(1), auditRows(t, db, targetID, models.AuditActionImpersonateStop))
}
