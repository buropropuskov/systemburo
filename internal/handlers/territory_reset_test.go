package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTerritoryReset_ResetExitedStatuses проверяет, что ежедневный сброс переводит
// только статус "Покинул/Выехал" (territory_status=2) в "Не входил/Не въезжал" (0),
// а "На территории" (1) не трогает. Живёт в handlers_test (а не services), потому что
// это интеграционный тест с БД: CI гоняет пакеты параллельно (-p 4) на общей
// auto_registry_test, и CleanDB из соседнего пакета снёс бы наши строки. Внутри одного
// пакета handlers_test тесты идут сериально, поэтому CleanDB изолирует прогон.
func TestTerritoryReset_ResetExitedStatuses(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	// Вспомогательный attachment: cars.attachment_id NOT NULL.
	var attachmentID int64
	require.NoError(t, db.Raw(
		`INSERT INTO attachments (attachment_type, created_at, updated_at)
		 VALUES ('cars', NOW(), NOW()) RETURNING id`).Scan(&attachmentID).Error)

	// Машины и сотрудники со статусами 0, 1, 2.
	carRows := []int64{}
	for _, status := range []int{0, 1, 2} {
		var id int64
		require.NoError(t, db.Raw(
			`INSERT INTO cars (attachment_id, territory_status, created_at, updated_at)
			 VALUES (?, ?, NOW(), NOW()) RETURNING id`, attachmentID, status).Scan(&id).Error,
			"вставка машины со статусом %d", status)
		carRows = append(carRows, id)
	}
	empRows := []int64{}
	for _, status := range []int{0, 1, 2} {
		var id int64
		require.NoError(t, db.Raw(
			`INSERT INTO employees (territory_status, created_at, updated_at)
			 VALUES (?, NOW(), NOW()) RETURNING id`, status).Scan(&id).Error,
			"вставка сотрудника со статусом %d", status)
		empRows = append(empRows, id)
	}

	svc := services.NewTerritoryResetService(db)
	empReset, carReset, err := svc.ResetExitedStatuses(context.Background())
	require.NoError(t, err)

	// Минимум по одному (в БД могли быть и другие status=2 от сидов - проверяем итог по своим строкам).
	assert.GreaterOrEqual(t, empReset, int64(1), "ожидался сброс минимум 1 сотрудника")
	assert.GreaterOrEqual(t, carReset, int64(1), "ожидался сброс минимум 1 машины")

	type statusRow struct {
		ID              int64
		TerritoryStatus *int
	}
	countByStatus := func(rows []statusRow) map[int]int {
		m := map[int]int{}
		for _, r := range rows {
			if r.TerritoryStatus != nil {
				m[*r.TerritoryStatus]++
			} else {
				m[0]++
			}
		}
		return m
	}

	var gotCars []statusRow
	require.NoError(t, db.Raw(`SELECT id, territory_status FROM cars WHERE id IN ?`, carRows).Scan(&gotCars).Error)
	require.Len(t, gotCars, 3)
	carStatuses := countByStatus(gotCars)
	assert.Equal(t, 2, carStatuses[0], "две машины со статусом 0 (исходная + сброшенная)")
	assert.Equal(t, 1, carStatuses[1], "одна машина со статусом 1 - не тронута")
	assert.Equal(t, 0, carStatuses[2], "ни одной машины со статусом 2 после сброса")

	var gotEmps []statusRow
	require.NoError(t, db.Raw(`SELECT id, territory_status FROM employees WHERE id IN ?`, empRows).Scan(&gotEmps).Error)
	require.Len(t, gotEmps, 3)
	empStatuses := countByStatus(gotEmps)
	assert.Equal(t, 2, empStatuses[0], "два сотрудника со статусом 0 (исходный + сброшенный)")
	assert.Equal(t, 1, empStatuses[1], "один сотрудник со статусом 1 - не тронут")
	assert.Equal(t, 0, empStatuses[2], "ни одного сотрудника со статусом 2 после сброса")
}
