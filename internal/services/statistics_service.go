package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
)

// StatisticsService — интерфейс бизнес-логики статистики дашборда.
type StatisticsService interface {
	GetSummary(ctx context.Context, from, to time.Time) (*models.StatsSummary, error)
	GetTimeline(ctx context.Context, from, to time.Time, metric, granularity string) ([]models.StatsTimelinePoint, error)
	GetRecentPassages(ctx context.Context, limit int) (*models.RecentPassages, error)
	GetReportCatalog(ctx context.Context) (*models.ReportCatalog, error)
	RunReport(ctx context.Context, req models.ReportRequest) (*models.ReportResponse, error)
	RunReportList(ctx context.Context, req models.ReportRequest) (*models.ReportListResponse, error)

	GetInsights(ctx context.Context, from, to string) (*models.InsightsResponse, error)

	ListReportTemplates(ctx context.Context, userID int) ([]models.ReportTemplate, error)
	CreateReportTemplate(ctx context.Context, userID int, req models.SaveReportTemplateRequest) (*models.ReportTemplate, error)
	UpdateReportTemplate(ctx context.Context, userID, id int, req models.SaveReportTemplateRequest) (*models.ReportTemplate, error)
	DeleteReportTemplate(ctx context.Context, userID, id int) error
}

type statisticsService struct {
	db *gorm.DB
}

// NewStatisticsService создаёт реализацию StatisticsService.
func NewStatisticsService(db *gorm.DB) StatisticsService {
	return &statisticsService{db: db}
}

// GetSummary возвращает сводную статистику за период [from, to].
func (s *statisticsService) GetSummary(ctx context.Context, from, to time.Time) (*models.StatsSummary, error) {
	var summary models.StatsSummary

	// total_applications
	if err := s.db.WithContext(ctx).
		Table("applications").
		Where("sending_datetime BETWEEN ? AND ?", from, to).
		Count(&summary.TotalApplications).Error; err != nil {
		return nil, fmt.Errorf("statistics: count applications: %w", err)
	}

	// by_attachment_type
	summary.ByAttachmentType = make([]models.AttachmentTypeCount, 0)
	if err := s.db.WithContext(ctx).
		Table("attachments a").
		Joins("JOIN applications app ON app.id = a.application_id").
		Joins("LEFT JOIN unique_attachments ua ON ua.id = a.unique_attachment_id").
		Where("app.sending_datetime BETWEEN ? AND ?", from, to).
		Select("COALESCE(ua.display_name, a.attachment_display_name, a.attachment_type) AS name, COUNT(*) AS count").
		Group("COALESCE(ua.display_name, a.attachment_display_name, a.attachment_type)").
		Order("count DESC").
		Scan(&summary.ByAttachmentType).Error; err != nil {
		return nil, fmt.Errorf("statistics: by_attachment_type: %w", err)
	}

	// by_status
	summary.ByStatus = make([]models.StatusCount, 0)
	if err := s.db.WithContext(ctx).
		Table("applications").
		Where("sending_datetime BETWEEN ? AND ?", from, to).
		Select("status, COUNT(*) AS count").
		Group("status").
		Scan(&summary.ByStatus).Error; err != nil {
		return nil, fmt.Errorf("statistics: by_status: %w", err)
	}

	// processed (терминальные статусы)
	terminalStatuses := []string{
		models.StatusCompleted,
		models.StatusApproved,
		models.StatusRejected,
		models.StatusRefused,
	}
	if err := s.db.WithContext(ctx).
		Table("applications").
		Where("sending_datetime BETWEEN ? AND ? AND status IN ?", from, to, terminalStatuses).
		Count(&summary.Processed).Error; err != nil {
		return nil, fmt.Errorf("statistics: processed: %w", err)
	}

	// in_work (активные статусы)
	inWorkStatuses := []string{
		models.StatusProcessing,
		models.StatusInWork,
		models.StatusApproval,
	}
	if err := s.db.WithContext(ctx).
		Table("applications").
		Where("sending_datetime BETWEEN ? AND ? AND status IN ?", from, to, inWorkStatuses).
		Count(&summary.InWork).Error; err != nil {
		return nil, fmt.Errorf("statistics: in_work: %w", err)
	}

	// cars_entered
	if err := s.db.WithContext(ctx).
		Table("cars_history").
		Where("action_type = 'entry' AND created_at BETWEEN ? AND ?", from, to).
		Count(&summary.CarsEntered).Error; err != nil {
		return nil, fmt.Errorf("statistics: cars_entered: %w", err)
	}

	// people_entered
	if err := s.db.WithContext(ctx).
		Table("employees_history").
		Where("action_type = 'entry' AND created_at BETWEEN ? AND ?", from, to).
		Count(&summary.PeopleEntered).Error; err != nil {
		return nil, fmt.Errorf("statistics: people_entered: %w", err)
	}

	// avg_cars_per_day
	days := int(to.Sub(from).Hours()/24) + 1
	if days < 1 {
		days = 1
	}
	summary.AvgCarsPerDay = float64(summary.CarsEntered) / float64(days)

	// items_sum
	var itemsSum struct{ Sum int64 }
	if err := s.db.WithContext(ctx).
		Table("items i").
		Joins("JOIN attachments a ON a.id = i.attachment_id").
		Joins("JOIN applications app ON app.id = a.application_id").
		Where("app.sending_datetime BETWEEN ? AND ?", from, to).
		Select("COALESCE(SUM(i.count), 0) AS sum").
		Scan(&itemsSum).Error; err != nil {
		return nil, fmt.Errorf("statistics: items_sum: %w", err)
	}
	summary.ItemsSum = itemsSum.Sum

	// cars_on_territory (territory_status = 1)
	if err := s.db.WithContext(ctx).
		Table("cars").
		Where("territory_status = 1").
		Count(&summary.CarsOnTerritory).Error; err != nil {
		return nil, fmt.Errorf("statistics: cars_on_territory: %w", err)
	}

	// people_on_territory (territory_status = 1)
	if err := s.db.WithContext(ctx).
		Table("employees").
		Where("territory_status = 1").
		Count(&summary.PeopleOnTerritory).Error; err != nil {
		return nil, fmt.Errorf("statistics: people_on_territory: %w", err)
	}

	// users_online: онлайн = вход за последние 15 минут, прокси без сессий.
	onlineThreshold := time.Now().UTC().Add(-15 * time.Minute)
	if err := s.db.WithContext(ctx).
		Table("users").
		Where("last_login_at >= ?", onlineThreshold).
		Count(&summary.UsersOnline).Error; err != nil {
		return nil, fmt.Errorf("statistics: users_online: %w", err)
	}

	// active_users
	if err := s.db.WithContext(ctx).
		Table("users").
		Where("is_active = true AND is_banned = false").
		Count(&summary.ActiveUsers).Error; err != nil {
		return nil, fmt.Errorf("statistics: active_users: %w", err)
	}

	// banned_users
	if err := s.db.WithContext(ctx).
		Table("users").
		Where("is_banned = true").
		Count(&summary.BannedUsers).Error; err != nil {
		return nil, fmt.Errorf("statistics: banned_users: %w", err)
	}

	// open_feedback
	if err := s.db.WithContext(ctx).
		Table("feedback").
		Where("status = ?", models.FeedbackOpen).
		Count(&summary.OpenFeedback).Error; err != nil {
		return nil, fmt.Errorf("statistics: open_feedback: %w", err)
	}

	// active_unload_places
	if err := s.db.WithContext(ctx).
		Table("unload_places").
		Where("is_active = true").
		Count(&summary.ActiveUnloadPlaces).Error; err != nil {
		return nil, fmt.Errorf("statistics: active_unload_places: %w", err)
	}

	// blacklist_cars
	if err := s.db.WithContext(ctx).
		Table("vehicle_blacklists").
		Where("is_active = true").
		Count(&summary.BlacklistCars).Error; err != nil {
		return nil, fmt.Errorf("statistics: blacklist_cars: %w", err)
	}

	// blacklist_people
	if err := s.db.WithContext(ctx).
		Table("person_blacklists").
		Where("is_active = true").
		Count(&summary.BlacklistPeople).Error; err != nil {
		return nil, fmt.Errorf("statistics: blacklist_people: %w", err)
	}

	// unique_cars
	if err := s.db.WithContext(ctx).
		Table("unique_cars").
		Count(&summary.UniqueCars).Error; err != nil {
		return nil, fmt.Errorf("statistics: unique_cars: %w", err)
	}

	// unique_people
	if err := s.db.WithContext(ctx).
		Table("unique_employees").
		Count(&summary.UniquePeople).Error; err != nil {
		return nil, fmt.Errorf("statistics: unique_people: %w", err)
	}

	return &summary, nil
}

// timelineSource описывает источник данных для GetTimeline.
type timelineSource struct {
	table    string
	tsColumn string
	filter   string // опциональный WHERE-фрагмент (безопасная константа, не пользовательский ввод)
}

// resolveTimelineSource возвращает параметры запроса по metric/granularity из whitelist.
// Вынесена отдельно для тестируемости без БД.
func resolveTimelineSource(metric, granularity string) (src timelineSource, unit string, err error) {
	metricMap := map[string]timelineSource{
		"applications":   {table: "applications", tsColumn: "sending_datetime", filter: ""},
		"car_entries":    {table: "cars_history", tsColumn: "created_at", filter: "action_type='entry'"},
		"people_entries": {table: "employees_history", tsColumn: "created_at", filter: "action_type='entry'"},
	}
	granularityMap := map[string]string{
		"day":   "day",
		"week":  "week",
		"month": "month",
	}

	src, ok := metricMap[metric]
	if !ok {
		return timelineSource{}, "", fmt.Errorf("statistics: unknown metric %q", metric)
	}
	unit, ok = granularityMap[granularity]
	if !ok {
		return timelineSource{}, "", fmt.Errorf("statistics: unknown granularity %q", granularity)
	}
	return src, unit, nil
}

// GetTimeline возвращает точки графика за период [from, to].
// metric и granularity проходят через whitelist — конкатенация пользовательского ввода в SQL исключена.
func (s *statisticsService) GetTimeline(ctx context.Context, from, to time.Time, metric, granularity string) ([]models.StatsTimelinePoint, error) {
	src, unit, err := resolveTimelineSource(metric, granularity)
	if err != nil {
		return nil, err
	}

	// table, tsColumn и unit берутся только из whitelist-карт — безопасно подставлять в SQL.
	// from, to и значения filter передаются через ? плейсхолдеры.
	selectExpr := fmt.Sprintf(
		"to_char(date_trunc('%s', %s), 'YYYY-MM-DD') AS date, COUNT(*) AS count",
		unit, src.tsColumn,
	)
	groupExpr := fmt.Sprintf("date_trunc('%s', %s)", unit, src.tsColumn)

	tx := s.db.WithContext(ctx).
		Table(src.table).
		Select(selectExpr).
		Where(fmt.Sprintf("%s BETWEEN ? AND ?", src.tsColumn), from, to)

	if src.filter != "" {
		tx = tx.Where(src.filter)
	}

	points := make([]models.StatsTimelinePoint, 0)
	if err := tx.
		Group(groupExpr).
		Order(groupExpr + " ASC").
		Scan(&points).Error; err != nil {
		return nil, fmt.Errorf("statistics: timeline: %w", err)
	}

	return points, nil
}

// GetRecentPassages возвращает последние отметки проходов людей и проездов машин
// (action_type entry/exit) для живых лент дашборда. Место отметки — таблица системы
// (history.table_id -> system_tables), организация — через вложение/заявку.
func (s *statisticsService) GetRecentPassages(ctx context.Context, limit int) (*models.RecentPassages, error) {
	if limit <= 0 {
		limit = 15
	}
	if limit > 50 {
		limit = 50
	}

	result := &models.RecentPassages{
		People: make([]models.RecentPassage, 0),
		Cars:   make([]models.RecentPassage, 0),
	}

	if err := s.db.WithContext(ctx).
		Table("employees_history eh").
		Joins("JOIN employees e ON e.id = eh.employee_id").
		Joins("LEFT JOIN attachments a ON a.id = e.attachment_id").
		Joins("LEFT JOIN applications app ON app.id = a.application_id").
		Joins("LEFT JOIN organizations org ON org.id = app.organization_id").
		Joins("LEFT JOIN companies comp ON comp.id = app.company_id").
		Joins("LEFT JOIN system_tables st ON st.id = eh.table_id").
		Where("eh.action_type IN ?", []string{"entry", "exit"}).
		Select("eh.action_type AS action_type, eh.created_at AS created_at, " +
			"TRIM(CONCAT(e.last_name, ' ', e.first_name, ' ', COALESCE(e.middle_name, ''))) AS subject, " +
			"'' AS mark, " +
			"COALESCE(org.name, comp.name, '') AS organization, " +
			"COALESCE(st.display_name, '') AS place").
		Order("eh.created_at DESC").
		Limit(limit).
		Scan(&result.People).Error; err != nil {
		return nil, fmt.Errorf("statistics: recent people passages: %w", err)
	}

	if err := s.db.WithContext(ctx).
		Table("cars_history ch").
		Joins("JOIN cars c ON c.id = ch.car_id").
		Joins("LEFT JOIN attachments a ON a.id = c.attachment_id").
		Joins("LEFT JOIN applications app ON app.id = a.application_id").
		Joins("LEFT JOIN organizations org ON org.id = app.organization_id").
		Joins("LEFT JOIN companies comp ON comp.id = app.company_id").
		Joins("LEFT JOIN system_tables st ON st.id = ch.table_id").
		Where("ch.action_type IN ?", []string{"entry", "exit"}).
		Select("ch.action_type AS action_type, ch.created_at AS created_at, " +
			"c.car_number AS subject, " +
			// mark_name - актуальное поле, car_brand - устаревший fallback (как в trash/blacklist).
			"COALESCE(NULLIF(c.mark_name, ''), c.car_brand, '') AS mark, " +
			"COALESCE(org.name, comp.name, '') AS organization, " +
			"COALESCE(st.display_name, '') AS place").
		Order("ch.created_at DESC").
		Limit(limit).
		Scan(&result.Cars).Error; err != nil {
		return nil, fmt.Errorf("statistics: recent car passages: %w", err)
	}

	return result, nil
}

// GetReportCatalog возвращает каталог конструктора отчётов: whitelist метрик,
// разрезов, фильтров и list-сущностей с подставленными значениями динамических
// справочников. Метаданные берутся из реестров (report_catalog.go), значения
// справочников — из БД.
func (s *statisticsService) GetReportCatalog(ctx context.Context) (*models.ReportCatalog, error) {
	dyn, err := s.loadDynamicReportOptions(ctx)
	if err != nil {
		return nil, err
	}
	catalog := buildReportCatalog(dyn)
	return &catalog, nil
}

// loadDynamicReportOptions подгружает значения справочников для dict-фильтров.
func (s *statisticsService) loadDynamicReportOptions(ctx context.Context) (dynamicReportOptions, error) {
	var dyn dynamicReportOptions

	// load подставляет table/column/where прямо в SQL (GORM не экранирует
	// строковые выражения), поэтому вызывать его допустимо ТОЛЬКО с константами
	// кода - никогда с пользовательским вводом.
	load := func(name, table, column, where string) ([]models.ReportOption, error) {
		var names []string
		tx := s.db.WithContext(ctx).Table(table).
			Distinct(column).
			Where(column + " IS NOT NULL AND " + column + " <> ''")
		if where != "" {
			tx = tx.Where(where)
		}
		if err := tx.Order(column+" ASC").Pluck(column, &names).Error; err != nil {
			return nil, fmt.Errorf("statistics: report catalog %s: %w", name, err)
		}
		opts := make([]models.ReportOption, 0, len(names))
		for _, n := range names {
			opts = append(opts, models.ReportOption{Value: n, Label: n})
		}
		return opts, nil
	}

	var err error
	if dyn.organizations, err = load("organizations", "organizations", "name", "is_active = true"); err != nil {
		return dyn, err
	}
	if dyn.companies, err = load("companies", "companies", "name", "is_active = true"); err != nil {
		return dyn, err
	}
	if dyn.attachmentTypes, err = load("attachment_types", "unique_attachments", "display_name", "is_active = true"); err != nil {
		return dyn, err
	}
	if dyn.citizenships, err = load("citizenships", "citizenships", "name", "is_active = true"); err != nil {
		return dyn, err
	}
	if dyn.unloadPlaces, err = load("unload_places", "unload_places", "name", "is_active = true"); err != nil {
		return dyn, err
	}

	return dyn, nil
}

// RunReport исполняет агрегатный отчёт конструктора (mode=aggregate): одна или
// несколько метрик (Metrics) x разрез x фильтры x период x sort/limit. Каждая
// метрика исполняется отдельным агрегатным запросом (у метрик разные базовые
// таблицы — общий GROUP BY невозможен) и сливается по подписи разреза в Go
// (report_engine.go, чистая логика). Планы собираются buildAggregatePlan из
// whitelist-схем: ввод сверяется по ключам и подставляется только через
// плейсхолдеры. Невалидный запрос -> ErrInvalidReportRequest (400 в handler).
func (s *statisticsService) RunReport(ctx context.Context, req models.ReportRequest) (*models.ReportResponse, error) {
	metrics, err := resolveReportMetrics(req)
	if err != nil {
		return nil, err
	}
	multi := len(metrics) > 1

	perMetric := make(map[string][]models.ReportAggregateRow, len(metrics))
	columns := make([]models.ReportMetricColumn, 0, len(metrics))
	for _, m := range metrics {
		mreq := req
		mreq.Metric = m
		plan, perr := buildAggregatePlan(mreq)
		if perr != nil {
			return nil, perr
		}
		// При мультиметриках итоговый лимит применяется после слияния (top-N по
		// сумме метрик), поэтому каждую метрику грузим широко.
		if multi {
			plan.limit = maxReportLimit
		}
		rows, rerr := s.execAggregatePlan(ctx, plan)
		if rerr != nil {
			return nil, rerr
		}
		perMetric[m] = rows
		columns = append(columns, models.ReportMetricColumn{
			Key:   m,
			Label: reportMetricRegistry[m].label,
			Unit:  plan.unit,
		})
	}

	metricRows, totals := mergeMetricRows(metrics, perMetric, req.Dimension, req.Sort, clampLimit(req.Limit))

	// Cross-tab (G4): при заданном req.Pivot и разрезе period — развернуть значения
	// оси pivot в добавочные колонки-счётчики (отдельной веткой движка). Невалидный
	// pivot (неизвестная ось / неприменима к метрикам / не period) -> 400.
	axis, pivotOn, perr := resolvePivot(req.Pivot, req.Dimension, metrics)
	if perr != nil {
		return nil, perr
	}
	if pivotOn {
		pivotCols, cerr := s.collectPivotColumns(ctx, metrics, axis, req, metricRows)
		if cerr != nil {
			return nil, cerr
		}
		columns = append(columns, pivotCols...)
	}

	// Метрики-средние (avg_*): целые счётчики бинов делятся на число дней бина в Go
	// (постпроцесс), значения переходят в FloatValues. Колонка помечается Float.
	var floatTotals map[string]float64
	for i, m := range metrics {
		if !avgMetrics[m] {
			continue
		}
		total := applyAvgPerDay(metricRows, m, req.Dimension, req.Granularity, req.Filters)
		columns[i].Float = true
		if floatTotals == nil {
			floatTotals = map[string]float64{}
		}
		floatTotals[m] = total
		delete(totals, m)
	}

	// Legacy-поля одиночной метрики (текущий FE читает Rows/Total/Unit): первая
	// метрика как label/value по уже упорядоченным строкам. Unit берём из той же
	// колонки (единый источник — план метрики), чтобы columns[].unit и legacy unit
	// не разъезжались.
	first := metrics[0]
	legacyRows := make([]models.ReportAggregateRow, 0, len(metricRows))
	for _, r := range metricRows {
		legacyRows = append(legacyRows, models.ReportAggregateRow{Label: r.Label, Value: r.Values[first]})
	}

	return &models.ReportResponse{
		Mode:        "aggregate",
		Metric:      first,
		Dimension:   req.Dimension,
		Unit:        columns[0].Unit,
		Rows:        legacyRows,
		Total:       totals[first],
		Columns:     columns,
		MetricRows:  metricRows,
		Totals:      totals,
		FloatTotals: floatTotals,
	}, nil
}

// collectPivotColumns исполняет cross-tab запрос по каждой метрике оси и вписывает
// ячейки в уже упорядоченные строки, возвращая добавочные pivot-колонки. Несколько
// метрик с одной осью дают один общий набор pivot-колонок (счётчики складываются —
// все метрики оси считают заявки по тому же выражению).
func (s *statisticsService) collectPivotColumns(ctx context.Context, metrics []string, axis models.ReportPivotInfo, req models.ReportRequest, metricRows []models.ReportMetricRow) ([]models.ReportMetricColumn, error) {
	var cells []pivotCell
	for _, m := range metrics {
		plan, perr := buildPivotPlan(m, axis.Key, req.Granularity, req.Filters)
		if perr != nil {
			return nil, perr
		}
		mcells, eerr := s.execPivotPlan(ctx, plan)
		if eerr != nil {
			return nil, eerr
		}
		cells = append(cells, mcells...)
	}
	return applyPivotCells(metricRows, cells, axis.Label), nil
}

// execPivotPlan исполняет cross-tab запрос: GROUP BY (период-бин, ось pivot) ->
// счётчик. period-подпись совпадает с label обычных period-строк (для слияния).
func (s *statisticsService) execPivotPlan(ctx context.Context, plan *pivotPlan) ([]pivotCell, error) {
	selectStr := fmt.Sprintf("%s AS period, %s AS pivot, %s AS count",
		plan.labelExpr, plan.pivotExpr, plan.aggExpr)
	tx := s.db.WithContext(ctx).Table(plan.table).Select(selectStr)
	for _, j := range plan.joins {
		tx = tx.Joins(j)
	}
	for _, w := range plan.wheres {
		tx = tx.Where(w.expr, w.args...)
	}

	cells := make([]pivotCell, 0)
	if err := tx.
		Group(plan.periodExpr + ", " + plan.pivotExpr).
		Scan(&cells).Error; err != nil {
		return nil, fmt.Errorf("statistics: run report pivot: %w", err)
	}
	return cells, nil
}

// execAggregatePlan исполняет один резолвленный план агрегата и возвращает строки
// (подпись разреза + значение метрики). GROUP BY/ORDER применяются только если
// заданы (разрез "none" даёт один итоговый ряд без них).
func (s *statisticsService) execAggregatePlan(ctx context.Context, plan *aggPlan) ([]models.ReportAggregateRow, error) {
	tx := s.db.WithContext(ctx).Table(plan.table).Select(plan.selectStr)
	for _, j := range plan.joins {
		tx = tx.Joins(j)
	}
	for _, w := range plan.wheres {
		tx = tx.Where(w.expr, w.args...)
	}
	if plan.groupExpr != "" {
		tx = tx.Group(plan.groupExpr)
	}
	if plan.orderStr != "" {
		tx = tx.Order(plan.orderStr)
	}

	rows := make([]models.ReportAggregateRow, 0)
	if err := tx.Limit(plan.limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("statistics: run report: %w", err)
	}
	return rows, nil
}

// RunReportList исполняет list-отчёт конструктора (mode=list): выгрузка строк
// сущности с whitelist-столбцами и фильтрами. План собирается чистой buildListPlan
// из whitelist-схем (report_list_engine.go) — ввод сверяется по ключам и
// подставляется только через плейсхолдеры. Невалидный запрос -> ErrInvalidReportRequest
// (400 в handler). Строки сканируются в []map по алиасам столбцов плана.
func (s *statisticsService) RunReportList(ctx context.Context, req models.ReportRequest) (*models.ReportListResponse, error) {
	plan, err := buildListPlan(req)
	if err != nil {
		return nil, err
	}

	tx := s.db.WithContext(ctx).Table(plan.table).Select(plan.selectStr, plan.selectArgs...)
	for _, j := range plan.joins {
		tx = tx.Joins(j)
	}
	for _, w := range plan.wheres {
		tx = tx.Where(w.expr, w.args...)
	}

	rows := make([]map[string]any, 0)
	if err := tx.
		Order(plan.orderStr).
		Limit(plan.limit).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("statistics: run report list: %w", err)
	}

	return &models.ReportListResponse{
		Mode:    "list",
		Entity:  req.Entity,
		Columns: plan.columns,
		Rows:    rows,
		Total:   len(rows),
	}, nil
}
