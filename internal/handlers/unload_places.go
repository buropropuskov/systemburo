package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/upload"

	"github.com/labstack/echo/v4"
)

// allowedImageTypes -- допустимые MIME-типы для загрузки фотографий мест разгрузки.
var allowedImageTypes = []string{
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
}

// UnloadPlaceHandler -- HTTP-обработчики мест разгрузки.
type UnloadPlaceHandler struct {
	service     services.UnloadPlaceService
	maxFileSize int64
	uploadDir   string
}

// NewUnloadPlaceHandler создаёт новый экземпляр обработчика мест разгрузки.
func NewUnloadPlaceHandler(service services.UnloadPlaceService, maxFileSize int64, uploadDir string) *UnloadPlaceHandler {
	return &UnloadPlaceHandler{
		service:     service,
		maxFileSize: maxFileSize,
		uploadDir:   filepath.Join(uploadDir, "unload_places"),
	}
}

// GetAll возвращает все места разгрузки с деталями.
// @Summary      Получение всех мест разгрузки
// @Description  Возвращает список мест разгрузки с расписанием, фото и текущим статусом (open/closed)
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        include_archived query bool false "Включить архивные места"
// @Success      200 {array} services.UnloadPlaceWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places [get]
func (h *UnloadPlaceHandler) GetAll(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	places, err := h.service.GetAll(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, places)
}

// GetByID возвращает место разгрузки по ID.
// @Summary      Получение места разгрузки по ID
// @Description  Возвращает место разгрузки с расписанием, фото и текущим статусом
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {object} services.UnloadPlaceWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id} [get]
func (h *UnloadPlaceHandler) GetByID(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	place, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, place)
}

// Create создаёт новое место разгрузки.
// @Summary      Создание места разгрузки
// @Description  Создаёт новое место разгрузки с указанными параметрами
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.CreateUnloadPlaceRequest true "Данные места разгрузки"
// @Success      200 {object} map[string]interface{} "id и message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places [post]
func (h *UnloadPlaceHandler) Create(c echo.Context) error {
	var req services.CreateUnloadPlaceRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	id, err := h.service.Create(c.Request().Context(), userID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Место разгрузки успешно создано",
	})
}

// Update обновляет место разгрузки.
// @Summary      Обновление места разгрузки
// @Description  Обновляет поля места разгрузки (только переданные)
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Param        request body services.UpdateUnloadPlaceRequest true "Обновляемые поля"
// @Success      200 {string} string "Место разгрузки успешно обновлено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id} [put]
func (h *UnloadPlaceHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req services.UpdateUnloadPlaceRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Update(c.Request().Context(), userID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Место разгрузки успешно обновлено")
}

// Delete удаляет место разгрузки.
// @Summary      Удаление места разгрузки
// @Description  Удаляет место разгрузки. Нельзя удалить если привязано к организациям или компаниям
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {string} string "Место разгрузки успешно удалено"
// @Failure      400 {object} models.HTTPError "Привязано к организациям/компаниям"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id} [delete]
func (h *UnloadPlaceHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Delete(c.Request().Context(), userID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Место разгрузки архивировано")
}

// Restore восстанавливает архивное место разгрузки.
// @Summary      Восстановление места разгрузки из архива
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {string} string "Место разгрузки восстановлено"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/restore [post]
func (h *UnloadPlaceHandler) Restore(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	userID, _ := c.Get("user_id").(int)
	if err := h.service.Restore(c.Request().Context(), userID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Место разгрузки восстановлено")
}

// GetUsage возвращает организации и компании, привязанные к месту разгрузки.
// @Summary      Привязки места разгрузки
// @Description  Организации и компании, к которым привязано место разгрузки (те же, что блокируют удаление)
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {object} services.UnloadPlaceUsage
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/usage [get]
func (h *UnloadPlaceHandler) GetUsage(c echo.Context) error {
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

// DetachAll снимает привязки места разгрузки ко всем организациям и компаниям.
// @Summary      Отвязать место разгрузки от всех организаций и компаний
// @Description  Разом снимает все привязки места к организациям/компаниям (с записью в историю каждой). После этого место можно архивировать
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {object} services.UnloadPlaceDetachResult
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/detach-all [post]
func (h *UnloadPlaceHandler) DetachAll(c echo.Context) error {
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

// DetachOrganization снимает привязку места разгрузки к одной организации.
// @Summary      Отвязать место разгрузки от организации
// @Description  Снимает привязку места к конкретной организации (с записью в её историю). Идемпотентно
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Param        org_id path int true "ID организации"
// @Success      200 {object} map[string]bool
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/organizations/{org_id} [delete]
func (h *UnloadPlaceHandler) DetachOrganization(c echo.Context) error {
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

// DetachCompany снимает привязку места разгрузки к одной компании.
// @Summary      Отвязать место разгрузки от компании
// @Description  Снимает привязку места к конкретной компании (с записью в её историю). Идемпотентно
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Param        company_id path int true "ID компании"
// @Success      200 {object} map[string]bool
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/companies/{company_id} [delete]
func (h *UnloadPlaceHandler) DetachCompany(c echo.Context) error {
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

// BulkArchive godoc
// @Summary      Групповая архивация мест разгрузки
// @Description  Архивирует набор мест разгрузки. Привязанные к организациям/компаниям попадают в Errors (частичный успех)
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID мест разгрузки"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /unload-places/bulk/archive [post]
func (h *UnloadPlaceHandler) BulkArchive(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны места разгрузки")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkArchive(c.Request().Context(), userID, req.IDs)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkRestore godoc
// @Summary      Групповое восстановление мест разгрузки
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID мест разгрузки"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /unload-places/bulk/restore [post]
func (h *UnloadPlaceHandler) BulkRestore(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны места разгрузки")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkRestore(c.Request().Context(), userID, req.IDs)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// GetHistory возвращает историю изменений места разгрузки. Без отдельного
// admin-гейта - намеренно, как и весь CRUD мест разгрузки (в отличие от
// org/company, где история под buropropuskov): доступ управляется группой protected.
// @Summary      История изменений места разгрузки
// @Description  Возвращает аудит создания/переименования/архивации/восстановления места разгрузки
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {array} models.UnloadPlaceHistoryItem
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/history [get]
func (h *UnloadPlaceHandler) GetHistory(c echo.Context) error {
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

// --- Временные слоты ---

// GetTimeSlots возвращает временные слоты места разгрузки.
// @Summary      Получение временных слотов
// @Description  Возвращает все временные слоты для указанного места разгрузки
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {array} models.UnloadPlaceTimeSlot
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/time-slots [get]
func (h *UnloadPlaceHandler) GetTimeSlots(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	slots, err := h.service.GetTimeSlots(c.Request().Context(), placeID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, slots)
}

// AddTimeSlot добавляет временной слот к месту разгрузки.
// @Summary      Добавление временного слота
// @Description  Создаёт новый временной слот для места разгрузки
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Param        request body services.CreateTimeSlotRequest true "Данные временного слота"
// @Success      200 {object} map[string]interface{} "id и message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/time-slots [post]
func (h *UnloadPlaceHandler) AddTimeSlot(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req services.CreateTimeSlotRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.AddTimeSlot(c.Request().Context(), placeID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Временной слот успешно добавлен",
	})
}

// UpdateTimeSlot обновляет временной слот.
// @Summary      Обновление временного слота
// @Description  Обновляет поля временного слота (только переданные)
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        place_id path int true "ID места разгрузки"
// @Param        slot_id path int true "ID временного слота"
// @Param        request body services.UpdateTimeSlotRequest true "Обновляемые поля"
// @Success      200 {string} string "Временной слот успешно обновлен"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{place_id}/time-slots/{slot_id} [put]
func (h *UnloadPlaceHandler) UpdateTimeSlot(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("place_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid place_id")
	}
	slotID, err := strconv.Atoi(c.Param("slot_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid slot_id")
	}
	var req services.UpdateTimeSlotRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateTimeSlot(c.Request().Context(), placeID, slotID, req); err != nil {
		return err
	}
	return RespondMessage(c, "Временной слот успешно обновлен")
}

// DeleteTimeSlot удаляет временной слот.
// @Summary      Удаление временного слота
// @Description  Удаляет временной слот из места разгрузки
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        place_id path int true "ID места разгрузки"
// @Param        slot_id path int true "ID временного слота"
// @Success      200 {string} string "Временной слот успешно удален"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{place_id}/time-slots/{slot_id} [delete]
func (h *UnloadPlaceHandler) DeleteTimeSlot(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("place_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid place_id")
	}
	slotID, err := strconv.Atoi(c.Param("slot_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid slot_id")
	}
	if err := h.service.DeleteTimeSlot(c.Request().Context(), placeID, slotID); err != nil {
		return err
	}
	return RespondMessage(c, "Временной слот успешно удален")
}

// --- Предупреждения по временным окнам (#1183) ---

// GetWarningWindows возвращает предупреждения по временным окнам места разгрузки.
// @Summary      Получение предупреждений по окнам
// @Description  Возвращает все предупреждения по временным окнам для места разгрузки
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {array} models.UnloadPlaceWarningWindow
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/warning-windows [get]
func (h *UnloadPlaceHandler) GetWarningWindows(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	windows, err := h.service.GetWarningWindows(c.Request().Context(), placeID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, windows)
}

// AddWarningWindow добавляет предупреждение по временному окну к месту разгрузки.
// @Summary      Добавление предупреждения по окну
// @Description  Создаёт новое предупреждение по временному окну для места разгрузки
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Param        request body models.WarningWindowRequest true "Данные предупреждения"
// @Success      200 {object} map[string]interface{} "id и message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/warning-windows [post]
func (h *UnloadPlaceHandler) AddWarningWindow(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.WarningWindowRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.AddWarningWindow(c.Request().Context(), placeID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Предупреждение по окну успешно добавлено",
	})
}

// UpdateWarningWindow обновляет предупреждение по временному окну.
// @Summary      Обновление предупреждения по окну
// @Description  Перезаписывает предупреждение по временному окну целиком
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        place_id path int true "ID места разгрузки"
// @Param        window_id path int true "ID предупреждения по окну"
// @Param        request body models.WarningWindowRequest true "Данные предупреждения"
// @Success      200 {string} string "Предупреждение по окну успешно обновлено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{place_id}/warning-windows/{window_id} [put]
func (h *UnloadPlaceHandler) UpdateWarningWindow(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("place_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid place_id")
	}
	windowID, err := strconv.Atoi(c.Param("window_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid window_id")
	}
	var req models.WarningWindowRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateWarningWindow(c.Request().Context(), placeID, windowID, req); err != nil {
		return err
	}
	return RespondMessage(c, "Предупреждение по окну успешно обновлено")
}

// DeleteWarningWindow удаляет предупреждение по временному окну.
// @Summary      Удаление предупреждения по окну
// @Description  Удаляет предупреждение по временному окну из места разгрузки
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        place_id path int true "ID места разгрузки"
// @Param        window_id path int true "ID предупреждения по окну"
// @Success      200 {string} string "Предупреждение по окну успешно удалено"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{place_id}/warning-windows/{window_id} [delete]
func (h *UnloadPlaceHandler) DeleteWarningWindow(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("place_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid place_id")
	}
	windowID, err := strconv.Atoi(c.Param("window_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid window_id")
	}
	if err := h.service.DeleteWarningWindow(c.Request().Context(), placeID, windowID); err != nil {
		return err
	}
	return RespondMessage(c, "Предупреждение по окну успешно удалено")
}

// --- Фотографии ---

// UploadPhoto загружает фотографию для места разгрузки.
// @Summary      Загрузка фотографии
// @Description  Загружает фотографию (multipart/form-data). Первая фотография становится главной
// @Tags         unload-places
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Param        file formData file true "Файл фотографии (макс. 10MB)"
// @Success      200 {object} map[string]interface{} "message и photo_ids"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/photos [post]
func (h *UnloadPlaceHandler) UploadPhoto(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	username := c.Get("username").(string)

	saved, err := upload.SaveMultipart(c, "photos", upload.Options{
		Dir:          h.uploadDir,
		URLPrefix:    "/api/uploads/unload_places",
		MaxFileSize:  h.maxFileSize,
		AllowedTypes: allowedImageTypes,
		NameSuffix:   strconv.Itoa(placeID),
	})
	if err != nil {
		return err
	}

	insertedIDs := make([]int, 0, len(saved))
	for _, f := range saved {
		id, err := h.service.UploadPhoto(
			c.Request().Context(), placeID, username,
			f.URL, f.FileName, f.MimeType, f.Size,
		)
		if err != nil {
			return err
		}
		insertedIDs = append(insertedIDs, id)
	}

	return RespondSuccess(c, map[string]interface{}{
		"message":   "Фотографии успешно загружены",
		"photo_ids": insertedIDs,
	})
}

// DeletePhoto удаляет фотографию места разгрузки.
// @Summary      Удаление фотографии
// @Description  Удаляет фотографию. Если удалена главная, следующая по дате загрузки становится главной
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        place_id path int true "ID места разгрузки"
// @Param        photo_id path int true "ID фотографии"
// @Success      200 {string} string "Фотография успешно удалена"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{place_id}/photos/{photo_id} [delete]
func (h *UnloadPlaceHandler) DeletePhoto(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("place_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid place_id")
	}
	photoID, err := strconv.Atoi(c.Param("photo_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid photo_id")
	}

	photoURL, err := h.service.DeletePhoto(c.Request().Context(), placeID, photoID)
	if err != nil {
		return err
	}

	// Удаляем файл с диска по реальному пути (не по публичному URL).
	filePath := filepath.Join(h.uploadDir, filepath.Base(photoURL))
	if _, statErr := os.Stat(filePath); statErr == nil {
		_ = os.Remove(filePath)
	}

	return RespondMessage(c, "Фотография успешно удалена")
}

// SetMainPhoto устанавливает главную фотографию для места разгрузки.
// @Summary      Установка главной фотографии
// @Description  Устанавливает указанную фотографию как главную, сбрасывая флаг у остальных
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        place_id path int true "ID места разгрузки"
// @Param        photo_id path int true "ID фотографии"
// @Success      200 {string} string "Главная фотография успешно установлена"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{place_id}/photos/{photo_id}/main [post]
func (h *UnloadPlaceHandler) SetMainPhoto(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("place_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid place_id")
	}
	photoID, err := strconv.Atoi(c.Param("photo_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid photo_id")
	}
	if err := h.service.SetMainPhoto(c.Request().Context(), placeID, photoID); err != nil {
		return err
	}
	return RespondMessage(c, "Главная фотография успешно установлена")
}
