package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PoolConfig задаёт параметры пула соединений database/sql под открытым GORM.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// ConfigureConnectionPool применяет параметры пула к открытому *gorm.DB.
// По умолчанию database/sql не ограничивает число открытых соединений и держит
// лишь 2 idle: под пиковой нагрузкой это либо упирается в postgres
// max_connections, либо рвёт и заново открывает соединения на каждом всплеске.
// Нулевые поля не трогаются - остаётся дефолт драйвера.
func ConfigureConnectionPool(db *gorm.DB, cfg PoolConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB for pool config: %w", err)
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
	return nil
}
