// Листинг 3.3 — Модульный тест обработчика /auth/sign-up (Go + testify)
// Файл: tests/backend/unit/auth_handler_test.go
// Основан на internal/handler/auth.go из dokkee-backend.

package handler

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

func (m *MockAuthorizationService) CreateUser(u dokkee.User) (int, error) {
	a := m.Called(u); return a.Int(0), a.Error(1)
}
func (m *MockAuthorizationService) GenerateToken(e, p string) (string, error) {
	a := m.Called(e, p); return a.String(0), a.Error(1)
}
func (m *MockAuthorizationService) ParseToken(t string) (int, error) {
	a := m.Called(t); return a.Int(0), a.Error(1)
}

func TestSignUp_OK(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(MockAuthorizationService)
	h := &Handler{services: &service.Service{Authorization: svc}}

	user := dokkee.User{
		Username: "ivanov", Password: "secret123",
		FirstName: "Иван", LastName: "Иванов",
		Email: "ivanov@example.com", Phone: "+79991234567",
	}
	svc.On("CreateUser", mock.AnythingOfType("dokkee.User")).Return(42, nil)

	body, _ := json.Marshal(user)
	req := httptest.NewRequest(http.MethodPost, "/auth/sign-up", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/auth/sign-up", h.signUp)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	svc.AssertExpectations(t)
}

func TestSignUp_DuplicateEmail(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := new(MockAuthorizationService)
	h := &Handler{services: &service.Service{Authorization: svc}}

	svc.On("CreateUser", mock.AnythingOfType("dokkee.User")).
		Return(0, errors.New("pq: duplicate key value violates unique constraint"))

	body, _ := json.Marshal(dokkee.User{
		Email: "dup@example.com", Phone: "+79991234567",
		Username: "u", Password: "p", FirstName: "F", LastName: "L",
	})
	req := httptest.NewRequest(http.MethodPost, "/auth/sign-up", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router := gin.New()
	router.POST("/auth/sign-up", h.signUp)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
