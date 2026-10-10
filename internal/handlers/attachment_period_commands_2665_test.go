package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type attachmentPeriod2665World struct {
	period2665World
	service          *services.AttachmentPeriodCommandService
	secondAttachment int
	secondKind       services.ElementKind
	secondTarget     int
	secondNeighbor   int
}

func setupAttachmentPeriod2665World(t *testing.T, kind services.ElementKind) attachmentPeriod2665World {
	t.Helper()
	w := setupPeriod2665World(t, kind)
	require.NoError(t, w.db.Table("applications").Where("id=?", w.application).Update("status", models.StatusProcessing).Error)
	// A managed per-user grant, not an automatic receiver/role assignment.
	require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: w.actor, PermissionKey: services.KeyApplicationPeriodChange, Value: "allow", GrantedAt: w.clock.UTC()}).Error)
	var parent models.Attachment
	require.NoError(t, w.db.First(&parent, w.attachment).Error)
	individual := map[string]any{"period_mode": models.PeriodIndividual, "entry_date_from": parent.EntryDateFrom, "entry_date_to": parent.EntryDateTo, "entry_time_from": parent.EntryTimeFrom, "entry_time_to": parent.EntryTimeTo}
	require.NoError(t, w.db.Table(string(kind)).Where("id=?", w.neighbor).Updates(individual).Error)
	otherKind, otherType := services.ElementCar, "cars"
	if kind == services.ElementCar {
		otherKind, otherType = services.ElementEmployee, "people"
	}
	active := 1
	second := models.Attachment{ApplicationID: &w.application, AttachmentType: otherType, Status: &active, EntryDateFrom: parent.EntryDateFrom, EntryDateTo: parent.EntryDateTo, EntryTimeFrom: parent.EntryTimeFrom, EntryTimeTo: parent.EntryTimeTo}
	require.NoError(t, w.db.Create(&second).Error)
	out := attachmentPeriod2665World{period2665World: w, service: services.NewAttachmentPeriodCommandService(w.db, services.NewAuditRecorder(w.db)), secondAttachment: second.ID, secondKind: otherKind}
	for i, mode := range []models.PeriodMode{models.PeriodInherit, models.PeriodIndividual} {
		var id int
		if otherKind == services.ElementCar {
			plate := []string{"GROUP2665-A", "GROUP2665-B"}[i]
			car := models.Car{AttachmentID: second.ID, CarNumber: &plate, Status: &active, PeriodMode: mode, EntryDateFrom: parent.EntryDateFrom, EntryDateTo: parent.EntryDateTo, EntryTimeFrom: parent.EntryTimeFrom, EntryTimeTo: parent.EntryTimeTo}
			require.NoError(t, w.db.Create(&car).Error)
			id = car.ID
		} else {
			employee := models.Employee{AttachmentID: &second.ID, Status: &active, PeriodMode: mode}
			if mode == models.PeriodIndividual {
				employee.EntryDateFrom, employee.EntryDateTo, employee.EntryTimeFrom, employee.EntryTimeTo = parent.EntryDateFrom, parent.EntryDateTo, parent.EntryTimeFrom, parent.EntryTimeTo
			}
			require.NoError(t, w.db.Create(&employee).Error)
			id = employee.ID
		}
		if i == 0 {
			out.secondTarget = id
		} else {
			out.secondNeighbor = id
		}
	}
	return out
}

func (w attachmentPeriod2665World) request(ids []int, policy string) services.ChangeAttachmentPeriodRequest {
	input := w.period2665World.request("", 7)
	return services.ChangeAttachmentPeriodRequest{AttachmentIDs: ids, Period: input.Period, IndividualPolicy: policy, Reason: "Исправление выбранных вложений"}
}

func (w attachmentPeriod2665World) snapshot(t *testing.T) string {
	t.Helper()
	var snapshot string
	require.NoError(t, w.db.Raw(`SELECT jsonb_build_object(
		'application', (SELECT to_jsonb(a) FROM applications a WHERE a.id=?),
		'attachments', (SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM attachments a WHERE a.application_id=?),
		'cars', (SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM cars e JOIN attachments a ON a.id=e.attachment_id WHERE a.application_id=?),
		'employees', (SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM employees e JOIN attachments a ON a.id=e.attachment_id WHERE a.application_id=?),
		'votes', (SELECT jsonb_agg(to_jsonb(v) ORDER BY v.id) FROM application_responsible_users v WHERE v.application_id=?)
	)::text`, w.application, w.application, w.application, w.application, w.application).Scan(&snapshot).Error)
	return snapshot
}

func (w attachmentPeriod2665World) votesAndApplication(t *testing.T) string {
	t.Helper()
	var snapshot string
	require.NoError(t, w.db.Raw(`SELECT jsonb_build_object(
		'application', (SELECT to_jsonb(a) FROM applications a WHERE a.id=?),
		'votes', (SELECT jsonb_agg(to_jsonb(v) ORDER BY v.id) FROM application_responsible_users v WHERE v.application_id=?)
	)::text`, w.application, w.application).Scan(&snapshot).Error)
	return snapshot
}

func (w attachmentPeriod2665World) entitySnapshot(t *testing.T, kind services.ElementKind, id int) string {
	t.Helper()
	var snapshot string
	require.NoError(t, w.db.Raw("SELECT to_jsonb(e)::text FROM "+string(kind)+" e WHERE e.id=?", id).Scan(&snapshot).Error)
	return snapshot
}

func (w attachmentPeriod2665World) auditCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	require.NoError(t, w.db.Model(&models.AuditLog{}).Count(&count).Error)
	return count
}

func TestAttachmentPeriod2665DBSelectionPoliciesAndRoundtrip(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, policy := range []string{services.IndividualPolicyPreserve, services.IndividualPolicyReplace} {
			for _, selection := range []string{"one", "two"} {
				t.Run(string(kind)+"/"+policy+"/"+selection, func(t *testing.T) {
					w := setupAttachmentPeriod2665World(t, kind)
					ctx := context.Background()
					ids := []int{w.attachment}
					if selection == "two" {
						ids = []int{w.secondAttachment, w.attachment}
					}
					req := w.request(ids, policy)
					before := w.snapshot(t)
					auditsBefore := w.auditCount(t)
					votesBefore := w.votesAndApplication(t)
					individualBefore := w.entitySnapshot(t, kind, w.neighbor)
					otherInheritedBefore := w.entitySnapshot(t, w.secondKind, w.secondTarget)
					otherIndividualBefore := w.entitySnapshot(t, w.secondKind, w.secondNeighbor)
					preview, err := w.service.Preview(ctx, w.actor, w.application, req)
					require.NoError(t, err)
					require.Equal(t, before, w.snapshot(t), "preview is read-only")
					require.Equal(t, auditsBefore, w.auditCount(t), "preview must not append audit events")
					require.False(t, preview.ApprovalsReset)
					require.Equal(t, len(ids), preview.AttachmentCount)
					require.Equal(t, len(ids), preview.IndividualCount, "equal-to-parent explicit individual must remain classified as individual")
					require.ElementsMatch(t, ids, preview.AttachmentIDs)
					require.Len(t, preview.Attachments, len(ids))
					wantEmployees, wantCars := 0, 0
					if kind == services.ElementEmployee || selection == "two" {
						wantEmployees = 2
					}
					if kind == services.ElementCar || selection == "two" {
						wantCars = 2
					}
					require.Equal(t, wantEmployees, preview.EmployeeCount)
					require.Equal(t, wantCars, preview.CarCount)
					for _, item := range preview.Attachments {
						entityKind, inheritedID, individualID := kind, w.target, w.neighbor
						if item.AttachmentID == w.secondAttachment {
							entityKind, inheritedID, individualID = w.secondKind, w.secondTarget, w.secondNeighbor
						} else {
							require.Equal(t, w.attachment, item.AttachmentID)
						}
						if entityKind == services.ElementCar {
							require.ElementsMatch(t, []int{inheritedID, individualID}, item.CarIDs)
							require.Equal(t, []int{individualID}, item.IndividualCarIDs)
							require.Empty(t, item.EmployeeIDs)
						} else {
							require.ElementsMatch(t, []int{inheritedID, individualID}, item.EmployeeIDs)
							require.Equal(t, []int{individualID}, item.IndividualEmployeeIDs)
							require.Empty(t, item.CarIDs)
						}
					}
					req.ExpectedRevision = preview.Revision
					changed, err := w.service.Change(ctx, w.actor, w.application, req)
					require.NoError(t, err)
					require.False(t, changed.ApprovalsReset)
					require.ElementsMatch(t, ids, changed.ChangedAttachmentIDs)
					require.Equal(t, votesBefore, w.votesAndApplication(t), "application and full vote rows must remain unchanged")
					var selected []struct {
						ID          int
						EntryDateTo string
					}
					require.NoError(t, w.db.Table("attachments").Select("id,entry_date_to").Where("id IN ?", ids).Scan(&selected).Error)
					require.Len(t, selected, len(ids))
					for _, row := range selected {
						require.Equal(t, req.Period.EntryDateTo, row.EntryDateTo)
					}
					for _, entity := range []struct {
						kind       services.ElementKind
						id         int
						chosen     bool
						individual bool
					}{
						{kind, w.target, true, false}, {kind, w.neighbor, true, true},
						{w.secondKind, w.secondTarget, selection == "two", false}, {w.secondKind, w.secondNeighbor, selection == "two", true},
					} {
						var stored struct {
							PeriodMode  models.PeriodMode
							EntryDateTo *string
						}
						require.NoError(t, w.db.Table(string(entity.kind)).Select("period_mode,entry_date_to").Where("id=?", entity.id).Scan(&stored).Error)
						if entity.individual {
							require.Equal(t, models.PeriodIndividual, stored.PeriodMode)
							if entity.chosen && policy == services.IndividualPolicyReplace {
								require.NotNil(t, stored.EntryDateTo)
								require.Equal(t, req.Period.EntryDateTo, *stored.EntryDateTo)
							}
						} else {
							require.Equal(t, models.PeriodInherit, stored.PeriodMode)
							if entity.chosen && entity.kind == services.ElementCar {
								require.NotNil(t, stored.EntryDateTo)
								require.Equal(t, req.Period.EntryDateTo, *stored.EntryDateTo)
							}
							if entity.kind == services.ElementEmployee {
								require.Nil(t, stored.EntryDateTo, "inherited Employee own period must remain NULL")
							}
						}
						changedIDs := changed.ChangedEmployeeIDs
						if entity.kind == services.ElementCar {
							changedIDs = changed.ChangedCarIDs
						}
						if entity.chosen && (!entity.individual || policy == services.IndividualPolicyReplace) {
							require.Contains(t, changedIDs, entity.id)
						} else {
							require.NotContains(t, changedIDs, entity.id)
						}
						var effective struct{ EntryDateTo *string }
						require.NoError(t, w.db.Raw("SELECT CASE WHEN e.period_mode='individual' THEN e.entry_date_to ELSE a.entry_date_to END AS entry_date_to FROM "+string(entity.kind)+" e JOIN attachments a ON a.id=e.attachment_id WHERE e.id=?", entity.id).Scan(&effective).Error)
						wantEnd := w.clock.AddDate(0, 0, 2).Format("2006-01-02")
						if entity.chosen && (!entity.individual || policy == services.IndividualPolicyReplace) {
							wantEnd = req.Period.EntryDateTo
						}
						require.NotNil(t, effective.EntryDateTo)
						require.Equal(t, wantEnd, *effective.EntryDateTo)
					}
					if policy == services.IndividualPolicyPreserve {
						require.Equal(t, individualBefore, w.entitySnapshot(t, kind, w.neighbor))
						require.Equal(t, otherIndividualBefore, w.entitySnapshot(t, w.secondKind, w.secondNeighbor), "preserve must retain complete individual rows for both kinds")
					}
					if selection == "one" {
						require.Equal(t, otherInheritedBefore, w.entitySnapshot(t, w.secondKind, w.secondTarget))
						require.Equal(t, otherIndividualBefore, w.entitySnapshot(t, w.secondKind, w.secondNeighbor))
						var unselectedEnd string
						require.NoError(t, w.db.Table("attachments").Select("entry_date_to").Where("id=?", w.secondAttachment).Scan(&unselectedEnd).Error)
						require.Equal(t, w.clock.AddDate(0, 0, 2).Format("2006-01-02"), unselectedEnd)
					}
					latest, err := w.service.Preview(ctx, w.actor, w.application, req)
					require.NoError(t, err)
					require.Equal(t, changed.Revision, latest.Revision, "returned revision must survive the real PostgreSQL roundtrip for the same payload")
					require.Equal(t, auditsBefore+int64(1+len(changed.ChangedEmployeeIDs)+len(changed.ChangedCarIDs)), w.auditCount(t), "each affected row and the application have atomic audit events; no event for preserved/unselected rows")
				})
			}
		}
	}
}

func TestAttachmentPeriod2665DBForeignSelectionAtomic(t *testing.T) {
	w := setupAttachmentPeriod2665World(t, services.ElementCar)
	ctx := context.Background()
	var original models.Application
	require.NoError(t, w.db.First(&original, w.application).Error)
	status, active := models.StatusProcessing, 1
	foreign := models.Application{OrganizationID: original.OrganizationID, SenderUserID: w.actor, Status: &status}
	require.NoError(t, w.db.Create(&foreign).Error)
	foreignAtt := models.Attachment{ApplicationID: &foreign.ID, AttachmentType: "items", Status: &active}
	require.NoError(t, w.db.Create(&foreignAtt).Error)
	req := w.request([]int{w.attachment}, services.IndividualPolicyReplace)
	preview, err := w.service.Preview(ctx, w.actor, w.application, req)
	require.NoError(t, err)
	req.AttachmentIDs = []int{w.attachment, foreignAtt.ID}
	req.ExpectedRevision = preview.Revision
	before, audits := w.snapshot(t), w.auditCount(t)
	_, err = w.service.Preview(ctx, w.actor, w.application, req)
	period2665RequireHTTPError(t, err, http.StatusForbidden)
	_, err = w.service.Change(ctx, w.actor, w.application, req)
	period2665RequireHTTPError(t, err, http.StatusForbidden)
	require.Equal(t, before, w.snapshot(t))
	require.Equal(t, audits, w.auditCount(t))
	var actual models.Attachment
	require.NoError(t, w.db.First(&actual, foreignAtt.ID).Error)
	require.Nil(t, actual.EntryDateTo)
}

func TestAttachmentPeriod2665DBStaleAndActorBoundRevision(t *testing.T) {
	for _, scenario := range []string{"parent", "composition", "actor", "lifecycle"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupAttachmentPeriod2665World(t, services.ElementEmployee)
			ctx := context.Background()
			req := w.request([]int{w.attachment, w.secondAttachment}, services.IndividualPolicyReplace)
			preview, err := w.service.Preview(ctx, w.actor, w.application, req)
			require.NoError(t, err)
			req.ExpectedRevision = preview.Revision
			actorID := w.actor
			switch scenario {
			case "parent":
				require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).UpdateColumn("entry_time_to", "23:00:00").Error)
			case "composition":
				active := 1
				require.NoError(t, w.db.Create(&models.Employee{AttachmentID: &w.attachment, Status: &active}).Error)
			case "actor":
				other := models.User{Username: "attachment2665_other", Password: "x", TypeID: 1, IsActive: true}
				require.NoError(t, w.db.Create(&other).Error)
				require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: other.ID, PermissionKey: services.KeyApplicationPeriodChange, Value: "allow", GrantedAt: w.clock.UTC()}).Error)
				require.NoError(t, w.db.Create(&models.ApplicationViewer{ApplicationID: w.application, UserID: other.ID}).Error)
				actorID = other.ID
				ownPreview, err := w.service.Preview(ctx, actorID, w.application, req)
				require.NoError(t, err, "second actor must have actual permission and visibility before testing the bound token")
				require.NotEqual(t, preview.Revision, ownPreview.Revision)
			case "lifecycle":
				require.NoError(t, w.db.Table("applications").Where("id=?", w.application).UpdateColumn("status", models.StatusInWork).Error)
			}
			before, audits := w.snapshot(t), w.auditCount(t)
			_, err = w.service.Change(ctx, actorID, w.application, req)
			period2665RequireHTTPError(t, err, http.StatusConflict)
			require.Equal(t, before, w.snapshot(t))
			require.Equal(t, audits, w.auditCount(t))
		})
	}
}

func TestAttachmentPeriod2665DBPermissionAccountAndWorkGates(t *testing.T) {
	for _, scenario := range []string{"personal_deny_admin", "banned_super", "archived_super", "invisible_actor", "completed"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupAttachmentPeriod2665World(t, services.ElementCar)
			ctx := context.Background()
			req := w.request([]int{w.attachment}, "")
			preview, err := w.service.Preview(ctx, w.actor, w.application, req)
			require.NoError(t, err)
			require.Equal(t, services.IndividualPolicyPreserve, preview.IndividualPolicy)
			req.ExpectedRevision = preview.Revision
			actorID := w.actor
			switch scenario {
			case "personal_deny_admin":
				require.NoError(t, w.db.Table("users").Where("id=?", w.actor).Update("is_admin", true).Error)
				require.NoError(t, w.db.Model(&models.UserPermissionOverride{}).Where("user_id=? AND permission_key=?", w.actor, services.KeyApplicationPeriodChange).Update("value", "deny").Error)
			case "banned_super":
				require.NoError(t, w.db.Table("users").Where("id=?", w.actor).Updates(map[string]any{"is_super_admin": true, "is_banned": true}).Error)
			case "archived_super":
				require.NoError(t, w.db.Table("users").Where("id=?", w.actor).Updates(map[string]any{"is_super_admin": true, "is_active": false}).Error)
			case "invisible_actor":
				other := models.User{Username: "attachment2665_invisible", Password: "x", TypeID: 1, IsActive: true}
				require.NoError(t, w.db.Create(&other).Error)
				require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: other.ID, PermissionKey: services.KeyApplicationPeriodChange, Value: "allow", GrantedAt: w.clock.UTC()}).Error)
				actorID = other.ID
			case "completed":
				require.NoError(t, w.db.Table("applications").Where("id=?", w.application).Update("status", models.StatusCompleted).Error)
			}
			before, audits := w.snapshot(t), w.auditCount(t)
			_, err = w.service.Preview(ctx, actorID, w.application, req)
			want := http.StatusForbidden
			if scenario == "completed" {
				want = http.StatusBadRequest
			}
			period2665RequireHTTPError(t, err, want)
			_, err = w.service.Change(ctx, actorID, w.application, req)
			if scenario == "completed" {
				want = http.StatusConflict
			}
			period2665RequireHTTPError(t, err, want)
			require.Equal(t, before, w.snapshot(t))
			require.Equal(t, audits, w.auditCount(t))
		})
	}
}

func TestAttachmentPeriod2665DBAuditFailureRollsBackEverything(t *testing.T) {
	w := setupAttachmentPeriod2665World(t, services.ElementEmployee)
	ctx := context.Background()
	req := w.request([]int{w.attachment, w.secondAttachment}, services.IndividualPolicyReplace)
	preview, err := w.service.Preview(ctx, w.actor, w.application, req)
	require.NoError(t, err)
	req.ExpectedRevision = preview.Revision
	before, audits := w.snapshot(t), w.auditCount(t)
	failure := errors.New("synthetic selected attachment audit storage failure")
	const callback = "attachment2665_fail_application_audit"
	seenWrites := false
	require.NoError(t, w.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		entry, ok := tx.Statement.Dest.(*models.AuditLog)
		if !ok || entry.EntityType != models.AuditEntityApplication || entry.EntityID == nil || *entry.EntityID != w.application || entry.ActorUserID == nil || *entry.ActorUserID != w.actor || entry.Action != models.AuditActionDatesChanged {
			return
		}
		var parentEnd, employeeEnd, carEnd string
		read := tx.Session(&gorm.Session{NewDB: true})
		if err := read.Table("attachments").Select("entry_date_to").Where("id=?", w.attachment).Scan(&parentEnd).Error; err != nil {
			tx.AddError(err)
			return
		}
		if err := tx.Session(&gorm.Session{NewDB: true}).Table("employees").Select("entry_date_to").Where("id=?", w.neighbor).Scan(&employeeEnd).Error; err != nil {
			tx.AddError(err)
			return
		}
		if err := tx.Session(&gorm.Session{NewDB: true}).Table("cars").Select("entry_date_to").Where("id=?", w.secondNeighbor).Scan(&carEnd).Error; err != nil {
			tx.AddError(err)
			return
		}
		seenWrites = parentEnd == req.Period.EntryDateTo && employeeEnd == req.Period.EntryDateTo && carEnd == req.Period.EntryDateTo
		tx.AddError(failure)
	}))
	t.Cleanup(func() { require.NoError(t, w.db.Callback().Create().Remove(callback)) })
	_, err = w.service.Change(ctx, w.actor, w.application, req)
	require.ErrorIs(t, err, failure)
	require.True(t, seenWrites, "inject failure only after actual parent/individual Employee/Car writes")
	require.Equal(t, before, w.snapshot(t), "parents, all entities, modes, timestamps, application and complete votes must roll back")
	require.Equal(t, audits, w.auditCount(t), "earlier entity audits must roll back with the final application audit failure")
}

func TestAttachmentPeriod2665DBNegativeConfirmationRejectsPreviewAndStalesChange(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupAttachmentPeriod2665World(t, kind)
			ctx := context.Background()
			req := w.request([]int{w.attachment, w.secondAttachment}, services.IndividualPolicyReplace)
			approvedPreview, err := w.service.Preview(ctx, w.actor, w.application, req)
			require.NoError(t, err)
			req.ExpectedRevision = approvedPreview.Revision
			// Keep Processing and change only the outcome, without relying on a
			// timestamp bump to invalidate the previously approved preview.
			require.NoError(t, w.db.Table("applications").Where("id=?", w.application).UpdateColumn("confirmation", models.ConfirmationRejected).Error)
			var state struct {
				Status       string
				Confirmation string
			}
			require.NoError(t, w.db.Table("applications").Select("status,confirmation").Where("id=?", w.application).Scan(&state).Error)
			require.Equal(t, models.StatusProcessing, state.Status)
			require.Equal(t, models.ConfirmationRejected, state.Confirmation)
			before, audits := w.snapshot(t), w.auditCount(t)
			negativePreview, err := w.service.Preview(ctx, w.actor, w.application, req)
			period2665RequireHTTPError(t, err, http.StatusBadRequest)
			require.Nil(t, negativePreview, "a rejected outcome must not issue a fresh usable revision")
			require.Equal(t, before, w.snapshot(t))
			require.Equal(t, audits, w.auditCount(t))
			changed, err := w.service.Change(ctx, w.actor, w.application, req)
			period2665RequireHTTPError(t, err, http.StatusConflict)
			require.Nil(t, changed)
			require.Equal(t, before, w.snapshot(t), "stale approved preview must not change parents, entities, votes or the negative outcome")
			require.Equal(t, audits, w.auditCount(t), "denied preview/change must append no partial audit")
		})
	}
}
