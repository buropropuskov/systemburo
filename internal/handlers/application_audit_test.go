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

// TestApplications_WriteFlip_ReadAndWorkflowToAuditLog (#870): проверяет, что действия
// workflow-цикла принимающего (read, take_to_work, reject, revoke_from_work, restore_to_work)
// пишутся в audit_log[application]. Используются два приложения:
// appAccept — флоу accept -> revoke; appReject — флоу reject -> restore.
func TestApplications_WriteFlip_ReadAndWorkflowToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Принимающий (approver) - тип 6, запись в application_approvers.
	testutil.RegisterUser(t, e, "wf1appr", "pass123", 6, td.OrgID, td.CompanyID)
	approverID := getUserID(t, db, "wf1appr")
	db.Exec("INSERT INTO application_approvers (user_id, created_at) VALUES (?, NOW()) ON CONFLICT DO NOTHING", approverID)
	approverToken, _ := testutil.LoginUser(t, e, "wf1appr", "pass123")

	senderToken := testutil.RegisterAndLogin(t, e, "wf1sndr", "pass123", 1, td.OrgID, td.CompanyID)

	// --- appAccept: read -> take_to_work(accept) -> revoke_from_work ---

	appAccept := createSimpleApplication(t, e, senderToken, td.OrgID)

	// read: принимающий открывает заявку со статусом "Непрочитано" -> action='read'.
	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d", appAccept), testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "GET app для read: %s", rec.Body.String())

	var n int64
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appAccept, "read").
		Count(&n).Error)
	assert.GreaterOrEqual(t, n, int64(1), "action='read' должна появиться в audit_log после открытия принимающим")

	// take_to_work(accept): принимающий берёт заявку в работу.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appAccept),
		fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, approverID),
		testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "take-to-work accept: %s", rec.Body.String())

	n = 0
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appAccept, "take_to_work").
		Count(&n).Error)
	assert.Equal(t, int64(1), n, "action='take_to_work' должна появиться в audit_log")

	// revoke_from_work: принимающий отзывает заявку из работы.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-from-work", appAccept),
		fmt.Sprintf(`{"user_id":%d,"comment":"нужны правки"}`, approverID),
		testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "revoke-from-work: %s", rec.Body.String())

	n = 0
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appAccept, "revoke_from_work").
		Count(&n).Error)
	assert.Equal(t, int64(1), n, "action='revoke_from_work' должна появиться в audit_log")

	// --- appReject: take_to_work(reject) -> restore_to_work ---

	appReject := createSimpleApplication(t, e, senderToken, td.OrgID)

	// take_to_work(reject): принимающий отказывает в принятии заявки.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appReject),
		fmt.Sprintf(`{"user_id":%d,"action":"reject","comment":"не подходит"}`, approverID),
		testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "take-to-work reject: %s", rec.Body.String())

	n = 0
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appReject, "reject").
		Count(&n).Error)
	assert.Equal(t, int64(1), n, "action='reject' (take-to-work путь) должна появиться в audit_log")

	// restore_to_work: принимающий возвращает отклонённую заявку в обработку.
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/restore-to-work", appReject),
		fmt.Sprintf(`{"user_id":%d,"comment":"пересмотрим"}`, approverID),
		testutil.AuthHeader(approverToken))
	require.Equal(t, http.StatusOK, rec.Code, "restore-to-work: %s", rec.Body.String())

	n = 0
	require.NoError(t, db.Table("audit_log").
		Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appReject, "restore_to_work").
		Count(&n).Error)
	assert.Equal(t, int64(1), n, "action='restore_to_work' должна появиться в audit_log")
}

// TestApplications_WriteFlip_ForwardAndApprovalToAuditLog (#870): проверяет, что действия
// пересылки и согласования пишутся в audit_log[application]:
//   - assigned_responsible, assigned_viewer, forwarded — при forward;
//   - approve, confirmation_change — при approve(approved);
//   - revoke_approval, confirmation_change — при revoke_approval;
//   - reject (approve-путь), confirmation_change — при approve(rejected) на отдельной заявке.
func TestApplications_WriteFlip_ForwardAndApprovalToAuditLog(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "wf2sndr", "pass123", 1, td.OrgID, td.CompanyID)

	// wf2resp — получает пересылку как ответственный (required_approval=true) и затем согласует.
	testutil.RegisterUser(t, e, "wf2resp", "pass123", 1, td.OrgID, td.CompanyID)
	respID := getUserID(t, db, "wf2resp")
	respToken, _ := testutil.LoginUser(t, e, "wf2resp", "pass123")

	// wf2view — получает пересылку как просматривающий (can_view=true).
	testutil.RegisterUser(t, e, "wf2view", "pass123", 1, td.OrgID, td.CompanyID)
	viewerID := getUserID(t, db, "wf2view")

	// --- Sub-test A: forward(resp+viewer) -> approve(approved) -> revoke_approval ---

	t.Run("ForwardApproveRevoke", func(t *testing.T) {
		appFwd := createSimpleApplication(t, e, senderToken, td.OrgID)

		// Пересылка: sender -> wf2resp(required_approval) + wf2view(viewer).
		fwdBody := fmt.Sprintf(`{"users":[
			{"user_id":%d,"required_approval":true,"can_view":false},
			{"user_id":%d,"required_approval":false,"can_view":true}
		]}`, respID, viewerID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appFwd), fwdBody, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		var n int64
		// assigned_responsible: одна запись на wf2resp.
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appFwd, "assigned_responsible").
			Count(&n).Error)
		assert.Equal(t, int64(1), n, "assigned_responsible должен быть записан после forward")

		// assigned_viewer: одна запись на wf2view.
		n = 0
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appFwd, "assigned_viewer").
			Count(&n).Error)
		assert.Equal(t, int64(1), n, "assigned_viewer должен быть записан после forward")

		// forwarded: одна сводная запись.
		n = 0
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appFwd, "forwarded").
			Count(&n).Error)
		assert.Equal(t, int64(1), n, "forwarded должен быть записан после forward")

		// approve(approved): wf2resp голосует за.
		approveBody := fmt.Sprintf(`{"user_id":%d,"status":"approved","comment":"ок"}`, respID)
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appFwd), approveBody, testutil.AuthHeader(respToken))
		require.Equal(t, http.StatusOK, rec.Code, "approve(approved): %s", rec.Body.String())

		n = 0
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appFwd, "approve").
			Count(&n).Error)
		assert.Equal(t, int64(1), n, "action='approve' должен быть записан после approve(approved)")

		// confirmation_change: после approve(approved) confirmation меняется "Согласование" -> "Согласовано".
		n = 0
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appFwd, "confirmation_change").
			Count(&n).Error)
		assert.GreaterOrEqual(t, n, int64(1), "confirmation_change должен быть записан после approve(approved)")
		afterApproveConfChange := n

		// revoke_approval: wf2resp отзывает согласование.
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-approval", appFwd),
			`{"comment":"передумал"}`,
			testutil.AuthHeader(respToken))
		require.Equal(t, http.StatusOK, rec.Code, "revoke-approval: %s", rec.Body.String())

		n = 0
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appFwd, "revoke_approval").
			Count(&n).Error)
		assert.Equal(t, int64(1), n, "action='revoke_approval' должен быть записан после revoke-approval")

		// confirmation_change должен добавить ещё одну запись (возврат к "Согласование").
		n = 0
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appFwd, "confirmation_change").
			Count(&n).Error)
		assert.Greater(t, n, afterApproveConfChange, "revoke_approval должен добавить ещё один confirmation_change")
	})

	// --- Sub-test B: forward(required) -> approve(rejected) ---

	t.Run("ForwardApproveReject", func(t *testing.T) {
		// wf2rej — ответственный для ветки отклонения.
		testutil.RegisterUser(t, e, "wf2rej", "pass123", 1, td.OrgID, td.CompanyID)
		rejID := getUserID(t, db, "wf2rej")
		rejToken, _ := testutil.LoginUser(t, e, "wf2rej", "pass123")

		appRej := createSimpleApplication(t, e, senderToken, td.OrgID)

		// Пересылка: sender -> wf2rej(required_approval).
		fwdBody := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, rejID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appRej), fwdBody, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward для reject-ветки: %s", rec.Body.String())

		// approve(rejected): wf2rej отклоняет заявку.
		rejectBody := fmt.Sprintf(`{"user_id":%d,"status":"rejected","comment":"не соответствует требованиям"}`, rejID)
		rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appRej), rejectBody, testutil.AuthHeader(rejToken))
		require.Equal(t, http.StatusOK, rec.Code, "approve(rejected): %s", rec.Body.String())

		var n int64
		// action='reject' (approve-путь).
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appRej, "reject").
			Count(&n).Error)
		assert.Equal(t, int64(1), n, "action='reject' (approve-путь) должен быть записан после approve(rejected)")

		// confirmation_change: "Согласование" -> "Не согласовано".
		n = 0
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND entity_id = ? AND action = ?", models.AuditEntityApplication, appRej, "confirmation_change").
			Count(&n).Error)
		assert.GreaterOrEqual(t, n, int64(1), "confirmation_change должен быть записан после approve(rejected)")
	})
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
