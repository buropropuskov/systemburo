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

// findHistoryEntry возвращает первую запись истории заявки с заданным action_type.
func findHistoryEntry(t *testing.T, hist []map[string]interface{}, actionType string) map[string]interface{} {
	t.Helper()
	for _, h := range hist {
		if h["action_type"] == actionType {
			return h
		}
	}
	t.Fatalf("в истории нет записи action_type=%q; есть: %v", actionType, historyActionTypes(hist))
	return nil
}

func historyActionTypes(hist []map[string]interface{}) []string {
	out := make([]string, 0, len(hist))
	for _, h := range hist {
		if at, ok := h["action_type"].(string); ok {
			out = append(out, at)
		}
	}
	return out
}

// TestApplications_HistoryGolden_ManualEntryRoundTrip - golden (#870, срез 1.14):
// ручная запись через POST /applications/history должна вернуться из GET history
// байт-в-байт: action_status, old/new/comment и metadata как JSON-объект (не строка),
// user_id = автор. Это самый строгий fidelity-чек двух гибридных полей заявки
// (action_status + metadata jsonb), которых нет у простых сущностей. Зелёный и до,
// и после cutover на audit_log.
func TestApplications_HistoryGolden_ManualEntryRoundTrip(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "goldman1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)
	userID := getUserID(t, db, "goldman1")

	body := fmt.Sprintf(`{
		"application_id": %d,
		"user_id": %d,
		"action_type": "comment",
		"action_status": "custom_status",
		"old_value": "before",
		"new_value": "after",
		"comment": "manual entry round-trip",
		"metadata": {"reason": "test", "n": 7}
	}`, appID, userID)
	rec := testutil.POST(t, e, "/applications/history", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)

	entry := findHistoryEntry(t, hist, "comment")
	assert.Equal(t, "custom_status", entry["action_status"])
	assert.Equal(t, "before", entry["old_value"])
	assert.Equal(t, "after", entry["new_value"])
	assert.Equal(t, "manual entry round-trip", entry["comment"])
	assert.Equal(t, float64(userID), entry["user_id"], "user_id = автор записи")

	meta, ok := entry["metadata"].(map[string]interface{})
	require.True(t, ok, "metadata должна вернуться JSON-объектом, а не строкой: %#v", entry["metadata"])
	assert.Equal(t, "test", meta["reason"])
	assert.Equal(t, float64(7), meta["n"])
}

// TestApplications_HistoryGolden_FlowShape - golden (#870, срез 1.14): проверяет форму
// записей реального флоу (create + assigned_responsible + approve) и квирк двух акторов:
// для assigned_responsible в user_id лежит НАЗНАЧЕННЫЙ (target), а не действующий
// пользователь. Плюс порядок - новые сверху (created_at DESC). Зелёный до и после cutover.
func TestApplications_HistoryGolden_FlowShape(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "gold_cars", "Gold Cars")

	senderToken := testutil.RegisterAndLogin(t, e, "goldsender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "goldsender")

	testutil.RegisterUser(t, e, "goldresp", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "goldresp")
	assignOrgUser(t, db, td.OrgID, respID, true)
	respToken, _ := testutil.LoginUser(t, e, "goldresp", "pass123")

	appID := submitCompleteApplication(t, e, senderToken, "Test Organization", uaID)
	require.NotZero(t, appID)

	approveBody := fmt.Sprintf(`{"user_id": %d, "status": "approved", "comment": "ok"}`, respID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), approveBody, testutil.AuthHeader(respToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(senderToken))
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.GreaterOrEqual(t, len(hist), 3)

	// create: автор = отправитель, new_value = номер заявки, metadata-объект со статусом.
	create := findHistoryEntry(t, hist, "create")
	assert.Equal(t, float64(senderID), create["user_id"], "create пишет отправитель")
	assert.NotEmpty(t, create["new_value"], "new_value = номер заявки")
	createMeta, ok := create["metadata"].(map[string]interface{})
	require.True(t, ok, "create.metadata должна быть объектом: %#v", create["metadata"])
	assert.Equal(t, "Согласование", createMeta["confirmation"])
	assert.Equal(t, "Непрочитано", createMeta["status"])

	// assigned_responsible: квирк - user_id = НАЗНАЧЕННЫЙ пользователь (не действующий).
	assigned := findHistoryEntry(t, hist, "assigned_responsible")
	assert.Equal(t, float64(respID), assigned["user_id"], "assigned_responsible.user_id = target")
	assignedMeta, ok := assigned["metadata"].(map[string]interface{})
	require.True(t, ok, "assigned_responsible.metadata должна быть объектом")
	assert.Contains(t, assignedMeta, "required_approval")
	assert.Contains(t, assignedMeta, "is_primary")

	// approve: действующий - согласующий, есть comment и metadata.
	approve := findHistoryEntry(t, hist, "approve")
	assert.Equal(t, float64(respID), approve["user_id"], "approve пишет согласующий")
	assert.Equal(t, "ok", approve["comment"])

	// Порядок: новые сверху (created_at не возрастает).
	var prev time.Time
	for i, h := range hist {
		ts, perr := time.Parse(time.RFC3339, h["created_at"].(string))
		require.NoError(t, perr)
		if i > 0 {
			assert.Falsef(t, ts.After(prev), "история должна быть по убыванию created_at (поз %d)", i)
		}
		prev = ts
	}
}

// TestApplications_History_UnionLegacyAndAudit - переходная модель #870 (срез 1.14):
// новые действия пишутся в audit_log[application], старые строки замороженной
// application_history по-прежнему видны через union, новые сверху. Гарантия
// "история та же" до финального backfill. Падает на коде до cutover (новая запись
// уходит в старую таблицу), зеленеет после переключения записи на recorder.
func TestApplications_History_UnionLegacyAndAudit(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "uniapp1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)
	userID := getUserID(t, db, "uniapp1")

	// Новое действие через API -> уходит в audit_log (cutover записи).
	body := fmt.Sprintf(`{
		"application_id": %d, "user_id": %d,
		"action_type": "comment", "comment": "new audit row",
		"metadata": {"src": "audit"}
	}`, appID, userID)
	rec := testutil.POST(t, e, "/applications/history", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Запись физически в audit_log, а не в старой application_history.
	var auditCount, legacyCount int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appID, "comment").
		Count(&auditCount).Error)
	assert.Equal(t, int64(1), auditCount, "новое действие должно попасть в audit_log")
	require.NoError(t, db.Table("application_history").
		Where("application_id = ? AND action_type = ?", appID, "comment").
		Count(&legacyCount).Error)
	assert.Equal(t, int64(0), legacyCount, "старая application_history больше не пишется")

	// Легаси-строка напрямую в старую таблицу (час назад, как до миграции).
	require.NoError(t, db.Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, old_value, new_value, comment, metadata, created_at)
		VALUES (?, ?, 'legacy_event', 'o', 'n', 'legacy comment', ?, ?)
	`, appID, userID, `{"src":"legacy"}`, time.Now().Add(-time.Hour)).Error)

	// Union отдаёт обе строки, новые сверху.
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2, "union должен отдать и audit_log, и legacy-строку: %v", historyActionTypes(hist))

	assert.Equal(t, "comment", hist[0]["action_type"], "новее сверху - audit_log")
	newMeta, ok := hist[0]["metadata"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "audit", newMeta["src"])

	assert.Equal(t, "legacy_event", hist[1]["action_type"], "legacy час назад - ниже")
	assert.Equal(t, "legacy comment", hist[1]["comment"])
	assert.Equal(t, "o", hist[1]["old_value"])
	assert.Equal(t, "n", hist[1]["new_value"])
	legacyMeta, ok := hist[1]["metadata"].(map[string]interface{})
	require.True(t, ok, "legacy.metadata должна вернуться объектом")
	assert.Equal(t, "legacy", legacyMeta["src"])

	// Кладём ещё одну легаси-строку с NULL user_id - reader (INNER JOIN users)
	// её НЕ показывает; фиксируем это поведение (две видимые строки, не три).
	require.NoError(t, db.Exec(`
		INSERT INTO application_history (application_id, user_id, action_type, created_at)
		VALUES (?, NULL, 'orphan_event', ?)
	`, appID, time.Now().Add(-2*time.Hour)).Error)
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Len(t, testutil.ParseSlice(t, rec), 2, "строка с NULL user_id не видна (INNER JOIN users)")
}

// TestApplications_History_TiebreakerByID - при РАВНОМ created_at порядок задаёт id DESC
// (новее = больший id сверху). Воспроизводит эффект старого ручного +1мс инкремента для
// нескольких записей одного действия, когда recorder проставил им один момент вставки.
func TestApplications_History_TiebreakerByID(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAndLogin(t, e, "tieapp1", "pass123", 1, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)
	userID := getUserID(t, db, "tieapp1")

	ts := time.Now().UTC()
	for _, act := range []string{"ev_first", "ev_second"} {
		require.NoError(t, db.Exec(`
			INSERT INTO audit_log (entity_type, entity_id, action, actor_user_id, details, created_at)
			VALUES (?, ?, ?, ?, '{}'::jsonb, ?)
		`, models.AuditEntityApplication, appID, act, userID, ts).Error)
	}

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/history", appID), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	hist := testutil.ParseSlice(t, rec)
	require.Len(t, hist, 2)
	assert.Equal(t, "ev_second", hist[0]["action_type"], "при равном created_at новее (больший id) сверху")
	assert.Equal(t, "ev_first", hist[1]["action_type"])
}
