package services

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// Intent is mandatory even when no file writer or postcommit observer is wired.
// Callers return this error from their mutation transaction before acknowledging it.
func enqueueArchiveExportTx(ctx context.Context, tx *gorm.DB, ids []int, reason string) error {
	if tx == nil {
		return errors.New("archive intent requires a transaction")
	}
	store := ArchiveExportOutbox{db: tx}
	return store.EnqueueTx(ctx, tx, ids, reason)
}

func enqueueArchiveSelectionTx(ctx context.Context, tx *gorm.DB, selectSQL string, args []interface{}, reason string) (int64, error) {
	if tx == nil {
		return 0, errors.New("archive intent requires a transaction")
	}
	store := ArchiveExportOutbox{db: tx}
	return store.EnqueueSelectionTx(ctx, tx, selectSQL, args, reason)
}

// The postcommit observer wakes the worker without rewriting a newer revision.
// Durable intent is written separately, inside the corresponding mutation TX.
type archiveExportWakeup struct{ store *ArchiveExportOutbox }

func NewArchiveExportWakeup(store *ArchiveExportOutbox) BlankExportEnqueuer {
	return archiveExportWakeup{store: store}
}

func (w archiveExportWakeup) EnqueueApplication(id int, _ string) {
	if id > 0 && w.store != nil {
		w.store.Nudge()
	}
}

func (w archiveExportWakeup) EnqueueApplications(_ []int, _ string) {
	if w.store != nil {
		w.store.Nudge()
	}
}
