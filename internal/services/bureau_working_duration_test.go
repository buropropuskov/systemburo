package services

import "testing"

// bureauWorkingDuration должна давать вызов SQL-функции с тем же порядком
// аргументов, что durationBetween (from, to). Корректность самого расчёта рабочих
// секунд проверяет DB-тест в internal/handlers (bureau_working_seconds_test.go).
func TestBureauWorkingDuration(t *testing.T) {
	got := bureauWorkingDuration("app.sending_datetime", "app.confirmation_datetime")
	want := "bureau_working_seconds(app.sending_datetime, app.confirmation_datetime)"
	if got != want {
		t.Fatalf("bureauWorkingDuration = %q, want %q", got, want)
	}
}
