package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"systemburo/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/argon2"
	"gorm.io/gorm"

	goargon2 "golang.org/x/crypto/argon2"
	_ "crypto/rand"
)

// AuthService defines the auth business logic interface.
// Создание пользователей идёт через UserService.Create (POST /users, admin-only).
// Публичная регистрация (POST /register) не поддерживается - см. удалённый Register handler.
type AuthService interface {
	Login(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error)
	RefreshToken(ctx context.Context, req models.RefreshTokenRequest) (*models.TokenPairResponse, error)
	Logout(ctx context.Context, username string, req models.LogoutRequest) error
	GetUserData(ctx context.Context, username string) (*models.UserDataResponse, error)
	GetCurrentUser(ctx context.Context, username string) (*models.CurrentUserResponse, error)
	GetUserTypes(ctx context.Context) ([]models.UserType, error)
}

// Claims is the JWT claims structure matching the Rust backend.
type Claims struct {
	UserID int `json:"user_id"`
	TypeID int `json:"type_id"`
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

func (s *authService) createAccessToken(username string, userID int, typeID int) (string, error) {
	claims := Claims{
		UserID: userID,
		TypeID: typeID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *authService) createRefreshJWT(username string) (string, error) {
	claims := Claims{
		TypeID: 0,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
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
)

// Login выполняет аутентификацию пользователя и возвращает пару токенов.
func (s *authService) Login(ctx context.Context, req models.LoginRequest) (*models.LoginResponse, error) {
	var user models.User
	err := s.db.WithContext(ctx).
		Preload("Organization").
		Preload("Company").
		Preload("UserType").
		Where("username = ?", req.Username).
		First(&user).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid credentials")
	}

	// Учётка заблокирована - не разрешаем попытки (даже с правильным паролем),
	// пока не истечёт lock-период. Это важно: иначе валидная попытка сбросила бы
	// счётчик и атакующий мог бы "разморозить" учётку угадав пароль в моменте.
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		remaining := int(time.Until(*user.LockedUntil).Seconds())
		return nil, echo.NewHTTPError(http.StatusTooManyRequests,
			fmt.Sprintf("Учётная запись временно заблокирована. Повторите через %d секунд.", remaining))
	}

	if !verifyPassword(user.Password, req.Password) {
		s.registerFailedLogin(ctx, &user)
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid credentials")
	}

	// Успешный вход - сбрасываем счётчик неудачных попыток и lock.
	if user.FailedLoginCount > 0 || user.LockedUntil != nil {
		s.db.WithContext(ctx).Model(&user).Updates(map[string]interface{}{
			"failed_login_count": 0,
			"locked_until":       nil,
		})
	}

	accessToken, err := s.createAccessToken(user.Username, user.ID, user.TypeID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create token")
	}

	refreshJWT, err := s.createRefreshJWT(user.Username)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create refresh token")
	}

	// Store hashed refresh token in DB
	rt := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashRefreshToken(refreshJWT),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}
	s.db.WithContext(ctx).Create(&rt)

	return &models.LoginResponse{
		Token:          accessToken,
		RefreshToken:   refreshJWT,
		Organization:   user.Organization.Name,
		OrganizationID: user.OrganizationID,
		Company:        user.Company.Name,
		CompanyID:      user.CompanyID,
		TypeID:         user.TypeID,
		UserType:       user.UserType.Name,
	}, nil
}

// RefreshToken обновляет пару access/refresh токенов с ротацией.
func (s *authService) RefreshToken(ctx context.Context, req models.RefreshTokenRequest) (*models.TokenPairResponse, error) {
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

	// Find and validate stored token
	tokenHash := hashRefreshToken(refreshToken)
	var storedToken models.RefreshToken
	err = s.db.WithContext(ctx).
		Where("user_id = ? AND token_hash = ? AND is_revoked = false", user.ID, tokenHash).
		First(&storedToken).Error
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Invalid refresh token")
	}

	// Revoke old token (one-time use)
	s.db.WithContext(ctx).Model(&storedToken).Update("is_revoked", true)

	// Create new pair
	newAccess, err := s.createAccessToken(username, user.ID, user.TypeID)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create token")
	}

	newRefresh, err := s.createRefreshJWT(username)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create refresh token")
	}

	// Store new refresh token
	rt := models.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashRefreshToken(newRefresh),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	}
	s.db.WithContext(ctx).Create(&rt)

	return &models.TokenPairResponse{
		Token:        newAccess,
		RefreshToken: newRefresh,
	}, nil
}

// Logout отзывает refresh-токен пользователя.
func (s *authService) Logout(ctx context.Context, username string, req models.LogoutRequest) error {
	var user models.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	tokenHash := hashRefreshToken(req.GetRefreshToken())
	s.db.WithContext(ctx).
		Where("user_id = ? AND token_hash = ?", user.ID, tokenHash).
		Delete(&models.RefreshToken{})

	return nil
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
		lockUntil := time.Now().Add(accountLockDuration)
		updates["locked_until"] = lockUntil
	}
	s.db.WithContext(ctx).Model(user).Updates(updates)
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
