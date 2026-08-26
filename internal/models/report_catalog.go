package models

// ReportFieldType — тип поля фильтра, по нему фронт выбирает контрол ввода.
type ReportFieldType string

const (
	ReportFieldDate ReportFieldType = "date" // диапазон дат (from/to)
	ReportFieldEnum ReportFieldType = "enum" // фиксированный набор значений (статусы)
	ReportFieldDict ReportFieldType = "dict" // динамический справочник (орг, места, ...)
)

// ReportOption — вариант значения для enum/dict-фильтра.
type ReportOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ReportMetricInfo — метрика агрегатного отчёта (что измеряем).
// Group — тематическая группа карточек метрик в гиде (Заявки/Машины/Люди/...).
// Dimensions перечисляет ключи разрезов, по которым эту метрику можно группировать.
// Filters — ключи фильтров, применимых к этой метрике (шаг "Фильтры" гида); все
// исполнимы движком (date_range + per-metric whitelist).
type ReportMetricInfo struct {
	Key        string   `json:"key"`
	Label      string   `json:"label"`
	Unit       string   `json:"unit,omitempty"`
	Group      string   `json:"group,omitempty"`
	Dimensions []string `json:"dimensions"`
	Filters    []string `json:"filters,omitempty"`
}

// ReportDimensionInfo — разрез (ось группировки) агрегатного отчёта.
type ReportDimensionInfo struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// ReportPivotInfo — ось cross-tab: значения этой оси разворачиваются в колонки при
// Dimension="period". Metrics — метрики, для которых pivot применим (фронт прячет
// опцию для прочих). Пока единственная ось — тип вложения для applications_count.
type ReportPivotInfo struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Metrics []string `json:"metrics"`
}

// ReportFilterInfo — поле фильтра. Для enum/dict Options заполнены значениями.
type ReportFilterInfo struct {
	Key     string          `json:"key"`
	Label   string          `json:"label"`
	Type    ReportFieldType `json:"type"`
	Options []ReportOption  `json:"options,omitempty"`
}

// ReportColumnInfo — столбец list-режима (выгрузки строк). Type (опц.) подсказывает
// фронту формат значения: "date"/"time"/"datetime" -> дд.мм.гггг и время без секунд.
// Пусто -> произвольный текст (номер, ФИО, организация), не форматируется.
type ReportColumnInfo struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Type  string `json:"type,omitempty"`
}

// ReportListEntityInfo — сущность list-режима: какие столбцы и фильтры доступны.
type ReportListEntityInfo struct {
	Key     string             `json:"key"`
	Label   string             `json:"label"`
	Columns []ReportColumnInfo `json:"columns"`
	Filters []string           `json:"filters"`
}

// ReportCatalog — каталог конструктора отчётов: whitelist метрик, разрезов,
// фильтров и list-сущностей с уже подставленными значениями динамических справочников.
// Источник правды для UI конструктора (B3) и движка исполнения (B2).
type ReportCatalog struct {
	Metrics       []ReportMetricInfo     `json:"metrics"`
	Dimensions    []ReportDimensionInfo  `json:"dimensions"`
	Filters       []ReportFilterInfo     `json:"filters"`
	ListEntities  []ReportListEntityInfo `json:"list_entities"`
	Granularities []ReportOption         `json:"granularities"`
	Pivots        []ReportPivotInfo      `json:"pivots,omitempty"`
}
