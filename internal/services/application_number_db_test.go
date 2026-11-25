package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestApplicationNumberCountedInMoscow(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	midnight := time.Date(2031, 12, 31, 21, 0, 0, 0, time.UTC)
	number, err := services.NextApplicationNumberForTest(db, midnight.Add(-time.Second))
	require.NoError(t, err)
	require.Equal(t, "№ 20311231/001", number)
	number, err = services.NextApplicationNumberForTest(db, midnight)
	require.NoError(t, err)
	require.Equal(t, "№ 20320101/001", number)

	// Откат всей подачи откатывает и выделение номера.
	rollback := errors.New("rollback")
	err = db.Transaction(func(tx *gorm.DB) error {
		n, err := services.NextApplicationNumberForTest(tx, midnight)
		require.NoError(t, err)
		require.Equal(t, "№ 20320101/002", n)
		return rollback
	})
	require.ErrorIs(t, err, rollback)

	// Два независимых соединения резервируют разные номера за одни сутки.
	type result struct {
		number string
		err    error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	for range 2 {
		go func() {
			<-start
			var n string
			err := db.Transaction(func(tx *gorm.DB) error {
				var err error
				n, err = services.NextApplicationNumberForTest(tx, midnight)
				return err
			})
			results <- result{n, err}
		}()
	}
	close(start)
	first, second := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.ElementsMatch(t, []string{"№ 20320101/002", "№ 20320101/003"}, []string{first.number, second.number})

	require.NoError(t, db.Exec("UPDATE application_number_counters SET last_number = 999 WHERE day_key = ?", "20320101").Error)
	number, err = services.NextApplicationNumberForTest(db, midnight)
	require.NoError(t, err)
	require.Equal(t, "№ 20320101/1000", number)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	number, err = services.NextApplicationNumberForTest(db.WithContext(ctx), midnight)
	require.Error(t, err)
	require.Empty(t, number)
}
