package database

import (
	"strings"
	"testing"
)

// TestEnsureUTCTimezone_AddsParameterWhenMissing проверяет, что TimeZone=UTC
// добавляется к URL-DSN, если он не задан.
func TestEnsureUTCTimezone_AddsParameterWhenMissing(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "URL без параметров",
			dsn:  "postgres://user:pass@host/db",
			want: "TimeZone=UTC",
		},
		{
			name: "URL с sslmode",
			dsn:  "postgres://user:pass@host/db?sslmode=disable",
			want: "TimeZone=UTC",
		},
		{
			name: "postgresql:// схема",
			dsn:  "postgresql://user:pass@host/db?sslmode=disable",
			want: "TimeZone=UTC",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EnsureUTCTimezone(tc.dsn)
			if !strings.Contains(got, tc.want) {
				t.Errorf("EnsureUTCTimezone(%q) = %q, want substring %q", tc.dsn, got, tc.want)
			}
		})
	}
}

// TestEnsureUTCTimezone_KeepsExistingTimezone проверяет, что параметр TimeZone
// не перезаписывается, если уже задан.
func TestEnsureUTCTimezone_KeepsExistingTimezone(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
	}{
		{
			name: "URL с TimeZone=Asia/Almaty",
			dsn:  "postgres://user:pass@host/db?TimeZone=Asia/Almaty",
		},
		{
			name: "URL с TimeZone=Europe/Moscow и другими параметрами",
			dsn:  "postgres://user:pass@host/db?sslmode=disable&TimeZone=Europe/Moscow",
		},
		{
			name: "keyword DSN с TimeZone",
			dsn:  "host=db user=postgres dbname=test TimeZone=Europe/London",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EnsureUTCTimezone(tc.dsn)
			if strings.Contains(got, "TimeZone=UTC") {
				t.Errorf("EnsureUTCTimezone(%q) wrongly added UTC, got %q", tc.dsn, got)
			}
		})
	}
}

// TestEnsureUTCTimezone_KeywordDSN проверяет добавление к keyword-DSN.
func TestEnsureUTCTimezone_KeywordDSN(t *testing.T) {
	dsn := "host=db user=postgres dbname=test sslmode=disable"
	got := EnsureUTCTimezone(dsn)
	if !strings.Contains(got, "TimeZone=UTC") {
		t.Errorf("EnsureUTCTimezone(%q) = %q, want substring TimeZone=UTC", dsn, got)
	}
}

// TestEnsureUTCTimezone_EmptyInput проверяет, что пустой DSN возвращается как есть.
func TestEnsureUTCTimezone_EmptyInput(t *testing.T) {
	if got := EnsureUTCTimezone(""); got != "" {
		t.Errorf("EnsureUTCTimezone(\"\") = %q, want empty", got)
	}
}

// TestEnsureUTCTimezone_PreservesExistingParams проверяет, что добавление
// TimeZone не ломает другие query-параметры.
func TestEnsureUTCTimezone_PreservesExistingParams(t *testing.T) {
	dsn := "postgres://user:pass@host/db?sslmode=disable&pool_max_conns=10"
	got := EnsureUTCTimezone(dsn)
	for _, must := range []string{"sslmode=disable", "pool_max_conns=10", "TimeZone=UTC"} {
		if !strings.Contains(got, must) {
			t.Errorf("EnsureUTCTimezone(%q) lost parameter %q: %q", dsn, must, got)
		}
	}
}
