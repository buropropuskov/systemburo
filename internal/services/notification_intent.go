package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	notificationIntentSubmit     = "application_submitted"
	notificationIntentSupplement = "supplement_created"
	notificationIntentBatch      = 25
)

// NotificationPendingDirectoryIDs includes only entries newly created by this
// submission (directoryResolution.PendingName != ""). Existing pending entries
// must not generate duplicate moderation invitations.
type NotificationPendingDirectoryIDs struct {
	OrganizationID *int
	CompanyID      *int
}

// EnqueueApplicationSubmittedNotificationTx never wakes a worker before commit.
func EnqueueApplicationSubmittedNotificationTx(ctx context.Context, tx *gorm.DB, applicationID, actorUserID int, pending ...NotificationPendingDirectoryIDs) error {
	if len(pending) > 1 {
		return fmt.Errorf("one pending-directory set is required")
	}
	intent := models.NotificationIntent{EventKey: fmt.Sprintf("submit:%d", applicationID), Kind: notificationIntentSubmit, ApplicationID: applicationID, ActorUserID: actorUserID}
	if len(pending) == 1 {
		intent.PendingOrganizationID = pending[0].OrganizationID
		intent.PendingCompanyID = pending[0].CompanyID
	}
	return enqueueNotificationIntentTx(ctx, tx, intent)
}

func EnqueueApplicationSupplementNotificationTx(ctx context.Context, tx *gorm.DB, applicationID, supplementID, actorUserID int, inWork bool) error {
	intent := models.NotificationIntent{EventKey: fmt.Sprintf("supplement-created:%d", supplementID), Kind: notificationIntentSupplement, ApplicationID: applicationID, SupplementID: &supplementID, ActorUserID: actorUserID, InWork: inWork}
	return enqueueNotificationIntentTx(ctx, tx, intent)
}

func enqueueNotificationIntentTx(ctx context.Context, tx *gorm.DB, intent models.NotificationIntent) error {
	if tx == nil || intent.ApplicationID <= 0 || intent.ActorUserID <= 0 || (intent.SupplementID != nil && *intent.SupplementID <= 0) {
		return fmt.Errorf("invalid notification intent identity")
	}
	for _, id := range []*int{intent.PendingOrganizationID, intent.PendingCompanyID} {
		if id != nil && *id <= 0 {
			return fmt.Errorf("invalid notification directory identity")
		}
	}
	intent.OccurredAt = time.Now()
	intent.NextAttemptAt = intent.OccurredAt
	intent.State = "pending"
	// A duplicate business command must not overwrite a pending immutable event.
	return tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&intent).Error
}

func notificationIntentBackoff(attempts int) time.Duration {
	delay := time.Second
	for i := 1; i < attempts && delay < 5*time.Minute; i++ {
		delay *= 2
	}
	return min(delay, 5*time.Minute)
}
