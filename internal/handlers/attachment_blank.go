package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// AttachmentBlankHandler - HTTP API скачивания заполненных Excel-бланков (#183).
type AttachmentBlankHandler struct {
	service services.AttachmentBlankService
}

// NewAttachmentBlankHandler создаёт handler.
func NewAttachmentBlankHandler(s services.AttachmentBlankService) *AttachmentBlankHandler {
	return &AttachmentBlankHandler{service: s}
}

// Download godoc
// @Summary      Скачать заполненный бланк для одного вложения заявки
// @Tags         attachment-blanks
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        attachment_id query int true "ID Attachment"
// @Success      200
// @Router       /applications/{id}/blank [get]
func (h *AttachmentBlankHandler) Download(c echo.Context) error {
	appID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid application id")
	}
	attID, err := strconv.Atoi(c.QueryParam("attachment_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "attachment_id required")
	}
	reader, filename, err := h.service.GenerateBlank(c.Request().Context(), appID, attID)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Type",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition",
		`attachment; filename="`+sanitizeHeader(filename)+`"`)
	return c.Stream(http.StatusOK,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", reader)
}

// sanitizeHeader убирает " и переносы строк из имени файла для Content-Disposition.
func sanitizeHeader(s string) string {
	return strings.NewReplacer(`"`, "", "\n", "", "\r", "").Replace(s)
}
