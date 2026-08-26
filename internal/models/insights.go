package models

// Инсайты аналитики (#632, E1) — эвристики поверх агрегатов движка отчётов.
// Считаются вызовами RunReport с разными запросами, без нового SQL.

// InsightsResponse — сводка готовых инсайтов за период.
type InsightsResponse struct {
	PeakHours   []PeakHoursInsight  `json:"peak_hours"`
	Comparisons []ComparisonInsight `json:"comparisons"`
	TopPlaces   []TopItemInsight    `json:"top_places"`
	TopOrgs     []TopItemInsight    `json:"top_orgs"`
	Trends      []TrendInsight      `json:"trends"`
}

// HourBucket — значение метрики в конкретный час суток (0-23).
type HourBucket struct {
	Hour  int   `json:"hour"`
	Value int64 `json:"value"`
}

// PeakHoursInsight — распределение метрики по часам суток с выделенным пиком.
type PeakHoursInsight struct {
	Metric    string       `json:"metric"`
	Label     string       `json:"label"`
	Unit      string       `json:"unit,omitempty"`
	PeakHour  int          `json:"peak_hour"`
	PeakValue int64        `json:"peak_value"`
	Hourly    []HourBucket `json:"hourly"`
}

// ComparisonInsight — метрика за текущий период против предыдущего равной длины.
type ComparisonInsight struct {
	Metric    string  `json:"metric"`
	Label     string  `json:"label"`
	Unit      string  `json:"unit,omitempty"`
	Current   int64   `json:"current"`
	Previous  int64   `json:"previous"`
	DeltaPct  float64 `json:"delta_pct"`
	Direction string  `json:"direction"` // up | down | flat
}

// TopItemInsight — позиция рейтинга (место разгрузки или организация).
type TopItemInsight struct {
	Metric string `json:"metric"`
	Label  string `json:"label"`
	Value  int64  `json:"value"`
}

// TrendInsight — динамика метрики по дням периода с направлением тренда.
type TrendInsight struct {
	Metric    string  `json:"metric"`
	Label     string  `json:"label"`
	Direction string  `json:"direction"` // up | down | flat
	Series    []int64 `json:"series"`
}
