package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// ListTemplates godoc
// @Summary      Список шаблонов отчётов
// @Description  Системные пресеты + личные шаблоны пользователя + расшаренные
// @Tags         statistics
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Security     BearerAuth
// @Router       /statistics/templates [get]
func (h *StatisticsHandler) ListTemplates(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	templates, err := h.service.ListReportTemplates(c.Request().Context(), userID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, templates)
}

// CreateTemplate godoc
// @Summary      Сохранить личный шаблон отчёта
// @Tags         statistics
// @Accept       json
// @Produce      json
// @Param        body body models.SaveReportTemplateRequest true "Шаблон"
// @Success      201 {object} models.ReportTemplate
// @Security     BearerAuth
// @Router       /statistics/templates [post]
func (h *StatisticsHandler) CreateTemplate(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	var req models.SaveReportTemplateRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	tpl, err := h.service.CreateReportTemplate(c.Request().Context(), userID, req)
	if err != nil {
		return templateHTTPError(err)
	}
	return RespondCreated(c, tpl)
}

// UpdateTemplate godoc
// @Summary      Обновить личный шаблон отчёта
// @Tags         statistics
// @Accept       json
// @Produce      json
// @Param        id path int true "ID шаблона"
// @Param        body body models.SaveReportTemplateRequest true "Шаблон"
// @Success      200 {object} models.ReportTemplate
// @Security     BearerAuth
// @Router       /statistics/templates/{id} [put]
func (h *StatisticsHandler) UpdateTemplate(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req models.SaveReportTemplateRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	tpl, err := h.service.UpdateReportTemplate(c.Request().Context(), userID, id, req)
	if err != nil {
		return templateHTTPError(err)
	}
	return RespondSuccess(c, tpl)
}

// DeleteTemplate godoc
// @Summary      Удалить личный шаблон отчёта
// @Tags         statistics
// @Produce      json
// @Param        id path int true "ID шаблона"
// @Success      200 {object} map[string]interface{}
// @Security     BearerAuth
// @Router       /statistics/templates/{id} [delete]
func (h *StatisticsHandler) DeleteTemplate(c echo.Context) error {
	userID, _ := c.Get("user_id").(int)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.DeleteReportTemplate(c.Request().Context(), userID, id); err != nil {
		return templateHTTPError(err)
	}
	return RespondMessage(c, "Шаблон удалён")
}

// templateHTTPError переводит доменные ошибки шаблонов в HTTP-статусы.
func templateHTTPError(err error) error {
	switch {
	case errors.Is(err, services.ErrTemplateNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "Шаблон не найден")
	case errors.Is(err, services.ErrTemplateForbidden):
		return echo.NewHTTPError(http.StatusForbidden, "Нет доступа к шаблону")
	case errors.Is(err, services.ErrTemplateSystem):
		return echo.NewHTTPError(http.StatusForbidden, "Системный шаблон нельзя изменить")
	case errors.Is(err, services.ErrTemplateInvalidConfig):
		return echo.NewHTTPError(http.StatusBadRequest, "Некорректная конфигурация шаблона")
	default:
		return err
	}
}
