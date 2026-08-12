package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// OrganizationHandler содержит HTTP-обработчики для управления организациями.
type OrganizationHandler struct {
	service  services.OrganizationService
	db       *gorm.DB
	resolver *services.PermissionResolver
}

// NewOrganizationHandler создаёт новый экземпляр обработчика организаций.
func NewOrganizationHandler(service services.OrganizationService, db *gorm.DB, resolver *services.PermissionResolver) *OrganizationHandler {
	return &OrganizationHandler{service: service, db: db, resolver: resolver}
}

// GetAll godoc
// @Summary      Получить все организации
// @Description  Возвращает список всех организаций (id, name)
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} services.OrganizationInfoResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /organizations [get]
func (h *OrganizationHandler) GetAll(c echo.Context) error {
	orgs, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, orgs)
}

// Suggest godoc
// @Summary      Подсказки организаций по наименованию
// @Description  Близкие к запросу проверенные организации (максимум 5) для ручного ввода наименования в заявке. Требует права application.organization.override. Запрос короче трёх символов даёт пустой список.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        q query string false "Наименование или его часть; короче трёх символов - пустой список"
// @Success      200 {object} services.DirectorySuggestAnswer
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /organizations/suggest [get]
func (h *OrganizationHandler) Suggest(c echo.Context) error {
	suggestions, err := h.service.Suggest(c.Request().Context(), c.QueryParam("q"))
	if err != nil {
		return err
	}
	return RespondSuccess(c, suggestions)
}

// ApproveModeration godoc
// @Summary      Подтвердить организацию «на проверке»
// @Description  Разбор записи, заведённой из заявки (#1437). Требует права application.organization.moderate. Если наименование столкнулось с уже проверенной записью, ответ приходит со status=conflict и самой записью - её предлагают выбрать вместо черновика; столкновение с другим черновиком даёт 400, его разбирают первым.
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {object} services.DirectoryModerationResult
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /organizations/{id}/moderation/approve [post]
func (h *OrganizationHandler) ApproveModeration(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}
	userID, _ := c.Get("user_id").(int)
	result, err := h.service.ApproveModeration(c.Request().Context(), userID, id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// RenameModeration godoc
// @Summary      Исправить наименование организации «на проверке»
// @Description  Правит наименование черновика и считает запись разобранной. Требует права application.organization.moderate. При совпадении с проверенной записью возвращает status=conflict, при совпадении с другим черновиком - 400.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Param        request body services.DirectoryRenameRequest true "Новое наименование"
// @Success      200 {object} services.DirectoryModerationResult
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /organizations/{id}/moderation/rename [patch]
func (h *OrganizationHandler) RenameModeration(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}
	var req services.DirectoryRenameRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	result, err := h.service.RenameModeration(c.Request().Context(), userID, id, req.Name)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// MergeModeration godoc
// @Summary      Привязать организацию «на проверке» к существующей
// @Description  Переносит заявки, вложения, машины, сотрудников и привязки черновика на выбранную проверенную организацию и удаляет черновик. Требует права application.organization.moderate.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации «на проверке»"
// @Param        request body services.DirectoryMergeRequest true "ID целевой организации"
// @Success      200 {object} services.DirectoryMergeResult
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /organizations/{id}/moderation/merge [post]
func (h *OrganizationHandler) MergeModeration(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}
	var req services.DirectoryMergeRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	result, err := h.service.MergeModeration(c.Request().Context(), userID, id, req.TargetID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// Create godoc
// @Summary      Создать организацию
// @Description  Создаёт новую организацию. Требует права buropropuskov
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.CreateOrganizationRequest true "Данные новой организации"
// @Success      200 {object} services.OrganizationInfoResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations [post]
func (h *OrganizationHandler) Create(c echo.Context) error {
	var req services.CreateOrganizationRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	org, err := h.service.Create(c.Request().Context(), userID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, org)
}

// Update godoc
// @Summary      Обновить организацию
// @Description  Обновляет название организации по ID. Требует права buropropuskov
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Param        request body services.CreateOrganizationRequest true "Обновлённые данные организации"
// @Success      200 {object} services.OrganizationInfoResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/{id} [put]
func (h *OrganizationHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	var req services.CreateOrganizationRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	org, err := h.service.Update(c.Request().Context(), userID, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, org)
}

// Delete godoc
// @Summary      Удалить организацию
// @Description  Удаляет организацию по ID. Нельзя удалить если есть привязанные пользователи. Требует права buropropuskov
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Router       /organizations/{id} [delete]
func (h *OrganizationHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.Delete(c.Request().Context(), userID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Organization archived")
}

// Restore godoc
// @Summary      Восстановить организацию из архива
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {string} string "Organization restored"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/{id}/restore [post]
func (h *OrganizationHandler) Restore(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Restore(c.Request().Context(), userID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Organization restored")
}

// GetHistory godoc
// @Summary      История изменений организации
// @Tags         organizations
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {array} models.OrganizationHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /organizations/{id}/history [get]
func (h *OrganizationHandler) GetHistory(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}
	items, err := h.service.GetHistory(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// GetWithUsers godoc
// @Summary      Получить организации с количеством пользователей
// @Description  Возвращает список организаций с количеством привязанных пользователей
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} services.OrganizationWithUsersResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /organizations/with-users [get]
func (h *OrganizationHandler) GetWithUsers(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	orgs, err := h.service.GetWithUsers(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, orgs)
}

// GetWithUsersExtended godoc
// @Summary      Получить организации с расширенной информацией
// @Description  Возвращает организации с количеством пользователей и местами разгрузки
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} map[string]interface{}
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /organizations/with-users-extended [get]
func (h *OrganizationHandler) GetWithUsersExtended(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	orgs, err := h.service.GetWithUsersExtended(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, orgs)
}

// GetMyOrganization godoc
// @Summary      Получить организацию текущего пользователя
// @Description  Возвращает организацию, к которой привязан текущий авторизованный пользователь
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} services.MyOrganizationResponse
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /get-organization [get]
func (h *OrganizationHandler) GetMyOrganization(c echo.Context) error {
	username := c.Get("username").(string)
	resp, err := h.service.GetMyOrganization(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// GetOrganizationUsers godoc
// @Summary      Получить пользователей организации
// @Description  Возвращает список ответственных пользователей организации. Маршрут открыт любому вошедшему (его же читает форма подачи заявки у своей организации), поэтому required_approval виден только тем, у кого есть право на раздел справочников, и заявителю - для его СОБСТВЕННОЙ организации (#2013). Остальным поле приходит null.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {array} services.OrganizationUserResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /organizations/{id}/users [get]
func (h *OrganizationHandler) GetOrganizationUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	users, err := h.service.GetOrganizationUsers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if !canSeeRequiredApproval(c, h.db, h.resolver, id, func(o callerOwnDirectoryIDs) *int { return o.OrganizationID }) {
		for i := range users {
			users[i].RequiredApproval = nil
		}
	}
	return RespondSuccess(c, users)
}

// GetMembers godoc
// @Summary      Получить участников организации
// @Description  Возвращает пользователей, привязанных к организации через organization_id (не ответственных)
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {array} services.MemberResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /organizations/{id}/members [get]
func (h *OrganizationHandler) GetMembers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	members, err := h.service.GetMembers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, members)
}

// ReassignUsers godoc
// @Summary      Перенести всех блокирующих пользователей в другую организацию
// @Description  Переносит активных участников организации в целевую (target_id),
// @Description  освобождая исходную для архивации. Требует права admin.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID исходной организации"
// @Param        request body services.ReassignUsersRequest true "ID целевой организации"
// @Success      200 {object} map[string]int "reassigned"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /organizations/{id}/reassign-users [post]
func (h *OrganizationHandler) ReassignUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}
	var req services.ReassignUsersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.TargetID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не указана целевая организация")
	}
	userID, _ := c.Get("user_id").(int)
	count, err := h.service.ReassignMembers(c.Request().Context(), userID, id, req.TargetID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]int{"reassigned": count})
}

// UpdateOrganizationUsers godoc
// @Summary      Обновить пользователей организации
// @Description  Заменяет список ответственных пользователей организации (replace-стратегия)
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Param        request body services.UpdateOrganizationUsersRequest true "Список ответственных пользователей"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /organizations/{id}/users [put]
func (h *OrganizationHandler) UpdateOrganizationUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	var req services.UpdateOrganizationUsersRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.UpdateOrganizationUsers(c.Request().Context(), userID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Organization users updated successfully")
}

// GetOrganizationTables godoc
// @Summary      Получить таблицы организации
// @Description  Возвращает список таблиц, привязанных к организации
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {array} services.OrganizationTableResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /organizations/{id}/tables [get]
func (h *OrganizationHandler) GetOrganizationTables(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	tables, err := h.service.GetOrganizationTables(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, tables)
}

// UpdateOrganizationTables godoc
// @Summary      Обновить таблицы организации
// @Description  Заменяет привязку таблиц к организации. Требует права buropropuskov
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Param        request body services.UpdateOrganizationTablesRequest true "Список ID таблиц"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/{id}/tables [put]
func (h *OrganizationHandler) UpdateOrganizationTables(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	var req services.UpdateOrganizationTablesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.UpdateOrganizationTables(c.Request().Context(), userID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Organization tables updated successfully")
}

// GetOrganizationUnloadPlaces godoc
// @Summary      Получить места разгрузки организации
// @Description  Возвращает список мест разгрузки, привязанных к организации
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Success      200 {array} services.OrganizationUnloadPlaceResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /organizations/{id}/unload-places [get]
func (h *OrganizationHandler) GetOrganizationUnloadPlaces(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	places, err := h.service.GetOrganizationUnloadPlaces(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, places)
}

// UpdateOrganizationUnloadPlaces godoc
// @Summary      Обновить места разгрузки организации
// @Description  Заменяет привязку мест разгрузки к организации. Требует права buropropuskov
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID организации"
// @Param        request body services.UpdateOrganizationUnloadPlacesRequest true "Список ID мест разгрузки"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/{id}/unload-places [put]
func (h *OrganizationHandler) UpdateOrganizationUnloadPlaces(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid organization ID")
	}

	var req services.UpdateOrganizationUnloadPlacesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.UpdateOrganizationUnloadPlaces(c.Request().Context(), userID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Unload places updated successfully")
}

// respondBulk отдаёт результат групповой операции: 200 при полном успехе, 207
// при частичном (envelope success=true, data=BulkOpResult - как batch машин).
func respondBulk(c echo.Context, res *services.BulkOpResult) error {
	return c.JSON(res.HTTPStatus(), Response{Success: true, Data: res})
}

// BulkUpdateType godoc
// @Summary      Групповая смена типа организаций
// @Description  Меняет тип у набора организаций. Требует права admin. type=null снимает тип.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkTypeRequest true "ID организаций и новый тип"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/bulk/type [post]
func (h *OrganizationHandler) BulkUpdateType(c echo.Context) error {
	var req services.BulkTypeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны организации")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkUpdateType(c.Request().Context(), userID, req.IDs, req.Type)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkAssignUnloadPlaces godoc
// @Summary      Групповое назначение мест разгрузки организациям
// @Description  Назначает места разгрузки набору организаций. mode=replace|add. Требует права admin.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUnloadPlacesRequest true "ID организаций, мест и режим"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/bulk/unload-places [post]
func (h *OrganizationHandler) BulkAssignUnloadPlaces(c echo.Context) error {
	var req services.BulkUnloadPlacesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны организации")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkAssignUnloadPlaces(c.Request().Context(), userID, req.IDs, req.UnloadPlaceIDs, req.Mode)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkAssignTables godoc
// @Summary      Групповое назначение таблиц организациям
// @Description  Назначает целевые таблицы набору организаций. mode=replace|add. Требует права admin.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkTablesRequest true "ID организаций, таблиц и режим"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/bulk/tables [post]
func (h *OrganizationHandler) BulkAssignTables(c echo.Context) error {
	var req services.BulkTablesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны организации")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkAssignTables(c.Request().Context(), userID, req.IDs, req.TableIDs, req.Mode)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkAssignUsers godoc
// @Summary      Групповое назначение ответственных организациям
// @Description  Назначает ответственных набору организаций. mode=replace|add. primary не назначается. Требует права admin.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUsersRequest true "ID организаций, логины, режим"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/bulk/users [post]
func (h *OrganizationHandler) BulkAssignUsers(c echo.Context) error {
	var req services.BulkUsersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны организации")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkAssignUsers(c.Request().Context(), userID, req.IDs, req.Users, req.Mode)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkArchive godoc
// @Summary      Групповое архивирование организаций
// @Description  Архивирует набор организаций. Активные с пользователями попадают в errors. Требует права admin.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "ID организаций"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/bulk/archive [post]
func (h *OrganizationHandler) BulkArchive(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны организации")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkArchive(c.Request().Context(), userID, req.IDs)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkRestore godoc
// @Summary      Групповое восстановление организаций
// @Description  Восстанавливает набор организаций из архива. Требует права admin.
// @Tags         organizations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "ID организаций"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /organizations/bulk/restore [post]
func (h *OrganizationHandler) BulkRestore(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны организации")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkRestore(c.Request().Context(), userID, req.IDs)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}
