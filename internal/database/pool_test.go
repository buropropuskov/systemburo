package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingPool запоминает, что и с какими значениями вызвали. Проверять настройку
// пула через живую базу незачем: соединения она не открывает, а тест, требующий
// поднятого контейнера, краснел бы от занятого стенда, а не от ошибки в коде.
type recordingPool struct {
	maxOpen     []int
	maxIdle     []int
	maxLifetime []time.Duration
	maxIdleTime []time.Duration
}

func (p *recordingPool) SetMaxOpenConns(n int)              { p.maxOpen = append(p.maxOpen, n) }
func (p *recordingPool) SetMaxIdleConns(n int)              { p.maxIdle = append(p.maxIdle, n) }
func (p *recordingPool) SetConnMaxLifetime(d time.Duration) { p.maxLifetime = append(p.maxLifetime, d) }
func (p *recordingPool) SetConnMaxIdleTime(d time.Duration) { p.maxIdleTime = append(p.maxIdleTime, d) }

// TestApplyPool_PassesValuesThrough - заданные значения доезжают до пула ровно
// такими, какими их задали.
func TestApplyPool_PassesValuesThrough(t *testing.T) {
	t.Parallel()

	pool := &recordingPool{}
	ApplyPool(pool, PoolConfig{
		MaxOpenConns:    50,
		MaxIdleConns:    25,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
	})

	assert.Equal(t, []int{50}, pool.maxOpen)
	assert.Equal(t, []int{25}, pool.maxIdle)
	assert.Equal(t, []time.Duration{time.Hour}, pool.maxLifetime)
	assert.Equal(t, []time.Duration{10 * time.Minute}, pool.maxIdleTime)
}

// TestApplyPool_ZeroLeavesDriverDefaults: нулевое поле не должно превращаться в
// вызов setter-а. У database/sql неположительный аргумент означает "снять
// ограничение", поэтому передача нулей вернула бы не драйверные умолчания, а третье
// поведение - безлимит открытых соединений вообще без простаивающих.
func TestApplyPool_ZeroLeavesDriverDefaults(t *testing.T) {
	t.Parallel()

	pool := &recordingPool{}
	ApplyPool(pool, PoolConfig{})

	assert.Empty(t, pool.maxOpen)
	assert.Empty(t, pool.maxIdle)
	assert.Empty(t, pool.maxLifetime)
	assert.Empty(t, pool.maxIdleTime)
}

// TestApplyPool_PartialConfig - незаданное поле не мешает применить остальные.
func TestApplyPool_PartialConfig(t *testing.T) {
	t.Parallel()

	pool := &recordingPool{}
	ApplyPool(pool, PoolConfig{MaxOpenConns: 12})

	assert.Equal(t, []int{12}, pool.maxOpen)
	assert.Empty(t, pool.maxIdle)
}

// deadConnector - соединитель, который никогда не соединяется. Нужен, чтобы собрать
// настоящий *sql.DB и прочитать его Stats: sql.OpenDB ленив и до первого запроса в
// базу не ходит, так что предел проверяется на реальном пуле без Postgres рядом.
type deadConnector struct{}

func (deadConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("no connection in test")
}
func (deadConnector) Driver() driver.Driver { return nil }

// TestApplyPool_RealSQLDBReportsLimit - предел доезжает до настоящего пула
// database/sql, а не только до нашего интерфейса.
func TestApplyPool_RealSQLDBReportsLimit(t *testing.T) {
	t.Parallel()

	db := sql.OpenDB(deadConnector{})
	defer db.Close()

	// До настройки database/sql не ограничивает число открытых соединений - именно
	// это умолчание и упирало систему в max_connections самой базы.
	require.Equal(t, 0, db.Stats().MaxOpenConnections)

	ApplyPool(db, PoolConfig{
		MaxOpenConns:    50,
		MaxIdleConns:    25,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
	})

	assert.Equal(t, 50, db.Stats().MaxOpenConnections)
}
