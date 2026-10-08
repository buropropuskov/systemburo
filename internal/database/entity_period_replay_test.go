package database_test

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"systemburo/internal/database"
	"systemburo/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// This test deliberately bypasses SetupTestApp: its initial preparation can
// truncate shared test tables. All writes here use one transaction, with only
// an owned schema and pg_catalog on its search_path, and are always rolled back.
// It covers GORM AllModels replay plus the period backfill, not the broader
// database.AutoMigrate wrapper with its extension and partition installers.
func openEntityPeriodTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL_TEST"))
	if dsn == "" {
		t.Skip("DATABASE_URL_TEST must explicitly identify an existing test database")
	}
	parsed, err := pgx.ParseConfig(dsn)
	// Never include a DSN or its parse error in test output: either may contain
	// credentials. No fallback to DATABASE_URL or automatic database creation.
	require.True(t, err == nil, "DATABASE_URL_TEST must be a valid PostgreSQL DSN")
	dbName := strings.ToLower(parsed.Database)
	require.True(t, strings.HasSuffix(dbName, "_test") || strings.HasPrefix(dbName, "test_"),
		"DATABASE_URL_TEST must select an explicitly named test database")
	if productionDSN := strings.TrimSpace(os.Getenv("DATABASE_URL")); productionDSN != "" {
		production, parseErr := pgx.ParseConfig(productionDSN)
		if parseErr == nil {
			require.False(t, production.Host == parsed.Host && production.Port == parsed.Port && production.Database == parsed.Database,
				"test database must differ from DATABASE_URL")
		}
	}

	db, err := gorm.Open(postgres.Open(database.EnsureUTCTimezone(dsn)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.True(t, err == nil, "cannot connect to the explicitly configured test database")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	return db
}

func TestEntityPeriodAllModelsReplayIsIsolated(t *testing.T) {
	db := openEntityPeriodTestDB(t)

	tx := db.Begin()
	require.NoError(t, tx.Error)
	defer func() { require.NoError(t, tx.Rollback().Error) }()
	var randomID [12]byte
	_, err := rand.Read(randomID[:])
	require.NoError(t, err)
	// This identifier contains only a fixed prefix and lowercase hex generated
	// here. It never accepts an environment value or a request parameter.
	schema := "period_replay_" + hex.EncodeToString(randomID[:])
	require.NoError(t, tx.Exec("CREATE SCHEMA "+schema).Error)
	require.NoError(t, tx.Exec("SET LOCAL search_path = "+schema+", pg_catalog").Error)
	assertPeriodReplaySchema(t, tx, schema)

	require.NoError(t, tx.AutoMigrate(database.AllModels()...))
	assertPeriodReplaySchema(t, tx, schema)
	// Before the first backfill, nullable modes model unclassified legacy rows.
	// Fixed dates test storage/migration only; no comparison to today's clock.
	require.NoError(t, tx.Exec(`INSERT INTO attachments
		(id, attachment_type, is_manual, entry_date_from, entry_date_to, entry_time_from, entry_time_to, status)
		VALUES (100, 'cars', true, '2030-10-08', '2030-10-09', '08:00:00', '22:00:00', 1),
		       (101, 'people', true, NULL, NULL, NULL, NULL, 1)`).Error)
	require.NoError(t, tx.Exec(`INSERT INTO employees (id, attachment_id, status)
		VALUES (100, 101, 1), (101, 101, 0)`).Error)
	require.NoError(t, tx.Exec(`INSERT INTO cars
		(id, attachment_id, entry_date_from, entry_date_to, entry_time_from, entry_time_to, status)
		VALUES (100, 100, '2030-10-08', '2030-10-09', '08:00', '22:00', 1),
		       (101, 100, '2030-10-08', '2030-10-15', '08:00', '22:00', 0)`).Error)
	require.NoError(t, database.BackfillEntityPeriodModes(tx))
	assertPeriodReplayMode(t, tx, "cars", 100, "inherit")
	assertPeriodReplayMode(t, tx, "cars", 101, "individual")
	assertPeriodReplayMode(t, tx, "employees", 100, "inherit")
	// Equal own/parent values remain individual when explicitly selected.
	require.NoError(t, tx.Exec(`UPDATE cars SET period_mode='individual' WHERE id=100`).Error)
	require.NoError(t, tx.Exec(`INSERT INTO employees
		(id, attachment_id, period_mode, entry_date_from, entry_date_to, entry_time_from, entry_time_to, status)
		VALUES (102, 101, 'individual', '2030-10-08', '2030-10-09', '08:00', '22:00', 1)`).Error)
	assertPeriodReplayDefaults(t, tx, schema)
	assertPeriodReplayRawBatch(t, tx, 200)
	before := periodReplaySnapshot(t, tx)

	// This is the second complete model migration, not a selective migration of
	// Employee/Car. GORM may reconcile model tags against the backfilled defaults.
	require.NoError(t, tx.AutoMigrate(database.AllModels()...))
	require.NoError(t, database.BackfillEntityPeriodModes(tx))
	assertPeriodReplaySchema(t, tx, schema)
	require.Equal(t, before, periodReplaySnapshot(t, tx), "replay must preserve dates, modes, statuses and attachment links")
	assertPeriodReplayMode(t, tx, "cars", 100, "individual")
	assertPeriodReplayDefaults(t, tx, schema)
	assertPeriodReplayRawBatch(t, tx, 300)
	assertPeriodReplayConstraints(t, tx)

	employee := models.Employee{}
	require.NoError(t, tx.Create(&employee).Error)
	require.Equal(t, models.PeriodInherit, employee.PeriodMode)
	car := models.Car{AttachmentID: 100}
	require.NoError(t, tx.Create(&car).Error)
	require.Equal(t, models.PeriodInherit, car.PeriodMode)
}

func assertPeriodReplaySchema(t *testing.T, tx *gorm.DB, expected string) {
	t.Helper()
	var current, path string
	require.NoError(t, tx.Raw("SELECT current_schema(), current_setting('search_path')").Row().Scan(&current, &path))
	require.Equal(t, expected, current)
	require.Equal(t, expected+", pg_catalog", path, "public and temporary schemas must not be on the explicit path")
	var wrongSchema int64
	require.NoError(t, tx.Raw(`SELECT count(*) FROM pg_class c
		JOIN pg_namespace n ON n.oid=c.relnamespace
		WHERE c.oid IN (to_regclass('attachments'), to_regclass('employees'), to_regclass('cars'))
		AND n.nspname<>?`, expected).Scan(&wrongSchema).Error)
	require.Zero(t, wrongSchema, "entity names must resolve only to this transaction's owned schema")
}

func assertPeriodReplayMode(t *testing.T, tx *gorm.DB, table string, id int, expected string) {
	t.Helper()
	require.Contains(t, []string{"employees", "cars"}, table)
	var mode string
	require.NoError(t, tx.Table(table).Select("period_mode").Where("id=?", id).Scan(&mode).Error)
	require.Equal(t, expected, mode)
}

func assertPeriodReplayDefaults(t *testing.T, tx *gorm.DB, schema string) {
	t.Helper()
	for _, table := range []string{"employees", "cars"} {
		var column struct {
			ColumnDefault string
			IsNullable    string
			DataType      string
		}
		require.NoError(t, tx.Raw(`SELECT column_default, is_nullable, data_type
			FROM information_schema.columns WHERE table_schema=? AND table_name=? AND column_name='period_mode'`,
			schema, table).Scan(&column).Error)
		require.Contains(t, column.ColumnDefault, "'inherit'")
		require.Equal(t, "NO", column.IsNullable)
		require.Equal(t, "character varying", column.DataType)
	}
}

// These SQL shapes intentionally omit period_mode, like legacy batch VALUES
// submission and supplement INSERT ... SELECT. They exercise database defaults
// without GORM's BeforeCreate hooks. They do not claim to run the full services.
func assertPeriodReplayRawBatch(t *testing.T, tx *gorm.DB, firstID int) {
	t.Helper()
	require.NoError(t, tx.Exec(`INSERT INTO employees (id, attachment_id, status)
		VALUES (?,101,1),(?,101,0)`, firstID, firstID+1).Error)
	require.NoError(t, tx.Exec(`INSERT INTO cars
		(id, attachment_id, entry_date_from, entry_time_from, entry_date_to, entry_time_to, status)
		VALUES (?,100,'2030-10-08','08:00','2030-10-09','22:00',1),
		       (?,100,'2030-10-08','08:00','2030-10-09','22:00',0)`, firstID, firstID+1).Error)
	require.NoError(t, tx.Exec(`INSERT INTO cars
		(id, attachment_id, entry_date_from, entry_time_from, entry_date_to, entry_time_to, status)
		SELECT ?,a.id,a.entry_date_from,a.entry_time_from,a.entry_date_to,a.entry_time_to,0
		FROM attachments a WHERE a.id=100`, firstID+2).Error)
	for _, table := range []string{"employees", "cars"} {
		assertPeriodReplayMode(t, tx, table, firstID, "inherit")
		assertPeriodReplayMode(t, tx, table, firstID+1, "inherit")
	}
	assertPeriodReplayMode(t, tx, "cars", firstID+2, "inherit")
}

func periodReplaySnapshot(t *testing.T, tx *gorm.DB) string {
	t.Helper()
	var snapshot string
	require.NoError(t, tx.Raw(`SELECT jsonb_agg(to_jsonb(rows) ORDER BY kind,id)::text FROM (
		SELECT 'car' AS kind,id,attachment_id,period_mode,entry_date_from,entry_date_to,entry_time_from,entry_time_to,status FROM cars
		UNION ALL
		SELECT 'employee',id,attachment_id,period_mode,entry_date_from,entry_date_to,entry_time_from,entry_time_to,status FROM employees
	) rows`).Scan(&snapshot).Error)
	return snapshot
}

func assertPeriodReplayConstraints(t *testing.T, tx *gorm.DB) {
	t.Helper()
	for _, table := range []string{"employees", "cars"} {
		for _, invalid := range []struct {
			value interface{}
			code  string
		}{{"unknown", "23514"}, {nil, "23502"}} {
			// A savepoint prevents the expected constraint error from aborting
			// the external transaction or masking subsequent assertions.
			require.NoError(t, tx.SavePoint("period_invalid_mode").Error)
			err := tx.Exec("UPDATE "+table+" SET period_mode=? WHERE id=100", invalid.value).Error
			require.NoError(t, tx.RollbackTo("period_invalid_mode").Error)
			var pgErr *pgconn.PgError
			require.ErrorAs(t, err, &pgErr)
			require.Equal(t, invalid.code, pgErr.Code)
			require.NoError(t, tx.Exec("RELEASE SAVEPOINT period_invalid_mode").Error)
		}
	}
}
