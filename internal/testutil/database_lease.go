package testutil

import (
	"context"
	"database/sql"
	"fmt"

	"systemburo/internal/database"
)

const testDatabaseLeaseKey int64 = 774219740002

// Keep one dedicated connection for this test binary's lifetime. Preparation
// alone is insufficient: another package can truncate tables while this one is
// running assertions. PostgreSQL releases the lease when the process exits,
// including a failed test binary. Pure and isolated-schema tests remain parallel.
var testDatabaseLease *databaseLease

type databaseLease struct {
	conn *sql.Conn
	pool *sql.DB
}

func acquireTestDatabaseLease(key int64) (*databaseLease, error) {
	// Never reserve a slot in cachedDB: transaction-connection regression tests
	// deliberately limit its pool to one connection. This pool owns only the lease.
	pool, err := sql.Open("pgx", database.EnsureUTCTimezone(getTestDSN()))
	if err != nil {
		return nil, fmt.Errorf("test database lease pool: %w", err)
	}
	pool.SetMaxOpenConns(1)
	ctx := context.Background()
	conn, err := pool.Conn(ctx)
	if err != nil {
		_ = pool.Close()
		return nil, fmt.Errorf("test database lease connection: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", key); err != nil {
		_ = conn.Close()
		_ = pool.Close()
		return nil, fmt.Errorf("test database lease: %w", err)
	}
	return &databaseLease{conn: conn, pool: pool}, nil
}
