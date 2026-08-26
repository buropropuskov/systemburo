package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApprovalWorkflow(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "two_required_both_approve_confirmed",
			run: func(t *testing.T) {
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)

				senderToken := testutil.RegisterAndLogin(t, e, "ar_sender1", "pass123", 1, td.OrgID, td.CompanyID)
				appID := createSimpleApplication(t, e, senderToken, td.OrgID)

				// Two required responsible users
				testutil.RegisterUser(t, e, "ar_resp1a", "pass123", 1, td.OrgID, td.CompanyID)
				resp1ID := getUserID(t, db, "ar_resp1a")
				testutil.RegisterUser(t, e, "ar_resp1b", "pass123", 1, td.OrgID, td.CompanyID)
				resp2ID := getUserID(t, db, "ar_resp1b")

				// Forward with required_approval=true
				body := fmt.Sprintf(`{"users":[
					{"user_id":%d,"required_approval":true,"can_view":false},
					{"user_id":%d,"required_approval":true,"can_view":false}
				]}`, resp1ID, resp2ID)
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

				// After forward, confirmation should be "Согласование" (pending)
				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)
				status := testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
				require.NotNil(t, status.Confirmation)
				assert.Equal(t, "Согласование", *status.Confirmation)

				// First user approves
				resp1Token, _ := testutil.LoginUser(t, e, "ar_resp1a", "pass123")
				approveBody := fmt.Sprintf(`{"user_id":%d,"status":"approved","comment":"ok"}`, resp1ID)
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), approveBody, testutil.AuthHeader(resp1Token))
				require.Equal(t, http.StatusOK, rec.Code, "approve1: %s", rec.Body.String())

				// Still pending — second hasn't voted
				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)
				status = testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
				require.NotNil(t, status.Confirmation)
				assert.Equal(t, "Согласование", *status.Confirmation)

				// Second user approves
				resp2Token, _ := testutil.LoginUser(t, e, "ar_resp1b", "pass123")
				approveBody = fmt.Sprintf(`{"user_id":%d,"status":"approved","comment":"ok too"}`, resp2ID)
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), approveBody, testutil.AuthHeader(resp2Token))
				require.Equal(t, http.StatusOK, rec.Code, "approve2: %s", rec.Body.String())

				// Now confirmation = "Согласовано"
				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)
				status = testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
				require.NotNil(t, status.Confirmation)
				assert.Equal(t, "Согласовано", *status.Confirmation)
			},
		},
		{
			name: "two_required_one_rejects_not_confirmed",
			run: func(t *testing.T) {
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)

				senderToken := testutil.RegisterAndLogin(t, e, "ar_sender2", "pass123", 1, td.OrgID, td.CompanyID)
				appID := createSimpleApplication(t, e, senderToken, td.OrgID)

				testutil.RegisterUser(t, e, "ar_resp2a", "pass123", 1, td.OrgID, td.CompanyID)
				resp1ID := getUserID(t, db, "ar_resp2a")
				testutil.RegisterUser(t, e, "ar_resp2b", "pass123", 1, td.OrgID, td.CompanyID)
				resp2ID := getUserID(t, db, "ar_resp2b")

				body := fmt.Sprintf(`{"users":[
					{"user_id":%d,"required_approval":true,"can_view":false},
					{"user_id":%d,"required_approval":true,"can_view":false}
				]}`, resp1ID, resp2ID)
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)

				// First approves
				resp1Token, _ := testutil.LoginUser(t, e, "ar_resp2a", "pass123")
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID),
					fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, resp1ID), testutil.AuthHeader(resp1Token))
				require.Equal(t, http.StatusOK, rec.Code)

				// Second rejects
				resp2Token, _ := testutil.LoginUser(t, e, "ar_resp2b", "pass123")
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID),
					fmt.Sprintf(`{"user_id":%d,"status":"rejected","comment":"no"}`, resp2ID), testutil.AuthHeader(resp2Token))
				require.Equal(t, http.StatusOK, rec.Code)

				// Confirmation = "Не согласовано"
				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)
				status := testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
				require.NotNil(t, status.Confirmation)
				assert.Equal(t, "Не согласовано", *status.Confirmation)
			},
		},
		{
			name: "only_optional_one_approves_confirmed",
			run: func(t *testing.T) {
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)

				senderToken := testutil.RegisterAndLogin(t, e, "ar_sender3", "pass123", 1, td.OrgID, td.CompanyID)
				appID := createSimpleApplication(t, e, senderToken, td.OrgID)

				testutil.RegisterUser(t, e, "ar_resp3a", "pass123", 1, td.OrgID, td.CompanyID)
				respID := getUserID(t, db, "ar_resp3a")

				// Forward with required_approval=false (optional)
				body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":false,"can_view":false}]}`, respID)
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)

				// Optional user approves
				respToken, _ := testutil.LoginUser(t, e, "ar_resp3a", "pass123")
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID),
					fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, respID), testutil.AuthHeader(respToken))
				require.Equal(t, http.StatusOK, rec.Code)

				// Confirmation = "Согласовано"
				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)
				status := testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
				require.NotNil(t, status.Confirmation)
				assert.Equal(t, "Согласовано", *status.Confirmation)
			},
		},
		{
			name: "only_optional_one_rejects_not_confirmed",
			run: func(t *testing.T) {
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)

				senderToken := testutil.RegisterAndLogin(t, e, "ar_sender4", "pass123", 1, td.OrgID, td.CompanyID)
				appID := createSimpleApplication(t, e, senderToken, td.OrgID)

				testutil.RegisterUser(t, e, "ar_resp4a", "pass123", 1, td.OrgID, td.CompanyID)
				respID := getUserID(t, db, "ar_resp4a")

				body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":false,"can_view":false}]}`, respID)
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)

				// Optional user rejects
				respToken, _ := testutil.LoginUser(t, e, "ar_resp4a", "pass123")
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID),
					fmt.Sprintf(`{"user_id":%d,"status":"rejected","comment":"nope"}`, respID), testutil.AuthHeader(respToken))
				require.Equal(t, http.StatusOK, rec.Code)

				// Confirmation = "Не согласовано"
				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)
				status := testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
				require.NotNil(t, status.Confirmation)
				assert.Equal(t, "Не согласовано", *status.Confirmation)
			},
		},
		{
			name: "revoke_approval_returns_to_pending",
			run: func(t *testing.T) {
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)

				senderToken := testutil.RegisterAndLogin(t, e, "ar_sender5", "pass123", 1, td.OrgID, td.CompanyID)
				appID := createSimpleApplication(t, e, senderToken, td.OrgID)

				testutil.RegisterUser(t, e, "ar_resp5a", "pass123", 1, td.OrgID, td.CompanyID)
				respID := getUserID(t, db, "ar_resp5a")

				// Forward optional responsible
				body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":false,"can_view":false}]}`, respID)
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)

				// Approve
				respToken, _ := testutil.LoginUser(t, e, "ar_resp5a", "pass123")
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID),
					fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, respID), testutil.AuthHeader(respToken))
				require.Equal(t, http.StatusOK, rec.Code)

				// Verify approved
				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)
				status := testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
				require.NotNil(t, status.Confirmation)
				assert.Equal(t, "Согласовано", *status.Confirmation)

				// Revoke
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/revoke-approval", appID),
					`{"comment":"changed mind"}`, testutil.AuthHeader(respToken))
				require.Equal(t, http.StatusOK, rec.Code)

				// Confirmation back to "Согласование"
				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)
				status = testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
				require.NotNil(t, status.Confirmation)
				assert.Equal(t, "Согласование", *status.Confirmation)
			},
		},
		{
			name: "mix_required_and_optional_required_takes_priority",
			run: func(t *testing.T) {
				testutil.CleanDB(t, db)
				td := testutil.SeedTestData(t, db)

				senderToken := testutil.RegisterAndLogin(t, e, "ar_sender6", "pass123", 1, td.OrgID, td.CompanyID)
				appID := createSimpleApplication(t, e, senderToken, td.OrgID)

				testutil.RegisterUser(t, e, "ar_req6", "pass123", 1, td.OrgID, td.CompanyID)
				reqID := getUserID(t, db, "ar_req6")
				testutil.RegisterUser(t, e, "ar_opt6", "pass123", 1, td.OrgID, td.CompanyID)
				optID := getUserID(t, db, "ar_opt6")

				// Forward: one required, one optional
				body := fmt.Sprintf(`{"users":[
					{"user_id":%d,"required_approval":true,"can_view":false},
					{"user_id":%d,"required_approval":false,"can_view":false}
				]}`, reqID, optID)
				rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)

				// Optional approves — confirmation stays pending because required hasn't voted
				optToken, _ := testutil.LoginUser(t, e, "ar_opt6", "pass123")
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID),
					fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, optID), testutil.AuthHeader(optToken))
				require.Equal(t, http.StatusOK, rec.Code)

				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)
				status := testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
				require.NotNil(t, status.Confirmation)
				assert.Equal(t, "Согласование", *status.Confirmation, "should be pending while required hasn't voted")

				// Required approves — now confirmed
				reqToken, _ := testutil.LoginUser(t, e, "ar_req6", "pass123")
				rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID),
					fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, reqID), testutil.AuthHeader(reqToken))
				require.Equal(t, http.StatusOK, rec.Code)

				rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/check-approval-status", appID), testutil.AuthHeader(senderToken))
				require.Equal(t, http.StatusOK, rec.Code)
				status = testutil.ParseResponse[services.ApprovalStatusResponse](t, rec)
				require.NotNil(t, status.Confirmation)
				assert.Equal(t, "Согласовано", *status.Confirmation, "required approved, so confirmed regardless of optional")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

// TestForwardAttachments проверяет запись строк forward_attachments при пересылке (#680):
// общий список вложений разворачивается построчно на каждого получателя, чужие ID
// отбрасываются, пустой список не пишет строк (обратная совместимость).
func TestForwardAttachments(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// createAppWithTwoAttachments создаёт заявку с двумя cars-вложениями и возвращает их ID.
	createAppWithTwoAttachments := func(t *testing.T, token, prefix string) (appID, attID1, attID2 int) {
		t.Helper()
		uaID1 := seedUniqueAttachment(t, db, "cars", prefix+"_a", prefix+" A")
		uaID2 := seedUniqueAttachment(t, db, "cars", prefix+"_b", prefix+" B")
		body := fmt.Sprintf(`{
			"message": "forward attachments test",
			"organization": "Test Organization",
			"responsible_person": "Test Person",
			"contact_phone": "+79001234567",
			"data_approval": true,
			"attachments": [
				{"attachment_type":"cars","attachment_name":"cars_a","attachment_display_name":"Cars A",
				 "unique_attachment_id":%d,"entry_date_from":"2026-04-01","entry_date_to":"2099-12-31",
				 "entry_time_from":"08:00","entry_time_to":"18:00",
				 "data":{"vehicles":[{"car_number":"B001BB777","car_brand":"Honda"}]}},
				{"attachment_type":"cars","attachment_name":"cars_b","attachment_display_name":"Cars B",
				 "unique_attachment_id":%d,"entry_date_from":"2026-04-01","entry_date_to":"2099-12-31",
				 "entry_time_from":"08:00","entry_time_to":"18:00",
				 "data":{"vehicles":[{"car_number":"C001CC777","car_brand":"Mazda"}]}}
			]
		}`, uaID1, uaID2)
		rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "create: %s", rec.Body.String())
		resp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)
		appID = resp.ApplicationID

		rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)
		attachments := testutil.ParseSlice(t, rec)
		require.Len(t, attachments, 2)
		return appID, int(attachments[0]["id"].(float64)), int(attachments[1]["id"].(float64))
	}

	type faRow struct {
		RecipientUserID int `gorm:"column:recipient_user_id"`
		AttachmentID    int `gorm:"column:attachment_id"`
	}
	forwardRows := func(t *testing.T, appID int) []faRow {
		t.Helper()
		var rows []faRow
		err := db.Raw("SELECT recipient_user_id, attachment_id FROM forward_attachments WHERE application_id = ? ORDER BY recipient_user_id, attachment_id", appID).Scan(&rows).Error
		require.NoError(t, err)
		return rows
	}

	// forwardedMeta читает metadata сводной записи forwarded. #870 (срез 1.14): запись
	// ушла в audit_log[application], metadata лежит внутри details->'metadata'.
	forwardedMeta := func(t *testing.T, appID int) map[string]interface{} {
		t.Helper()
		var raw string
		err := db.Raw("SELECT (details->'metadata')::text FROM audit_log WHERE entity_type = 'application' AND entity_id = ? AND action = 'forwarded'", appID).Scan(&raw).Error
		require.NoError(t, err)
		require.NotEmpty(t, raw, "должна быть запись истории forwarded")
		var m map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(raw), &m))
		return m
	}

	t.Run("subset_written_per_recipient", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fa_sender1", "pass123", 1, td.OrgID, td.CompanyID)
		appID, attID1, _ := createAppWithTwoAttachments(t, senderToken, "fa_cars1")

		testutil.RegisterUser(t, e, "fa_resp1", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "fa_resp1")

		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}],"attachment_ids":[%d]}`, respID, attID1)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		rows := forwardRows(t, appID)
		require.Len(t, rows, 1, "ровно одна строка на (получатель, выбранное вложение)")
		assert.Equal(t, respID, rows[0].RecipientUserID)
		assert.Equal(t, attID1, rows[0].AttachmentID)
	})

	t.Run("empty_list_writes_nothing", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fa_sender2", "pass123", 1, td.OrgID, td.CompanyID)
		appID, _, _ := createAppWithTwoAttachments(t, senderToken, "fa_cars2")

		testutil.RegisterUser(t, e, "fa_resp2", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "fa_resp2")

		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, respID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		assert.Empty(t, forwardRows(t, appID), "без attachment_ids строк быть не должно (видит все)")
	})

	t.Run("all_foreign_ids_writes_nothing", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fa_sender4", "pass123", 1, td.OrgID, td.CompanyID)
		appID, _, _ := createAppWithTwoAttachments(t, senderToken, "fa_cars4")

		testutil.RegisterUser(t, e, "fa_resp4", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "fa_resp4")

		// Все ID чужие (не из этой заявки) - все отброшены, строк нет. Важно для чтения:
		// получатель остаётся в режиме "видит все" (нет строк), а не "видит ничего".
		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}],"attachment_ids":[999998,999999]}`, respID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		assert.Empty(t, forwardRows(t, appID), "чужие attachment_ids отброшены -> строк нет")
	})

	t.Run("fanout_to_recipients_filters_foreign_ids", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fa_sender3", "pass123", 1, td.OrgID, td.CompanyID)
		appID, attID1, attID2 := createAppWithTwoAttachments(t, senderToken, "fa_cars3")

		testutil.RegisterUser(t, e, "fa_resp3", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "fa_resp3")
		testutil.RegisterUser(t, e, "fa_view3", "pass123", 1, td.OrgID, td.CompanyID)
		viewerID := getUserID(t, db, "fa_view3")

		// Чужой attachment_id 999999 должен быть отброшен фильтром по application_id.
		body := fmt.Sprintf(`{"users":[
			{"user_id":%d,"required_approval":true,"can_view":false},
			{"user_id":%d,"required_approval":false,"can_view":true}
		],"attachment_ids":[%d,%d,999999]}`, respID, viewerID, attID1, attID2)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		rows := forwardRows(t, appID)
		require.Len(t, rows, 4, "2 получателя x 2 валидных вложения, чужой ID отброшен")
		got := map[int][]int{}
		for _, r := range rows {
			got[r.RecipientUserID] = append(got[r.RecipientUserID], r.AttachmentID)
		}
		assert.ElementsMatch(t, []int{attID1, attID2}, got[respID])
		assert.ElementsMatch(t, []int{attID1, attID2}, got[viewerID])
	})

	t.Run("history_whole_application", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fa_sender5", "pass123", 1, td.OrgID, td.CompanyID)
		appID, _, _ := createAppWithTwoAttachments(t, senderToken, "fa_cars5")

		testutil.RegisterUser(t, e, "fa_resp5", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "fa_resp5")

		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, respID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		m := forwardedMeta(t, appID)
		assert.Equal(t, true, m["whole"], "пустой attachment_ids -> переслана вся заявка")
		if atts, ok := m["attachments"].([]interface{}); ok {
			assert.Empty(t, atts, "у целой заявки список вложений пуст")
		}
	})

	t.Run("history_specific_attachments", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fa_sender6", "pass123", 1, td.OrgID, td.CompanyID)
		appID, attID1, _ := createAppWithTwoAttachments(t, senderToken, "fa_cars6")

		testutil.RegisterUser(t, e, "fa_resp6", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "fa_resp6")

		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}],"attachment_ids":[%d]}`, respID, attID1)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		m := forwardedMeta(t, appID)
		assert.Equal(t, false, m["whole"], "выбран subset -> не вся заявка")
		atts, ok := m["attachments"].([]interface{})
		require.True(t, ok, "attachments должен быть списком имён")
		require.Len(t, atts, 1)
		assert.Equal(t, "Cars A", atts[0], "имя выбранного вложения попадает в историю")
	})
}
