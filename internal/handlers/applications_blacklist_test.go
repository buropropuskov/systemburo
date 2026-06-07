package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// submitCarApp подаёт полную заявку с одной машиной (номер + mark_id) и возвращает recorder.
func submitCarApp(t *testing.T, e *echo.Echo, db *gorm.DB, token, orgName, tag, carNumber string, markID int) *httptest.ResponseRecorder {
	t.Helper()
	uaID := seedUniqueAttachment(t, db, "cars", fmt.Sprintf("car_tmpl_%s_%s", t.Name(), tag), "Car Template")
	body := fmt.Sprintf(`{
		"message": "blacklist guard",
		"organization": "%s",
		"responsible_person": "Test",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "car_tmpl",
			"attachment_display_name": "Car Template",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"vehicles": [{"car_number": "%s", "car_brand": "Kamaz", "mark_id": %d}]}
		}]
	}`, orgName, uaID, carNumber, markID)
	return testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
}

// submitPersonApp подаёт полную заявку с одним человеком (ФИО) и возвращает recorder.
func submitPersonApp(t *testing.T, e *echo.Echo, db *gorm.DB, token, orgName, tag, last, first, middle string, citizenshipID, tableID int) *httptest.ResponseRecorder {
	t.Helper()
	uaID := seedUniqueAttachment(t, db, "people", fmt.Sprintf("emp_tmpl_%s_%s", t.Name(), tag), "Employee Template")
	body := fmt.Sprintf(`{
		"message": "blacklist guard",
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
			"data": {"employees": [{"last_name": "%s", "first_name": "%s", "middle_name": "%s", "citizenship_id": %d, "position": "Worker", "passport_series_number": "1234 567890", "target_tables": [%d]}]}
		}]
	}`, orgName, uaID, last, first, middle, citizenshipID, tableID)
	return testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
}

// TestSubmitCompleteApplication_BlacklistGuard проверяет серверный гард ЧС (#443):
// заявку с заблокированной машиной/человеком отклоняем 409 (на случай обхода фронта),
// а чистую - пропускаем.
func TestSubmitCompleteApplication_BlacklistGuard(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	ctx := context.Background()

	const orgName = "Test Organization"
	token := testutil.RegisterAndLogin(t, e, "bl_guard_user", "pass123", 1, td.OrgID, td.CompanyID)
	userID := getUserID(t, db, "bl_guard_user")

	mark := seedMark(t, db, "BL_Guard_Mark")
	citizenshipID := seedCitizenship(t, db)
	tableID := seedSystemTable(t, db)

	_, err := newVehicleBlacklistService(db).Create(ctx, models.CreateVehicleBlacklistRequest{
		CarNumber: "C777CC799", MarkID: mark.ID, Reason: "угон",
	}, userID)
	require.NoError(t, err)

	_, err = newPersonBlacklistService(db).Create(ctx, models.CreatePersonBlacklistRequest{
		LastName: "Petrov", FirstName: "Petr", MiddleName: "Petrovich", Reason: "нарушение",
	}, userID)
	require.NoError(t, err)

	t.Run("заблокированную машину нельзя подать", func(t *testing.T) {
		rec := submitCarApp(t, e, db, token, orgName, "blocked", "C777CC799", mark.ID)
		require.Equal(t, http.StatusConflict, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "чёрном списке")
	})

	t.Run("машину не из ЧС подать можно", func(t *testing.T) {
		rec := submitCarApp(t, e, db, token, orgName, "clean", "D888DD799", mark.ID)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	})

	t.Run("заблокированного человека нельзя подать", func(t *testing.T) {
		rec := submitPersonApp(t, e, db, token, orgName, "blocked", "Petrov", "Petr", "Petrovich", citizenshipID, tableID)
		require.Equal(t, http.StatusConflict, rec.Code, "body: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), "чёрном списке")
	})

	t.Run("человека не из ЧС подать можно", func(t *testing.T) {
		rec := submitPersonApp(t, e, db, token, orgName, "clean", "Sidorov", "Sidr", "Sidorovich", citizenshipID, tableID)
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	})
}
