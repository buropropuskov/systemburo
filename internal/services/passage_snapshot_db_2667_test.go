package services_test

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"testing"
)

func TestPassage2667DBSnapshotCountsAndFrozenExpiry(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementCar, services.ElementEmployee} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPassage2667World(t, kind)
			ctx := context.Background()
			recorder := services.NewAuditRecorder(w.db)
			cars := services.NewCarService(w.db, recorder)
			employees := services.NewEmployeeService(w.db, recorder)
			snapshots := services.NewTableSnapshotService(w.db, cars, employees, services.NewEmployeesHistoryService(w.db))
			read := func() (*models.TableSnapshot, []struct {
				PassageState services.PassageState     `json:"passage_state"`
				Admission    services.PassageAdmission `json:"admission"`
			}) {
				id, err := snapshots.SnapshotTable(ctx, w.table, models.SnapshotReasonManual, &w.actor)
				require.NoError(t, err)
				snapshot, err := snapshots.GetSnapshot(ctx, w.table, id)
				require.NoError(t, err)
				var payload models.SnapshotPayload
				require.NoError(t, json.Unmarshal(snapshot.Payload, &payload))
				var rows []struct {
					PassageState services.PassageState     `json:"passage_state"`
					Admission    services.PassageAdmission `json:"admission"`
				}
				require.NoError(t, json.Unmarshal(payload.Rows, &rows))
				require.Len(t, rows, 1)
				require.Equal(t, "expired", rows[0].Admission.Reason)
				require.False(t, rows[0].Admission.CanEnter)
				require.False(t, rows[0].Admission.CanExit)
				return snapshot, rows
			}
			require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.id).Update("territory_status", 0).Error)
			snap, rows := read()
			var counts models.SnapshotCounts
			require.NoError(t, json.Unmarshal(snap.Counts, &counts))
			require.Equal(t, 1, counts.OnTerritory)
			require.True(t, rows[0].PassageState.Open)
			commands := services.NewPassageCommandService(w.db, recorder)
			closed, err := commands.Correct(ctx, w.actor, kind, w.id, w.correction())
			require.NoError(t, err)
			snap, rows = read()
			require.NoError(t, json.Unmarshal(snap.Counts, &counts))
			require.Equal(t, 1, counts.Corrected)
			require.Zero(t, counts.Exited)
			require.Zero(t, counts.OnTerritory)
			require.False(t, rows[0].PassageState.CanRevertCorrection)
			exported, _, err := snapshots.BuildSnapshotExport(ctx, w.table, &snap.ID)
			require.NoError(t, err)
			require.Len(t, exported.Rows, 1)
			require.Contains(t, exported.Rows[0], "Учёт исправлен")
			require.NotContains(t, exported.Rows[0], "Выехал")
			revert := w.correction()
			revert.ExpectedLastEventID = &closed.PassageState.LastEventID
			_, err = commands.RevertCorrection(ctx, w.actor, kind, w.id, revert)
			require.NoError(t, err)
			_, err = commands.Mark(ctx, w.actor, kind, w.id, services.PassageCommandRequest{TableID: &w.table, TerritoryStatus: 2})
			require.NoError(t, err)
			require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.id).Update("territory_status", 0).Error)
			snap, rows = read()
			counts = models.SnapshotCounts{}
			require.NoError(t, json.Unmarshal(snap.Counts, &counts))
			require.Equal(t, 1, counts.Exited)
			require.Zero(t, counts.OnTerritory)
			require.True(t, rows[0].PassageState.InExitGrace)
			exported, _, err = snapshots.BuildSnapshotExport(ctx, w.table, &snap.ID)
			require.NoError(t, err)
			require.Contains(t, exported.Rows[0], "Выехал")
			require.NotContains(t, exported.Rows[0], "Учёт исправлен")
		})
	}
}
