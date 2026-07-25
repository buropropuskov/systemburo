package handlers

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/upload"

	"github.com/labstack/echo/v4"
)

// SystemTableHandler -- HTTP-обработчики системных таблиц.
type SystemTableHandler struct {
	service     services.SystemTableService
	recorder    services.AuditRecorder
	maxFileSize int64
	uploadDir   string
}

// NewSystemTableHandler создаёт новый экземпляр обработчика системных таблиц.
// recorder может быть nil - тогда логирование действий отключено.
func NewSystemTableHandler(service services.SystemTableService, recorder services.AuditRecorder, maxFileSize int64, uploadDir string) *SystemTableHandler {
	return &SystemTableHandler{
		service:     service,
		recorder:    recorder,
		maxFileSize: maxFileSize,
		uploadDir:   filepath.Join(uploadDir, "system_tables"),
	}
}

// logAction пишет запись аудита. Безопасно вызывать с nil-recorder.
func (h *SystemTableHandler) logAction(ctx context.Context, c echo.Context, tableID int, actionType string, details interface{}) {
	if h.recorder == nil {
		return
	}
	var userID *int
	if v := c.Get("user_id"); v != nil {
		if id, ok := v.(int); ok && id > 0 {
			userID = &id
		}
	}
	h.recorder.Log(ctx, nil, models.AuditEntitySystemTable, &tableID, actionType, userID, details)
}

// GetAll godoc
// @Summary      Получение системных таблиц
// @Description  По умолчанию возвращает только активные. include_archived=true возвращает только архивные (мягко удалённые).
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        include_archived query bool false "Вернуть архивные таблицы вместо активных"
// @Success      200 {array} models.SystemTableWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /system-tables [get]
func (h *SystemTableHandler) GetAll(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	tables, err := h.service.GetAll(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, tables)
}

// GetByID godoc
// @Summary      Получение системной таблицы по ID
// @Description  Возвращает системную таблицу с полями, слотами, фото и текущим статусом
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Success      200 {object} models.SystemTableWithDetails
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id} [get]
func (h *SystemTableHandler) GetByID(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	table, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, table)
}

// GetByName godoc
// @Summary      Получение системной таблицы по имени
// @Description  Возвращает системную таблицу с полями, слотами, фото и текущим статусом
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        name path string true "Имя таблицы"
// @Param        allow_archived query bool false "Искать и среди архивных тоже (для страницы версий)"
// @Success      200 {object} models.SystemTableWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/name/{name} [get]
func (h *SystemTableHandler) GetByName(c echo.Context) error {
	name := c.Param("name")
	allowArchived := c.QueryParam("allow_archived") == "1" || c.QueryParam("allow_archived") == "true"
	table, err := h.service.GetByName(c.Request().Context(), name, allowArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, table)
}

// Create godoc
// @Summary      Создание системной таблицы
// @Description  Создаёт новую системную таблицу с полями по умолчанию на основе table_type
// @Tags         system-tables
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateSystemTableRequest true "Данные таблицы"
// @Success      200 {object} map[string]interface{} "id, message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /system-tables [post]
func (h *SystemTableHandler) Create(c echo.Context) error {
	var req models.CreateSystemTableRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return err
	}
	h.logAction(c.Request().Context(), c, id, models.SystemTableActionCreated, map[string]interface{}{
		"name":         req.Name,
		"display_name": req.DisplayName,
		"table_type":   req.TableType,
	})
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Системная таблица успешно создана",
	})
}

// Update godoc
// @Summary      Обновление системной таблицы
// @Description  Обновляет поля системной таблицы (частичное обновление)
// @Tags         system-tables
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        request body models.UpdateSystemTableRequest true "Данные для обновления"
// @Success      200 {string} string "Системная таблица успешно обновлена"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id} [put]
func (h *SystemTableHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateSystemTableRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Update(c.Request().Context(), id, req); err != nil {
		return err
	}
	// В history-details пишем только реально заданные поля - чтобы UI не показывал
	// "статус: -" для полей, которые юзер не трогал.
	details := buildUpdateDetails(req)
	if len(details) > 0 {
		h.logAction(c.Request().Context(), c, id, models.SystemTableActionUpdated, details)
	}
	return RespondMessage(c, "Системная таблица успешно обновлена")
}

// buildUpdateDetails преобразует UpdateSystemTableRequest в map с только не-nil
// полями (через json-tag-имена), чтобы запись в history содержала только реально
// изменённые значения.
func buildUpdateDetails(req models.UpdateSystemTableRequest) map[string]interface{} {
	out := map[string]interface{}{}
	if req.DisplayName != nil {
		out["display_name"] = *req.DisplayName
	}
	if req.TableType != nil {
		out["table_type"] = *req.TableType
	}
	if req.ShowFactTable != nil {
		out["show_fact_table"] = *req.ShowFactTable
	}
	if req.FactTableHint != nil {
		out["fact_table_hint"] = *req.FactTableHint
	}
	if req.Instruction != nil {
		out["instruction"] = *req.Instruction
	}
	if req.MapLink != nil {
		out["map_link"] = *req.MapLink
	}
	if req.Status != nil {
		out["status"] = *req.Status
	}
	if req.StatusComment != nil {
		out["status_comment"] = *req.StatusComment
	}
	if req.LocationDescription != nil {
		out["location_description"] = *req.LocationDescription
	}
	if req.Warning != nil {
		out["warning"] = *req.Warning
	}
	if req.FontSize != nil {
		out["font_size"] = *req.FontSize
	}
	if req.RowDensity != nil {
		out["row_density"] = *req.RowDensity
	}
	if req.FontSizeFact != nil {
		out["font_size_fact"] = *req.FontSizeFact
	}
	if req.RowDensityFact != nil {
		out["row_density_fact"] = *req.RowDensityFact
	}
	return out
}

// Delete godoc
// @Summary      Удаление системной таблицы
// @Description  Мягкое удаление (is_active=false) - таблица уходит в архив. Проверяет привязки к организациям и компаниям
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Success      200 {string} string "Системная таблица успешно удалена"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id} [delete]
func (h *SystemTableHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	h.logAction(c.Request().Context(), c, id, models.SystemTableActionArchived, nil)
	return RespondMessage(c, "Системная таблица успешно удалена")
}

// Restore godoc
// @Summary      Восстановление системной таблицы из архива
// @Description  Возвращает таблицу из архива (is_active=false -> true)
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Success      200 {string} string "Системная таблица восстановлена"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id}/restore [post]
func (h *SystemTableHandler) Restore(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.Restore(c.Request().Context(), id); err != nil {
		return err
	}
	h.logAction(c.Request().Context(), c, id, models.SystemTableActionRestored, nil)
	return RespondMessage(c, "Системная таблица восстановлена")
}

// GetUsage возвращает организации и компании, привязанные к таблице.
// @Summary      Привязки системной таблицы
// @Description  Организации и компании, к которым привязана таблица (те же, что блокируют удаление)
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Success      200 {object} services.SystemTableUsage
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /system-tables/{id}/usage [get]
func (h *SystemTableHandler) GetUsage(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	usage, err := h.service.GetUsage(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, usage)
}

// DetachAll снимает привязки таблицы ко всем организациям и компаниям.
// @Summary      Отвязать таблицу от всех организаций и компаний
// @Description  Разом снимает все привязки таблицы к организациям/компаниям (с записью в историю каждой). После этого таблицу можно архивировать
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Success      200 {object} services.SystemTableDetachResult
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /system-tables/{id}/detach-all [post]
func (h *SystemTableHandler) DetachAll(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.DetachAll(c.Request().Context(), userID, id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, res)
}

// DetachOrganization снимает привязку таблицы к одной организации.
// @Summary      Отвязать таблицу от организации
// @Description  Снимает привязку таблицы к конкретной организации (с записью в её историю). Идемпотентно
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        org_id path int true "ID организации"
// @Success      200 {object} map[string]bool
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /system-tables/{id}/organizations/{org_id} [delete]
func (h *SystemTableHandler) DetachOrganization(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	orgID, err := strconv.Atoi(c.Param("org_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid organization id")
	}
	userID, _ := c.Get("user_id").(int)
	detached, err := h.service.DetachOrganization(c.Request().Context(), userID, id, orgID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, echo.Map{"detached": detached})
}

// DetachCompany снимает привязку таблицы к одной компании.
// @Summary      Отвязать таблицу от компании
// @Description  Снимает привязку таблицы к конкретной компании (с записью в её историю). Идемпотентно
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        company_id path int true "ID компании"
// @Success      200 {object} map[string]bool
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /system-tables/{id}/companies/{company_id} [delete]
func (h *SystemTableHandler) DetachCompany(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	companyID, err := strconv.Atoi(c.Param("company_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid company id")
	}
	userID, _ := c.Get("user_id").(int)
	detached, err := h.service.DetachCompany(c.Request().Context(), userID, id, companyID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, echo.Map{"detached": detached})
}

// logBulkResult пишет запись аудита для каждого id из запроса, для которого
// групповая операция реально прошла успешно (нет в res.Errors). Аудит для
// системных таблиц живёт в handler-слое (см. logAction), а не в сервисе (как
// у марок) - BulkOpResult сервиса не возвращает список успешных id, поэтому
// дедуп запроса и вычитание провалившихся делаем здесь же.
func (h *SystemTableHandler) logBulkResult(c echo.Context, ids []int, res *services.BulkOpResult, actionType string) {
	if h.recorder == nil {
		return
	}
	failed := make(map[int]struct{}, len(res.Errors))
	for _, e := range res.Errors {
		failed[e.ID] = struct{}{}
	}
	logged := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := logged[id]; ok {
			continue
		}
		logged[id] = struct{}{}
		if _, bad := failed[id]; bad {
			continue
		}
		h.logAction(c.Request().Context(), c, id, actionType, nil)
	}
}

// BulkArchive godoc
// @Summary      Групповая архивация системных таблиц
// @Tags         system-tables
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID таблиц"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /system-tables/bulk/archive [post]
func (h *SystemTableHandler) BulkArchive(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны таблицы")
	}
	res, err := h.service.BulkArchive(c.Request().Context(), req.IDs)
	if err != nil {
		return err
	}
	h.logBulkResult(c, req.IDs, res, models.SystemTableActionArchived)
	return respondBulk(c, res)
}

// BulkRestore godoc
// @Summary      Групповое восстановление системных таблиц
// @Tags         system-tables
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID таблиц"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /system-tables/bulk/restore [post]
func (h *SystemTableHandler) BulkRestore(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны таблицы")
	}
	res, err := h.service.BulkRestore(c.Request().Context(), req.IDs)
	if err != nil {
		return err
	}
	h.logBulkResult(c, req.IDs, res, models.SystemTableActionRestored)
	return respondBulk(c, res)
}

// GetHistory godoc
// @Summary      История изменений системной таблицы
// @Description  Возвращает все CRUD-действия над таблицей (created/updated/archived/restored/columns/appearance)
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Success      200 {array} models.SystemTableHistoryItem
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /system-tables/{id}/history [get]
func (h *SystemTableHandler) GetHistory(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	items, err := h.service.GetHistory(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// UpdateFields godoc
// @Summary      Bulk-обновление видимости столбцов таблицы (#345)
// @Description  Обновляет is_visible для перечисленных field_name. Поля не из БД игнорируются.
// @Tags         system-tables
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        request body models.UpdateFieldsRequest true "Список столбцов с новой видимостью"
// @Success      200 {string} string "Видимость столбцов обновлена"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id}/fields [put]
func (h *SystemTableHandler) UpdateFields(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateFieldsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateFields(c.Request().Context(), id, req); err != nil {
		return err
	}
	h.logAction(c.Request().Context(), c, id, models.SystemTableActionColumnsUpdated, map[string]interface{}{
		"variant": "main",
		"fields":  req.Fields,
	})
	return RespondMessage(c, "Видимость столбцов обновлена")
}

// UpdateFactFields godoc
// @Summary      Bulk-обновление столбцов FactTable (#345)
// @Description  То же что UpdateFields, но для table_fields_fact.
// @Tags         system-tables
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        request body models.UpdateFieldsRequest true "Список столбцов с новой видимостью"
// @Success      200 {string} string "Видимость столбцов FactTable обновлена"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id}/fact-fields [put]
func (h *SystemTableHandler) UpdateFactFields(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateFieldsRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateFactFields(c.Request().Context(), id, req); err != nil {
		return err
	}
	h.logAction(c.Request().Context(), c, id, models.SystemTableActionColumnsUpdated, map[string]interface{}{
		"variant": "fact",
		"fields":  req.Fields,
	})
	return RespondMessage(c, "Видимость столбцов FactTable обновлена")
}

// --- Временные слоты ---

// GetTimeSlots godoc
// @Summary      Получение временных слотов таблицы
// @Description  Возвращает все временные слоты для указанной системной таблицы
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Success      200 {array} models.SystemTableTimeSlot
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /system-tables/{id}/time-slots [get]
func (h *SystemTableHandler) GetTimeSlots(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	slots, err := h.service.GetTimeSlots(c.Request().Context(), tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, slots)
}

// AddTimeSlot godoc
// @Summary      Добавление временного слота
// @Description  Создаёт новый временной слот для системной таблицы
// @Tags         system-tables
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        request body models.CreateTimeSlotRequest true "Данные слота"
// @Success      200 {object} map[string]interface{} "id, message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id}/time-slots [post]
func (h *SystemTableHandler) AddTimeSlot(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.CreateTimeSlotRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.AddTimeSlot(c.Request().Context(), tableID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Временной слот успешно добавлен",
	})
}

// UpdateTimeSlot godoc
// @Summary      Обновление временного слота
// @Description  Обновляет временной слот системной таблицы (частичное обновление)
// @Tags         system-tables
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        table_id path int true "ID таблицы"
// @Param        slot_id path int true "ID слота"
// @Param        request body models.UpdateTimeSlotRequest true "Данные для обновления"
// @Success      200 {string} string "Временной слот успешно обновлен"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{table_id}/time-slots/{slot_id} [put]
func (h *SystemTableHandler) UpdateTimeSlot(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("table_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table_id")
	}
	slotID, err := strconv.Atoi(c.Param("slot_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid slot_id")
	}
	var req models.UpdateTimeSlotRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateTimeSlot(c.Request().Context(), tableID, slotID, req); err != nil {
		return err
	}
	return RespondMessage(c, "Временной слот успешно обновлен")
}

// DeleteTimeSlot godoc
// @Summary      Удаление временного слота
// @Description  Удаляет временной слот системной таблицы
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        table_id path int true "ID таблицы"
// @Param        slot_id path int true "ID слота"
// @Success      200 {string} string "Временной слот успешно удален"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{table_id}/time-slots/{slot_id} [delete]
func (h *SystemTableHandler) DeleteTimeSlot(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("table_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table_id")
	}
	slotID, err := strconv.Atoi(c.Param("slot_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid slot_id")
	}
	if err := h.service.DeleteTimeSlot(c.Request().Context(), tableID, slotID); err != nil {
		return err
	}
	return RespondMessage(c, "Временной слот успешно удален")
}

// --- Предупреждения по временным окнам (#1183) ---

// GetWarningWindows godoc
// @Summary      Получение предупреждений по окнам таблицы
// @Description  Возвращает все предупреждения по временным окнам для системной таблицы
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Success      200 {array} models.SystemTableWarningWindow
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /system-tables/{id}/warning-windows [get]
func (h *SystemTableHandler) GetWarningWindows(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	windows, err := h.service.GetWarningWindows(c.Request().Context(), tableID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, windows)
}

// AddWarningWindow godoc
// @Summary      Добавление предупреждения по окну
// @Description  Создаёт новое предупреждение по временному окну для системной таблицы
// @Tags         system-tables
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        request body models.WarningWindowRequest true "Данные предупреждения"
// @Success      200 {object} map[string]interface{} "id, message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id}/warning-windows [post]
func (h *SystemTableHandler) AddWarningWindow(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.WarningWindowRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.AddWarningWindow(c.Request().Context(), tableID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Предупреждение по окну успешно добавлено",
	})
}

// UpdateWarningWindow godoc
// @Summary      Обновление предупреждения по окну
// @Description  Перезаписывает предупреждение по временному окну целиком
// @Tags         system-tables
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        table_id path int true "ID таблицы"
// @Param        window_id path int true "ID предупреждения по окну"
// @Param        request body models.WarningWindowRequest true "Данные предупреждения"
// @Success      200 {string} string "Предупреждение по окну успешно обновлено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{table_id}/warning-windows/{window_id} [put]
func (h *SystemTableHandler) UpdateWarningWindow(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("table_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table_id")
	}
	windowID, err := strconv.Atoi(c.Param("window_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid window_id")
	}
	var req models.WarningWindowRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateWarningWindow(c.Request().Context(), tableID, windowID, req); err != nil {
		return err
	}
	return RespondMessage(c, "Предупреждение по окну успешно обновлено")
}

// DeleteWarningWindow godoc
// @Summary      Удаление предупреждения по окну
// @Description  Удаляет предупреждение по временному окну из системной таблицы
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        table_id path int true "ID таблицы"
// @Param        window_id path int true "ID предупреждения по окну"
// @Success      200 {string} string "Предупреждение по окну успешно удалено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{table_id}/warning-windows/{window_id} [delete]
func (h *SystemTableHandler) DeleteWarningWindow(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("table_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table_id")
	}
	windowID, err := strconv.Atoi(c.Param("window_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid window_id")
	}
	if err := h.service.DeleteWarningWindow(c.Request().Context(), tableID, windowID); err != nil {
		return err
	}
	return RespondMessage(c, "Предупреждение по окну успешно удалено")
}

// --- Фотографии ---

// UploadPhoto godoc
// @Summary      Загрузка фотографии таблицы
// @Description  Загружает фотографию для системной таблицы (multipart/form-data, поле "file"). Первая фотография автоматически становится главной
// @Tags         system-tables
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID таблицы"
// @Param        file formData file true "Файл фотографии (макс. 10MB)"
// @Success      200 {object} map[string]interface{} "message, photo_ids"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{id}/photos [post]
func (h *SystemTableHandler) UploadPhoto(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	username := c.Get("username").(string)

	saved, err := upload.SaveMultipart(c, "photos", upload.Options{
		Dir:          h.uploadDir,
		URLPrefix:    "/api/uploads/system_tables",
		MaxFileSize:  h.maxFileSize,
		AllowedTypes: allowedImageTypes,
		NameSuffix:   strconv.Itoa(tableID),
	})
	if err != nil {
		return err
	}

	photoIDs := make([]int, 0, len(saved))
	for _, f := range saved {
		id, err := h.service.UploadPhoto(c.Request().Context(), tableID, username, f.URL, f.FileName, f.MimeType, f.Size)
		if err != nil {
			return err
		}
		photoIDs = append(photoIDs, id)
	}

	return RespondSuccess(c, map[string]interface{}{
		"message":   "Фотографии успешно загружены",
		"photo_ids": photoIDs,
	})
}

// DeletePhoto godoc
// @Summary      Удаление фотографии таблицы
// @Description  Удаляет фотографию и файл. Если удалена главная -- следующая по дате становится главной
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        table_id path int true "ID таблицы"
// @Param        photo_id path int true "ID фотографии"
// @Success      200 {string} string "Фотография успешно удалена"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{table_id}/photos/{photo_id} [delete]
func (h *SystemTableHandler) DeletePhoto(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("table_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table_id")
	}
	photoID, err := strconv.Atoi(c.Param("photo_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid photo_id")
	}
	if err := h.service.DeletePhoto(c.Request().Context(), tableID, photoID); err != nil {
		return err
	}
	return RespondMessage(c, "Фотография успешно удалена")
}

// SetMainPhoto godoc
// @Summary      Установка главной фотографии
// @Description  Устанавливает указанную фотографию как главную для таблицы, сбрасывая флаг у остальных
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Param        table_id path int true "ID таблицы"
// @Param        photo_id path int true "ID фотографии"
// @Success      200 {string} string "Главная фотография успешно установлена"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/{table_id}/photos/{photo_id}/main [post]
func (h *SystemTableHandler) SetMainPhoto(c echo.Context) error {
	tableID, err := strconv.Atoi(c.Param("table_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid table_id")
	}
	photoID, err := strconv.Atoi(c.Param("photo_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid photo_id")
	}
	if err := h.service.SetMainPhoto(c.Request().Context(), tableID, photoID); err != nil {
		return err
	}
	return RespondMessage(c, "Главная фотография успешно установлена")
}
