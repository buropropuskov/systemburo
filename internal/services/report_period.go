package services

import (
	"context"
	"fmt"

	"systemburo/internal/models"
)

// periodPlan — план запроса границ дат отчёта. Ось времени берём ровно ту, по
// которой сужает фильтр date_range: поля периода в мастере должны показывать
// границы того, что фильтр реально ограничивает, а не первую попавшуюся дату в
// таблице результата (#2341).
type periodPlan struct {
	table    string
	joins    []string
	wheres   []whereClause
	tsColumn string
}

// buildReportPeriodPlan собирает план границ по тем же whitelist-схемам, что и сам
// отчёт. nil без ошибки — у отчёта нет оси времени (сущность без даты): границ не
// существует, и это не ошибка запроса.
func buildReportPeriodPlan(req models.ReportRequest) (*periodPlan, error) {
	if req.Mode == "list" {
		exec, ok := listExecRegistry[req.Entity]
		if !ok {
			return nil, errInvalidReport("entity")
		}
		if exec.tsColumn == "" {
			return nil, nil
		}
		// joins нужны целиком: baseWhere сущности ссылается на присоединённые
		// таблицы (у заявок на работы — на вид вложения из unique_attachments).
		plan := &periodPlan{
			table:    exec.base,
			joins:    append([]string{}, exec.joins...),
			tsColumn: exec.tsColumn,
		}
		if exec.baseWhere.expr != "" {
			plan.wheres = append(plan.wheres, exec.baseWhere)
		}
		return plan, nil
	}

	metric := req.Metric
	if metric == "" && len(req.Metrics) > 0 {
		metric = req.Metrics[0]
	}
	schema, ok := aggMetricRegistry[metric]
	if !ok {
		return nil, errInvalidReport("metric")
	}
	if schema.tsColumn == "" {
		return nil, nil
	}
	plan := &periodPlan{table: schema.base, tsColumn: schema.tsColumn}
	// У нынешних метрик ось и baseWhere живут в базовой таблице, join не нужен;
	// схема с tsJoin оставлена рабочей, чтобы новая метрика не считала границы по
	// неполному запросу.
	switch schema.tsJoin {
	case jChain:
		plan.joins = append(plan.joins, schema.joinBlock...)
	case jAttach:
		plan.joins = append(plan.joins, schema.attachJoin...)
	}
	if schema.baseWhere != "" {
		plan.wheres = append(plan.wheres, whereClause{expr: schema.baseWhere})
	}
	return plan, nil
}

// ReportDataPeriod отдаёт границы дат, доступные отчёту: самую раннюю и самую
// позднюю дату по его оси времени. Ими мастер разворачивает пресет «Весь период» в
// конкретный диапазон — пустые поля читались как «фильтр не применился» (#2341).
// Данных нет или у отчёта нет оси времени — пустые границы, не ошибка.
func (s *statisticsService) ReportDataPeriod(ctx context.Context, req models.ReportRequest) (*models.ReportDataPeriod, error) {
	plan, err := buildReportPeriodPlan(req)
	if err != nil {
		return nil, err
	}
	if plan == nil {
		return &models.ReportDataPeriod{}, nil
	}

	ts := tzColumn(plan.tsColumn)
	var row struct {
		FromTS *string
		ToTS   *string
	}
	tx := s.db.WithContext(ctx).Table(plan.table).Select(fmt.Sprintf(
		"to_char(MIN(%[1]s), 'YYYY-MM-DD') AS from_ts, to_char(MAX(%[1]s), 'YYYY-MM-DD') AS to_ts", ts))
	for _, j := range plan.joins {
		tx = tx.Joins(j)
	}
	for _, w := range plan.wheres {
		tx = tx.Where(w.expr, w.args...)
	}
	if err := tx.Take(&row).Error; err != nil {
		return nil, fmt.Errorf("statistics: report data period: %w", err)
	}

	res := &models.ReportDataPeriod{}
	if row.FromTS != nil {
		res.From = *row.FromTS
	}
	if row.ToTS != nil {
		res.To = *row.ToTS
	}
	return res, nil
}
