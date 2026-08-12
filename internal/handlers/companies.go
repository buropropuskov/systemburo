package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// CompanyHandler HTTP-обработчики для работы с компаниями.
type CompanyHandler struct {
	service  services.CompanyService
	db       *gorm.DB
	resolver *services.PermissionResolver
}

// NewCompanyHandler создаёт экземпляр обработчика компаний.
func NewCompanyHandler(service services.CompanyService, db *gorm.DB, resolver *services.PermissionResolver) *CompanyHandler {
	return &CompanyHandler{service: service, db: db, resolver: resolver}
}

// GetAll godoc
// @Summary      Получить все компании
// @Description  Возвращает список всех компаний, отсортированных по имени
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.Company
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /companies [get]
func (h *CompanyHandler) GetAll(c echo.Context) error {
	companies, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, companies)
}

// Suggest godoc
// @Summary      Подсказки компаний по наименованию
// @Description  Близкие к запросу проверенные компании (максимум 5) для ручного ввода наименования в заявке. Требует права application.organization.override. Запрос короче трёх символов даёт пустой список.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        q query string false "Наименование или его часть; короче трёх символов - пустой список"
// @Success      200 {object} services.DirectorySuggestAnswer
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /companies/suggest [get]
func (h *CompanyHandler) Suggest(c echo.Context) error {
	suggestions, err := h.service.Suggest(c.Request().Context(), c.QueryParam("q"))
	if err != nil {
		return err
	}
	return RespondSuccess(c, suggestions)
}

// ApproveModeration godoc
// @Summary      Подтвердить компанию «на проверке»
// @Description  Разбор записи, заведённой из заявки (#1437). Требует права application.organization.moderate. При совпадении наименования с проверенной записью ответ приходит со status=conflict и самой записью; столкновение с другим черновиком даёт 400, его разбирают первым.
// @Tags         companies
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {object} services.DirectoryModerationResult
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /companies/{id}/moderation/approve [post]
func (h *CompanyHandler) ApproveModeration(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	userID, _ := c.Get("user_id").(int)
	result, err := h.service.ApproveModeration(c.Request().Context(), userID, id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}

// RenameModeration godoc
// @Summary      Исправить наименование компании «на проверке»
// @Description  Правит наименование черновика и считает запись разобранной. Требует права application.organization.moderate. При совпадении с проверенной записью возвращает status=conflict, при совпадении с другим черновиком - 400.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Param        request body services.DirectoryRenameRequest true "Новое наименование"
// @Success      200 {object} services.DirectoryModerationResult
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /companies/{id}/moderation/rename [patch]
func (h *CompanyHandler) RenameModeration(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
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
// @Summary      Привязать компанию «на проверке» к существующей
// @Description  Переносит заявки, вложения, машины, сотрудников и привязки черновика на выбранную проверенную компанию и удаляет черновик. Требует права application.organization.moderate.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании «на проверке»"
// @Param        request body services.DirectoryMergeRequest true "ID целевой компании"
// @Success      200 {object} services.DirectoryMergeResult
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /companies/{id}/moderation/merge [post]
func (h *CompanyHandler) MergeModeration(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
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

// GetWithUsers godoc
// @Summary      Получить компании с количеством пользователей
// @Description  Возвращает список компаний с количеством привязанных пользователей
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} services.CompanyWithUsersResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /companies/with-users [get]
func (h *CompanyHandler) GetWithUsers(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	companies, err := h.service.GetWithUsers(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, companies)
}

// GetWithUsersExtended godoc
// @Summary      Получить компании с расширенной информацией
// @Description  Возвращает компании с количеством пользователей и местами разгрузки
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} services.CompanyWithUsersExtendedResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /companies/with-users-extended [get]
func (h *CompanyHandler) GetWithUsersExtended(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	companies, err := h.service.GetWithUsersExtended(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, companies)
}

// Create godoc
// @Summary      Создать компанию
// @Description  Создаёт новую компанию. Требуются права buropropuskov
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.CreateCompanyRequest true "Данные новой компании"
// @Success      200 {object} models.Company
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies [post]
func (h *CompanyHandler) Create(c echo.Context) error {
	var req services.CreateCompanyRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	company, err := h.service.Create(c.Request().Context(), userID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, company)
}

// Update godoc
// @Summary      Обновить компанию
// @Description  Обновляет название компании по ID. Требуются права buropropuskov
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Param        request body services.CreateCompanyRequest true "Обновлённые данные компании"
// @Success      200 {object} models.Company
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/{id} [put]
func (h *CompanyHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.CreateCompanyRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	company, err := h.service.Update(c.Request().Context(), userID, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, company)
}

// Delete godoc
// @Summary      Удалить компанию
// @Description  Удаляет компанию по ID. Нельзя удалить если есть привязанные пользователи. Требуются права buropropuskov
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Router       /companies/{id} [delete]
func (h *CompanyHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.Delete(c.Request().Context(), userID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Company archived")
}

// Restore godoc
// @Summary      Восстановить компанию из архива
// @Tags         companies
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {string} string "Company restored"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/{id}/restore [post]
func (h *CompanyHandler) Restore(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Restore(c.Request().Context(), userID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Company restored")
}

// GetHistory godoc
// @Summary      История изменений компании
// @Tags         companies
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {array} models.CompanyHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/{id}/history [get]
func (h *CompanyHandler) GetHistory(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	items, err := h.service.GetHistory(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// GetUsers godoc
// @Summary      Получить пользователей компании
// @Description  Возвращает список ответственных пользователей компании. Маршрут открыт любому вошедшему (его же читает форма подачи заявки у своей компании), поэтому required_approval виден только тем, у кого есть право на раздел справочников, и заявителю - для его СОБСТВЕННОЙ компании (#2013). Остальным поле приходит null.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {array} services.CompanyUserResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /companies/{id}/users [get]
func (h *CompanyHandler) GetUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	users, err := h.service.GetUsers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	if !canSeeRequiredApproval(c, h.db, h.resolver, id, func(o callerOwnDirectoryIDs) *int { return o.CompanyID }) {
		for i := range users {
			users[i].RequiredApproval = nil
		}
	}
	return RespondSuccess(c, users)
}

// GetMembers godoc
// @Summary      Получить участников компании
// @Description  Возвращает пользователей, привязанных к компании через company_id (не ответственных)
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {array} services.MemberResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /companies/{id}/members [get]
func (h *CompanyHandler) GetMembers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	members, err := h.service.GetMembers(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, members)
}

// ReassignUsers godoc
// @Summary      Перенести всех блокирующих пользователей в другую компанию
// @Description  Переносит активных участников компании в целевую (target_id),
// @Description  освобождая исходную для архивации. Требует права admin.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID исходной компании"
// @Param        request body services.ReassignUsersRequest true "ID целевой компании"
// @Success      200 {object} map[string]int "reassigned"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /companies/{id}/reassign-users [post]
func (h *CompanyHandler) ReassignUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}
	var req services.ReassignUsersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if req.TargetID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не указана целевая компания")
	}
	userID, _ := c.Get("user_id").(int)
	count, err := h.service.ReassignMembers(c.Request().Context(), userID, id, req.TargetID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]int{"reassigned": count})
}

// UpdateUsers godoc
// @Summary      Обновить пользователей компании
// @Description  Заменяет список ответственных пользователей компании с поддержкой обязательного согласования
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Param        request body services.UpdateCompanyUsersRequest true "Список ответственных пользователей"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /companies/{id}/users [put]
func (h *CompanyHandler) UpdateUsers(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.UpdateCompanyUsersRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.UpdateUsers(c.Request().Context(), userID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Company users updated successfully")
}

// GetUnloadPlaces godoc
// @Summary      Получить места разгрузки компании
// @Description  Возвращает список активных мест разгрузки, привязанных к компании
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {array} services.CompanyUnloadPlaceResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /companies/{id}/unload-places [get]
func (h *CompanyHandler) GetUnloadPlaces(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	places, err := h.service.GetUnloadPlaces(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, places)
}

// UpdateUnloadPlaces godoc
// @Summary      Обновить места разгрузки компании
// @Description  Обновляет привязку мест разгрузки к компании. Требуются права buropropuskov
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Param        request body services.UpdateCompanyUnloadPlacesRequest true "Список ID мест разгрузки"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/{id}/unload-places [put]
func (h *CompanyHandler) UpdateUnloadPlaces(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.UpdateCompanyUnloadPlacesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.UpdateUnloadPlaces(c.Request().Context(), userID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Unload places updated successfully")
}

// GetTables godoc
// @Summary      Получить таблицы компании
// @Description  Возвращает список активных таблиц, привязанных к компании
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Success      200 {array} services.CompanyTableResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /companies/{id}/tables [get]
func (h *CompanyHandler) GetTables(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	tables, err := h.service.GetTables(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, tables)
}

// UpdateTables godoc
// @Summary      Обновить таблицы компании
// @Description  Обновляет привязку таблиц к компании. Требуются права buropropuskov
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID компании"
// @Param        request body services.UpdateCompanyTablesRequest true "Список ID таблиц"
// @Success      200 {object} map[string]string "message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/{id}/tables [put]
func (h *CompanyHandler) UpdateTables(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid company ID")
	}

	var req services.UpdateCompanyTablesRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.UpdateTables(c.Request().Context(), userID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Company tables updated successfully")
}

// BulkUpdateType godoc
// @Summary      Групповая смена типа компаний
// @Description  Меняет тип у набора компаний. Требует права admin. type=null снимает тип.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkTypeRequest true "ID компаний и новый тип"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/bulk/type [post]
func (h *CompanyHandler) BulkUpdateType(c echo.Context) error {
	var req services.BulkTypeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны компании")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkUpdateType(c.Request().Context(), userID, req.IDs, req.Type)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkAssignUnloadPlaces godoc
// @Summary      Групповое назначение мест разгрузки компаниям
// @Description  Назначает места разгрузки набору компаний. mode=replace|add. Требует права admin.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUnloadPlacesRequest true "ID компаний, мест и режим"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/bulk/unload-places [post]
func (h *CompanyHandler) BulkAssignUnloadPlaces(c echo.Context) error {
	var req services.BulkUnloadPlacesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны компании")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkAssignUnloadPlaces(c.Request().Context(), userID, req.IDs, req.UnloadPlaceIDs, req.Mode)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkAssignTables godoc
// @Summary      Групповое назначение таблиц компаниям
// @Description  Назначает целевые таблицы набору компаний. mode=replace|add. Требует права admin.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkTablesRequest true "ID компаний, таблиц и режим"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/bulk/tables [post]
func (h *CompanyHandler) BulkAssignTables(c echo.Context) error {
	var req services.BulkTablesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны компании")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkAssignTables(c.Request().Context(), userID, req.IDs, req.TableIDs, req.Mode)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkAssignUsers godoc
// @Summary      Групповое назначение ответственных компаниям
// @Description  Назначает ответственных набору компаний. mode=replace|add. primary не назначается. Требует права admin.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkUsersRequest true "ID компаний, логины, режим"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/bulk/users [post]
func (h *CompanyHandler) BulkAssignUsers(c echo.Context) error {
	var req services.BulkUsersRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны компании")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkAssignUsers(c.Request().Context(), userID, req.IDs, req.Users, req.Mode)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkArchive godoc
// @Summary      Групповое архивирование компаний
// @Description  Архивирует набор компаний. Активные с пользователями попадают в errors. Требует права admin.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "ID компаний"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/bulk/archive [post]
func (h *CompanyHandler) BulkArchive(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны компании")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkArchive(c.Request().Context(), userID, req.IDs)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkRestore godoc
// @Summary      Групповое восстановление компаний
// @Description  Восстанавливает набор компаний из архива. Требует права admin.
// @Tags         companies
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "ID компаний"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /companies/bulk/restore [post]
func (h *CompanyHandler) BulkRestore(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны компании")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkRestore(c.Request().Context(), userID, req.IDs)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}
