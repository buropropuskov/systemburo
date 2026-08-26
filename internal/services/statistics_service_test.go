package services

import (
	"testing"
)

func TestResolveTimelineSource_ValidInputs(t *testing.T) {
	tests := []struct {
		metric      string
		granularity string
		wantTable   string
		wantCol     string
		wantUnit    string
	}{
		{"applications", "day", "applications", "sending_datetime", "day"},
		{"applications", "week", "applications", "sending_datetime", "week"},
		{"applications", "month", "applications", "sending_datetime", "month"},
		// car_entries читает audit_log[car] (#870, F.5 read-switch); источник в
		// carsHistoryUnion, колонки квалифицированы alias'ом ch (как в report_engine).
		{"car_entries", "day", carsHistoryUnion + " ch", "ch.created_at", "day"},
		// people_entries читает audit_log[employee] (#870, F.6 read-switch); источник в
		// employeesHistoryUnion, колонки квалифицированы alias'ом eh (как в report_engine).
		{"people_entries", "week", employeesHistoryUnion + " eh", "eh.created_at", "week"},
	}

	for _, tc := range tests {
		t.Run(tc.metric+"/"+tc.granularity, func(t *testing.T) {
			src, unit, err := resolveTimelineSource(tc.metric, tc.granularity)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if src.table != tc.wantTable {
				t.Errorf("table: got %q, want %q", src.table, tc.wantTable)
			}
			if src.tsColumn != tc.wantCol {
				t.Errorf("tsColumn: got %q, want %q", src.tsColumn, tc.wantCol)
			}
			if unit != tc.wantUnit {
				t.Errorf("unit: got %q, want %q", unit, tc.wantUnit)
			}
		})
	}
}

func TestResolveTimelineSource_InvalidInputs(t *testing.T) {
	tests := []struct {
		metric      string
		granularity string
		desc        string
	}{
		{"unknown_metric", "day", "неизвестный metric"},
		{"applications", "hour", "неизвестная granularity"},
		{"'; DROP TABLE applications; --", "day", "SQL-инъекция в metric"},
		{"applications", "day; DROP TABLE cars", "SQL-инъекция в granularity"},
		{"", "day", "пустой metric"},
		{"applications", "", "пустая granularity"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			_, _, err := resolveTimelineSource(tc.metric, tc.granularity)
			if err == nil {
				t.Errorf("ожидалась ошибка для metric=%q granularity=%q, но её нет", tc.metric, tc.granularity)
			}
		})
	}
}
