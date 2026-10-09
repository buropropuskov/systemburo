package fakedata

import (
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"testing"
	"time"
)

func TestHistoricalPassage2667WholeWindow(t *testing.T) {
	from, to, startClock, endClock := "2031-12-31", "2032-01-01", "08:00", "18:00"
	start := time.Date(2031, 12, 31, 8, 0, 0, 0, services.MoscowLocation())
	end := time.Date(2032, 1, 1, 18, 0, 0, 0, services.MoscowLocation())
	now := end.Add(24 * time.Hour)
	c := passageCandidate{AcceptedAt: start.Add(-time.Hour), Period: models.EntryPeriod{EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &startClock, EntryTimeTo: &endClock}, Source: "individual", ValidMode: true}
	lo, hi, ok := historicalPassageWindow(c, now)
	require.True(t, ok)
	require.True(t, lo.Equal(start))
	require.True(t, hi.Equal(end.Add(-time.Microsecond)))
	require.False(t, passageWindowOpen(c, now))
	require.True(t, passageWindowOpen(c, start))
	require.False(t, passageWindowOpen(c, end))
	stream := NewStream(2667, "historical-whole-window")
	for i := 0; i < 100; i++ {
		entry := passageMoment(stream, lo, hi.Add(-time.Microsecond)).UTC().Truncate(time.Microsecond)
		exit := passageMoment(stream, entry, hi).UTC().Truncate(time.Microsecond)
		require.False(t, entry.Before(start))
		require.True(t, exit.After(entry))
		require.True(t, exit.Before(end))
	}
	c.AcceptedAt = end
	_, _, ok = historicalPassageWindow(c, now)
	require.False(t, ok)
	c.AcceptedAt = start
	c.ValidMode = false
	require.Empty(t, passableCandidates([]passageCandidate{c}, now))
}

func TestHistoricalPassage2667MicrosecondAndDateOnly(t *testing.T) {
	date := "2032-01-01"
	start := time.Date(2032, 1, 1, 0, 0, 0, 0, services.MoscowLocation())
	c := passageCandidate{AcceptedAt: start, Period: models.EntryPeriod{EntryDateFrom: &date, EntryDateTo: &date}, Source: "attachment", ValidMode: true}
	lo, hi, ok := historicalPassageWindow(c, start.Add(2*time.Microsecond))
	require.True(t, ok)
	entry := passageMoment(NewStream(1, "tiny-entry"), lo, hi.Add(-time.Microsecond))
	exit := passageMoment(NewStream(2, "tiny-exit"), entry, hi)
	require.True(t, exit.After(entry))
	_, _, ok = historicalPassageWindow(c, start.Add(time.Microsecond))
	require.False(t, ok)
	_, hi, ok = historicalPassageWindow(c, start.AddDate(0, 0, 2))
	require.True(t, ok)
	require.True(t, hi.Equal(start.AddDate(0, 0, 1).Add(-time.Microsecond)))
	c.AcceptedAt = start.Add(500 * time.Nanosecond)
	lo, _, ok = historicalPassageWindow(c, start.Add(time.Second))
	require.True(t, ok)
	require.False(t, lo.Before(c.AcceptedAt))
	require.Zero(t, lo.Nanosecond()%1000)
}

func TestHistoricalPassage2667SessionClockDoesNotEscape(t *testing.T) {
	live := time.Date(2032, 2, 1, 0, 0, 0, 0, time.UTC)
	old := live.Add(-24 * time.Hour).Add(987654321 * time.Nanosecond)
	// No dialector or connection is used: this asserts GORM session config cloning.
	db := &gorm.DB{Config: &gorm.Config{NowFunc: func() time.Time { return live }}}
	session := historicalPassageDB(db, old)
	require.Equal(t, live, db.NowFunc())
	require.Equal(t, old.Truncate(time.Microsecond), session.NowFunc())
	require.Equal(t, live, db.NowFunc())
}
