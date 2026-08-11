package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Тесты запрета повторного пароля. Проверка стоит одного Argon2id на каждую
// прежний пароль, поэтому пароли здесь короткие по количеству: каждый лишний
// вызов смены - это ещё десятая доля секунды на прогон.

// usedPwdSvc - сервис пользователей поверх тестовой базы.
func usedPwdSvc(db *gorm.DB) services.UserService {
	return services.NewUserService(db, services.NewNotificationService(db))
}

// mkUsedPwdUser заводит работника ЧЕРЕЗ сервис - тем же путём, что и форма
// создания. Важно именно так: первый пароль запоминается вместе с учётной
// записью, и тесты повтора опираются на это.
func mkUsedPwdUser(t *testing.T, db *gorm.DB, td testutil.TestData, username, password string) models.User {
	t.Helper()
	require.NoError(t, usedPwdSvc(db).Create(context.Background(), 0, models.RegisterRequest{
		Username: username, Password: password, TypeID: 1,
		OrganizationID: td.OrgID, CompanyID: td.CompanyID,
	}))
	var u models.User
	require.NoError(t, db.Where("username = ?", username).First(&u).Error)
	return u
}

// mkBareUser заводит работника напрямую в базе, минуя сервис: тестам, которые
// сами наполняют перечень прежних паролей, нужен пустой перечень и известный хеш.
func mkBareUser(t *testing.T, db *gorm.DB, td testutil.TestData, username, password string) models.User {
	t.Helper()
	u := models.User{
		Username: username, Password: services.HashPassword(password), TypeID: 1,
		OrganizationID: &td.OrgID, CompanyID: &td.CompanyID,
	}
	require.NoError(t, db.Create(&u).Error)
	return u
}

// usedPasswordRows - прежние пароли работника, новые первыми.
func usedPasswordRows(t *testing.T, db *gorm.DB, userID int) []models.UsedPassword {
	t.Helper()
	var rows []models.UsedPassword
	require.NoError(t, db.Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").Find(&rows).Error)
	return rows
}

func currentPasswordHash(t *testing.T, db *gorm.DB, userID int) string {
	t.Helper()
	var u models.User
	require.NoError(t, db.First(&u, userID).Error)
	return u.Password
}

// TestUsedPasswords_AdminChangeRejectsReuse: администратор не может вернуть
// работнику пароль, который у того уже был. Ради этого правила перечень и заведён.
func TestUsedPasswords_AdminChangeRejectsReuse(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const first = "firstpassword12345"
	const second = "secondpassword6789"

	svc := usedPwdSvc(db)
	u := mkUsedPwdUser(t, db, td, "usedpwd_admin_reuse", first)

	require.NoError(t, svc.UpdatePassword(context.Background(), 0, u.Username,
		models.UpdatePasswordRequest{Password: second}, nil))
	afterSecond := currentPasswordHash(t, db, u.ID)

	err := svc.UpdatePassword(context.Background(), 0, u.Username,
		models.UpdatePasswordRequest{Password: first}, nil)
	require.Error(t, err, "повтор прежнего пароля должен отклоняться")
	assert.Contains(t, err.Error(), "уже использовался")

	assert.Equal(t, afterSecond, currentPasswordHash(t, db, u.ID),
		"отклонённая смена не должна трогать действующий пароль")
}

// TestUsedPasswords_SelfChangeRejectsReuse: то же правило на самостоятельной
// смене. Путь другой (ручка требует текущий пароль), запрет обязан быть тот же.
func TestUsedPasswords_SelfChangeRejectsReuse(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const first = "selffirstpass12345"
	const second = "selfsecondpass6789"

	mkUsedPwdUser(t, db, td, "usedpwd_self_reuse", first)
	token, _ := testutil.LoginUser(t, e, "usedpwd_self_reuse", first)

	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+first+`","new_password":"`+second+`"}`,
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+second+`","new_password":"`+first+`"}`,
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "уже использовался")

	// Отказ не должен ломать действующий пароль: вход по нему обязан работать.
	loginRec := testutil.POST(t, e, "/login",
		`{"username":"usedpwd_self_reuse","password":"`+second+`"}`, nil)
	assert.Equal(t, http.StatusOK, loginRec.Code, loginRec.Body.String())
}

// TestUsedPasswords_WrittenOnEveryPath: запись появляется на каждом пути -
// при заведении учётной записи, при смене администратором и при самостоятельной.
// Пропусти любой из них, и запрет на повтор станет дырявым именно там.
func TestUsedPasswords_WrittenOnEveryPath(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const created = "createdpass12345"
	const byAdmin = "adminsetpass6789"
	const bySelf = "selfsetpass01234"

	u := mkUsedPwdUser(t, db, td, "usedpwd_paths", created)
	rows := usedPasswordRows(t, db, u.ID)
	require.Len(t, rows, 1, "заведение учётной записи запоминает первый пароль")
	assert.Equal(t, currentPasswordHash(t, db, u.ID), rows[0].PasswordHash)

	require.NoError(t, usedPwdSvc(db).UpdatePassword(context.Background(), 0, u.Username,
		models.UpdatePasswordRequest{Password: byAdmin}, nil))
	rows = usedPasswordRows(t, db, u.ID)
	require.Len(t, rows, 2, "смена администратором запоминает пароль")
	assert.Equal(t, currentPasswordHash(t, db, u.ID), rows[0].PasswordHash)

	token, _ := testutil.LoginUser(t, e, u.Username, byAdmin)
	rec := testutil.PUT(t, e, "/api/users/me/password",
		`{"current_password":"`+byAdmin+`","new_password":"`+bySelf+`"}`,
		testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rows = usedPasswordRows(t, db, u.ID)
	require.Len(t, rows, 3, "самостоятельная смена запоминает пароль")
	assert.Equal(t, currentPasswordHash(t, db, u.ID), rows[0].PasswordHash)
}

// TestUsedPasswords_RotationRecordsIssuedPassword: плановая смена меняет пароль
// своим запросом, мимо UpdatePassword, и запоминание пароля там пришлось делать
// отдельно. Без неё работник смог бы задать себе пароль из письма ещё раз.
func TestUsedPasswords_RotationRecordsIssuedPassword(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	svc, _ := rotationEnv(t, db)
	u := mkRotationUser(t, db, td, "usedpwd_rotation", "rot@example.org", time.Now().AddDate(0, 0, -200))

	result, err := svc.Run(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, result.Changed)

	rows := usedPasswordRows(t, db, u.ID)
	require.Len(t, rows, 1, "плановая смена обязана оставить запись")
	assert.Equal(t, currentPasswordHash(t, db, u.ID), rows[0].PasswordHash,
		"запомнить должно именно тот хеш, который выдан работнику")
}

// TestUsedPasswords_BeyondDepthAllowed: пароль, ушедший за глубину проверки,
// снова разрешён. Глубина - осознанный компромисс: каждая запись стоит отдельного
// Argon2id, а проверка всех прежних заставила бы форму думать секундами.
func TestUsedPasswords_BeyondDepthAllowed(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const ancient = "ancientpassword12"
	const filler = "fillerpassword345"

	u := mkBareUser(t, db, td, "usedpwd_depth", "currentpass12345")

	// Древний пароль - самая старая запись, поверх него ровно десять свежих.
	// Хеш заполнителя считаем один раз: проверка всё равно перебирает записи, а
	// не пароли, и десять одинаковых хешей стоят столько же, сколько десять разных.
	base := time.Now().Add(-24 * time.Hour)
	require.NoError(t, db.Create(&models.UsedPassword{
		UserID: u.ID, PasswordHash: services.HashPassword(ancient), CreatedAt: base,
	}).Error)
	fillerHash := services.HashPassword(filler)
	for i := 1; i <= 10; i++ {
		require.NoError(t, db.Create(&models.UsedPassword{
			UserID: u.ID, PasswordHash: fillerHash, CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}).Error)
	}

	svc := usedPwdSvc(db)
	require.NoError(t, svc.UpdatePassword(context.Background(), 0, u.Username,
		models.UpdatePasswordRequest{Password: ancient}, nil),
		"пароль за пределами глубины проверки должен приниматься")

	// А заполнитель внутри глубины - по-прежнему нет.
	err := svc.UpdatePassword(context.Background(), 0, u.Username,
		models.UpdatePasswordRequest{Password: filler}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "уже использовался")
}

// TestUsedPasswords_TrimsBeyondDepth: за учётной записью не копится больше
// записей, чем проверяется. Хвост читать некому, а это хеши действовавших паролей.
func TestUsedPasswords_TrimsBeyondDepth(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	u := mkBareUser(t, db, td, "usedpwd_trim", "trimcurrent12345")

	// Заведомо негодные строки вместо хешей: разбор PHC отвергает их сразу, и
	// двенадцать записей не стоят двенадцати вычислений Argon2id.
	base := time.Now().Add(-24 * time.Hour)
	for i := 1; i <= 12; i++ {
		require.NoError(t, db.Create(&models.UsedPassword{
			UserID:       u.ID,
			PasswordHash: fmt.Sprintf("не-хеш-%02d", i),
			CreatedAt:    base.Add(time.Duration(i) * time.Minute),
		}).Error)
	}

	require.NoError(t, usedPwdSvc(db).UpdatePassword(context.Background(), 0, u.Username,
		models.UpdatePasswordRequest{Password: "trimmednewpass12"}, nil))

	rows := usedPasswordRows(t, db, u.ID)
	require.Len(t, rows, 10, "записей должно остаться ровно на глубину проверки")
	assert.Equal(t, currentPasswordHash(t, db, u.ID), rows[0].PasswordHash, "новый пароль - самая свежая запись")
	assert.Equal(t, "не-хеш-12", rows[1].PasswordHash, "срезается хвост, а не голова")
	assert.Equal(t, "не-хеш-04", rows[9].PasswordHash)
}

// TestUsedPasswords_RollbackTakesPasswordWithIt: пароль и запись о нём пишутся
// одной транзакцией. Проверяем в самую опасную сторону - сорванная запись прежнего пароля
// обязана отменить и смену пароля, иначе система разрешит вернуться к паролю,
// который сама же и поставила.
func TestUsedPasswords_RollbackTakesPasswordWithIt(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	u := mkBareUser(t, db, td, "usedpwd_rollback", "rollbackcurrent12")
	before := currentPasswordHash(t, db, u.ID)

	// Запись ломаем на стороне базы: NOT VALID проверяет только новые
	// строки, поэтому ограничение навешивается мгновенно и не трогает чужие.
	require.NoError(t, db.Exec(
		`ALTER TABLE used_passwords ADD CONSTRAINT usedpwd_test_block CHECK (false) NOT VALID`).Error)
	t.Cleanup(func() {
		db.Exec(`ALTER TABLE used_passwords DROP CONSTRAINT IF EXISTS usedpwd_test_block`)
	})

	err := usedPwdSvc(db).UpdatePassword(context.Background(), 0, u.Username,
		models.UpdatePasswordRequest{Password: "rollbacknewpass12"}, nil)
	require.Error(t, err, "сорванная запись прежнего пароля должна валить всю смену")

	assert.Equal(t, before, currentPasswordHash(t, db, u.ID),
		"пароль обязан откатиться вместе с записью")
	assert.Empty(t, usedPasswordRows(t, db, u.ID), "записи после отката остаться не должно")
}

// TestUsedPasswords_CascadesOnUserDelete: удалённая учётная запись не оставляет
// после себя набор своих хешей.
func TestUsedPasswords_CascadesOnUserDelete(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	u := mkBareUser(t, db, td, "usedpwd_cascade", "cascadepass12345")
	require.NoError(t, db.Create(&models.UsedPassword{
		UserID: u.ID, PasswordHash: "не-хеш-каскад",
	}).Error)
	require.Len(t, usedPasswordRows(t, db, u.ID), 1)

	require.NoError(t, db.Delete(&models.User{}, u.ID).Error)
	assert.Empty(t, usedPasswordRows(t, db, u.ID))
}
