package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// GuideHandler -- разделы руководства (read-only API + скачивание PDF).
type GuideHandler struct {
	service   services.GuideService
	uploadDir string
}

// NewGuideHandler. uploadPath -- корень uploads; PDF разделов лежат в uploads/guide.
func NewGuideHandler(service services.GuideService, uploadPath string) *GuideHandler {
	return &GuideHandler{
		service:   service,
		uploadDir: filepath.Join(uploadPath, "guide"),
	}
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

	path := filepath.Join(h.uploadDir, sec.StoredName)
	if _, statErr := os.Stat(path); statErr != nil {
		return echo.NewHTTPError(http.StatusNotFound, "guide file not available")
	}

	name := sec.FileName
	if name == "" {
		name = "guide.pdf"
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+sanitizeHeader(name)+`"`)
	return c.File(path)
}
