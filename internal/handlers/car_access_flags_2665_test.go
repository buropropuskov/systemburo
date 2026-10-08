package handlers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

func TestCarAccess2665DBReadersKeepOwnAndParentSeparate(t *testing.T) {
	for bits := 0; bits < 16; bits++ {
		t.Run(fmt.Sprint(bits), func(t *testing.T) {
			w := setupPeriod2665World(t, services.ElementCar)
			ownRoof, ownParking, parentRoof, parentParking := bits&1 != 0, bits&2 != 0, bits&4 != 0, bits&8 != 0
			require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).Updates(map[string]any{"roof_access": parentRoof, "free_parking": parentParking}).Error)
			require.NoError(t, w.db.Table("cars").Where("id=?", w.target).Updates(map[string]any{"individual_roof_access": ownRoof, "individual_free_parking": ownParking}).Error)
			table := models.SystemTable{Name: "flags2665_reader", TableType: "cars", IsActive: true}
			require.NoError(t, w.db.Create(&table).Error)
			for _, id := range []int{w.target, w.neighbor} {
				require.NoError(t, w.db.Create(&models.CarTargetTable{CarID: id, TableID: table.ID}).Error)
			}
			registryIDs := periodRegistry2665Identities(t, w)
			assert := func(id int, flags services.CarAccessFlags) {
				wantOwnRoof, wantOwnParking := false, false
				if id == w.target {
					wantOwnRoof, wantOwnParking = ownRoof, ownParking
				}
				require.Equal(t, services.CarAccessFlags{IndividualRoofAccess: wantOwnRoof, IndividualFreeParking: wantOwnParking, RoofAccess: wantOwnRoof || parentRoof, FreeParking: wantOwnParking || parentParking}, flags)
			}
			ctx := context.Background()
			tableRows, err := services.NewCarService(w.db, services.NewAuditRecorder(w.db)).GetActiveCarsForTable(ctx, table.ID)
			require.NoError(t, err)
			require.Len(t, tableRows, 2)
			for _, row := range tableRows {
				assert(row.ID, row.CarAccessFlags)
			}
			apps := services.NewApplicationService(w.db, nil, nil, nil, nil, services.NewAuditRecorder(w.db))
			details, err := apps.GetAttachmentCars(ctx, w.attachment, services.SupplementScopeAll)
			require.NoError(t, err)
			require.Len(t, details, 2)
			for _, row := range details {
				assert(row.ID, row.CarAccessFlags)
			}
			registry := services.NewUniqueCarService(w.db)
			rows, err := registry.GetAll(ctx, "period2665_actor", "")
			require.NoError(t, err)
			require.Len(t, rows, 2)
			for _, row := range rows {
				require.NotNil(t, row.ActiveCarID)
				assert(*row.ActiveCarID, row.CarAccessFlags)
				require.Contains(t, registryIDs, row.ID)
			}
			paged, total, err := registry.GetAllPaginated(ctx, "period2665_actor", "", "", 1, 20)
			require.NoError(t, err)
			require.EqualValues(t, 2, total)
			for _, row := range paged {
				require.NotNil(t, row.ActiveCarID)
				assert(*row.ActiveCarID, row.CarAccessFlags)
			}
			attachments, err := apps.GetApplicationAttachments(ctx, w.application, 0)
			require.NoError(t, err)
			for _, attachment := range attachments {
				if attachment.ID == w.attachment {
					require.Equal(t, parentRoof, attachment.RoofAccess)
					require.Equal(t, parentParking, attachment.FreeParking)
				}
			}
		})
	}
}

func TestCarAccess2665DBSnapshotTracksIndividualFlags(t *testing.T) {
	w := seedPeriodBlank2665(t, setupArchiveWorld(t), services.ElementCar)
	first := w.reexport(t, w.app)
	beforeHash := w.registryRow(t, w.app, 0).ContentHash
	w.snapshotBytes(t, first, w.own)
	require.NoError(t, w.db.Table("cars").Where("id=?", w.entity).UpdateColumn("individual_roof_access", true).Error)
	changed := w.reexport(t, w.app)
	require.True(t, changed.Snapshot.Written)
	require.NotEqual(t, beforeHash, w.registryRow(t, w.app, 0).ContentHash)
	data := w.snapshotBytes(t, changed, w.own)
	var snapshot struct {
		Attachments []struct {
			RoofAccess bool                      `json:"roof_access"`
			Cars       []services.CarAccessFlags `json:"cars"`
		} `json:"attachments"`
	}
	require.NoError(t, json.Unmarshal(data, &snapshot))
	require.Len(t, snapshot.Attachments, 1)
	require.False(t, snapshot.Attachments[0].RoofAccess)
	require.Len(t, snapshot.Attachments[0].Cars, 1)
	require.True(t, snapshot.Attachments[0].Cars[0].IndividualRoofAccess)
	require.True(t, snapshot.Attachments[0].Cars[0].RoofAccess)
	unchanged := w.reexport(t, w.app)
	require.False(t, unchanged.Snapshot.Written)
	require.Equal(t, data, w.snapshotBytes(t, unchanged, w.own))
}
