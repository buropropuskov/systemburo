package database

import (
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PoolConfig - параметры пула соединений database/sql под открытым GORM.
//
// Нулевое поле означает "не трогать": остаётся умолчание драйвера. Так параметр,
// который администратор не задал и не захотел задавать, не подменяется молча нашим
// представлением о правильном значении - подменяет его config.Config своими
// envDefault, и там это видно.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// ConnPool - та часть пула database/sql, которую здесь настраивают. Интерфейс, а не
// *sql.DB, чтобы применение параметров проверялось тестом без живой базы: сама
// настройка соединения не открывает и в базу не ходит, и требовать её ради проверки
// значило бы получить тест, который краснеет от занятого контейнера, а не от ошибки.
type ConnPool interface {
	SetMaxOpenConns(n int)
	SetMaxIdleConns(n int)
	SetConnMaxLifetime(d time.Duration)
	SetConnMaxIdleTime(d time.Duration)
}

var _ ConnPool = (*sql.DB)(nil)

// ApplyPool переносит параметры в пул. Неположительные значения пропускаются: у
// database/sql все четыре setter-а трактуют их как "снять ограничение", то есть
// применение пустого PoolConfig не вернуло бы драйверные умолчания, а выдало бы
// третье поведение - безлимитное число соединений без единого простаивающего.
func ApplyPool(pool ConnPool, cfg PoolConfig) {
	if cfg.MaxOpenConns > 0 {
		pool.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		pool.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		pool.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		pool.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
}

// ConfigureConnectionPool достаёт пул из открытого GORM и применяет параметры.
//
// До этой настройки действовали умолчания драйвера: число открытых соединений не
// ограничено, простаивающих держится два. Первое означает, что всплеск запросов
// упирается в max_connections самой базы и возвращается отказами, второе - что
// соединение открывается почти на каждый запрос, а Postgres форкает под него
// отдельный процесс.
func ConfigureConnectionPool(db *gorm.DB, cfg PoolConfig) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB for pool config: %w", err)
	}
	ApplyPool(sqlDB, cfg)
	return nil
}
