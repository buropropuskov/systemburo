package services

import (
	"strings"
	"testing"
)

// These are SQL construction contracts, not evidence of DB lifecycle behavior.
func TestEntityPeriod2665RestoreSQLUsesWholeEffectiveWindow(t *testing.T) {
	for _, alias := range []string{"c", "e", "emp"} {
		t.Run(alias, func(t *testing.T) {
			period, err := EntityEffectivePeriodSQL(alias, "a")
			if err != nil {
				t.Fatal(err)
			}
			got, err := restorableEntityPeriodSQL(alias)
			if err != nil {
				t.Fatal(err)
			}
			until := strings.NewReplacer(
				"a.entry_date_to", period.DateTo,
				"a.entry_time_to", period.TimeTo,
			).Replace(passValidNowSQL("a"))
			want := period.ValidMode + " AND (" + period.Source + " <> 'individual' OR " + period.Bounded + ") AND " + until
			if got != want {
				t.Fatalf("restore SQL does not preserve effective mode and legacy end semantics:\ngot %s\nwant %s", got, want)
			}
			if !strings.Contains(got, moscowNowSQL) || !strings.Contains(got, "TIME '23:59:59'") {
				t.Fatal("Moscow clock or legacy optional end-clock fallback was lost")
			}
			if strings.Contains(got, "CURRENT_DATE") || strings.Contains(got, "{{") {
				t.Fatal("restore SQL contains a UTC-day shortcut or an unresolved marker")
			}
		})
	}
}

func TestEntityPeriod2665RestoreSQLRejectsUnsafeAlias(t *testing.T) {
	for _, alias := range []string{"", "c; SELECT 1"} {
		got, err := restorableEntityPeriodSQL(alias)
		if err == nil || got != "" {
			t.Fatalf("unsafe alias must fail closed: %q, %v", got, err)
		}
	}
}
