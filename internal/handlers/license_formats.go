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
// @Description  Возвращает список всех форматов номерных знаков с их ячейками
// @Tags         license-formats
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.LicensePlateFormatWithCells
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /license-plate-formats [get]
func (h *LicensePlateFormatHandler) GetAll(c echo.Context) error {
	formats, err := h.service.GetAll(c.Request().Context())
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

	id, err := h.service.Create(c.Request().Context(), req)
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

	if err := h.service.Update(c.Request().Context(), id, req); err != nil {
		return err
	}

	return RespondMessage(c, "Формат номеров успешно обновлен")
}

// Delete godoc
// @Summary      Удалить формат номерного знака
// @Description  Удаляет формат номерного знака по указанному ID
// @Tags         license-formats
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID формата"
// @Success      200 {string} string "Формат номеров успешно удален"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /license-plate-formats/{id} [delete]
func (h *LicensePlateFormatHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}

	return RespondMessage(c, "Формат номеров успешно удален")
}
