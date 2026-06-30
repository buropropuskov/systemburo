package handlers_test

import (
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStatisticsIndexes_Created проверяет, что аддитивные индексы аналитики (#632)
// создаются AutoMigrate. Индексы под фильтр даты заявок, движок въездов/входов
// (audit_log по entity/created_at) и список машин по статусу — ускоряют реальные
// запросы отчётов.
func TestStatisticsIndexes_Created(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	expected := []string{
		"idx_applications_sending_datetime",
		"idx_audit_entity",
		"idx_cars_territory_status",
	}
	for _, idx := range expected {
		var count int64
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM pg_indexes WHERE indexname = ?`, idx).Scan(&count).Error)
		assert.Equal(t, int64(1), count, "индекс %s должен существовать", idx)
	}
}
