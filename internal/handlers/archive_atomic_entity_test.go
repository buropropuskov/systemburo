package handlers_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Fail only the outbox write, after the mutation and required audit have run.
func failArchiveEntityIntent(t *testing.T, db *gorm.DB) *int {
	t.Helper()
	calls := 0
	const callback = "archive_atomic_entity_intent_failure"
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(callback, func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "INSERT INTO archive_export_jobs") {
			calls++
			if calls == 1 {
				tx.AddError(errors.New("synthetic archive intent failure"))
			}
		}
	}))
	t.Cleanup(func() { require.NoError(t, db.Callback().Raw().Remove(callback)) })
	return &calls
}

func requireArchiveEntityAuditCount(t *testing.T, w period2665World, id int, want int64) {
	t.Helper()
	entityType := models.AuditEntityEmployee
	if w.kind == services.ElementCar {
		entityType = models.AuditEntityCar
	}
	var count int64
	require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=?", entityType, id).Count(&count).Error)
	require.Equal(t, want, count)
}

func TestArchiveAtomicEntityPeriodIntentRollback(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPeriod2665World(t, kind)
			p := wirePeriodPublishers2665(t, w)
			preview, err := w.commands.Inspect(context.Background(), w.actor, kind, w.target, nil)
			require.NoError(t, err)
			before, parent := w.rowSnapshot(t, w.target), w.parentAndVotesSnapshot(t)
			calls := failArchiveEntityIntent(t, w.db)
			_, err = w.commands.Change(context.Background(), w.actor, kind, w.target, w.request(preview.Revision, 7))
			require.Error(t, err)
			require.Equal(t, 1, *calls)
			require.Equal(t, before, w.rowSnapshot(t, w.target))
			require.Equal(t, parent, w.parentAndVotesSnapshot(t))
			requireArchiveEntityAuditCount(t, w, w.target, 0)
			p.requireSilent(t)
		})
	}
}

func TestArchiveAtomicManualAttachIntentRollback(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, single := range []bool{false, true} {
			name := string(kind) + "/bulk"
			if single {
				name = string(kind) + "/single"
			}
			t.Run(name, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				p := wireManualAttachPublishers2665(t, &w)
				before, orphan := w.rowSnapshot(t, w.entity), singleAttach2665JSON(t, w.db, "attachments", w.orphan)
				var run func() error
				if single {
					command := services.NewSingleManualAttachCommandService(w.db, services.NewAuditRecorder(w.db), nil, nil)
					require.NoError(t, services.ConfigureSingleManualAttachUpdates(command, p.applications))
					request := singleAttach2665Request(w, false)
					preview, err := command.Preview(context.Background(), w.actor, kind, w.entity, request)
					require.NoError(t, err)
					request.ExpectedRevision = preview.Revision
					run = func() error {
						_, err := command.Attach(context.Background(), w.actor, kind, w.entity, request)
						return err
					}
				} else {
					run = func() error {
						_, err := w.service.AttachToApplication(context.Background(), w.orphan, services.AttachToApplicationRequest{TargetAttachmentID: &w.attachment, PeriodChoice: "source", SourceAttachmentID: &w.attachment}, w.actor)
						return err
					}
				}
				calls := failArchiveEntityIntent(t, w.db)
				require.Error(t, run())
				require.Equal(t, 1, *calls)
				require.Equal(t, before, w.rowSnapshot(t, w.entity))
				require.Equal(t, orphan, singleAttach2665JSON(t, w.db, "attachments", w.orphan))
				requireArchiveEntityAuditCount(t, w.period2665World, w.entity, 0)
				p.requireSilent(t)
			})
		}
	}
}

func TestArchiveAtomicEntityBulkPartialSuccess(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, operation := range []string{"add", "move", "unbind"} {
			t.Run(string(kind)+"/"+operation, func(t *testing.T) {
				w := setupPeriod2665World(t, kind)
				tableType := models.TableTypePeople
				if kind == services.ElementCar {
					tableType = models.TableTypeCars
				}
				from := models.SystemTable{Name: "archive_atomic_from", TableType: tableType, IsActive: true}
				to := models.SystemTable{Name: "archive_atomic_to", TableType: tableType, IsActive: true}
				require.NoError(t, w.db.Create(&from).Error)
				require.NoError(t, w.db.Create(&to).Error)
				binding, column := "employee_target_tables", "employee_id"
				if kind == services.ElementCar {
					binding, column = "car_target_tables", "car_id"
				}
				if operation != "add" {
					for _, id := range []int{w.target, w.neighbor} {
						require.NoError(t, w.db.Exec("INSERT INTO "+binding+" ("+column+",table_id) VALUES (?,?)", id, from.ID).Error)
					}
				}
				before := w.rowSnapshot(t, w.target)
				calls := failArchiveEntityIntent(t, w.db)
				ids := []int{w.target, w.neighbor}
				var result *services.BulkOpResult
				var err error
				if kind == services.ElementCar {
					svc := services.NewCarService(w.db, services.NewAuditRecorder(w.db))
					switch operation {
					case "add":
						result, err = svc.BulkAddTable(context.Background(), services.BulkAddCarsTableRequest{IDs: ids, TableIDs: []int{to.ID}}, w.actor)
					case "move":
						result, err = svc.BulkMoveTable(context.Background(), services.BulkMoveCarsTableRequest{IDs: ids, FromTableID: from.ID, ToTableIDs: []int{to.ID}}, w.actor)
					case "unbind":
						result, err = svc.BulkUnbindTable(context.Background(), services.BulkUnbindCarsTableRequest{IDs: ids, TableID: from.ID}, w.actor)
					}
				} else {
					svc := services.NewEmployeeService(w.db, services.NewAuditRecorder(w.db))
					switch operation {
					case "add":
						result, err = svc.BulkAddTable(context.Background(), services.EmployeeBulkAddTableRequest{IDs: ids, TableIDs: []int{to.ID}}, w.actor)
					case "move":
						result, err = svc.BulkMoveTable(context.Background(), services.EmployeeBulkMoveTableRequest{IDs: ids, FromTableID: from.ID, ToTableIDs: []int{to.ID}}, w.actor)
					case "unbind":
						result, err = svc.BulkUnbindTable(context.Background(), services.EmployeeBulkUnbindTableRequest{IDs: ids, TableID: from.ID}, w.actor)
					}
				}
				require.NoError(t, err)
				require.Equal(t, 1, result.SuccessCount)
				require.Equal(t, 1, result.ErrorCount)
				require.Equal(t, 2, *calls)
				require.Equal(t, before, w.rowSnapshot(t, w.target))
				requireArchiveEntityAuditCount(t, w, w.target, 0)
				var links int64
				require.NoError(t, w.db.Table(binding).Where(column+"=?", w.target).Count(&links).Error)
				if operation == "add" {
					require.Zero(t, links)
				} else {
					require.EqualValues(t, 1, links)
				}
				require.NoError(t, w.db.Table(binding).Where(column+"=?", w.neighbor).Count(&links).Error)
				if operation == "unbind" {
					require.Zero(t, links)
				} else {
					require.EqualValues(t, 1, links)
				}
				var jobs int64
				require.NoError(t, w.db.Table("archive_export_jobs").Where("application_id=?", w.application).Count(&jobs).Error)
				require.EqualValues(t, 1, jobs)
			})
		}
	}
}

func TestArchiveAtomicAttachmentPeriodIntentRollback(t *testing.T) {
	w := setupAttachmentPeriod2665World(t, services.ElementEmployee)
	p := wirePeriodPublishers2665(t, w.period2665World)
	require.NoError(t, services.ConfigureAttachmentPeriodUpdates(w.service, p.applications))
	request := w.request([]int{w.attachment, w.secondAttachment}, services.IndividualPolicyReplace)
	preview, err := w.service.Preview(context.Background(), w.actor, w.application, request)
	require.NoError(t, err)
	request.ExpectedRevision = preview.Revision
	before := w.snapshot(t)
	calls := failArchiveEntityIntent(t, w.db)
	_, err = w.service.Change(context.Background(), w.actor, w.application, request)
	require.Error(t, err)
	require.Equal(t, 1, *calls)
	require.Equal(t, before, w.snapshot(t))
	var audits int64
	require.NoError(t, w.db.Model(&models.AuditLog{}).Count(&audits).Error)
	require.Zero(t, audits)
	p.requireSilent(t)
}

func TestArchiveAtomicEntityBulkAddNoOp(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPeriod2665World(t, kind)
			tableType := models.TableTypePeople
			if kind == services.ElementCar {
				tableType = models.TableTypeCars
			}
			table := models.SystemTable{Name: "archive_atomic_noop", TableType: tableType, IsActive: true}
			require.NoError(t, w.db.Create(&table).Error)
			if kind == services.ElementCar {
				require.NoError(t, w.db.Create(&models.CarTargetTable{CarID: w.target, TableID: table.ID}).Error)
			} else {
				require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: w.target, TableID: table.ID}).Error)
			}
			calls := failArchiveEntityIntent(t, w.db)
			var result *services.BulkOpResult
			var err error
			if kind == services.ElementCar {
				result, err = services.NewCarService(w.db, services.NewAuditRecorder(w.db)).BulkAddTable(context.Background(), services.BulkAddCarsTableRequest{IDs: []int{w.target}, TableIDs: []int{table.ID}}, w.actor)
			} else {
				result, err = services.NewEmployeeService(w.db, services.NewAuditRecorder(w.db)).BulkAddTable(context.Background(), services.EmployeeBulkAddTableRequest{IDs: []int{w.target}, TableIDs: []int{table.ID}}, w.actor)
			}
			require.NoError(t, err)
			require.Equal(t, 1, result.SuccessCount)
			require.Zero(t, result.ErrorCount)
			require.Zero(t, *calls)
			requireArchiveEntityAuditCount(t, w, w.target, 0)
		})
	}
}
