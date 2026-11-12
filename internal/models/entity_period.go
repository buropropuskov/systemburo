package models

import (
	"errors"
	"strings"
	"time"
)

// PeriodMode belongs to one application/manual row, not to a registry identity.
type PeriodMode string

const (
	PeriodInherit    PeriodMode = "inherit"
	PeriodIndividual PeriodMode = "individual"
)

var ErrInvalidPeriodMode = errors.New("invalid period mode")
var ErrInvalidIndividualPeriod = errors.New("invalid individual period")

// EntryPeriod keeps the existing Moscow wall-clock wire/storage representation.
type EntryPeriod struct {
	EntryDateFrom *string `json:"entry_date_from"`
	EntryDateTo   *string `json:"entry_date_to"`
	EntryTimeFrom *string `json:"entry_time_from"`
	EntryTimeTo   *string `json:"entry_time_to"`
}

type EffectivePeriod struct {
	EntryPeriod
	Bounded bool   `json:"bounded"`
	Source  string `json:"source"`
}

// ResolveEntityPeriod selects a whole window. Equal values do not erase an explicit
// individual mode. It does not grant access or infer physical presence.
func ResolveEntityPeriod(mode PeriodMode, own, parent EntryPeriod, manual bool) (EffectivePeriod, error) {
	var window EntryPeriod
	source := "attachment"
	switch mode {
	case "", PeriodInherit: // Empty is accepted only for compatibility during rollout.
		window = parent
	case PeriodIndividual:
		window = own
		source = "individual"
		if err := ValidateStoredIndividualPeriod(window); err != nil {
			return EffectivePeriod{}, err
		}
	default:
		return EffectivePeriod{}, ErrInvalidPeriodMode
	}
	bounded := periodValue(window.EntryDateTo) != ""
	if manual && mode != PeriodIndividual && emptyEntryPeriod(window) {
		source = "manual_unbounded"
	}
	return EffectivePeriod{EntryPeriod: window, Bounded: bounded, Source: source}, nil
}

// ValidateStoredIndividualPeriod accepts legacy optional clocks. New writes must
// additionally require both clocks and a future end in their command validator.
func ValidateStoredIndividualPeriod(p EntryPeriod) error {
	from, errFrom := time.Parse("2006-01-02", periodValue(p.EntryDateFrom))
	to, errTo := time.Parse("2006-01-02", periodValue(p.EntryDateTo))
	if errFrom != nil || errTo != nil || to.Before(from) {
		return ErrInvalidIndividualPeriod
	}
	clock := func(raw *string, fallback string) (time.Duration, error) {
		value := periodValue(raw)
		if value == "" {
			value = fallback
		}
		for _, layout := range []string{"15:04:05", "15:04"} {
			if parsed, err := time.Parse(layout, value); err == nil {
				return time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute + time.Duration(parsed.Second())*time.Second, nil
			}
		}
		return 0, ErrInvalidIndividualPeriod
	}
	start, errStart := clock(p.EntryTimeFrom, "00:00:00")
	end, errEnd := clock(p.EntryTimeTo, "23:59:59")
	if errStart != nil || errEnd != nil || !to.Add(end).After(from.Add(start)) {
		return ErrInvalidIndividualPeriod
	}
	return nil
}

func emptyEntryPeriod(p EntryPeriod) bool {
	return periodValue(p.EntryDateFrom) == "" && periodValue(p.EntryDateTo) == "" &&
		periodValue(p.EntryTimeFrom) == "" && periodValue(p.EntryTimeTo) == ""
}

// ValidateOptionalEntryPeriod checks new manual input without requiring dates or
// clocks for an unbounded pass. Overnight clocks remain supported; only a date
// range that actually runs backwards is rejected. Stored legacy rows are untouched.
func ValidateOptionalEntryPeriod(p EntryPeriod) error {
	for _, raw := range []*string{p.EntryDateFrom, p.EntryDateTo} {
		if raw != nil && *raw != "" {
			if parsed, err := time.Parse("2006-01-02", *raw); err != nil || parsed.Format("2006-01-02") != *raw {
				return ErrInvalidIndividualPeriod
			}
		}
	}
	for _, raw := range []*string{p.EntryTimeFrom, p.EntryTimeTo} {
		if raw == nil || *raw == "" {
			continue
		}
		minute, minuteErr := time.Parse("15:04", *raw)
		second, secondErr := time.Parse("15:04:05", *raw)
		if (minuteErr != nil || minute.Format("15:04") != *raw) && (secondErr != nil || second.Format("15:04:05") != *raw) {
			return ErrInvalidIndividualPeriod
		}
	}
	if p.EntryDateFrom != nil && p.EntryDateTo != nil && *p.EntryDateFrom != "" && *p.EntryDateTo != "" && *p.EntryDateTo < *p.EntryDateFrom {
		return ErrInvalidIndividualPeriod
	}
	return nil
}

func periodValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
