package models

// ReportFilterValue — одно применённое значение фильтра в запросе отчёта.
// Для dict/enum-фильтров заполняется Values; для date_range — From/To (YYYY-MM-DD).
type ReportFilterValue struct {
	Key    string   `json:"key"`
	Values []string `json:"values,omitempty"`
	From   string   `json:"from,omitempty"`
	To     string   `json:"to,omitempty"`
}

// ReportRequest — запрос конструктора отчётов (POST /statistics/report).
// Mode=aggregate: разрез (Dimension) + одна метрика (Metric) ИЛИ несколько (Metrics).
// Metrics имеет приоритет; если он пуст — берётся Metric (обратная совместимость с
// одиночным конструктором). Каждая метрика становится колонкой результата.
// Dimension="none" — без разреза (один итоговый ряд).
// Pivot (опц.) — cross-tab: при Dimension="period" каждое значение оси Pivot
// (например "attachment_type") разворачивается в отдельную колонку-счётчик рядом с
// метриками. Пустой Pivot -> обычный отчёт. Mode=list: выгрузка строк сущности (Entity).
type ReportRequest struct {
	Mode        string   `json:"mode"`
	Metric      string   `json:"metric"`
	Metrics     []string `json:"metrics,omitempty"`
	Dimension   string   `json:"dimension"`
	Granularity string   `json:"granularity"`
	Pivot       string   `json:"pivot,omitempty"`
	Entity      string   `json:"entity"`
	// Columns (опц., только Mode=list) — какие столбцы сущности отдавать. Пусто —
	// все столбцы каталога. Порядок берётся из каталога, а не из запроса: он же
	// определяет порядок колонок в выгрузке.
	Columns []string            `json:"columns,omitempty"`
	Filters []ReportFilterValue `json:"filters"`
	Sort    string              `json:"sort"`
	Limit   int                 `json:"limit"`
}

// ReportAggregateRow — строка агрегатного отчёта: бакет разреза + значение метрики.
type ReportAggregateRow struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
}

// ReportColumnKind различает тип колонки результата: обычная метрика или
// pivot-колонка cross-tab (значение оси Pivot, развёрнутое в столбец).
type ReportColumnKind string

const (
	ReportColumnMetric ReportColumnKind = "metric" // обычная метрика (Values)
	ReportColumnPivot  ReportColumnKind = "pivot"  // cross-tab колонка (Values)
)

// ReportValueType — тип значения колонки, по которому фронт выбирает формат
// (аналог ReportColumnInfo.Type у list-режима: форматируем по ТИПУ КОЛОНКИ, а не
// по виду значения). Пусто — обычное число.
type ReportValueType string

// ReportValueDuration — значение колонки это длительность в СЕКУНДАХ (#1240):
// фронт показывает её как «2 ч 15 мин», хранит и сортирует как число.
const ReportValueDuration ReportValueType = "duration"

// ReportMetricColumn — колонка мультиметричного/cross-tab отчёта (ключ, подпись, единица).
// Kind пуст для обычных метрик (обратная совместимость) либо "pivot" для cross-tab
// колонок. Float=true -> значение колонки лежит в ReportMetricRow.FloatValues
// (дробные метрики вроде среднего), иначе — в Values (целые счётчики).
// Type задаёт формат значения (пусто — число, "duration" — секунды).
type ReportMetricColumn struct {
	Key   string           `json:"key"`
	Label string           `json:"label"`
	Unit  string           `json:"unit,omitempty"`
	Kind  ReportColumnKind `json:"kind,omitempty"`
	Float bool             `json:"float,omitempty"`
	Type  ReportValueType  `json:"type,omitempty"`
}

// ReportMetricRow — строка мультиметрик: подпись разреза + значение каждой колонки
// (по её ключу). Целочисленные колонки (счётчики, суммы, pivot) -> Values; дробные
// (средние) -> FloatValues. Колонки без значения в этом бакете -> 0. FloatValues
// заполняется только при наличии дробных метрик (omitempty -> старый ответ не меняется).
type ReportMetricRow struct {
	Label       string             `json:"label"`
	Values      map[string]int64   `json:"values"`
	FloatValues map[string]float64 `json:"float_values,omitempty"`
}

// ReportResponse — результат агрегатного отчёта.
// Legacy-поля (Metric/Unit/Rows/Total) — для одиночной метрики (текущий FE).
// Мультиметрики (GR0): Columns — колонки-метрики, MetricRows — строки разреза со
// значением каждой метрики, Totals — итог по каждой метрике. Эти три заполняются
// всегда (в т.ч. при одной метрике) — единый формат для гида.
type ReportResponse struct {
	Mode      string               `json:"mode"`
	Metric    string               `json:"metric,omitempty"`
	Dimension string               `json:"dimension,omitempty"`
	Unit      string               `json:"unit,omitempty"`
	Rows      []ReportAggregateRow `json:"rows"`
	Total     int64                `json:"total"`

	Columns     []ReportMetricColumn `json:"columns,omitempty"`
	MetricRows  []ReportMetricRow    `json:"metric_rows,omitempty"`
	Totals      map[string]int64     `json:"totals,omitempty"`
	FloatTotals map[string]float64   `json:"float_totals,omitempty"`
}

// ReportListResponse — результат list-отчёта (mode=list): выгрузка строк сущности.
// Columns задаёт порядок и подписи столбцов (из каталога B1), Rows — значения
// строк парами ключ->значение (ключи совпадают с Columns[].Key). Total — число
// возвращённых строк (с учётом лимита).
type ReportListResponse struct {
	Mode    string             `json:"mode"`
	Entity  string             `json:"entity"`
	Columns []ReportColumnInfo `json:"columns"`
	Rows    []map[string]any   `json:"rows"`
	Total   int                `json:"total"`
}
