package services

import (
	"strings"
	"testing"

	"systemburo/internal/models"
)

func TestResolveReportMetrics(t *testing.T) {
	cases := []struct {
		name    string
		req     models.ReportRequest
		want    []string
		wantErr bool
	}{
		{"metrics explicit", models.ReportRequest{Metrics: []string{"applications_count", "items_sum"}}, []string{"applications_count", "items_sum"}, false},
		{"fallback to single metric", models.ReportRequest{Metric: "car_entries_count"}, []string{"car_entries_count"}, false},
		{"metrics override single", models.ReportRequest{Metric: "items_sum", Metrics: []string{"applications_count"}}, []string{"applications_count"}, false},
		{"dedup and trim", models.ReportRequest{Metrics: []string{"applications_count", " applications_count ", "", "  ", "items_sum"}}, []string{"applications_count", "items_sum"}, false},
		{"empty request", models.ReportRequest{}, nil, true},
		{"only blanks", models.ReportRequest{Metrics: []string{"", "   "}}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveReportMetrics(tc.req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ожидалась ошибка, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMergeMetricRows_CombinesMissingAsZero(t *testing.T) {
	metrics := []string{"a", "b"}
	perMetric := map[string][]models.ReportAggregateRow{
		"a": {{Label: "Орг1", Value: 5}, {Label: "Орг2", Value: 3}},
		"b": {{Label: "Орг1", Value: 2}, {Label: "Орг3", Value: 7}},
	}
	rows, totals := mergeMetricRows(metrics, perMetric, "organization", "", 0)

	// default категориальный: по убыванию суммы метрик. Орг1=7, Орг3=7 (tie ->
	// стабильно по первому появлению), Орг2=3.
	wantOrder := []string{"Орг1", "Орг3", "Орг2"}
	if len(rows) != 3 {
		t.Fatalf("ожидалось 3 строки, got %d", len(rows))
	}
	for i, label := range wantOrder {
		if rows[i].Label != label {
			t.Errorf("row[%d]: got %q, want %q", i, rows[i].Label, label)
		}
	}
	// отсутствующая метрика -> 0.
	byLabel := map[string]models.ReportMetricRow{}
	for _, r := range rows {
		byLabel[r.Label] = r
	}
	if byLabel["Орг1"].Values["a"] != 5 || byLabel["Орг1"].Values["b"] != 2 {
		t.Errorf("Орг1: %+v", byLabel["Орг1"].Values)
	}
	if byLabel["Орг2"].Values["b"] != 0 {
		t.Errorf("Орг2 без метрики b должна давать 0, got %d", byLabel["Орг2"].Values["b"])
	}
	if byLabel["Орг3"].Values["a"] != 0 {
		t.Errorf("Орг3 без метрики a должна давать 0, got %d", byLabel["Орг3"].Values["a"])
	}
	if totals["a"] != 8 || totals["b"] != 9 {
		t.Errorf("totals: got %+v, want a=8 b=9", totals)
	}
}

func TestMergeMetricRows_PeriodChronological(t *testing.T) {
	rows, _ := mergeMetricRows(
		[]string{"a"},
		map[string][]models.ReportAggregateRow{
			"a": {{Label: "2026-06-03", Value: 3}, {Label: "2026-06-01", Value: 5}, {Label: "2026-06-02", Value: 1}},
		},
		"period", "", 0,
	)
	want := []string{"2026-06-01", "2026-06-02", "2026-06-03"}
	for i, label := range want {
		if rows[i].Label != label {
			t.Errorf("period row[%d]: got %q, want %q", i, rows[i].Label, label)
		}
	}
}

func TestMergeMetricRows_HourOfDayNumeric(t *testing.T) {
	// численная сортировка: "2" < "9" < "10" (строковая дала бы "10","2","9").
	rows, _ := mergeMetricRows(
		[]string{"a"},
		map[string][]models.ReportAggregateRow{
			"a": {{Label: "9", Value: 1}, {Label: "10", Value: 1}, {Label: "2", Value: 1}},
		},
		"hour_of_day", "", 0,
	)
	want := []string{"2", "9", "10"}
	for i, label := range want {
		if rows[i].Label != label {
			t.Errorf("hour row[%d]: got %q, want %q", i, rows[i].Label, label)
		}
	}
}

func TestMergeMetricRows_LimitAndTotalsOverVisible(t *testing.T) {
	rows, totals := mergeMetricRows(
		[]string{"a"},
		map[string][]models.ReportAggregateRow{
			"a": {{Label: "x", Value: 10}, {Label: "y", Value: 5}, {Label: "z", Value: 1}},
		},
		"organization", "", 2,
	)
	if len(rows) != 2 {
		t.Fatalf("limit 2: got %d строк", len(rows))
	}
	if rows[0].Label != "x" || rows[1].Label != "y" {
		t.Errorf("ожидался top-2 x,y, got %q,%q", rows[0].Label, rows[1].Label)
	}
	// totals считаются по видимым (усечённым) строкам: 10+5, z отброшена.
	if totals["a"] != 15 {
		t.Errorf("totals по видимым строкам: got %d, want 15", totals["a"])
	}
}

func TestMergeMetricRows_ExplicitLabelSort(t *testing.T) {
	rows, _ := mergeMetricRows(
		[]string{"a"},
		map[string][]models.ReportAggregateRow{
			"a": {{Label: "Бета", Value: 1}, {Label: "Альфа", Value: 99}},
		},
		"organization", sortLabelAsc, 0,
	)
	if rows[0].Label != "Альфа" || rows[1].Label != "Бета" {
		t.Errorf("label_asc должен игнорировать значение: got %q,%q", rows[0].Label, rows[1].Label)
	}
}

func TestAggregatePlan_DimensionNone(t *testing.T) {
	req := models.ReportRequest{Metric: "applications_count", Dimension: dimNone}
	plan, err := buildAggregatePlan(req)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if plan.groupExpr != "" {
		t.Errorf("none: ожидался пустой groupExpr, got %q", plan.groupExpr)
	}
	if plan.orderStr != "" {
		t.Errorf("none: ожидался пустой orderStr, got %q", plan.orderStr)
	}
	if !strings.Contains(plan.selectStr, "'Итого' AS label") {
		t.Errorf("none: ожидалась константная подпись, got %q", plan.selectStr)
	}
	if !strings.Contains(plan.selectStr, "COUNT(DISTINCT app.id) AS value") {
		t.Errorf("none: ожидался aggExpr метрики, got %q", plan.selectStr)
	}
}

func TestAggregatePlan_DimensionNoneKeepsFilters(t *testing.T) {
	// Без разреза фильтры по-прежнему применяются (WHERE), даже без GROUP BY.
	req := models.ReportRequest{
		Metric:    "applications_count",
		Dimension: dimNone,
		Filters: []models.ReportFilterValue{
			{Key: "status", Values: []string{models.StatusCompleted}},
		},
	}
	plan, err := buildAggregatePlan(req)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	var found bool
	for _, w := range plan.wheres {
		if strings.Contains(w.expr, "app.status IN") {
			found = true
		}
	}
	if !found {
		t.Errorf("фильтр status должен применяться и без разреза: %+v", plan.wheres)
	}
}
