package handlers

import (
	"net/http"
	"net/url"
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
// @Description  Поиск человека по имени или по номеру документа. Склейки по имени нет: однофамильцы существуют, решает оператор. Право page.admin.pd_subject.
// @Tags         pd-subject
// @Produce      json
// @Security     BearerAuth
// @Param        fio query string false "Фамилия Имя Отчество"
// @Param        document query string false "Номер паспорта или патента"
// @Success      200 {array} pdsubject.Candidate
// @Router       /pd-subject/candidates [get]
func (h *PDSubjectHandler) Find(c echo.Context) error {
	found, err := h.service.Find(c.Request().Context(), c.QueryParam("fio"), c.QueryParam("document"))
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
	registryID, _ := strconv.Atoi(c.QueryParam("registry_id"))
	employeeID, _ := strconv.Atoi(c.QueryParam("employee_id"))
	if registryID <= 0 && employeeID <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "укажите registry_id или employee_id")
	}
	resp, err := h.service.Report(c.Request().Context(), registryID, employeeID)
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
	// Имя справки кириллическое, а заголовки HTTP - ASCII: без filename*=UTF-8''
	// браузер сохраняет файл кракозябрами (поймано ручной проверкой на стенде).
	// Тот же приём, что у выгрузки бланков в attachment_blank.go: ASCII-запасное имя
	// для старых клиентов плюс закодированное настоящее.
	c.Response().Header().Set(echo.HeaderContentDisposition,
		`attachment; filename="pd-subject-report.xlsx"; filename*=UTF-8''`+url.PathEscape(name))
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
	employeeID, _ := strconv.Atoi(c.QueryParam("employee_id"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	entries, err := h.service.Disclosures(c.Request().Context(), registryID, employeeID, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return RespondSuccess(c, entries)
}
