package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMarks_History_UnionLegacyAndAudit проверяет переходную модель #870:
// новые действия пишутся в audit_log, а строки из замороженной mark_histories
// по-прежнему видны в истории через union. Гарантирует "история та же" до backfill.
func TestMarks_History_UnionLegacyAndAudit(t *testing.T) {
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

	// Подтверждаем, что новая запись физически в audit_log (а не в старой таблице).
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").Where("entity_type = ? AND entity_id = ?", models.AuditEntityMark, id).Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "created должен попасть в audit_log")
	require.NoError(t, db.Table("mark_histories").Where("mark_id = ?", id).Count(&legacyCount).Error)
	assert.Equal(t, int64(0), legacyCount, "старая таблица больше не пишется")

	// Легаси-строка напрямую в старую таблицу с более ранним временем (как до миграции).
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

	// История endpoint-а объединяет обе таблицы, новые сверху.
	rec = testutil.GET(t, e, fmt.Sprintf("/marks/%d/history", id), h)
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "union должен отдать и audit_log, и legacy-строку")
	assert.Equal(t, "created", hist[0]["action_type"], "новее сверху - created из audit_log")
	assert.Equal(t, "renamed", hist[1]["action_type"], "legacy renamed час назад - ниже")
	// audit_log-строка несёт new_value в details; legacy-строка несёт поле напрямую.
	assert.Equal(t, "Тест-Аудит-Марка", hist[0]["new_value"], "audit_log->>'new_value' должен прийти в ответе")
	assert.Equal(t, "Старое Имя", hist[1]["old_value"], "legacy old_value должен прийти в ответе")
}
