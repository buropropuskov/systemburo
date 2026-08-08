package handlers

import (
	"net/http"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// AttachmentImportHandler - HTTP API приёма заполненного Excel-бланка для массового
// ввода участников/машин заявки (blank-import, срез C1C2).
type AttachmentImportHandler struct {
	service services.AttachmentImportService
}

// NewAttachmentImportHandler создаёт handler.
func NewAttachmentImportHandler(s services.AttachmentImportService) *AttachmentImportHandler {
	return &AttachmentImportHandler{service: s}
}

// ImportList godoc
// @Summary      Загрузить заполненный Excel-бланк для массового ввода списка
// @Description  Гейт action.import.list, право не super-only. Кривой бланк (не .xlsx, чужой тип вложения, изменённая структура колонок, пустой или слишком длинный список) отлетает целиком с понятным текстом, до разбора отдельных строк - построчный разбор появится следующим срезом.
// @Tags         attachment-blanks
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID UniqueAttachment"
// @Param        file formData file true "Заполненный .xlsx бланк"
// @Success      200 {object} services.ImportListResult
// @Failure      400 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /attachments/{id}/import-list [post]
func (h *AttachmentImportHandler) ImportList(c echo.Context) error {
	uaID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "file required")
	}

	result, err := h.service.ImportList(c.Request().Context(), uaID, file)
	if err != nil {
		return err
	}
	return RespondSuccess(c, result)
}
