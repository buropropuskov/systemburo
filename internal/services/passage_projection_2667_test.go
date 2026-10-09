package services

import (
	"strings"
	"systemburo/internal/models"
	"testing"
	"time"
)

func passage2667String(value string) *string { return &value }
func passage2667Int(value int) *int          { return &value }
func passage2667Period() models.EffectivePeriod {
	return models.EffectivePeriod{EntryPeriod: models.EntryPeriod{EntryDateFrom: passage2667String("2028-12-31"), EntryDateTo: passage2667String("2029-01-02"), EntryTimeFrom: passage2667String("08:00"), EntryTimeTo: passage2667String("22:00")}, Bounded: true, Source: "individual"}
}

func TestPassage2667WholeMoscowWindow(t *testing.T) {
	p := passage2667Period()
	for _, tc := range []struct {
		at      string
		allowed bool
		reason  string
	}{
		{"2028-12-31T04:59:59Z", false, "not_started"},
		{"2028-12-31T05:00:00Z", true, ""},
		// A multi-day permit is continuous, not repeated daily opening hours.
		{"2029-01-01T01:00:00Z", true, ""},
		{"2029-01-02T18:59:59Z", true, ""},
		{"2029-01-02T19:00:00Z", false, "expired"},
	} {
		t.Run(tc.at, func(t *testing.T) {
			now, err := time.Parse(time.RFC3339, tc.at)
			if err != nil {
				t.Fatal(err)
			}
			got, reason := passageWindow(p, now)
			if got != tc.allowed || reason != tc.reason {
				t.Fatalf("allowed=%v reason=%q", got, reason)
			}
		})
	}
	p.EntryTimeFrom = nil
	p.EntryTimeTo = nil
	now := time.Date(2029, 1, 2, 20, 59, 59, 999999999, time.UTC)
	if ok, _ := passageWindow(p, now); !ok {
		t.Fatal("date-only final fractional second must remain valid")
	}
	if ok, reason := passageWindow(p, now.Add(time.Nanosecond)); ok || reason != "expired" {
		t.Fatal("date-only end is exclusive next Moscow midnight")
	}
	p.Source = "manual_unbounded"
	p.EntryPeriod = models.EntryPeriod{}
	if ok, _ := passageWindow(p, now); !ok {
		t.Fatal("manual unbounded")
	}
	p.Source = "attachment"
	if ok, reason := passageWindow(p, now); ok || reason != "invalid_period" {
		t.Fatal("missing nonmanual period must fail closed")
	}
}

func TestPassage2667ProjectionUnknownAndGrace(t *testing.T) {
	now := time.Date(2029, 1, 1, 12, 0, 0, 0, time.UTC)
	unknown, err := projectPassageRow(passageProjectionRow{TerritoryStatus: passage2667Int(1)}, now)
	if err != nil || !unknown.Open || unknown.EntryTimeKnown || unknown.NeedsAttention {
		t.Fatalf("unknown: %+v %v", unknown, err)
	}
	exitedAt := now.Add(-5 * time.Minute)
	row := passageProjectionRow{ID: 5, Action: "exit", CreatedAt: &exitedAt, ClosesUnknownEntry: true}
	before, err := projectPassageRow(row, now.Add(-time.Nanosecond))
	if err != nil || !before.InExitGrace || before.Open {
		t.Fatalf("before grace: %+v %v", before, err)
	}
	exact, err := projectPassageRow(row, now)
	if err != nil || exact.InExitGrace {
		t.Fatalf("exclusive grace: %+v %v", exact, err)
	}
	row.ClosesUnknownEntry = false
	orphan, err := projectPassageRow(row, now.Add(-time.Minute))
	if err != nil || orphan.GraceUntil != nil {
		t.Fatalf("orphan: %+v %v", orphan, err)
	}
	row.Action = PassageCorrectionAction
	corrected, err := projectPassageRow(row, now.Add(-time.Minute))
	if err != nil || !corrected.InExitGrace {
		t.Fatalf("correction: %+v %v", corrected, err)
	}
}

func TestPassage2667AttentionBoundaryAndPreviousPassage(t *testing.T) {
	now := time.Date(2029, 1, 1, 12, 0, 0, 0, time.UTC)
	entered := now.Add(-48 * time.Hour)
	row := passageProjectionRow{ID: 4, Action: "entry", CreatedAt: &entered, TableID: passage2667Int(9)}
	exact, err := projectPassageRow(row, now)
	if err != nil || exact.NeedsAttention || !exact.EntryTimeKnown || exact.EntryTableID == nil || *exact.EntryTableID != 9 {
		t.Fatalf("exact: %+v %v", exact, err)
	}
	later, err := projectPassageRow(row, now.Add(time.Nanosecond))
	if err != nil || !later.NeedsAttention {
		t.Fatalf("later: %+v %v", later, err)
	}
	exited := now.Add(-time.Minute)
	row = passageProjectionRow{ID: 7, Action: "exit", CreatedAt: &exited, PreviousID: 4, PreviousAction: "entry", PreviousAt: &entered}
	state, err := projectPassageRow(row, now)
	if err != nil || !state.InExitGrace || state.Open {
		t.Fatalf("paired exit: %+v %v", state, err)
	}
}

func TestPassage2667SQLProjectionFiltersBothEvents(t *testing.T) {
	for _, kind := range []ElementKind{ElementEmployee, ElementCar} {
		sql, err := PassageProjectionSQL(kind, "e", "p")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(sql, "p.action IN ('entry','exit','passage_close')") != 2 || strings.Count(sql, "LIMIT 1") != 2 || strings.Count(sql, "passage_close_revert") != 2 {
			t.Fatal(sql)
		}
		if strings.Contains(sql, "deactivate") || strings.Contains(sql, "dates_changed") {
			t.Fatal("nonpassage audits cannot pair an exit")
		}
	}
	if _, err := PassageProjectionSQL(ElementCar, "e;DROP TABLE cars", "p"); err == nil {
		t.Fatal("unsafe alias")
	}
	if _, err := PassageProjectionSQL(ElementKind("users"), "e", "p"); err == nil {
		t.Fatal("unsupported kind")
	}
}

func TestPassage2667ExpectedAndLifecycle(t *testing.T) {
	id := int64(7)
	if err := passageExpected(&id, 7, true); err != nil {
		t.Fatal(err)
	}
	if passageExpected(&id, 8, true) == nil || passageExpected(nil, 0, true) == nil {
		t.Fatal("stale and missing correction preconditions")
	}
	if err := passageExpected(nil, 7, false); err != nil {
		t.Fatal("ordinary legacy optional precondition")
	}
	now := time.Date(2029, 1, 1, 12, 0, 0, 0, time.UTC)
	snapshot := entityPeriodSnapshot{Manual: false, ApplicationStatus: models.StatusInWork, Confirmation: models.ConfirmationApproved, Status: passage2667Int(1), AttachmentStatus: passage2667Int(1)}
	if ok, _ := passageLifecycle(snapshot, passage2667Period(), now); !ok {
		t.Fatal("valid working permit")
	}
	for _, status := range []string{models.StatusWithdrawn, "unknown"} {
		copy := snapshot
		copy.ApplicationStatus = status
		if ok, _ := passageLifecycle(copy, passage2667Period(), now); ok {
			t.Fatal("inactive lifecycle admission")
		}
	}
	snapshot.Archived = true
	if ok, _ := passageLifecycle(snapshot, passage2667Period(), now); ok {
		t.Fatal("archive admission")
	}
}

func TestPassage2667RetentionSQLClockAndScope(t *testing.T) {
	now := time.Date(2029, 1, 1, 12, 0, 0, 0, time.UTC)
	sql, args, err := PassageRetentionSQL("e", "p", now)
	if err != nil || len(args) != 1 {
		t.Fatalf("%s %v %v", sql, args, err)
	}
	if !args[0].(time.Time).Equal(now.Add(-5*time.Minute)) || !strings.Contains(sql, "created_at>?") || !strings.Contains(sql, "previous_action='entry'") {
		t.Fatal("retention boundary/pairing")
	}
	if strings.Contains(sql, "table_id") || strings.Contains(sql, "status=0") {
		t.Fatal("retention cannot grant table scope or admission")
	}
}

func TestPassage2667CorrectionTableLifecycle(t *testing.T) {
	cases := []struct {
		name     string
		snapshot entityPeriodSnapshot
		blocked  bool
	}{
		{"work", entityPeriodSnapshot{ApplicationStatus: models.StatusInWork, Confirmation: models.ConfirmationApproved}, false},
		{"completed", entityPeriodSnapshot{ApplicationStatus: models.StatusCompleted, Confirmation: models.ConfirmationApproved}, false},
		{"manual", entityPeriodSnapshot{Manual: true}, false},
		{"withdrawn", entityPeriodSnapshot{ApplicationStatus: models.StatusWithdrawn, Confirmation: models.ConfirmationApproved}, true},
		{"archived", entityPeriodSnapshot{Archived: true, ApplicationStatus: models.StatusCompleted, Confirmation: models.ConfirmationApproved}, true},
		{"removed manual", entityPeriodSnapshot{Removed: true, Manual: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := passageCorrectionTableLifecycle(tc.snapshot)
			if (err != nil) != tc.blocked {
				t.Fatalf("blocked=%v error=%v", tc.blocked, err)
			}
		})
	}
}
