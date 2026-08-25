package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// errRefreshTokenQueryDown - причина, которую подставляем вместо ответа базы на
// SELECT по refresh_tokens.
var errRefreshTokenQueryDown = errors.New("pq: server closed the connection unexpectedly")

// errRefreshTokenWriteDown - то же самое, но для ЗАПИСИ (UPDATE/INSERT) refresh_tokens.
var errRefreshTokenWriteDown = errors.New("pq: could not extend file - no space left on device")

// newRefreshService - тот же сервис, что и newLoginService в auth_login_db_error_test.go,
// но под своим именем: делить одну функцию между файлами того же пакета можно, только
// проще не плодить путаницу, какой файл её "хозяин".
func newRefreshService(db *gorm.DB) services.AuthService {
	return services.NewAuthService(db, testutil.TestJWTSecret, testutil.TestJWTRefreshSecret,
		15*time.Minute, 168*time.Hour)
}

// wrapForBreak поднимает поверх РАБОЧЕЙ тест-БД второе gorm-соединение с общим
// пулом (postgres.Config.Conn) - тот же приём, что и breakUsersQuery в
// auth_login_db_error_test.go. register навешивает конкретный callback (что именно
// ломать), broken управляет тем, ломает ли соединение прямо сейчас.
func wrapForBreak(t *testing.T, base *gorm.DB, register func(db *gorm.DB, broken *bool)) (*gorm.DB, *bool) {
	t.Helper()
	sqlDB, err := base.DB()
	require.NoError(t, err)
	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sqlDB}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)

	broken := false
	register(db, &broken)
	return db, &broken
}

// breakRefreshTokenQuery ломает SELECT по refresh_tokens (storedToken lookup).
func breakRefreshTokenQuery(t *testing.T, base *gorm.DB) (*gorm.DB, *bool) {
	t.Helper()
	return wrapForBreak(t, base, func(db *gorm.DB, broken *bool) {
		require.NoError(t, db.Callback().Query().Before("gorm:query").
			Register("test:break_refresh_token_query", func(tx *gorm.DB) {
				if !*broken {
					return
				}
				if _, ok := tx.Statement.Dest.(*models.RefreshToken); ok {
					tx.AddError(errRefreshTokenQueryDown)
				}
			}))
	})
}

// breakRefreshTokenWrite ломает и UPDATE, и INSERT по refresh_tokens. Модель
// определяем по Statement.Model - для Updates(map[...]) он остаётся указателем на
// models.RefreshToken, даже когда Dest - сама map, а для Create Dest и Model совпадают.
func breakRefreshTokenWrite(t *testing.T, base *gorm.DB) (*gorm.DB, *bool) {
	t.Helper()
	return wrapForBreak(t, base, func(db *gorm.DB, broken *bool) {
		hook := func(tx *gorm.DB) {
			if !*broken {
				return
			}
			if _, ok := tx.Statement.Model.(*models.RefreshToken); ok {
				tx.AddError(errRefreshTokenWriteDown)
			}
		}
		require.NoError(t, db.Callback().Update().Before("gorm:update").
			Register("test:break_refresh_token_update", hook))
		require.NoError(t, db.Callback().Create().Before("gorm:create").
			Register("test:break_refresh_token_create", hook))
	})
}

// breakRefreshTokenCreateOnly ломает ТОЛЬКО INSERT (новый токен), UPDATE ротации
// проходит штатно. Нужен чтобы доказать: без транзакции обновление старого токена
// (revoked) и запись нового - два независимых запроса, и обрыв второго оставляет
// старый токен отозванным, а новый - не сохранённым (#2016, "мёртвая сессия").
func breakRefreshTokenCreateOnly(t *testing.T, base *gorm.DB) (*gorm.DB, *bool) {
	t.Helper()
	return wrapForBreak(t, base, func(db *gorm.DB, broken *bool) {
		require.NoError(t, db.Callback().Create().Before("gorm:create").
			Register("test:break_refresh_token_create_only", func(tx *gorm.DB) {
				if !*broken {
					return
				}
				if _, ok := tx.Statement.Model.(*models.RefreshToken); ok {
					tx.AddError(errRefreshTokenWriteDown)
				}
			}))
	})
}

// breakRefreshTokenUpdateOnly - зеркало предыдущего: ломает только UPDATE (ротацию
// старого токена), INSERT нового проходит штатно. Без транзакции это создаёт ВТОРОЙ
// одновременно живой токен в той же семье - ротация перестаёт быть ротацией.
func breakRefreshTokenUpdateOnly(t *testing.T, base *gorm.DB) (*gorm.DB, *bool) {
	t.Helper()
	return wrapForBreak(t, base, func(db *gorm.DB, broken *bool) {
		require.NoError(t, db.Callback().Update().Before("gorm:update").
			Register("test:break_refresh_token_update_only", func(tx *gorm.DB) {
				if !*broken {
					return
				}
				if _, ok := tx.Statement.Model.(*models.RefreshToken); ok {
					tx.AddError(errRefreshTokenWriteDown)
				}
			}))
	})
}

// loginForRefreshTests регистрирует и логинит пользователя, возвращает выданный
// refresh JWT (тот же, что ушёл бы в HttpOnly cookie) и id пользователя.
func loginForRefreshTests(t *testing.T, e *echo.Echo, db *gorm.DB, username string) (string, int) {
	t.Helper()
	td := testutil.SeedTestData(t, db)
	testutil.RegisterUser(t, e, username, "correctpass", 1, td.OrgID, td.CompanyID)
	svc := newRefreshService(db)
	resp, err := svc.Login(context.Background(),
		models.LoginRequest{Username: username, Password: "correctpass"},
		&services.RequestMeta{IPAddress: "198.51.100.20", UserAgent: "go-test"})
	require.NoError(t, err)

	var user models.User
	require.NoError(t, db.Where("username = ?", username).First(&user).Error)
	return resp.RefreshToken, user.ID
}

// TestRefreshToken_UserLookupDBFailureIsNotUnauthorized: недоступная выборка
// пользователя при обновлении токена не должна читаться как "юзера нет" - иначе
// секундная авария базы разлогинивает того, у кого в этот момент фоном продлевалась
// сессия (#2016), хотя его refresh-токен ещё вполне действителен.
func TestRefreshToken_UserLookupDBFailureIsNotUnauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	refreshToken, _ := loginForRefreshTests(t, e, db, "refreshuser1")

	wrapped, broken := breakUsersQuery(t, db)
	svc := newRefreshService(wrapped)

	*broken = true
	_, err := svc.RefreshToken(context.Background(),
		models.RefreshTokenRequest{RefreshToken: refreshToken}, &services.RequestMeta{})
	require.Error(t, err)

	var ae *apperr.Error
	require.ErrorAs(t, err, &ae, "сбой базы обязан приходить доменной ошибкой 500, а не 401 echo.HTTPError")
	assert.Equal(t, http.StatusInternalServerError, ae.Code)
	assert.NotContains(t, ae.Message, "User not found")
	assert.ErrorIs(t, err, errUsersQueryDown, "первопричина обязана доехать до логов")
}

// TestRefreshToken_TokenLookupDBFailureIsNotUnauthorized: та же логика для выборки
// самой записи refresh_tokens - обрыв запроса не значит "токен недействителен".
func TestRefreshToken_TokenLookupDBFailureIsNotUnauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	refreshToken, _ := loginForRefreshTests(t, e, db, "refreshuser2")

	wrapped, broken := breakRefreshTokenQuery(t, db)
	svc := newRefreshService(wrapped)

	*broken = true
	_, err := svc.RefreshToken(context.Background(),
		models.RefreshTokenRequest{RefreshToken: refreshToken}, &services.RequestMeta{})
	require.Error(t, err)

	var ae *apperr.Error
	require.ErrorAs(t, err, &ae, "сбой базы обязан приходить доменной ошибкой 500, а не 401 echo.HTTPError")
	assert.Equal(t, http.StatusInternalServerError, ae.Code)
	assert.NotContains(t, ae.Message, "Invalid refresh token")
	assert.ErrorIs(t, err, errRefreshTokenQueryDown)

	// После восстановления базы тем же токеном можно продлить сессию как ни в чём
	// не бывало - выборка была нечитаема, а не токен недействителен.
	*broken = false
	_, err = svc.RefreshToken(context.Background(),
		models.RefreshTokenRequest{RefreshToken: refreshToken}, &services.RequestMeta{})
	assert.NoError(t, err)
}

// TestRefreshToken_RotationCreateFailureRollsBackRevoke - главный сценарий "мёртвой
// сессии": обрыв записи НОВОГО токена не должен оставлять старый отозванным. Раньше
// (Updates + Create без транзакции, без проверки .Error) функция в этом случае молча
// возвращала УСПЕХ с токенами, которых на самом деле нет в базе - следующий refresh
// упирался бы в 401 без всякой связи с причиной.
func TestRefreshToken_RotationCreateFailureRollsBackRevoke(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	refreshToken, userID := loginForRefreshTests(t, e, db, "refreshuser3")

	wrapped, broken := breakRefreshTokenCreateOnly(t, db)
	svc := newRefreshService(wrapped)

	*broken = true
	resp, err := svc.RefreshToken(context.Background(),
		models.RefreshTokenRequest{RefreshToken: refreshToken}, &services.RequestMeta{})

	require.Error(t, err, "обрыв записи нового токена обязан провалить запрос, а не тихо выдать невалидную пару")
	assert.Nil(t, resp)

	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, http.StatusInternalServerError, ae.Code)
	assert.ErrorIs(t, err, errRefreshTokenWriteDown)

	// Старый токен обязан остаться РАБОЧИМ: транзакция откатила его revoke вместе
	// с непрошедшей записью нового.
	var rows []models.RefreshToken
	require.NoError(t, db.Where("user_id = ?", userID).Find(&rows).Error)
	require.Len(t, rows, 1, "новый токен не должен был появиться в базе")
	assert.False(t, rows[0].IsRevoked, "старый токен не должен оставаться отозванным при обрыве записи нового")

	// И им можно продлить сессию сразу после восстановления базы.
	*broken = false
	_, err = svc.RefreshToken(context.Background(),
		models.RefreshTokenRequest{RefreshToken: refreshToken}, &services.RequestMeta{})
	assert.NoError(t, err, "после аварии старый токен обязан оставаться рабочим")
}

// TestRefreshToken_RotationUpdateFailureDoesNotDuplicateSession - зеркальный
// сценарий: обрыв UPDATE (пометка старого токена revoked), INSERT нового при этом
// проходит. Без транзакции это оставляло ДВА одновременно валидных токена одной
// семьи - инвариант ротации ("один живой токен на семью") ломался молча.
func TestRefreshToken_RotationUpdateFailureDoesNotDuplicateSession(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	refreshToken, userID := loginForRefreshTests(t, e, db, "refreshuser4")

	wrapped, broken := breakRefreshTokenUpdateOnly(t, db)
	svc := newRefreshService(wrapped)

	*broken = true
	resp, err := svc.RefreshToken(context.Background(),
		models.RefreshTokenRequest{RefreshToken: refreshToken}, &services.RequestMeta{})

	require.Error(t, err, "обрыв ротации старого токена обязан провалить запрос, а не тихо расплодить сессии")
	assert.Nil(t, resp)

	var rows []models.RefreshToken
	require.NoError(t, db.Where("user_id = ?", userID).Find(&rows).Error)
	require.Len(t, rows, 1, "новый токен не должен был появиться, пока старый не отозван той же транзакцией")
	assert.False(t, rows[0].IsRevoked)
}

// TestRefreshToken_ReuseFamilyInvalidationFailureIsLogged: при обнаружении reuse
// ответ (401, отказ) не зависит от того, удалось ли инвалидировать всю семью
// токенов - он и так отказ. Но раньше ошибка записи терялась молча: если семья не
// инвалидирована, токены атакующего/легитимного юзера остаются рабочими, и никто
// об этом не узнаёт. Теперь это уходит в журнал событий.
func TestRefreshToken_ReuseFamilyInvalidationFailureIsLogged(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	refreshToken, userID := loginForRefreshTests(t, e, db, "refreshuser5")

	// Ротация без обрыва: refreshToken становится "старым, отозванным" - ровно
	// то, что нужно для reuse detection при повторном предъявлении.
	svc := newRefreshService(db)
	_, err := svc.RefreshToken(context.Background(),
		models.RefreshTokenRequest{RefreshToken: refreshToken}, &services.RequestMeta{})
	require.NoError(t, err)

	// Отодвигаем revoked_at за пределы refreshReuseGraceWindow (10s), иначе повтор
	// попадёт в ветку "race между вкладками", а не в reuse detection.
	require.NoError(t, db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND is_revoked = true", userID).
		Update("revoked_at", time.Now().UTC().Add(-time.Minute)).Error)

	wrapped, broken := breakRefreshTokenWrite(t, db)
	svcBroken := newRefreshService(wrapped)

	*broken = true
	_, err = svcBroken.RefreshToken(context.Background(),
		models.RefreshTokenRequest{RefreshToken: refreshToken}, &services.RequestMeta{})
	require.Error(t, err)

	var he *echo.HTTPError
	require.ErrorAs(t, err, &he, "reuse detection остаётся echo.HTTPError - ответ не должен меняться")
	assert.Equal(t, http.StatusUnauthorized, he.Code)
	assert.Equal(t, "Refresh token reuse detected, please log in again", he.Message)

	var logged models.AuthEvent
	require.NoError(t, db.Where("username = ? AND event_type = ?", "refreshuser5", models.AuthEventRefreshError).
		First(&logged).Error, "провал инвалидации семьи обязан оставить запись в журнале")
	assert.False(t, logged.Success)
	assert.Equal(t, "family invalidation", logged.Detail,
		"в личной истории входов не должно быть текста ошибки драйвера - только стадия")
}
