package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"systemburo/internal/export"
	"systemburo/internal/logmask"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
)

// Выгрузка журнала обращений в .xlsx (#2125).
//
// Раньше метод отдавал сплошной текст с рамкой из знаков «=»: открыть его
// таблицей нельзя, отобрать строки нельзя, а обещание документации -- «выгрузка в
// электронную таблицу». Файл собирается тем же пакетом export, что реестр заявок,
// и тем же порядком строк, что показан на экране.
//
// Адреса прогоняются через ту же маску, что и запись журнала: записи, сделанные до
// перехода на белый список, лежат в базе с открытыми поисковыми строками (ФИО,
// номера заявок), и файл унёс бы их наружу.

// requestLogsHeaders -- шапка выгрузки. Порядок повторяет чтение строки на экране:
// когда, что запрашивали, чем ответили, сколько это заняло и кто это был.
var requestLogsHeaders = []string{
	"Дата и время",
	"Метод",
	"Адрес",
	"Код ответа",
	"Длительность, мс",
	"Пользователь",
}

// Export godoc
// @Summary      Выгрузка журнала обращений в .xlsx
// @Description  Отдаёт текущую выборку журнала файлом: те же фильтры и тот же порядок,
// @Description  что на экране. Значения параметров адреса, кроме служебных, затёрты.
// @Description  Число выгруженных и отсечённых записей -- в заголовках X-Export-*.
// @Tags         request-logs
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Security     BearerAuth
// @Param        user_id   query int    false "ID пользователя"
// @Param        method    query string false "HTTP метод"
// @Param        status    query int    false "HTTP статус"
// @Param        status_min query int   false "Нижняя граница кода ответа (400 -- только ошибки)"
// @Param        status_max query int   false "Верхняя граница кода ответа"
// @Param        min_duration_ms query int false "Только ответы дольше указанного времени, мс"
// @Param        from_date query string false "Дата начала (ISO 8601 или YYYY-MM-DD)"
// @Param        to_date   query string false "Дата окончания (ISO 8601 или YYYY-MM-DD)"
// @Param        search    query string false "Поиск по URL и username"
// @Param        sort      query string false "Поле сортировки" Enums(created_at, method, url, status, username, duration) default(created_at)
// @Param        order     query string false "Направление сортировки" Enums(asc, desc) default(desc)
// @Success      200 {file}   file
// @Failure      401 {object} models.HTTPError
// @Failure      403 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /request-logs/export [get]
func (h *RequestLogsHandler) Export(c echo.Context) error {
	var q models.RequestLogsQuery
	if err := c.Bind(&q); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid query parameters")
	}

	res, err := h.service.Export(c.Request().Context(), q)
	if err != nil {
		return err
	}

	table := export.Table{
		Title:    "Журнал обращений",
		Subtitle: requestLogsSubtitle(res),
		Headers:  requestLogsHeaders,
		Rows:     make([][]string, 0, len(res.Rows)),
	}
	for i := range res.Rows {
		table.Rows = append(table.Rows, requestLogRow(res.Rows[i]))
	}

	blob, err := export.ToXLSX(table)
	if err != nil {
		return err
	}

	h.recordExport(c, q, res)

	name := fmt.Sprintf("request-logs-%s.xlsx", time.Now().Format("2006-01-02"))
	head := c.Response().Header()
	head.Set(echo.HeaderContentDisposition, `attachment; filename="`+name+`"`)
	// Числа охвата заголовками, а не в теле: тело -- поток байтов файла. Экран
	// читает их, чтобы сказать, что выгрузка обрезана, и предложить сузить период.
	head.Set("X-Export-Rows", strconv.Itoa(len(res.Rows)))
	head.Set("X-Export-Total", strconv.FormatInt(res.Total, 10))
	head.Set("X-Export-Truncated", strconv.FormatBool(res.Truncated))
	head.Set(echo.HeaderAccessControlExposeHeaders, "Content-Disposition, X-Export-Rows, X-Export-Total, X-Export-Truncated")
	return c.Blob(http.StatusOK, export.MIMEXLSX, blob)
}

// requestLogRow разворачивает запись журнала в строку выгрузки.
//
// Длительность берётся из микросекундной колонки (#2125): треть записей отвечает
// быстрее миллисекунды, и целые миллисекунды показывали по ним ноль. У записей,
// сделанных до перехода, микросекунд нет -- тогда идут миллисекунды как есть.
func requestLogRow(l models.RequestLogs) []string {
	duration := ""
	switch {
	case l.DurationUs != nil:
		duration = strconv.FormatFloat(float64(*l.DurationUs)/1000, 'f', 3, 64)
	case l.DurationMs != nil:
		duration = strconv.Itoa(*l.DurationMs)
	}

	status := ""
	if l.ResponseStatus != nil {
		status = strconv.Itoa(*l.ResponseStatus)
	}

	return []string{
		l.CreatedAt.Format("02.01.2006 15:04:05"),
		optionalString(l.Method),
		logmask.RawURL(optionalString(l.URL)),
		status,
		duration,
		optionalString(l.Username),
	}
}

// optionalString -- пустая ячейка вместо «<nil>»: в журнале необязательны и метод,
// и адрес (обращение могло не дойти до маршрутизации), и имя (запрос без входа).
func optionalString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// requestLogsSubtitle -- строка под заголовком файла: сколько записей в нём и
// сколько подходило под отбор. Обрезка проговаривается словами: файл уходит из
// системы и живёт своей жизнью, а «10 000 строк» без оговорки читаются как весь
// период.
func requestLogsSubtitle(res models.RequestLogsExport) string {
	if !res.Truncated {
		return fmt.Sprintf("записей: %d", len(res.Rows))
	}
	return fmt.Sprintf("записей: %d из %d, показаны первые %d в выбранном порядке -- сузьте период или отбор",
		len(res.Rows), res.Total, res.Limit)
}

// recordExport пишет в audit_log факт выгрузки журнала: кто, когда, сколько строк
// унёс и каким отбором. Файл уносит адреса обращений сотен пользователей разом --
// это то же снятие данных пачкой, что выгрузка реестра заявок, и оно обязано
// оставлять след.
//
// Значение поиска в след НЕ идёт, только признак: строка поиска -- то самое место,
// где в журнале оседали ФИО и номера заявок, и переписывать их в аудит значило бы
// вынести их из журнала во второе хранилище.
//
// Log, а не Record: сорвавшаяся запись аудита не должна отменять уже собранный файл.
func (h *RequestLogsHandler) recordExport(c echo.Context, q models.RequestLogsQuery, res models.RequestLogsExport) {
	if h.recorder == nil {
		return
	}

	var actorID *int
	if uid, ok := c.Get("user_id").(int); ok && uid > 0 {
		actorID = &uid
	}

	details := requestLogsExportDetails{
		Rows:      len(res.Rows),
		Total:     res.Total,
		Truncated: res.Truncated,
		From:      q.From,
		To:        q.To,
		Method:    q.Method,
		Status:    q.Status,
		StatusMin: q.StatusMin,
		StatusMax: q.StatusMax,
		UserID:    q.UserID,
		Searched:  q.Search != "",
		Sort:      q.Sort,
		Order:     q.Order,
	}
	h.recorder.Log(c.Request().Context(), nil, models.AuditEntityRequestLogExport, nil,
		models.RequestLogExportActionExported, actorID, details)
}

// requestLogsExportDetails -- подробности следа выгрузки.
type requestLogsExportDetails struct {
	Rows      int    `json:"rows"`
	Total     int64  `json:"total"`
	Truncated bool   `json:"truncated"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	Method    string `json:"method,omitempty"`
	Status    *int   `json:"status,omitempty"`
	StatusMin *int   `json:"status_min,omitempty"`
	StatusMax *int   `json:"status_max,omitempty"`
	UserID    *int   `json:"user_id,omitempty"`
	Searched  bool   `json:"searched"`
	Sort      string `json:"sort,omitempty"`
	Order     string `json:"order,omitempty"`
}
