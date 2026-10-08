package testutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDatabaseLeaseExcludesAnotherConnectionUntilRelease(t *testing.T) {
	db := initTestDB()
	// Independent from an already initialized SetupTestApp in this same binary.
	const key = testDatabaseLeaseKey + 1
	lease, err := acquireTestDatabaseLease(key)
	require.NoError(t, err)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = lease.conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", key)
		_ = lease.conn.Close()
		_ = lease.pool.Close()
	})
	pool, err := db.DB()
	require.NoError(t, err)
	other, err := pool.Conn(ctx)
	require.NoError(t, err)
	defer other.Close()
	var acquired bool
	require.NoError(t, other.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired))
	require.False(t, acquired, "another test process must not prepare or clean the shared database")
	_, err = lease.conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", key)
	require.NoError(t, err)
	require.NoError(t, other.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired))
	require.True(t, acquired, "the next process may proceed after the lease is released")
	_, err = other.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", key)
	require.NoError(t, err)
}
