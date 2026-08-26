package services

import (
	"errors"
	"strings"
	"testing"

	"systemburo/internal/models"
)

// TestAggregateEngine_CoversCatalogDimensions гарантирует, что движок умеет
// собрать план для КАЖДОГО разреза, заявленного каталогом B1 для метрики.
// Иначе UI предложит комбинацию, которая упадёт 500 на исполнении.
func TestAggregateEngine_CoversCatalogDimensions(t *testing.T) {
	for _, metric := range reportMetricOrder {
		for _, dim := range reportMetricRegistry[metric].dimensions {
			req := models.ReportRequest{Metric: metric, Dimension: dim}
			plan, err := buildAggregatePlan(req)
			if err != nil {
				t.Errorf("metric %q dim %q: движок не собрал план: %v", metric, dim, err)
				continue
			}
			if plan.groupExpr == "" || plan.selectStr == "" {
				t.Errorf("metric %q dim %q: пустой groupExpr/selectStr", metric, dim)
			}
			if !strings.Contains(plan.selectStr, "AS label") || !strings.Contains(plan.selectStr, "AS value") {
				t.Errorf("metric %q dim %q: некорректный select %q", metric, dim, plan.selectStr)
			}
		}
	}
}

// TestAggregateEngine_DimsPublishedInCatalog — обратная сторона
// CoversCatalogDimensions: разрез, который движок умеет резолвить для метрики,
// обязан быть заявлен каталогом. Иначе запись в dims мертва — buildAggregatePlan
// сверяется с каталогом и отобьёт разрез раньше, чем дойдёт до схемы, а автор
// схемы будет уверен, что разрез работает. period/hour_of_day/none строятся по
// tsColumn, а не из dims, поэтому в этой проверке не участвуют.
func TestAggregateEngine_DimsPublishedInCatalog(t *testing.T) {
	for metric, schema := range aggMetricRegistry {
		published := reportMetricRegistry[metric].dimensions
		for dim := range schema.dims {
			if !contains(published, dim) {
				t.Errorf("metric %q: движок резолвит разрез %q, но каталог его не публикует — запись в dims мертва",
					metric, dim)
			}
		}
	}
}

// TestAggregateEngine_AggExprMatchesCatalog сверяет источники агрегата: каждая
// метрика каталога (B1) должна иметь схему исполнения (B2) и наоборот.
func TestAggregateEngine_AggExprMatchesCatalog(t *testing.T) {
	for _, metric := range reportMetricOrder {
		if _, ok := aggMetricRegistry[metric]; !ok {
			t.Errorf("метрика каталога %q без схемы исполнения", metric)
		}
	}
	for metric := range aggMetricRegistry {
		if _, ok := reportMetricRegistry[metric]; !ok {
			t.Errorf("схема исполнения %q без метрики в каталоге", metric)
		}
	}
}

func TestAggregatePlan_FiltersAndJoins(t *testing.T) {
	req := models.ReportRequest{
		Metric:    "applications_count",
		Dimension: "organization",
		Filters: []models.ReportFilterValue{
			{Key: "status", Values: []string{models.StatusCompleted, models.StatusInWork}},
		},
	}
	plan, err := buildAggregatePlan(req)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}

	// dim organization -> joinBlock (org/comp) добавлен.
	joined := strings.Join(plan.joins, " ")
	if !strings.Contains(joined, "organizations org") || !strings.Contains(joined, "companies comp") {
		t.Errorf("ожидался join org/company, got %v", plan.joins)
	}
	// attachments join НЕ нужен (ни разрез, ни фильтр его не требуют).
	if strings.Contains(joined, "attachments att") {
		t.Errorf("лишний join вложений: %v", plan.joins)
	}
	// фильтр статуса -> WHERE app.status IN c аргументом-срезом.
	var found bool
	for _, w := range plan.wheres {
		if strings.Contains(w.expr, "app.status IN") {
			found = true
			if len(w.args) != 1 {
				t.Errorf("ожидался 1 аргумент-срез, got %v", w.args)
			}
		}
	}
	if !found {
		t.Errorf("фильтр status не попал в where: %+v", plan.wheres)
	}
	// COUNT(DISTINCT app.id) защищает от двойного счёта при join'ах.
	if !strings.Contains(plan.selectStr, "COUNT(DISTINCT app.id)") {
		t.Errorf("ожидался COUNT(DISTINCT app.id), got %q", plan.selectStr)
	}
}

func TestAggregatePlan_AttachmentTypeNeedsAttachJoin(t *testing.T) {
	req := models.ReportRequest{Metric: "applications_count", Dimension: "attachment_type"}
	plan, err := buildAggregatePlan(req)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	joined := strings.Join(plan.joins, " ")
	if !strings.Contains(joined, "attachments att") || !strings.Contains(joined, "unique_attachments ua") {
		t.Errorf("ожидался attach-join, got %v", plan.joins)
	}
	// org/company join не нужен для одного только attachment_type.
	if strings.Contains(joined, "organizations org") {
		t.Errorf("лишний join org: %v", plan.joins)
	}
}

func TestAggregatePlan_PeriodAndDateRange(t *testing.T) {
	req := models.ReportRequest{
		Metric:      "car_entries_count",
		Dimension:   "period",
		Granularity: "week",
		Filters: []models.ReportFilterValue{
			{Key: "date_range", From: "2026-06-01", To: "2026-06-17"},
		},
	}
	plan, err := buildAggregatePlan(req)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !strings.Contains(plan.groupExpr, "date_trunc('week', (ch.created_at AT TIME ZONE 'Europe/Moscow'))") {
		t.Errorf("ожидался date_trunc week в МСК, got %q", plan.groupExpr)
	}
	// baseWhere метрики (entry) присутствует.
	var hasBase, hasFrom, hasTo bool
	for _, w := range plan.wheres {
		if strings.Contains(w.expr, "ch.action_type = 'entry'") {
			hasBase = true
		}
		if strings.Contains(w.expr, "ch.created_at >= ?") {
			hasFrom = true
		}
		if strings.Contains(w.expr, "ch.created_at <= ?") {
			hasTo = true
		}
	}
	if !hasBase || !hasFrom || !hasTo {
		t.Errorf("base/from/to: %v/%v/%v wheres=%+v", hasBase, hasFrom, hasTo, plan.wheres)
	}
	// период по умолчанию сортируется хронологически (по groupExpr ASC).
	if !strings.HasSuffix(plan.orderStr, "ASC") || !strings.Contains(plan.orderStr, "date_trunc") {
		t.Errorf("ожидалась хронологическая сортировка, got %q", plan.orderStr)
	}
	// period без фильтров/разрезов с join не требует join'ов (ts на базовой таблице).
	if len(plan.joins) != 0 {
		t.Errorf("для car period join не нужен, got %v", plan.joins)
	}
}

func TestAggregatePlan_HourOfDayAndValueSort(t *testing.T) {
	req := models.ReportRequest{Metric: "people_entries_count", Dimension: "hour_of_day", Sort: sortValueDesc}
	plan, err := buildAggregatePlan(req)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !strings.Contains(plan.groupExpr, "EXTRACT(HOUR FROM (eh.created_at AT TIME ZONE 'Europe/Moscow'))") {
		t.Errorf("ожидался EXTRACT HOUR в МСК, got %q", plan.groupExpr)
	}
	if plan.orderStr != "COUNT(*) DESC" {
		t.Errorf("ожидался ORDER BY COUNT(*) DESC, got %q", plan.orderStr)
	}
}

func TestAggregatePlan_ItemsSumAlwaysJoins(t *testing.T) {
	req := models.ReportRequest{Metric: "items_sum", Dimension: "period"}
	plan, err := buildAggregatePlan(req)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	// tsColumn items_sum (app.sending_datetime) лежит за join'ом -> join обязателен.
	if len(plan.joins) == 0 || !strings.Contains(strings.Join(plan.joins, " "), "applications app") {
		t.Errorf("items_sum period требует join к applications, got %v", plan.joins)
	}
	if !strings.Contains(plan.selectStr, "SUM(items.count)") {
		t.Errorf("ожидался SUM(items.count), got %q", plan.selectStr)
	}
}

func TestAggregatePlan_ItemsSumDateRangeFilterJoins(t *testing.T) {
	// items_sum: tsColumn=app.sending_datetime лежит за join'ом. Фильтр date_range
	// должен тянуть joinBlock через свой tsJoin даже без разреза period.
	req := models.ReportRequest{
		Metric:    "items_sum",
		Dimension: "organization",
		Filters: []models.ReportFilterValue{
			{Key: "date_range", From: "2026-06-01", To: "2026-06-17"},
		},
	}
	plan, err := buildAggregatePlan(req)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	joined := strings.Join(plan.joins, " ")
	if !strings.Contains(joined, "applications app") {
		t.Errorf("date_range фильтр items_sum должен тянуть join к applications, got %v", plan.joins)
	}
	var hasRange bool
	for _, w := range plan.wheres {
		if strings.Contains(w.expr, "app.sending_datetime >= ?") {
			hasRange = true
		}
	}
	if !hasRange {
		t.Errorf("date_range по app.sending_datetime не попал в where: %+v", plan.wheres)
	}
}

func TestAggregatePlan_RejectsInvalid(t *testing.T) {
	cases := []struct {
		name string
		req  models.ReportRequest
	}{
		{"unknown metric", models.ReportRequest{Metric: "nope", Dimension: "status"}},
		{"empty dimension", models.ReportRequest{Metric: "applications_count"}},
		{"dimension not in catalog", models.ReportRequest{Metric: "applications_count", Dimension: "hour_of_day"}},
		{"unknown filter", models.ReportRequest{Metric: "applications_count", Dimension: "status",
			Filters: []models.ReportFilterValue{{Key: "citizenship", Values: []string{"РФ"}}}}},
		{"unreachable filter", models.ReportRequest{Metric: "items_sum", Dimension: "organization",
			Filters: []models.ReportFilterValue{{Key: "status", Values: []string{models.StatusCompleted}}}}},
		{"bad granularity", models.ReportRequest{Metric: "applications_count", Dimension: "period", Granularity: "year"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildAggregatePlan(tc.req)
			if err == nil {
				t.Fatalf("ожидалась ошибка валидации")
			}
			if !errors.Is(err, ErrInvalidReportRequest) {
				t.Errorf("ожидался ErrInvalidReportRequest, got %v", err)
			}
		})
	}
}

func TestAggregatePlan_EmptyFilterSkipped(t *testing.T) {
	req := models.ReportRequest{
		Metric:    "applications_count",
		Dimension: "status",
		Filters: []models.ReportFilterValue{
			{Key: "organization", Values: []string{"", "  "}}, // только пустые -> пропустить
			{Key: "date_range"}, // без границ -> пропустить
		},
	}
	plan, err := buildAggregatePlan(req)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	// пустые фильтры не дают ни WHERE, ни join'ов.
	if len(plan.wheres) != 0 {
		t.Errorf("пустые фильтры не должны давать WHERE: %+v", plan.wheres)
	}
	if len(plan.joins) != 0 {
		t.Errorf("пустые фильтры не должны тянуть join'ы: %v", plan.joins)
	}
}

func TestClampLimit(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, defaultReportLimit},
		{-5, defaultReportLimit},
		{50, 50},
		{maxReportLimit + 1, maxReportLimit},
	}
	for _, c := range cases {
		if got := clampLimit(c.in); got != c.want {
			t.Errorf("clampLimit(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}
