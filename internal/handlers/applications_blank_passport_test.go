package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSubmitCompleteApplication_BlankPassportStoredAsNull: паспорт опционален (у мигрантов
// вместо него патент), и группу без паспортов подают на мероприятия целыми списками. Если
// путь записи кладёт указатель на "", HMAC у всех получается одинаковым, и дедуп
// PARTITION BY passport_series_number_hmac в GetActiveEmployeesForTable оставляет охраннику
// одного человека из всей группы. Проверяем обе стороны: NULL в базе и обе строки в таблице.
func TestSubmitCompleteApplication_BlankPassportStoredAsNull(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	const orgName = "Test Organization"
	token := testutil.RegisterAndLogin(t, e, "blank_pass_user", "pass123", 1, td.OrgID, td.CompanyID)
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)
	uaID := seedUniqueAttachment(t, db, "people", "blank_pass_tmpl", "People Template")

	body := fmt.Sprintf(`{
		"message": "группа без паспортов",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "people",
			"attachment_name": "people_tmpl",
			"attachment_display_name": "People Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"employees": [
				{"last_name": "Безпаспортов", "first_name": "Первый", "citizenship_id": %d, "position": "Worker", "passport_series_number": "", "patent_number": "", "target_tables": [%d]},
				{"last_name": "Безпаспортов", "first_name": "Второй", "citizenship_id": %d, "position": "Worker", "passport_series_number": "   ", "target_tables": [%d]}
			]}
		}]
	}`, orgName, uaID, citizenshipID, tableID, citizenshipID, tableID)

	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	var withHMAC int64
	require.NoError(t, db.Raw(
		`SELECT COUNT(*) FROM employees
		 WHERE last_name = 'Безпаспортов'
		   AND (passport_series_number_hmac IS NOT NULL OR patent_number_hmac IS NOT NULL)`,
	).Scan(&withHMAC).Error)
	assert.Zero(t, withHMAC, "незаполненные паспорт и патент должны храниться как NULL, а не как HMAC пустой строки")

	// Активируем заявку: сотрудники видны в таблице только со status = 1 у согласованной
	// заявки в работе (см. GetActiveEmployeesForTable).
	require.NoError(t, db.Exec(
		`UPDATE applications SET confirmation = 'Согласовано', status = 'В работе'
		 WHERE sender_user_id = (SELECT id FROM users WHERE username = 'blank_pass_user')`,
	).Error)
	require.NoError(t, db.Exec(
		`UPDATE employees SET status = 1 WHERE last_name = 'Безпаспортов'`,
	).Error)

	rows, err := services.NewEmployeeService(db, services.NewAuditRecorder(db)).
		GetActiveEmployeesForTable(context.Background(), tableID)
	require.NoError(t, err)
	assert.Len(t, rows, 2, "оба сотрудника без паспорта должны остаться в таблице проходной")
}
