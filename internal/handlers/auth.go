package handlers

import (
	"net/http"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// refreshCookiePath - под /api, потому что читают cookie только продление
// сеанса и выход, а раздача загруженных файлов проверяет её как пропуск
// (см. middleware.FileAccess). Раньше стоял "/", и маркер продления ездил с
// каждым запросом за картинкой и на страницу pgAdmin.
const refreshCookiePath = "/api"

// legacyRefreshCookiePath - прежний путь cookie. Выход чистит и его: у тех,
// кто вошёл до сужения пути, в браузере лежит cookie с Path=/, и без явного
// удаления она пережила бы выход и осталась бы годной ещё на неделю.
const legacyRefreshCookiePath = "/"

type AuthHandler struct {
	service       services.AuthService
	maintenance   services.MaintenanceService
	cookieSecure  bool
	refreshMaxAge int
}

// NewAuthHandler создаёт новый экземпляр AuthHandler.
// cookieSecure должен быть true в prod/staging (HTTPS), false только для
// локальной разработки на http://localhost.
// refreshTTL задаёт MaxAge для refresh cookie в секундах.
// maintenance используется для 503-bypass не-админам на /login при
// включённом режиме техработ.
func NewAuthHandler(service services.AuthService, maintenance services.MaintenanceService, cookieSecure bool, refreshTTL time.Duration) *AuthHandler {
	return &AuthHandler{
		service:       service,
		maintenance:   maintenance,
		cookieSecure:  cookieSecure,
		refreshMaxAge: int(refreshTTL.Seconds()),
	}
}

// setRefreshCookie выставляет HttpOnly refresh cookie.
func (h *AuthHandler) setRefreshCookie(c echo.Context, token string) {
	c.SetCookie(&http.Cookie{
		Name:     services.RefreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		MaxAge:   h.refreshMaxAge,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearRefreshCookie удаляет refresh cookie (MaxAge: -1) по текущему и прежнему
// пути: браузер различает cookie с разным Path и сам старую не удалит.
func (h *AuthHandler) clearRefreshCookie(c echo.Context) {
	for _, path := range []string{refreshCookiePath, legacyRefreshCookiePath} {
		c.SetCookie(&http.Cookie{
			Name:     services.RefreshCookieName,
			Value:    "",
			Path:     path,
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   h.cookieSecure,
			SameSite: http.SameSiteStrictMode,
		})
	}
}

// Login godoc
// @Summary      Авторизация
// @Description  Проверяет credentials и возвращает access + refresh токены
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body models.LoginRequest true "Логин и пароль"
// @Success      200 {object} models.LoginResponse
// @Failure      401 {object} models.HTTPError "Неверный логин или пароль"
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
	// Maintenance-bypass: super-admin всегда проходит, иначе -
	// отзываем только что выданный refresh и возвращаем 503. Проверку
	// делаем после пароля чтобы не выдавать 503 как oracle для подбора
	// учёток (одинаковая задержка на верный и неверный пароль).
	if h.maintenance != nil && !resp.IsSuperAdmin {
		if st := h.maintenance.GetStatusCached(c.Request().Context()); st != nil && st.Enabled {
			_ = h.service.Logout(c.Request().Context(), req.Username,
				models.LogoutRequest{RefreshToken: resp.RefreshToken}, requestMeta(c))
			return echo.NewHTTPError(http.StatusServiceUnavailable,
				"Сервис временно недоступен: технические работы")
		}
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
// @Success      200 {object} models.TokenPairResponse
// @Failure      401 {object} models.HTTPError "Invalid refresh token"
// @Router       /refresh-token [post]
func (h *AuthHandler) RefreshToken(c echo.Context) error {
	// Только HttpOnly cookie: тела запроса здесь больше нет. Приём маркера из
	// тела оставался с тех пор, как он ещё отдавался клиенту в JSON; фронт им не
	// пользуется, а лишний путь в обновлении сеанса приходится держать в голове
	// каждому, кто трогает авторизацию.
	ck, err := c.Cookie(services.RefreshCookieName)
	if err != nil || ck.Value == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "Invalid refresh token")
	}
	req := models.RefreshTokenRequest{RefreshToken: ck.Value}
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
// @Success      200 {string} string "Logged out successfully"
// @Failure      401 {object} models.HTTPError
// @Router       /logout [post]
func (h *AuthHandler) Logout(c echo.Context) error {
	username := c.Get("username").(string)
	// Маркер берётся только из cookie. Её отсутствие выходу не мешает: cookie
	// всё равно чистится, а сеанс без маркера отзывать нечего.
	var req models.LogoutRequest
	if ck, err := c.Cookie(services.RefreshCookieName); err == nil {
		req.RefreshToken = ck.Value
	}
	if err := h.service.Logout(c.Request().Context(), username, req, requestMeta(c)); err != nil {
		// Всё равно чистим cookie - даже если DB-запись не удалилась.
		h.clearRefreshCookie(c)
		return err
	}
	h.clearRefreshCookie(c)
	return RespondMessage(c, "Logged out successfully")
}

// LogoutAll godoc
// @Summary      Выйти со всех устройств
// @Description  Отзывает все активные refresh-токены пользователя
// @Tags         auth
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]int "количество отозванных сессий в поле revoked"
// @Failure      401 {object} models.HTTPError
// @Router       /logout-all [post]
func (h *AuthHandler) LogoutAll(c echo.Context) error {
	username := c.Get("username").(string)
	revoked, err := h.service.LogoutAll(c.Request().Context(), username)
	if err != nil {
		return err
	}
	h.clearRefreshCookie(c)
	return RespondSuccess(c, map[string]int{"revoked": revoked})
}

// ResetLockout godoc
// @Summary      Снять блокировку входа
// @Description  Обнуляет счётчик неудачных попыток, лестницу кулдаунов и сам лок учётной записи
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Логин пользователя"
// @Success      200 {object} map[string]bool "reset=false, если блокировки не было"
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/reset-lockout [post]
//
// ResetLockout -- POST /users/:username/reset-lockout. Права проверяет
// middleware (page.admin.users) -- тот же гейт, что у смены пароля.
func (h *AuthHandler) ResetLockout(c echo.Context) error {
	username := c.Param("username")
	if username == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Не указан пользователь")
	}
	reset, err := h.service.ResetLoginLockout(c.Request().Context(), username, GetUserID(c))
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]bool{"reset": reset})
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
