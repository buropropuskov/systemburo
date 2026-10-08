package database_test

import (
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"systemburo/internal/database"
	"systemburo/internal/testutil"
	"testing"
)

// Temporary tables shadow only this connection's entity tables. No shared rows,
// defaults or constraints are modified; all fixtures disappear at transaction end.
func periodMigrationTables(t *testing.T, tx *gorm.DB) {
	t.Helper()
	for _, ddl := range []string{
		`CREATE TEMP TABLE attachments (id integer PRIMARY KEY, entry_date_from varchar(20), entry_date_to varchar(20), entry_time_from varchar(20), entry_time_to varchar(20)) ON COMMIT DROP`,
		`CREATE TEMP TABLE employees (id integer PRIMARY KEY, period_mode varchar(16), entry_date_from varchar(20), entry_date_to varchar(20), entry_time_from varchar(20), entry_time_to varchar(20)) ON COMMIT DROP`,
		`CREATE TEMP TABLE cars (id integer PRIMARY KEY, attachment_id integer, period_mode varchar(16), entry_date_from varchar(20), entry_date_to varchar(20), entry_time_from varchar(20), entry_time_to varchar(20)) ON COMMIT DROP`,
	} {
		require.NoError(t, tx.Exec(ddl).Error)
	}
}

func TestBackfillEntityPeriodModesPreservesAndIsIdempotent(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		periodMigrationTables(t, tx)
		require.NoError(t, tx.Exec(`INSERT INTO attachments VALUES (1,'2030-10-08','2030-10-09','08:00:00','22:00:00'),(2,NULL,NULL,NULL,NULL)`).Error)
		require.NoError(t, tx.Exec(`INSERT INTO employees(id) VALUES (1)`).Error)
		require.NoError(t, tx.Exec(`INSERT INTO cars VALUES
			(1,1,NULL,'2030-10-08','2030-10-09','08:00','22:00'),
			(2,1,NULL,'2030-10-08','2030-10-15','08:00','22:00'),
			(3,2,NULL,NULL,NULL,NULL,NULL),
			(4,1,'individual','2030-10-08','2030-10-09','08:00:00','22:00:00')`).Error)
		require.NoError(t, database.BackfillEntityPeriodModes(tx))
		var rows []struct {
			ID          int
			PeriodMode  string
			EntryDateTo *string
		}
		require.NoError(t, tx.Table("cars").Order("id").Scan(&rows).Error)
		require.Len(t, rows, 4)
		require.Equal(t, []string{"inherit", "individual", "inherit", "individual"}, []string{rows[0].PeriodMode, rows[1].PeriodMode, rows[2].PeriodMode, rows[3].PeriodMode})
		require.Equal(t, "2030-10-15", *rows[1].EntryDateTo)
		// Explicitly individual equal windows must stay individual after a restart.
		require.NoError(t, database.BackfillEntityPeriodModes(tx))
		var mode string
		require.NoError(t, tx.Raw(`SELECT period_mode FROM cars WHERE id=4`).Scan(&mode).Error)
		require.Equal(t, "individual", mode)
		// Raw insert paths omitted by GORM receive the post-classification default.
		require.NoError(t, tx.Exec(`INSERT INTO employees(id) VALUES(2)`).Error)
		require.NoError(t, tx.Raw(`SELECT period_mode FROM employees WHERE id=2`).Scan(&mode).Error)
		require.Equal(t, "inherit", mode)
		return nil
	}))
}

func TestBackfillEntityPeriodModesInvalidLegacyRollsBack(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		periodMigrationTables(t, tx)
		require.NoError(t, tx.Exec(`INSERT INTO attachments VALUES(1,'2030-10-08','2030-10-09',NULL,NULL)`).Error)
		require.NoError(t, tx.Exec(`INSERT INTO employees(id) VALUES(1)`).Error)
		require.NoError(t, tx.Exec(`INSERT INTO cars VALUES(1,1,NULL,NULL,'2030-10-15',NULL,NULL)`).Error)
		require.ErrorContains(t, database.BackfillEntityPeriodModes(tx), "inventory required")
		var classified int64
		require.NoError(t, tx.Table("employees").Where("period_mode IS NOT NULL").Count(&classified).Error)
		require.Zero(t, classified, "backfill must be atomic on an invalid legacy window")
		return nil
	}))
}
