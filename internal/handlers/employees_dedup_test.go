package handlers_test

// TestEmployeeDedup_* проверяют дедупликацию сотрудников в GetActiveEmployeesForTable:
// человек с несколькими активными вложениями должен показываться одной строкой
// с максимальным entry_date_to. Живёт в handlers_test (а не services), потому что
// CI гоняет пакеты параллельно (-p 4) на общей auto_registry_test, и CleanDB из
// соседнего пакета снёс бы наши строки. Внутри одного пакета handlers_test тесты
// идут сериально, поэтому CleanDB изолирует прогон.

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEmployeeDedup_OnePersonTwoAttachments: один человек (одинаковый hmac паспорта),
// два активных вложения с разными entry_date_to - ожидаем ОДНУ строку с большей датой.
func TestEmployeeDedup_OnePersonTwoAttachments(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	today := time.Now()
	dateFrom := today.AddDate(0, 0, -1).Format("2006-01-02") // вчера - оба вложения активны
	dateTo1 := today.AddDate(0, 0, 5).Format("2006-01-02")   // сегодня+5
	dateTo2 := today.AddDate(0, 0, 15).Format("2006-01-02")  // сегодня+15 (большая)

	// Организация
	var orgID int64
	require.NoError(t, db.Raw(
		`INSERT INTO organizations (name) VALUES ('DedupOrg') RETURNING id`,
	).Scan(&orgID).Error)

	// Пользователь-отправитель (минимальный)
	var senderUserID int64
	require.NoError(t, db.Raw(
		`INSERT INTO users (username, password, type_id, organization_id)
		 VALUES ('dedup_sender', 'x', 1, ?) RETURNING id`, orgID,
	).Scan(&senderUserID).Error)

	// Заявка (confirmation=approved, status=in_work)
	confirmation := "Согласовано"
	status := "В работе"
	var appID int64
	require.NoError(t, db.Raw(
		`INSERT INTO applications (organization_id, sender_user_id, confirmation, status)
		 VALUES (?, ?, ?, ?) RETURNING id`,
		orgID, senderUserID, confirmation, status,
	).Scan(&appID).Error)

	// system_table (тип people)
	dn := "Dedup Table"
	var tableID int64
	require.NoError(t, db.Raw(
		`INSERT INTO system_tables (name, display_name, table_type, is_active, created_at, updated_at)
		 VALUES ('dedup_table', ?, 'people', true, NOW(), NOW()) RETURNING id`, dn,
	).Scan(&tableID).Error)

	// Уникальный HMAC паспорта (имитируем - просто строка)
	passHMAC := "dedup_test_hmac_abc123"

	// Вложение 1 - entry_date_to = dateTo1
	var att1ID int64
	require.NoError(t, db.Raw(
		`INSERT INTO attachments (application_id, attachment_type, entry_date_from, entry_date_to,
		  entry_time_from, entry_time_to, created_at, updated_at)
		 VALUES (?, 'people', ?, ?, '08:00', '18:00', NOW(), NOW()) RETURNING id`,
		appID, dateFrom, dateTo1,
	).Scan(&att1ID).Error)

	// Сотрудник 1 - привязан к вложению 1
	ln1, fn1 := "Дедупов", "Тест"
	statusActive := 1
	var emp1ID int64
	require.NoError(t, db.Raw(
		`INSERT INTO employees (attachment_id, last_name, first_name, status,
		  passport_series_number_hmac, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, NOW(), NOW()) RETURNING id`,
		att1ID, ln1, fn1, statusActive, passHMAC,
	).Scan(&emp1ID).Error)

	// Привязка сотрудника 1 к таблице
	require.NoError(t, db.Exec(
		`INSERT INTO employee_target_tables (employee_id, table_id) VALUES (?, ?)`,
		emp1ID, tableID,
	).Error)

	// Вложение 2 - entry_date_to = dateTo2 (большая)
	var att2ID int64
	require.NoError(t, db.Raw(
		`INSERT INTO attachments (application_id, attachment_type, entry_date_from, entry_date_to,
		  entry_time_from, entry_time_to, created_at, updated_at)
		 VALUES (?, 'people', ?, ?, '09:00', '19:00', NOW(), NOW()) RETURNING id`,
		appID, dateFrom, dateTo2,
	).Scan(&att2ID).Error)

	// Сотрудник 2 - тот же человек (тот же hmac), другое вложение
	var emp2ID int64
	require.NoError(t, db.Raw(
		`INSERT INTO employees (attachment_id, last_name, first_name, status,
		  passport_series_number_hmac, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, NOW(), NOW()) RETURNING id`,
		att2ID, ln1, fn1, statusActive, passHMAC,
	).Scan(&emp2ID).Error)

	// Привязка сотрудника 2 к таблице
	require.NoError(t, db.Exec(
		`INSERT INTO employee_target_tables (employee_id, table_id) VALUES (?, ?)`,
		emp2ID, tableID,
	).Error)

	// Вызов сервиса
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	result, err := svc.GetActiveEmployeesForTable(context.Background(), int(tableID))
	require.NoError(t, err)

	// Ожидаем ОДНУ строку - человек схлопнулся
	require.Len(t, result, 1, "один человек с двумя вложениями должен давать одну строку")

	// Строка должна содержать МАКСИМАЛЬНУЮ дату (dateTo2 = сегодня+15)
	assert.Equal(t, dateTo2, *result[0].EntryDateTo,
		"entry_date_to должен быть от вложения с большей датой")
}

// TestEmployeeDedup_NullPassportNotCollapsed: сотрудник без паспорта (hmac IS NULL)
// в двух вложениях - обе строки должны остаться, схлопывания нет.
func TestEmployeeDedup_NullPassportNotCollapsed(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	today := time.Now()
	dateFrom := today.AddDate(0, 0, -1).Format("2006-01-02")
	dateTo1 := today.AddDate(0, 0, 5).Format("2006-01-02")
	dateTo2 := today.AddDate(0, 0, 15).Format("2006-01-02")

	// Организация
	var orgID int64
	require.NoError(t, db.Raw(
		`INSERT INTO organizations (name) VALUES ('DedupNullOrg') RETURNING id`,
	).Scan(&orgID).Error)

	// Пользователь-отправитель
	var senderUserID int64
	require.NoError(t, db.Raw(
		`INSERT INTO users (username, password, type_id, organization_id)
		 VALUES ('dedup_null_sender', 'x', 1, ?) RETURNING id`, orgID,
	).Scan(&senderUserID).Error)

	// Заявка
	confirmation := "Согласовано"
	status := "В работе"
	var appID int64
	require.NoError(t, db.Raw(
		`INSERT INTO applications (organization_id, sender_user_id, confirmation, status)
		 VALUES (?, ?, ?, ?) RETURNING id`,
		orgID, senderUserID, confirmation, status,
	).Scan(&appID).Error)

	// system_table
	dn := "Dedup Null Table"
	var tableID int64
	require.NoError(t, db.Raw(
		`INSERT INTO system_tables (name, display_name, table_type, is_active, created_at, updated_at)
		 VALUES ('dedup_null_table', ?, 'people', true, NOW(), NOW()) RETURNING id`, dn,
	).Scan(&tableID).Error)

	statusActive := 1

	// Вложение 1
	var att1ID int64
	require.NoError(t, db.Raw(
		`INSERT INTO attachments (application_id, attachment_type, entry_date_from, entry_date_to,
		  created_at, updated_at)
		 VALUES (?, 'people', ?, ?, NOW(), NOW()) RETURNING id`,
		appID, dateFrom, dateTo1,
	).Scan(&att1ID).Error)

	// Сотрудник "По факту" (hmac IS NULL)
	ln := "Бесфактный"
	var emp1ID int64
	require.NoError(t, db.Raw(
		`INSERT INTO employees (attachment_id, last_name, first_name, status,
		  passport_series_number_hmac, created_at, updated_at)
		 VALUES (?, ?, 'Иван', ?, NULL, NOW(), NOW()) RETURNING id`,
		att1ID, ln, statusActive,
	).Scan(&emp1ID).Error)

	require.NoError(t, db.Exec(
		`INSERT INTO employee_target_tables (employee_id, table_id) VALUES (?, ?)`,
		emp1ID, tableID,
	).Error)

	// Вложение 2
	var att2ID int64
	require.NoError(t, db.Raw(
		`INSERT INTO attachments (application_id, attachment_type, entry_date_from, entry_date_to,
		  created_at, updated_at)
		 VALUES (?, 'people', ?, ?, NOW(), NOW()) RETURNING id`,
		appID, dateFrom, dateTo2,
	).Scan(&att2ID).Error)

	// Второй сотрудник "По факту" - тоже NULL hmac
	var emp2ID int64
	require.NoError(t, db.Raw(
		`INSERT INTO employees (attachment_id, last_name, first_name, status,
		  passport_series_number_hmac, created_at, updated_at)
		 VALUES (?, ?, 'Пётр', ?, NULL, NOW(), NOW()) RETURNING id`,
		att2ID, ln, statusActive,
	).Scan(&emp2ID).Error)

	require.NoError(t, db.Exec(
		`INSERT INTO employee_target_tables (employee_id, table_id) VALUES (?, ?)`,
		emp2ID, tableID,
	).Error)

	// Вызов сервиса
	svc := services.NewEmployeeService(db, services.NewAuditRecorder(db))
	result, err := svc.GetActiveEmployeesForTable(context.Background(), int(tableID))
	require.NoError(t, err)

	// Ожидаем ДВЕ строки - NULL-паспорт не схлопывается
	assert.Len(t, result, 2, "сотрудники без паспорта (hmac NULL) не должны схлопываться")
}
