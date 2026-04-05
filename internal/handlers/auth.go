package handlers

import (
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	service services.AuthService
}

func NewAuthHandler(service services.AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

// Register godoc
// @Summary      Регистрация пользователя
// @Description  Создаёт нового пользователя с указанными данными
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.RegisterRequest true "Данные регистрации"
// @Success      200 {string} string "User registered successfully"
// @Failure      400 {object} models.HTTPError "Username already exists"
// @Router       /register [post]
func (h *AuthHandler) Register(c echo.Context) error {
	var req models.RegisterRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Register(c.Request().Context(), req); err != nil {
		return err
	}
	return RespondMessage(c, "User registered successfully")
}

// Login godoc
// @Summary      Авторизация
// @Description  Проверяет credentials и возвращает access + refresh токены
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.LoginRequest true "Логин и пароль"
// @Success      200 {object} models.LoginResponse
// @Failure      401 {object} models.HTTPError "Invalid credentials"
// @Router       /login [post]
func (h *AuthHandler) Login(c echo.Context) error {
	var req models.LoginRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := h.service.Login(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// RefreshToken godoc
// @Summary      Обновление токена
// @Description  Выдаёт новую пару access + refresh токенов по refresh token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.RefreshTokenRequest true "Refresh token"
// @Success      200 {object} models.TokenPairResponse
// @Failure      401 {object} models.HTTPError "Invalid refresh token"
// @Router       /refresh-token [post]
func (h *AuthHandler) RefreshToken(c echo.Context) error {
	var req models.RefreshTokenRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	resp, err := h.service.RefreshToken(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// Logout godoc
// @Summary      Выход из системы
// @Description  Отзывает refresh token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.LogoutRequest true "Refresh token для отзыва"
// @Success      200 {string} string "Logged out successfully"
// @Failure      401 {object} models.HTTPError
// @Router       /logout [post]
func (h *AuthHandler) Logout(c echo.Context) error {
	username := c.Get("username").(string)
	var req models.LogoutRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if err := h.service.Logout(c.Request().Context(), username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Logged out successfully")
}

// GetUserData godoc
// @Summary      Данные текущего пользователя (краткие)
// @Description  Возвращает основные данные авторизованного пользователя
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.UserDataResponse
// @Failure      401 {object} models.HTTPError
// @Router       /user-data [get]
func (h *AuthHandler) GetUserData(c echo.Context) error {
	username := c.Get("username").(string)
	resp, err := h.service.GetUserData(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// GetCurrentUser godoc
// @Summary      Полный профиль текущего пользователя
// @Description  Возвращает все данные авторизованного пользователя включая тип и организацию
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.CurrentUserResponse
// @Failure      401 {object} models.HTTPError
// @Router       /users/me [get]
func (h *AuthHandler) GetCurrentUser(c echo.Context) error {
	username := c.Get("username").(string)
	resp, err := h.service.GetCurrentUser(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// GetUserTypes godoc
// @Summary      Список типов пользователей
// @Description  Возвращает все типы пользователей (публичный эндпоинт)
// @Tags         auth
// @Produce      json
// @Success      200 {array} models.UserType
// @Router       /user-types [get]
func (h *AuthHandler) GetUserTypes(c echo.Context) error {
	types, err := h.service.GetUserTypes(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, types)
}
