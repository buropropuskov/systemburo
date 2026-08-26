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

// TestPersonBlacklist_WriteFlip_UpdateRestoreToAuditLog: update/archive/restore через API
// пишут соответствующие действия в audit_log (#870): 'updated', 'archived', 'restored'.
func TestPersonBlacklist_WriteFlip_UpdateRestoreToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	rec := testutil.POST(t, e, "/person-blacklist", `{"last_name":"Правков","first_name":"Правк","reason":"update-test"}`, h)
	require.Equal(t, http.StatusCreated, rec.Code, "create: %s", rec.Body.String())
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// PUT update - меняем только reason, identity не меняем.
	rec = testutil.PUT(t, e, fmt.Sprintf("/person-blacklist/%d", id), `{"last_name":"Правков","first_name":"Правк","reason":"новая причина"}`, h)
	require.Equal(t, http.StatusOK, rec.Code, "update: %s", rec.Body.String())

	var updCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityPersonBlacklist, id, models.BlacklistActionUpdated).
		Count(&updCount).Error)
	assert.Equal(t, int64(1), updCount, "update должен попасть в audit_log")

	// DELETE (archive) -> 'archived'.
	rec = testutil.DELETE(t, e, fmt.Sprintf("/person-blacklist/%d", id), h)
	require.Equal(t, http.StatusOK, rec.Code, "archive: %s", rec.Body.String())

	var archCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityPersonBlacklist, id, models.BlacklistActionArchived).
		Count(&archCount).Error)
	assert.Equal(t, int64(1), archCount, "archive должен попасть в audit_log")

	// POST restore -> 'restored'.
	rec = testutil.POST(t, e, fmt.Sprintf("/person-blacklist/%d/restore", id), "", h)
	require.Equal(t, http.StatusOK, rec.Code, "restore: %s", rec.Body.String())

	var restCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityPersonBlacklist, id, models.BlacklistActionRestored).
		Count(&restCount).Error)
	assert.Equal(t, int64(1), restCount, "restore должен попасть в audit_log")
}
