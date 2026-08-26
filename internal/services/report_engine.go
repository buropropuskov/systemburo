package services

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"systemburo/internal/models"
)

// Движок исполнения агрегатных отчётов (B2). Метрики, разрезы и поля фильтров —
// строго whitelist (схемы ниже): все SQL-фрагменты статические константы кода,
// а пользовательский ввод сверяется по ключам карт и передаётся только через
// плейсхолдеры (?). Конкатенации ввода в SQL нет. Допустимые разрезы метрики
// дублируют каталог B1 (reportMetricRegistry.dimensions) — тест ловит расхождение.

// ErrInvalidReportRequest — запрос отчёта не прошёл валидацию (неизвестная
// метрика/разрез/фильтр и т.п.). Handler маппит в 400 без эха ввода.
var ErrInvalidReportRequest = errors.New("invalid report request")

func errInvalidReport(field string) error {
	return fmt.Errorf("%w: %s", ErrInvalidReportRequest, field)
}

// joinKind — какой блок join'ов требуется разрезу/фильтру.
type joinKind int

const (
	jNone   joinKind = iota // достаточно базовой таблицы
	jChain                  // основной 1:1 join-блок метрики (org/company/...)
	jAttach                 // отдельный join к вложениям (fan-out; только applications_count)
)

const (
	sortValueDesc = "value_desc"
	sortValueAsc  = "value_asc"
	sortLabelAsc  = "label_asc"
	sortLabelDesc = "label_desc"
)

const (
	defaultReportLimit = 100
	maxReportLimit     = 1000
)

// aggColumn — SQL-выражение разреза/фильтра и блок join'ов, нужный для доступа к нему.
type aggColumn struct {
	expr string
	join joinKind
}

// aggMetricSchema — план сборки агрегатного запроса для одной метрики.
// joinBlock — 1:1 LEFT JOIN'ы (не размножают строки, добавляются по требованию);
// attachJoin — fan-out join к вложениям, поэтому applications_count считает
// COUNT(DISTINCT app.id). period/hour_of_day строятся в коде по tsColumn.
// valueType — тип значения для форматирования на фронте (пусто — число,
// "duration" — секунды); едет в колонку ответа через план.
type aggMetricSchema struct {
	base       string
	aggExpr    string
	baseWhere  string
	tsColumn   string
	tsJoin     joinKind
	unit       string
	valueType  models.ReportValueType
	joinBlock  []string
	attachJoin []string
	dims       map[string]aggColumn
	filters    map[string]aggColumn
}

var aggMetricRegistry = map[string]aggMetricSchema{
	"applications_count": {
		base:     "applications app",
		aggExpr:  "COUNT(DISTINCT app.id)",
		tsColumn: "app.sending_datetime",
		tsJoin:   jNone,
		unit:     "шт",
		joinBlock: []string{
			"LEFT JOIN organizations org ON org.id = app.organization_id",
			"LEFT JOIN companies comp ON comp.id = app.company_id",
		},
		attachJoin: []string{
			"JOIN attachments att ON att.application_id = app.id",
			"LEFT JOIN unique_attachments ua ON ua.id = att.unique_attachment_id",
		},
		dims: map[string]aggColumn{
			"status":          {expr: "app.status", join: jNone},
			"organization":    {expr: "COALESCE(org.name, '(без организации)')", join: jChain},
			"company":         {expr: "COALESCE(comp.name, '(без компании)')", join: jChain},
			"attachment_type": {expr: "COALESCE(ua.display_name, att.attachment_display_name, att.attachment_type)", join: jAttach},
		},
		filters: map[string]aggColumn{
			"status":          {expr: "app.status", join: jNone},
			"organization":    {expr: "org.name", join: jChain},
			"company":         {expr: "comp.name", join: jChain},
			"attachment_type": {expr: "COALESCE(ua.display_name, att.attachment_display_name, att.attachment_type)", join: jAttach},
		},
	},
	// base — подзапрос-источник carsHistoryUnion (audit_log[car], #870 F.5 read-switch):
	// подставляется как FROM (...) ch, поэтому baseWhere/tsColumn/joinBlock читают
	// ch.* как и раньше. До-cutover въезды cars_history перенесены в audit_log
	// backfill'ом, поэтому отчёт видит и исторические, и новые события.
	"car_entries_count": {
		base:      carsHistoryUnion + " ch",
		aggExpr:   "COUNT(*)",
		baseWhere: "ch.action_type = 'entry'",
		tsColumn:  "ch.created_at",
		tsJoin:    jNone,
		unit:      "шт",
		joinBlock: []string{
			"JOIN cars c ON c.id = ch.car_id",
			"LEFT JOIN attachments att ON att.id = c.attachment_id",
			"LEFT JOIN applications app ON app.id = att.application_id",
			"LEFT JOIN organizations org ON org.id = app.organization_id",
			"LEFT JOIN companies comp ON comp.id = app.company_id",
			"LEFT JOIN unique_attachments ua ON ua.id = att.unique_attachment_id",
		},
		dims: map[string]aggColumn{
			"unload_place": {expr: "COALESCE(c.unload_place, '(не указано)')", join: jChain},
			"organization": {expr: "COALESCE(org.name, comp.name, '(без организации)')", join: jChain},
		},
		filters: map[string]aggColumn{
			"organization":    {expr: "org.name", join: jChain},
			"company":         {expr: "comp.name", join: jChain},
			"status":          {expr: "app.status", join: jChain},
			"attachment_type": {expr: "COALESCE(ua.display_name, att.attachment_display_name, att.attachment_type)", join: jChain},
			"unload_place":    {expr: "c.unload_place", join: jChain},
		},
	},
	// base — подзапрос-источник employeesHistoryUnion (audit_log[employee], #870 F.6
	// read-switch): подставляется как FROM (...) eh, поэтому baseWhere/tsColumn/joinBlock
	// читают eh.* как и раньше. До-cutover въезды employees_history перенесены в
	// audit_log backfill'ом, поэтому отчёт видит и исторические, и новые события.
	"people_entries_count": {
		base:      employeesHistoryUnion + " eh",
		aggExpr:   "COUNT(*)",
		baseWhere: "eh.action_type = 'entry'",
		tsColumn:  "eh.created_at",
		tsJoin:    jNone,
		unit:      "шт",
		joinBlock: []string{
			"JOIN employees e ON e.id = eh.employee_id",
			"LEFT JOIN citizenships cz ON cz.id = e.citizenship_id",
			"LEFT JOIN attachments att ON att.id = e.attachment_id",
			"LEFT JOIN applications app ON app.id = att.application_id",
			"LEFT JOIN organizations org ON org.id = app.organization_id",
			"LEFT JOIN companies comp ON comp.id = app.company_id",
			"LEFT JOIN unique_attachments ua ON ua.id = att.unique_attachment_id",
		},
		dims: map[string]aggColumn{
			"organization": {expr: "COALESCE(org.name, comp.name, '(без организации)')", join: jChain},
		},
		filters: map[string]aggColumn{
			"organization":    {expr: "org.name", join: jChain},
			"company":         {expr: "comp.name", join: jChain},
			"status":          {expr: "app.status", join: jChain},
			"attachment_type": {expr: "COALESCE(ua.display_name, att.attachment_display_name, att.attachment_type)", join: jChain},
			"citizenship":     {expr: "cz.name", join: jChain},
		},
	},
	// avg_cars_per_day считает COUNT(entry) по бинам так же, как car_entries_count;
	// деление на число календарных дней бина выполняет RunReport (postprocess),
	// поэтому aggExpr тут — счётчик, а не среднее. Доступные разрезы: period/none.
	"avg_cars_per_day": {
		base:      carsHistoryUnion + " ch",
		aggExpr:   "COUNT(*)",
		baseWhere: "ch.action_type = 'entry'",
		tsColumn:  "ch.created_at",
		tsJoin:    jNone,
		unit:      "шт/день",
		joinBlock: []string{
			"JOIN cars c ON c.id = ch.car_id",
			"LEFT JOIN attachments att ON att.id = c.attachment_id",
			"LEFT JOIN applications app ON app.id = att.application_id",
			"LEFT JOIN organizations org ON org.id = app.organization_id",
			"LEFT JOIN companies comp ON comp.id = app.company_id",
			"LEFT JOIN unique_attachments ua ON ua.id = att.unique_attachment_id",
		},
		dims: map[string]aggColumn{},
		filters: map[string]aggColumn{
			"organization":    {expr: "org.name", join: jChain},
			"company":         {expr: "comp.name", join: jChain},
			"status":          {expr: "app.status", join: jChain},
			"attachment_type": {expr: "COALESCE(ua.display_name, att.attachment_display_name, att.attachment_type)", join: jChain},
			"unload_place":    {expr: "c.unload_place", join: jChain},
		},
	},
	"items_sum": {
		base:     "items",
		aggExpr:  "COALESCE(SUM(items.count), 0)",
		tsColumn: "app.sending_datetime",
		tsJoin:   jChain,
		unit:     "шт",
		joinBlock: []string{
			"JOIN attachments att ON att.id = items.attachment_id",
			"LEFT JOIN applications app ON app.id = att.application_id",
			"LEFT JOIN organizations org ON org.id = app.organization_id",
			"LEFT JOIN companies comp ON comp.id = app.company_id",
		},
		dims: map[string]aggColumn{
			"organization": {expr: "COALESCE(org.name, '(без организации)')", join: jChain},
			"company":      {expr: "COALESCE(comp.name, '(без компании)')", join: jChain},
		},
		filters: map[string]aggColumn{
			"organization": {expr: "org.name", join: jChain},
			"company":      {expr: "comp.name", join: jChain},
		},
	},
}

var aggGranularity = map[string]string{
	"day":   "day",
	"week":  "week",
	"month": "month",
}

// whereClause — WHERE-фрагмент плана: выражение с плейсхолдерами и его аргументы.
type whereClause struct {
	expr string
	args []any
}

// aggPlan — резолвленный план агрегатного запроса (только whitelist-выражения).
type aggPlan struct {
	table     string
	selectStr string
	joins     []string
	wheres    []whereClause
	groupExpr string
	orderStr  string
	limit     int
	unit      string
	valueType models.ReportValueType
}

// buildAggregatePlan собирает план агрегатного отчёта из whitelist-схем.
// Чистая функция (без БД) — тестируется напрямую. Любой неизвестный/недоступный
// для метрики разрез или фильтр -> ErrInvalidReportRequest.
func buildAggregatePlan(req models.ReportRequest) (*aggPlan, error) {
	schema, ok := aggMetricRegistry[req.Metric]
	if !ok {
		return nil, errInvalidReport("metric")
	}
	if req.Dimension == "" {
		return nil, errInvalidReport("dimension")
	}
	// "none" (без разреза) универсален для любой метрики — не входит в каталожный
	// список разрезов метрики, проверяется отдельно. Остальные разрезы должны быть
	// заявлены в каталоге B1 для этой метрики (единый источник правды для UI и движка).
	if req.Dimension != dimNone && !contains(reportMetricRegistry[req.Metric].dimensions, req.Dimension) {
		return nil, errInvalidReport("dimension")
	}

	groupExpr, labelExpr, dimJoin, err := resolveAggDimension(schema, req.Dimension, req.Granularity)
	if err != nil {
		return nil, err
	}

	var needChain, needAttach bool
	addJoinNeed(dimJoin, &needChain, &needAttach)

	plan := &aggPlan{
		table:     schema.base,
		selectStr: labelExpr + " AS label, " + schema.aggExpr + " AS value",
		groupExpr: groupExpr,
		unit:      schema.unit,
		valueType: schema.valueType,
	}
	if schema.baseWhere != "" {
		plan.wheres = append(plan.wheres, whereClause{expr: schema.baseWhere})
	}

	for _, f := range req.Filters {
		wc, fjoin, ferr := resolveAggFilter(schema, f)
		if ferr != nil {
			return nil, ferr
		}
		if wc == nil { // пустой фильтр (нет значений/границ) — пропускаем
			continue
		}
		plan.wheres = append(plan.wheres, *wc)
		addJoinNeed(fjoin, &needChain, &needAttach)
	}

	if needChain {
		plan.joins = append(plan.joins, schema.joinBlock...)
	}
	if needAttach {
		plan.joins = append(plan.joins, schema.attachJoin...)
	}

	// Без разреза — один итоговый ряд, сортировка не нужна (groupExpr пуст).
	if groupExpr != "" {
		plan.orderStr = resolveAggOrder(req.Sort, req.Dimension, groupExpr, schema.aggExpr)
	}
	plan.limit = clampLimit(req.Limit)
	return plan, nil
}

func addJoinNeed(jk joinKind, needChain, needAttach *bool) {
	switch jk {
	case jChain:
		*needChain = true
	case jAttach:
		*needAttach = true
	}
}

// resolveAggDimension возвращает GROUP BY-выражение, выражение подписи и нужный
// join. period/hour_of_day строятся по tsColumn метрики; остальные — из карты dims.
func resolveAggDimension(s aggMetricSchema, dim, granularity string) (group, label string, jk joinKind, err error) {
	switch dim {
	case dimNone:
		// Без GROUP BY: единственный ряд с константной подписью.
		return "", "'Итого'", jNone, nil
	case "period":
		unit := "day"
		if granularity != "" {
			u, ok := aggGranularity[granularity]
			if !ok {
				return "", "", jNone, errInvalidReport("granularity")
			}
			unit = u
		}
		group = fmt.Sprintf("date_trunc('%s', %s)", unit, tzColumn(s.tsColumn))
		label = fmt.Sprintf("to_char(%s, 'YYYY-MM-DD')", group)
		return group, label, s.tsJoin, nil
	case "hour_of_day":
		group = fmt.Sprintf("(EXTRACT(HOUR FROM %s))::int", tzColumn(s.tsColumn))
		label = group + "::text"
		return group, label, s.tsJoin, nil
	default:
		col, ok := s.dims[dim]
		if !ok {
			return "", "", jNone, errInvalidReport("dimension")
		}
		return col.expr, col.expr, col.join, nil
	}
}

// resolveAggFilter превращает поле фильтра в WHERE-фрагмент. Возвращает nil без
// ошибки, если фильтр пустой (нет значений/границ) — такой фильтр не применяется.
func resolveAggFilter(s aggMetricSchema, f models.ReportFilterValue) (*whereClause, joinKind, error) {
	if f.Key == "date_range" {
		var parts []string
		var args []any
		if t, ok := parseReportDate(f.From, false); ok {
			parts = append(parts, s.tsColumn+" >= ?")
			args = append(args, t)
		}
		if t, ok := parseReportDate(f.To, true); ok {
			parts = append(parts, s.tsColumn+" <= ?")
			args = append(args, t)
		}
		if len(parts) == 0 {
			return nil, jNone, nil
		}
		return &whereClause{expr: strings.Join(parts, " AND "), args: args}, s.tsJoin, nil
	}

	col, ok := s.filters[f.Key]
	if !ok {
		return nil, jNone, errInvalidReport("filter")
	}
	vals := nonEmpty(f.Values)
	if len(vals) == 0 {
		return nil, jNone, nil
	}
	return &whereClause{expr: col.expr + " IN ?", args: []any{vals}}, col.join, nil
}

// resolveAggOrder выбирает ORDER BY из whitelist. По умолчанию временные разрезы
// (period/hour_of_day) сортируются хронологически, категориальные — по убыванию
// значения. Неизвестный sort трактуется как дефолт (значения только из констант,
// ввод не подставляется).
func resolveAggOrder(sort, dim, groupExpr, aggExpr string) string {
	switch sort {
	case sortValueDesc:
		return aggExpr + " DESC"
	case sortValueAsc:
		return aggExpr + " ASC"
	case sortLabelAsc:
		return groupExpr + " ASC"
	case sortLabelDesc:
		return groupExpr + " DESC"
	}
	if dim == "period" || dim == "hour_of_day" {
		return groupExpr + " ASC"
	}
	return aggExpr + " DESC"
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultReportLimit
	}
	if limit > maxReportLimit {
		return maxReportLimit
	}
	return limit
}

// parseReportDate парсит YYYY-MM-DD как границу московских суток (бакетинг тоже в
// МСК). endOfDay=true -> 23:59:59 МСК (для верхней границы). Инстант сравнивается
// с timestamptz-колонками корректно вне зависимости от их хранения в UTC.
func parseReportDate(s string, endOfDay bool) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02", s, analyticsLocation)
	if err != nil {
		return time.Time{}, false
	}
	if endOfDay {
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, analyticsLocation), true
	}
	return t, true
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// resolveReportMetrics нормализует список метрик запроса: Metrics приоритетнее
// одиночного Metric (обратная совместимость), пустые и дубликаты убираются с
// сохранением порядка. Пустой результат -> ErrInvalidReportRequest. Существование
// каждой метрики проверяет buildAggregatePlan при сборке её плана.
func resolveReportMetrics(req models.ReportRequest) ([]string, error) {
	raw := req.Metrics
	if len(raw) == 0 && req.Metric != "" {
		raw = []string{req.Metric}
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, m := range raw {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	if len(out) == 0 {
		return nil, errInvalidReport("metric")
	}
	return out, nil
}

// metricIsDerived — значение метрики это производная статистика (среднее,
// перцентиль, доля), а не счётчик событий. Отсюда два следствия, каждое со своим
// вызовом ниже: пустой бин у такой метрики не ноль, а «нет данных», и итог по
// окну не получить сложением бинов.
func metricIsDerived(metric string) bool {
	return aggMetricRegistry[metric].valueType == models.ReportValueDuration || rateMetrics[metric]
}

// metricOmitsFakeZero — метрике нельзя дорисовывать 0 в бины, где у неё нет строк.
// Для счётчиков 0 честен («в этот день не было въездов»), а для производной метрики
// неотличим от реального значения: у длительности — от «прошло мгновенно» (в
// мультиметрике этапы имеют разное покрытие: заявку согласовали, но ещё не
// завершили), у доли — от «отказов не было», хотя заявок в бине не было вовсе
// (метрики других баз бьют бины по своим датам). Отсутствие ключа = «нет данных»
// (фронт рисует прочерк).
func metricOmitsFakeZero(metric string) bool {
	return metricIsDerived(metric)
}

// metricTotalNotAdditive — итог метрики по окну НЕЛЬЗЯ получить сложением значений
// бинов: сумма средних не среднее, перцентили не складываются вовсе, а пять дней
// по 20% отказов это не 100% отказов. Такие итоги пересчитываются по всей выборке
// отдельным запросом (execWindowTotal).
func metricTotalNotAdditive(metric string) bool {
	return metricIsDerived(metric)
}

// mergeMetricRows сливает построчные результаты метрик в мультиметричные строки
// (подпись разреза -> значение каждой метрики), упорядочивает их и применяет лимит.
// Отсутствующая в бине метрика -> 0 для счётчиков и пропуск ключа для длительностей
// (metricOmitsFakeZero). Итоги (totals) считаются по уже усечённым строкам — как
// сумма видимых значений каждой метрики. Чистая функция.
func mergeMetricRows(metrics []string, perMetric map[string][]models.ReportAggregateRow, dim, sortKey string, limit int) ([]models.ReportMetricRow, map[string]int64) {
	order := make([]string, 0)
	bucket := make(map[string]map[string]int64)
	for _, m := range metrics {
		for _, r := range perMetric[m] {
			vals, ok := bucket[r.Label]
			if !ok {
				vals = make(map[string]int64, len(metrics))
				bucket[r.Label] = vals
				order = append(order, r.Label)
			}
			vals[m] = r.Value
		}
	}

	rows := make([]models.ReportMetricRow, 0, len(order))
	for _, label := range order {
		vals := make(map[string]int64, len(metrics))
		for _, m := range metrics {
			v, ok := bucket[label][m]
			if !ok && metricOmitsFakeZero(m) {
				continue // нет данных != 0, ключ не выставляем
			}
			vals[m] = v // отсутствие счётчика -> 0 (в бине не было событий)
		}
		rows = append(rows, models.ReportMetricRow{Label: label, Values: vals})
	}

	orderMetricRows(rows, dim, sortKey, metrics)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}

	totals := make(map[string]int64, len(metrics))
	for _, r := range rows {
		for _, m := range metrics {
			totals[m] += r.Values[m]
		}
	}
	return rows, totals
}

// orderMetricRows упорядочивает мультиметричные строки in-place. Явный sort имеет
// приоритет: value_* сортирует по СУММЕ метрик строки (единый порядок для всех
// колонок; сортировать по одной метрике при мультивыборе неоднозначно — какой),
// label_* — по подписи разреза. Без явного sort: период — хронологически (ISO-даты),
// час суток — численно, прочие категориальные — по убыванию суммы метрик. Для
// одиночной метрики сумма = её значение, т.е. порядок совпадает с SQL-сортировкой.
// Разрез "none" даёт один ряд (порядок не важен).
func orderMetricRows(rows []models.ReportMetricRow, dim, sortKey string, metrics []string) {
	rowTotal := func(r models.ReportMetricRow) int64 {
		var t int64
		for _, m := range metrics {
			t += r.Values[m]
		}
		return t
	}
	switch sortKey {
	case sortLabelAsc:
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Label < rows[j].Label })
		return
	case sortLabelDesc:
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Label > rows[j].Label })
		return
	case sortValueAsc:
		sort.SliceStable(rows, func(i, j int) bool { return rowTotal(rows[i]) < rowTotal(rows[j]) })
		return
	case sortValueDesc:
		sort.SliceStable(rows, func(i, j int) bool { return rowTotal(rows[i]) > rowTotal(rows[j]) })
		return
	}
	switch dim {
	case "period":
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].Label < rows[j].Label })
	case "hour_of_day":
		sort.SliceStable(rows, func(i, j int) bool { return hourLabel(rows[i].Label) < hourLabel(rows[j].Label) })
	case dimNone:
		// один итоговый ряд — сортировка не нужна
	default:
		sort.SliceStable(rows, func(i, j int) bool { return rowTotal(rows[i]) > rowTotal(rows[j]) })
	}
}

// hourLabel парсит подпись часа суток ("0".."23") в число для численной сортировки.
func hourLabel(label string) int {
	n, err := strconv.Atoi(strings.TrimSpace(label))
	if err != nil {
		return 0
	}
	return n
}
