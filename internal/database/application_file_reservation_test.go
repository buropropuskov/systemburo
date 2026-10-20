package database_test

import (
	"errors"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestApplicationFileReservationMigration(t *testing.T) {
	registered := false
	for _, model := range database.AllModels() {
		if _, ok := model.(*models.ApplicationFileReservation); ok {
			registered = true
		}
	}
	require.True(t, registered)
	db := openEntityPeriodTestDB(t)
	rollback := errors.New("rollback owned quota schema")
	err := db.Transaction(func(tx *gorm.DB) error {
		schema := "quota_" + uuid.New().String()[:8]
		require.NoError(t, tx.Exec("CREATE SCHEMA "+schema).Error)
		require.NoError(t, tx.Exec("SET LOCAL search_path = "+schema+", pg_catalog").Error)
		var active string
		require.NoError(t, tx.Raw("SELECT current_schema()").Scan(&active).Error)
		require.Equal(t, schema, active)
		require.NoError(t, tx.Exec("CREATE TABLE application_files (id bigint PRIMARY KEY, file_size bigint NOT NULL)").Error)
		require.NoError(t, tx.Exec("INSERT INTO application_files VALUES (1,314),(2,159)").Error)
		for i := 0; i < 2; i++ {
			require.NoError(t, tx.AutoMigrate(&models.ApplicationFileReservation{}))
		}
		var sizes []int64
		require.NoError(t, tx.Table("application_files").Order("id").Pluck("file_size", &sizes).Error)
		require.Equal(t, []int64{314, 159}, sizes)
		var indexes int64
		require.NoError(t, tx.Raw("SELECT COUNT(*) FROM pg_indexes WHERE schemaname = ? AND tablename = ? AND indexname IN (?,?)", schema, "application_file_reservations", "idx_file_reservation_owner_state", "idx_file_reservation_state_expiry").Scan(&indexes).Error)
		require.EqualValues(t, 2, indexes)
		base := models.ApplicationFileReservation{UploadedBy: 1, State: models.ApplicationFileReservationActive, ReservedCount: 1, ReservedBytes: 10, StoredNames: []string{}, WorkerID: uuid.NewString(), CreatedAt: time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2040, 1, 1, 0, 5, 0, 0, time.UTC)}
		for _, invalid := range []func(*models.ApplicationFileReservation){
			func(row *models.ApplicationFileReservation) { row.UploadedBy = 0 },
			func(row *models.ApplicationFileReservation) { row.ReservedCount = 0 },
			func(row *models.ApplicationFileReservation) { row.ReservedBytes = -1 },
			func(row *models.ApplicationFileReservation) { row.PreparedBytes = 11 },
			func(row *models.ApplicationFileReservation) { row.State = "invalid" },
		} {
			row := base
			row.ID = uuid.NewString()
			invalid(&row)
			failed := tx.Transaction(func(savepoint *gorm.DB) error { return savepoint.Create(&row).Error })
			var pg *pgconn.PgError
			require.ErrorAs(t, failed, &pg)
			require.Equal(t, "23514", pg.Code)
		}
		base.ID = uuid.NewString()
		require.NoError(t, tx.Create(&base).Error)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
