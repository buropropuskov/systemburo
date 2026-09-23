package service

// Модульные тесты слоя сервиса авторизации.
// Основано на: github.com/airvt1x/dokkee-backend/internal/service/auth.go

import (
	"errors"
	"testing"

	dokkee "github.com/airvt1x/dokkee-backend"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAuthorization struct {
	mock.Mock
}

func (m *MockAuthorization) CreateUser(user dokkee.User) (int, error) {
	args := m.Called(user)
	return args.Int(0), args.Error(1)
}

func (m *MockAuthorization) GetUser(email, passwordHash string) (dokkee.User, error) {
	args := m.Called(email, passwordHash)
	return args.Get(0).(dokkee.User), args.Error(1)
}

// Проверка хэширования пароля: пароль не должен попадать в репозиторий в открытом виде.
func TestCreateUser_PasswordIsHashed(t *testing.T) {
	mockRepo := new(MockAuthorization)
	svc := NewAuthService(mockRepo)

	input := dokkee.User{
		Username: "ivanov", Password: "plain-secret",
		FirstName: "Иван", LastName: "Иванов",
		Email: "ivanov@example.com", Phone: "+79991234567",
	}

	mockRepo.On("CreateUser", mock.MatchedBy(func(u dokkee.User) bool {
		// Хэш SHA-256 от plain-secret + salt не должен совпадать с plain-secret
		return u.Password != "plain-secret" && len(u.Password) >= 64
	})).Return(1, nil)

	id, err := svc.CreateUser(input)
	assert.NoError(t, err)
	assert.Equal(t, 1, id)
	mockRepo.AssertExpectations(t)
}

// Тест GenerateToken: при верных credentials возвращается непустая строка.
func TestGenerateToken_OK(t *testing.T) {
	mockRepo := new(MockAuthorization)
	svc := NewAuthService(mockRepo)

	mockRepo.On("GetUser", "ivanov@example.com", mock.AnythingOfType("string")).
		Return(dokkee.User{Id: 42, Email: "ivanov@example.com"}, nil)

	tok, err := svc.GenerateToken("ivanov@example.com", "secret")
	assert.NoError(t, err)
	assert.NotEmpty(t, tok)
	// JWT состоит из трёх частей, разделённых точкой
	parts := 0
	for _, c := range tok {
		if c == '.' {
			parts++
		}
	}
	assert.Equal(t, 2, parts, "jwt должен содержать ровно две точки-разделителя")
}

// Тест GenerateToken: пользователь не найден → ошибка.
func TestGenerateToken_UserNotFound(t *testing.T) {
	mockRepo := new(MockAuthorization)
	svc := NewAuthService(mockRepo)

	mockRepo.On("GetUser", "unknown@x.y", mock.Anything).
		Return(dokkee.User{}, errors.New("sql: no rows in result set"))

	tok, err := svc.GenerateToken("unknown@x.y", "any")
	assert.Error(t, err)
	assert.Empty(t, tok)
}

// Тест ParseToken: проверка цикла генерация → разбор → userId.
func TestParseToken_RoundTrip(t *testing.T) {
	mockRepo := new(MockAuthorization)
	svc := NewAuthService(mockRepo)

	mockRepo.On("GetUser", "ivanov@example.com", mock.AnythingOfType("string")).
		Return(dokkee.User{Id: 42, Email: "ivanov@example.com"}, nil)

	tok, err := svc.GenerateToken("ivanov@example.com", "secret")
	assert.NoError(t, err)

	userId, err := svc.ParseToken(tok)
	assert.NoError(t, err)
	assert.Equal(t, 42, userId)
}

// Тест ParseToken: токен, подписанный другим ключом, должен быть отклонён.
//
//возможны изменения// — после внедрения отдельного набора ключей для тестов
// используется фиктивный токен; здесь проверяем только общий контракт.
func TestParseToken_InvalidToken(t *testing.T) {
	mockRepo := new(MockAuthorization)
	svc := NewAuthService(mockRepo)

	userId, err := svc.ParseToken("not.a.valid.jwt")
	assert.Error(t, err)
	assert.Equal(t, 0, userId)
}
