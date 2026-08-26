package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"systemburo/internal/export"
	"systemburo/internal/services"

	"github.com/labstack/echo/v4"
)

// Выгрузка реестра заявок за период (#1832).
//
// Данные берутся тем же сервисным методом, что и список Центра, а не своим
// запросом. Это не экономия кода, а требование: в GetApplications уже сидят и
// скоупинг видимости (принимающий видит все, остальные - свои), и подмена ФИО у
// тех, кто не давал согласия на обработку персональных данных. Свой запрос стал бы
// способом выгрузить то, чего человек не видит на экране, и снять маску - ровно та
// дыра, которую закрывали в #1472 со стороны сквозного поиска.
//
// Обращения к выгрузке пишутся в журнал 152-ФЗ: путь добавлен в pdPaths
// (internal/middleware/pd_audit.go). Один файл уносит персональные данные пачкой,
// поэтому он там наравне со сводкой согласий и выгрузкой файлового архива.

// registryHeaders - шапка выгружаемого реестра. Порядок колонок повторяет порядок
// чтения строки в Центре: чем заявка опознаётся, потом кто и от кого, потом что с
// ней стало.
var registryHeaders = []string{
	"Номер заявки",
	"Дата подачи",
	"Организация",
	"Компания",
	"Заявитель",
	"Статус",
	"Согласование",
	"Дата согласования",
	"Принял",
	"Людей",
	"Машин",
	"Срок с",
	"Срок по",
	"Отметок чёрного списка",
	"Файлы",
}

// ExportApplications godoc
// @Summary      Выгрузка реестра заявок в .xlsx
// @Description  Отдаёт текущую выборку Центра заявок файлом. Фильтры и видимость - те же,
// @Description  что у GET /applications; ФИО без согласия на обработку ПД заменены логином.
// @Tags         applications
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Security     BearerAuth
// @Param        search_query      query string false "Поисковый запрос"
// @Param        organization_ids  query string false "ID организаций через запятую"
// @Param        company_ids       query string false "ID компаний через запятую"
// @Param        unload_place_ids  query string false "ID мест разгрузки через запятую"
// @Param        passage_table_ids query string false "ID таблиц проходной через запятую"
// @Param        confirmation      query string false "Статус согласования"
// @Param        status            query string false "Статус заявки"
// @Param        date_from         query string false "Дата от (YYYY-MM-DD)"
// @Param        date_to           query string false "Дата до (YYYY-MM-DD)"
// @Param        archive           query bool   false "Архивные заявки"
// @Success      200 {file}    file
// @Failure      401 {object}  models.HTTPError
// @Failure      403 {object}  models.HTTPError
// @Failure      500 {object}  models.HTTPError
// @Router       /applications/export [get]
func (h *ApplicationHandler) ExportApplications(c echo.Context) error {
	username := c.Get("username").(string)

	filter, err := bindApplicationListFilter(c)
	if err != nil {
		return err
	}

	apps, err := h.service.GetApplications(c.Request().Context(), username, filter)
	if err != nil {
		return err
	}

	ids := make([]int, 0, len(apps))
	for i := range apps {
		ids = append(ids, apps[i].ID)
	}
	extras, err := h.service.GetRegistryExtras(c.Request().Context(), ids)
	if err != nil {
		return err
	}

	table := export.Table{
		Title:    "Реестр заявок",
		Subtitle: registrySubtitle(filter, len(apps)),
		Headers:  registryHeaders,
		Rows:     make([][]string, 0, len(apps)),
	}
	for i := range apps {
		table.Rows = append(table.Rows, registryRow(apps[i], extras[apps[i].ID]))
	}

	blob, err := export.ToXLSX(table)
	if err != nil {
		return err
	}

	name := fmt.Sprintf("applications-%s.xlsx", time.Now().Format("2006-01-02"))
	c.Response().Header().Set(echo.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	return c.Blob(http.StatusOK, export.MIMEXLSX, blob)
}

// registryRow разворачивает заявку в строку реестра.
//
// Статус и согласование пишутся как есть, без своего словаря: на экране человек
// видит ровно эти значения, и перевод в выгрузке разошёлся бы с интерфейсом.
func registryRow(a services.ApplicationWithDetails, ex services.ApplicationRegistryExtras) []string {
	files := "нет"
	if a.HasFiles {
		files = "есть"
	}
	return []string{
		a.ApplicationNumber,
		a.SendingDatetime.Format("02.01.2006 15:04"),
		a.OrganizationName,
		a.CompanyName,
		personName(a.SenderFullName, a.SenderName),
		a.Status,
		a.Confirmation,
		formatOptionalTime(a.ConfirmationDatetime),
		personName(a.ResponsibleFullName, a.ResponsibleName),
		strconv.Itoa(ex.PeopleCount),
		strconv.Itoa(ex.CarsCount),
		formatISODate(ex.EntryDateFrom),
		formatISODate(ex.EntryDateTo),
		strconv.Itoa(a.BlacklistFlagsCount),
		files,
	}
}

// personName предпочитает полное имя короткому. Оба уже прошли маскировку в
// сервисе, поэтому здесь не решается, показывать ли имя - только какое из двух.
func personName(full *string, short string) string {
	if full != nil && strings.TrimSpace(*full) != "" {
		return *full
	}
	return short
}

// formatOptionalTime - пустая ячейка вместо нулевой даты: «01.01.0001» в отчёте
// читается как данные, хотя означает их отсутствие.
func formatOptionalTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	return t.Format("02.01.2006 15:04")
}

// formatISODate переводит хранимую строку YYYY-MM-DD в привычный вид. Значение,
// не похожее на дату, отдаётся как есть - в базе varchar, и обрезать непонятное
// молча хуже, чем показать.
func formatISODate(s string) string {
	if s == "" {
		return ""
	}
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return s
	}
	return d.Format("02.01.2006")
}

// registrySubtitle - строка под заголовком: за какой период выгрузка и сколько в
// ней заявок. Период показывается только когда он задан фильтром, иначе честнее
// не писать ничего, чем подставлять «с начала времён».
func registrySubtitle(filter services.ApplicationFilter, count int) string {
	parts := make([]string, 0, 3)
	switch {
	case filter.DateFrom != nil && *filter.DateFrom != "" && filter.DateTo != nil && *filter.DateTo != "":
		parts = append(parts, fmt.Sprintf("период: %s - %s",
			formatISODate(*filter.DateFrom), formatISODate(*filter.DateTo)))
	case filter.DateFrom != nil && *filter.DateFrom != "":
		parts = append(parts, "период: с "+formatISODate(*filter.DateFrom))
	case filter.DateTo != nil && *filter.DateTo != "":
		parts = append(parts, "период: по "+formatISODate(*filter.DateTo))
	}
	if filter.Archive != nil && *filter.Archive {
		parts = append(parts, "архив")
	}
	parts = append(parts, fmt.Sprintf("заявок: %d", count))
	return strings.Join(parts, ", ")
}
