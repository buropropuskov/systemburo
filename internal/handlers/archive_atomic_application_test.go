package handlers_test

import (
	"context"
	"errors"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

// The injected failure precedes the PostgreSQL call: the transaction must roll
// back because the service returns the intent error, not because PostgreSQL
// itself has entered an aborted state.
func TestArchiveAtomicApplication_AssignmentsRollbackOnIntentFailure(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")
	senderID := seedAttachSender(t, db, td.OrgID)
	post := seedCarsTable(t, db, "archive_atomic_post", "Archive atomic post")
	place := seedPlace(t, db, "Archive atomic place")

	for _, kind := range []string{"posts", "places"} {
		t.Run(kind, func(t *testing.T) {
			appID, carID := seedAssignApp(t, db, td.OrgID, senderID, models.StatusInWork, "ATOMIC-"+kind)
			publisher := &fakePublisher{}
			svc := transactionIntegrityService(db, publisher)
			observer := &periodArchive2665Spy{}
			svc.SetBlankExportEnqueuer(observer)
			injected := errors.New("archive intent unavailable")
			calls := applicationSQLIntegrityFault(t, db, false, "insert into archive_export_jobs", 1, injected)
			apply := func(ids []int) error {
				if kind == "posts" {
					return svc.AssignElementTables(context.Background(), "testadmin", appID, services.AssignElementTablesRequest{
						ElementType: "cars", ElementIDs: []int{carID}, TableIDs: ids, Mode: services.AssignModeReplace,
					})
				}
				return svc.AssignCarUnloadPlaces(context.Background(), "testadmin", appID, services.AssignCarUnloadPlacesRequest{
					CarIDs: []int{carID}, PlaceIDs: ids, Mode: services.AssignModeReplace,
				})
			}

			// Empty-to-empty assignment is a no-op even with an unavailable store.
			require.NoError(t, apply(nil))
			require.Zero(t, *calls)
			observer.applicationIDs = nil
			observer.reasons = nil
			before := remainingTransactionSnapshot(t, db)
			id := post
			if kind == "places" {
				id = place
			}
			require.Error(t, apply([]int{id}))
			require.Equal(t, 1, *calls, "the fault must occur after the domain changes")
			require.Equal(t, before, remainingTransactionSnapshot(t, db))
			require.Empty(t, carTableIDs(t, db, carID))
			var placeCount, jobCount int64
			require.NoError(t, db.Model(&models.CarUnloadPlace{}).Where("car_id = ?", carID).Count(&placeCount).Error)
			require.Zero(t, placeCount)
			require.NoError(t, db.Model(&models.ArchiveExportJob{}).Where("application_id = ?", appID).Count(&jobCount).Error)
			require.Zero(t, jobCount)
			require.Empty(t, observer.applicationIDs, "no aftercommit archive observer on rollback")
			require.Empty(t, publisher.events, "no aftercommit realtime event on rollback")
		})
	}
}

func TestArchiveAtomicApplication_CreateAndReadIntent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	const sender = "archive_atomic_sender"
	testutil.RegisterAndLogin(t, e, sender, "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, sender)

	t.Run("create", func(t *testing.T) {
		svc := transactionIntegrityService(db, &fakePublisher{})
		observer := &periodArchive2665Spy{}
		svc.SetBlankExportEnqueuer(observer)
		before := remainingTransactionSnapshot(t, db)
		calls := applicationSQLIntegrityFault(t, db, false, "insert into archive_export_jobs", 1, errors.New("create intent unavailable"))
		response, err := svc.CreateApplication(context.Background(), sender, services.ApplicationCreateRequest{
			ApplicationConfirmationInput: seedApplicationConfirmation(t, db),
			OrganizationID:               &td.OrgID, CompanyID: &td.CompanyID, DataApproval: true,
		}, false)
		require.Error(t, err)
		require.Nil(t, response)
		require.Equal(t, 1, *calls)
		require.Equal(t, before, remainingTransactionSnapshot(t, db))
		require.Empty(t, observer.applicationIDs)
	})

	t.Run("read_transition", func(t *testing.T) {
		appID := suppApp(t, db, td.OrgID, senderID, "ATOMIC-READ", models.ConfirmationPending, models.StatusUnread)
		svc := transactionIntegrityService(db, &fakePublisher{})
		observer := &periodArchive2665Spy{}
		svc.SetBlankExportEnqueuer(observer)
		calls := applicationSQLIntegrityFault(t, db, false, "insert into archive_export_jobs", 1, errors.New("read intent unavailable"))
		_, err := svc.GetApplicationByID(context.Background(), sender, appID, false)
		require.NoError(t, err, "sender read does not change status or need an intent")
		require.Zero(t, *calls)
		require.Empty(t, observer.applicationIDs)
		before := remainingTransactionSnapshot(t, db)
		response, err := svc.GetApplicationByID(context.Background(), "testadmin", appID, true)
		require.Error(t, err)
		require.Nil(t, response)
		require.Equal(t, 1, *calls)
		require.Equal(t, before, remainingTransactionSnapshot(t, db), "reading status, timestamp, audit and seen marker roll back")
		require.Empty(t, observer.applicationIDs)

		// The one-shot injected failure is now spent: retry commits one intent.
		_, err = svc.GetApplicationByID(context.Background(), "testadmin", appID, true)
		require.NoError(t, err)
		require.Equal(t, 2, *calls)
		require.Equal(t, []int{appID}, observer.applicationIDs)
		var job models.ArchiveExportJob
		require.NoError(t, db.Where("application_id = ?", appID).First(&job).Error)
		_, err = svc.GetApplicationByID(context.Background(), "testadmin", appID, true)
		require.NoError(t, err)
		require.Equal(t, 2, *calls, "repeat read does not enqueue")
		require.Equal(t, []int{appID}, observer.applicationIDs)
		var repeated models.ArchiveExportJob
		require.NoError(t, db.Where("application_id = ?", appID).First(&repeated).Error)
		require.Equal(t, job.Revision, repeated.Revision)
	})
}

func TestArchiveAtomicApplication_MutationsRollbackAndRetry(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	transactionIntegrityKey(t)
	td := testutil.SeedTestData(t, db)
	const sender, actor = "archive_atomic_mutation_sender", "archive_atomic_mutation_actor"
	for _, username := range []string{sender, actor} {
		testutil.RegisterAndLogin(t, e, username, "pass123", 1, td.OrgID, td.CompanyID)
	}
	senderID, actorID := getUserID(t, db, sender), getUserID(t, db, actor)
	makeApprover(t, db, actor)
	require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: actorID,
		PermissionKey: services.KeyActionApproveApplication, Value: "allow"}).Error)
	post := seedCarsTable(t, db, "archive_atomic_mutation_post", "Archive atomic mutation post")
	place := seedPlace(t, db, "Archive atomic mutation place")

	for _, operation := range []string{"submit", "withdraw", "approve", "accept_supplement"} {
		t.Run(operation, func(t *testing.T) {
			status, confirmation := models.StatusInWork, models.ConfirmationApproved
			if operation == "approve" {
				status, confirmation = models.StatusProcessing, models.ConfirmationPending
			}
			appID := suppApp(t, db, td.OrgID, senderID, "ATOMIC-"+operation, confirmation, status)
			attID := suppAttachment(t, db, appID, "people", "2099-12-31")
			empID := suppNewEmployee(t, db, attID, "Atomic mutation original", nil)
			require.NoError(t, db.Model(&models.Employee{}).Where("id = ?", empID).Update("status", 1).Error)
			suppResponsible(t, db, appID, actorID, true, "pending")
			round := models.ApplicationSupplement{ApplicationID: appID, Number: 1,
				Status: models.SupplementApproved, CreatedByUserID: senderID}
			require.NoError(t, db.Create(&round).Error)
			pendingID := suppNewEmployee(t, db, attID, "Atomic mutation pending", &round.ID)
			req := services.CompleteApplicationRequest{}
			if operation == "submit" {
				req = transactionIntegrityRequest(t, db, "cars", 1, 0, []int{post}, place)
			}
			publisher := &fakePublisher{}
			svc := transactionIntegrityService(db, publisher)
			observer := &periodArchive2665Spy{}
			svc.SetBlankExportEnqueuer(observer)
			before := remainingTransactionSnapshot(t, db)
			beforeCounts := transactionIntegritySnapshot(t, db)
			var beforeJobs int64
			require.NoError(t, db.Model(&models.ArchiveExportJob{}).Count(&beforeJobs).Error)
			calls := applicationSQLIntegrityFault(t, db, false, "insert into archive_export_jobs", 1, errors.New("mutation intent unavailable"))
			invoke := func() error {
				switch operation {
				case "submit":
					response, err := svc.SubmitCompleteApplication(context.Background(), sender, req, false)
					if err == nil {
						appID = response.ApplicationID
					}
					return err
				case "withdraw":
					return svc.WithdrawApplication(context.Background(), sender, appID)
				case "approve":
					return svc.ApproveApplicationByUser(context.Background(), actor, appID, services.UserApprovalRequest{Status: "approved"})
				default:
					_, err := svc.DecideSupplement(context.Background(), actor, appID, round.ID, services.SupplementDecisionRequest{Action: "accept"})
					return err
				}
			}
			require.Error(t, invoke())
			require.Equal(t, 1, *calls, "failure must reach the outbox after domain writes")
			require.Equal(t, before, remainingTransactionSnapshot(t, db))
			require.Equal(t, beforeCounts, transactionIntegritySnapshot(t, db))
			require.Empty(t, observer.applicationIDs)
			require.Empty(t, publisher.events)
			var jobCount int64
			require.NoError(t, db.Model(&models.ArchiveExportJob{}).Count(&jobCount).Error)
			require.Equal(t, beforeJobs, jobCount)

			require.NoError(t, invoke(), "retry must commit both domain changes and intent")
			require.Equal(t, 2, *calls)
			require.Equal(t, []int{appID}, observer.applicationIDs)
			var job models.ArchiveExportJob
			require.NoError(t, db.Where("application_id = ?", appID).First(&job).Error)
			require.Equal(t, models.ArchiveExportJobPending, job.State)
			var application models.Application
			require.NoError(t, db.First(&application, appID).Error)
			switch operation {
			case "submit":
				var attachments int64
				require.NoError(t, db.Model(&models.Attachment{}).Where("application_id = ?", appID).Count(&attachments).Error)
				require.EqualValues(t, 1, attachments)
			case "withdraw":
				require.Equal(t, models.StatusWithdrawn, *application.Status)
				require.NotNil(t, application.WithdrawnAt)
				require.Zero(t, suppDecEmployeeStatus(t, db, empID))
			case "approve":
				require.Equal(t, models.ConfirmationApproved, *application.Confirmation)
				var vote models.ApplicationResponsibleUser
				require.NoError(t, db.Where("application_id = ? AND user_id = ?", appID, actorID).First(&vote).Error)
				require.Equal(t, "approved", *vote.ApprovalStatus)
			default:
				require.Equal(t, 1, suppDecEmployeeStatus(t, db, pendingID))
				require.NoError(t, db.First(&round, round.ID).Error)
				require.Equal(t, models.SupplementAccepted, round.Status)
			}
		})
	}
}
