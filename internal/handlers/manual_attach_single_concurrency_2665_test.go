package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/realtime"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSingleManualAttach2665DBConcurrentSameEntity(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, createNew := range []bool{false, true} {
			label := "existing"
			if createNew {
				label = "new"
			}
			t.Run(string(kind)+"/"+label, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				command := singleAttach2665Service(w)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				requests := [2]services.SingleManualAttachRequest{singleAttach2665Request(w, createNew), singleAttach2665Request(w, createNew)}
				for i := range requests {
					preview, err := command.Preview(ctx, w.actor, kind, w.entity, requests[i])
					require.NoError(t, err)
					requests[i].ExpectedRevision = preview.Revision
				}
				require.Equal(t, requests[0].ExpectedRevision, requests[1].ExpectedRevision)
				orphanBefore := singleAttach2665JSON(t, w.db, "attachments", w.orphan)
				neighborBefore := singleAttach2665JSON(t, w.db, string(kind), w.individual)
				parentBefore := w.parentAndVotesSnapshot(t)
				var attachmentsBefore int64
				require.NoError(t, w.db.Model(&models.Attachment{}).Count(&attachmentsBefore).Error)
				var published atomic.Int32
				command.SetAfterAttach(func(context.Context, int) { published.Add(1) })

				// Both transactions reach the actual application-lock statement with
				// their original manual locator before either is allowed to acquire it.
				// This coordinates real SQL transactions; it does not mock a result.
				ready, release := make(chan struct{}, 2), make(chan struct{})
				var releaseOnce sync.Once
				unblock := func() { releaseOnce.Do(func() { close(release) }) }
				defer unblock()
				const callback = "single_attach2665_concurrent_application_barrier"
				require.NoError(t, w.db.Callback().Row().Before("gorm:row").Register(callback, func(tx *gorm.DB) {
					if !strings.Contains(tx.Statement.SQL.String(), "FOR UPDATE OF app") {
						return
					}
					select {
					case ready <- struct{}{}:
					case <-ctx.Done():
						tx.AddError(ctx.Err())
						return
					}
					select {
					case <-release:
					case <-ctx.Done():
						tx.AddError(ctx.Err())
					}
				}))
				t.Cleanup(func() { require.NoError(t, w.db.Callback().Row().Remove(callback)) })
				type outcome struct {
					result *services.SingleManualAttachResult
					err    error
				}
				results := make(chan outcome, 2)
				var workers sync.WaitGroup
				workers.Add(2)
				defer func() { cancel(); unblock(); workers.Wait() }()
				for _, request := range requests {
					request := request
					go func() {
						defer workers.Done()
						result, err := command.Attach(ctx, w.actor, kind, w.entity, request)
						results <- outcome{result, err}
					}()
				}
				for i := 0; i < 2; i++ {
					select {
					case <-ready:
					case <-ctx.Done():
						unblock()
						t.Fatal("both real transactions did not reach the application-lock barrier")
					}
				}
				unblock()
				successes, conflicts := 0, 0
				var winner *services.SingleManualAttachResult
				for i := 0; i < 2; i++ {
					select {
					case outcome := <-results:
						if outcome.err == nil {
							successes++
							winner = outcome.result
						} else {
							period2665RequireHTTPError(t, outcome.err, http.StatusConflict)
							conflicts++
						}
					case <-ctx.Done():
						t.Fatal("concurrent attaches did not finish within the bounded context")
					}
				}
				require.Equal(t, 1, successes)
				require.Equal(t, 1, conflicts)
				require.EqualValues(t, 1, published.Load())
				require.NotNil(t, winner)
				require.NotNil(t, winner.DestinationAttachmentID)
				binding, _, _ := w.readPeriod(t, w.entity)
				require.Equal(t, *winner.DestinationAttachmentID, binding)
				require.Equal(t, orphanBefore, singleAttach2665JSON(t, w.db, "attachments", w.orphan))
				require.Equal(t, neighborBefore, singleAttach2665JSON(t, w.db, string(kind), w.individual))
				require.Equal(t, parentBefore, w.parentAndVotesSnapshot(t))
				var attachmentsAfter int64
				require.NoError(t, w.db.Model(&models.Attachment{}).Count(&attachmentsAfter).Error)
				if createNew {
					require.Equal(t, attachmentsBefore+1, attachmentsAfter)
				} else {
					require.Equal(t, attachmentsBefore, attachmentsAfter)
				}
				entityType := models.AuditEntityEmployee
				if kind == services.ElementCar {
					entityType = models.AuditEntityCar
				}
				for _, action := range []string{"update", models.AuditActionDatesChanged} {
					var count int64
					require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND actor_user_id=? AND action=?", entityType, w.entity, w.actor, action).Count(&count).Error)
					require.EqualValues(t, 1, count, "losing transaction must not duplicate either selected-row audit")
				}
			})
		}
	}
}

func TestSingleManualAttach2665DBRealPublishersSelectedOnly(t *testing.T) {
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"commit_existing", "commit_new", "preview", "rollback"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				w := setupManualAttach2665World(t, kind)
				selectedScope := bindPeriodPublisher2665Table(t, w.period2665World, kind, w.entity)
				neighborScope := bindPeriodPublisher2665Table(t, w.period2665World, kind, w.individual)
				unrelatedScope := bindPeriodPublisher2665Table(t, w.period2665World, kind, w.target)
				p := wirePeriodPublishers2665(t, w.period2665World)
				resolver := services.NewPermissionResolver(w.db)
				command := services.NewSingleManualAttachCommandService(w.db, services.NewAuditRecorder(w.db), services.NewTablesRefreshPublisher(w.db, resolver, p.events), services.NewAvailableRefreshPublisher(w.db, resolver, p.events))
				require.NoError(t, services.ConfigureSingleManualAttachUpdates(command, p.applications))
				request := singleAttach2665Request(w, scenario == "commit_new")
				preview, err := command.Preview(context.Background(), w.actor, kind, w.entity, request)
				require.NoError(t, err)
				request.ExpectedRevision = preview.Revision
				p.requireSilent(t)
				if scenario == "preview" {
					return
				}
				entityType := models.AuditEntityEmployee
				if kind == services.ElementCar {
					entityType = models.AuditEntityCar
				}
				orphanBefore := singleAttach2665JSON(t, w.db, "attachments", w.orphan)
				neighborBefore := singleAttach2665JSON(t, w.db, string(kind), w.individual)
				assertCommitted := func() {
					binding, _, _ := w.readPeriod(t, w.entity)
					require.NotEqual(t, w.orphan, binding, "root connection sees committed move before delivery")
					for _, action := range []string{"update", models.AuditActionDatesChanged} {
						var count int64
						require.NoError(t, w.db.Model(&models.AuditLog{}).Where("entity_type=? AND entity_id=? AND actor_user_id=? AND action=?", entityType, w.entity, w.actor, action).Count(&count).Error)
						require.EqualValues(t, 1, count, "both audits must be committed before any publisher")
					}
					require.Equal(t, orphanBefore, singleAttach2665JSON(t, w.db, "attachments", w.orphan))
					require.Equal(t, neighborBefore, singleAttach2665JSON(t, w.db, string(kind), w.individual))
				}
				queueObserved, eventsObserved := 0, 0
				p.applications.SetBlankExportEnqueuer(&manualAttachArchive2665Spy{periodArchive2665Spy: p.archive, onEnqueue: func(id int) { require.Equal(t, w.application, id); assertCommitted(); queueObserved++ }})
				p.events.onEvent = func(realtime.Event) { assertCommitted(); eventsObserved++ }
				failure := errors.New("synthetic single attach publisher audit rollback")
				if scenario == "rollback" {
					const callback = "single_attach2665_publishers_rollback"
					require.NoError(t, w.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
						entry, ok := tx.Statement.Dest.(*models.AuditLog)
						if ok && entry.EntityType == entityType && entry.EntityID != nil && *entry.EntityID == w.entity && entry.Action == models.AuditActionDatesChanged {
							tx.AddError(failure)
						}
					}))
					t.Cleanup(func() { require.NoError(t, w.db.Callback().Create().Remove(callback)) })
				}
				before := w.snapshot(t)
				_, err = command.Attach(context.Background(), w.actor, kind, w.entity, request)
				if scenario == "rollback" {
					require.ErrorIs(t, err, failure)
					require.Equal(t, before, w.snapshot(t))
					p.requireSilent(t)
					require.Zero(t, eventsObserved)
					require.Zero(t, queueObserved)
					return
				}
				require.NoError(t, err)
				require.Equal(t, 1, p.events.count("tables.refresh", selectedScope))
				require.Zero(t, p.events.count("tables.refresh", neighborScope))
				require.Zero(t, p.events.count("tables.refresh", unrelatedScope))
				require.Equal(t, 1, p.events.count("available.new", "available"))
				require.Equal(t, 1, p.events.count("application.updated", fmt.Sprintf("application:%d", w.application)))
				require.Equal(t, 1, p.events.count("applications.refresh", "applications-center"))
				require.Equal(t, 4, eventsObserved)
				require.Equal(t, 1, queueObserved)
				require.Equal(t, []int{w.application}, p.archive.applicationIDs)
				require.Equal(t, []string{services.BlankExportReasonUpdate}, p.archive.reasons)
				require.Empty(t, p.notifications.calls)
			})
		}
	}
}
