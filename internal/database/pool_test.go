package database_test

import (
	"sync"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestConfigureConnectionPool проверяет, что лимит открытых соединений реально
// удерживается: при MaxOpenConns=2 и пачке параллельных медленных запросов
// одновременно открыто не больше двух, а лишние ждут очереди (WaitCount>0).
// Открываем отдельное соединение, чтобы не трогать общий пул тестовой БД.
func TestConfigureConnectionPool(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(
		postgres.Open(database.EnsureUTCTimezone(testutil.TestDSN())),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)},
	)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, database.ConfigureConnectionPool(db, database.PoolConfig{
		MaxOpenConns:    2,
		MaxIdleConns:    2,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
	}))

	require.Equal(t, 2, sqlDB.Stats().MaxOpenConnections)

	const workers = 6
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// pg_sleep держит соединение занятым, чтобы пул дошёл до предела.
			require.NoError(t, db.Exec("SELECT pg_sleep(0.1)").Error)
		}()
	}
	wg.Wait()

	stats := sqlDB.Stats()
	require.LessOrEqual(t, stats.OpenConnections, 2, "пул не должен превышать MaxOpenConns")
	require.Positive(t, stats.WaitCount, "часть запросов должна была ждать освобождения соединения")
}
