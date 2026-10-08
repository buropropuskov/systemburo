package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The table producer and application service are real. Only their delivery
// boundaries are spies; no permission, scope or command SQL is mocked.
type periodPublisher2665Spy struct {
	mu        sync.Mutex
	events    []realtime.Event
	audiences [][]int
	onEvent   func(realtime.Event)
}

func (p *periodPublisher2665Spy) Publish(userID int, event realtime.Event) {
	p.PublishMany([]int{userID}, event)
}
func (p *periodPublisher2665Spy) PublishMany(users []int, event realtime.Event) {
	p.mu.Lock()
	p.events = append(p.events, event)
	p.audiences = append(p.audiences, append([]int(nil), users...))
	check := p.onEvent
	p.mu.Unlock()
	if check != nil {
		check(event)
	}
}
func (p *periodPublisher2665Spy) count(eventType, scope string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	count := 0
	for _, event := range p.events {
		if event.Type == eventType && (scope == "" || event.Scope == scope) {
			count++
		}
	}
	return count
}
func (p *periodPublisher2665Spy) empty(t *testing.T) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	require.Empty(t, p.events)
}

type periodArchive2665Spy struct {
	applicationIDs []int
	reasons        []string
}

func (p *periodArchive2665Spy) EnqueueApplication(id int, reason string) {
	p.applicationIDs = append(p.applicationIDs, id)
	p.reasons = append(p.reasons, reason)
}
func (p *periodArchive2665Spy) EnqueueApplications(ids []int, reason string) {
	for _, id := range ids {
		p.EnqueueApplication(id, reason)
	}
}

type periodNotification2665Call struct {
	userID                     int
	kind, title, message, data string
}
type periodNotification2665Spy struct {
	services.NotificationService // Unused interface methods deliberately remain unavailable.
	calls                        []periodNotification2665Call
}

func (p *periodNotification2665Spy) CreateForUser(_ context.Context, userID int, kind, title, message string, data *string) error {
	value := ""
	if data != nil {
		value = *data
	}
	p.calls = append(p.calls, periodNotification2665Call{userID, kind, title, message, value})
	return nil
}

type periodPublishers2665Harness struct {
	events        *periodPublisher2665Spy
	archive       *periodArchive2665Spy
	notifications *periodNotification2665Spy
	applications  services.ApplicationService
}

func wirePeriodPublishers2665(t *testing.T, w period2665World) periodPublishers2665Harness {
	t.Helper()
	events, archive, notifications := &periodPublisher2665Spy{}, &periodArchive2665Spy{}, &periodNotification2665Spy{}
	require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: w.actor, PermissionKey: "page.available", Value: "allow", GrantedAt: w.clock.UTC()}).Error)
	resolver := services.NewPermissionResolver(w.db)
	producer := services.NewTablesRefreshPublisher(w.db, resolver, events)
	available := services.NewAvailableRefreshPublisher(w.db, resolver, events)
	apps := services.NewApplicationService(w.db, nil, notifications, nil, nil, services.NewAuditRecorder(w.db), services.WithApplicationTablesProducer(producer), services.WithApplicationAvailableProducer(available), services.WithRealtimePublisher(events))
	apps.SetBlankExportEnqueuer(archive)
	require.NoError(t, services.ConfigureEntityPeriodUpdates(w.commands, apps))
	return periodPublishers2665Harness{events, archive, notifications, apps}
}

func bindPeriodPublisher2665Table(t *testing.T, w period2665World, kind services.ElementKind, id int) string {
	t.Helper()
	name := fmt.Sprintf("period_pub2665_%s_%d", kind, id)
	tableType := models.TableTypePeople
	if kind == services.ElementCar {
		tableType = models.TableTypeCars
	}
	table := models.SystemTable{Name: name, TableType: tableType, IsActive: true}
	require.NoError(t, w.db.Create(&table).Error)
	require.NoError(t, w.db.Create(&models.UserPermissionOverride{UserID: w.actor, PermissionKey: "table." + name + ".view", Value: "allow", GrantedAt: w.clock.UTC()}).Error)
	if kind == services.ElementCar {
		require.NoError(t, w.db.Create(&models.CarTargetTable{CarID: id, TableID: table.ID}).Error)
	} else {
		require.NoError(t, w.db.Create(&models.EmployeeTargetTable{EmployeeID: id, TableID: table.ID}).Error)
	}
	return fmt.Sprintf("tables:%d", table.ID)
}

func requirePeriodPublisher2665Committed(t *testing.T, w period2665World, kind services.ElementKind, id int, dateTo string) {
	t.Helper()
	// Use the root DB handle, never the command transaction. This must see both
	// the new row and audit at the instant the real producer publishes.
	var row struct {
		PeriodMode  models.PeriodMode
		EntryDateTo string
	}
	require.NoError(t, w.db.Table(string(kind)).Select("period_mode,entry_date_to").Where("id=?", id).Scan(&row).Error)
	require.Equal(t, models.PeriodIndividual, row.PeriodMode)
	require.Equal(t, dateTo, row.EntryDateTo)
	entityType := models.AuditEntityEmployee
	if kind == services.ElementCar {
		entityType = models.AuditEntityCar
	}
	var audits int64
	require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND actor_user_id=? AND action=?", entityType, id, w.actor, models.AuditActionDatesChanged).Count(&audits).Error)
	require.EqualValues(t, 1, audits)
}

func (p periodPublishers2665Harness) requireSilent(t *testing.T) {
	t.Helper()
	p.events.empty(t)
	require.Empty(t, p.archive.applicationIDs)
	require.Empty(t, p.notifications.calls)
}

func TestEntityPeriod2665DBPublishersSuccessAfterCommit(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPeriod2665World(t, kind)
			targetScope := bindPeriodPublisher2665Table(t, w, kind, w.target)
			neighborScope := bindPeriodPublisher2665Table(t, w, kind, w.neighbor)
			p := wirePeriodPublishers2665(t, w)
			ctx := context.Background()
			preview, err := w.commands.Inspect(ctx, w.actor, kind, w.target, nil)
			require.NoError(t, err)
			p.requireSilent(t)
			req := w.request(preview.Revision, 7)
			seenCommitted := 0
			p.events.onEvent = func(event realtime.Event) {
				if event.Type != "tables.refresh" {
					return
				}
				require.Equal(t, targetScope, event.Scope)
				requirePeriodPublisher2665Committed(t, w, kind, w.target, req.Period.EntryDateTo)
				seenCommitted++
			}
			_, err = w.commands.Change(ctx, w.actor, kind, w.target, req)
			require.NoError(t, err)
			require.Equal(t, 1, seenCommitted)
			require.Equal(t, 1, p.events.count("tables.refresh", targetScope))
			require.Zero(t, p.events.count("tables.refresh", neighborScope))
			require.Equal(t, 1, p.events.count("available.new", "available"))
			require.Equal(t, 1, p.events.count("application.updated", fmt.Sprintf("application:%d", w.application)))
			require.Equal(t, 1, p.events.count("applications.refresh", "applications-center"))
			require.Equal(t, []int{w.application}, p.archive.applicationIDs)
			require.Equal(t, []string{services.BlankExportReasonUpdate}, p.archive.reasons)
			require.NotEmpty(t, p.notifications.calls)
			for _, call := range p.notifications.calls {
				require.NotEqual(t, w.actor, call.userID)
				require.Equal(t, services.NotificationTypeApplicationDatesChanged, call.kind)
				var payload map[string]any
				require.NoError(t, json.Unmarshal([]byte(call.data), &payload))
				require.Len(t, payload, 4)
				require.Equal(t, float64(w.application), payload["application_id"])
				require.Equal(t, float64(w.target), payload["entity_id"])
				require.Equal(t, string(kind), payload["entity_kind"])
				require.Equal(t, false, payload["approvals_reset"])
				require.NotContains(t, call.message, "TEST2665-0", "notification must not resolve the vehicle plate")
			}
		})
	}
}

func TestEntityPeriod2665DBPublishersFailuresStaySilent(t *testing.T) {
	for _, scenario := range []string{"inspect", "noop", "forbidden", "stale", "audit_rollback"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupPeriod2665World(t, services.ElementCar)
			bindPeriodPublisher2665Table(t, w, w.kind, w.target)
			p := wirePeriodPublishers2665(t, w)
			ctx := context.Background()
			preview, err := w.commands.Inspect(ctx, w.actor, w.kind, w.target, nil)
			require.NoError(t, err)
			p.requireSilent(t)
			if scenario == "inspect" {
				return
			}
			req := w.request(preview.Revision, 7)
			want := http.StatusBadRequest
			var failure error
			switch scenario {
			case "noop":
				require.NoError(t, w.db.Table("cars").Where("id=?", w.target).Updates(map[string]any{"period_mode": models.PeriodIndividual, "entry_date_from": req.Period.EntryDateFrom, "entry_date_to": req.Period.EntryDateTo, "entry_time_from": "08:00:00", "entry_time_to": "22:00:00"}).Error)
				current, err := w.commands.Inspect(ctx, w.actor, w.kind, w.target, nil)
				require.NoError(t, err)
				req.ExpectedRevision = current.Revision
			case "forbidden":
				require.NoError(t, w.db.Model(&models.UserPermissionOverride{}).Where("user_id=? AND permission_key=?", w.actor, services.KeyDetailPeriodChange).Update("value", "deny").Error)
				want = http.StatusForbidden
			case "stale":
				require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).UpdateColumn("entry_time_to", "23:00:00").Error)
				want = http.StatusConflict
			case "audit_rollback":
				failure = errors.New("synthetic entity publisher audit failure")
				const callback = "period2665_publisher_entity_audit_failure"
				require.NoError(t, w.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
					entry, ok := tx.Statement.Dest.(*models.AuditLog)
					if ok && entry.EntityType == models.AuditEntityCar && entry.EntityID != nil && *entry.EntityID == w.target && entry.ActorUserID != nil && *entry.ActorUserID == w.actor && entry.Action == models.AuditActionDatesChanged {
						tx.AddError(failure)
					}
				}))
				t.Cleanup(func() { require.NoError(t, w.db.Callback().Create().Remove(callback)) })
			}
			before, parentBefore := w.rowSnapshot(t, w.target), w.parentAndVotesSnapshot(t)
			_, err = w.commands.Change(ctx, w.actor, w.kind, w.target, req)
			if failure != nil {
				require.ErrorIs(t, err, failure)
			} else {
				period2665RequireHTTPError(t, err, want)
			}
			p.requireSilent(t)
			require.Equal(t, before, w.rowSnapshot(t, w.target))
			require.Equal(t, parentBefore, w.parentAndVotesSnapshot(t))
		})
	}
}

func TestAttachmentPeriod2665DBPublishersChangedIDsAfterCommit(t *testing.T) {
	for _, policy := range []string{services.IndividualPolicyPreserve, services.IndividualPolicyReplace} {
		t.Run(policy, func(t *testing.T) {
			w := setupAttachmentPeriod2665World(t, services.ElementEmployee)
			scopes := make(map[string]struct {
				kind services.ElementKind
				id   int
			})
			for _, row := range []struct {
				kind services.ElementKind
				id   int
			}{{w.kind, w.target}, {w.kind, w.neighbor}, {w.secondKind, w.secondTarget}, {w.secondKind, w.secondNeighbor}} {
				scopes[bindPeriodPublisher2665Table(t, w.period2665World, row.kind, row.id)] = row
			}
			// A third attachment is outside the requested selection.
			active := 1
			unselected := models.Attachment{ApplicationID: &w.application, AttachmentType: "people", Status: &active}
			require.NoError(t, w.db.Create(&unselected).Error)
			employee := models.Employee{AttachmentID: &unselected.ID, Status: &active}
			require.NoError(t, w.db.Create(&employee).Error)
			unselectedScope := bindPeriodPublisher2665Table(t, w.period2665World, services.ElementEmployee, employee.ID)
			p := wirePeriodPublishers2665(t, w.period2665World)
			require.NoError(t, services.ConfigureAttachmentPeriodUpdates(w.service, p.applications))
			ctx := context.Background()
			req := w.request([]int{w.attachment, w.secondAttachment}, policy)
			preview, err := w.service.Preview(ctx, w.actor, w.application, req)
			require.NoError(t, err)
			p.requireSilent(t)
			req.ExpectedRevision = preview.Revision
			seenCommitted := 0
			p.events.onEvent = func(event realtime.Event) {
				if event.Type != "tables.refresh" {
					return
				}
				row, exists := scopes[event.Scope]
				require.True(t, exists, "only tables belonging to selected changed IDs may refresh")
				// An inherited Employee keeps own NULL; verify its committed parent
				// in addition to the individual/Car own columns.
				var effective struct{ EntryDateTo string }
				require.NoError(t, w.db.Raw("SELECT CASE WHEN e.period_mode='individual' THEN e.entry_date_to ELSE a.entry_date_to END AS entry_date_to FROM "+string(row.kind)+" e JOIN attachments a ON a.id=e.attachment_id WHERE e.id=?", row.id).Scan(&effective).Error)
				require.Equal(t, req.Period.EntryDateTo, effective.EntryDateTo)
				entityType := models.AuditEntityEmployee
				if row.kind == services.ElementCar {
					entityType = models.AuditEntityCar
				}
				var audits int64
				require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND actor_user_id=? AND action=?", entityType, row.id, w.actor, models.AuditActionDatesChanged).Count(&audits).Error)
				require.EqualValues(t, 1, audits)
				var applicationAudits int64
				require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND actor_user_id=? AND action=?", models.AuditEntityApplication, w.application, w.actor, models.AuditActionDatesChanged).Count(&applicationAudits).Error)
				require.EqualValues(t, 1, applicationAudits, "even the final application audit must already be committed before the first refresh")
				seenCommitted++
			}
			result, err := w.service.Change(ctx, w.actor, w.application, req)
			require.NoError(t, err)
			want := 2
			if policy == services.IndividualPolicyReplace {
				want = 4
			}
			require.Equal(t, want, seenCommitted)
			for scope, row := range scopes {
				changedIDs := result.ChangedEmployeeIDs
				if row.kind == services.ElementCar {
					changedIDs = result.ChangedCarIDs
				}
				changed := false
				for _, id := range changedIDs {
					if id == row.id {
						changed = true
					}
				}
				if changed {
					require.Equal(t, 1, p.events.count("tables.refresh", scope))
				} else {
					require.Zero(t, p.events.count("tables.refresh", scope))
				}
			}
			require.Zero(t, p.events.count("tables.refresh", unselectedScope))
			require.Equal(t, 1, p.events.count("available.new", "available"))
			require.Equal(t, 1, p.events.count("application.updated", fmt.Sprintf("application:%d", w.application)))
			require.Equal(t, 1, p.events.count("applications.refresh", "applications-center"))
			require.Equal(t, []int{w.application}, p.archive.applicationIDs)
			require.NotEmpty(t, p.notifications.calls)
			for _, call := range p.notifications.calls {
				require.NotEqual(t, w.actor, call.userID)
				require.Equal(t, services.NotificationTypeApplicationDatesChanged, call.kind)
				var payload map[string]any
				require.NoError(t, json.Unmarshal([]byte(call.data), &payload))
				require.Len(t, payload, 3)
				require.Equal(t, float64(w.application), payload["application_id"])
				require.Equal(t, false, payload["approvals_reset"])
				require.ElementsMatch(t, []any{float64(w.attachment), float64(w.secondAttachment)}, payload["attachment_ids"])
			}
		})
	}
}

func TestAttachmentPeriod2665DBPublishersFailuresStaySilent(t *testing.T) {
	for _, scenario := range []string{"preview", "noop", "forbidden", "stale", "audit_rollback"} {
		t.Run(scenario, func(t *testing.T) {
			w := setupAttachmentPeriod2665World(t, services.ElementEmployee)
			bindPeriodPublisher2665Table(t, w.period2665World, w.kind, w.target)
			p := wirePeriodPublishers2665(t, w.period2665World)
			require.NoError(t, services.ConfigureAttachmentPeriodUpdates(w.service, p.applications))
			ctx := context.Background()
			req := w.request([]int{w.attachment}, services.IndividualPolicyPreserve)
			if scenario == "noop" {
				req.Period.EntryDateTo = w.clock.AddDate(0, 0, 2).Format("2006-01-02")
			}
			preview, err := w.service.Preview(ctx, w.actor, w.application, req)
			require.NoError(t, err)
			p.requireSilent(t)
			if scenario == "preview" {
				return
			}
			req.ExpectedRevision = preview.Revision
			want := http.StatusBadRequest
			var failure error
			switch scenario {
			case "forbidden":
				require.NoError(t, w.db.Model(&models.UserPermissionOverride{}).Where("user_id=? AND permission_key=?", w.actor, services.KeyApplicationPeriodChange).Update("value", "deny").Error)
				want = http.StatusForbidden
			case "stale":
				require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).UpdateColumn("entry_time_to", "23:00:00").Error)
				want = http.StatusConflict
			case "audit_rollback":
				failure = errors.New("synthetic group publisher audit failure")
				const callback = "period2665_publisher_group_audit_failure"
				require.NoError(t, w.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
					entry, ok := tx.Statement.Dest.(*models.AuditLog)
					if ok && entry.EntityType == models.AuditEntityApplication && entry.EntityID != nil && *entry.EntityID == w.application && entry.ActorUserID != nil && *entry.ActorUserID == w.actor && entry.Action == models.AuditActionDatesChanged {
						tx.AddError(failure)
					}
				}))
				t.Cleanup(func() { require.NoError(t, w.db.Callback().Create().Remove(callback)) })
			}
			before, audits := w.snapshot(t), w.auditCount(t)
			_, err = w.service.Change(ctx, w.actor, w.application, req)
			if failure != nil {
				require.ErrorIs(t, err, failure)
			} else {
				period2665RequireHTTPError(t, err, want)
			}
			p.requireSilent(t)
			require.Equal(t, before, w.snapshot(t))
			require.Equal(t, audits, w.auditCount(t))
		})
	}
}

func TestEntityPeriod2665DBPublishersManualWithoutApplication(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		t.Run(string(kind), func(t *testing.T) {
			w := setupPeriod2665World(t, kind)
			scope := bindPeriodPublisher2665Table(t, w, kind, w.target)
			require.NoError(t, w.db.Table("attachments").Where("id=?", w.attachment).Updates(map[string]any{"application_id": nil, "is_manual": true, "created_by_user_id": w.actor}).Error)
			p := wirePeriodPublishers2665(t, w)
			ctx := context.Background()
			preview, err := w.commands.Inspect(ctx, w.actor, kind, w.target, nil)
			require.NoError(t, err)
			require.Nil(t, preview.ApplicationID)
			p.requireSilent(t)
			req := w.request(preview.Revision, 7)
			seenCommitted := 0
			p.events.onEvent = func(event realtime.Event) {
				if event.Type == "tables.refresh" {
					requirePeriodPublisher2665Committed(t, w, kind, w.target, req.Period.EntryDateTo)
					seenCommitted++
				}
			}
			changed, err := w.commands.Change(ctx, w.actor, kind, w.target, req)
			require.NoError(t, err)
			require.Nil(t, changed.ApplicationID)
			require.Equal(t, 1, seenCommitted)
			require.Equal(t, 1, p.events.count("tables.refresh", scope))
			require.Equal(t, 1, p.events.count("available.new", "available"))
			require.Zero(t, p.events.count("application.updated", ""))
			require.Zero(t, p.events.count("applications.refresh", ""))
			require.Empty(t, p.archive.applicationIDs)
			require.Empty(t, p.notifications.calls)
		})
	}
}
