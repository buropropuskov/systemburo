package services

import (
	"testing"
	"time"

	"systemburo/internal/models"
)

// Persistence, coalescing and ACK behavior live in handlers' DB-backed tests.
func TestBlankExportQueue_NudgeNonBlocking(t *testing.T) {
	outbox := NewArchiveExportOutbox(nil)
	outbox.Nudge()
	done := make(chan struct{})
	go func() {
		outbox.Nudge()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("nudge blocked while the wake channel was full")
	}
	select {
	case <-outbox.Wake():
	default:
		t.Fatal("nudge did not wake the worker")
	}
}

func TestBlankExportService_NilReceiverSafe(t *testing.T) {
	var service *BlankExportService
	service.EnqueueApplication(1, BlankExportReasonSubmit)
	service.EnqueueApplications([]int{1, 2}, BlankExportReasonUpdate)
	service.Nudge()
	service.SetArchiveExportOutbox(nil)
	var outbox *ArchiveExportOutbox
	outbox.EnqueueApplication(1, BlankExportReasonSubmit)
	outbox.EnqueueApplications([]int{1, 2}, BlankExportReasonUpdate)
}

func TestBlankExportWorker_CompleteOutcomes(t *testing.T) {
	result := &models.BlankExportResult{Snapshot: models.BlankExportSnapshotResult{Status: models.BlankExportOK}}
	if !archiveExportComplete(result) {
		t.Fatal("an application without ordinary attachments can complete")
	}
	for _, status := range []string{models.BlankExportOK, models.BlankExportSkipped, models.BlankExportNoTemplate, models.BlankExportOrphan} {
		result.Items = []models.BlankExportItem{{Status: status, Frozen: true}}
		if !archiveExportComplete(result) {
			t.Fatalf("persisted outcome %s was rejected", status)
		}
	}
	for _, status := range []string{"", models.BlankExportPending, models.BlankExportFailed, models.BlankExportBlocked} {
		result.Items = []models.BlankExportItem{{Status: status}}
		if archiveExportComplete(result) {
			t.Fatalf("incomplete outcome %s was accepted", status)
		}
	}
	result.Items = nil
	result.Snapshot.Status = models.BlankExportFailed
	if archiveExportComplete(result) || archiveExportComplete(nil) {
		t.Fatal("failed or absent snapshot was accepted")
	}
}
