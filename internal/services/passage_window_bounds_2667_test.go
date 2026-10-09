package services

import (
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"systemburo/internal/models"
	"testing"
	"time"
)

func TestPassage2667WindowBoundsSameAdmission(t *testing.T) {
	from, to, clockFrom, clockTo := "2031-12-31", "2032-01-01", "08:00", "18:00:00"
	p := models.EffectivePeriod{EntryPeriod: models.EntryPeriod{EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &clockFrom, EntryTimeTo: &clockTo}, Source: "individual", Bounded: true}
	start, end, bounded, err := PassageWindowBounds(p)
	require.NoError(t, err)
	require.True(t, bounded)
	require.True(t, start.Equal(time.Date(2031, 12, 31, 5, 0, 0, 0, time.UTC)))
	require.True(t, end.Equal(time.Date(2032, 1, 1, 15, 0, 0, 0, time.UTC)))
	ok, reason := passageWindow(p, start)
	require.True(t, ok)
	require.Empty(t, reason)
	ok, reason = passageWindow(p, start.Add(-time.Microsecond))
	require.False(t, ok)
	require.Equal(t, "not_started", reason)
	ok, reason = passageWindow(p, end)
	require.False(t, ok)
	require.Equal(t, "expired", reason)
	p.EntryTimeFrom = nil
	p.EntryTimeTo = nil
	start, end, bounded, err = PassageWindowBounds(p)
	require.NoError(t, err)
	require.True(t, bounded)
	require.True(t, start.Equal(time.Date(2031, 12, 30, 21, 0, 0, 0, time.UTC)))
	require.True(t, end.Equal(time.Date(2032, 1, 1, 21, 0, 0, 0, time.UTC)))
	invalid := "invalid"
	p.EntryDateTo = &invalid
	_, _, _, err = PassageWindowBounds(p)
	require.Error(t, err)
	ok, reason = passageWindow(p, start)
	require.False(t, ok)
	require.Equal(t, "invalid_period", reason)
	_, _, bounded, err = PassageWindowBounds(models.EffectivePeriod{Source: "manual_unbounded"})
	require.NoError(t, err)
	require.False(t, bounded)
}

func TestPassage2667ConstructorUsesTrustedSessionClock(t *testing.T) {
	live := time.Date(2032, 2, 1, 0, 0, 0, 0, time.UTC)
	historical := live.Add(-24 * time.Hour)
	db := &gorm.DB{Config: &gorm.Config{NowFunc: func() time.Time { return live }}}
	session := db.Session(&gorm.Session{NowFunc: func() time.Time { return historical }})
	require.Equal(t, historical, NewPassageCommandService(session, nil).now())
	require.Equal(t, live, NewPassageCommandService(db, nil).now())
	require.Equal(t, live, db.NowFunc())
	require.WithinDuration(t, time.Now().UTC(), NewPassageCommandService(nil, nil).now(), time.Second)
}
