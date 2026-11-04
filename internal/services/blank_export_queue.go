package services

import (
	"context"
	"log/slog"
	"time"
)

// BlankExportEnqueuer preserves postcommit callers while they move to EnqueueTx.
// The void adapter cannot make the preceding mutation atomic with enqueue.
type BlankExportEnqueuer interface {
	EnqueueApplication(applicationID int, reason string)
	EnqueueApplications(applicationIDs []int, reason string)
}

const archiveEnqueueTimeout = 5 * time.Second

func (s *ArchiveExportOutbox) EnqueueApplication(applicationID int, reason string) {
	s.EnqueueApplications([]int{applicationID}, reason)
}

func (s *ArchiveExportOutbox) EnqueueApplications(applicationIDs []int, reason string) {
	if s == nil || len(applicationIDs) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), archiveEnqueueTimeout)
	defer cancel()
	if err := s.Enqueue(ctx, applicationIDs, reason); err != nil {
		slog.Error("archive enqueue failed", "application_count", len(applicationIDs), "error", err)
	}
}

// SetArchiveExportOutbox shares the independently constructed durable store.
// Configure at startup before processing requests.
func (s *BlankExportService) SetArchiveExportOutbox(store *ArchiveExportOutbox) {
	if s != nil && store != nil {
		s.outbox = store
	}
}

func (s *BlankExportService) EnqueueApplication(applicationID int, reason string) {
	if s != nil {
		s.outbox.EnqueueApplication(applicationID, reason)
	}
}

func (s *BlankExportService) EnqueueApplications(applicationIDs []int, reason string) {
	if s != nil {
		s.outbox.EnqueueApplications(applicationIDs, reason)
	}
}

func (s *BlankExportService) Nudge() {
	if s != nil && s.outbox != nil {
		s.outbox.Nudge()
	}
}
