package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"

	_ "crypto/rand"
	goargon2 "golang.org/x/crypto/argon2"
)

// RequestMeta - метаданные HTTP-запроса, пробрасываемые в auth-операции.
// IP и User-Agent нужны и для audit log (AuthEvent), и для binding на refresh
// токен. nil допустим (обратная совместимость / тесты без meta).
type RequestMeta struct {
	IPAddress string
	UserAgent string
}

// AuthService defines the auth business logic interface.
// Создание пользователей идёт через UserService.Create (POST /users, admin-only).
// Публичная регистрация (POST /register) не поддерживается - см. удалённый Register handler.
type AuthService interface {
	Login(ctx context.Context, req models.LoginRequest, meta *RequestMeta) (*models.LoginResponse, error)
	RefreshToken(ctx context.Context, req models.RefreshTokenRequest, meta *RequestMeta) (*models.TokenPairResponse, error)
	Logout(ctx context.Context, username string, req models.LogoutRequest, meta *RequestMeta) error
	LogoutAll(ctx context.Context, username string) (int, error)
	GetUserData(ctx context.Context, username string) (*models.UserDataResponse, error)
	GetCurrentUser(ctx context.Context, username string) (*models.CurrentUserResponse, error)
	GetUserTypes(ctx context.Context) ([]models.UserType, error)
	ResetLoginLockout(ctx context.Context, username string, actorUserID int) (bool, error)
}

// Claims is the JWT claims structure matching the Rust backend.
// is_super_admin кладётся в access-токен чтобы middleware мог проверить
// без дополнительного запроса в БД на каждый запрос (#231).
type Claims struct {
	UserID       int  `json:"user_id"`
	TypeID       int  `json:"type_id"`
	IsSuperAdmin bool `json:"is_super_admin"`
	// ImpersonatedBy - идентификатор администратора, открывшего сеанс «войти как
	// пользователь» (#1912). У маркеров обычного входа поле пустое и, благодаря
	// omitempty, вовсе не попадает в полезную нагрузку: их вид и разбор не меняются.
	// Заполнено только у маркеров, выданных ImpersonationService.
	ImpersonatedBy *int `json:"impersonated_by,omitempty"`
	jwt.RegisteredClaims
}

type authService struct {
	db                  *gorm.DB
	jwtSecret           []byte
	jwtRefreshSecret    []byte
	accessTTL           time.Duration
	refreshTTL          time.Duration
	loginGuard          *loginGuard
	notificationService NotificationService
}

// AuthServiceOption конфигурирует authService при создании.
type AuthServiceOption func(*authService)

// WithAuthNotifications подключает уведомление владельца учётки о переходе в
// блокировку входа (#1748 S3). Опционально, nil-safe: без неё уведомление просто
// не создаётся (тесты, offline) - сама блокировка от этого не зависит.
func WithAuthNotifications(ns NotificationService) AuthServiceOption {
	return func(s *authService) { s.notificationService = ns }
}

// NewAuthService создаёт сервис аутентификации с JWT-секретами и TTL токенов.
// accessTTL — время жизни access-токена (короткое, например 15m).
// refreshTTL — время жизни refresh-токена (длинное, например 168h = 7d).
func NewAuthService(db *gorm.DB, jwtSecret, jwtRefreshSecret string, accessTTL, refreshTTL time.Duration, opts ...AuthServiceOption) AuthService {
	s := &authService{
		db:               db,
		jwtSecret:        []byte(jwtSecret),
		jwtRefreshSecret: []byte(jwtRefreshSecret),
		accessTTL:        accessTTL,
		refreshTTL:       refreshTTL,
		// Единый per-IP счётчик попыток входа: одинаков для существующих и
		// несуществующих логинов (счётчик показывается всегда, не палит существование).
		loginGuard: newLoginGuard(maxFailedLoginsBeforeLock, loginFailureWindow, accountLockDuration),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// intPtrOrNil возвращает указатель на v или nil если v <= 0.
// Используется для FK-полей (organization_id, company_id) где 0 = "не привязан",
// а в БД FK constraint требует либо NULL либо существующий id.
func intPtrOrNil(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

// --- Password Hashing (Argon2id, compatible with Rust argon2 crate) ---

// HashPassword - публичная точка хеширования для кода вне пакета: сидов,
// вспомогательных команд и тестовых помощников. До неё каждый из них нёс
// собственную копию параметров Argon2id, и правка параметров здесь молча
// оставляла бы тех троих со старыми - тестовые учётные записи переставали бы
// логиниться, а причина была бы неочевидна.
func HashPassword(password string) string {
	return hashPassword(password)
}

func hashPassword(password string) string {
	salt := generateSalt()
	// Argon2id with default params matching Rust argon2 0.5.3 defaults:
	// m=19456 (19 MiB), t=2, p=1
	var hash []byte
	withArgon2Slot(func() {
		hash = argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	})
	// PHC format: $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
	saltB64 := base64RawEncode(salt)
	hashB64 := base64RawEncode(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=19456,t=2,p=1$%s$%s", goargon2.Version, saltB64, hashB64)
}

// verifyPassword сравнивает пароль с сохранённым PHC-хешем. Второй результат -
// ошибка РАЗБОРА хеша (обрезанная строка, чужой алгоритм, битый base64), а не
// "пароль не подошёл". Вызывающий обязан различать эти два случая: иначе дефект
// сохранённых данных выглядит как неверно введённый пароль и наказывается как
// он - счётчиком неудачных попыток и лестницей блокировки (#2017).
func verifyPassword(phcHash, password string) (bool, error) {
	// Parse PHC format
	salt, expectedHash, params, err := parsePHC(phcHash)
	if err != nil {
		return false, err
	}
	var computed []byte
	withArgon2Slot(func() {
		computed = argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(expectedHash)))
	})
	return subtleCompare(computed, expectedHash), nil
}

// --- JWT ---

func (s *authService) createAccessToken(username string, userID int, typeID int, isSuperAdmin bool) (string, error) {
	claims := Claims{
		UserID:       userID,
		TypeID:       typeID,
		IsSuperAdmin: isSuperAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *authService) createRefreshJWT(username string) (string, error) {
	// JTI (JWT ID) делает каждый refresh-JWT уникальным, даже если выдан
	// в ту же секунду. Без него refresh-токены с одинаковым subject+exp
	// генерируют идентичные JWT -> идентичный hash -> конфликт в uniqueIndex.
	claims := Claims{
		TypeID: 0,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ID:        uuid.NewString(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.refreshTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtRefreshSecret)
}

func (s *authService) decodeRefreshToken(tokenString string) (*Claims, error) {
	return DecodeRefreshToken(tokenString, s.jwtRefreshSecret)
}

// hashRefreshToken produces "sha256:<hex>" matching Rust backend format.
func hashRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return "sha256:" + hex.EncodeToString(h[:])
}

// --- Service Methods ---

// Пороги lockout-а учётки по количеству неверных попыток. Защита от distributed
// brute-force когда атакующие идут с разных IP (IP-лимитер их не ловит, т.к.
// счётчик per-IP, но счётчик per-username общий и копится от всех источников).
// 5 попыток в пределах loginFailureWindow -> блокировка по лестнице ниже.
const (
	maxFailedLoginsBeforeLock = 5
	// accountLockDuration - первая ступень лестницы. Ею же живёт per-IP гвард:
	// его длительность плоская, чтобы офис за общим адресом не улетал на час.
	accountLockDuration = 1 * time.Minute
	// lockoutLevelDecay - сколько учётка должна прожить без неудачных попыток,
	// чтобы лестница вернулась на первую ступень. Без затухания единственная
	// блокировка полгода назад встречала бы человека сразу часом.
	lockoutLevelDecay = 24 * time.Hour
	// refreshReuseGraceWindow: окно, в течение которого только что отозванный
	// refresh-токен НЕ считается reuse-атакой. Защита от ложного срабатывания
	// при параллельных refresh из двух табов (общий cookie). Внутри окна -
	// 401 "retry" без family kill, после - полноценный reuse detection.
	// 10s выбрано как компромисс: легитимные race-условия проходят, окно
	// для атакующего минимальное (Auth0 дефолт 0-30s).
	refreshReuseGraceWindow = 10 * time.Second
)

// lockoutSteps - во сколько раз каждая следующая блокировка длиннее первой.
// При базе в минуту это 1, 5, 15, 30 и 60 минут. Каждые maxFailedLoginsBeforeLock
// неудач поднимают ступень; после последней лестница упирается в час и дальше не
// растёт. Ступень обнуляется успешным входом, сбросом из админки и сутками без
// неудачных попыток (lockoutLevelDecay).
//
// Хранятся именно множители, а не готовые длительности: по этой же лестнице живёт
// счётчик пары «адрес + логин» в loginGuard, у которого своя база (в тестах
// короткая). Общая форма лестницы гарантирует, что сроки для существующего и
// выдуманного логина совпадают.
var lockoutSteps = []int{1, 5, 15, 30, 60}

// stepDuration - длительность ступени level от базового шага base.
func stepDuration(base time.Duration, level int) time.Duration {
	if level < 0 {
		level = 0
	}
	if level >= len(lockoutSteps) {
		level = len(lockoutSteps) - 1
	}
	return base * time.Duration(lockoutSteps[level])
}

// lockoutDuration возвращает длительность блокировки учётной записи для ступени
// level (0 - первая блокировка).
func lockoutDuration(level int) time.Duration {
	return stepDuration(accountLockDuration, level)
}

// lockoutError - единый ответ на любую блокировку входа. Текст ОДИН для адреса и
// для учётной записи: разные формулировки сообщали бы перебирающему, существует
// ли логин, - блокировка учётки бывает только у существующего.
func (s *authService) lockoutError(seconds int) error {
	return apperr.TooManyRequests(
		fmt.Sprintf("Слишком много попыток. Вход заблокирован на %d секунд.", seconds)).
		WithHeader("Retry-After", strconv.Itoa(seconds))
}

// accountLockSeconds - остаток блокировки учётной записи в секундах (0, если
// логина нет или он не заперт). Отдельный лёгкий запрос по индексу: полную
// запись на этом пути читать незачем, а пароль здесь не проверяется вовсе.
func (s *authService) accountLockSeconds(ctx context.Context, username string) int {
	var lockedUntil *time.Time
	if err := s.db.WithContext(ctx).Table("users").Select("locked_until").
		Where("username = ?", username).Scan(&lockedUntil).Error; err != nil {
		// Решение об отказе принято счётчиком в памяти и от этого запроса не
		// зависит, поэтому вход не роняем. Но молчать нельзя: без остатка по
		// учётке ответ пообещает вход раньше, чем он откроется.
		slog.Warn("не удалось прочитать остаток блокировки учётной записи", "username", username, "error", err)
		return 0
	}
	if lockedUntil == nil || !lockedUntil.After(time.Now().UTC()) {
		return 0
	}
	return secondsUntil(*lockedUntil)
}

// loginFailureResponse формирует ответ на неудачную попытку входа: 401 с остатком
// попыток ("Осталось попыток: N") либо, если попытка исчерпала лимит, сразу 429 с
// таймером блокировки (без промежуточного "осталось 0").
//
// accountLockedUntil - момент, до которого заблокирована сама учётка (nil, если
// эта попытка её не заперла). Таймер берётся БОЛЬШИЙ из двух: у слоёв разные
// длительности (у адреса плоская минута, у учётки лестница), и показать меньший
// значит пообещать вход раньше, чем он откроется.
func (s *authService) loginFailureResponse(ip, username string, accountLockedUntil *time.Time) error {
	remaining, blockedSec, _ := s.loginGuard.recordFailure(ip, username)
	if accountLockedUntil != nil {
		if sec := secondsUntil(*accountLockedUntil); sec > blockedSec {
			blockedSec = sec
		}
	}
	if blockedSec > 0 {
		return s.lockoutError(blockedSec)
	}
	return apperr.Unauthorized("Неверный логин или пароль").
		WithHeader("X-Auth-Attempts-Remaining", strconv.Itoa(remaining))
}

// loginUnavailable - ответ на СБОЙ входа, а не на неверные учётные данные:
// недоступная база, исчерпанный пул соединений, таймаут запроса. От обычного
// отказа отличается тремя вещами.
//
// Наружу уходит 500, а не 401: дело не в пароле, и повторять подбор бессмысленно.
// 503 тут занят - его отдаёт режим технических работ, и фронт по нему уводит на
// страницу техработ, где сам же читает окно работ из базы, которой сейчас нет.
// Для 500 на /login у клиента исключение из редиректа на страницу ошибки: форма
// входа показывает текст сама.
//
// Счётчик блокировки НЕ трогается (loginFailureResponse тут не зовётся): иначе
// после аварии люди сидели бы заперты за то, чего не делали.
//
// Причина пишется и в журнал событий, и в системный лог. Журнал живёт в той же
// базе и при полной аварии не сохранится - системный лог её переживает.
func (s *authService) loginUnavailable(ctx context.Context, username string, meta *RequestMeta, stage string, cause error) error {
	slog.Error("вход не выполнен из-за ошибки обращения к базе", "stage", stage, "username", username, "error", cause)
	s.recordAuthEvent(ctx, nil, username, models.AuthEventLoginError, false, meta,
		fmt.Sprintf("%s: %v", stage, cause))
	return apperr.Internal("Вход временно недоступен из-за ошибки на сервере. Повторите попытку позже.", cause)
}

// loginBadHash - ответ на ПОВРЕЖДЁННУЮ запись пароля конкретной учётной записи:
// строка в users.password не разбирается как Argon2id PHC (обрезана, обнулена,
// записана другим алгоритмом). От loginUnavailable отличается причиной и
// адресатом починки: дело не в недоступности базы (та отвечает исправно), а в
// дефекте ОДНОЙ записи - чинить нужно именно её (обычно принудительным сбросом
// пароля), поэтому у события собственный тип (#2017).
//
// Счётчик блокировки НЕ трогается по той же причине, что и при аварии базы:
// человек мог ввести верный пароль, наказывать его за чужой дефект данных нельзя.
func (s *authService) loginBadHash(ctx context.Context, username string, meta *RequestMeta, cause error) error {
	slog.Error("вход не выполнен: повреждена запись пароля в базе", "username", username, "error", cause)
	s.recordAuthEvent(ctx, nil, username, models.AuthEventLoginBadHash, false, meta,
		fmt.Sprintf("password hash unreadable: %v", cause))
	return apperr.Internal("Вход невозможен из-за ошибки на сервере. Обратитесь к администратору.", cause)
}

// Login выполняет аутентификацию пользователя и возвращает пару токенов.
func (s *authService) Login(ctx context.Context, req models.LoginRequest, meta *RequestMeta) (*models.LoginResponse, error) {
	ip := ""
	if meta != nil {
		ip = meta.IPAddress
	}

	// Счётчик в памяти уже запер адрес или пару «адрес + логин» - сразу таймер,
	// без проверки пароля. Он работает одинаково для существующих и несуществующих
	// логинов, поэтому и сроки для них совпадают. Учётка при этом может быть заперта
	// дольше (её лестница копит неудачи со всех адресов), поэтому берём больший
	// срок: меньший обещал бы вход раньше, чем он откроется.
	if sec, blocked := s.loginGuard.blockedSeconds(ip, req.Username); blocked {
		if accSec := s.accountLockSeconds(ctx, req.Username); accSec > sec {
			sec = accSec
		}
		s.recordAuthEvent(ctx, nil, req.Username, models.AuthEventLoginLocked, false, meta,
			fmt.Sprintf("ip locked for %ds", sec))
		return nil, s.lockoutError(sec)
	}

	var user models.User
	err := s.db.WithContext(ctx).
		Preload("Organization").
		Preload("Company").
		Preload("UserType").
		Where("username = ?", req.Username).
		First(&user).Error
	if err != nil {
		// Ошибка запроса - это не только отсутствующая строка: недоступная база,
		// исчерпанный пул, таймаут, сбой преднагрузки связанных сущностей. Считать
		// их «логина нет» значит врать в журнале и наказывать человека за аварию.
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, s.loginUnavailable(ctx, req.Username, meta, "user lookup", err)
		}
		s.recordAuthEvent(ctx, nil, req.Username, models.AuthEventLoginFailed, false, meta, "user not found")
		// Счётчик показываем и для несуществующего логина (тот же ответ, что и при
		// неверном пароле) - иначе по наличию счётчика можно перебирать имена.
		return nil, s.loginFailureResponse(ip, req.Username, nil)
	}

	// Учётка заблокирована - не разрешаем попытки (даже с правильным паролем),
	// пока не истечёт lock-период. Это важно: иначе валидная попытка сбросила бы
	// счётчик и атакующий мог бы "разморозить" учётку угадав пароль в моменте.
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now().UTC()) {
		remaining := secondsUntil(*user.LockedUntil)
		s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventLoginLocked, false, meta,
			fmt.Sprintf("locked for %ds", remaining))
		return nil, s.lockoutError(remaining)
	}

	// Блокировка ИСТЕКЛА (LockedUntil в прошлом) -> сбрасываем счётчик, даём свежий
	// цикл из maxFailedLoginsBeforeLock попыток. Без этого счётчик остаётся на пороге,
	// и первая же новая неверная попытка мгновенно лочит заново с "осталось 0".
	if user.LockedUntil != nil {
		if err := s.db.WithContext(ctx).Model(&user).Updates(map[string]interface{}{
			"failed_login_count": 0,
			"locked_until":       nil,
		}).Error; err != nil {
			slog.Error("не удалось снять истёкшую блокировку входа", "username", user.Username, "error", err)
		}
		user.FailedLoginCount = 0
		user.LockedUntil = nil
	}

	passwordMatches, verifyErr := verifyPassword(user.Password, req.Password)
	if verifyErr != nil {
		// Хеш не разбирается - это не "человек ошибся паролем", а дефект данных
		// этой учётки. Считать его неверным паролем значит копить лестницу
		// блокировки на человеке, который мог ввести пароль верно (#2017).
		return nil, s.loginBadHash(ctx, user.Username, meta, verifyErr)
	}
	if !passwordMatches {
		// registerFailedLogin ведёт per-user счётчик - бэкстоп от distributed
		// brute-force (атака с разных IP, которую per-IP guard не ловит).
		lockedUntil := s.registerFailedLogin(ctx, &user)
		s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventLoginFailed, false, meta, "wrong password")
		return nil, s.loginFailureResponse(ip, user.Username, lockedUntil)
	}

	// Архивная учётка (is_active=false): пароль верный, аутентификация прошла,
	// но доступа нет - токен не выдаём. Пользователь видит, что учётка отключена.
	if !user.IsActive {
		s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventLoginFailed, false, meta, "account disabled")
		return nil, echo.NewHTTPError(http.StatusForbidden, "Учётная запись отключена. Обратитесь к администратору.")
	}

	// Успешный вход - сбрасываем счётчик неудачных попыток и lock (и per-IP, и per-user).
	// Также апдейтим last_login_at для аудита активности.
	s.loginGuard.reset(ip)
	now := time.Now().UTC()
	updates := map[string]interface{}{"last_login_at": now}
	if user.FailedLoginCount > 0 || user.LockedUntil != nil || user.LockoutLevel > 0 {
		updates["failed_login_count"] = 0
		updates["locked_until"] = nil
		// Лестницу тоже опускаем на первую ступень: человек доказал, что он хозяин
		// учётки, и следующая серия опечаток не должна встречать его получасом.
		updates["lockout_level"] = 0
		updates["last_failed_login_at"] = nil
	}
	if err := s.db.WithContext(ctx).Model(&user).Updates(updates).Error; err != nil {
		// Пароль уже проверен, вход состоялся - ронять его из-за отметки активности
		// незачем. Но потерянный сброс счётчика встретит человека блокировкой на
		// ровном месте, и разбирать это придётся по логу.
		slog.Error("не удалось сохранить состояние успешного входа", "username", user.Username, "error", err)
	}

	accessToken, err := s.createAccessToken(user.Username, user.ID, user.TypeID, user.IsSuperAdmin)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create token")
	}

	refreshJWT, err := s.createRefreshJWT(user.Username)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create refresh token")
	}

	// Store hashed refresh token in DB. FamilyID генерируется при login,
	// наследуется всеми refresh-токенами одной сессии. IP/UA - soft-binding
	// для аудита и детекции аномалий.
	familyID := uuid.NewString()
	rt := models.RefreshToken{
		UserID:    user.ID,
		FamilyID:  familyID,
		TokenHash: hashRefreshToken(refreshJWT),
		ExpiresAt: time.Now().UTC().Add(s.refreshTTL),
		IPAddress: metaIPPtr(meta),
		UserAgent: metaUAPtr(meta),
	}
	if err := s.db.WithContext(ctx).Create(&rt).Error; err != nil {
		// Ошибку тут глотать нельзя: refresh-токен на руках есть, а в базе его нет,
		// и сессия тихо умрёт на первом же продлении - человек вылетит посреди работы
		// без объяснения. Честнее отказать сразу тем же ответом, что и при сбое базы.
		return nil, s.loginUnavailable(ctx, user.Username, meta, "refresh token store", err)
	}

	s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventLoginSuccess, true, meta, "")

	return &models.LoginResponse{
		Token:          accessToken,
		RefreshToken:   refreshJWT,
		Organization:   user.Organization.Name,
		OrganizationID: user.OrganizationID,
		Company:        user.Company.Name,
		CompanyID:      user.CompanyID,
		TypeID:         user.TypeID,
		UserType:       user.UserType.Name,
		IsSuperAdmin:   user.IsSuperAdmin,
		IsBanned:       user.IsBanned,
	}, nil
}

// logRefreshDBError пишет сбой обращения к базе при обновлении токена в системный
// лог и в журнал событий, НЕ решая при этом, каким будет ответ вызывающему коду -
// вызывающий код мог уже принять решение по другой причине (см. reuse detection
// ниже, где ответ 401 не зависит от исхода инвалидации семьи). Detail - только
// стадия, без текста ошибки драйвера: если userID уже известен, запись видна в
// личной истории входов пользователя (ListForUser фильтрует по user_id), а туда
// нельзя пересылать "pq: sorry, too many clients already" и подобное.
func (s *authService) logRefreshDBError(ctx context.Context, userID *int, username string, meta *RequestMeta, stage string, cause error) {
	slog.Error("обновление токена не выполнено из-за ошибки обращения к базе", "stage", stage, "username", username, "error", cause)
	s.recordAuthEvent(ctx, userID, username, models.AuthEventRefreshError, false, meta, stage)
}

// refreshUnavailable - ответ на СБОЙ обновления токена, а не на его невалидность:
// недоступная база, исчерпанный пул, сорванная запись ротации. Аналог
// loginUnavailable (#2006): 500, а не 401 - секундная авария базы не должна
// разлогинивать того, у кого в этот момент фоном продлевалась сессия (#2016),
// хотя его refresh-токен ещё вполне действителен. Клиент вправе повторить запрос
// тем же токеном.
func (s *authService) refreshUnavailable(ctx context.Context, userID *int, username string, meta *RequestMeta, stage string, cause error) error {
	s.logRefreshDBError(ctx, userID, username, meta, stage, cause)
	return apperr.Internal("Не удалось обновить сессию из-за ошибки на сервере. Повторите попытку позже.", cause)
}

// RefreshToken обновляет пару access/refresh токенов с ротацией.
// Reuse detection: если пришёл валидный по подписи, но уже отозванный токен -
// это признак кражи (либо attacker, либо legitimate user использовал старую копию
// после ротации). Реакция: инвалидировать всю семью (family_id) и заставить
// перелогиниться. См. Auth0 refresh token rotation pattern.
func (s *authService) RefreshToken(ctx context.Context, req models.RefreshTokenRequest, meta *RequestMeta) (*models.TokenPairResponse, error) {
	refreshToken := req.GetRefreshToken()
	claims, err := s.decodeRefreshToken(refreshToken)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid refresh token")
	}

	username, _ := claims.GetSubject()

	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		// Ошибка запроса - это не только отсутствующая строка: недоступная база,
		// исчерпанный пул, таймаут. Считать их "юзера нет" значит выкидывать на
		// форму входа того, у кого сейчас просто не работает база (#2016).
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, s.refreshUnavailable(ctx, nil, username, meta, "user lookup", err)
		}
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	// Архивная учётка не может обновлять токены - существующая сессия гаснет.
	if !user.IsActive {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Учётная запись отключена")
	}

	// Ищем запись без фильтра is_revoked - чтобы отличить "не существует" от
	// "существует, но отозван" (второе - признак replay-атаки).
	tokenHash := hashRefreshToken(refreshToken)
	var storedToken models.RefreshToken
	err = s.db.WithContext(ctx).
		Where("user_id = ? AND token_hash = ?", user.ID, tokenHash).
		First(&storedToken).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, s.refreshUnavailable(ctx, &user.ID, user.Username, meta, "token lookup", err)
		}
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid refresh token")
	}

	if storedToken.IsRevoked {
		// Grace-window: если токен был revoked совсем недавно (< refreshReuseGraceWindow),
		// это скорее race-condition между двумя табами (общий HttpOnly cookie),
		// чем настоящий reuse. Возвращаем 401 retry без family kill -
		// клиент повторит refresh, теперь уже с обновлённым cookie от первого таба.
		if storedToken.RevokedAt != nil &&
			time.Since(*storedToken.RevokedAt) < refreshReuseGraceWindow {
			slog.Info("refresh token grace-window hit, treating as race not reuse",
				"user_id", user.ID, "family_id", storedToken.FamilyID,
				"revoked_age_ms", time.Since(*storedToken.RevokedAt).Milliseconds())
			return nil, echo.NewHTTPError(http.StatusUnauthorized, "Token recently rotated, please retry")
		}

		// Reuse detection: токен валиден по подписи, отозван давно.
		// Инвалидируем всю семью - включая текущий активный refresh attacker-а
		// или legitimate user-а. Оба будут вынуждены перелогиниться.
		now := time.Now().UTC()
		if err := s.db.WithContext(ctx).
			Model(&models.RefreshToken{}).
			Where("family_id = ? AND is_revoked = false", storedToken.FamilyID).
			Updates(map[string]any{"is_revoked": true, "revoked_at": now}).Error; err != nil {
			// Отказ по reuse уже принят и не меняется от исхода этой записи - токен
			// и так пойман на повторном использовании. Но если сама инвалидация
			// семьи не прошла, токены атакующего/легитимного юзера могли остаться
			// рабочими - это должно быть видно, а не потеряно молча.
			s.logRefreshDBError(ctx, &user.ID, user.Username, meta, "family invalidation", err)
		}
		slog.Warn("refresh token reuse detected - family invalidated",
			"user_id", user.ID, "family_id", storedToken.FamilyID)
		s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventTokenReuseDetected, false, meta,
			"family_id="+storedToken.FamilyID)
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Refresh token reuse detected, please log in again")
	}

	newAccess, err := s.createAccessToken(username, user.ID, user.TypeID, user.IsSuperAdmin)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create token")
	}

	newRefresh, err := s.createRefreshJWT(username)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create refresh token")
	}

	rt := models.RefreshToken{
		UserID:    user.ID,
		FamilyID:  storedToken.FamilyID,
		TokenHash: hashRefreshToken(newRefresh),
		ExpiresAt: time.Now().UTC().Add(s.refreshTTL),
		IPAddress: metaIPPtr(meta),
		UserAgent: metaUAPtr(meta),
	}

	// Ротация: помечаем старый revoked + пишем новый ОДНОЙ транзакцией. Раздельными
	// запросами (как было) старый токен мог оказаться отозван, а новый - не
	// записан: сессия тихо умирала бы на следующем продлении без всякой связи с
	// причиной (#2016). В транзакции любой сбой откатывает обе записи - старый
	// токен остаётся рабочим, и клиент может безопасно повторить запрос им же.
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&storedToken).
			Updates(map[string]any{"is_revoked": true, "revoked_at": now}).Error; err != nil {
			return err
		}
		return tx.Create(&rt).Error
	}); err != nil {
		return nil, s.refreshUnavailable(ctx, &user.ID, user.Username, meta, "token rotation", err)
	}

	s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventRefresh, true, meta, "")

	return &models.TokenPairResponse{
		Token:        newAccess,
		RefreshToken: newRefresh,
	}, nil
}

// Logout отзывает refresh-токен пользователя.
func (s *authService) Logout(ctx context.Context, username string, req models.LogoutRequest, meta *RequestMeta) error {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	tokenHash := hashRefreshToken(req.GetRefreshToken())
	s.db.WithContext(ctx).
		Where("user_id = ? AND token_hash = ?", user.ID, tokenHash).
		Delete(&models.RefreshToken{})

	s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventLogout, true, meta, "")

	return nil
}

// LogoutAll отзывает ВСЕ активные refresh_tokens пользователя. Использовать
// когда подозревается компрометация (юзер сам инициировал "выйти везде"),
// или автоматически при смене пароля. Возвращает количество отозванных токенов.
func (s *authService) LogoutAll(ctx context.Context, username string) (int, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	now := time.Now().UTC()
	result := s.db.WithContext(ctx).
		Model(&models.RefreshToken{}).
		Where("user_id = ? AND is_revoked = false", user.ID).
		Updates(map[string]any{"is_revoked": true, "revoked_at": now})
	if result.Error != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Failed to revoke sessions")
	}
	return int(result.RowsAffected), nil
}

// GetUserData возвращает профильные данные пользователя по username.
func (s *authService) GetUserData(ctx context.Context, username string) (*models.UserDataResponse, error) {
	var user models.User
	err := s.db.WithContext(ctx).
		Preload("Organization").
		Preload("Company").
		Where("username = ?", username).
		First(&user).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	return &models.UserDataResponse{
		Username:       user.Username,
		Organization:   user.Organization.Name,
		OrganizationID: user.OrganizationID,
		Company:        user.Company.Name,
		CompanyID:      user.CompanyID,
		LastName:       user.LastName,
		FirstName:      user.FirstName,
		MiddleName:     user.MiddleName,
		Phone:          user.Phone,
	}, nil
}

// GetCurrentUser возвращает полную информацию о текущем пользователе.
func (s *authService) GetCurrentUser(ctx context.Context, username string) (*models.CurrentUserResponse, error) {
	var user models.User
	err := s.db.WithContext(ctx).
		Preload("Organization").
		Preload("Company").
		Preload("UserType").
		Where("username = ?", username).
		First(&user).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	return &models.CurrentUserResponse{
		ID:             user.ID,
		Username:       user.Username,
		Organization:   user.Organization.Name,
		OrganizationID: user.OrganizationID,
		Company:        user.Company.Name,
		CompanyID:      user.CompanyID,
		TypeID:         user.TypeID,
		UserType:       user.UserType.Name,
		UserTypeCode:   user.UserType.Code,
		IsSuperAdmin:   user.IsSuperAdmin,
		IsBanned:       user.IsBanned,
		LastName:       user.LastName,
		FirstName:      user.FirstName,
		MiddleName:     user.MiddleName,
		Position:       user.Position,
		Email:          user.Email,
		Phone:          user.Phone,
	}, nil
}

// GetUserTypes возвращает список всех типов пользователей.
func (s *authService) GetUserTypes(ctx context.Context) ([]models.UserType, error) {
	types := make([]models.UserType, 0)
	if err := s.db.WithContext(ctx).Order("id").Find(&types).Error; err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to fetch user types")
	}
	return types, nil
}

// registerFailedLogin увеличивает счётчик неудачных попыток и лочит учётку,
// если достигнут порог. Запрос не прерывается - клиент всё равно получит
// "Неверный логин или пароль", чтобы не раскрывать состояние лока.
// Возвращает момент окончания блокировки, если эта попытка её и поставила,
// иначе nil - вызывающий показывает пользователю честный таймер.
func (s *authService) registerFailedLogin(ctx context.Context, user *models.User) *time.Time {
	now := time.Now().UTC()

	// Давняя неудача не копится: попытки считаются в пределах окна, иначе одна
	// опечатка утром и четыре вечером сложились бы в блокировку.
	count := user.FailedLoginCount
	if user.LastFailedLoginAt == nil || now.Sub(*user.LastFailedLoginAt) > loginFailureWindow {
		count = 0
	}
	// Сутки без неудач возвращают лестницу на первую ступень.
	level := user.LockoutLevel
	if user.LastFailedLoginAt == nil || now.Sub(*user.LastFailedLoginAt) > lockoutLevelDecay {
		level = 0
	}

	count++
	updates := map[string]interface{}{
		"failed_login_count":   count,
		"last_failed_login_at": now,
		"lockout_level":        level,
	}
	user.FailedLoginCount = count
	user.LastFailedLoginAt = &now
	user.LockoutLevel = level

	if count < maxFailedLoginsBeforeLock {
		s.saveFailedLoginState(ctx, user, updates)
		return nil
	}

	// Порог достигнут: блокируем на текущую ступень и поднимаем её для следующего
	// круга. Счётчик обнуляем - после отсидки человек получает свежие попытки,
	// а не мгновенный ре-лок с первой же ошибки.
	dur := lockoutDuration(level)
	lockUntil := now.Add(dur)
	updates["locked_until"] = lockUntil
	updates["failed_login_count"] = 0
	updates["lockout_level"] = level + 1
	user.FailedLoginCount = 0
	user.LockedUntil = &lockUntil
	user.LockoutLevel = level + 1
	// Отдельное event - удобно алёртить "аккаунт только что залочили".
	s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventAccountLocked, false, nil,
		fmt.Sprintf("locked for %s after %d failed attempts (step %d)", dur, maxFailedLoginsBeforeLock, level+1))
	s.saveFailedLoginState(ctx, user, updates)
	// Уведомление - РОВНО в момент перехода в блокировку, не на каждой неудачной
	// попытке (иначе спам ровно там, где его боятся). user здесь всегда существующая
	// запись из БД (registerFailedLogin зовётся Login() только после успешного поиска
	// по username) - для выдуманного логина уведомлять некого и нечего.
	s.notifyLoginBlocked(ctx, user, maxFailedLoginsBeforeLock, lockUntil)
	return &lockUntil
}

// notifyLoginBlocked создаёт персистентное уведомление о блокировке входа.
// Пользователь увидит его только после снятия блокировки (залогиниться и прочитать,
// пока учётка заперта, он не может) - запись всё равно нужна, она объясняет, что
// случилось и когда доступ вернётся. Best-effort: ошибка не должна ломать сам логин-флоу.
func (s *authService) notifyLoginBlocked(ctx context.Context, user *models.User, attempts int, lockedUntil time.Time) {
	if s.notificationService == nil {
		return
	}

	untilMSK := lockedUntil.In(moscowWorkModeLoc).Format("02.01.2006 15:04")
	message := fmt.Sprintf(
		"Слишком много неудачных попыток входа (%d) в вашу учётную запись. Вход заблокирован до %s (МСК).",
		attempts, untilMSK,
	)

	dataPayload := map[string]any{
		"attempts":     attempts,
		"locked_until": lockedUntil.UTC().Format(time.RFC3339),
	}
	dataBytes, err := json.Marshal(dataPayload)
	if err != nil {
		slog.Warn("не удалось сериализовать payload уведомления о блокировке входа", "error", err)
		return
	}
	dataStr := string(dataBytes)

	if err := s.notificationService.CreateForUser(
		ctx, user.ID,
		NotificationTypeLoginBlocked,
		"Вход временно заблокирован",
		message,
		&dataStr,
	); err != nil {
		slog.Warn("не удалось создать уведомление о блокировке входа", "user_id", user.ID, "error", err)
	}
}

// saveFailedLoginState пишет состояние счётчика/блокировки. Запрос не прерываем -
// клиент в любом случае получит отказ, - но провал записи означает, что защита от
// перебора не работает, и молчать о нём нельзя.
func (s *authService) saveFailedLoginState(ctx context.Context, user *models.User, updates map[string]interface{}) {
	if err := s.db.WithContext(ctx).Model(user).Updates(updates).Error; err != nil {
		slog.Error("не удалось сохранить счётчик неудачных входов", "username", user.Username, "error", err)
	}
}

// ResetLoginLockout снимает блокировку входа с учётной записи: обнуляет счётчик
// неудач, лестницу кулдаунов и сам лок, а заодно чистит per-IP счётчики адресов,
// с которых этот логин не проходил. Возвращает false, если сбрасывать было нечего.
func (s *authService) ResetLoginLockout(ctx context.Context, username string, actorUserID int) (bool, error) {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return false, apperr.NotFound("Пользователь не найден")
	}

	s.loginGuard.resetUser(user.Username)

	hadLockout := user.LockedUntil != nil || user.FailedLoginCount > 0 || user.LockoutLevel > 0
	if !hadLockout {
		return false, nil
	}

	if err := s.db.WithContext(ctx).Model(&user).Updates(map[string]interface{}{
		"failed_login_count":   0,
		"locked_until":         nil,
		"lockout_level":        0,
		"last_failed_login_at": nil,
	}).Error; err != nil {
		return false, fmt.Errorf("reset login lockout: %w", err)
	}

	// Событие ложится в ту же ленту, где живут сами блокировки (вкладка «История
	// входов», категория «Блокировки») - иначе снятие пришлось бы искать в другом журнале.
	s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventLockoutReset, true, nil,
		fmt.Sprintf("сброшено администратором %s", s.usernameByID(ctx, actorUserID)))
	return true, nil
}

// usernameByID возвращает логин по id для человекочитаемых деталей событий.
// Пустой результат заменяется на id - деталь не должна теряться из-за неудачного запроса.
func (s *authService) usernameByID(ctx context.Context, userID int) string {
	if userID <= 0 {
		return "(система)"
	}
	var name string
	s.db.WithContext(ctx).Table("users").Select("username").Where("id = ?", userID).Scan(&name)
	if name == "" {
		return fmt.Sprintf("id=%d", userID)
	}
	return name
}

// recordAuthEvent пишет запись в auth_events. meta может быть nil (тесты/тесты
// без http-контекста). Ошибки логируются, но не пропагируются наверх - audit log
// best-effort, он не должен ломать основной login/logout flow.
func (s *authService) recordAuthEvent(ctx context.Context, userID *int, username, eventType string, success bool, meta *RequestMeta, detail string) {
	ip, ua := "", ""
	if meta != nil {
		ip = meta.IPAddress
		ua = meta.UserAgent
	}
	if len(ua) > 255 {
		ua = ua[:255]
	}
	if len(detail) > 255 {
		detail = detail[:255]
	}
	ev := models.AuthEvent{
		UserID:    userID,
		Username:  username,
		EventType: eventType,
		Success:   success,
		IPAddress: ip,
		UserAgent: ua,
		Detail:    detail,
	}
	if err := s.db.WithContext(ctx).Create(&ev).Error; err != nil {
		slog.Warn("failed to record auth event", "event_type", eventType, "username", username, "error", err)
	}
}

// metaIPPtr возвращает *string к IP из meta или nil если meta nil / IP пустой.
func metaIPPtr(meta *RequestMeta) *string {
	if meta == nil || meta.IPAddress == "" {
		return nil
	}
	v := meta.IPAddress
	return &v
}

// metaUAPtr - аналогично для UA.
func metaUAPtr(meta *RequestMeta) *string {
	if meta == nil || meta.UserAgent == "" {
		return nil
	}
	v := meta.UserAgent
	return &v
}

// --- Helpers ---

// DecodeAccessToken validates and parses an access JWT. Used by middleware.
func DecodeAccessToken(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// RefreshCookieName - имя cookie с маркером продления сеанса. Объявлено здесь,
// а не в handlers: то же имя читает middleware доступа к загруженным файлам.
const RefreshCookieName = "refresh_token"

// DecodeRefreshToken проверяет подпись и срок маркера продления. Отзыв маркера
// эта проверка не видит - она не обращается к базе; для решений, где важен
// отзыв, берётся AuthService.RefreshToken.
func DecodeRefreshToken(tokenString string, secret []byte) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
