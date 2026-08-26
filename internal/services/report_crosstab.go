package services

import (
	"fmt"
	"sort"
	"time"

	"systemburo/internal/models"
)

// Cross-tab (G4): при разрезе period значения оси pivot (тип вложения)
// разворачиваются в отдельные колонки-счётчики рядом с обычными метриками.
// Pivot-колонки динамические (зависят от данных). Реализовано отдельной веткой,
// не трогая mergeMetricRows: pivot активируется только при заданном req.Pivot и
// Dimension="period"; иначе путь движка не меняется (обратная совместимость).

// pivotColumnPrefix — префикс ключа pivot-колонки в Values/Columns. Отделяет
// cross-tab колонки от ключей метрик (метрики берут ключи из реестра).
const pivotColumnPrefix = "pivot:"

// pivotPlan — план cross-tab запроса: period-бин + значение оси pivot -> счётчик.
// SQL-выражения — только whitelist (period по tsColumn метрики, pivotExpr из схемы).
type pivotPlan struct {
	table      string
	periodExpr string // GROUP BY-выражение периода-бина
	labelExpr  string // подпись бина (совпадает с label обычных period-строк)
	pivotExpr  string // GROUP BY-выражение оси pivot (значение -> колонка)
	aggExpr    string
	joins      []string
	wheres     []whereClause
}

// pivotCell — одна ячейка cross-tab: период-бин, значение оси, счётчик.
type pivotCell struct {
	Period string `json:"period"`
	Pivot  string `json:"pivot"`
	Count  int64  `json:"count"`
}

// resolvePivot проверяет ось pivot против whitelist и применимость к метрикам
// запроса. Возвращает (axis, true) только когда pivot задан, ось известна и
// применима ко ВСЕМ метрикам запроса при Dimension="period". Иначе (false) —
// обычный путь без cross-tab.
func resolvePivot(pivot, dimension string, metrics []string) (models.ReportPivotInfo, bool, error) {
	if pivot == "" {
		return models.ReportPivotInfo{}, false, nil
	}
	if dimension != "period" {
		return models.ReportPivotInfo{}, false, errInvalidReport("pivot")
	}
	var axis models.ReportPivotInfo
	found := false
	for _, p := range reportPivotRegistry {
		if p.Key == pivot {
			axis = p
			found = true
			break
		}
	}
	if !found {
		return models.ReportPivotInfo{}, false, errInvalidReport("pivot")
	}
	for _, m := range metrics {
		if !contains(axis.Metrics, m) {
			return models.ReportPivotInfo{}, false, errInvalidReport("pivot")
		}
	}
	return axis, true, nil
}

// buildPivotPlan собирает план cross-tab запроса из whitelist-схемы метрики и оси.
// Чистая функция (без БД) — тестируется напрямую. period строится по tsColumn
// метрики (как обычный разрез period), pivot — по выражению из схемы метрики dims.
// Фильтры запроса применяются тем же резолвером, что и в обычном плане.
func buildPivotPlan(metric, pivotKey, granularity string, filters []models.ReportFilterValue) (*pivotPlan, error) {
	schema, ok := aggMetricRegistry[metric]
	if !ok {
		return nil, errInvalidReport("metric")
	}
	pivotCol, ok := schema.dims[pivotKey]
	if !ok {
		return nil, errInvalidReport("pivot")
	}

	unit := "day"
	if granularity != "" {
		u, gok := aggGranularity[granularity]
		if !gok {
			return nil, errInvalidReport("granularity")
		}
		unit = u
	}
	periodExpr := fmt.Sprintf("date_trunc('%s', %s)", unit, schema.tsColumn)
	labelExpr := fmt.Sprintf("to_char(%s, 'YYYY-MM-DD')", periodExpr)

	plan := &pivotPlan{
		table:      schema.base,
		periodExpr: periodExpr,
		labelExpr:  labelExpr,
		pivotExpr:  pivotCol.expr,
		aggExpr:    schema.aggExpr,
	}
	if schema.baseWhere != "" {
		plan.wheres = append(plan.wheres, whereClause{expr: schema.baseWhere})
	}

	var needChain, needAttach bool
	addJoinNeed(schema.tsJoin, &needChain, &needAttach)
	addJoinNeed(pivotCol.join, &needChain, &needAttach)

	for _, f := range filters {
		wc, fjoin, ferr := resolveAggFilter(schema, f)
		if ferr != nil {
			return nil, ferr
		}
		if wc == nil {
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
	return plan, nil
}

// applyPivotCells вписывает cross-tab ячейки в уже собранные построчные данные и
// возвращает добавочные pivot-колонки + их итоги (ключ колонки -> сумма по видимым
// строкам, для строки «Итого»). Чистая функция. Колонки строятся по всем встреченным
// значениям оси (детерминированный порядок — по убыванию суммы, затем по имени), их
// ключи — pivotColumnPrefix+значение. Бины, которых нет среди rows (отфильтрованы
// лимитом), игнорируются — cross-tab не добавляет новых строк, и в итоги не входят.
func applyPivotCells(rows []models.ReportMetricRow, cells []pivotCell, axisLabel string) ([]models.ReportMetricColumn, map[string]int64) {
	rowByLabel := make(map[string]int, len(rows))
	for i, r := range rows {
		rowByLabel[r.Label] = i
	}

	pivotTotals := make(map[string]int64)
	for _, c := range cells {
		ri, ok := rowByLabel[c.Period]
		if !ok {
			continue // бин вне видимых строк (лимит) — не разворачиваем
		}
		key := pivotColumnPrefix + c.Pivot
		if rows[ri].Values == nil {
			rows[ri].Values = map[string]int64{}
		}
		rows[ri].Values[key] += c.Count
		pivotTotals[c.Pivot] += c.Count
	}

	pivots := make([]string, 0, len(pivotTotals))
	for p := range pivotTotals {
		pivots = append(pivots, p)
	}
	sort.SliceStable(pivots, func(i, j int) bool {
		if pivotTotals[pivots[i]] != pivotTotals[pivots[j]] {
			return pivotTotals[pivots[i]] > pivotTotals[pivots[j]]
		}
		return pivots[i] < pivots[j]
	})

	cols := make([]models.ReportMetricColumn, 0, len(pivots))
	colTotals := make(map[string]int64, len(pivots))
	for _, p := range pivots {
		key := pivotColumnPrefix + p
		cols = append(cols, models.ReportMetricColumn{
			Key:   key,
			Label: axisLabel + ": " + p,
			Unit:  "шт",
			Kind:  models.ReportColumnPivot,
		})
		colTotals[key] = pivotTotals[p]
	}
	// Строки без значения по колонке должны давать 0 (явный нуль для FE-таблицы).
	for ri := range rows {
		if rows[ri].Values == nil {
			rows[ri].Values = map[string]int64{}
		}
		for _, p := range pivots {
			key := pivotColumnPrefix + p
			if _, ok := rows[ri].Values[key]; !ok {
				rows[ri].Values[key] = 0
			}
		}
	}
	return cols, colTotals
}

// avgMetrics — метрики-средние: значение бина = счётчик / число календарных дней
// бина. Обрабатываются постпроцессом RunReport (значение дробное -> FloatValues).
var avgMetrics = map[string]bool{
	"avg_cars_per_day": true,
}

// reportDateBounds извлекает границы периода [from, to] из date_range-фильтра запроса
// (в МСК, как и бакетинг — parseReportDate). Нужны для деления крайних неполных
// бинов на фактическое число дней пересечения с периодом.
func reportDateBounds(filters []models.ReportFilterValue) (from, to time.Time, hasFrom, hasTo bool) {
	for _, f := range filters {
		if f.Key != "date_range" {
			continue
		}
		if t, ok := parseReportDate(f.From, false); ok {
			from, hasFrom = t, true
		}
		if t, ok := parseReportDate(f.To, true); ok {
			to, hasTo = t, true
		}
	}
	return from, to, hasFrom, hasTo
}

// binDays возвращает число календарных дней в периоде-бине гранулярности unit,
// начинающемся в binStart, с поправкой на пересечение с запрошенным окном [from,to]:
// крайний неполный бин делится на фактическое число дней внутри окна (а не на полную
// длину бина). Если границы окна не заданы — берётся полная длина бина (day=1,
// week=7, month=число дней месяца).
func binDays(binStart time.Time, unit string, from, to time.Time, hasFrom, hasTo bool) float64 {
	binEnd := nextBinStart(binStart, unit) // эксклюзивный конец бина
	lo := binStart
	if hasFrom && from.After(lo) {
		lo = from
	}
	hi := binEnd
	if hasTo {
		// to — конец дня (23:59:59) МСК; эксклюзивная граница окна = начало след. суток
		// в той же зоне (иначе смешение с MSK-бинами даёт дробные дни).
		toExcl := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, analyticsLocation).AddDate(0, 0, 1)
		if toExcl.Before(hi) {
			hi = toExcl
		}
	}
	days := hi.Sub(lo).Hours() / 24
	if days < 1 {
		days = 1 // защита от деления на ноль/отрицательного пересечения
	}
	return days
}

// nextBinStart возвращает начало следующего бина гранулярности unit. week —
// ISO-неделя (date_trunc('week') начинает с понедельника), поэтому +7 дней.
func nextBinStart(binStart time.Time, unit string) time.Time {
	switch unit {
	case "week":
		return binStart.AddDate(0, 0, 7)
	case "month":
		return binStart.AddDate(0, 1, 0)
	default: // day
		return binStart.AddDate(0, 0, 1)
	}
}

// applyAvgPerDay превращает целые счётчики метрики-среднего в дробные средние в день.
// Чистая функция. Для разреза period каждое значение бина делится на число дней бина
// (binDays, с поправкой на крайние неполные бины). Для разреза none знаменатель —
// число дней всего окна [from,to] (общее среднее). Значения переносятся из Values в
// FloatValues и удаляются из Values (метрика дробная). Округление до 1 знака.
func applyAvgPerDay(rows []models.ReportMetricRow, metric, dimension, granularity string, filters []models.ReportFilterValue) float64 {
	from, to, hasFrom, hasTo := reportDateBounds(filters)
	unit := "day"
	if granularity != "" {
		if u, ok := aggGranularity[granularity]; ok {
			unit = u
		}
	}

	var sumCount int64
	for i := range rows {
		raw := rows[i].Values[metric]
		sumCount += raw
		delete(rows[i].Values, metric)
		if rows[i].FloatValues == nil {
			rows[i].FloatValues = map[string]float64{}
		}
		var days float64
		if dimension == "period" {
			if binStart, ok := parseReportDate(rows[i].Label, false); ok {
				days = binDays(binStart, unit, from, to, hasFrom, hasTo)
			} else {
				days = 1
			}
		} else {
			days = windowDays(from, to, hasFrom, hasTo)
		}
		rows[i].FloatValues[metric] = round1(float64(raw) / days)
	}

	// Итог метрики-среднего: общий счётчик / число дней окна (а не сумма средних
	// бинов — последняя не имеет смысла как "среднее за период").
	return round1(float64(sumCount) / windowDays(from, to, hasFrom, hasTo))
}

// windowDays — число календарных дней в окне [from,to] включительно; без заданных
// границ -> 1 (нет периода — среднее вырождается в сам счётчик). Считается по датам
// (to нормализуется к началу своих суток), чтобы не зависеть от 23:59:59-хвоста.
func windowDays(from, to time.Time, hasFrom, hasTo bool) float64 {
	if !hasFrom || !hasTo {
		return 1
	}
	fromDay := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, analyticsLocation)
	toDay := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, analyticsLocation)
	days := int(toDay.Sub(fromDay).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	return float64(days)
}

// round1 округляет до одного знака после запятой.
func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
