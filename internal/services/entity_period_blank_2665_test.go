package services

import (
	"crypto/sha256"
	"encoding/json"
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

func blank2665Window(from, to, start, end string) models.EntryPeriod {
	return models.EntryPeriod{EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &start, EntryTimeTo: &end}
}

func blank2665Attachment(p models.EntryPeriod) *models.Attachment {
	return &models.Attachment{ID: 12, EntryDateFrom: p.EntryDateFrom, EntryDateTo: p.EntryDateTo, EntryTimeFrom: p.EntryTimeFrom, EntryTimeTo: p.EntryTimeTo}
}

func TestEntityPeriod2665BlankAndSnapshotUseSameWholeWindow(t *testing.T) {
	parent := blank2665Window("2030-10-08", "2030-10-09", "08:00:00", "22:00:00")
	own := blank2665Window("2030-10-10", "2030-10-15", "10:00:00", "18:00:00")
	for _, tc := range []struct {
		name   string
		mode   models.PeriodMode
		own    models.EntryPeriod
		want   models.EntryPeriod
		source string
	}{
		{"inherit ignores old own window", models.PeriodInherit, own, parent, "attachment"},
		{"inherit after own cleared", models.PeriodInherit, models.EntryPeriod{}, parent, "attachment"},
		{"legacy empty mode inherits", "", own, parent, "attachment"},
		{"individual outlives parent", models.PeriodIndividual, own, own, "individual"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := blank2665Attachment(parent)
			car := models.Car{PeriodMode: tc.mode, EntryDateFrom: tc.own.EntryDateFrom, EntryDateTo: tc.own.EntryDateTo, EntryTimeFrom: tc.own.EntryTimeFrom, EntryTimeTo: tc.own.EntryTimeTo}
			bctx := &BlankContext{Attachment: a, Cars: []models.Car{car}}
			require.NoError(t, validateBlankEntityPeriods(bctx))
			snap, err := snapshotCarWithPeriod(snapshotCar{ID: 7, Number: "TEST-7"}, a, tc.mode, tc.own)
			require.NoError(t, err)
			require.Equal(t, derefStr(tc.want.EntryDateFrom), snap.EntryDateFrom)
			require.Equal(t, derefStr(tc.want.EntryDateTo), snap.EntryDateTo)
			require.Equal(t, derefStr(tc.want.EntryTimeFrom), snap.EntryTimeFrom)
			require.Equal(t, derefStr(tc.want.EntryTimeTo), snap.EntryTimeTo)
			require.Equal(t, tc.source, snap.PeriodSource)
			require.True(t, snap.PeriodBounded)
			require.Equal(t, tc.own, snap.StoredPeriod)
			require.Equal(t, "TEST-7", snap.Number)
			for path, want := range map[string]string{
				"car.entry_date_from": formatDate(snap.EntryDateFrom), "car.entry_date_to": formatDate(snap.EntryDateTo),
				"car.entry_time_from": formatTime(snap.EntryTimeFrom), "car.entry_time_to": formatTime(snap.EntryTimeTo),
			} {
				require.Equal(t, want, resolveValue(bctx, path, 0), path)
			}
			require.Equal(t, "09.10.2030", resolveValue(bctx, "attachment.entry_date_to", 0))
			require.Equal(t, "22:00", resolveValue(bctx, "attachment.entry_time_to", 0))
			require.Equal(t, parent.EntryDateTo, a.EntryDateTo)
			emp, err := snapshotEmployeeWithPeriod(snapshotEmployee{ID: 8}, a, tc.mode, tc.own)
			require.NoError(t, err)
			require.Equal(t, snap.EntryDateTo, emp.EntryDateTo)
			require.Equal(t, snap.EntryTimeFrom, emp.EntryTimeFrom)
			require.Equal(t, snap.EntryTimeTo, emp.EntryTimeTo)
			require.Equal(t, tc.source, emp.PeriodSource)
			require.Equal(t, tc.own, emp.StoredPeriod)
		})
	}
}

func TestEntityPeriod2665BlankRejectsInvalidWindowsWithoutOwnFallback(t *testing.T) {
	valid := blank2665Window("2030-10-08", "2030-10-09", "08:00:00", "22:00:00")
	for _, tc := range []struct {
		name   string
		mode   models.PeriodMode
		parent *models.Attachment
		own    models.EntryPeriod
	}{
		{"unknown mode", "unknown", blank2665Attachment(valid), valid},
		{"individual missing own dates", models.PeriodIndividual, blank2665Attachment(valid), models.EntryPeriod{}},
		{"individual reversed", models.PeriodIndividual, blank2665Attachment(valid), blank2665Window("2030-10-10", "2030-10-09", "08:00", "22:00")},
		{"individual bad clock", models.PeriodIndividual, blank2665Attachment(valid), blank2665Window("2030-10-08", "2030-10-09", "99:00", "22:00")},
		{"missing parent", models.PeriodInherit, nil, valid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			car := models.Car{PeriodMode: tc.mode, EntryDateFrom: tc.own.EntryDateFrom, EntryDateTo: tc.own.EntryDateTo, EntryTimeFrom: tc.own.EntryTimeFrom, EntryTimeTo: tc.own.EntryTimeTo}
			bctx := &BlankContext{Attachment: tc.parent, Cars: []models.Car{car}}
			require.Error(t, validateBlankEntityPeriods(bctx))
			require.Empty(t, resolveValue(bctx, "car.entry_date_to", 0))
			_, err := snapshotCarWithPeriod(snapshotCar{}, tc.parent, tc.mode, tc.own)
			require.Error(t, err)
			_, err = snapshotEmployeeWithPeriod(snapshotEmployee{}, tc.parent, tc.mode, tc.own)
			require.Error(t, err)
		})
	}
	invalidEmployee := &BlankContext{Attachment: blank2665Attachment(valid), Employees: []models.Employee{{PeriodMode: models.PeriodIndividual}}}
	require.Error(t, validateBlankEntityPeriods(invalidEmployee))
}

func TestEntityPeriod2665BlankPreservesUnboundedAndLegacyClocks(t *testing.T) {
	manual := &models.Attachment{ID: 12, IsManual: true}
	car, err := snapshotCarWithPeriod(snapshotCar{}, manual, models.PeriodInherit, models.EntryPeriod{})
	require.NoError(t, err)
	require.False(t, car.PeriodBounded)
	require.Equal(t, "manual_unbounded", car.PeriodSource)
	require.Empty(t, car.EntryDateTo)
	emp, err := snapshotEmployeeWithPeriod(snapshotEmployee{}, manual, models.PeriodInherit, models.EntryPeriod{})
	require.NoError(t, err)
	require.Equal(t, "manual_unbounded", emp.PeriodSource)
	legacy := blank2665Window("2030-10-08", "2030-10-09", "", "")
	legacy.EntryTimeFrom, legacy.EntryTimeTo = nil, nil
	car, err = snapshotCarWithPeriod(snapshotCar{}, blank2665Attachment(legacy), models.PeriodInherit, models.EntryPeriod{})
	require.NoError(t, err)
	require.True(t, car.PeriodBounded)
	require.Empty(t, car.EntryTimeFrom)
	require.Empty(t, car.EntryTimeTo)
	// Historical parent windows were allowed to omit their start. Do not add a
	// stricter inheritance gate while switching the source of exported values.
	legacy.EntryDateFrom = nil
	car, err = snapshotCarWithPeriod(snapshotCar{}, blank2665Attachment(legacy), models.PeriodInherit, models.EntryPeriod{})
	require.NoError(t, err)
	require.Equal(t, "2030-10-09", car.EntryDateTo)
	require.Empty(t, car.EntryDateFrom)
}

func TestEntityPeriod2665SnapshotHashIncludesModeSourceAndEmployeeWindow(t *testing.T) {
	window := blank2665Window("2030-10-08", "2030-10-09", "08:00", "22:00")
	a := blank2665Attachment(window)
	inherit, err := snapshotCarWithPeriod(snapshotCar{ID: 7}, a, models.PeriodInherit, window)
	require.NoError(t, err)
	individual, err := snapshotCarWithPeriod(snapshotCar{ID: 7}, a, models.PeriodIndividual, window)
	require.NoError(t, err)
	encode := func(v any) []byte { data, err := json.Marshal(v); require.NoError(t, err); return data }
	first, second := encode(inherit), encode(individual)
	require.NotEqual(t, sha256.Sum256(first), sha256.Sum256(second), "equal dates do not erase mode/source")
	require.Equal(t, first, encode(inherit), "stable repeated serialization")
	var oldReader struct {
		ID          int    `json:"id"`
		EntryDateTo string `json:"entry_date_to"`
	}
	require.NoError(t, json.Unmarshal(second, &oldReader))
	require.Equal(t, individual.EntryDateTo, oldReader.EntryDateTo)
	emp, err := snapshotEmployeeWithPeriod(snapshotEmployee{ID: 8}, a, models.PeriodIndividual, window)
	require.NoError(t, err)
	before := encode(emp)
	changed := blank2665Window("2030-10-08", "2030-10-15", "08:00", "22:00")
	emp, err = snapshotEmployeeWithPeriod(snapshotEmployee{ID: 8}, a, models.PeriodIndividual, changed)
	require.NoError(t, err)
	require.NotEqual(t, sha256.Sum256(before), sha256.Sum256(encode(emp)))
	require.Contains(t, string(encode(emp)), `"stored_period"`)
	require.Equal(t, 2, archiveSnapshotSchemaVersion)
}
