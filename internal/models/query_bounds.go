package models

import (
	"time"

	"systemburo/internal/apperr"
)

// QueryDateBounds is an optional calendar interval [From, To) in Moscow time.
type QueryDateBounds struct {
	From *time.Time
	To   *time.Time
}

// ParseQueryDateBounds validates real ISO calendar days before building SQL.
// The requested final day is included by using the next midnight exclusively.
func ParseQueryDateBounds(from, to string) (QueryDateBounds, error) {
	loc := time.FixedZone("MSK", 3*60*60)
	var bounds QueryDateBounds
	parse := func(value string) (*time.Time, error) {
		if value == "" {
			return nil, nil
		}
		day, err := time.ParseInLocation("2006-01-02", value, loc)
		if err != nil || day.Format("2006-01-02") != value {
			return nil, apperr.Validation("Invalid calendar date")
		}
		return &day, nil
	}
	var err error
	if bounds.From, err = parse(from); err != nil {
		return QueryDateBounds{}, err
	}
	last, err := parse(to)
	if err != nil {
		return QueryDateBounds{}, err
	}
	if bounds.From != nil && last != nil && bounds.From.After(*last) {
		return QueryDateBounds{}, apperr.Validation("Invalid date range")
	}
	if last != nil {
		end := last.AddDate(0, 0, 1)
		bounds.To = &end
	}
	return bounds, nil
}
