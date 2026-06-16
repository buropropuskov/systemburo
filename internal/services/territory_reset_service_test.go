package services

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL не задан, пропускаем интеграционный тест")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err, "открытие тестовой БД")
	return db
}

func TestTerritoryReset_ResetExitedStatuses(t *testing.T) {
	db := openTestDB(t)

	// Создаём вспомогательный attachment (cars требует attachment_id из-за FK).
	var attachmentID int64
	err := db.Raw(`INSERT INTO attachments (attachment_type, created_at, updated_at) VALUES ('cars', NOW(), NOW()) RETURNING id`).
		Scan(&attachmentID).Error
	require.NoError(t, err, "создание тестового attachment")
	t.Cleanup(func() {
		db.Exec("DELETE FROM attachments WHERE id = ?", attachmentID)
	})

	// Вставляем машины со статусами 0, 1, 2 (через SQL чтобы явно задать territory_status).
	carRows := []int64{}
	for _, status := range []int{0, 1, 2} {
		var id int64
		err := db.Raw(
			`INSERT INTO cars (attachment_id, car_number, territory_status, created_at, updated_at)
			 VALUES (?, ?, ?, NOW(), NOW()) RETURNING id`,
			attachmentID, nil, status,
		).Scan(&id).Error
		require.NoError(t, err, "вставка машины со статусом %d", status)
		carRows = append(carRows, id)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM cars WHERE id IN ?", carRows)
	})

	// Вставляем сотрудников со статусами 0, 1, 2 (attachment_id nullable у employees).
	empRows := []int64{}
	for _, status := range []int{0, 1, 2} {
		var id int64
		err := db.Raw(
			`INSERT INTO employees (territory_status, created_at, updated_at)
			 VALUES (?, NOW(), NOW()) RETURNING id`,
			status,
		).Scan(&id).Error
		require.NoError(t, err, "вставка сотрудника со статусом %d", status)
		empRows = append(empRows, id)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM employees WHERE id IN ?", empRows)
	})

	svc := NewTerritoryResetService(db)
	empReset, carReset, err := svc.ResetExitedStatuses(context.Background())
	require.NoError(t, err)

	// Счётчики: ровно по одному с territory_status=2 в наших строках.
	assert.GreaterOrEqual(t, empReset, int64(1), "ожидался сброс минимум 1 сотрудника")
	assert.GreaterOrEqual(t, carReset, int64(1), "ожидался сброс минимум 1 машины")

	// Проверяем итоговые статусы вставленных машин.
	type statusRow struct {
		ID              int64
		TerritoryStatus *int
	}

	var gotCars []statusRow
	require.NoError(t, db.Raw(`SELECT id, territory_status FROM cars WHERE id IN ?`, carRows).Scan(&gotCars).Error)
	require.Len(t, gotCars, 3)
	carStatuses := map[int]int{}
	for _, c := range gotCars {
		if c.TerritoryStatus != nil {
			carStatuses[*c.TerritoryStatus]++
		} else {
			carStatuses[0]++
		}
	}
	assert.Equal(t, 2, carStatuses[0], "две машины со статусом 0 (исходная + сброшенная)")
	assert.Equal(t, 1, carStatuses[1], "одна машина со статусом 1 - не тронута")
	assert.Equal(t, 0, carStatuses[2], "ни одной машины со статусом 2 после сброса")

	// Проверяем итоговые статусы вставленных сотрудников.
	var gotEmps []statusRow
	require.NoError(t, db.Raw(`SELECT id, territory_status FROM employees WHERE id IN ?`, empRows).Scan(&gotEmps).Error)
	require.Len(t, gotEmps, 3)
	empStatuses := map[int]int{}
	for _, e := range gotEmps {
		if e.TerritoryStatus != nil {
			empStatuses[*e.TerritoryStatus]++
		} else {
			empStatuses[0]++
		}
	}
	assert.Equal(t, 2, empStatuses[0], "два сотрудника со статусом 0 (исходный + сброшенный)")
	assert.Equal(t, 1, empStatuses[1], "один сотрудник со статусом 1 - не тронут")
	assert.Equal(t, 0, empStatuses[2], "ни одного сотрудника со статусом 2 после сброса")
}
