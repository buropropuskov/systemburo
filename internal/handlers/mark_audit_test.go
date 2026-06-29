package handlers_test

import (
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

// TestMarks_History_BackfillLegacyIntoAudit проверяет финал #870 (срез F.3):
// читатель истории марки переведён на audit_log-only, а до-cutover строки замороженной
// mark_histories поднимаются в audit_log разовым BackfillAuditFromLegacy. Плоские
// old_value/new_value сворачиваются в details в той же форме, что пишет recorder.
func TestMarks_History_BackfillLegacyIntoAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Создание через API -> запись created уходит уже в audit_log (cutover записи).
	rec := testutil.POST(t, e, "/marks", `{"name":"Тест-Аудит-Марка"}`, h)
	require.Equal(t, http.StatusCreated, rec.Code)
	id := int(testutil.ParseMap(t, rec)["id"].(float64))
	// marks нет в CleanDB - чистим за собой, иначе повтор make test упрётся в 409 (активное имя занято).
	defer db.Where("id = ?", id).Delete(&models.Mark{})

	// Легаси-строка напрямую в замороженную таблицу с более ранним временем - как
	// строка, накопленная до cutover и ещё не перенесённая в audit_log.
	oldVal := "Старое Имя"
	newVal := "Тест-Аудит-Марка"
	legacy := models.MarkHistory{
		MarkID:     id,
		ActionType: models.MarkActionRenamed,
		OldValue:   &oldVal,
		NewValue:   &newVal,
		CreatedAt:  time.Now().Add(-time.Hour),
	}
	require.NoError(t, db.Create(&legacy).Error)

	// До backfill читатель видит только audit_log -> legacy-строка ещё невидима.
	rec = testutil.GET(t, e, fmt.Sprintf("/marks/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, testutil.ParseSlice(t, rec), 1, "до backfill видно только audit_log")

	// CleanDB-Seed уже выставил гард-флаг (backfill прогонялся на пустой таблице) -
	// снимаем, чтобы перенести только что вставленную legacy-строку.
	require.NoError(t, db.Where("key = ?", "audit_backfilled:"+models.AuditEntityMark).
		Delete(&models.SystemSetting{}).Error)
	require.NoError(t, database.BackfillAuditFromLegacy(db))

	// Легаси-строка физически скопирована в audit_log, а старая таблица цела (бэкап).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityMark, id, models.MarkActionRenamed).
		Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "legacy renamed перенесён в audit_log")
	require.NoError(t, db.Table("mark_histories").Where("mark_id = ?", id).Count(&legacyCount).Error)
	assert.Equal(t, int64(1), legacyCount, "старая таблица не тронута backfill'ом - read-only бэкап")

	// История отдаёт обе записи из audit_log, новые сверху; old/new из свёрнутого details.
	rec = testutil.GET(t, e, fmt.Sprintf("/marks/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "после backfill видны и created, и перенесённая legacy-строка")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "renamed", hist[1]["action_type"], "legacy renamed час назад - ниже")
	assert.Equal(t, "Тест-Аудит-Марка", hist[0]["new_value"], "created несёт new_value в details")
	assert.Equal(t, "Старое Имя", hist[1]["old_value"], "перенесённый old_value из details")
	assert.Equal(t, "Тест-Аудит-Марка", hist[1]["new_value"], "перенесённый new_value из details")

	// Идемпотентность: повторный backfill не дублирует (гард-флаг снова стоит).
	require.NoError(t, database.BackfillAuditFromLegacy(db))
	rec = testutil.GET(t, e, fmt.Sprintf("/marks/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, testutil.ParseSlice(t, rec), 2, "повторный backfill не создаёт дублей")
}
