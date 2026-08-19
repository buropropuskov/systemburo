package database_test

import (
	"fmt"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// TestLogPartitionMaintenance проверяет сквозной путь партиционирования
// request_logs: таблица партиционирована, а воркер сворачивает партицию старше
// retention в дневные агрегаты (с нормализацией endpoint) и дропает её.
func TestLogPartitionMaintenance(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// request_logs должна быть нативно партиционированной (relkind 'p').
	// Схему указываем явно: тестовая оснастка держит копии таблиц в схеме
	// снимка, и запрос по одному имени возвращал две строки, из которых
	// выбиралась произвольная.
	var relkind string
	require.NoError(t, db.Raw(`SELECT c.relkind FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname='request_logs' AND n.nspname='public'`).Scan(&relkind).Error)
	require.Equal(t, "p", relkind, "request_logs должна быть партиционированной")

	// Готовим партицию на 40 дней назад и кладём в неё запись с числовым
	// сегментом в URL - агрегатор должен нормализовать его в :id.
	old := time.Now().UTC().AddDate(0, 0, -40)
	part := "request_logs_" + old.Format("2006_01_02")
	createPart := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF request_logs FOR VALUES FROM ('%s') TO ('%s')`,
		part, old.Format("2006-01-02"), old.AddDate(0, 0, 1).Format("2006-01-02"),
	)
	require.NoError(t, db.Exec(createPart).Error)
	// Уникальный путь, которого нет в других тестовых данных (тесты делят одну БД).
	require.NoError(t, db.Exec(
		`INSERT INTO request_logs (url, method, response_status, duration_ms, created_at) VALUES (?,?,?,?,?)`,
		"/api/zzz-parttest/4517?x=1", "GET", 200, 12, old,
	).Error)

	// detailDays=30 -> партиция возрастом 40 дней сворачивается и дропается.
	require.NoError(t, database.MaintainLogPartitions(db, 30, 7, 36))

	// Партиция удалена.
	var partCount int
	require.NoError(t, db.Raw(`SELECT count(*) FROM pg_class WHERE relname=?`, part).Scan(&partCount).Error)
	require.Equal(t, 0, partCount, "старая партиция должна быть дропнута")

	// Агрегат записан с нормализованным endpoint (/api/employees/:id).
	var reqCount int64
	require.NoError(t, db.Raw(
		`SELECT request_count FROM request_logs_daily WHERE endpoint=? AND method='GET' AND status_class=2`,
		"/api/zzz-parttest/:id",
	).Scan(&reqCount).Error)
	require.Equal(t, int64(1), reqCount, "лог должен попасть в дневной агрегат с нормализованным endpoint")
}

// TestPDAuditPartitioned проверяет, что pd_audit_logs тоже партиционирована
// (помесячно) и старые партиции дропаются по retention.
func TestPDAuditPartitioned(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	var relkind string
	require.NoError(t, db.Raw(`SELECT c.relkind FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname='pd_audit_logs' AND n.nspname='public'`).Scan(&relkind).Error)
	require.Equal(t, "p", relkind, "pd_audit_logs должна быть партиционированной")

	// Партиция на 40 месяцев назад (за пределами retention=36) должна дропнуться.
	old := time.Now().UTC().AddDate(0, -40, 0)
	part := "pd_audit_logs_" + old.Format("2006_01")
	createPart := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF pd_audit_logs FOR VALUES FROM ('%s') TO ('%s')`,
		part, old.Format("2006-01")+"-01", old.AddDate(0, 1, 0).Format("2006-01")+"-01",
	)
	require.NoError(t, db.Exec(createPart).Error)

	require.NoError(t, database.MaintainLogPartitions(db, 30, 7, 36))

	var partCount int
	require.NoError(t, db.Raw(`SELECT count(*) FROM pg_class WHERE relname=?`, part).Scan(&partCount).Error)
	require.Equal(t, 0, partCount, "партиция аудита старше retention должна быть дропнута")
}

// Свёртка партиции сохраняет длительности в микросекундах: по миллисекундной
// колонке треть запросов давала ноль, и перцентили в истории вырождались (#2125).
// Записи, сделанные до перехода, читаются из миллисекунд.
func TestLogAggregateKeepsMicroseconds(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	old := time.Now().UTC().AddDate(0, 0, -45)
	part := "request_logs_" + old.Format("2006_01_02")
	require.NoError(t, db.Exec(fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF request_logs FOR VALUES FROM ('%s') TO ('%s')`,
		part, old.Format("2006-01-02"), old.AddDate(0, 0, 1).Format("2006-01-02"),
	)).Error)

	// Быстрый ответ: в миллисекундах это ноль, в микросекундах 400.
	require.NoError(t, db.Exec(
		`INSERT INTO request_logs (url, method, response_status, duration_ms, duration_us, created_at)
		 VALUES (?,?,?,?,?,?)`,
		"/api/zzz-us-fast", "GET", 200, 0, 400, old,
	).Error)
	// Запись до перехода: микросекунд нет, берём миллисекунды.
	require.NoError(t, db.Exec(
		`INSERT INTO request_logs (url, method, response_status, duration_ms, created_at)
		 VALUES (?,?,?,?,?)`,
		"/api/zzz-us-legacy", "GET", 200, 12, old,
	).Error)

	require.NoError(t, database.MaintainLogPartitions(db, 30, 7, 36))

	var fastUs, legacyUs int64
	require.NoError(t, db.Raw(
		`SELECT avg_duration_us FROM request_logs_daily WHERE endpoint = ?`, "/api/zzz-us-fast",
	).Scan(&fastUs).Error)
	require.NoError(t, db.Raw(
		`SELECT avg_duration_us FROM request_logs_daily WHERE endpoint = ?`, "/api/zzz-us-legacy",
	).Scan(&legacyUs).Error)

	require.Equal(t, int64(400), fastUs, "быстрый ответ не должен схлопываться в ноль")
	require.Equal(t, int64(12_000), legacyUs, "старая запись читается из миллисекунд")
}

// Экран мониторинга фильтрует журнал по статусу и методу, всегда за период и всегда
// от свежих записей. Без пары (поле, created_at) выборка «только ошибки» читала все
// партиции последовательно, поэтому индексы досоздаются и на стендах, где таблица
// была создана раньше них (#2125).
func TestLogFilterIndexes(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	for _, idx := range []string{
		"idx_request_logs_status_created",
		"idx_request_logs_method_created",
	} {
		var count int
		require.NoError(t, db.Raw(
			`SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND tablename='request_logs' AND indexname=?`,
			idx,
		).Scan(&count).Error)
		require.Equal(t, 1, count, "индекс %s должен быть на request_logs", idx)
	}
}
