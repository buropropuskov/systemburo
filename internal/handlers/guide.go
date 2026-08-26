package handlers

import (
	"errors"
	"net/http"
	"path/filepath"
	"strings"

	"systemburo/internal/download"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// GuideHandler -- разделы руководства: read-only API + скачивание PDF (B1) и
// админ-управление (правка текста + PDF-файл по разделу, B1b; авторизация page.admin
// на роут-middleware).
type GuideHandler struct {
	service     services.GuideService
	fileSvc     services.DocumentFileService
	maxFileSize int64
}

// NewGuideHandler. fileSvc указывает на uploads/guide (см. main.go), maxFileSize -- лимит PDF.
func NewGuideHandler(service services.GuideService, fileSvc services.DocumentFileService, maxFileSize int64) *GuideHandler {
	return &GuideHandler{
		service:     service,
		fileSvc:     fileSvc,
		maxFileSize: maxFileSize,
	}
}

// updateGuideContentRequest -- правка текста раздела руководства.
type updateGuideContentRequest struct {
	Lead  string   `json:"lead"`
	Items []string `json:"items"`
}

// ListSections godoc
// @Summary Разделы руководства, доступные пользователю
// @Description Возвращает разделы (user/guard/admin), на которые есть право guide.<role>.
// @Tags guide
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /guide/sections [get]
func (h *GuideHandler) ListSections(c echo.Context) error {
	userID := GetUserID(c)
	sections, err := h.service.ListForUser(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load guide sections")
	}
	return RespondSuccess(c, sections)
}

// Download godoc
// @Summary Скачать PDF раздела руководства
// @Tags guide
// @Param role path string true "Роль раздела (user|guard|admin)"
// @Success 200 {file} binary
// @Failure 403 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security BearerAuth
// @Router /guide/sections/{role}/download [get]
func (h *GuideHandler) Download(c echo.Context) error {
	userID := GetUserID(c)
	role := c.Param("role")
	if !services.IsGuideRole(role) {
		return echo.NewHTTPError(http.StatusNotFound, "guide section not found")
	}

	sec, allowed, err := h.service.GetSectionForUser(c.Request().Context(), userID, role)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load guide section")
	}
	if !allowed {
		return echo.NewHTTPError(http.StatusForbidden, "no access to this guide section")
	}
	if sec == nil || sec.StoredName == "" {
		return echo.NewHTTPError(http.StatusNotFound, "guide file not available")
	}

	path := filepath.Join(h.fileSvc.UploadDir(), sec.StoredName)
	name := sec.FileName
	if name == "" {
		name = "guide.pdf"
	}
	return download.Serve(c, download.File{Path: path, Name: name})
}

// AdminListSections godoc
// @Summary Все разделы руководства (админ-управление)
// @Description Возвращает все разделы без фильтра по правам. Гейт page.admin.
// @Tags guide
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security BearerAuth
// @Router /guide/admin/sections [get]
func (h *GuideHandler) AdminListSections(c echo.Context) error {
	sections, err := h.service.ListAll(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load guide sections")
	}
	return RespondSuccess(c, sections)
}

// UpdateSection godoc
// @Summary Правка текста раздела руководства (lead + items)
// @Tags guide
// @Accept json
// @Produce json
// @Param role path string true "Роль раздела (user|guard|admin)"
// @Param request body updateGuideContentRequest true "lead + items"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security BearerAuth
// @Router /guide/admin/sections/{role} [put]
func (h *GuideHandler) UpdateSection(c echo.Context) error {
	role := c.Param("role")
	if !services.IsGuideRole(role) {
		return echo.NewHTTPError(http.StatusNotFound, "guide section not found")
	}

	var req updateGuideContentRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}

	items := make([]string, 0, len(req.Items))
	for _, it := range req.Items {
		if trimmed := strings.TrimSpace(it); trimmed != "" {
			items = append(items, trimmed)
		}
	}

	resp, err := h.service.UpdateContent(c.Request().Context(), role, strings.TrimSpace(req.Lead), items)
	if err != nil {
		return mapGuideError(err)
	}
	return RespondSuccess(c, resp)
}

// UploadFile godoc
// @Summary Загрузить/заменить PDF раздела руководства
// @Tags guide
// @Accept multipart/form-data
// @Produce json
// @Param role path string true "Роль раздела (user|guard|admin)"
// @Param file formData file true "PDF-файл"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security BearerAuth
// @Router /guide/admin/sections/{role}/file [put]
func (h *GuideHandler) UploadFile(c echo.Context) error {
	role := c.Param("role")
	if !services.IsGuideRole(role) {
		return echo.NewHTTPError(http.StatusNotFound, "guide section not found")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Файл не передан")
	}
	if strings.ToLower(filepath.Ext(file.Filename)) != ".pdf" {
		return echo.NewHTTPError(http.StatusBadRequest, "Недопустимый тип файла. Разрешён только PDF")
	}

	ctx := c.Request().Context()
	// Save валидирует размер и magic-байты %PDF (familyPDF), сохраняет в uploads/guide.
	storedName, savedExt, err := h.fileSvc.Save(ctx, file, h.maxFileSize)
	if err != nil {
		return err
	}

	resp, oldStored, err := h.service.SetFile(ctx, role, services.GuideFileMeta{
		StoredName: storedName,
		FileName:   file.Filename,
		Ext:        savedExt,
		MimeType:   services.DetectMimeType(savedExt),
		Size:       file.Size,
	})
	if err != nil {
		h.fileSvc.Delete(storedName)
		return mapGuideError(err)
	}
	if oldStored != "" && oldStored != storedName {
		h.fileSvc.Delete(oldStored)
	}
	return RespondSuccess(c, resp)
}

// DeleteFile godoc
// @Summary Удалить PDF раздела руководства
// @Tags guide
// @Produce json
// @Param role path string true "Роль раздела (user|guard|admin)"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security BearerAuth
// @Router /guide/admin/sections/{role}/file [delete]
func (h *GuideHandler) DeleteFile(c echo.Context) error {
	role := c.Param("role")
	if !services.IsGuideRole(role) {
		return echo.NewHTTPError(http.StatusNotFound, "guide section not found")
	}

	resp, oldStored, err := h.service.ClearFile(c.Request().Context(), role)
	if err != nil {
		return mapGuideError(err)
	}
	if oldStored != "" {
		h.fileSvc.Delete(oldStored)
	}
	return RespondSuccess(c, resp)
}

// mapGuideError переводит доменные ошибки сервиса в HTTP-статусы.
func mapGuideError(err error) error {
	if errors.Is(err, services.ErrGuideSectionNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "guide section not found")
	}
	return echo.NewHTTPError(http.StatusInternalServerError, "failed to update guide section")
}
