package handlers_test

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"net/http"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"testing"
)

func TestSingleManualAttach2665DBCarFlags(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, createNew := range []bool{false, true} {
			for _, bits := range []int{0, 1, 2, 3, 4, 5, 6, 7} {
				if kind == services.ElementEmployee && !createNew && (bits&2 != 0) != (bits&4 != 0) {
					continue
				}
				t.Run(fmt.Sprintf("%s/new=%t/flags=%d", kind, createNew, bits), func(t *testing.T) {
					w := setupManualAttach2665World(t, kind)
					own, source, target := bits&1 != 0, bits&2 != 0, bits&4 != 0
					require.NoError(t, w.db.Table("attachments").Where("id=?", w.orphan).Updates(map[string]any{"roof_access": source, "free_parking": source}).Error)
					require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).Updates(map[string]any{"roof_access": target, "free_parking": target}).Error)
					if kind == services.ElementCar {
						require.NoError(t, w.db.Table("cars").Where("id=?", w.entity).Updates(map[string]any{"individual_roof_access": own, "individual_free_parking": own}).Error)
					}
					orphanBefore := singleAttach2665JSON(t, w.db, "attachments", w.orphan)
					targetBefore := singleAttach2665JSON(t, w.db, "attachments", w.attachment)
					neighborBefore := singleAttach2665JSON(t, w.db, string(kind), w.individual)
					command := singleAttach2665Service(w)
					req := singleAttach2665Request(w, createNew)
					preview, err := command.Preview(context.Background(), w.actor, kind, w.entity, req)
					require.NoError(t, err)
					if kind == services.ElementCar {
						require.NotNil(t, preview.NewFlags)
						require.NotNil(t, preview.CurrentFlags)
						require.Equal(t, own || source, preview.NewFlags.IndividualRoofAccess)
						require.Equal(t, own || source, preview.NewFlags.IndividualFreeParking)
						wantEffective := own || source || (!createNew && target)
						require.Equal(t, wantEffective, preview.NewFlags.RoofAccess)
						require.Equal(t, wantEffective, preview.NewFlags.FreeParking)
					} else {
						require.Nil(t, preview.NewFlags)
						require.Nil(t, preview.CurrentFlags)
					}
					req.ExpectedRevision = preview.Revision
					changed, err := command.Attach(context.Background(), w.actor, kind, w.entity, req)
					require.NoError(t, err)
					require.Equal(t, preview.NewFlags, changed.NewFlags)
					if kind == services.ElementCar {
						var row models.Car
						require.NoError(t, w.db.First(&row, w.entity).Error)
						require.Equal(t, own || source, row.IndividualRoofAccess)
						require.Equal(t, own || source, row.IndividualFreeParking)
						var audit struct{ OldRoof, NewRoof bool }
						require.NoError(t, w.db.Raw(`SELECT (details->'old_flags'->>'individual_roof_access')::boolean AS old_roof,(details->'new_flags'->>'individual_roof_access')::boolean AS new_roof FROM audit_log WHERE entity_type=? AND entity_id=? AND action='update'`, models.AuditEntityCar, w.entity).Scan(&audit).Error)
						require.Equal(t, own, audit.OldRoof)
						require.Equal(t, own || source, audit.NewRoof)
					}
					require.Equal(t, orphanBefore, singleAttach2665JSON(t, w.db, "attachments", w.orphan))
					require.Equal(t, targetBefore, singleAttach2665JSON(t, w.db, "attachments", w.attachment))
					require.Equal(t, neighborBefore, singleAttach2665JSON(t, w.db, string(kind), w.individual))
				})
			}
		}
	}
}

func TestSingleManualAttach2665DBEmployeeFlagsMismatchIsAtomic(t *testing.T) {
	for _, field := range []string{"roof_access", "free_parking"} {
		for _, source := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/source=%t", field, source), func(t *testing.T) {
				w := setupManualAttach2665World(t, services.ElementEmployee)
				command := singleAttach2665Service(w)
				req := singleAttach2665Request(w, false)
				preview, err := command.Preview(context.Background(), w.actor, services.ElementEmployee, w.entity, req)
				require.NoError(t, err)
				req.ExpectedRevision = preview.Revision
				require.NoError(t, w.db.Table("attachments").Where("id=?", w.orphan).UpdateColumn(field, source).Error)
				require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).UpdateColumn(field, !source).Error)
				before := w.snapshot(t)
				_, err = command.Preview(context.Background(), w.actor, services.ElementEmployee, w.entity, req)
				period2665RequireHTTPError(t, err, http.StatusConflict)
				_, err = command.Attach(context.Background(), w.actor, services.ElementEmployee, w.entity, req)
				period2665RequireHTTPError(t, err, http.StatusConflict)
				require.Equal(t, before, w.snapshot(t), "employee, sibling, orphan, target, votes and audit must remain unchanged")
			})
		}
	}
}

func TestSingleManualAttach2665DBOwnFlagsStaleWithoutTimestampChange(t *testing.T) {
	w := setupManualAttach2665World(t, services.ElementCar)
	command := singleAttach2665Service(w)
	req := singleAttach2665Request(w, false)
	preview, err := command.Preview(context.Background(), w.actor, services.ElementCar, w.entity, req)
	require.NoError(t, err)
	req.ExpectedRevision = preview.Revision
	require.NoError(t, w.db.Table("cars").Where("id=?", w.entity).UpdateColumn("individual_roof_access", true).Error)
	before := w.snapshot(t)
	_, err = command.Attach(context.Background(), w.actor, services.ElementCar, w.entity, req)
	period2665RequireHTTPError(t, err, http.StatusConflict)
	require.Equal(t, before, w.snapshot(t))
}
