package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// blankAccessService - гейты доступа, нужные для скачивания бланка. Узкий интерфейс
// поверх ApplicationService: участник заявки проверяется как в детали заявки, охрана -
// как в детали вкладки "Доступные мне".
type blankAccessService interface {
	CanAccessApplication(ctx context.Context, applicationID int, username string, isSuperAdmin bool) bool
	CanSecurityViewAttachment(ctx context.Context, userID int, unrestricted bool, attachmentID int) (bool, error)
	IsSecurityUser(ctx context.Context, userID int) (bool, error)
}

// AttachmentBlankHandler - HTTP API скачивания заполненных Excel-бланков (#183).
type AttachmentBlankHandler struct {
	service  services.AttachmentBlankService
	access   blankAccessService
	resolver *services.PermissionResolver
}

// NewAttachmentBlankHandler создаёт handler.
func NewAttachmentBlankHandler(s services.AttachmentBlankService, access blankAccessService, resolver *services.PermissionResolver) *AttachmentBlankHandler {
	return &AttachmentBlankHandler{service: s, access: access, resolver: resolver}
}

// Download godoc
// @Summary      Скачать заполненный бланк для одного вложения заявки
// @Description  Доступ: участник заявки (как у детали заявки) либо охрана/носитель page.available по своему вложению (как у детали вкладки "Доступные мне"). Прочим - 403.
// @Tags         attachment-blanks
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Param        attachment_id query int true "ID Attachment"
// @Success      200
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
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

	allowed, err := h.canDownload(c, appID, attID)
	if err != nil {
		return err
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}

	reader, filename, err := h.service.GenerateBlank(c.Request().Context(), appID, attID)
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Type",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	encoded := url.PathEscape(filename)
	c.Response().Header().Set("Content-Disposition",
		`attachment; filename="blank.xlsx"; filename*=UTF-8''`+encoded)
	return c.Stream(http.StatusOK,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", reader)
}

// canDownload повторяет обе точки, откуда бланк запрашивают: деталь заявки (DownloadBlanksModal)
// и вкладка "Доступные мне" (AccessibleAttachmentsView). Охранник заявку не видит вовсе, поэтому
// одного CanAccessApplication мало - у него свой гейт по конкретному вложению.
func (h *AttachmentBlankHandler) canDownload(c echo.Context, appID, attID int) (bool, error) {
	ctx := c.Request().Context()
	if username, ok := c.Get("username").(string); ok && username != "" {
		if h.access.CanAccessApplication(ctx, appID, username, IsSuperAdmin(c)) {
			return true, nil
		}
	}

	userID, unrestricted, err := securityScope(c, h.access, h.resolver)
	if err != nil {
		var he *echo.HTTPError
		if errors.As(err, &he) && he.Code == http.StatusForbidden {
			// Не участник и не охрана - обычный отказ, а не сбой резолвера.
			return false, nil
		}
		return false, err
	}
	return h.access.CanSecurityViewAttachment(ctx, userID, unrestricted, attID)
}
