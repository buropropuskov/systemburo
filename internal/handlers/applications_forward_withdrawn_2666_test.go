package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type forward2666Fixture struct {
	e        *echo.Echo
	db       *gorm.DB
	appID    int
	targetID int
	sender   string
	actors   map[string]string
}

func forward2666Setup(t *testing.T, withdrawn bool) forward2666Fixture {
	t.Helper()
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	f := forward2666Fixture{e: e, db: db, actors: map[string]string{}}
	f.sender = testutil.RegisterAndLogin(t, e, "forward2666_sender", "pass123", 1, td.OrgID, td.CompanyID)
	f.actors["sender"] = f.sender
	for _, role := range []string{"responsible", "viewer", "approver", "stranger"} {
		f.actors[role] = testutil.RegisterAndLogin(t, e, "forward2666_"+role, "pass123", 1, td.OrgID, td.CompanyID)
	}
	f.actors["super"] = testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "forward2666_approver")
	testutil.RegisterUser(t, e, "forward2666_target", "pass123", 1, td.OrgID, td.CompanyID)
	f.targetID = getUserID(t, db, "forward2666_target")
	uniqueID := seedUniqueAttachment(t, db, "cars", "forward2666_material", "Test material")
	f.appID = submitCompleteApplication(t, e, f.sender, "Test Organization", uniqueID)
	for _, role := range []string{"responsible", "viewer"} {
		id := getUserID(t, db, "forward2666_"+role)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", f.appID), forwardBody(id, false, role == "viewer"), testutil.AuthHeader(f.sender))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	if withdrawn {
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/withdraw", f.appID), "", testutil.AuthHeader(f.sender))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	return f
}

// Snapshot values, not only counts: a denied mixed request must not change rows.
func forward2666Snapshot(t *testing.T, f forward2666Fixture) map[string]string {
	t.Helper()
	result := map[string]string{}
	queries := map[string]string{
		"application":       "SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]'::jsonb)::text FROM applications x WHERE id = ?",
		"responsibles":      "SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]'::jsonb)::text FROM application_responsible_users x WHERE application_id = ?",
		"viewers":           "SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]'::jsonb)::text FROM application_viewers x WHERE application_id = ?",
		"forward_materials": "SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]'::jsonb)::text FROM forward_attachments x WHERE application_id = ?",
		"attachments":       "SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]'::jsonb)::text FROM attachments x WHERE application_id = ?",
		"cars":              "SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]'::jsonb)::text FROM cars x WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)",
		"employees":         "SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]'::jsonb)::text FROM employees x WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)",
		"audit":             "SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text), '[]'::jsonb)::text FROM audit_log x WHERE entity_type = 'application' AND entity_id = ?",
	}
	for name, query := range queries {
		var value string
		require.NoError(t, f.db.Raw(query, f.appID).Scan(&value).Error)
		result[name] = value
	}
	return result
}

func TestForwardWithdrawn2666_RolesCanForwardOnlyForView(t *testing.T) {
	for _, role := range []string{"sender", "responsible", "viewer", "approver", "super"} {
		t.Run(role, func(t *testing.T) {
			f := forward2666Setup(t, true)
			before := forward2666Snapshot(t, f)
			rec := testutil.POST(t, f.e, fmt.Sprintf("/applications/%d/forward", f.appID), forwardBody(f.targetID, false, true), testutil.AuthHeader(f.actors[role]))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			after := forward2666Snapshot(t, f)
			for _, name := range []string{"application", "responsibles", "attachments", "cars", "employees"} {
				assert.Equal(t, before[name], after[name], name+" must not be restored or reactivated")
			}
			assert.True(t, viewerIDsOf(t, f.e, f.sender, f.appID)[f.targetID])
			assert.False(t, responsibleIDsOf(t, f.e, f.sender, f.appID)[f.targetID])
			// Both history surfaces preserve safe historical purpose, not guessed names.
			for _, surface := range forward2664Surfaces(t, f.e, f.sender, f.appID) {
				details := forward2664Details(t, surface)
				require.Len(t, details, 1)
				assert.Equal(t, "view", details[f.targetID]["purpose"])
				assert.Equal(t, false, details[f.targetID]["required_approval"])
				assert.Equal(t, true, details[f.targetID]["access_granted"])
				assert.NotEmpty(t, details[f.targetID]["display_name"])
			}
		})
	}
}

func TestForwardWithdrawn2666_ApprovalAndMixedRequestsAreAtomicDenials(t *testing.T) {
	f := forward2666Setup(t, true)
	for _, role := range []string{"sender", "responsible", "viewer", "approver", "super"} {
		for _, flags := range []struct{ required, view bool }{{false, false}, {true, false}, {true, true}} {
			t.Run(fmt.Sprintf("%s_required_%t_view_%t", role, flags.required, flags.view), func(t *testing.T) {
				before := forward2666Snapshot(t, f)
				// A valid view first must not be committed before the invalid assignment.
				body := fmt.Sprintf(`{"users":[{"user_id":%d,"can_view":true},{"user_id":%d,"required_approval":%t,"can_view":%t}]}`, f.targetID, f.targetID, flags.required, flags.view)
				rec := testutil.POST(t, f.e, fmt.Sprintf("/applications/%d/forward", f.appID), body, testutil.AuthHeader(f.actors[role]))
				require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
				assert.Equal(t, before, forward2666Snapshot(t, f))
			})
		}
	}
}

func TestForwardWithdrawn2666_AccessBeforeStatus(t *testing.T) {
	f := forward2666Setup(t, true)
	before := forward2666Snapshot(t, f)
	rec := testutil.POST(t, f.e, fmt.Sprintf("/applications/%d/forward", f.appID), forwardBody(f.targetID, true, false), testutil.AuthHeader(f.actors["stranger"]))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), models.StatusWithdrawn)
	assert.Contains(t, rec.Body.String(), "permission to forward")
	assert.Equal(t, before, forward2666Snapshot(t, f))
}

func TestForwardWithdrawn2666_ActiveApprovalRemainsAvailable(t *testing.T) {
	f := forward2666Setup(t, false)
	rec := testutil.POST(t, f.e, fmt.Sprintf("/applications/%d/forward", f.appID), forwardBody(f.targetID, true, false), testutil.AuthHeader(f.sender))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, responsibleIDsOf(t, f.e, f.sender, f.appID)[f.targetID])
	for _, surface := range forward2664Surfaces(t, f.e, f.sender, f.appID) {
		details := forward2664Details(t, surface)
		require.Contains(t, details, f.targetID)
		assert.Equal(t, "approval", details[f.targetID]["purpose"])
		assert.Equal(t, true, details[f.targetID]["required_approval"])
	}
}

func TestForwardWithdrawn2666_RecipientCircleAndHistoryMasksRemainEnforced(t *testing.T) {
	f := forward2666Setup(t, true)
	foreignOrg, foreignCompany := seedOrgAndCompany(t, f.db, "Forward2666Foreign")
	testutil.RegisterUser(t, f.e, "forward2666_foreign", "pass123", 1, foreignOrg, foreignCompany)
	foreignID := getUserID(t, f.db, "forward2666_foreign")
	before := forward2666Snapshot(t, f)
	rec := testutil.POST(t, f.e, fmt.Sprintf("/applications/%d/forward", f.appID), forwardBody(foreignID, false, true), testutil.AuthHeader(f.sender))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, before, forward2666Snapshot(t, f), "ordinary sender cannot expand the permitted recipient circle")
	setUserName(t, f.db, "forward2666_foreign", "SyntheticHidden", "Recipient", "")
	enableConsent(t, f.e, f.actors["super"], "<p>Synthetic consent text</p>")
	rec = testutil.POST(t, f.e, fmt.Sprintf("/applications/%d/forward", f.appID), forwardBody(foreignID, false, true), testutil.AuthHeader(f.actors["super"]))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, viewerIDsOf(t, f.e, f.actors["super"], f.appID)[foreignID], "super administrator keeps the existing broad recipient circle")
	for _, surface := range forward2664Surfaces(t, f.e, f.actors["super"], f.appID) {
		details := forward2664Details(t, surface)
		require.Contains(t, details, foreignID)
		assert.Equal(t, "view", details[foreignID]["purpose"])
		assert.Equal(t, "@forward2666_foreign", details[foreignID]["display_name"])
		assert.NotContains(t, fmt.Sprint(surface["recipients"]), "SyntheticHidden")
	}
}

type forward2666LockContextKey struct{}

func TestForwardWithdrawn2666_ConcurrentStatusCommitIsReadUnderLock(t *testing.T) {
	f := forward2666Setup(t, false)
	before := forward2666Snapshot(t, f)
	// Reproduce the same row lock used by WithdrawApplication, without sleeps.
	withdrawTx := f.db.Begin()
	require.NoError(t, withdrawTx.Error)
	defer withdrawTx.Rollback()
	require.NoError(t, withdrawTx.Exec("SELECT id FROM applications WHERE id = ? FOR UPDATE", f.appID).Error)
	require.NoError(t, withdrawTx.Exec("UPDATE applications SET status = ? WHERE id = ?", models.StatusWithdrawn, f.appID).Error)
	atLock := make(chan struct{})
	var once sync.Once
	const callback = "forward2666_status_lock_test"
	require.NoError(t, f.db.Callback().Row().Before("gorm:row").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Context.Value(forward2666LockContextKey{}) == true && strings.Contains(tx.Statement.SQL.String(), "SELECT status, confirmation FROM applications") && strings.Contains(tx.Statement.SQL.String(), "FOR UPDATE") {
			once.Do(func() { close(atLock) })
		}
	}))
	defer func() { require.NoError(t, f.db.Callback().Row().Remove(callback)) }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/applications/%d/forward", f.appID), strings.NewReader(forwardBody(f.targetID, true, false)))
	request.Header = testutil.AuthHeader(f.sender)
	request.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	request = request.WithContext(context.WithValue(ctx, forward2666LockContextKey{}, true))
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { defer close(done); f.e.ServeHTTP(response, request) }()
	select {
	case <-atLock:
	case <-ctx.Done():
		// Release the DB lock and join before failing or cleaning the database.
		withdrawTx.Rollback()
		<-done
		t.Fatalf("forward request did not reach the transaction status lock: status=%d body=%s", response.Code, response.Body.String())
	}
	if err := withdrawTx.Commit().Error; err != nil {
		withdrawTx.Rollback()
		cancel()
		<-done
		require.NoError(t, err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		<-done
		t.Fatal("forward request did not complete after the competing status commit")
	}
	require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	after := forward2666Snapshot(t, f)
	for key, value := range before {
		if key != "application" {
			assert.Equal(t, value, after[key], key+" must remain unchanged by denied forwarding")
		}
	}
	var status string
	require.NoError(t, f.db.Raw("SELECT status FROM applications WHERE id = ?", f.appID).Scan(&status).Error)
	assert.Equal(t, models.StatusWithdrawn, status)
}
