package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// ApplicationHandler HTTP-обработчики для работы с заявками.
type ApplicationHandler struct {
	service  services.ApplicationService
	resolver *services.PermissionResolver
}

// NewApplicationHandler создаёт экземпляр обработчика заявок.
func NewApplicationHandler(service services.ApplicationService, resolver *services.PermissionResolver) *ApplicationHandler {
	return &ApplicationHandler{service: service, resolver: resolver}
}

// canOverrideOrganization сообщает, вправе ли подающий указать организацию или компанию,
// отличную от своей (#1437). Резолвер истинен для супер-админа (allowAll), администратора
// (adminAll, включая руководителей: миграция перенесла тип manager на is_admin) и для
// явного гранта роли, группы или личного override; бан и личные deny он учитывает.
func (h *ApplicationHandler) canOverrideOrganization(c echo.Context) (bool, error) {
	set, err := h.resolver.Resolve(c.Request().Context(), GetUserID(c))
	if err != nil {
		return false, err
	}
	return set.Has(services.KeyApplicationOrganizationOverride), nil
}

// bindApplicationListFilter собирает фильтр списка заявок из query-параметров.
// Одна функция на список и на выгрузку реестра (#1832): набор фильтров у них
// обязан совпадать, а скопированный парсинг разъезжается с первым же новым
// фильтром - в файл уедет не то, что человек видит на экране.
//
// archive и active_today разбираются руками: Bind не кладёт "true"/"false" в
// *bool, а отсутствие параметра должно означать «фильтр не задан», а не false.
func bindApplicationListFilter(c echo.Context) (services.ApplicationFilter, error) {
	var filter services.ApplicationFilter
	if err := c.Bind(&filter); err != nil {
		return filter, echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if archiveStr := c.QueryParam("archive"); archiveStr != "" {
		archive := archiveStr == "true"
		filter.Archive = &archive
	}
	if activeTodayStr := c.QueryParam("active_today"); activeTodayStr != "" {
		activeToday := activeTodayStr == "true"
		filter.ActiveToday = &activeToday
	}
	return filter, nil
}

// GetApplications godoc
// @Summary      Список заявок для Центра заявок
// @Description  Возвращает заявки с фильтрацией. Принимающие видят все, обычные пользователи -- только свои.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        search_query      query string false "Поисковый запрос"
// @Param        organization_id   query int    false "ID организации"
// @Param        company_id        query int    false "ID компании"
// @Param        organization_ids  query string false "ID организаций через запятую (мультивыбор)"
// @Param        company_ids       query string false "ID компаний через запятую (мультивыбор)"
// @Param        unload_place_ids  query string false "ID мест разгрузки через запятую (мультивыбор)"
// @Param        passage_table_ids query string false "ID таблиц проходной через запятую (мультивыбор)"
// @Param        confirmation      query string false "Статус согласования"
// @Param        status            query string false "Статус заявки"
// @Param        date_from         query string false "Дата от (YYYY-MM-DD)"
// @Param        date_to           query string false "Дата до (YYYY-MM-DD)"
// @Success      200 {array}  services.ApplicationWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications [get]
func (h *ApplicationHandler) GetApplications(c echo.Context) error {
	username := c.Get("username").(string)

	filter, err := bindApplicationListFilter(c)
	if err != nil {
		return err
	}

	// Legacy mode: if per_page not specified, return all (backward compat)
	if c.QueryParam("per_page") == "" {
		apps, err := h.service.GetApplications(c.Request().Context(), username, filter)
		if err != nil {
			return err
		}
		return RespondSuccess(c, apps)
	}

	var params models.PaginationParams
	if err := c.Bind(&params); err != nil {
		params = models.PaginationParams{}
	}
	params.Normalize()

	data, total, err := h.service.GetApplicationsPaginated(
		c.Request().Context(), username, filter, params.Page, params.PerPage,
	)
	if err != nil {
		return err
	}
	return RespondPaginated(c, data, models.PaginationMeta{
		Total: total, Page: params.Page, PerPage: params.PerPage,
	})
}

// GetAttachableApplications godoc
// @Summary      Заявки, доступные для привязки ручного вложения
// @Description  Активные согласованные заявки для привязки (#1049 режим-2). Только super/admin
// @Description  (гейт page.admin). В отличие от GET /applications НЕ скоупит по автор/ответственный
// @Description  - админ видит все заявки для привязки.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        search_query query string false "Поисковый запрос"
// @Success      200 {array} services.ApplicationWithDetails
// @Failure      403 {object} models.HTTPError
// @Router       /applications/attachable [get]
func (h *ApplicationHandler) GetAttachableApplications(c echo.Context) error {
	username := c.Get("username").(string)

	var filter services.ApplicationFilter
	if err := c.Bind(&filter); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	apps, err := h.service.GetAttachableApplications(c.Request().Context(), username, filter)
	if err != nil {
		return err
	}
	return RespondSuccess(c, apps)
}

// GetUserApplications godoc
// @Summary      Заявки текущего пользователя
// @Description  Возвращает заявки текущего пользователя (отправленные им или его организацией)
// @Description  с фильтрацией. Без per_page - полный список (legacy). С per_page - страница
// @Description  через GetUserApplicationsPaginated, meta.total в envelope (#1158).
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        search_query query string false "Поисковый запрос"
// @Param        confirmation query string false "Статус согласования"
// @Param        status       query string false "Статус заявки"
// @Param        date_from    query string false "Дата от (YYYY-MM-DD)"
// @Param        date_to      query string false "Дата до (YYYY-MM-DD)"
// @Param        page         query int    false "Номер страницы"
// @Param        per_page     query int    false "Размер страницы (включает пагинацию)"
// @Success      200 {array}  services.ApplicationWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/user [get]
func (h *ApplicationHandler) GetUserApplications(c echo.Context) error {
	username := c.Get("username").(string)

	var filter services.ApplicationFilter
	if err := c.Bind(&filter); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Legacy mode: per_page не задан - полный список (обратная совместимость).
	if c.QueryParam("per_page") == "" {
		apps, err := h.service.GetUserApplications(c.Request().Context(), username, filter)
		if err != nil {
			return err
		}
		return RespondSuccess(c, apps)
	}

	var params models.PaginationParams
	if err := c.Bind(&params); err != nil {
		params = models.PaginationParams{}
	}
	params.Normalize()

	data, total, err := h.service.GetUserApplicationsPaginated(
		c.Request().Context(), username, filter, params.Page, params.PerPage,
	)
	if err != nil {
		return err
	}
	return RespondPaginated(c, data, models.PaginationMeta{
		Total: total, Page: params.Page, PerPage: params.PerPage,
	})
}

// GetApplicationByID godoc
// @Summary      Получение заявки по ID
// @Description  Возвращает заявку с обновлением статуса при первом прочтении не отправителем.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id} [get]
func (h *ApplicationHandler) GetApplicationByID(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	app, err := h.service.GetApplicationByID(c.Request().Context(), username, id)
	if err != nil {
		return err
	}

	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	return RespondSuccess(c, app)
}

// GetApplicationDetails godoc
// @Summary      Расширенная информация о заявке
// @Description  Возвращает детальную информацию о заявке с ответственными пользователями.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/details [get]
func (h *ApplicationHandler) GetApplicationDetails(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	username := c.Get("username").(string)
	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	// Открытие детали = пользователь увидел текущий статус: гасим его флаг "статус
	// обновился" (#1349). Best-effort - сбой отметки не должен ломать выдачу деталей.
	if err := h.service.MarkStatusSeen(c.Request().Context(), username, id); err != nil {
		slog.Warn("Не удалось отметить просмотр статуса заявки", "application_id", id, "error", err)
	}

	details, err := h.service.GetApplicationDetails(c.Request().Context(), username, id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, details)
}

// SetBureauNote godoc
// @Summary      Заметка бюро по заявке
// @Description  Сохраняет рабочую заметку принимающих по заявке; пустой текст снимает её.
// @Description  Доступно только принимающим, остальным 403. В ответ детали заявки заметка
// @Description  попадает тоже только принимающему.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                          true "ID заявки"
// @Param        request body services.SetBureauNoteRequest true "Текст заметки"
// @Success      200 {object} services.BureauNoteView
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id}/bureau-note [put]
func (h *ApplicationHandler) SetBureauNote(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	var req services.SetBureauNoteRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	// Доступ к заявке отдельно не проверяем: гейт сервиса (роль принимающего) строго
	// уже - принимающий видит все заявки (CanAccessApplication пускает его без условий).
	note, err := h.service.SetBureauNote(c.Request().Context(), c.Get("username").(string), id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, note)
}

// CreateApplication godoc
// @Summary      Создание простой заявки
// @Description  Создаёт заявку с автоматическим назначением ответственных из организации/компании.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.ApplicationCreateRequest true "Данные заявки"
// @Success      200 {object} services.ApplicationCreateResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications [post]
func (h *ApplicationHandler) CreateApplication(c echo.Context) error {
	username := c.Get("username").(string)

	var req services.ApplicationCreateRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	canOverride, err := h.canOverrideOrganization(c)
	if err != nil {
		return err
	}

	resp, err := h.service.CreateApplication(c.Request().Context(), username, req, canOverride)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// SubmitCompleteApplication godoc
// @Summary      Создание полной заявки с вложениями
// @Description  Создаёт заявку вместе с вложениями (машины, сотрудники, ТМЦ) в одной транзакции.
// @Description  Организация и компания, отличные от указанных в профиле, требуют права application.organization.override.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.CompleteApplicationRequest true "Полные данные заявки"
// @Success      200 {object} services.CompleteApplicationResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/submit-complete-application [post]
func (h *ApplicationHandler) SubmitCompleteApplication(c echo.Context) error {
	username := c.Get("username").(string)

	var req services.CompleteApplicationRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	// BindAndValidate срезы не валидирует by design (см. её докблок) - без явного потолка
	// data.employees/data.vehicles/data.items можно раздуть произвольным числом строк
	// вплоть до BodyLimit группы (blank-import, срез A2A3).
	// Считаем суммарно по всему запросу, а не по каждому вложению отдельно: число
	// вложений не ограничено, и попарно-проходящие списки складывались бы в одну
	// транзакцию кратно выше потолка.
	var totalEmployees, totalVehicles, totalItems int
	for _, att := range req.Attachments {
		if att.Data.Employees != nil {
			totalEmployees += len(*att.Data.Employees)
		}
		if att.Data.Vehicles != nil {
			totalVehicles += len(*att.Data.Vehicles)
		}
		if att.Data.Items != nil {
			totalItems += len(*att.Data.Items)
		}
	}
	if err := ValidateSliceCap(totalEmployees, MaxSubmitRowsPerList, "сотрудников"); err != nil {
		return err
	}
	if err := ValidateSliceCap(totalVehicles, MaxSubmitRowsPerList, "машин"); err != nil {
		return err
	}
	if err := ValidateSliceCap(totalItems, MaxSubmitRowsPerList, "ТМЦ"); err != nil {
		return err
	}

	canOverride, err := h.canOverrideOrganization(c)
	if err != nil {
		return err
	}

	resp, err := h.service.SubmitCompleteApplication(c.Request().Context(), username, req, canOverride)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// UpdateApplication godoc
// @Summary      Обновление заявки
// @Description  Обновляет confirmation, status и/или responsible_comment заявки.
// @Tags         applications
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int                              true  "ID заявки"
// @Param        request body services.ApplicationUpdateRequest true  "Обновляемые поля"
// @Success      200 {object} services.ApplicationUpdateResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/{id} [put]
func (h *ApplicationHandler) UpdateApplication(c echo.Context) error {
	username := c.Get("username").(string)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}

	if !h.service.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	var req services.ApplicationUpdateRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	resp, err := h.service.UpdateApplication(c.Request().Context(), username, id, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// GetUnreadCount godoc
// @Summary      Количество непрочитанных заявок
// @Description  Возвращает количество непрочитанных активных заявок для текущего пользователя.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.UnreadCountResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/unread-count [get]
func (h *ApplicationHandler) GetUnreadCount(c echo.Context) error {
	username := c.Get("username").(string)

	resp, err := h.service.GetUnreadCount(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// GetUserStatusUpdatesCount godoc
// @Summary      Число заявок ЛК с обновлённым статусом
// @Description  Счётчик для чипа "Обновления" в ЛК (#1349): заявки пользователя (его или
// @Description  организации), чей статус/подтверждение менялись после последнего просмотра.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.StatusUpdatesCountResponse
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /applications/user/status-updates-count [get]
func (h *ApplicationHandler) GetUserStatusUpdatesCount(c echo.Context) error {
	username := c.Get("username").(string)

	resp, err := h.service.GetUserStatusUpdatesCount(c.Request().Context(), username)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}
