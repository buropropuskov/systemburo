package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Человек, возразивший против обработки, в новых заявках не заводится (#2361).
// Иначе отметка о возражении не значила бы ничего: данные внесли бы снова следующей
// же заявкой, и аннулирование действующих пропусков оказалось бы бессмысленным.
func TestPDObjection_SubmitRejectsObjectedPerson(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	uaID := seedUniqueAttachment(t, db, "people", fmt.Sprintf("objection_tmpl_%s", t.Name()), "Люди")

	const passport = "4502 909090"
	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees",
		`{"last_name":"Возразивший","first_name":"Олег","position":"Слесарь","passport_series_number":"`+passport+`","pd_consent":true}`, h).Code)

	var id int
	require.NoError(t, db.Model(&models.UniqueEmployee{}).Where("last_name = ?", "Возразивший").
		Select("id").Row().Scan(&id))

	// До отметки заявка с этим человеком проходит: убеждаемся, что дальше отклонит
	// именно возражение, а не что-то другое в составе заявки.
	before := submitPeopleApplication(t, e, h, uaID, "Возразивший", passport)
	require.NotEqual(t, http.StatusConflict, before.Code,
		"без возражения заявка по этой причине не отклоняется: "+before.Body.String())

	require.Equal(t, http.StatusOK, testutil.POST(t, e, "/unique-employees/"+itoa(id)+"/objection",
		`{"source":"заявление в бюро"}`, h).Code)

	after := submitPeopleApplication(t, e, h, uaID, "Возразивший", passport)
	assert.Equal(t, http.StatusConflict, after.Code, "после возражения заявка отклоняется")
	assert.Contains(t, after.Body.String(), "возразил", "причина названа словами")

	// Однофамилец с другим документом проходит: ищем по документу, а не по имени,
	// иначе чужое возражение закрыло бы дорогу постороннему человеку.
	other := submitPeopleApplication(t, e, h, uaID, "Возразивший", "4502 111000")
	assert.NotEqual(t, http.StatusConflict, other.Code,
		"однофамилец с другим паспортом под чужое возражение не попадает: "+other.Body.String())
}

// submitPeopleApplication подаёт заявку с одним работником.
func submitPeopleApplication(t *testing.T, e *echo.Echo, h http.Header, uaID int, lastName, passport string) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{
		"message": "проверка возражения",
		"organization": "Test Organization",
		"responsible_person": "Ответственный",
		"contact_phone": "+79001234567",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "people",
			"attachment_name": "people_tmpl",
			"attachment_display_name": "Люди",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {
				"employees": [{"last_name": "%s", "first_name": "Олег", "position": "Слесарь",
					"passport_series_number": "%s", "citizenship_id": 1, "pd_consent": true}]
			}
		}]
	}`, uaID, lastName, passport)
	return testutil.POST(t, e, "/applications/submit-complete-application", body, h)
}
