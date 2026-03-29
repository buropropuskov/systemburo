package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// AttachmentHandler -- HTTP-обработчики для шаблонов вложений (unique_attachments).
type AttachmentHandler struct {
	service services.AttachmentService
}

// NewAttachmentHandler создаёт новый экземпляр AttachmentHandler.
func NewAttachmentHandler(service services.AttachmentService) *AttachmentHandler {
	return &AttachmentHandler{service: service}
}

// GetActive обрабатывает GET /attachments.
func (h *AttachmentHandler) GetActive(c echo.Context) error {
	attachments, err := h.service.GetActive(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, attachments)
}

// GetAll обрабатывает GET /attachments/all.
func (h *AttachmentHandler) GetAll(c echo.Context) error {
	attachments, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, attachments)
}

// GetByID обрабатывает GET /attachments/:id.
func (h *AttachmentHandler) GetByID(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}
	attachment, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, attachment)
}

// Create обрабатывает POST /attachments.
func (h *AttachmentHandler) Create(c echo.Context) error {
	var req models.CreateUniqueAttachmentRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	resp, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, resp)
}

// Update обрабатывает PUT /attachments/:id.
func (h *AttachmentHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}
	var req models.UpdateUniqueAttachmentRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.service.Update(c.Request().Context(), id, req); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, "Вложение успешно обновлено")
}

// Delete обрабатывает DELETE /attachments/:id.
func (h *AttachmentHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, "Вложение успешно удалено")
}

// Restore обрабатывает PUT /attachments/:id/restore.
func (h *AttachmentHandler) Restore(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}
	if err := h.service.Restore(c.Request().Context(), id); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, "Вложение успешно восстановлено")
}
