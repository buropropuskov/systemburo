package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Keep the actual application queue caller, observing its delivery boundary at
// the instant enqueue occurs. Root-DB reads below cannot see uncommitted moves.
type manualAttachArchive2665Spy struct {
	*periodArchive2665Spy
	onEnqueue func(int)
}

func (s *manualAttachArchive2665Spy) EnqueueApplication(id int, reason string) {
	if s.onEnqueue != nil {
		s.onEnqueue(id)
	}
	s.periodArchive2665Spy.EnqueueApplication(id, reason)
}

func (s *manualAttachArchive2665Spy) EnqueueApplications(ids []int, reason string) {
	for _, id := range ids {
		s.EnqueueApplication(id, reason)
	}
}

func wireManualAttachPublishers2665(t *testing.T, w *manualAttach2665World) periodPublishers2665Harness {
	t.Helper()
	p := wirePeriodPublishers2665(t, w.period2665World)
	resolver := services.NewPermissionResolver(w.db)
	w.service = services.NewManualAttachService(w.db, services.NewAuditRecorder(w.db),
		services.NewTablesRefreshPublisher(w.db, resolver, p.events),
		services.NewAvailableRefreshPublisher(w.db, resolver, p.events))
	require.NoError(t, services.ConfigureManualAttachUpdates(w.service, p.applications))
	return p
}

func TestManualAttach2665DBPublishersCommittedExactlyOnce(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, operation := range []string{"reattach", "adopt"} {
			t.Run(string(kind)+"/"+operation, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				movedScope := bindPeriodPublisher2665Table(t, w.period2665World, kind, w.entity)
				keptScope := bindPeriodPublisher2665Table(t, w.period2665World, kind, w.individual)
				unrelatedScope := bindPeriodPublisher2665Table(t, w.period2665World, kind, w.target)
				p := wireManualAttachPublishers2665(t, &w)
				request := services.AttachToApplicationRequest{TargetAttachmentID: &w.attachment, PeriodChoice: "source", SourceAttachmentID: &w.attachment}
				expectedAttachment := w.attachment
				if operation == "adopt" {
					request.TargetAttachmentID, request.ApplicationID = nil, &w.application
					expectedAttachment = w.orphan
				}
				parentBefore := w.parentAndVotesSnapshot(t)
				_, _, individualBefore := w.readPeriod(t, w.individual)
				assertCommitted := func() {
					for _, id := range []int{w.entity, w.individual} {
						var attachmentID int
						require.NoError(t, w.db.Table(string(kind)).Select("attachment_id").Where("id=?", id).Scan(&attachmentID).Error)
						require.Equal(t, expectedAttachment, attachmentID)
					}
					var parent models.Attachment
					require.NoError(t, w.db.First(&parent, expectedAttachment).Error)
					require.NotNil(t, parent.ApplicationID)
					require.Equal(t, w.application, *parent.ApplicationID)
					require.False(t, parent.IsManual)
					if operation == "reattach" {
						var orphans int64
						require.NoError(t, w.db.Model(&models.Attachment{}).Where("id=?", w.orphan).Count(&orphans).Error)
						require.Zero(t, orphans)
					}
					entityType := models.AuditEntityEmployee
					if kind == services.ElementCar {
						entityType = models.AuditEntityCar
					}
					var audits int64
					require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND actor_user_id=? AND action=?", entityType, w.entity, w.actor, models.AuditActionDatesChanged).Count(&audits).Error)
					require.EqualValues(t, 1, audits)
					require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id IN ? AND actor_user_id=? AND action=?", entityType, []int{w.entity, w.individual}, w.actor, "update").Count(&audits).Error)
					require.EqualValues(t, 2, audits, "both final attachment audit rows must be committed before delivery")
				}
				queueObserved, eventsObserved := 0, 0
				p.applications.SetBlankExportEnqueuer(&manualAttachArchive2665Spy{periodArchive2665Spy: p.archive, onEnqueue: func(id int) {
					require.Equal(t, w.application, id)
					assertCommitted()
					queueObserved++
				}})
				p.events.onEvent = func(event realtime.Event) {
					assertCommitted()
					eventsObserved++
				}
				p.requireSilent(t)
				result, err := w.service.AttachToApplication(context.Background(), w.orphan, request, w.actor)
				require.NoError(t, err)
				require.True(t, result.Success)
				require.Equal(t, w.application, result.ApplicationID)
				require.Equal(t, 1, queueObserved)
				require.Equal(t, 5, eventsObserved, "two table scopes, available, application detail and center")
				require.Equal(t, 1, p.events.count("tables.refresh", movedScope))
				require.Equal(t, 1, p.events.count("tables.refresh", keptScope))
				require.Zero(t, p.events.count("tables.refresh", unrelatedScope))
				require.Equal(t, 1, p.events.count("available.new", "available"))
				require.Equal(t, 1, p.events.count("application.updated", fmt.Sprintf("application:%d", w.application)))
				require.Equal(t, 1, p.events.count("applications.refresh", "applications-center"))
				require.Equal(t, []int{w.application}, p.archive.applicationIDs)
				require.Equal(t, []string{services.BlankExportReasonUpdate}, p.archive.reasons)
				require.Empty(t, p.notifications.calls, "manual hook does not introduce participant notifications")
				require.Equal(t, parentBefore, w.parentAndVotesSnapshot(t))
				_, mode, kept := w.readPeriod(t, w.individual)
				require.Equal(t, models.PeriodIndividual, mode)
				require.Equal(t, individualBefore, kept)
			})
		}
	}
}

func TestManualAttach2665DBPublishersDeniedAndRollbackSilent(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"denied", "audit_rollback"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				bindPeriodPublisher2665Table(t, w.period2665World, kind, w.entity)
				bindPeriodPublisher2665Table(t, w.period2665World, kind, w.individual)
				p := wireManualAttachPublishers2665(t, &w)
				failure := errors.New("synthetic manual attach publisher regression audit failure")
				seenMove := false
				if scenario == "denied" {
					require.NoError(t, w.db.Model(&models.UserPermissionOverride{}).Where("user_id=? AND permission_key=?", w.actor, services.KeyDetailPeriodChange).Update("value", "deny").Error)
				} else {
					const callback = "manual_attach_publishers2665_fail_audit"
					entityType := models.AuditEntityEmployee
					if kind == services.ElementCar {
						entityType = models.AuditEntityCar
					}
					require.NoError(t, w.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
						entry, ok := tx.Statement.Dest.(*models.AuditLog)
						if !ok || entry.EntityType != entityType || entry.EntityID == nil || *entry.EntityID != w.entity || entry.ActorUserID == nil || *entry.ActorUserID != w.actor || entry.Action != models.AuditActionDatesChanged {
							return
						}
						var attachmentID int
						if err := tx.Session(&gorm.Session{NewDB: true}).Table(string(kind)).Select("attachment_id").Where("id=?", w.entity).Scan(&attachmentID).Error; err != nil {
							tx.AddError(err)
							return
						}
						seenMove = attachmentID == w.attachment
						tx.AddError(failure)
					}))
					t.Cleanup(func() { require.NoError(t, w.db.Callback().Create().Remove(callback)) })
				}
				before := w.snapshot(t)
				_, err := w.service.AttachToApplication(context.Background(), w.orphan, services.AttachToApplicationRequest{TargetAttachmentID: &w.attachment, PeriodChoice: "source", SourceAttachmentID: &w.attachment}, w.actor)
				if scenario == "denied" {
					period2665RequireHTTPError(t, err, http.StatusForbidden)
				} else {
					require.ErrorIs(t, err, failure)
					require.True(t, seenMove, "fault follows the real transactional entity move")
				}
				require.Equal(t, before, w.snapshot(t), "rollback preserves orphan, rows, periods, votes and all audit records")
				p.requireSilent(t)
			})
		}
	}
}
