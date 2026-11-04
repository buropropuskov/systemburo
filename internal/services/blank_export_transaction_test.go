package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchiveExportWakeupDoesNotWriteAndCoalesces(t *testing.T) {
	// No database: the observer must only signal, never issue a second enqueue.
	store := NewArchiveExportOutbox(nil)
	observer := NewArchiveExportWakeup(store)
	observer.EnqueueApplication(17, "update")
	observer.EnqueueApplications([]int{18, 19}, "update")
	require.Len(t, store.wake, 1)
	<-store.Wake()
	// SQL-selected callers do not materialize the application IDs in Go.
	observer.EnqueueApplications(nil, "update")
	require.Len(t, store.wake, 1)
}

func TestArchiveExportTransactionRequiresDatabase(t *testing.T) {
	require.Error(t, enqueueArchiveExportTx(context.Background(), nil, []int{1}, "update"))
	_, err := enqueueArchiveSelectionTx(context.Background(), nil, "SELECT 1 AS application_id", nil, "update")
	require.Error(t, err)
}
