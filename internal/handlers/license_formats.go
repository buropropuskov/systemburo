package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

type LicensePlateFormatHandler struct {
	service services.LicensePlateFormatService
}

func NewLicensePlateFormatHandler(service services.LicensePlateFormatService) *LicensePlateFormatHandler {
	return &LicensePlateFormatHandler{service: service}
}

// GetAll godoc
// @Summary      Получить все форматы номерных знаков
// @Description  Возвращает список форматов номерных знаков с их ячейками. По умолчанию только активные; include_archived=true добавляет архивные.
// @Tags         license-formats
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        include_archived query bool false "Включить архивные форматы"
// @Success      200 {array} models.LicensePlateFormatWithCells
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /license-plate-formats [get]
func (h *LicensePlateFormatHandler) GetAll(c echo.Context) error {
	includeArchived := c.QueryParam("include_archived") == "true"
	formats, err := h.service.GetAll(c.Request().Context(), includeArchived)
	if err != nil {
		return err
	}
	return RespondSuccess(c, formats)
}

// Create godoc
// @Summary      Создать формат номерного знака
// @Description  Создаёт новый формат номерного знака с ячейками
// @Tags         license-formats
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateLicensePlateFormatRequest true "Данные нового формата"
// @Success      200 {object} models.CreateFormatResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /license-plate-formats [post]
func (h *LicensePlateFormatHandler) Create(c echo.Context) error {
	var req models.CreateLicensePlateFormatRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	id, err := h.service.Create(c.Request().Context(), userID, req)
	if err != nil {
		return err
	}

	return RespondSuccess(c, models.CreateFormatResponse{
		ID:      id,
		Message: "Формат номеров успешно создан",
	})
}

// Update godoc
// @Summary      Обновить формат номерного знака
// @Description  Обновляет формат номерного знака и его ячейки по указанному ID
// @Tags         license-formats
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID формата"
// @Param        request body models.UpdateLicensePlateFormatRequest true "Обновлённые данные формата"
// @Success      200 {string} string "Формат номеров успешно обновлен"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /license-plate-formats/{id} [put]
func (h *LicensePlateFormatHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	var req models.UpdateLicensePlateFormatRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.Update(c.Request().Context(), userID, id, req); err != nil {
		return err
	}

	return RespondMessage(c, "Формат номеров успешно обновлен")
}

// Delete godoc
// @Summary      Архивировать формат номерного знака
// @Description  Архивирует формат (soft-delete, is_active=false). Формат по умолчанию архивировать нельзя.
// @Tags         license-formats
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID формата"
// @Success      200 {string} string "Формат номеров архивирован"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      409 {object} models.HTTPError
// @Router       /license-plate-formats/{id} [delete]
func (h *LicensePlateFormatHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.Delete(c.Request().Context(), userID, id); err != nil {
		return err
	}

	return RespondMessage(c, "Формат номеров архивирован")
}

// Restore godoc
// @Summary      Восстановить формат номерного знака из архива
// @Description  Возвращает формат из архива (is_active=true)
// @Tags         license-formats
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID формата"
// @Success      200 {string} string "Формат номеров восстановлен из архива"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /license-plate-formats/{id}/restore [post]
func (h *LicensePlateFormatHandler) Restore(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	userID, _ := c.Get("user_id").(int)
	if err := h.service.Restore(c.Request().Context(), userID, id); err != nil {
		return err
	}

	return RespondMessage(c, "Формат номеров восстановлен из архива")
}

// BulkArchive godoc
// @Summary      Групповая архивация форматов номеров
// @Tags         license-formats
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID форматов"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /license-plate-formats/bulk/archive [post]
func (h *LicensePlateFormatHandler) BulkArchive(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны форматы")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkArchive(c.Request().Context(), userID, req.IDs)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// BulkRestore godoc
// @Summary      Групповое восстановление форматов номеров
// @Tags         license-formats
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.BulkIDsRequest true "Список ID форматов"
// @Success      200 {object} services.BulkOpResult
// @Success      207 {object} services.BulkOpResult "Частичный успех"
// @Failure      400 {object} models.HTTPError
// @Router       /license-plate-formats/bulk/restore [post]
func (h *LicensePlateFormatHandler) BulkRestore(c echo.Context) error {
	var req services.BulkIDsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}
	if len(req.IDs) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Не выбраны форматы")
	}
	userID, _ := c.Get("user_id").(int)
	res, err := h.service.BulkRestore(c.Request().Context(), userID, req.IDs)
	if err != nil {
		return err
	}
	return respondBulk(c, res)
}

// GetHistory godoc
// @Summary      История изменений формата номерного знака
// @Description  Возвращает аудит создания/изменения/архивации/восстановления формата (новые сверху)
// @Tags         license-formats
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID формата"
// @Success      200 {array} models.LicensePlateFormatHistoryItem
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /license-plate-formats/{id}/history [get]
func (h *LicensePlateFormatHandler) GetHistory(c echo.Context) error {
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
