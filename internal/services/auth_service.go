package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"

	goargon2 "golang.org/x/crypto/argon2"
	_ "crypto/rand"
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
}

// Claims is the JWT claims structure matching the Rust backend.
// is_super_admin кладётся в access-токен чтобы middleware мог проверить
// без дополнительного запроса в БД на каждый запрос (#231).
type Claims struct {
	UserID       int  `json:"user_id"`
	TypeID       int  `json:"type_id"`
	IsSuperAdmin bool `json:"is_super_admin"`
	jwt.RegisteredClaims
}

type authService struct {
	db               *gorm.DB
	jwtSecret        []byte
	jwtRefreshSecret []byte
	accessTTL        time.Duration
	refreshTTL       time.Duration
}

// NewAuthService создаёт сервис аутентификации с JWT-секретами и TTL токенов.
// accessTTL — время жизни access-токена (короткое, например 15m).
// refreshTTL — время жизни refresh-токена (длинное, например 168h = 7d).
func NewAuthService(db *gorm.DB, jwtSecret, jwtRefreshSecret string, accessTTL, refreshTTL time.Duration) AuthService {
	return &authService{
		db:               db,
		jwtSecret:        []byte(jwtSecret),
		jwtRefreshSecret: []byte(jwtRefreshSecret),
		accessTTL:        accessTTL,
		refreshTTL:       refreshTTL,
	}
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

func hashPassword(password string) string {
	salt := generateSalt()
	// Argon2id with default params matching Rust argon2 0.5.3 defaults:
	// m=19456 (19 MiB), t=2, p=1
	hash := argon2.IDKey([]byte(password), salt, 2, 19456, 1, 32)
	// PHC format: $argon2id$v=19$m=19456,t=2,p=1$<salt>$<hash>
	saltB64 := base64RawEncode(salt)
	hashB64 := base64RawEncode(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=19456,t=2,p=1$%s$%s", goargon2.Version, saltB64, hashB64)
}

func verifyPassword(phcHash, password string) bool {
	// Parse PHC format
	salt, expectedHash, params, err := parsePHC(phcHash)
	if err != nil {
		return false
	}
	computed := argon2.IDKey([]byte(password), salt, params.time, params.memory, params.threads, uint32(len(expectedHash)))
	return subtleCompare(computed, expectedHash)
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
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtRefreshSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
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
// 10 попыток за любое время подряд без успеха -> lock на 30 минут.
const (
	maxFailedLoginsBeforeLock = 10
	accountLockDuration       = 30 * time.Minute
	// refreshReuseGraceWindow: окно, в течение которого только что отозванный
	// refresh-токен НЕ считается reuse-атакой. Защита от ложного срабатывания
	// при параллельных refresh из двух табов (общий cookie). Внутри окна -
	// 401 "retry" без family kill, после - полноценный reuse detection.
	// 10s выбрано как компромисс: легитимные race-условия проходят, окно
	// для атакующего минимальное (Auth0 дефолт 0-30s).
	refreshReuseGraceWindow = 10 * time.Second
)

// Login выполняет аутентификацию пользователя и возвращает пару токенов.
func (s *authService) Login(ctx context.Context, req models.LoginRequest, meta *RequestMeta) (*models.LoginResponse, error) {
	var user models.User
	err := s.db.WithContext(ctx).
		Preload("Organization").
		Preload("Company").
		Preload("UserType").
		Where("username = ?", req.Username).
		First(&user).Error
	if err != nil {
		s.recordAuthEvent(ctx, nil, req.Username, models.AuthEventLoginFailed, false, meta, "user not found")
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid credentials")
	}

	// Учётка заблокирована - не разрешаем попытки (даже с правильным паролем),
	// пока не истечёт lock-период. Это важно: иначе валидная попытка сбросила бы
	// счётчик и атакующий мог бы "разморозить" учётку угадав пароль в моменте.
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now().UTC()) {
		remaining := int(time.Until(*user.LockedUntil).Seconds())
		s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventLoginLocked, false, meta,
			fmt.Sprintf("locked for %ds", remaining))
		return nil, echo.NewHTTPError(http.StatusTooManyRequests,
			fmt.Sprintf("Учётная запись временно заблокирована. Повторите через %d секунд.", remaining))
	}

	if !verifyPassword(user.Password, req.Password) {
		s.registerFailedLogin(ctx, &user)
		s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventLoginFailed, false, meta, "wrong password")
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid credentials")
	}

	// Успешный вход - сбрасываем счётчик неудачных попыток и lock.
	// Также апдейтим last_login_at для аудита активности.
	now := time.Now().UTC()
	updates := map[string]interface{}{"last_login_at": now}
	if user.FailedLoginCount > 0 || user.LockedUntil != nil {
		updates["failed_login_count"] = 0
		updates["locked_until"] = nil
	}
	s.db.WithContext(ctx).Model(&user).Updates(updates)

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
	s.db.WithContext(ctx).Create(&rt)

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
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	// Ищем запись без фильтра is_revoked - чтобы отличить "не существует" от
	// "существует, но отозван" (второе - признак replay-атаки).
	tokenHash := hashRefreshToken(refreshToken)
	var storedToken models.RefreshToken
	err = s.db.WithContext(ctx).
		Where("user_id = ? AND token_hash = ?", user.ID, tokenHash).
		First(&storedToken).Error
	if err != nil {
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
		s.db.WithContext(ctx).
			Model(&models.RefreshToken{}).
			Where("family_id = ? AND is_revoked = false", storedToken.FamilyID).
			Updates(map[string]any{"is_revoked": true, "revoked_at": now})
		slog.Warn("refresh token reuse detected - family invalidated",
			"user_id", user.ID, "family_id", storedToken.FamilyID)
		s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventTokenReuseDetected, false, meta,
			"family_id="+storedToken.FamilyID)
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Refresh token reuse detected, please log in again")
	}

	// Ротация: помечаем старый revoked + revoked_at, выдаём новую пару в той же family.
	now := time.Now().UTC()
	s.db.WithContext(ctx).Model(&storedToken).
		Updates(map[string]any{"is_revoked": true, "revoked_at": now})

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
	s.db.WithContext(ctx).Create(&rt)

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
// если достигнут порог. Ошибки логируются, но не прерывают запрос - клиент
// всё равно получит "Invalid credentials", чтобы не раскрывать состояние лока.
func (s *authService) registerFailedLogin(ctx context.Context, user *models.User) {
	user.FailedLoginCount++
	updates := map[string]interface{}{
		"failed_login_count": user.FailedLoginCount,
	}
	if user.FailedLoginCount >= maxFailedLoginsBeforeLock {
		lockUntil := time.Now().UTC().Add(accountLockDuration)
		updates["locked_until"] = lockUntil
		// Отдельное event - удобно алёртить "аккаунт только что залочили".
		s.recordAuthEvent(ctx, &user.ID, user.Username, models.AuthEventAccountLocked, false, nil,
			fmt.Sprintf("locked for %s after %d failed attempts", accountLockDuration, user.FailedLoginCount))
	}
	s.db.WithContext(ctx).Model(user).Updates(updates)
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
