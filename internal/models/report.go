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
// Mode=aggregate: Metric+Dimension обязательны (агрегатный разрез).
// Mode=list: выгрузка строк сущности (Entity) — добавляется отдельным срезом.
type ReportRequest struct {
	Mode        string              `json:"mode"`
	Metric      string              `json:"metric"`
	Dimension   string              `json:"dimension"`
	Granularity string              `json:"granularity"`
	Entity      string              `json:"entity"`
	Filters     []ReportFilterValue `json:"filters"`
	Sort        string              `json:"sort"`
	Limit       int                 `json:"limit"`
}

// ReportAggregateRow — строка агрегатного отчёта: бакет разреза + значение метрики.
type ReportAggregateRow struct {
	Label string `json:"label"`
	Value int64  `json:"value"`
}

// ReportResponse — результат агрегатного отчёта: строки по разрезу и итог (сумма значений).
type ReportResponse struct {
	Mode      string               `json:"mode"`
	Metric    string               `json:"metric,omitempty"`
	Dimension string               `json:"dimension,omitempty"`
	Unit      string               `json:"unit,omitempty"`
	Rows      []ReportAggregateRow `json:"rows"`
	Total     int64                `json:"total"`
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
