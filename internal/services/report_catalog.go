package services

import "systemburo/internal/models"

// Реестры конструктора отчётов — единый whitelist для каталога (B1) и движка
// исполнения (B2). Все SQL-фрагменты здесь — статические константы кода; ввод
// пользователя сверяется по ключам этих карт и никогда не конкатенируется в SQL.
// Клиенту отдаётся только метаданные (ключи/подписи/значения справочников),
// SQL-выражения наружу не выходят (неэкспортируемые поля, не сериализуются).

// optionsSource указывает GetReportCatalog, откуда взять значения для dict-фильтра.
type optionsSource string

const (
	srcNone            optionsSource = ""
	srcStatuses        optionsSource = "statuses"
	srcOrganizations   optionsSource = "organizations"
	srcCompanies       optionsSource = "companies"
	srcAttachmentTypes optionsSource = "attachment_types"
	srcCitizenships    optionsSource = "citizenships"
	srcUnloadPlaces    optionsSource = "unload_places"
)

// metricDef — агрегатная метрика. baseTable/aggExpr/baseFilter — безопасные
// константы для сборки запроса движком B2; dimensions ограничивает разрезы.
// group — тематическая группа карточек метрик в гиде (шаг "Что считаем").
type metricDef struct {
	label      string
	unit       string
	group      string
	baseTable  string
	aggExpr    string
	baseFilter string
	dimensions []string
}

// Тематические группы метрик (шаг 1 гида). Значения совпадают с подписями карточек.
const (
	metricGroupApplications = "Заявки"
	metricGroupCars         = "Машины"
	metricGroupPeople       = "Люди"
	// metricGroupProcessing - длительности этапов обработки заявки и качество её
	// исхода (#1240; report_duration_metrics.go и report_quality_metrics.go
	// регистрируют их в реестры через init).
	metricGroupProcessing = "Обработка заявок"
	// metricGroupApprovers - метрики самих согласующих: время реакции и нагрузка
	// (#1240, report_approver_metrics.go). Отдельная группа, т.к. считаются не по
	// заявкам, а по голосам согласующих.
	metricGroupApprovers = "Согласующие"
	// metricGroupAcceptors - метрики принимающих: время принятия в работу и нагрузка
	// (#1251 S3, report_acceptor_metrics.go). Считаются по первому принятию заявки,
	// не по самим заявкам.
	metricGroupAcceptors = "Принимающие"
)

// dimensionDef — разрез группировки. Конкретное GROUP BY-выражение и join-путь
// зависят от метрики и резолвятся движком B2; здесь — только подпись для UI.
type dimensionDef struct {
	label string
}

// filterDef — поле фильтра. source задаёт справочник значений для dict-типа.
type filterDef struct {
	label  string
	typ    models.ReportFieldType
	source optionsSource
}

// listColumnDef — столбец list-режима (ключ + подпись).
type listColumnDef struct {
	key   string
	label string
	// format подсказывает фронту тип значения: "date"/"time"/"datetime". Пусто -> текст.
	format string
}

// listEntityDef — сущность list-режима: набор столбцов и применимых фильтров.
type listEntityDef struct {
	label   string
	columns []listColumnDef
	filters []string
}

// Порядок ключей фиксируется явно — карты в Go неупорядочены, а каталог должен
// отдаваться стабильно (детерминированный ответ, удобный для тестов и кэша).

var reportMetricOrder = []string{
	"applications_count",
	"car_entries_count",
	"people_entries_count",
	"avg_cars_per_day",
	"items_sum",
}

var reportMetricRegistry = map[string]metricDef{
	"applications_count": {
		label:      "Количество заявок",
		unit:       "шт",
		group:      metricGroupApplications,
		baseTable:  "applications",
		aggExpr:    "COUNT(*)",
		dimensions: []string{"status", "organization", "company", "attachment_type", "period"},
	},
	"car_entries_count": {
		label:      "Въезды машин",
		unit:       "шт",
		group:      metricGroupCars,
		baseTable:  "cars_history",
		aggExpr:    "COUNT(*)",
		baseFilter: "action_type = 'entry'",
		dimensions: []string{"period", "hour_of_day", "unload_place", "organization"},
	},
	"people_entries_count": {
		label:      "Входы людей",
		unit:       "шт",
		group:      metricGroupPeople,
		baseTable:  "employees_history",
		aggExpr:    "COUNT(*)",
		baseFilter: "action_type = 'entry'",
		dimensions: []string{"period", "hour_of_day", "organization"},
	},
	"avg_cars_per_day": {
		label:      "Среднее машин в день",
		unit:       "шт/день",
		group:      metricGroupCars,
		baseTable:  "cars_history",
		aggExpr:    "COUNT(*)",
		baseFilter: "action_type = 'entry'",
		// Только period (среднее в день имеет смысл только по времени) и none
		// (общее среднее за весь период) — последний валидируется универсально.
		dimensions: []string{"period"},
	},
	"items_sum": {
		label:      "Количество товаров",
		unit:       "шт",
		group:      metricGroupApplications,
		baseTable:  "items",
		aggExpr:    "COALESCE(SUM(items.count), 0)",
		dimensions: []string{"organization", "company", "period"},
	},
}

// dimNone — универсальный разрез "без разреза": один итоговый ряд без GROUP BY.
// Применим к любой метрике, поэтому намеренно НЕ входит в metricDef.dimensions
// (движок валидирует его отдельной веткой). В каталог добавлен как самостоятельный
// разрез, чтобы гид показал его опцией; UI задействует его в срезе GR2/GR3.
const dimNone = "none"

// dimByApprover — разрез по согласующему (#1240, B3). Применим ТОЛЬКО к метрикам
// с базой application_responsible_users (report_approver_metrics.go): там строка
// это голос, и разрез ничего не размножает. Метрикам заявки он не даётся — 1
// заявка : N согласующих размножили бы её по числу голосов.
const dimByApprover = "by_approver"

// dimByAcceptor — разрез по принимающему (#1251 S3). Применим ТОЛЬКО к метрикам
// принимающих (report_acceptor_metrics.go): там база — подзапрос первого принятия
// на заявку, строка = одна заявка на её принимающего, разрез ничего не размножает.
const dimByAcceptor = "by_acceptor"

var reportDimensionOrder = []string{
	dimNone,
	"status",
	"organization",
	"company",
	"attachment_type",
	"unload_place",
	dimByApprover,
	dimByAcceptor,
	"period",
	"hour_of_day",
}

var reportDimensionRegistry = map[string]dimensionDef{
	dimNone:           {label: "Без разреза"},
	"status":          {label: "Статус заявки"},
	"organization":    {label: "Организация"},
	"company":         {label: "Компания"},
	"attachment_type": {label: "Тип вложения"},
	"unload_place":    {label: "Место разгрузки"},
	dimByApprover:     {label: "Согласующий"},
	dimByAcceptor:     {label: "Принимающий"},
	"period":          {label: "Период (дата)"},
	"hour_of_day":     {label: "Час суток"},
}

// pivotAttachmentType — ключ cross-tab оси "тип вложения" (значения -> колонки).
const pivotAttachmentType = "attachment_type"

// reportPivotRegistry — whitelist осей cross-tab. Каждая ось знает, для каких
// метрик она применима (движок разворачивает её в колонки счётчиков заявок).
var reportPivotRegistry = []models.ReportPivotInfo{
	{Key: pivotAttachmentType, Label: "Тип вложения", Metrics: []string{"applications_count"}},
}

var reportFilterOrder = []string{
	"date_range",
	"status",
	"organization",
	"company",
	"attachment_type",
	"citizenship",
	"unload_place",
}

var reportFilterRegistry = map[string]filterDef{
	"date_range":      {label: "Период", typ: models.ReportFieldDate, source: srcNone},
	"status":          {label: "Статус заявки", typ: models.ReportFieldEnum, source: srcStatuses},
	"organization":    {label: "Организация", typ: models.ReportFieldDict, source: srcOrganizations},
	"company":         {label: "Компания", typ: models.ReportFieldDict, source: srcCompanies},
	"attachment_type": {label: "Тип вложения", typ: models.ReportFieldDict, source: srcAttachmentTypes},
	"citizenship":     {label: "Гражданство", typ: models.ReportFieldDict, source: srcCitizenships},
	"unload_place":    {label: "Место разгрузки", typ: models.ReportFieldDict, source: srcUnloadPlaces},
}

var reportListEntityOrder = []string{
	"work_applications",
	"applications",
	"cars",
	"people",
}

var reportListEntityRegistry = map[string]listEntityDef{
	"work_applications": {
		label: "Заявка на работы",
		columns: []listColumnDef{
			{key: "number", label: "Номер заявки"},
			{key: "org_or_company", label: "Организация/Компания"},
			{key: "work_name", label: "Наименование работ"},
			{key: "responsible", label: "Ответственный"},
			{key: "work_period", label: "Период работ", format: "date"},
			{key: "work_time", label: "Время работ", format: "time"},
			{key: "people_count", label: "Кол-во людей"},
		},
		filters: []string{"date_range", "organization", "status"},
	},
	"applications": {
		label: "Заявки",
		columns: []listColumnDef{
			{key: "number", label: "Номер заявки"},
			{key: "status", label: "Статус"},
			{key: "organization", label: "Организация"},
			{key: "company", label: "Компания"},
			{key: "sending_datetime", label: "Дата подачи", format: "datetime"},
			{key: "attachments_count", label: "Вложений"},
		},
		filters: []string{"date_range", "status", "organization", "company"},
	},
	"cars": {
		label: "Машины",
		columns: []listColumnDef{
			{key: "car_number", label: "Гос. номер"},
			{key: "mark", label: "Марка"},
			{key: "organization", label: "Организация"},
			{key: "place", label: "Место разгрузки"},
			{key: "territory_status", label: "На территории"},
		},
		filters: []string{"organization", "unload_place"},
	},
	"people": {
		label: "Люди",
		columns: []listColumnDef{
			{key: "full_name", label: "ФИО"},
			{key: "organization", label: "Организация"},
			{key: "citizenship", label: "Гражданство"},
			{key: "place", label: "Место разгрузки"},
			{key: "territory_status", label: "На территории"},
		},
		filters: []string{"organization", "citizenship"},
	},
}

// reportGranularities — допустимые гранулярности для разреза "period".
// Совпадают с whitelist GetTimeline (resolveTimelineSource).
var reportGranularities = []models.ReportOption{
	{Value: "day", Label: "По дням"},
	{Value: "week", Label: "По неделям"},
	{Value: "month", Label: "По месяцам"},
}

// statusOptions — статичные значения enum-фильтра "status".
func statusOptions() []models.ReportOption {
	statuses := []string{
		models.StatusUnread,
		models.StatusProcessing,
		models.StatusApproval,
		models.StatusApproved,
		models.StatusRejected,
		models.StatusInWork,
		models.StatusCompleted,
		models.StatusRefused,
	}
	opts := make([]models.ReportOption, 0, len(statuses))
	for _, s := range statuses {
		opts = append(opts, models.ReportOption{Value: s, Label: s})
	}
	return opts
}

// dynamicReportOptions — значения динамических справочников, подгружаемые из БД
// и подставляемые в dict-фильтры каталога.
type dynamicReportOptions struct {
	organizations   []models.ReportOption
	companies       []models.ReportOption
	attachmentTypes []models.ReportOption
	citizenships    []models.ReportOption
	unloadPlaces    []models.ReportOption
}

// optionsFor возвращает значения для фильтра по его source.
func (d dynamicReportOptions) optionsFor(src optionsSource) []models.ReportOption {
	switch src {
	case srcStatuses:
		return statusOptions()
	case srcOrganizations:
		return d.organizations
	case srcCompanies:
		return d.companies
	case srcAttachmentTypes:
		return d.attachmentTypes
	case srcCitizenships:
		return d.citizenships
	case srcUnloadPlaces:
		return d.unloadPlaces
	default:
		return nil
	}
}

// metricApplicableFilters выводит список применимых к метрике фильтров ИЗ схемы
// движка (aggMetricRegistry) — единый источник, исключающий рассинхрон каталога и
// исполнителя. date_range применим ко всем метрикам (по tsColumn); остальные —
// только те, что движок умеет резолвить для этой метрики. Порядок — reportFilterOrder.
func metricApplicableFilters(metric string) []string {
	schema, ok := aggMetricRegistry[metric]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(reportFilterOrder))
	for _, f := range reportFilterOrder {
		if _, resolvable := schema.filters[f]; f == "date_range" || resolvable {
			out = append(out, f)
		}
	}
	return out
}

// buildReportCatalog собирает каталог из whitelist-реестров и подгруженных
// значений справочников. Чистая функция (без БД) — тестируется напрямую.
func buildReportCatalog(dyn dynamicReportOptions) models.ReportCatalog {
	cat := models.ReportCatalog{
		Metrics:       make([]models.ReportMetricInfo, 0, len(reportMetricOrder)),
		Dimensions:    make([]models.ReportDimensionInfo, 0, len(reportDimensionOrder)),
		Filters:       make([]models.ReportFilterInfo, 0, len(reportFilterOrder)),
		ListEntities:  make([]models.ReportListEntityInfo, 0, len(reportListEntityOrder)),
		Granularities: reportGranularities,
		Pivots:        reportPivotRegistry,
	}

	for _, key := range reportMetricOrder {
		def := reportMetricRegistry[key]
		cat.Metrics = append(cat.Metrics, models.ReportMetricInfo{
			Key:        key,
			Label:      def.label,
			Unit:       def.unit,
			Group:      def.group,
			Dimensions: def.dimensions,
			Filters:    metricApplicableFilters(key),
		})
	}

	for _, key := range reportDimensionOrder {
		cat.Dimensions = append(cat.Dimensions, models.ReportDimensionInfo{
			Key:   key,
			Label: reportDimensionRegistry[key].label,
		})
	}

	for _, key := range reportFilterOrder {
		def := reportFilterRegistry[key]
		cat.Filters = append(cat.Filters, models.ReportFilterInfo{
			Key:     key,
			Label:   def.label,
			Type:    def.typ,
			Options: dyn.optionsFor(def.source),
		})
	}

	for _, key := range reportListEntityOrder {
		def := reportListEntityRegistry[key]
		cols := make([]models.ReportColumnInfo, 0, len(def.columns))
		for _, c := range def.columns {
			cols = append(cols, models.ReportColumnInfo{Key: c.key, Label: c.label, Type: c.format})
		}
		cat.ListEntities = append(cat.ListEntities, models.ReportListEntityInfo{
			Key:     key,
			Label:   def.label,
			Columns: cols,
			Filters: def.filters,
		})
	}

	return cat
}
