package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// SystemTableHandler -- HTTP-обработчики системных таблиц.
type SystemTableHandler struct {
	service services.SystemTableService
}

// NewSystemTableHandler создаёт новый экземпляр обработчика системных таблиц.
func NewSystemTableHandler(service services.SystemTableService) *SystemTableHandler {
	return &SystemTableHandler{service: service}
}

// GetAll godoc
// @Summary      Получение всех системных таблиц
// @Description  Возвращает все активные системные таблицы с полями, слотами, фото и текущим статусом (open/closed)
// @Tags         system-tables
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.SystemTableWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /system-tables [get]
func (h *SystemTableHandler) GetAll(c echo.Context) error {
	tables, err := h.service.GetAll(c.Request().Context())
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
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	id, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return err
	}
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
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.Update(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Системная таблица успешно обновлена")
}

// Delete godoc
// @Summary      Удаление системной таблицы
// @Description  Мягкое удаление (is_active=false). Проверяет привязки к организациям и компаниям
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
	return RespondMessage(c, "Системная таблица успешно удалена")
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
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
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
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
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

	form, err := c.MultipartForm()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Error reading multipart")
	}

	files := form.File["file"]
	if len(files) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "No files provided")
	}

	var photoIDs []int
	for _, file := range files {
		id, err := h.service.UploadPhoto(c.Request().Context(), tableID, username, file)
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
