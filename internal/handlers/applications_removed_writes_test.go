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

// TestApplications_ParticipantCannotForgeStatusOrHistory: у участника заявки нет пути
// выставить себе "Согласовано" или статус в обход голосования и дописать в журнал заявки
// произвольную запись. Прямые PUT /applications/:id и POST /applications/history сняты.
func TestApplications_ParticipantCannotForgeStatusOrHistory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "forger1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	var before models.Application
	require.NoError(t, db.First(&before, appID).Error)

	rec := testutil.PUT(t, e, fmt.Sprintf("/applications/%d", appID),
		`{"confirmation":"Согласовано","status":"В работе"}`, testutil.AuthHeader(token))
	assert.NotEqual(t, http.StatusOK, rec.Code, "PUT заявки: %s", rec.Body.String())

	rec = testutil.POST(t, e, "/applications/history",
		fmt.Sprintf(`{"application_id":%d,"action_type":"forged","comment":"подделка"}`, appID), testutil.AuthHeader(token))
	assert.NotEqual(t, http.StatusOK, rec.Code, "POST истории: %s", rec.Body.String())

	var after models.Application
	require.NoError(t, db.First(&after, appID).Error)
	assert.Equal(t, before.Confirmation, after.Confirmation)
	assert.Equal(t, before.Status, after.Status)

	var forged int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, "forged").
		Count(&forged).Error)
	assert.Zero(t, forged, "в журнале заявки не должно быть подделанной записи")
}
