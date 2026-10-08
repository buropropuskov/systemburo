package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
)

// ConfigureEntityPeriodUpdates wires the existing application/table/archive
// publishers at startup. The closure runs only after the command commits.
func ConfigureEntityPeriodUpdates(commands *EntityPeriodCommandService, applications ApplicationService) error {
	app, ok := applications.(*applicationService)
	if commands == nil || !ok || app == nil {
		return fmt.Errorf("entity period update publishers require application service")
	}
	commands.afterChange = func(ctx context.Context, actorID int, kind ElementKind, old, next EntityPeriodCommandResult, reason string) {
		if app.tablesProducer != nil {
			if kind == ElementCar {
				app.tablesProducer.NotifyCarsChangedBatch(ctx, []int{next.EntityID})
			} else {
				app.tablesProducer.NotifyEmployeesChangedBatch(ctx, []int{next.EntityID})
			}
		}
		if app.availableProducer != nil {
			app.availableProducer.NotifyAvailableChanged(ctx)
		}
		if next.ApplicationID == nil {
			return
		}
		applicationID := *next.ApplicationID
		app.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
		if app.notificationService == nil {
			return
		}
		entity := "человека"
		if kind == ElementCar {
			entity = "машины"
		}
		message := fmt.Sprintf("Изменён срок %s: было %s, стало %s. Причина: %s", entity, describeEntityPeriod(old), describeEntityPeriod(next), reason)
		payload, err := json.Marshal(map[string]any{
			"application_id": applicationID, "entity_id": next.EntityID,
			"entity_kind": kind, "approvals_reset": false,
		})
		if err != nil {
			slog.Error("period notification payload", "application_id", applicationID, "err", err)
			return
		}
		data := string(payload)
		for _, userID := range app.applicationParticipants(ctx, applicationID) {
			if userID == actorID {
				continue
			}
			if err := app.notificationService.CreateForUser(ctx, userID, NotificationTypeApplicationDatesChanged, "Изменён индивидуальный срок", message, &data); err != nil {
				slog.Warn("period notification", "application_id", applicationID, "user_id", userID, "err", err)
			}
		}
	}
	return nil
}

// ConfigureAttachmentPeriodUpdates publishes only after all selected rows and
// audit records have committed; previews and rolled-back changes stay silent.
func ConfigureAttachmentPeriodUpdates(commands *AttachmentPeriodCommandService, applications ApplicationService) error {
	app, ok := applications.(*applicationService)
	if commands == nil || !ok || app == nil {
		return fmt.Errorf("attachment period update publishers require application service")
	}
	commands.afterChange = func(ctx context.Context, actorID int, result AttachmentPeriodCommandResult, reason string) {
		if app.tablesProducer != nil {
			if len(result.ChangedCarIDs) > 0 {
				app.tablesProducer.NotifyCarsChangedBatch(ctx, result.ChangedCarIDs)
			}
			if len(result.ChangedEmployeeIDs) > 0 {
				app.tablesProducer.NotifyEmployeesChangedBatch(ctx, result.ChangedEmployeeIDs)
			}
		}
		if app.availableProducer != nil {
			app.availableProducer.NotifyAvailableChanged(ctx)
		}
		app.notifyApplicationUpdated(ctx, result.ApplicationID, archiveDataChanged)
		if app.notificationService == nil {
			return
		}
		payload, err := json.Marshal(map[string]any{
			"application_id": result.ApplicationID, "attachment_ids": result.AttachmentIDs,
			"approvals_reset": false,
		})
		if err != nil {
			return
		}
		data := string(payload)
		message := fmt.Sprintf("Изменён срок выбранных вложений: %s. Причина: %s", attachmentPeriodText(result.NewPeriod), reason)
		for _, userID := range app.applicationParticipants(ctx, result.ApplicationID) {
			if userID == actorID {
				continue
			}
			if err := app.notificationService.CreateForUser(ctx, userID, NotificationTypeApplicationDatesChanged, "Изменён срок вложений", message, &data); err != nil {
				slog.Warn("attachment period notification", "application_id", result.ApplicationID, "user_id", userID, "err", err)
			}
		}
	}
	return nil
}

// Manual moves change both the application contents and its generated blanks.
// Existing table/available publishers remain owned by the attach service.
func ConfigureManualAttachUpdates(manual ManualAttachService, applications ApplicationService) error {
	attach, ok := manual.(*manualAttachService)
	app, appOK := applications.(*applicationService)
	if !ok || attach == nil || !appOK || app == nil {
		return fmt.Errorf("manual attach publishers require application service")
	}
	attach.afterAttach = func(ctx context.Context, applicationID int) {
		app.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
	}
	return nil
}

// Single-record moves invalidate the same application content and blank cache.
func ConfigureSingleManualAttachUpdates(attach *SingleManualAttachCommandService, applications ApplicationService) error {
	app, ok := applications.(*applicationService)
	if attach == nil || !ok || app == nil {
		return fmt.Errorf("single manual attach publishers require application service")
	}
	attach.SetAfterAttach(func(ctx context.Context, applicationID int) {
		app.notifyApplicationUpdated(ctx, applicationID, archiveDataChanged)
	})
	return nil
}
