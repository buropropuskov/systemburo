package services_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/database"
	"systemburo/internal/handlers"
	"systemburo/internal/middleware"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAccountOwnerEncryptionKeyRotation(t *testing.T) {
	f := setupIntentFixture(t)
	ctx := context.Background()
	oldKey := bytes.Repeat([]byte{0x72}, 32)
	newKey := bytes.Repeat([]byte{0x73}, 32)
	evidence, err := crypto.Encrypt("Учебное подтверждение", oldKey)
	require.NoError(t, err)
	reason, err := crypto.Encrypt("Учебное основание", oldKey)
	require.NoError(t, err)
	confirmation := models.AccountOwnerConfirmation{
		UserID: f.approver.ID, ConfirmedByID: f.author.ID,
		EffectiveFrom: time.Date(2021, 11, 15, 7, 42, 0, 0, time.UTC), Evidence: evidence,
	}
	require.NoError(t, f.db.Create(&confirmation).Error)
	lookup := models.AccountOwnerVerification{
		ApprovalEventID: 1, ActorUserID: f.author.ID, ConfirmationID: &confirmation.ID, Reason: reason,
	}
	require.NoError(t, f.db.Create(&lookup).Error)

	_, err = database.Reencrypt(ctx, f.db, database.ReencryptOptions{OldKey: oldKey, Apply: true})
	require.ErrorContains(t, err, "для перевода account_owner_confirmations требуются прежний и новый ключи")
	_, err = database.Reencrypt(ctx, f.db, database.ReencryptOptions{OldKey: oldKey, NewKey: newKey, Apply: true})
	require.NoError(t, err)
	require.NoError(t, f.db.First(&confirmation, confirmation.ID).Error)
	require.NoError(t, f.db.First(&lookup, lookup.ID).Error)
	for ciphertext, expected := range map[string]string{confirmation.Evidence: "Учебное подтверждение", lookup.Reason: "Учебное основание"} {
		actual, err := crypto.Decrypt(ciphertext, newKey)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
		_, err = crypto.Decrypt(ciphertext, oldKey)
		require.Error(t, err)
	}
}

func TestAccountOwnerLookupHTTP(t *testing.T) {
	f := setupIntentFixture(t)
	app := f.createApplication(t, f.db)
	approvedAt := time.Date(2021, 11, 15, 7, 42, 0, 0, time.UTC)
	event := models.AuditLog{
		EntityType: models.AuditEntityApplication, EntityID: &app.ID,
		Action: "approve", ActorUserID: &f.approver.ID, CreatedAt: approvedAt,
	}
	require.NoError(t, f.db.Create(&event).Error)
	require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.author.ID).Update("is_admin", true).Error)

	store := services.NewAccountOwnerStore(f.db, bytes.Repeat([]byte{0x72}, 32))
	h := handlers.NewAccountOwnerHandler(store, f.app)
	e := echo.New()
	authenticated := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			middleware.SetSessionIdentity(c, models.SessionIdentity{
				OwnerUserID: f.author.ID, EffectiveUserID: f.author.ID, Username: f.author.Username,
			})
			return next(c)
		}
	}
	e.POST("/applications/:id/history/:event_id/account-owner", authenticated(h.Verify))
	e.GET("/account-owner/users/:id", authenticated(h.Confirmation))
	e.POST("/account-owner/users/:id", authenticated(h.Confirm))
	send := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		return rec
	}
	lookup := func(eventID int, body string) *httptest.ResponseRecorder {
		t.Helper()
		return send(http.MethodPost, fmt.Sprintf("/applications/%d/history/%d/account-owner", app.ID, eventID), body)
	}
	userURL := fmt.Sprintf("/account-owner/users/%d", f.approver.ID)
	confirmation := `{"full_name":"Учебный владелец","source_ref":"Учебная запись № 18","effective_from":"2021-11-15T06:42:00Z","expected_revision":0,"confirmed":true}`
	reason := `{"reason":"Учебное обращение № 17"}`
	require.Equal(t, http.StatusForbidden, lookup(event.ID, reason).Code)
	require.Equal(t, http.StatusForbidden, send(http.MethodGet, userURL, "").Code)
	require.Equal(t, http.StatusForbidden, send(http.MethodPost, userURL, confirmation).Code)
	require.NoError(t, f.db.Create(&models.UserPermissionOverride{
		UserID: f.author.ID, PermissionKey: services.KeyActionVerifyAccountOwner, Value: "allow",
	}).Error)
	require.Equal(t, http.StatusBadRequest, lookup(event.ID, `{"reason":" "}`).Code)
	require.Equal(t, http.StatusNotFound, lookup(event.ID+1, reason).Code)

	unconfirmed := lookup(event.ID, reason)
	require.Equal(t, http.StatusOK, unconfirmed.Code, unconfirmed.Body.String())
	require.NotContains(t, unconfirmed.Body.String(), "full_name")
	require.Equal(t, http.StatusForbidden, send(http.MethodPost, fmt.Sprintf("/account-owner/users/%d", f.author.ID), confirmation).Code)
	saved := send(http.MethodPost, userURL, confirmation)
	require.Equal(t, http.StatusOK, saved.Code, saved.Body.String())
	require.Equal(t, http.StatusConflict, send(http.MethodPost, userURL, confirmation).Code)
	status := send(http.MethodGet, userURL, "")
	require.Equal(t, http.StatusOK, status.Code, status.Body.String())
	require.NotContains(t, status.Body.String(), "Учебный владелец")
	confirmed := lookup(event.ID, reason)
	require.Equal(t, http.StatusOK, confirmed.Code, confirmed.Body.String())
	require.Contains(t, confirmed.Body.String(), "Учебный владелец")
	require.NotContains(t, confirmed.Body.String(), f.approver.Username)

	require.NoError(t, f.db.Model(&event).Update("details", fmt.Sprintf(`{"impersonated_by":%d}`, f.author.ID)).Error)
	impersonated := lookup(event.ID, reason)
	require.Equal(t, http.StatusOK, impersonated.Code, impersonated.Body.String())
	require.Contains(t, impersonated.Body.String(), fmt.Sprintf(`"on_behalf_of":%d`, f.approver.ID))
	require.Contains(t, impersonated.Body.String(), fmt.Sprintf(`"user_id":%d`, f.author.ID))
	require.NotContains(t, impersonated.Body.String(), "Учебный владелец")
}

func TestAccountOwnerAccessRequiresEligibleRoleAndDirectGrant(t *testing.T) {
	f := setupIntentFixture(t)
	store := services.NewAccountOwnerStore(f.db, bytes.Repeat([]byte{0x72}, 32))
	userID := f.author.ID
	identity := models.SessionIdentity{OwnerUserID: userID, EffectiveUserID: userID}
	ctx := services.WithSessionIdentity(context.Background(), identity)
	check := func(ctx context.Context, verify, confirm bool) {
		t.Helper()
		actual, err := store.Access(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, services.AccountOwnerAccess{Verify: verify, Confirm: confirm}, actual)
	}

	user := f.db.Model(&models.User{}).Where("id = ?", userID)
	require.NoError(t, user.Update("is_super_admin", true).Error)
	check(ctx, false, false)

	grant := models.UserPermissionOverride{
		UserID: userID, PermissionKey: services.KeyActionVerifyAccountOwner, Value: "allow",
	}
	require.NoError(t, f.db.Create(&grant).Error)
	check(ctx, true, true)
	check(context.Background(), false, false)
	identity.OwnerUserID = f.approver.ID
	identity.ImpersonatedBy = &f.approver.ID
	check(services.WithSessionIdentity(context.Background(), identity), false, false)

	require.NoError(t, user.Update("is_super_admin", false).Error)
	check(ctx, false, false)

	var bureauType models.UserType
	require.NoError(t, f.db.Where("code = ?", "buropropuskov").First(&bureauType).Error)
	require.NoError(t, user.Update("type_id", bureauType.ID).Error)
	check(ctx, true, false)
	require.NoError(t, user.Update("is_banned", true).Error)
	check(ctx, false, false)
}

func TestAccountOwnerConfirmationForApproval(t *testing.T) {
	f := setupIntentFixture(t)
	app := f.createApplication(t, f.db)
	ctx := context.Background()
	key := bytes.Repeat([]byte{0x72}, 32)
	store := services.NewAccountOwnerStore(f.db, key)
	approvedAt := time.Date(2021, 11, 15, 7, 42, 0, 0, time.UTC)
	event := models.AuditLog{
		EntityType: models.AuditEntityApplication, EntityID: &app.ID,
		Action: "approve", ActorUserID: &f.approver.ID, CreatedAt: approvedAt,
	}
	require.NoError(t, f.db.Create(&event).Error)
	reason := "Учебное обращение № 17"

	unconfirmed, err := store.VerifyApproval(ctx, app.ID, event.ID, f.author.ID, reason)
	require.NoError(t, err)
	require.Equal(t, f.approver.ID, unconfirmed.UserID)
	require.True(t, approvedAt.Equal(unconfirmed.Approved))
	require.Nil(t, unconfirmed.Evidence)

	input := services.AccountOwnerConfirmationInput{
		EffectiveFrom: approvedAt.Add(-time.Hour),
		Evidence: services.AccountOwnerEvidence{
			FullName: "Учебный владелец", SourceRef: "Учебная запись выдачи № 18",
		},
	}
	_, err = services.NewAccountOwnerStore(f.db, nil).Confirm(ctx, f.approver.ID, f.author.ID, input)
	require.ErrorIs(t, err, services.ErrAccountOwnerStorageUnavailable)
	revision, err := store.Confirm(ctx, f.approver.ID, f.author.ID, input)
	require.NoError(t, err)
	_, err = store.Confirm(ctx, f.approver.ID, f.author.ID, input)
	require.ErrorIs(t, err, services.ErrAccountOwnerConfirmationChanged)

	var stored models.AccountOwnerConfirmation
	require.NoError(t, f.db.First(&stored, revision).Error)
	require.NotContains(t, stored.Evidence, input.Evidence.FullName)
	require.NotContains(t, stored.Evidence, input.Evidence.SourceRef)
	require.NoError(t, f.db.Model(&models.User{}).Where("id = ?", f.approver.ID).
		Updates(map[string]any{"last_name": "Изменённый профиль", "is_active": false}).Error)
	result, err := store.VerifyApproval(ctx, app.ID, event.ID, f.author.ID, reason)
	require.NoError(t, err)
	require.NotNil(t, result.Evidence)
	require.Equal(t, input.Evidence, *result.Evidence)

	var lookup models.AccountOwnerVerification
	require.NoError(t, f.db.Where("approval_event_id = ? AND confirmation_id = ?", event.ID, revision).Take(&lookup).Error)
	require.Equal(t, f.author.ID, lookup.ActorUserID)
	require.NotContains(t, lookup.Reason, reason)
	actualReason, err := crypto.Decrypt(lookup.Reason, key)
	require.NoError(t, err)
	require.Equal(t, reason, actualReason)
	var auditCount int64
	require.NoError(t, f.db.Model(&models.PDAuditLog{}).
		Where("resource = ? AND resource_id = ?", "account_owner", event.ID).Count(&auditCount).Error)
	require.EqualValues(t, 2, auditCount)

	_, err = store.VerifyApproval(ctx, app.ID+1, event.ID, f.author.ID, reason)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = store.VerifyApproval(ctx, app.ID, event.ID, f.author.ID, " ")
	require.ErrorIs(t, err, services.ErrInvalidAccountOwnerConfirmation)

	require.NoError(t, f.db.Exec(`ALTER TABLE pd_audit_logs ADD CONSTRAINT
		reject_owner_lookup_test CHECK (resource <> 'account_owner') NOT VALID`).Error)
	t.Cleanup(func() {
		require.NoError(t, f.db.Exec("ALTER TABLE pd_audit_logs DROP CONSTRAINT IF EXISTS reject_owner_lookup_test").Error)
	})
	blocked, err := store.VerifyApproval(ctx, app.ID, event.ID, f.author.ID, reason)
	require.Error(t, err)
	require.Nil(t, blocked)
	var savedLookups int64
	require.NoError(t, f.db.Model(&models.AccountOwnerVerification{}).
		Where("approval_event_id = ?", event.ID).Count(&savedLookups).Error)
	require.EqualValues(t, 2, savedLookups, "the supporting lookup log must roll back")
	require.NoError(t, f.db.Exec("ALTER TABLE pd_audit_logs DROP CONSTRAINT reject_owner_lookup_test").Error)

	input.ExpectedRevision = revision
	input.EffectiveFrom = approvedAt.Add(time.Hour)
	_, err = store.Confirm(ctx, f.approver.ID, f.author.ID, input)
	require.NoError(t, err)
	corrected, err := store.VerifyApproval(ctx, app.ID, event.ID, f.author.ID, reason)
	require.NoError(t, err)
	require.Nil(t, corrected.Evidence)
}
