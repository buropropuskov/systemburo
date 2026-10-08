package models

import (
	"errors"
	"testing"
)

func periodString(value string) *string { return &value }
func testEntryPeriod() EntryPeriod {
	return EntryPeriod{EntryDateFrom: periodString("2030-10-08"), EntryDateTo: periodString("2030-10-09"), EntryTimeFrom: periodString("08:00"), EntryTimeTo: periodString("22:00")}
}

func TestResolveEntityPeriod(t *testing.T) {
	parent := testEntryPeriod()
	for _, tc := range []struct {
		name        string
		mode        PeriodMode
		own, parent EntryPeriod
		manual      bool
		source      string
		bounded     bool
		wantErr     error
	}{
		{"inherited", PeriodInherit, EntryPeriod{}, parent, false, "attachment", true, nil},
		{"equal individual remains explicit", PeriodIndividual, parent, parent, false, "individual", true, nil},
		{"manual unbounded", PeriodInherit, EntryPeriod{}, EntryPeriod{}, true, "manual_unbounded", false, nil},
		{"manual individual", PeriodIndividual, parent, EntryPeriod{}, true, "individual", true, nil},
		{"invalid mode", PeriodMode("other"), parent, parent, false, "", false, ErrInvalidPeriodMode},
		{"individual missing period does not inherit", PeriodIndividual, EntryPeriod{}, parent, false, "", false, ErrInvalidIndividualPeriod},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveEntityPeriod(tc.mode, tc.own, tc.parent, tc.manual)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if got.Source != tc.source || got.Bounded != tc.bounded {
				t.Fatalf("effective period %+v", got)
			}
			if tc.mode == PeriodIndividual && got.EntryDateTo != tc.own.EntryDateTo {
				t.Fatal("individual window was replaced")
			}
		})
	}
}

func TestValidateStoredIndividualPeriod(t *testing.T) {
	valid := testEntryPeriod()
	for _, tc := range []struct {
		name    string
		period  EntryPeriod
		invalid bool
	}{
		{"legacy no clocks", EntryPeriod{EntryDateFrom: valid.EntryDateFrom, EntryDateTo: valid.EntryDateTo}, false},
		{"past dates are valid storage", EntryPeriod{EntryDateFrom: periodString("2000-01-01"), EntryDateTo: periodString("2000-01-02")}, false},
		{"missing start", EntryPeriod{EntryDateTo: valid.EntryDateTo}, true},
		{"bad calendar", EntryPeriod{EntryDateFrom: periodString("2030-02-30"), EntryDateTo: valid.EntryDateTo}, true},
		{"reversed", EntryPeriod{EntryDateFrom: valid.EntryDateTo, EntryDateTo: valid.EntryDateFrom}, true},
		{"bad clock", EntryPeriod{EntryDateFrom: valid.EntryDateFrom, EntryDateTo: valid.EntryDateTo, EntryTimeTo: periodString("25:00")}, true},
		{"equal endpoints", EntryPeriod{EntryDateFrom: valid.EntryDateFrom, EntryDateTo: valid.EntryDateFrom, EntryTimeFrom: periodString("08:00"), EntryTimeTo: periodString("08:00:00")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateStoredIndividualPeriod(tc.period); (err != nil) != tc.invalid {
				t.Fatalf("error %v, invalid=%v", err, tc.invalid)
			}
		})
	}
}

func TestEntityCreateDefaultsPeriodMode(t *testing.T) {
	emp := Employee{}
	car := Car{}
	if err := emp.BeforeCreate(nil); err != nil {
		t.Fatal(err)
	}
	if err := car.BeforeCreate(nil); err != nil {
		t.Fatal(err)
	}
	if emp.PeriodMode != PeriodInherit || car.PeriodMode != PeriodInherit {
		t.Fatal("new rows must inherit")
	}
	emp.PeriodMode = PeriodIndividual
	car.PeriodMode = PeriodIndividual
	_ = emp.BeforeCreate(nil)
	_ = car.BeforeCreate(nil)
	if emp.PeriodMode != PeriodIndividual || car.PeriodMode != PeriodIndividual {
		t.Fatal("explicit mode overwritten")
	}
}
