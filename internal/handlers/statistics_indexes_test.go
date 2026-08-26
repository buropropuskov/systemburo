package handlers_test

import (
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatisticsIndexes_Created проверяет, что аддитивные индексы аналитики (#632)
// создаются AutoMigrate. Индексы под фильтр даты заявок, движок въездов/входов
// (audit_log по entity/created_at), выборки аудита по действию за период
// (лента журнала обработки, #1251) и список машин по статусу — ускоряют реальные
// запросы отчётов.
func TestStatisticsIndexes_Created(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	expected := []string{
		"idx_applications_sending_datetime",
		"idx_audit_entity",
		"idx_audit_entity_action",
		"idx_cars_territory_status",
	}
	for _, idx := range expected {
		var count int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pg_indexes WHERE indexname = ?`, idx).Scan(&count).Error)
		assert.Equal(t, int64(1), count, "индекс %s должен существовать", idx)
	}

	// Составные индексы audit_log проверяем по СОСТАВУ, а не только по имени:
	// порядок колонок задаётся priority в gorm-тегах модели, и его перестановка
	// сохраняет имя индекса, но обесценивает выборку (ведущая колонка перестаёт
	// совпадать с предикатом запроса).
	composite := map[string]string{
		"idx_audit_entity":        "(entity_type, entity_id, created_at)",
		"idx_audit_entity_action": "(entity_type, action, created_at)",
	}
	for idx, cols := range composite {
		var def string
		require.NoError(t, db.Raw(`SELECT indexdef FROM pg_indexes WHERE indexname = ?`, idx).Scan(&def).Error)
		assert.Contains(t, def, cols, "индекс %s должен идти по колонкам %s", idx, cols)
	}
}
