package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"systemburo/internal/pdsubject"
)

// PDSubjectHandler - сведения о субъекте персональных данных (#2356): поиск человека,
// состав того, что система о нём хранит, и выгрузка справки с записью в журнал выдач.
//
// Раздел закрыт правом page.admin.pd_subject, выгрузка - парным правом
// action.pd_subject.export: увиденное на экране остаётся в системе, а выданный файл
// уходит третьему лицу и живёт дальше сам по себе.
type PDSubjectHandler struct {
	service *pdsubject.Service
}

func NewPDSubjectHandler(s *pdsubject.Service) *PDSubjectHandler {
	return &PDSubjectHandler{service: s}
}

// Find godoc
// @Summary      Поиск человека по имени
// @Description  Записи реестра и участники заявок с таким же ФИО. Склейки по имени нет: однофамильцы существуют, решает оператор. Право page.admin.pd_subject.
// @Tags         pd-subject
// @Produce      json
// @Security     BearerAuth
// @Param        fio query string true "Фамилия Имя Отчество"
// @Success      200 {array} pdsubject.Candidate
// @Router       /pd-subject/candidates [get]
func (h *PDSubjectHandler) Find(c echo.Context) error {
	found, err := h.service.FindByName(c.Request().Context(), c.QueryParam("fio"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return RespondSuccess(c, found)
}

// Report godoc
// @Summary      Сведения о человеке
// @Description  Что система хранит о человеке: сведения, заявки, проходы, посты. Право page.admin.pd_subject.
// @Tags         pd-subject
// @Produce      json
// @Security     BearerAuth
// @Param        registry_id query int true "Идентификатор записи реестра"
// @Success      200 {object} pdsubject.ReportResponse
// @Router       /pd-subject/report [get]
func (h *PDSubjectHandler) Report(c echo.Context) error {
	registryID, err := strconv.Atoi(c.QueryParam("registry_id"))
	if err != nil || registryID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "registry_id обязателен")
	}
	resp, err := h.service.Report(c.Request().Context(), registryID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return RespondSuccess(c, resp)
}

// Export godoc
// @Summary      Выгрузить справку о человеке
// @Description  Собирает справку файлом и пишет выдачу в журнал. Получатель и реквизиты запроса обязательны. Права page.admin.pd_subject и action.pd_subject.export.
// @Tags         pd-subject
// @Accept       json
// @Produce      application/octet-stream
// @Security     BearerAuth
// @Param        request body pdsubject.ExportRequest true "Кому и по какому запросу"
// @Success      200 {file} binary
// @Router       /pd-subject/export [post]
func (h *PDSubjectHandler) Export(c echo.Context) error {
	var req pdsubject.ExportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body: "+err.Error())
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	data, name, mime, err := h.service.Export(c.Request().Context(), req,
		pdsubject.Actor{UserID: GetUserID(c), Username: GetUsername(c)})
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	return c.Blob(http.StatusOK, mime, data)
}

// Disclosures godoc
// @Summary      Журнал выдач сведений
// @Description  Кому, когда и по какому запросу выдавали сведения. Право page.admin.pd_subject.
// @Tags         pd-subject
// @Produce      json
// @Security     BearerAuth
// @Param        registry_id query int false "Только по одному человеку"
// @Success      200 {array} models.PDDisclosure
// @Router       /pd-subject/disclosures [get]
func (h *PDSubjectHandler) Disclosures(c echo.Context) error {
	registryID, _ := strconv.Atoi(c.QueryParam("registry_id"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	entries, err := h.service.Disclosures(c.Request().Context(), registryID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return RespondSuccess(c, entries)
}
