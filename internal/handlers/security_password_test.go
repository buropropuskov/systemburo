package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Пароль работника поста ведёт бюро пропусков (#2280): своей формы смены у него
// нет, окно обязательной смены его не запирает, плановая ротация его не трогает.
// Проверки собраны здесь, а не разнесены по трём файлам: правило одно, и ломается
// оно целиком - стоит вернуть тип в любой из трёх выборок.

const (
	guardPassOld = "guardpassword12345"
	guardPassNew = "guardpassword67890"
)

// securityTypeID - идентификатор типа «Охранник» из справочника, а не константа:
// порядок засева не обещан, а код обещан (user_types.is_system).
func securityTypeID(t *testing.T, db *gorm.DB) int {
	t.Helper()
	var userType models.UserType
	require.NoError(t, db.Where("code = ?", "security").First(&userType).Error)
	return userType.ID
}

// TestSecurityGuard_PasswordGateLetsThrough: поднятый флаг обязательной смены не
// запирает работника поста. Флаг на его записи может стоять с тех пор, как учётку
// завели прежним порядком, - и без исключения пост упирался бы в форму, которую
// сервер ему всё равно не даёт пройти.
func TestSecurityGuard_PasswordGateLetsThrough(t *testing.T) {
	e, db, cleanup := testutil.SetupTestAppWithPasswordGate(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "guard_gate_user", guardPassOld,
		securityTypeID(t, db), td.OrgID, td.CompanyID)
	require.NoError(t, db.Exec(
		"UPDATE users SET must_change_password = true WHERE username = ?", "guard_gate_user").Error)

	rec := testutil.GET(t, e, "/api/citizenships", testutil.AuthHeader(token))
	assert.NotEqual(t, "1", rec.Header().Get("X-Password-Change-Required"),
		"работника поста окно смены пароля не запирает")
	assert.NotContains(t, rec.Body.String(), "PASSWORD_CHANGE_REQUIRED")
}

// TestSecurityGuard_CannotChangeOwnPassword: запрос мимо интерфейса тоже
// отклоняется. Кнопку в кабинете охраннику не рисуют, но проверка стоит на сервере -
// спрятанная кнопка не защита.
func TestSecurityGuard_CannotChangeOwnPassword(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "guard_self_pass", guardPassOld,
		securityTypeID(t, db), td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+guardPassOld+`","new_password":"`+guardPassNew+`"}`,
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// Пароль остался прежним: вход по старому проходит, по новому - нет.
	oldRec := testutil.POST(t, e, "/login",
		`{"username":"guard_self_pass","password":"`+guardPassOld+`"}`, nil)
	assert.Equal(t, http.StatusOK, oldRec.Code, oldRec.Body.String())
	newRec := testutil.POST(t, e, "/login",
		`{"username":"guard_self_pass","password":"`+guardPassNew+`"}`, nil)
	assert.NotEqual(t, http.StatusOK, newRec.Code, "новый пароль не должен был примениться")
}

// TestCreateUser_SecurityWithoutForcedChange: новой учётной записи поста флаг
// обязательной смены не поднимают, обычному работнику - поднимают, как и прежде.
func TestCreateUser_SecurityWithoutForcedChange(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	cases := []struct {
		name     string
		username string
		typeID   int
		forced   bool
	}{
		{"работник поста", "guard_created", securityTypeID(t, db), false},
		{"обычный работник", "plain_created", 1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"username":%q,"password":"createdpassword123",`+
				`"type_id":%d,"organization_id":%d,"company_id":%d,"last_name":"Проверка"}`,
				tc.username, tc.typeID, td.OrgID, td.CompanyID)
			rec := testutil.POST(t, e, "/api/users", body, testutil.AuthHeader(admin))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			var created models.User
			require.NoError(t, db.Where("username = ?", tc.username).First(&created).Error)
			assert.Equal(t, tc.forced, created.MustChangePassword)
		})
	}
}

// TestRotation_SkipsSecurity: плановая смена паролей проходит мимо поста. Иначе
// работник упёрся бы в форму смены посреди смены - и пропускать было бы некому.
func TestRotation_SkipsSecurity(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	long := time.Now().AddDate(0, 0, -200)

	guard := models.User{
		Username: "rot_guard", Password: "старый-хэш", TypeID: securityTypeID(t, db),
		OrganizationID: &td.OrgID, CompanyID: &td.CompanyID, PasswordChangedAt: &long,
	}
	require.NoError(t, db.Create(&guard).Error)
	plain := mkRotationUser(t, db, td, "rot_plain", "plain@example.org", long)

	_, err := svc.MarkExpired(context.Background())
	require.NoError(t, err)

	var afterGuard, afterPlain models.User
	require.NoError(t, db.First(&afterGuard, guard.ID).Error)
	require.NoError(t, db.First(&afterPlain, plain.ID).Error)
	assert.False(t, afterGuard.MustChangePassword, "пост в плановую смену не попадает")
	assert.True(t, afterPlain.MustChangePassword, "обычный работник по-прежнему попадает")
}
