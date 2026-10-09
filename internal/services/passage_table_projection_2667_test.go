package services

import (
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm/schema"
)

func TestPassage2667TableEligibilityCalendarAndGrace(t *testing.T) {
	now := time.Date(2035, 12, 31, 21, 0, 0, 0, time.UTC)
	period, err := EntityEffectivePeriodSQL("c", "a")
	if err != nil {
		t.Fatal(err)
	}
	condition, args, err := tablePassageEligibilitySQL("c", "p", period, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || args[0] != "2036-01-01" || !args[1].(time.Time).Equal(now.Add(-5*time.Minute)) {
		t.Fatalf("one captured Moscow calendar/grace clock: %#v", args)
	}
	for _, required := range []string{"c.status=1", "manual_unbounded", "p.action='entry'", "p.previous_action='entry'", "p.closes_unknown_entry", "p.created_at>?"} {
		if !strings.Contains(condition, required) {
			t.Fatalf("missing %s: %s", required, condition)
		}
	}
	if strings.Contains(condition, "table_id") || strings.Contains(condition, "date_removed") {
		t.Fatal("eligibility must stay inside caller scope/removal guards")
	}
}

func TestPassage2667TableProjectionScanAliases(t *testing.T) {
	parsed, err := schema.Parse(&tableCarRow{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	for column, owner := range map[string]string{
		"id": "ID", "passage_id": "Projection", "passage_entity_id": "Projection",
		"passage_created_at": "Projection", "passage_previous_at": "Projection",
		"effective_entry_date_from": "Period", "effective_source": "Period", "effective_valid_mode": "Period",
	} {
		field := parsed.LookUpField(column)
		if field == nil || field.BindNames[0] != owner {
			t.Fatalf("%s mapped to %#v, want %s", column, field, owner)
		}
	}
	selection := tablePassageProjectionSelectSQL("c", "p")
	if !strings.Contains(selection, "c.id AS passage_entity_id") || !strings.Contains(selection, "p.id AS passage_id") {
		t.Fatal("audit ID must not replace entity ID")
	}
}

func TestPassage2667TablePeriodWholeWindow(t *testing.T) {
	from, to, start, end := "2035-12-31", "2036-01-02", "08:00:00", "20:00:00"
	row := tablePassagePeriodRow{EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &start, EntryTimeTo: &end, Source: "individual", Bounded: true, ValidMode: true}
	period := row.effective()
	if period.Source != "individual" || !period.Bounded || period.EntryDateFrom != &from || period.EntryTimeTo != &end {
		t.Fatal("effective DTO must preserve the whole selected window")
	}
	// A reader without an actor never turns territory/cache state into a grant.
	var response TableCarResponse
	if response.Admission.CanEnter || response.Admission.CanExit || response.PassageState.CanCorrect {
		t.Fatal("zero viewer admission must fail closed")
	}
}
