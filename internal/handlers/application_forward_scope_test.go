package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestForwardScope_TransitiveSelection(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	sender := testutil.RegisterAndLogin(t, e, "scope_sender", "pass123", 1, td.OrgID, td.CompanyID)
	limited := testutil.RegisterAndLogin(t, e, "scope_limited", "pass123", 1, td.OrgID, td.CompanyID)
	target := testutil.RegisterAndLogin(t, e, "scope_target", "pass123", 1, td.OrgID, td.CompanyID)
	receiver := testutil.RegisterAndLogin(t, e, "scope_receiver", "pass123", 1, td.OrgID, td.CompanyID)
	makeApprover(t, db, "scope_receiver")
	super := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	senderID, limitedID := getUserID(t, db, "scope_sender"), getUserID(t, db, "scope_limited")
	targetID, receiverID := getUserID(t, db, "scope_target"), getUserID(t, db, "scope_receiver")
	point := time.Now().UTC()
	until := point.AddDate(1, 0, 0).Format("2006-01-02")
	foreignApp := suppApp(t, db, td.OrgID, senderID, "SCOPE-FOREIGN", models.ConfirmationPending, models.StatusProcessing)
	foreign := suppAttachment(t, db, foreignApp, "items", until)
	var invalidMessage interface{}
	for index, tc := range []struct {
		name, actor, selection, prior  string
		responsible, repeated, invalid bool
		wantAll                        bool
	}{
		{name: "implicit_subset", actor: "limited"},
		{name: "explicit_subset", actor: "limited", selection: "a"},
		{name: "responsible_subset", actor: "limited", responsible: true},
		{name: "duplicate_ids", actor: "limited", selection: "duplicate"},
		{name: "duplicate_new_recipient", actor: "limited", repeated: true},
		{name: "hidden_selection", actor: "limited", selection: "b", invalid: true},
		{name: "mixed_hidden", actor: "limited", selection: "mixed_hidden", invalid: true},
		{name: "foreign_selection", actor: "limited", selection: "foreign", invalid: true},
		{name: "absent_selection", actor: "limited", selection: "absent", invalid: true},
		{name: "negative_selection", actor: "limited", selection: "negative", invalid: true},
		{name: "empty_effective_subset", actor: "limited", selection: "empty_scope", invalid: true},
		{name: "full_actor_mixed_foreign", actor: "sender", selection: "mixed_foreign", invalid: true},
		{name: "full_actor_invalid", actor: "sender", selection: "absent", invalid: true},
		{name: "full_sender_all", actor: "sender", wantAll: true},
		{name: "super_keeps_all", actor: "super", wantAll: true},
		{name: "full_viewer_not_narrowed", actor: "limited", prior: "viewer", wantAll: true},
		{name: "full_responsible_not_narrowed", actor: "limited", prior: "responsible", wantAll: true},
		{name: "subset_union", actor: "limited", prior: "subset", wantAll: true},
		{name: "receiver_not_narrowed", actor: "limited", prior: "receiver", wantAll: true},
		{name: "sender_not_narrowed", actor: "limited", prior: "sender", wantAll: true},
		{name: "super_recipient_not_narrowed", actor: "limited", prior: "super", wantAll: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := suppApp(t, db, td.OrgID, senderID, fmt.Sprintf("SCOPE-%02d", index), models.ConfirmationPending, models.StatusProcessing)
			a, b := suppAttachment(t, db, app, "items", until), suppAttachment(t, db, app, "items", until)
			if tc.responsible {
				suppResponsible(t, db, app, limitedID, false, "pending")
			} else {
				require.NoError(t, db.Create(&models.ApplicationViewer{ApplicationID: app, UserID: limitedID, CreatedAt: point}).Error)
			}
			allowedID := a
			if tc.selection == "empty_scope" {
				allowedID = foreign
			}
			require.NoError(t, db.Create(&models.ForwardAttachment{ApplicationID: app, RecipientUserID: limitedID, AttachmentID: allowedID, CreatedAt: point}).Error)
			id, targetToken := targetID, target
			switch tc.prior {
			case "viewer", "subset":
				require.NoError(t, db.Create(&models.ApplicationViewer{ApplicationID: app, UserID: id, CreatedAt: point}).Error)
				if tc.prior == "subset" {
					require.NoError(t, db.Create(&models.ForwardAttachment{ApplicationID: app, RecipientUserID: id, AttachmentID: b, CreatedAt: point}).Error)
				}
			case "responsible":
				suppResponsible(t, db, app, id, false, "pending")
			case "receiver":
				id, targetToken = receiverID, receiver
			case "sender":
				id, targetToken = senderID, sender
			case "super":
				id, targetToken = getUserID(t, db, "testadmin"), super
			}
			actor := map[string]string{"sender": sender, "limited": limited, "super": super}[tc.actor]
			var selected []int
			switch tc.selection {
			case "a":
				selected = []int{a}
			case "b":
				selected = []int{b}
			case "duplicate":
				selected = []int{a, a}
			case "mixed_hidden":
				selected = []int{a, b}
			case "foreign":
				selected = []int{foreign}
			case "absent":
				var count int64
				require.NoError(t, db.Table("attachments").Where("id = ?", 999998).Count(&count).Error)
				require.Zero(t, count)
				selected = []int{999998}
			case "negative":
				selected = []int{-1}
			case "mixed_foreign":
				selected = []int{a, foreign}
			}
			users := []map[string]interface{}{{"user_id": id, "can_view": true, "required_approval": false}}
			if tc.repeated {
				users = append(users, users[0])
			}
			payload, err := json.Marshal(map[string]interface{}{"users": users, "attachment_ids": selected})
			require.NoError(t, err)
			before := remainingTransactionSnapshot(t, db)
			rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", app), string(payload), testutil.AuthHeader(actor))
			if tc.invalid {
				require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
				var body map[string]interface{}
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				require.NotEmpty(t, body["error"])
				if invalidMessage == nil {
					invalidMessage = body["error"]
				}
				require.Equal(t, invalidMessage, body["error"], "hidden, absent and foreign IDs do not disclose different existence states")
				require.Equal(t, before, remainingTransactionSnapshot(t, db), "rejected scope leaves no grants, assignments, history or notifications")
				return
			}
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			read := testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", app), testutil.AuthHeader(targetToken))
			require.Equal(t, http.StatusOK, read.Code, read.Body.String())
			visible := testutil.ParseSlice(t, read)
			want := []int{a}
			if tc.wantAll {
				want = append(want, b)
			}
			got := make([]int, 0, len(visible))
			for _, row := range visible {
				got = append(got, int(row["id"].(float64)))
			}
			require.ElementsMatch(t, want, got)
			var stored []int
			require.NoError(t, db.Table("forward_attachments").Where("application_id = ? AND recipient_user_id = ?", app, id).Pluck("attachment_id", &stored).Error)
			if tc.wantAll && tc.prior != "subset" {
				require.Empty(t, stored, "existing all access stays all, not a new restriction")
				c := suppAttachment(t, db, app, "items", until)
				read = testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", app), testutil.AuthHeader(targetToken))
				require.Equal(t, http.StatusOK, read.Code)
				got = nil
				for _, row := range testutil.ParseSlice(t, read) {
					got = append(got, int(row["id"].(float64)))
				}
				require.ElementsMatch(t, []int{a, b, c}, got)
			} else {
				require.ElementsMatch(t, want, stored)
			}
			var scope string
			require.NoError(t, db.Raw("SELECT details->'metadata'->>'attachment_scope' FROM audit_log WHERE entity_type = 'application' AND entity_id = ? AND action = 'forwarded' ORDER BY id DESC LIMIT 1", app).Scan(&scope).Error)
			if tc.actor == "limited" || len(selected) > 0 {
				require.Equal(t, "selected", scope)
			} else {
				require.Equal(t, "all", scope)
			}
		})
	}
}

func TestForwardScope_LookupFailureRollsBack(t *testing.T) {
	e, base, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, base)
	td := testutil.SeedTestData(t, base)
	testutil.RegisterUser(t, e, "scope_fault_author", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "scope_fault_target", "pass123", 1, td.OrgID, td.CompanyID)
	authorID, targetID := getUserID(t, base, "scope_fault_author"), getUserID(t, base, "scope_fault_target")
	pool, err := base.DB()
	require.NoError(t, err)
	for index, match := range []string{"FROM forward_attachments WHERE application_id", "SELECT id FROM attachments WHERE application_id", "SELECT u.id FROM users u WHERE u.id IN"} {
		t.Run(fmt.Sprintf("query_%d", index), func(t *testing.T) {
			db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{Logger: base.Logger})
			require.NoError(t, err)
			app := suppApp(t, db, td.OrgID, authorID, fmt.Sprintf("SCOPE-FAULT-%d", index), models.ConfirmationPending, models.StatusProcessing)
			att := suppAttachment(t, db, app, "items", time.Now().UTC().AddDate(1, 0, 0).Format("2006-01-02"))
			before := remainingTransactionSnapshot(t, db)
			hit := false
			name := "test:forward_scope_lookup"
			require.NoError(t, db.Callback().Row().Before("gorm:row").Register(name, func(tx *gorm.DB) {
				if strings.Contains(tx.Statement.SQL.String(), match) {
					hit = true
					tx.AddError(errors.New("synthetic scope lookup failure"))
				}
			}))
			t.Cleanup(func() { require.NoError(t, db.Callback().Row().Remove(name)) })
			pub := &fakePublisher{}
			err = transactionIntegrityService(db, pub).ForwardApplication(context.Background(), "scope_fault_author", app, false, services.ForwardApplicationRequest{
				Users: []services.ForwardUser{{UserID: targetID, CanView: true}}, AttachmentIDs: []int{att},
			})
			require.True(t, hit)
			var failure *echo.HTTPError
			var domain *apperr.Error
			require.Error(t, err)
			if errors.As(err, &failure) {
				require.Equal(t, http.StatusInternalServerError, failure.Code)
			} else {
				require.ErrorAs(t, err, &domain)
				require.Equal(t, http.StatusInternalServerError, domain.Code)
			}
			require.Equal(t, before, remainingTransactionSnapshot(t, db))
			require.Empty(t, pub.events)
		})
	}
}
