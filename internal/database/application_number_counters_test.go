package database_test

import (
	"testing"

	"systemburo/internal/database"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitializeApplicationNumberCounters(t *testing.T) {
	db := openEntityPeriodTestDB(t)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Exec("SET LOCAL search_path = pg_temp, pg_catalog").Error)
	require.NoError(t, tx.Exec("CREATE TEMP TABLE applications (application_number text)").Error)
	require.NoError(t, tx.Exec("CREATE TEMP TABLE destruction_log (entity_type text, application_number text)").Error)
	require.NoError(t, tx.Exec(`INSERT INTO applications VALUES
		('№ 20300101/001'), ('№ 20300101/003'), ('№ 20300101/003'),
		('20300102/1000'), ('другой формат'), (NULL)`).Error)
	require.NoError(t, tx.Exec(`INSERT INTO destruction_log VALUES
		('application','№ 20300101/009'), ('application','20300103/012'),
		('organization','20300101/777')`).Error)
	require.NoError(t, database.InitializeApplicationNumberCounters(tx))
	type counter struct {
		DayKey     string
		LastNumber int
	}
	read := func(db *gorm.DB) []counter {
		t.Helper()
		var result []counter
		require.NoError(t, db.Table("application_number_counters").Order("day_key").Find(&result).Error)
		return result
	}
	require.Equal(t, []counter{{"20300101", 9}, {"20300102", 1000}, {"20300103", 12}}, read(tx))
	require.NoError(t, tx.Exec("UPDATE application_number_counters SET last_number = 10 WHERE day_key = '20300101'").Error)
	require.NoError(t, database.InitializeApplicationNumberCounters(tx))
	require.Equal(t, 10, read(tx)[0].LastNumber)
}
