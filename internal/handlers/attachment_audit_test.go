package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAttachments_History_UnionLegacyAndAudit проверяет переходную модель #870:
// новые действия пишутся в audit_log, а строки из замороженной unique_attachment_histories
// по-прежнему видны в истории через union. Гарантирует "история та же" до backfill.
func TestAttachments_History_UnionLegacyAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Создание через API -> запись created уходит уже в audit_log (cutover записи).
	rec := testutil.POST(t, e, "/attachments", `{"attachment_type":"cars","name":"union-test-att","display_name":"Union Test","title":"T"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Подтверждаем, что новая запись физически в audit_log (а не в старой таблице).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").Where("entity_type = ? AND entity_id = ?", models.AuditEntityUniqueAttachment, id).Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "created должен попасть в audit_log")
	require.NoError(t, db.Table("unique_attachment_histories").Where("unique_attachment_id = ?", id).Count(&legacyCount).Error)
	assert.Equal(t, int64(0), legacyCount, "старая таблица больше не пишется")

	// Легаси-строка напрямую в старую таблицу с более ранним временем (как до миграции).
	legacy := models.UniqueAttachmentHistory{
		UniqueAttachmentID: id,
		ActionType:         models.UniqueAttachmentActionUpdated,
		Details:            json.RawMessage(`{"display_name":{"old":"Старое","new":"Union Test"}}`),
		CreatedAt:          time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// История endpoint-а объединяет обе таблицы, новые сверху.
	histRec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/history", id), h)
	require.Equal(t, http.StatusOK, histRec.Code)
	hist := testutil.ParseSlice(t, histRec)
	require.Len(t, hist, 2, "union должен отдать и audit_log, и legacy-строку")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "updated", hist[1]["action_type"], "legacy updated час назад - ниже")
}
