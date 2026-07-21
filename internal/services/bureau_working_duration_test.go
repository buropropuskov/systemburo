package services

import (
	"strings"
	"testing"
)

// bureauWorkingDuration должна давать вызов SQL-функции рабочих секунд с тем же
// порядком аргументов, что durationBetween (from, to), и календарный фолбэк на
// пустом графике Бюро (#1251 S2). Корректность самого расчёта рабочих секунд
// проверяет DB-тест в internal/handlers (bureau_working_seconds_test.go), обе ветки
// на реальных данных — TestRunReport_DurationMetrics_BureauWorkingTime.
func TestBureauWorkingDuration(t *testing.T) {
	got := bureauWorkingDuration("app.sending_datetime", "app.confirmation_datetime")

	// Рабочая ветка: вызов функции с порядком аргументов from, to.
	if want := "bureau_working_seconds(app.sending_datetime, app.confirmation_datetime)"; !strings.Contains(got, want) {
		t.Fatalf("bureauWorkingDuration = %q, ожидался вызов %q", got, want)
	}
	// Фолбэк: календарная разница (durationBetween, порядок to - from) при пустом графике.
	if want := durationBetween("app.sending_datetime", "app.confirmation_datetime"); !strings.Contains(got, want) {
		t.Fatalf("bureauWorkingDuration = %q, ожидался календарный фолбэк %q", got, want)
	}
	// Условие фолбэка — наличие активных слотов расписания.
	if !strings.Contains(got, "EXISTS") || !strings.Contains(got, "bureau_time_slots") {
		t.Fatalf("bureauWorkingDuration = %q, ожидался гард EXISTS по bureau_time_slots", got)
	}
	// Второй фолбэк: нулевое пересечение с графиком тоже уходит в календарное время,
	// иначе события вне рабочих часов дают «0 секунд» вместо реальной длительности.
	if !strings.Contains(got, "NULLIF") || !strings.Contains(got, "COALESCE") {
		t.Fatalf("bureauWorkingDuration = %q, ожидался COALESCE/NULLIF для нулевого пересечения", got)
	}
}
