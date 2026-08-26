package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
)

func TestResolvePivot(t *testing.T) {
	cases := []struct {
		name    string
		pivot   string
		dim     string
		metrics []string
		wantOn  bool
		wantErr bool
	}{
		{"empty pivot -> off, no error", "", "period", []string{"applications_count"}, false, false},
		{"valid attachment_type on period", pivotAttachmentType, "period", []string{"applications_count"}, true, false},
		{"pivot with non-period dimension -> error", pivotAttachmentType, "organization", []string{"applications_count"}, false, true},
		{"unknown pivot axis -> error", "nope", "period", []string{"applications_count"}, false, true},
		{"pivot not applicable to metric -> error", pivotAttachmentType, "period", []string{"car_entries_count"}, false, true},
		{"pivot applicable to all metrics required", pivotAttachmentType, "period", []string{"applications_count", "items_sum"}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, on, err := resolvePivot(tc.pivot, tc.dim, tc.metrics)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ожидалась ошибка валидации")
				}
				if !errors.Is(err, ErrInvalidReportRequest) {
					t.Errorf("ожидался ErrInvalidReportRequest, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
			if on != tc.wantOn {
				t.Errorf("pivotOn: got %v, want %v", on, tc.wantOn)
			}
		})
	}
}

func TestBuildPivotPlan_PeriodAndAttachJoin(t *testing.T) {
	plan, err := buildPivotPlan("applications_count", pivotAttachmentType, "week",
		[]models.ReportFilterValue{{Key: "date_range", From: "2026-06-01", To: "2026-06-17"}})
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if !strings.Contains(plan.periodExpr, "date_trunc('week', app.sending_datetime)") {
		t.Errorf("ожидался period week, got %q", plan.periodExpr)
	}
	if !strings.Contains(plan.pivotExpr, "att.attachment_display_name") {
		t.Errorf("ожидалось выражение типа вложения, got %q", plan.pivotExpr)
	}
	// attach-join обязателен (выражение оси лежит за вложениями).
	joined := strings.Join(plan.joins, " ")
	if !strings.Contains(joined, "attachments att") || !strings.Contains(joined, "unique_attachments ua") {
		t.Errorf("ожидался attach-join, got %v", plan.joins)
	}
	if !strings.Contains(plan.aggExpr, "COUNT(DISTINCT app.id)") {
		t.Errorf("ожидался COUNT(DISTINCT app.id), got %q", plan.aggExpr)
	}
	// date_range -> WHERE по sending_datetime.
	var hasFrom bool
	for _, w := range plan.wheres {
		if strings.Contains(w.expr, "app.sending_datetime >= ?") {
			hasFrom = true
		}
	}
	if !hasFrom {
		t.Errorf("date_range не попал в where: %+v", plan.wheres)
	}
}

func TestBuildPivotPlan_RejectsBadAxisOrGranularity(t *testing.T) {
	cases := []struct {
		name        string
		metric      string
		pivot       string
		granularity string
	}{
		{"unknown metric", "nope", pivotAttachmentType, "week"},
		{"axis not a dim of metric", "items_sum", pivotAttachmentType, "week"},
		{"bad granularity", "applications_count", pivotAttachmentType, "year"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildPivotPlan(tc.metric, tc.pivot, tc.granularity, nil)
			if err == nil {
				t.Fatalf("ожидалась ошибка")
			}
			if !errors.Is(err, ErrInvalidReportRequest) {
				t.Errorf("ожидался ErrInvalidReportRequest, got %v", err)
			}
		})
	}
}

func TestApplyPivotCells_BuildsColumnsAndFillsRows(t *testing.T) {
	rows := []models.ReportMetricRow{
		{Label: "2026-06-01", Values: map[string]int64{"applications_count": 5}},
		{Label: "2026-06-08", Values: map[string]int64{"applications_count": 3}},
	}
	cells := []pivotCell{
		{Period: "2026-06-01", Pivot: "Люди", Count: 2},
		{Period: "2026-06-01", Pivot: "Машины", Count: 3},
		{Period: "2026-06-08", Pivot: "Машины", Count: 3}, // в этом бине только машины
		{Period: "2026-07-01", Pivot: "Люди", Count: 9},   // бин вне видимых строк -> игнор
	}
	cols, colTotals := applyPivotCells(rows, cells, "Вложения")

	// Колонки: Машины (итог 6) перед Люди (итог 2) — по убыванию суммы.
	if len(cols) != 2 {
		t.Fatalf("ожидалось 2 pivot-колонки, got %d (%+v)", len(cols), cols)
	}
	if cols[0].Key != pivotColumnPrefix+"Машины" || cols[1].Key != pivotColumnPrefix+"Люди" {
		t.Errorf("порядок колонок по убыванию суммы нарушен: %q,%q", cols[0].Key, cols[1].Key)
	}
	if cols[0].Label != "Вложения: Машины" || cols[0].Kind != models.ReportColumnPivot {
		t.Errorf("ожидался label/kind pivot-колонки, got %q/%q", cols[0].Label, cols[0].Kind)
	}

	// Итоги pivot-колонок (для строки «Итого») = суммы по видимым строкам, по ключу
	// колонки. Бин 2026-07-01 (вне видимых строк) в итоги не входит: Машины 3+3=6, Люди 2.
	if colTotals[pivotColumnPrefix+"Машины"] != 6 {
		t.Errorf("итог Машины: ожидалось 6, got %d", colTotals[pivotColumnPrefix+"Машины"])
	}
	if colTotals[pivotColumnPrefix+"Люди"] != 2 {
		t.Errorf("итог Люди: ожидалось 2, got %d", colTotals[pivotColumnPrefix+"Люди"])
	}

	// Метрика осталась нетронутой, pivot-значения вписаны, отсутствующие -> 0.
	r0 := rows[0]
	if r0.Values["applications_count"] != 5 {
		t.Errorf("метрика не должна меняться, got %d", r0.Values["applications_count"])
	}
	if r0.Values[pivotColumnPrefix+"Люди"] != 2 || r0.Values[pivotColumnPrefix+"Машины"] != 3 {
		t.Errorf("ячейки бина 06-01: %+v", r0.Values)
	}
	r1 := rows[1]
	if r1.Values[pivotColumnPrefix+"Машины"] != 3 {
		t.Errorf("бин 06-08 машины: got %d", r1.Values[pivotColumnPrefix+"Машины"])
	}
	if v, ok := r1.Values[pivotColumnPrefix+"Люди"]; !ok || v != 0 {
		t.Errorf("отсутствующая pivot-колонка должна давать явный 0, got %v ok=%v", v, ok)
	}
}

func TestWindowDays(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
		want     float64
	}{
		{"single day", "2026-06-01", "2026-06-01", 1},
		{"seventeen days inclusive", "2026-06-01", "2026-06-17", 17},
		{"full week", "2026-06-01", "2026-06-07", 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, _ := parseReportDate(tc.from, false)
			to, _ := parseReportDate(tc.to, true)
			if got := windowDays(from, to, true, true); got != tc.want {
				t.Errorf("windowDays = %v, want %v", got, tc.want)
			}
		})
	}
	if got := windowDays(time.Time{}, time.Time{}, false, false); got != 1 {
		t.Errorf("без границ окна windowDays должен быть 1, got %v", got)
	}
}

func TestBinDays_FullAndPartial(t *testing.T) {
	from, _ := parseReportDate("2026-06-03", false) // среда
	to, _ := parseReportDate("2026-06-17", true)

	cases := []struct {
		name     string
		binStart string
		unit     string
		want     float64
	}{
		// Неделя ISO начинается с понедельника. Бин 06-01..06-07, окно с 06-03 ->
		// пересечение 06-03..06-07 = 5 дней (крайний неполный бин).
		{"partial leading week", "2026-06-01", "week", 5},
		// Полная средняя неделя 06-08..06-14 целиком в окне -> 7.
		{"full middle week", "2026-06-08", "week", 7},
		// Бин 06-15..06-21, окно до 06-17 включительно -> 06-15..06-17 = 3 дня.
		{"partial trailing week", "2026-06-15", "week", 3},
		// Гранулярность day -> всегда 1.
		{"day bin", "2026-06-10", "day", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binStart, _ := parseReportDate(tc.binStart, false)
			if got := binDays(binStart, tc.unit, from, to, true, true); got != tc.want {
				t.Errorf("binDays(%s,%s) = %v, want %v", tc.binStart, tc.unit, got, tc.want)
			}
		})
	}
}

func TestApplyAvgPerDay_PeriodWeekly(t *testing.T) {
	// entries по неделям, окно 2026-06-03..2026-06-17.
	rows := []models.ReportMetricRow{
		{Label: "2026-06-01", Values: map[string]int64{"avg_cars_per_day": 10}}, // 5 дней пересечения -> 2.0
		{Label: "2026-06-08", Values: map[string]int64{"avg_cars_per_day": 14}}, // 7 дней -> 2.0
		{Label: "2026-06-15", Values: map[string]int64{"avg_cars_per_day": 9}},  // 3 дня -> 3.0
	}
	filters := []models.ReportFilterValue{{Key: "date_range", From: "2026-06-03", To: "2026-06-17"}}

	total := applyAvgPerDay(rows, "avg_cars_per_day", "period", "week", filters)

	wantFloat := []float64{2.0, 2.0, 3.0}
	for i, w := range wantFloat {
		if rows[i].FloatValues["avg_cars_per_day"] != w {
			t.Errorf("row[%d] float: got %v, want %v", i, rows[i].FloatValues["avg_cars_per_day"], w)
		}
		// целое значение метрики убрано из Values (метрика дробная).
		if _, ok := rows[i].Values["avg_cars_per_day"]; ok {
			t.Errorf("row[%d]: целое значение должно быть удалено из Values", i)
		}
	}
	// Итог = суммарный счётчик (10+14+9=33) / число дней окна (15) = 2.2.
	if total != 2.2 {
		t.Errorf("total avg: got %v, want 2.2", total)
	}
}

func TestApplyAvgPerDay_NoneWholeWindow(t *testing.T) {
	rows := []models.ReportMetricRow{
		{Label: "Итого", Values: map[string]int64{"avg_cars_per_day": 30}},
	}
	filters := []models.ReportFilterValue{{Key: "date_range", From: "2026-06-01", To: "2026-06-10"}}

	total := applyAvgPerDay(rows, "avg_cars_per_day", dimNone, "", filters)

	// 30 entries / 10 дней = 3.0.
	if rows[0].FloatValues["avg_cars_per_day"] != 3.0 {
		t.Errorf("none avg: got %v, want 3.0", rows[0].FloatValues["avg_cars_per_day"])
	}
	if total != 3.0 {
		t.Errorf("none total: got %v, want 3.0", total)
	}
}
