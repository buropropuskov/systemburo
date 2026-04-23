package handlers

import (
	"net/http"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

const refreshCookieName = "refresh_token"

type AuthHandler struct {
	service       services.AuthService
	cookieSecure  bool
	refreshMaxAge int
}

// NewAuthHandler создаёт новый экземпляр AuthHandler.
// cookieSecure должен быть true в prod/staging (HTTPS), false только для
// локальной разработки на http://localhost.
// refreshTTL задаёт MaxAge для refresh cookie в секундах.
func NewAuthHandler(service services.AuthService, cookieSecure bool, refreshTTL time.Duration) *AuthHandler {
	return &AuthHandler{
		service:       service,
		cookieSecure:  cookieSecure,
		refreshMaxAge: int(refreshTTL.Seconds()),
	}
}

// setRefreshCookie выставляет HttpOnly refresh cookie.
func (h *AuthHandler) setRefreshCookie(c echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   h.refreshMaxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearRefreshCookie удаляет refresh cookie (MaxAge: -1).
func (h *AuthHandler) clearRefreshCookie(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
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
	resp, err := h.service.Login(c.Request().Context(), req, requestMeta(c))
	if err != nil {
		return err
	}
	// refresh_token уходит в HttpOnly cookie, в JSON его не отдаём.
	h.setRefreshCookie(c, resp.RefreshToken)
	resp.RefreshToken = ""
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
	// Берём refresh_token из HttpOnly cookie. Body оставлен для
	// обратной совместимости - если cookie нет, fallback на body.
	if ck, err := c.Cookie(refreshCookieName); err == nil && ck.Value != "" {
		req.RefreshToken = ck.Value
	} else {
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
		}
	}
	resp, err := h.service.RefreshToken(c.Request().Context(), req, requestMeta(c))
	if err != nil {
		return err
	}
	// Новый refresh_token - снова в cookie.
	h.setRefreshCookie(c, resp.RefreshToken)
	resp.RefreshToken = ""
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
	// Берём refresh из cookie, fallback на body.
	if ck, err := c.Cookie(refreshCookieName); err == nil && ck.Value != "" {
		req.RefreshToken = ck.Value
	} else {
		_ = c.Bind(&req)
	}
	if err := h.service.Logout(c.Request().Context(), username, req, requestMeta(c)); err != nil {
		// Всё равно чистим cookie - даже если DB-запись не удалилась.
		h.clearRefreshCookie(c)
		return err
	}
	h.clearRefreshCookie(c)
	return RespondMessage(c, "Logged out successfully")
}

// requestMeta - helper для сбора IP/UA из echo.Context в services.RequestMeta.
func requestMeta(c echo.Context) *services.RequestMeta {
	return &services.RequestMeta{
		IPAddress: c.RealIP(),
		UserAgent: c.Request().UserAgent(),
	}
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
