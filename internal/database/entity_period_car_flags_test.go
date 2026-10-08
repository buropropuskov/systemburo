package database_test

import (
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"systemburo/internal/database"
	"systemburo/internal/testutil"
	"testing"
)

func TestEntityPeriodCarFlagsMigrationDefaultsAndReplay(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		periodMigrationTables(t, tx)
		for _, sql := range []string{`INSERT INTO attachments(id) VALUES(1)`, `INSERT INTO employees(id) VALUES(1)`, `INSERT INTO cars(id,attachment_id) VALUES(1,1)`} {
			require.NoError(t, tx.Exec(sql).Error)
		}
		require.NoError(t, database.BackfillEntityPeriodModes(tx))
		var initial struct{ Roof, Parking bool }
		require.NoError(t, tx.Raw(`SELECT individual_roof_access AS roof,individual_free_parking AS parking FROM cars WHERE id=1`).Scan(&initial).Error)
		require.False(t, initial.Roof)
		require.False(t, initial.Parking)
		require.NoError(t, tx.Exec(`UPDATE cars SET individual_roof_access=true,individual_free_parking=true WHERE id=1`).Error)
		require.NoError(t, database.BackfillEntityPeriodModes(tx))
		require.NoError(t, tx.Exec(`INSERT INTO cars(id,attachment_id) VALUES(2,1)`).Error)
		var rows []struct {
			ID            int
			Roof, Parking bool
		}
		require.NoError(t, tx.Raw(`SELECT id,individual_roof_access AS roof,individual_free_parking AS parking FROM cars ORDER BY id`).Scan(&rows).Error)
		require.Len(t, rows, 2)
		require.True(t, rows[0].Roof)
		require.True(t, rows[0].Parking)
		require.False(t, rows[1].Roof)
		require.False(t, rows[1].Parking)
		return nil
	}))
}
