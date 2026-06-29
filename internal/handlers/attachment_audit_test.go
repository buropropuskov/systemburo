package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAttachments_History_BackfillLegacyIntoAudit проверяет финал #870 (срез F.2):
// читатель истории переведён на audit_log-only, а до-cutover строки замороженной
// unique_attachment_histories поднимаются в audit_log разовым BackfillAuditFromLegacy.
// Так гарантируется "история та же" уже без union: новые действия и перенесённые
// старые видны вместе, старая таблица остаётся бэкапом и больше не читается.
func TestAttachments_History_BackfillLegacyIntoAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Создание через API -> запись created уходит в audit_log (cutover записи).
	rec := testutil.POST(t, e, "/attachments", `{"attachment_type":"cars","name":"union-test-att","display_name":"Union Test","title":"T"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))

	// Легаси-строка напрямую в замороженную таблицу с более ранним временем - как
	// строка, накопленная до cutover и ещё не перенесённая в audit_log.
	legacy := models.UniqueAttachmentHistory{
		UniqueAttachmentID: id,
		ActionType:         models.UniqueAttachmentActionUpdated,
		Details:            json.RawMessage(`{"display_name":{"old":"Старое","new":"Union Test"}}`),
		CreatedAt:          time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// До backfill читатель видит только audit_log -> legacy-строка ещё невидима.
	histRec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/history", id), h)
	require.Equal(t, http.StatusOK, histRec.Code)
	require.Len(t, testutil.ParseSlice(t, histRec), 1, "до backfill видно только audit_log")

	// CleanDB-Seed уже выставил гард-флаг (backfill прогонялся на пустой таблице) -
	// снимаем, чтобы перенести только что вставленную legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityUniqueAttachment).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// Легаси-строка физически скопирована в audit_log, а старая таблица цела (бэкап).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityUniqueAttachment, id, models.UniqueAttachmentActionUpdated).
		Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "legacy updated перенесён в audit_log")
	require.NoError(t, db.Table("unique_attachment_histories").Where("unique_attachment_id = ?", id).Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "старая таблица не тронута backfill'ом - read-only бэкап")

	// История отдаёт обе записи из audit_log, новые сверху.
	histRec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/history", id), h)
	require.Equal(t, http.StatusOK, histRec.Code)
	hist := testutil.ParseSlice(t, histRec)
	require.Len(t, hist, 2, "после backfill видны и created, и перенесённая legacy-строка")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "updated", hist[1]["action_type"], "legacy updated час назад - ниже")
	// details перенесены verbatim (update-diff хранит old/new).
	updated := hist[1]["details"].(map[string]interface{})
	displayName := updated["display_name"].(map[string]interface{})
	assert.Equal(t, "Старое", displayName["old"])
	assert.Equal(t, "Union Test", displayName["new"])

	// Идемпотентность: повторный backfill не дублирует (гард-флаг снова стоит).
	require.NoError(t, database.BackfillAuditFromLegacy(db))
	histRec = testutil.GET(t, e, fmt.Sprintf("/attachments/%d/history", id), h)
	require.Equal(t, http.StatusOK, histRec.Code)
	assert.Len(t, testutil.ParseSlice(t, histRec), 2, "повторный backfill не создаёт дублей")
}
