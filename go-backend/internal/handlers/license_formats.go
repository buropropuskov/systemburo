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

// GetAll — GET /license-plate-formats
func (h *LicensePlateFormatHandler) GetAll(c echo.Context) error {
	formats, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, formats)
}

// Create — POST /license-plate-formats
func (h *LicensePlateFormatHandler) Create(c echo.Context) error {
	var req models.CreateLicensePlateFormatRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	id, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, models.CreateFormatResponse{
		ID:      id,
		Message: "Формат номеров успешно создан",
	})
}

// Update — PUT /license-plate-formats/:id
func (h *LicensePlateFormatHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	var req models.UpdateLicensePlateFormatRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := h.service.Update(c.Request().Context(), id, req); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, "Формат номеров успешно обновлен")
}

// Delete — DELETE /license-plate-formats/:id
func (h *LicensePlateFormatHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, "Формат номеров успешно удален")
}
