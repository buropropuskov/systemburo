package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForwardApplication_Archived_Allowed закрывает #869: архивную (давно
// завершённую) заявку можно переслать - пересылка это маршрутизация, а не смена
// статуса. До правки checkNotArchived в ForwardApplication отдавал 403
// "Архивная заявка доступна только для чтения".
func TestForwardApplication_Archived_Allowed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	uaID := seedUniqueAttachment(t, db, "cars", "cars_fwd_arch", "Cars")
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

	// Делаем заявку архивной: статус Завершено + окно въезда закрылось >1 мес назад.
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).Update("status", models.StatusCompleted).Error)
	require.NoError(t, db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01").Error)

	// Получатель пересылки.
	testutil.RegisterUser(t, e, "fwd_arch_resp", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "fwd_arch_resp")

	// Пересылаем архивную заявку ответственному с обязательным согласованием.
	body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, respID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "пересылка архивной заявки должна проходить (#869): %s", rec.Body.String())

	// Пересылка реально исполнилась: получатель назначен ответственным. Confirmation
	// завершённой заявки пересылка не пересчитывает - итог согласования зафиксирован.
	var assigned int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM application_responsible_users
		WHERE application_id = ? AND user_id = ?`, appID, respID).Scan(&assigned).Error)
	assert.EqualValues(t, 1, assigned)
}
