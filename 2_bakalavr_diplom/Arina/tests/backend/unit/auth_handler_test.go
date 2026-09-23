package handler

// Расширенный набор модульных тестов для обработчиков /auth.
// Базовые позитивные сценарии уже присутствуют в репозитории (auth_test.go),
// здесь добавлены негативные кейсы и проверки middleware.
//
// Основано на: github.com/airvt1x/dokkee-backend/internal/handler/auth.go
// Зависимости: github.com/stretchr/testify, github.com/gin-gonic/gin

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	dokkee "github.com/airvt1x/dokkee-backend"
	"github.com/airvt1x/dokkee-backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockAuthorizationService struct {
	mock.Mock
}

func (m *MockAuthorizationService) CreateUser(user dokkee.User) (int, error) {
	args := m.Called(user)
	return args.Int(0), args.Error(1)
}

func (m *MockAuthorizationService) GenerateToken(email, password string) (string, error) {
	args := m.Called(email, password)
	return args.String(0), args.Error(1)
}

func (m *MockAuthorizationService) ParseToken(token string) (int, error) {
	args := m.Called(token)
	return args.Int(0), args.Error(1)
}

// Тест sign-up: валидный ввод → 200 и id пользователя.
func TestSignUp_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockAuthorizationService)
	h := &Handler{services: &service.Service{Authorization: mockSvc}}

	user := dokkee.User{
		Username: "ivanov", Password: "secret123",
		FirstName: "Иван", LastName: "Иванов",
		Email: "ivanov@example.com", Phone: "+79991234567",
	}
	mockSvc.On("CreateUser", mock.AnythingOfType("dokkee.User")).Return(42, nil)

	body, _ := json.Marshal(user)
	req := httptest.NewRequest(http.MethodPost, "/auth/sign-up", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/auth/sign-up", h.signUp)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(42), resp["id"])
	mockSvc.AssertExpectations(t)
}

// Негативный тест: невалидный JSON → 400.
func TestSignUp_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{services: &service.Service{Authorization: new(MockAuthorizationService)}}

	req := httptest.NewRequest(http.MethodPost, "/auth/sign-up",
		bytes.NewBufferString(`{not-a-json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/auth/sign-up", h.signUp)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Негативный тест: обязательные поля отсутствуют → 400.
func TestSignUp_MissingRequiredFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{services: &service.Service{Authorization: new(MockAuthorizationService)}}

	// отсутствует email — обязательное поле в модели User
	body := `{"username":"u","password":"p","first_name":"F","last_name":"L","phone":"+79991112233"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/sign-up", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/auth/sign-up", h.signUp)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// Негативный тест: репозиторий вернул ошибку (дубликат email) → 500.
// //возможны изменения// — текущий handler возвращает 500 для любой ошибки сервиса;
// после внедрения типизированных ошибок код должен стать 409 Conflict.
func TestSignUp_DuplicateEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockAuthorizationService)
	h := &Handler{services: &service.Service{Authorization: mockSvc}}

	user := dokkee.User{
		Username: "ivanov", Password: "secret",
		FirstName: "И", LastName: "И",
		Email: "dup@example.com", Phone: "+79991234567",
	}
	mockSvc.On("CreateUser", mock.AnythingOfType("dokkee.User")).
		Return(0, errors.New("pq: duplicate key value violates unique constraint"))

	body, _ := json.Marshal(user)
	req := httptest.NewRequest(http.MethodPost, "/auth/sign-up", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/auth/sign-up", h.signUp)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Позитивный sign-in: верные credentials → 200 и token.
func TestSignIn_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockAuthorizationService)
	h := &Handler{services: &service.Service{Authorization: mockSvc}}

	mockSvc.On("GenerateToken", "ivanov@example.com", "secret").
		Return("eyJhbGciOiJIUzI1NiJ9.xyz.sig", nil)

	body := `{"email":"ivanov@example.com","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/sign-in", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/auth/sign-in", h.signIn)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.xyz.sig", resp["token"])
}

// Негативный sign-in: неверный пароль → 500 (после введения типизированных ошибок — 401).
func TestSignIn_InvalidCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockAuthorizationService)
	h := &Handler{services: &service.Service{Authorization: mockSvc}}

	mockSvc.On("GenerateToken", "a@b.c", "wrong").
		Return("", errors.New("sql: no rows in result set"))

	body := `{"email":"a@b.c","password":"wrong"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/sign-in", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/auth/sign-in", h.signIn)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// Проверка middleware userIdentity: пустой заголовок Authorization → 401.
//
//возможны изменения// — в репозитории middleware определён (middleware.go),
// но пока не прикручен ни к одному маршруту. Тест гарантирует корректное
// поведение при подключении в Handler.InitRoutes.
func TestMiddleware_EmptyAuthHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{services: &service.Service{Authorization: new(MockAuthorizationService)}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents", nil)
	w := httptest.NewRecorder()

	router := gin.New()
	api := router.Group("/api/v1", h.userIdentity)
	api.GET("/documents", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Невалидный формат Authorization (без Bearer) → 401.
func TestMiddleware_MalformedAuthHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &Handler{services: &service.Service{Authorization: new(MockAuthorizationService)}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents", nil)
	req.Header.Set("Authorization", "BrokenToken")
	w := httptest.NewRecorder()

	router := gin.New()
	api := router.Group("/api/v1", h.userIdentity)
	api.GET("/documents", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// Валидный токен → middleware проставляет userId в контекст, запрос проходит до хэндлера.
func TestMiddleware_ValidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockSvc := new(MockAuthorizationService)
	h := &Handler{services: &service.Service{Authorization: mockSvc}}

	mockSvc.On("ParseToken", "valid.jwt.token").Return(777, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents", nil)
	req.Header.Set("Authorization", "Bearer valid.jwt.token")
	w := httptest.NewRecorder()

	var captured int
	router := gin.New()
	api := router.Group("/api/v1", h.userIdentity)
	api.GET("/documents", func(c *gin.Context) {
		captured = c.GetInt("userId")
		c.Status(http.StatusOK)
	})
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 777, captured)
}
