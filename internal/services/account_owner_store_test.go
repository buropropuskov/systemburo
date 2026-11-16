package services_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

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
