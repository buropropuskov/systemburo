package handlers

import (
	"errors"
	"net/http"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// UsersHandler — HTTP-обработчики управления пользователями (admin-only).
type UsersHandler struct {
	service services.UserService
	// rotation - смена пароля с отправкой письмом (#1910). Подключается сеттером:
	// сервис создаётся позже хендлера, а в тестах его может не быть.
	rotation *services.PasswordRotationService
}

// SetRotationService подключает сервис смены пароля с отправкой письмом.
func (h *UsersHandler) SetRotationService(s *services.PasswordRotationService) {
	h.rotation = s
}

// NewUsersHandler создаёт новый экземпляр обработчика пользователей.
func NewUsersHandler(service services.UserService) *UsersHandler {
	return &UsersHandler{service: service}
}

// Create godoc
// @Summary      Создание пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.RegisterRequest true "Данные нового пользователя"
// @Success      200 {string} string "User created successfully"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users [post]
func (h *UsersHandler) Create(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	var req models.RegisterRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Create(c.Request().Context(), userID, req); err != nil {
		return err
	}
	return RespondMessage(c, "User created successfully")
}

// GetAll godoc
// @Summary      Получение списка всех пользователей
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array}  models.UserInfoResponse
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/all [get]
func (h *UsersHandler) GetAll(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	result, err := h.service.GetAll(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// GetRecipientCandidates godoc
// @Summary      Кандидаты в получатели заявки
// @Description  Пользователи, которых автор может добавить получателем заявки: коллеги по организации и компании плюс руководители. Доступно любому авторизованному - выбор получателя есть у всех, кто подаёт заявку, а список людей своей организации и так открыт (/organizations/{id}/users).
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array}  models.RecipientCandidate
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/recipient-candidates [get]
func (h *UsersHandler) GetRecipientCandidates(c echo.Context) error {
	username, _ := c.Get("username").(string)
	result, err := h.service.GetRecipientCandidates(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// UpdateType godoc
// @Summary      Обновление типа пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string                     true "Имя пользователя"
// @Param        request  body   models.UpdateUserTypeRequest true "Новый type_id"
// @Success      200 {string} string "User type updated successfully"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/type [put]
func (h *UsersHandler) UpdateType(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	var req models.UpdateUserTypeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateType(c.Request().Context(), userID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "User type updated successfully")
}

// UpdatePassword godoc
// @Summary      Обновление пароля пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string                       true "Имя пользователя"
// @Param        request  body   models.UpdatePasswordRequest true "Новый пароль"
// @Success      200 {string} string "Password updated successfully"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/password [put]
func (h *UsersHandler) UpdatePassword(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	var req models.UpdatePasswordRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdatePassword(c.Request().Context(), userID, username, req, requestMeta(c)); err != nil {
		return err
	}
	return RespondMessage(c, "Password updated successfully")
}

// ChangeOwnPassword godoc
// @Summary      Смена собственного пароля
// @Description  Меняет пароль текущего пользователя по подтверждению текущим паролем. Права не требуются.
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.ChangeOwnPasswordRequest true "Текущий и новый пароль"
// @Success      200 {string} string "Password changed successfully"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      429 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/me/password [put]
func (h *UsersHandler) ChangeOwnPassword(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	if userID == 0 {
		return echo.NewHTTPError(http.StatusUnauthorized, "Пользователь не авторизован")
	}
	var req models.ChangeOwnPasswordRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	// Маркер продления текущей сессии: она переживёт смену пароля, остальные
	// погаснут. Без cookie (запрос мимо браузера) сохранять нечего - тогда
	// отзываются все, как и раньше.
	keep := ""
	if cookie, err := c.Cookie(services.RefreshCookieName); err == nil && cookie != nil {
		keep = cookie.Value
	}
	if err := h.service.ChangeOwnPassword(c.Request().Context(), userID, req, requestMeta(c), keep); err != nil {
		return err
	}
	return RespondMessage(c, "Password changed successfully")
}

// UpdateInfo godoc
// @Summary      Обновление персональных данных пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string                       true "Имя пользователя"
// @Param        request  body   models.UpdateUserInfoRequest true "ФИО, должность, email, телефон"
// @Success      200 {string} string "User info updated successfully"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/info [put]
func (h *UsersHandler) UpdateInfo(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	var req models.UpdateUserInfoRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateInfo(c.Request().Context(), userID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "User info updated successfully")
}

// UpdateOrganization godoc
// @Summary      Обновление организации пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string                              true "Имя пользователя"
// @Param        request  body   models.UpdateUserOrganizationRequest true "Новый organization_id"
// @Success      200 {string} string "Organization updated successfully"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/organization [put]
func (h *UsersHandler) UpdateOrganization(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	var req models.UpdateUserOrganizationRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateOrganization(c.Request().Context(), userID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Organization updated successfully")
}

// UpdateCompany godoc
// @Summary      Обновление компании пользователя
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string                          true "Имя пользователя"
// @Param        request  body   models.UpdateUserCompanyRequest true "Новый company_id"
// @Success      200 {string} string "Company updated successfully"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/company [put]
func (h *UsersHandler) UpdateCompany(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	var req models.UpdateUserCompanyRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateCompany(c.Request().Context(), userID, username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Company updated successfully")
}

// Delete godoc
// @Summary      Архивация пользователя (soft-delete)
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string true "Имя пользователя"
// @Success      200 {object} map[string]string "message"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username} [delete]
func (h *UsersHandler) Delete(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	if err := h.service.Delete(c.Request().Context(), userID, username); err != nil {
		return err
	}
	return RespondMessage(c, "User archived successfully")
}

// Restore godoc
// @Summary      Восстановление пользователя из архива
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path   string true "Имя пользователя"
// @Success      200 {object} map[string]string "message"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /users/{username}/restore [post]
func (h *UsersHandler) Restore(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	username := c.Param("username")
	if err := h.service.Restore(c.Request().Context(), userID, username); err != nil {
		return err
	}
	return RespondMessage(c, "User restored successfully")
}

// BulkArchive godoc
// @Summary      Групповое архивирование пользователей
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUsernamesRequest true "Список username"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /users/bulk/archive [post]
func (h *UsersHandler) BulkArchive(c echo.Context) error {
	var req services.BulkUsernamesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.Usernames) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны пользователи")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkArchive(c.Request().Context(), userID, req.Usernames)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkRestore godoc
// @Summary      Групповое восстановление пользователей
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUsernamesRequest true "Список username"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /users/bulk/restore [post]
func (h *UsersHandler) BulkRestore(c echo.Context) error {
	var req services.BulkUsernamesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.Usernames) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны пользователи")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkRestore(c.Request().Context(), userID, req.Usernames)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkUpdateType godoc
// @Summary      Групповая смена типа пользователей
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUserTypeRequest true "Список username и type_id"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /users/bulk/type [post]
func (h *UsersHandler) BulkUpdateType(c echo.Context) error {
	var req services.BulkUserTypeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.Usernames) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны пользователи")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkUpdateType(c.Request().Context(), userID, req.Usernames, req.TypeID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkAssignOrganization godoc
// @Summary      Групповое назначение организации пользователям
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUserOrganizationRequest true "Список username и organization_id"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /users/bulk/organization [post]
func (h *UsersHandler) BulkAssignOrganization(c echo.Context) error {
	var req services.BulkUserOrganizationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.Usernames) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны пользователи")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkAssignOrganization(c.Request().Context(), userID, req.Usernames, req.OrganizationID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkAssignCompany godoc
// @Summary      Групповое назначение компании пользователям
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUserCompanyRequest true "Список username и company_id"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /users/bulk/company [post]
func (h *UsersHandler) BulkAssignCompany(c echo.Context) error {
	var req services.BulkUserCompanyRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.Usernames) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны пользователи")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkAssignCompany(c.Request().Context(), userID, req.Usernames, req.CompanyID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// GetHistory godoc
// @Summary      История изменений пользователя
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        username path int true "Имя пользователя"
// @Success      200 {array} models.UserHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/history [get]
func (h *UsersHandler) GetHistory(c echo.Context) error {
	username := c.Param("username")
	items, err := h.service.GetHistory(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// GetUserUnloadPlaces godoc
// @Summary      Получение мест разгрузки охранника
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Имя пользователя"
// @Success      200 {array} models.UnloadPlace
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/unload-places [get]
func (h *UsersHandler) GetUserUnloadPlaces(c echo.Context) error {
	username := c.Param("username")
	places, err := h.service.GetUserUnloadPlaces(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, places)
}

// SetUserUnloadPlaces godoc
// @Summary      Замена мест разгрузки охранника
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Имя пользователя"
// @Param        request body models.SetUserUnloadPlacesRequest true "Список ID мест разгрузки"
// @Success      200 {string} string "Unload places updated successfully"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/unload-places [put]
func (h *UsersHandler) SetUserUnloadPlaces(c echo.Context) error {
	username := c.Param("username")
	var req models.SetUserUnloadPlacesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.SetUserUnloadPlaces(c.Request().Context(), username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Unload places updated successfully")
}

// GetUserTables godoc
// @Summary      Получение мест прохода охранника
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Имя пользователя"
// @Success      200 {array} models.SystemTable
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/tables [get]
func (h *UsersHandler) GetUserTables(c echo.Context) error {
	username := c.Param("username")
	tables, err := h.service.GetUserTables(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, tables)
}

// SetUserTables godoc
// @Summary      Замена мест прохода охранника
// @Tags         users
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Имя пользователя"
// @Param        request body models.SetUserTablesRequest true "Список ID мест прохода"
// @Success      200 {string} string "Tables updated successfully"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /users/{username}/tables [put]
func (h *UsersHandler) SetUserTables(c echo.Context) error {
	username := c.Param("username")
	var req models.SetUserTablesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.SetUserTables(c.Request().Context(), username, req); err != nil {
		return err
	}
	return RespondMessage(c, "Tables updated successfully")
}

// RotatePassword godoc
// @Summary      Сменить пароль работнику и отправить письмом
// @Description  Генерирует пароль по действующей политике, меняет его и отправляет работнику на почту. Требует настроенной почты и указанного адреса.
// @Tags         users
// @Produce      json
// @Security     BearerAuth
// @Param        username path string true "Имя пользователя"
// @Success      200 {string} string "Password rotated"
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      412 {object} models.HTTPError
// @Router       /users/{username}/rotate-password [post]
//
// RotatePassword меняет пароль работнику и отправляет его письмом. Закрывает
// случай «работник потерял пароль»: до этого пароль придумывали руками и
// диктовали по телефону, то есть он проходил через третьи уши.
func (h *UsersHandler) RotatePassword(c echo.Context) error {
	if h.rotation == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "Смена пароля с отправкой письмом недоступна")
	}
	username := c.Param("username")
	if err := h.rotation.RotateOne(c.Request().Context(), username, GetUserID(c)); err != nil {
		if errors.Is(err, services.ErrRotationMailNotConfigured) {
			return echo.NewHTTPError(http.StatusPreconditionFailed,
				"Почта не настроена: отправить новый пароль нечем")
		}
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return RespondMessage(c, "Password rotated")
}
