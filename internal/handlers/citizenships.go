package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// CitizenshipHandler -- HTTP-обработчики гражданств.
type CitizenshipHandler struct {
	service services.CitizenshipService
}

// NewCitizenshipHandler создаёт новый экземпляр обработчика гражданств.
func NewCitizenshipHandler(service services.CitizenshipService) *CitizenshipHandler {
	return &CitizenshipHandler{service: service}
}

// GetAll godoc
// @Summary      Получить все гражданства
// @Description  Возвращает список гражданств. По умолчанию только активные; include_archived=true добавляет архивные.
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        include_archived query bool false "Включить архивные гражданства"
// @Success      200 {array} models.Citizenship
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /citizenships [get]
func (h *CitizenshipHandler) GetAll(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	citizenships, err := h.service.GetAll(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, citizenships)
}

// Create godoc
// @Summary      Создать гражданство
// @Description  Создаёт новое гражданство с указанными параметрами
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateCitizenshipRequest true "Данные нового гражданства"
// @Success      200 {object} map[string]interface{} "id и message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /citizenships [post]
func (h *CitizenshipHandler) Create(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	var req models.CreateCitizenshipRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.Create(c.Request().Context(), userID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Гражданство успешно создано",
	})
}

// Update godoc
// @Summary      Обновить гражданство
// @Description  Обновляет данные гражданства по указанному ID
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID гражданства"
// @Param        request body models.UpdateCitizenshipRequest true "Обновлённые данные гражданства"
// @Success      200 {string} string "Гражданство успешно обновлено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /citizenships/{id} [put]
func (h *CitizenshipHandler) Update(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.UpdateCitizenshipRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Update(c.Request().Context(), userID, id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Гражданство успешно обновлено")
}

// Delete godoc
// @Summary      Архивировать гражданство
// @Description  Архивирует гражданство (soft-delete, is_active=false). Гражданство по умолчанию архивировать нельзя.
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID гражданства"
// @Success      200 {string} string "Гражданство архивировано"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Router       /citizenships/{id} [delete]
func (h *CitizenshipHandler) Delete(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.Delete(c.Request().Context(), userID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Гражданство архивировано")
}

// Restore godoc
// @Summary      Восстановить гражданство из архива
// @Description  Возвращает гражданство из архива (is_active=true)
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID гражданства"
// @Success      200 {string} string "Гражданство восстановлено из архива"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /citizenships/{id}/restore [post]
func (h *CitizenshipHandler) Restore(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.Restore(c.Request().Context(), userID, id); err != nil {
		return err
	}
	return RespondMessage(c, "Гражданство восстановлено из архива")
}

// BulkArchive godoc
// @Summary      Групповая архивация гражданств
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID гражданств"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /citizenships/bulk/archive [post]
func (h *CitizenshipHandler) BulkArchive(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны гражданства")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkArchive(c.Request().Context(), req.IDs, userID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkRestore godoc
// @Summary      Групповое восстановление гражданств
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID гражданств"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /citizenships/bulk/restore [post]
func (h *CitizenshipHandler) BulkRestore(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны гражданства")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkRestore(c.Request().Context(), req.IDs, userID)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// GetHistory godoc
// @Summary      История изменений гражданства
// @Description  Возвращает аудит создания/изменения/архивации/восстановления гражданства (новые сверху)
// @Tags         citizenships
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID гражданства"
// @Success      200 {array} models.CitizenshipHistoryItem
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /citizenships/{id}/history [get]
func (h *CitizenshipHandler) GetHistory(c echo.Context) error {
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

// ClearDefaults godoc
// @Summary      Сбросить гражданства по умолчанию
// @Description  Сбрасывает флаг is_default у всех гражданств
// @Tags         citizenships
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {string} string "Все гражданства по умолчанию сброшены"
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Router       /citizenships/clear-default [post]
func (h *CitizenshipHandler) ClearDefaults(c echo.Context) error {
	if err := h.service.ClearDefaults(c.Request().Context()); err != nil {
		return err
	}
	return RespondMessage(c, "Все гражданства по умолчанию сброшены")
}
