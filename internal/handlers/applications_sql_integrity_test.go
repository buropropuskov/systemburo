package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Inject before the driver executes SQL. A PostgreSQL error would abort the
// transaction even if the caller ignored it, concealing the original regression.
func applicationSQLIntegrityFault(t *testing.T, db *gorm.DB, row bool, match string, at int, injected error) *int {
	t.Helper()
	calls := 0
	name := "test:application_sql_integrity:" + strings.ReplaceAll(t.Name(), "/", ":")
	fault := func(tx *gorm.DB) {
		sql := strings.ToLower(strings.Join(strings.Fields(tx.Statement.SQL.String()), " "))
		if strings.Contains(sql, match) {
			calls++
			if calls == at {
				tx.AddError(injected)
			}
		}
	}
	if row {
		require.NoError(t, db.Callback().Row().Before("gorm:row").Register(name, fault))
		t.Cleanup(func() { require.NoError(t, db.Callback().Row().Remove(name)) })
	} else {
		require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(name, fault))
		t.Cleanup(func() { require.NoError(t, db.Callback().Raw().Remove(name)) })
	}
	return &calls
}

func TestApplicationSQLIntegrity_CheckedReadsAndWrites(t *testing.T) {
	e, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	transactionIntegrityKey(t)
	const sender = "sql_integrity_sender"
	token := testutil.RegisterAndLogin(t, e, sender, "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, sender)
	ids := make([]int, 5)
	for i := range ids {
		username := fmt.Sprintf("sql_integrity_recipient_%d", i)
		testutil.RegisterUser(t, e, username, "pass123", 1, td.OrgID, td.CompanyID)
		ids[i] = getUserID(t, db, username)
		if i < 3 {
			require.NoError(t, db.Create(&models.UserPermissionOverride{UserID: ids[i],
				PermissionKey: services.KeyActionApproveApplication, Value: "allow"}).Error)
		}
	}
	common, orgOnly, companyOnly := ids[0], ids[1], ids[2]
	// The common user is optional in the organization and required in the company:
	// the successful path must merge required=true and retain the organization primary.
	require.NoError(t, db.Exec(`INSERT INTO organization_users (organization_id,user_id,is_primary,required_approval)
		VALUES (?, ?, true, false), (?, ?, false, true)`, td.OrgID, common, td.OrgID, orgOnly).Error)
	require.NoError(t, db.Exec(`INSERT INTO companies_users (company_id,user_id,is_primary,required_approval)
		VALUES (?, ?, false, true), (?, ?, true, false)`, td.CompanyID, common, td.CompanyID, companyOnly).Error)
	placeIDs := []int{seedPlace(t, db, "SQL integrity place 1"), seedPlace(t, db, "SQL integrity place 2")}
	carTable := seedCarsTable(t, db, "sql_integrity_cars", "SQL integrity cars")
	req := transactionIntegrityRequest(t, db, "cars", 2, 0, []int{carTable}, placeIDs[0])
	req.OrganizationID, req.CompanyID = &td.OrgID, &td.CompanyID
	for i := range *req.Attachments[0].Data.Vehicles {
		(*req.Attachments[0].Data.Vehicles)[i].UnloadPlaces = append([]int(nil), placeIDs...)
	}
	items := []services.ItemInput{{Name: "SQL fixture item 1", Count: 1}, {Name: "SQL fixture item 2", Count: 2}}
	itemsAtt := req.Attachments[0]
	itemsAtt.AttachmentType = "items"
	itemsAtt.UniqueAttachmentID = seedUniqueAttachment(t, db, "items", "sql_integrity_items", "SQL integrity items")
	itemsAtt.Data = services.AttachmentContentData{Items: &items}
	itemsAtt.UnloadPlaces = append([]int(nil), placeIDs...)
	req.Attachments = append(req.Attachments, itemsAtt)
	readers := []int{ids[3], common, ids[3], ids[4]}
	req.Readers = &readers
	file := uploadDraftFile(t, e, token, "sql_fixture.png", realPNG(t))
	req.FileIDs = []int{file.ID}
	var originalDraft models.ApplicationFile
	require.NoError(t, db.First(&originalDraft, file.ID).Error)
	diskPath := filepath.Join(uploadDir, "application_files", originalDraft.StoredName)
	originalBytes, err := os.ReadFile(diskPath)
	require.NoError(t, err)
	oldID := suppApp(t, db, td.OrgID, senderID, "SQL-INTEGRITY-OLD", models.ConfirmationApproved, models.StatusProcessing)
	oldAttID := suppAttachment(t, db, oldID, "people", "2099-12-31")
	oldEmpID := suppNewEmployee(t, db, oldAttID, "SQL original", nil)
	originalApp := suppReadApplication(t, db, oldID)
	var originalEmployee models.Employee
	require.NoError(t, db.First(&originalEmployee, oldEmpID).Error)
	recorder := services.NewAuditRecorder(db)
	newService := func(pub *fakePublisher) services.ApplicationService {
		return services.NewApplicationService(db, services.NewPermissionService(db), services.NewNotificationService(db),
			services.NewVehicleBlacklistService(db, recorder), services.NewPersonBlacklistService(db, recorder), recorder,
			services.WithRealtimePublisher(pub),
			services.WithApplicationFiles(services.NewApplicationFileService(db, uploadDir, recorder), 30, 100*1024*1024))
	}
	for _, tc := range []struct {
		name, sql string
		row       bool
		at        int
	}{
		{"organization_read", "from organization_users where organization_id =", true, 1},
		{"company_read", "from companies_users where company_id =", true, 1},
		{"primary_update", "update applications set responsible_user_id =", false, 1},
		{"second_responsible", "insert into application_responsible_users", false, 2},
		{"second_viewer", "insert into application_viewers", false, 2},
		{"second_car_attachment_place", "insert into attachment_unload_places", false, 2},
		{"second_item", "insert into items", false, 2},
		// The cars attachment has already emitted two place writes; call four is
		// the second place of items, after both items and the first place exist.
		{"second_item_attachment_place", "insert into attachment_unload_places", false, 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := transactionIntegritySnapshot(t, db)
			fault := errors.New("synthetic pre-SQL integrity error")
			calls := applicationSQLIntegrityFault(t, db, tc.row, tc.sql, tc.at, fault)
			pub := &fakePublisher{}
			resp, err := newService(pub).SubmitCompleteApplication(context.Background(), sender, req, false)
			require.Error(t, err, "SQL read failures must not become empty successful assignee lists")
			require.Nil(t, resp)
			var appErr *apperr.Error
			require.ErrorAs(t, err, &appErr)
			require.Equal(t, http.StatusInternalServerError, appErr.Code)
			require.ErrorIs(t, err, fault, "diagnostic cause must survive the neutral application error")
			require.Equal(t, tc.at, *calls, "fault must hit the intended statement/loop iteration")
			require.Equal(t, before, transactionIntegritySnapshot(t, db), "no partial entities, bindings, audit, viewers or success notifications")
			require.Empty(t, pub.events, "postcommit realtime must not run on failure")
			require.Equal(t, originalApp, suppReadApplication(t, db, oldID))
			var employee models.Employee
			require.NoError(t, db.First(&employee, oldEmpID).Error)
			require.Equal(t, originalEmployee, employee)
			var draft models.ApplicationFile
			require.NoError(t, db.First(&draft, file.ID).Error)
			require.Equal(t, originalDraft, draft, "uploaded file must remain an unchanged unattached draft")
			bytes, err := os.ReadFile(diskPath)
			require.NoError(t, err)
			require.Equal(t, originalBytes, bytes)
		})
	}

	t.Run("success_preserves_assignees_viewers_and_attachment_places", func(t *testing.T) {
		pub := &fakePublisher{}
		svc := newService(pub)
		resp, err := svc.SubmitCompleteApplication(context.Background(), sender, req, false)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotEmpty(t, resp.ApplicationNumber)
		app := suppReadApplication(t, db, resp.ApplicationID)
		require.Equal(t, td.OrgID, app.OrganizationID)
		require.Equal(t, td.CompanyID, *app.CompanyID)
		require.Equal(t, common, *app.ResponsibleUserID, "organization primary wins over company primary")
		var responsible []models.ApplicationResponsibleUser
		require.NoError(t, db.Where("application_id = ?", app.ID).Find(&responsible).Error)
		require.Len(t, responsible, 3, "shared organization/company approver must be deduplicated")
		byUser := map[int]models.ApplicationResponsibleUser{}
		for _, row := range responsible {
			byUser[row.UserID] = row
			require.Equal(t, "pending", *row.ApprovalStatus)
		}
		require.True(t, byUser[common].RequiredApproval, "required flags merge with OR")
		require.True(t, byUser[common].IsPrimary)
		require.True(t, byUser[orgOnly].RequiredApproval)
		require.False(t, byUser[companyOnly].RequiredApproval)
		var viewerIDs []int
		require.NoError(t, db.Table("application_viewers").Where("application_id = ?", app.ID).Pluck("user_id", &viewerIDs).Error)
		require.ElementsMatch(t, ids[3:], viewerIDs, "duplicate readers and existing approvers must not become extra viewers")
		var attachments []models.Attachment
		require.NoError(t, db.Where("application_id = ?", app.ID).Order("id").Find(&attachments).Error)
		require.Len(t, attachments, 2)
		for i, att := range attachments {
			var places []int
			require.NoError(t, db.Table("attachment_unload_places").Where("attachment_id = ?", att.ID).Pluck("unload_place_id", &places).Error)
			require.ElementsMatch(t, placeIDs, places, "both cars union and items attachment places are preserved")
			if i == 0 {
				require.Equal(t, "cars", att.AttachmentType)
				var cars []models.Car
				require.NoError(t, db.Where("attachment_id = ?", att.ID).Order("id").Find(&cars).Error)
				require.Len(t, cars, 2)
				for j, car := range cars {
					require.Equal(t, (*req.Attachments[0].Data.Vehicles)[j].CarNumber, *car.CarNumber)
					var carPlaces []int
					require.NoError(t, db.Table("car_unload_places").Where("car_id = ?", car.ID).Pluck("unload_place_id", &carPlaces).Error)
					require.ElementsMatch(t, placeIDs, carPlaces)
				}
			} else {
				require.Equal(t, "items", att.AttachmentType)
				var stored []models.Item
				require.NoError(t, db.Where("attachment_id = ?", att.ID).Order("id").Find(&stored).Error)
				require.Len(t, stored, 2)
				for j, item := range stored {
					require.Equal(t, items[j].Name, *item.Name)
					require.EqualValues(t, items[j].Count, *item.Count)
				}
			}
		}
		var attached models.ApplicationFile
		require.NoError(t, db.First(&attached, file.ID).Error)
		require.Equal(t, app.ID, *attached.ApplicationID)
		require.NotEmpty(t, pub.events, "successful commit retains realtime refresh")
		// Direct service calls commit durable work; unlike the HTTP fixture,
		// they do not drive the worker after the request completes.
		var intent models.NotificationIntent
		require.NoError(t, db.Where("event_key = ?", fmt.Sprintf("submit:%d", app.ID)).Take(&intent).Error)
		require.Positive(t, intent.ID)
		require.Equal(t, app.ID, intent.ApplicationID)
		require.Equal(t, senderID, intent.ActorUserID)
		require.Equal(t, "pending", intent.State, "business commit must retain the notification intent")
		var notices int64
		require.NoError(t, db.Table("notifications").Where("user_id = ? AND type = ? AND data->>'application_id' = ?",
			senderID, services.NotificationTypeApplicationCreated, fmt.Sprint(app.ID)).Count(&notices).Error)
		require.Zero(t, notices, "committing durable work must not synchronously materialize notifications")
		worker, err := services.NewNotificationIntentWorker(db, svc, services.NewNotificationService(db))
		require.NoError(t, err)
		workCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		processed, failed := worker.Process(workCtx)
		require.Equal(t, 1, processed)
		require.Zero(t, failed)
		var completed models.NotificationIntent
		require.NoError(t, db.Where("event_key = ?", intent.EventKey).Take(&completed).Error)
		require.Equal(t, intent.ID, completed.ID)
		require.Equal(t, "done", completed.State)
		require.NotNil(t, completed.CompletedAt)
		require.NoError(t, db.Table("notifications").Where("user_id = ? AND type = ? AND data->>'application_id' = ?",
			senderID, services.NotificationTypeApplicationCreated, fmt.Sprint(app.ID)).Count(&notices).Error)
		require.EqualValues(t, 1, notices)
	})
}
