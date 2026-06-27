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
	history     services.SystemTableHistoryService
	maxFileSize int64
	uploadDir   string
}

// NewSystemTableHandler создаёт новый экземпляр обработчика системных таблиц.
// history может быть nil - тогда логирование действий отключено.
func NewSystemTableHandler(service services.SystemTableService, history services.SystemTableHistoryService, maxFileSize int64, uploadDir string) *SystemTableHandler {
	return &SystemTableHandler{
		service:     service,
		history:     history,
		maxFileSize: maxFileSize,
		uploadDir:   filepath.Join(uploadDir, "system_tables"),
	}
}

// logAction пишет запись в историю если сервис подключён. Безопасно вызывать с nil-историей.
func (h *SystemTableHandler) logAction(ctx context.Context, c echo.Context, tableID int, actionType string, details interface{}) {
	if h.history == nil {
		return
	}
	var userID *int
	if v := c.Get("user_id"); v != nil {
		if id, ok := v.(int); ok && id > 0 {
			userID = &id
		}
	}
	_ = h.history.Log(ctx, tableID, userID, actionType, details)
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
// @Success      200 {object} models.SystemTableWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /system-tables/name/{name} [get]
func (h *SystemTableHandler) GetByName(c echo.Context) error {
	name := c.Param("name")
	table, err := h.service.GetByName(c.Request().Context(), name)
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
	if h.history == nil {
		return RespondSuccess(c, []models.SystemTableHistoryItem{})
	}
	items, err := h.history.GetHistory(c.Request().Context(), id)
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
