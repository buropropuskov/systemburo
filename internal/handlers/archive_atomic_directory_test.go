package handlers_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type archiveAtomicDirectoryObserver struct{ calls int }

func (s *archiveAtomicDirectoryObserver) EnqueueApplication(int, string)    { s.calls++ }
func (s *archiveAtomicDirectoryObserver) EnqueueApplications([]int, string) { s.calls++ }

// An actual outbox INSERT failure must roll back the name/status/links and audit.
// No-op actions still succeed while that same INSERT is forbidden.
func TestArchiveAtomicDirectory(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	ctx := context.Background()
	var actor models.User
	require.NoError(t, db.Order("id").First(&actor).Error)

	for _, kind := range []string{"organization", "company"} {
		for _, action := range []string{"update", "stale_noop", "approve", "rename", "merge", "update_noop", "type_only", "approve_noop", "rename_noop"} {
			t.Run(kind+"/"+action, func(t *testing.T) {
				name := fmt.Sprintf(`ООО "Атомарность %s %s"`, kind, action)
				targetName := fmt.Sprintf(`ООО "Цель %s %s"`, kind, action)
				typ := models.OrgTypeContractor
				var sourceID, targetID int
				table, column, entity := "organizations", "organization_id", models.AuditEntityOrganization
				observer := &archiveAtomicDirectoryObserver{}
				var update func(string, *string) error
				var approve, rename, merge func() error
				if kind == "organization" {
					source := seedModerationOrg(t, db, name, models.ModerationPending)
					target := seedModerationOrg(t, db, targetName, models.ModerationApproved)
					sourceID, targetID = source.ID, target.ID
					svc := services.NewOrganizationService(db)
					svc.SetBlankExportEnqueuer(observer)
					update = func(n string, typ *string) error {
						_, err := svc.Update(ctx, actor.ID, sourceID, services.CreateOrganizationRequest{Name: n, Type: typ})
						return err
					}
					approve = func() error { _, err := svc.ApproveModeration(ctx, actor.ID, sourceID); return err }
					rename = func() error { _, err := svc.RenameModeration(ctx, actor.ID, sourceID, name); return err }
					merge = func() error { _, err := svc.MergeModeration(ctx, actor.ID, sourceID, targetID); return err }
				} else {
					table, column, entity = "companies", "company_id", models.AuditEntityCompany
					source := models.Company{Name: name, Type: &typ, IsActive: true, ModerationStatus: models.ModerationPending}
					target := models.Company{Name: targetName, Type: &typ, IsActive: true, ModerationStatus: models.ModerationApproved}
					require.NoError(t, db.Create(&source).Error)
					require.NoError(t, db.Create(&target).Error)
					sourceID, targetID = source.ID, target.ID
					svc := services.NewCompanyService(db)
					svc.SetBlankExportEnqueuer(observer)
					update = func(n string, typ *string) error {
						_, err := svc.Update(ctx, actor.ID, sourceID, services.CreateCompanyRequest{Name: n, Type: typ})
						return err
					}
					approve = func() error { _, err := svc.ApproveModeration(ctx, actor.ID, sourceID); return err }
					rename = func() error { _, err := svc.RenameModeration(ctx, actor.ID, sourceID, name); return err }
					merge = func() error { _, err := svc.MergeModeration(ctx, actor.ID, sourceID, targetID); return err }
				}

				// Separate target application must not get a job just because the source merges.
				newApp := func(refID int) int {
					app := models.Application{OrganizationID: td.OrgID, SenderUserID: actor.ID}
					if kind == "organization" {
						app.OrganizationID = refID
					} else {
						app.CompanyID = &refID
					}
					require.NoError(t, db.Create(&app).Error)
					return app.ID
				}
				appID, targetAppID := newApp(sourceID), newApp(targetID)
				attachment := models.Attachment{ApplicationID: &appID, AttachmentType: "people"}
				require.NoError(t, db.Create(&attachment).Error)
				require.NoError(t, db.Model(&attachment).Update(column, sourceID).Error)

				oldName := name
				if action == "approve" || action == "rename" {
					oldName = "ооо" + name[len("ООО"):]
					require.NoError(t, db.Table(table).Where("id = ?", sourceID).Update("name", oldName).Error)
				}
				var mutate func() error
				switch action {
				case "update":
					mutate = func() error { return update(name+" новое", &typ) }
				case "update_noop", "stale_noop":
					mutate = func() error { return update(name, &typ) }
				case "type_only":
					mutate = func() error { return update(name, nil) }
				case "approve", "approve_noop":
					mutate = approve
				case "rename", "rename_noop":
					mutate = rename
				case "merge":
					mutate = merge
				}
				noIntent := action == "update_noop" || action == "type_only" || action == "approve_noop" || action == "rename_noop"
				func() {
					constraint := "archive_atomic_directory_reject"
					require.NoError(t, db.Exec(fmt.Sprintf("ALTER TABLE archive_export_jobs ADD CONSTRAINT %s CHECK (application_id <> %d)", constraint, appID)).Error)
					defer func() {
						require.NoError(t, db.Exec("ALTER TABLE archive_export_jobs DROP CONSTRAINT "+constraint).Error)
					}()
					if action == "stale_noop" {
						// Commit a different name immediately after the preflight read,
						// before Update starts its mutation TX. The request still carries
						// its earlier name and must now enqueue the real change back.
						oldName = "ооо" + name[len("ООО"):]
						changed := false
						const callback = "archive_atomic_directory_interleave"
						require.NoError(t, db.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
							if !changed && tx.Statement.Table == table {
								changed = true
								require.NoError(t, db.Table(table).Where("id = ?", sourceID).Update("name", oldName).Error)
							}
						}))
						defer func() { require.NoError(t, db.Callback().Query().Remove(callback)) }()
					}
					err := mutate()
					if noIntent {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
					}
				}()
				require.Zero(t, observer.calls, "an intent failure or no-op must not notify the postcommit observer")
				var count int64
				require.NoError(t, db.Model(&models.ArchiveExportJob{}).Where("application_id IN ?", []int{appID, targetAppID}).Count(&count).Error)
				require.Zero(t, count)
				if noIntent {
					return
				}

				var source struct {
					Name             string
					ModerationStatus string
				}
				require.NoError(t, db.Table(table).Where("id = ?", sourceID).Take(&source).Error)
				require.Equal(t, oldName, source.Name)
				require.Equal(t, models.ModerationPending, source.ModerationStatus)
				require.Equal(t, int64(1), countRows(t, db, "applications", column, sourceID))
				require.Equal(t, int64(1), countRows(t, db, "attachments", column, sourceID))
				require.Empty(t, auditActions(t, db, entity, sourceID))
				require.Empty(t, auditActions(t, db, entity, targetID))

				// Retry after recovery proves the source SELECT is evaluated before merge moves links.
				if action == "merge" {
					// Stop synchronously after the job INSERT, before merge starts its
					// ref UPDATEs. A different DB transaction must already be unable
					// to lock the source application. The old job-first path fails
					// this probe because it has not touched applications yet.
					func() {
						const callback = "archive_atomic_directory_app_lock"
						var lockErr error
						var observed bool
						require.NoError(t, db.Callback().Raw().After("gorm:raw").Register(callback, func(tx *gorm.DB) {
							if tx.Error != nil || !strings.Contains(tx.Statement.SQL.String(), "INSERT INTO archive_export_jobs") {
								return
							}
							observed = true
							probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
							defer cancel()
							probe := db.WithContext(probeCtx).Begin()
							if probe.Error != nil {
								lockErr = probe.Error
								return
							}
							defer probe.Rollback()
							lockErr = probe.Exec("SELECT id FROM applications WHERE id = ? FOR UPDATE NOWAIT", appID).Error
						}))
						defer func() { require.NoError(t, db.Callback().Raw().Remove(callback)) }()
						require.NoError(t, mutate())
						require.True(t, observed, "the lock probe must run at the outbox INSERT boundary")
						var sqlState interface{ SQLState() string }
						require.ErrorAs(t, lockErr, &sqlState)
						require.Equal(t, "55P03", sqlState.SQLState(), "application must be locked before merge updates its links")
					}()
				} else {
					require.NoError(t, mutate())
				}
				require.Equal(t, 1, observer.calls)
				var job models.ArchiveExportJob
				require.NoError(t, db.First(&job, "application_id = ?", appID).Error)
				require.Equal(t, models.ArchiveExportJobPending, job.State)
				require.NoError(t, db.Model(&models.ArchiveExportJob{}).Where("application_id = ?", targetAppID).Count(&count).Error)
				require.Zero(t, count)
				if action == "merge" {
					require.Zero(t, countRows(t, db, table, "id", sourceID))
					require.Equal(t, int64(2), countRows(t, db, "applications", column, targetID))
					require.Equal(t, int64(1), countRows(t, db, "attachments", column, targetID))
				}
			})
		}
	}
}
