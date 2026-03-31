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

// GetActive godoc
// @Summary      Получить активные шаблоны вложений
// @Description  Возвращает список активных шаблонов вложений (is_active = true)
// @Tags         attachments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.UniqueAttachment
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /attachments [get]
func (h *AttachmentHandler) GetActive(c echo.Context) error {
	attachments, err := h.service.GetActive(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, attachments)
}

// GetAll godoc
// @Summary      Получить все шаблоны вложений
// @Description  Возвращает полный список шаблонов вложений, включая неактивные
// @Tags         attachments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} models.UniqueAttachment
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /attachments/all [get]
func (h *AttachmentHandler) GetAll(c echo.Context) error {
	attachments, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, attachments)
}

// GetByID godoc
// @Summary      Получить шаблон вложения по ID
// @Description  Возвращает шаблон вложения по указанному идентификатору
// @Tags         attachments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID шаблона вложения"
// @Success      200 {object} models.UniqueAttachment
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /attachments/{id} [get]
func (h *AttachmentHandler) GetByID(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}
	attachment, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, attachment)
}

// Create godoc
// @Summary      Создать шаблон вложения
// @Description  Создаёт новый шаблон вложения с указанными параметрами
// @Tags         attachments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body models.CreateUniqueAttachmentRequest true "Данные нового шаблона вложения"
// @Success      200 {object} models.CreateUniqueAttachmentResponse
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /attachments [post]
func (h *AttachmentHandler) Create(c echo.Context) error {
	var req models.CreateUniqueAttachmentRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	resp, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, resp)
}

// Update godoc
// @Summary      Обновить шаблон вложения
// @Description  Обновляет данные шаблона вложения по указанному ID
// @Tags         attachments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID шаблона вложения"
// @Param        request body models.UpdateUniqueAttachmentRequest true "Обновлённые данные шаблона"
// @Success      200 {string} string "Вложение успешно обновлено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /attachments/{id} [put]
func (h *AttachmentHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}
	var req models.UpdateUniqueAttachmentRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Update(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Вложение успешно обновлено")
}

// Delete godoc
// @Summary      Удалить шаблон вложения
// @Description  Мягкое удаление шаблона вложения по указанному ID (is_active = false)
// @Tags         attachments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID шаблона вложения"
// @Success      200 {string} string "Вложение успешно удалено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /attachments/{id} [delete]
func (h *AttachmentHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondMessage(c, "Вложение успешно удалено")
}

// Restore godoc
// @Summary      Восстановить шаблон вложения
// @Description  Восстанавливает ранее удалённый шаблон вложения (is_active = true)
// @Tags         attachments
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID шаблона вложения"
// @Success      200 {string} string "Вложение успешно восстановлено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /attachments/{id}/restore [put]
func (h *AttachmentHandler) Restore(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid attachment ID")
	}
	if err := h.service.Restore(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondMessage(c, "Вложение успешно восстановлено")
}
