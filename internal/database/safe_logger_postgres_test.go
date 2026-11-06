package database_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"systemburo/internal/database"
	"systemburo/internal/logmask"
)

func TestSafeLoggerPostgresDistinguishesMissingRowAndSQLFailure(t *testing.T) {
	db := openEntityPeriodTestDB(t)
	var out bytes.Buffer
	safe := database.NewSafeLogger(slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{ReplaceAttr: logmask.ReplaceAttr})), logger.Info)
	tx := db.Session(&gorm.Session{Logger: safe}).Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Exec("CREATE TEMP TABLE diagnostic_owned_probe (id integer) ON COMMIT DROP").Error)
	var absent struct{ ID int }
	err := tx.Table("diagnostic_owned_probe").Where("id = ?", 123456).First(&absent).Error
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, tx.SavePoint("diagnostic_probe").Error)
	var sqlError *pgconn.PgError
	require.True(t, errors.As(tx.Exec("SELECT 1 / 0").Error, &sqlError))
	require.Equal(t, "22012", sqlError.Code)
	require.NoError(t, tx.RollbackTo("diagnostic_probe").Error)
	require.NotContains(t, out.String(), "diagnostic_owned_probe")
	require.NotContains(t, out.String(), "SELECT")
	require.NotContains(t, out.String(), "123456")
	require.NotContains(t, out.String(), "division by zero")
	decoder := json.NewDecoder(strings.NewReader(out.String()))
	var failures []map[string]any
	for decoder.More() {
		var entry map[string]any
		require.NoError(t, decoder.Decode(&entry))
		if entry["level"] == "ERROR" {
			failures = append(failures, entry)
		}
	}
	require.Len(t, failures, 2)
	require.Equal(t, "not_found", failures[0]["error_class"])
	require.Equal(t, "22012", failures[1]["error_code"])
	require.NotEmpty(t, failures[0]["db_caller"])
	require.NotEmpty(t, failures[1]["db_caller"])
	for _, failure := range failures {
		require.Regexp(t, `^internal/database_test\.TestSafeLoggerPostgresDistinguishesMissingRowAndSQLFailure:[1-9][0-9]*$`, failure["db_caller"])
	}
}
