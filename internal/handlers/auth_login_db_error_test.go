package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// errUsersQueryDown - причина, которую подставляем вместо ответа базы. Текст взят
// похожим на настоящий отказ пула: именно он обязан доехать до журнала.
var errUsersQueryDown = errors.New("pq: sorry, too many clients already")

// breakUsersQuery поднимает поверх РАБОЧЕЙ тест-БД второе gorm-соединение, у которого
// выборка пользователей падает по флагу. Пул соединений общий (postgres.Config.Conn),
// новая только цепочка обработчиков - поэтому подмена не задевает ни соседние тесты,
// ни общую базу, которой пользуются параллельно.
//
// Полностью закрытое соединение тут не годится: с ним не сохранилась бы и запись
// журнала, а проверяем мы как раз её - что причина у аварии и у неверного логина разная.
func breakUsersQuery(t *testing.T, base *gorm.DB) (*gorm.DB, *bool) {
	t.Helper()

	sqlDB, err := base.DB()
	require.NoError(t, err)

	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)

	broken := false
	require.NoError(t, db.Callback().Query().Before("gorm:query").
		Register("test:break_users_query", func(tx *gorm.DB) {
			if !broken {
				return
			}
			if _, ok := tx.Statement.Dest.(*models.User); ok {
				tx.AddError(errUsersQueryDown)
			}
		}))

	return db, &broken
}

func newLoginService(db *gorm.DB) services.AuthService {
	return services.NewAuthService(db, testutil.TestJWTSecret, testutil.TestJWTRefreshSecret,
		15*time.Minute, 168*time.Hour)
}

// failuresBeforeLock спрашивает порог блокировки у самого кода вместо того, чтобы
// повторять константу в тесте: подними порог в auth_service - и повтор с зашитой
// пятёркой перестанет доказывать хоть что-нибудь, оставаясь зелёным.
//
// Меряем на своём сервисе, своём адресе и выдуманном логине: собственный счётчик
// в памяти и никакой записи в учётных записях - основной сценарий не задет.
func failuresBeforeLock(t *testing.T, db *gorm.DB) int {
	t.Helper()

	_, err := newLoginService(db).Login(context.Background(),
		models.LoginRequest{Username: "threshold-probe", Password: "x"},
		&services.RequestMeta{IPAddress: "203.0.113.250", UserAgent: "go-test"})

	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	left, convErr := strconv.Atoi(ae.Headers["X-Auth-Attempts-Remaining"])
	require.NoError(t, convErr, "остаток попыток обязан приходить числом")
	require.Positive(t, left)
	return left + 1
}

// TestLogin_DBFailureIsNotBadCredentials: недоступная выборка пользователя обязана
// отвечать не так, как опечатка в логине, и не копить лестницу блокировки. Иначе
// авария базы читается как «люди путают пароли», а после её починки те же люди
// упираются в блокировку за то, чего не делали.
func TestLogin_DBFailureIsNotBadCredentials(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "dbfailuser", "correctpass", 1, td.OrgID, td.CompanyID)

	wrapped, broken := breakUsersQuery(t, db)
	svc := newLoginService(wrapped)
	meta := &services.RequestMeta{IPAddress: "198.51.100.7", UserAgent: "go-test"}
	creds := models.LoginRequest{Username: "dbfailuser", Password: "correctpass"}

	// Ровно столько неудач запирает вход, если считать их неудачами человека.
	attempts := failuresBeforeLock(t, db)

	*broken = true
	for i := 0; i < attempts; i++ {
		_, err := svc.Login(context.Background(), creds, meta)
		require.Error(t, err)

		var ae *apperr.Error
		require.ErrorAs(t, err, &ae, "сбой базы обязан приходить доменной ошибкой, а не голым echo-ответом")
		assert.Equal(t, http.StatusInternalServerError, ae.Code,
			"клиент не должен думать, что дело в пароле")
		assert.NotContains(t, ae.Message, "Неверный логин или пароль")
		assert.NotContains(t, ae.Message, "Слишком много попыток")
		assert.Empty(t, ae.Headers["X-Auth-Attempts-Remaining"],
			"остаток попыток тут не при чём - попытки никто не тратил")
		assert.ErrorIs(t, err, errUsersQueryDown, "первопричина обязана доехать до логов")
	}

	// Счётчик учётной записи авария не двигает: до него исполнение не доходит.
	var user models.User
	require.NoError(t, db.Where("username = ?", "dbfailuser").First(&user).Error)
	assert.Equal(t, 0, user.FailedLoginCount)
	assert.Nil(t, user.LockedUntil)

	// Главное следствие: база вернулась - человек входит сразу, с того же адреса и
	// тем же логином. Если бы авария копила счётчик, тут был бы 429.
	*broken = false
	resp, err := svc.Login(context.Background(), creds, meta)
	require.NoError(t, err, "после аварии вход обязан открыться без отсидки")
	assert.NotEmpty(t, resp.Token)
}

// TestLogin_DBFailureAndMissingUserDifferInJournal: в журнале авария и несуществующий
// логин обязаны выглядеть по-разному. Запись «пользователь не найден» на сбое запроса -
// утверждение, которое никто не проверял.
func TestLogin_DBFailureAndMissingUserDifferInJournal(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "journaluser", "correctpass", 1, td.OrgID, td.CompanyID)

	wrapped, broken := breakUsersQuery(t, db)
	svc := newLoginService(wrapped)
	meta := &services.RequestMeta{IPAddress: "198.51.100.8", UserAgent: "go-test"}

	*broken = true
	_, err := svc.Login(context.Background(),
		models.LoginRequest{Username: "journaluser", Password: "correctpass"}, meta)
	require.Error(t, err)

	*broken = false
	_, err = svc.Login(context.Background(),
		models.LoginRequest{Username: "ghostuser", Password: "whatever"}, meta)
	require.Error(t, err)

	var outage models.AuthEvent
	require.NoError(t, db.Where("username = ?", "journaluser").
		Where("event_type = ?", models.AuthEventLoginError).First(&outage).Error,
		"сбой базы обязан оставлять собственный тип события")
	assert.False(t, outage.Success)
	assert.Nil(t, outage.UserID, "на сбое выборки пользователь не опознан")
	assert.Contains(t, outage.Detail, "user lookup")
	assert.Contains(t, outage.Detail, errUsersQueryDown.Error(), "в журнале настоящая причина")

	var missing models.AuthEvent
	require.NoError(t, db.Where("username = ?", "ghostuser").
		Where("event_type = ?", models.AuthEventLoginFailed).First(&missing).Error)
	assert.Equal(t, "user not found", missing.Detail)

	// Ни одной записи «неудачный вход» на аварии: именно эта подмена причины и
	// превращала недоступную базу в «люди вводят неправильные пароли».
	var failedOnOutage int64
	require.NoError(t, db.Model(&models.AuthEvent{}).
		Where("username = ? AND event_type = ?", "journaluser", models.AuthEventLoginFailed).
		Count(&failedOnOutage).Error)
	assert.Zero(t, failedOnOutage)
}
