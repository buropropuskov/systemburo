package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// NotificationIntentWorker has no per-event goroutines. The owner runs Process
// from one lifecycle-owned loop with both periodic polling and Wake hints.
type NotificationIntentWorker struct {
	db            *gorm.DB
	app           *applicationService
	notifications *notificationService
	wake          chan struct{}
	now           func() time.Time
}

func NewNotificationIntentWorker(db *gorm.DB, app ApplicationService, notifications NotificationService) (*NotificationIntentWorker, error) {
	a, ok := app.(*applicationService)
	n, nok := notifications.(*notificationService)
	if db == nil || !ok || a == nil || !nok || n == nil {
		return nil, fmt.Errorf("notification intent worker requires database-backed application and notification services")
	}
	return &NotificationIntentWorker{db: db, app: a, notifications: n, wake: make(chan struct{}, 1), now: time.Now}, nil
}

func (w *NotificationIntentWorker) Wake() <-chan struct{} { return w.wake }

func (w *NotificationIntentWorker) Nudge() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Process handles at most 25 due immutable events sequentially. In-app writes
// and ACK share one transaction; dispatch is best-effort after its commit.
func (w *NotificationIntentWorker) Process(ctx context.Context) (processed, failed int) {
	var due []models.NotificationIntent
	scanCtx, cancelScan := context.WithTimeout(ctx, 5*time.Second)
	scanErr := w.db.WithContext(scanCtx).Where("state = ? AND next_attempt_at <= ?", "pending", w.now()).
		Order("next_attempt_at, event_key").Limit(notificationIntentBatch).Find(&due).Error
	cancelScan()
	if scanErr != nil {
		if ctx.Err() == nil {
			slog.Warn("notification intent scan failed", "error_type", fmt.Sprintf("%T", scanErr))
		}
		return 0, 1
	}
	for _, candidate := range due {
		if ctx.Err() != nil {
			break
		}
		// PermissionResolver owns its connection/cache; finish those reads before
		// taking either domain or intent locks. Actual audience is re-read via tx.
		jobCtx, cancelJob := context.WithTimeout(ctx, 120*time.Second)
		rights, err := w.prepareModeratorRights(jobCtx, candidate)
		var dispatches []NotificationDispatch
		done := false
		if err == nil {
			err = w.db.WithContext(jobCtx).Transaction(func(tx *gorm.DB) error {
				var app notificationIntentApplication
				columns := "id, application_number, organization_id, company_id, sender_user_id, message"
				if importedNotificationIntent(candidate.Kind) {
					columns = "id, organization_id, company_id, sender_user_id"
				}
				loadErr := tx.Table("applications").Clauses(clause.Locking{Strength: "KEY SHARE"}).
					Select(columns).
					Where("id = ?", candidate.ApplicationID).Take(&app).Error
				if errors.Is(loadErr, gorm.ErrRecordNotFound) {
					// No rows may be recreated after physical purge.
					result := tx.Where("event_key = ? AND application_id = ?", candidate.EventKey, candidate.ApplicationID).Delete(&models.NotificationIntent{})
					done = result.RowsAffected > 0
					return result.Error
				}
				if loadErr != nil {
					return loadErr
				}
				var intent models.NotificationIntent
				loadErr = tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
					Where("event_key = ? AND state = ? AND next_attempt_at <= ?", candidate.EventKey, "pending", w.now()).Take(&intent).Error
				if errors.Is(loadErr, gorm.ErrRecordNotFound) {
					return nil
				}
				if loadErr != nil {
					return loadErr
				}
				if intent.ApplicationID != candidate.ApplicationID {
					return fmt.Errorf("notification intent application changed")
				}
				drafts, err := w.buildIntentDrafts(jobCtx, tx, intent, app, rights)
				if err != nil {
					return err
				}
				dispatches, err = w.notifications.PersistNotificationBatchTx(jobCtx, tx, drafts)
				if err != nil {
					return err
				}
				if err := tx.Model(&models.NotificationIntent{}).Where("event_key = ?", intent.EventKey).
					Updates(map[string]any{"state": "done", "completed_at": w.now()}).Error; err != nil {
					return err
				}
				done = true
				return nil
			})
		}
		cancelJob()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			failed++
			slog.Warn("notification intent deferred", "kind", candidate.Kind, "error_type", fmt.Sprintf("%T", err))
			// Retry is detached from the expired job, but never from shutdown.
			retryCtx, cancelRetry := context.WithTimeout(ctx, 5*time.Second)
			retryErr := w.retry(retryCtx, candidate)
			cancelRetry()
			if retryErr != nil {
				slog.Warn("notification intent retry scheduling failed", "error_type", fmt.Sprintf("%T", retryErr))
			}
			continue
		}
		if done {
			processed++
			dispatchCtx, cancelDispatch := context.WithTimeout(ctx, 5*time.Second)
			w.notifications.DispatchCommitted(dispatchCtx, dispatches)
			cancelDispatch()
		}
	}
	return processed, failed
}

func (w *NotificationIntentWorker) retry(ctx context.Context, intent models.NotificationIntent) error {
	return w.db.WithContext(ctx).Model(&models.NotificationIntent{}).
		Where("event_key = ? AND state = ? AND next_attempt_at = ? AND attempts = ?", intent.EventKey, "pending", intent.NextAttemptAt, intent.Attempts).
		Updates(map[string]any{"attempts": intent.Attempts + 1, "next_attempt_at": w.now().Add(notificationIntentBackoff(intent.Attempts + 1))}).Error
}

func (w *NotificationIntentWorker) prepareModeratorRights(ctx context.Context, intent models.NotificationIntent) (map[int]bool, error) {
	rights := make(map[int]bool)
	if (intent.Kind != notificationIntentSubmit && intent.Kind != notificationIntentImportedSubmit) || (intent.PendingOrganizationID == nil && intent.PendingCompanyID == nil) {
		return rights, nil
	}
	if w.app.permissionResolver == nil {
		return nil, fmt.Errorf("notification moderation resolver is required")
	}
	excludedID := intent.ActorUserID
	if intent.Kind == notificationIntentImportedSubmit {
		var app struct{ SenderUserID int }
		err := w.db.WithContext(ctx).Table("applications").Select("sender_user_id").Where("id = ?", intent.ApplicationID).Take(&app).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return rights, nil
		}
		if err != nil {
			return nil, err
		}
		excludedID = app.SenderUserID
	}
	audience, err := notificationIntentAudience(ctx, w.db, intent.ApplicationID, excludedID)
	if err != nil {
		return nil, err
	}
	for _, userID := range audience {
		if userID == excludedID {
			continue
		}
		set, err := w.app.permissionResolver.ResolveFresh(ctx, userID)
		if err != nil {
			// A physically removed account is a normal skip, not a poison event.
			var count int64
			if checkErr := w.db.WithContext(ctx).Table("users").Where("id = ?", userID).Count(&count).Error; checkErr != nil {
				return nil, checkErr
			}
			if count == 0 {
				rights[userID] = false
				continue
			}
			return nil, err
		}
		rights[userID] = set.Has(KeyApplicationOrganizationModerate)
	}
	return rights, nil
}
