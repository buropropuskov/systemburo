package handlers_test

import (
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
