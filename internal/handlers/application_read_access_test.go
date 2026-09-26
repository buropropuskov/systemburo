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

// TestGetApplicationByID_StrangerLeavesNoTrace: посторонний (коллега отправителя по
// организации, но не участник заявки) получает 403 и не оставляет следов: статус
// "Непрочитано" не переходит в "В обработке", прочтение не пишется в журнал, отметка
// просмотра статуса не появляется. Раньше доступ проверялся после сервиса, и все три
// записи успевали закоммититься.
func TestGetApplicationByID_StrangerLeavesNoTrace(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "readacc_sender", "pass123", 1, td.OrgID, td.CompanyID)
	strangerToken := testutil.RegisterAndLogin(t, e, "readacc_stranger", "pass123", 1, td.OrgID, td.CompanyID)
	strangerID := getUserID(t, db, "readacc_stranger")
	appID := createSimpleApplication(t, e, senderToken, td.OrgID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appID), testutil.AuthHeader(strangerToken))
	require.Equal(t, http.StatusForbidden, rec.Code, "посторонний не открывает заявку: %s", rec.Body.String())

	var status string
	var readingSet bool
	require.NoError(t, db.Raw(`SELECT status, reading_datetime IS NOT NULL FROM applications WHERE id = ?`, appID).
		Row().Scan(&status, &readingSet))
	assert.Equal(t, "Непрочитано", status, "чужое открытие не двигает статус")
	assert.False(t, readingSet, "чужое открытие не проставляет время прочтения")

	var reads int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, "read").
		Count(&reads).Error)
	assert.Zero(t, reads, "прочтение постороннего не попадает в журнал")

	var views int64
	require.NoError(t, db.Table("application_status_views").
		Where("application_id = ? AND user_id = ?", appID, strangerID).
		Count(&views).Error)
	assert.Zero(t, views, "отметка просмотра статуса постороннему не ставится")
}
