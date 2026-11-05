package services

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNotificationIntentRequiresTransactionAndValidIdentity(t *testing.T) {
	ctx := context.Background()
	require.Error(t, EnqueueApplicationSubmittedNotificationTx(ctx, nil, 1, 1))
	require.Error(t, EnqueueApplicationSupplementNotificationTx(ctx, nil, 1, 1, 1, true))
	require.Error(t, EnqueueApplicationSubmittedNotificationTx(ctx, &gorm.DB{}, 0, 1))
	require.Error(t, EnqueueApplicationSubmittedNotificationTx(ctx, &gorm.DB{}, 1, 0))
	negative := -1
	require.Error(t, EnqueueApplicationSubmittedNotificationTx(ctx, &gorm.DB{}, 1, 1, NotificationPendingDirectoryIDs{OrganizationID: &negative}))
	require.Error(t, EnqueueApplicationSupplementNotificationTx(ctx, &gorm.DB{}, 1, 0, 1, true))
}

func TestNotificationIntentBackoffIsBoundedWithoutOverflow(t *testing.T) {
	for _, tc := range []struct {
		attempts int
		want     time.Duration
	}{
		{0, time.Second}, {1, time.Second}, {2, 2 * time.Second}, {9, 256 * time.Second}, {10, 5 * time.Minute}, {1 << 30, 5 * time.Minute},
	} {
		require.Equal(t, tc.want, notificationIntentBackoff(tc.attempts))
	}
}

type unsupportedIntentApplication struct{ ApplicationService }
type unsupportedIntentNotifications struct{ NotificationService }

func TestNotificationIntentConstructorRejectsUnsupportedFakes(t *testing.T) {
	_, err := NewNotificationIntentWorker(&gorm.DB{}, unsupportedIntentApplication{}, unsupportedIntentNotifications{})
	require.Error(t, err)
	_, err = NewNotificationIntentWorker(nil, nil, nil)
	require.Error(t, err)
	worker, err := NewNotificationIntentWorker(&gorm.DB{}, &applicationService{}, &notificationService{})
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		worker.Nudge()
	}
	require.Len(t, worker.wake, 1)
	<-worker.Wake()
}
