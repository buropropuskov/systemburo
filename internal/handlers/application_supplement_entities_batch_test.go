package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSupplementEntitiesBatch_BoundariesMappingAndRollback(t *testing.T) {
	e, originalDB, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, originalDB)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	pool, err := originalDB.DB()
	require.NoError(t, err)
	// The HTTP app retains its original registry for asynchronous PDAudit.
	// Fault callbacks and the observed service use this independent GORM registry.
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{Logger: originalDB.Logger, NowFunc: func() time.Time { return now }})
	require.NoError(t, err)
	td := testutil.SeedTestData(t, db)
	authorID, token := suppAuthor(t, e, db, "entities_batch_author", td.OrgID, td.CompanyID)
	voterID, _ := suppAuthor(t, e, db, "entities_batch_voter", td.OrgID, td.CompanyID)
	postIDs := []int{seedCarsTable(t, db, "entities_post_1", "Entities post 1"), seedCarsTable(t, db, "entities_post_2", "Entities post 2")}
	placeIDs := []int{seedPlace(t, db, "Entities unload 1"), seedPlace(t, db, "Entities unload 2")}
	number := 0
	fixture := func(t *testing.T, status string, cars, items int, nullDates, specialPlates bool) (int, int, int, int, int, services.CreateSupplementRequest) {
		t.Helper()
		number++
		appID := suppApp(t, db, td.OrgID, authorID, fmt.Sprintf("ENTITIES-%02d", number), models.ConfirmationApproved, status)
		carsAtt := suppAttachment(t, db, appID, "cars", "2099-12-31")
		itemsAtt := suppAttachment(t, db, appID, "items", "2099-12-31")
		if nullDates {
			require.NoError(t, db.Model(&models.Attachment{}).Where("id = ?", carsAtt).Updates(map[string]interface{}{
				"entry_date_from": nil, "entry_date_to": nil, "entry_time_from": nil, "entry_time_to": nil,
			}).Error)
		}
		oldCar := models.Car{AttachmentID: carsAtt, CarNumber: testutil.Ptr("ORIGINAL"), CarBrand: testutil.Ptr("Original brand"), Status: testutil.Ptr(1)}
		require.NoError(t, db.Create(&oldCar).Error)
		require.NoError(t, db.Create(&models.CarTargetTable{CarID: oldCar.ID, TableID: postIDs[0]}).Error)
		require.NoError(t, db.Create(&models.CarUnloadPlace{CarID: oldCar.ID, UnloadPlaceID: placeIDs[0]}).Error)
		oldItem := models.Item{AttachmentID: itemsAtt, Name: testutil.Ptr("Original item"), Count: testutil.Ptr(7), DateCreated: &now}
		require.NoError(t, db.Create(&oldItem).Error)
		require.NoError(t, db.Create(&models.ApplicationResponsibleUser{ApplicationID: appID, UserID: voterID, RequiredApproval: true,
			ApprovalStatus: testutil.Ptr("approved"), ApprovalComment: testutil.Ptr("Existing approval"), ApprovalDatetime: &now, CreatedAt: now}).Error)
		req := services.CreateSupplementRequest{}
		if cars > 0 {
			rows := make([]services.VehicleInput, cars)
			for i := range rows {
				rows[i] = services.VehicleInput{CarNumber: fmt.Sprintf("E%06d", i), CarBrand: fmt.Sprintf("Brand-%04d", i),
					UnloadPlace: testutil.Ptr(fmt.Sprintf("Legacy place %d", i)), PDConsent: i%2 == 0}
				switch i % 3 {
				case 0:
					rows[i].TargetTables, rows[i].UnloadPlaces = []int{postIDs[0], postIDs[1], postIDs[0]}, []int{placeIDs[0], placeIDs[1], placeIDs[0]}
				case 1:
					rows[i].TargetTables, rows[i].UnloadPlaces = postIDs[1:], placeIDs[1:]
				}
			}
			if specialPlates {
				rows[0].CarNumber, rows[1].CarNumber, rows[2].CarNumber = "По факту", "", ""
			}
			req.Additions = append(req.Additions, services.SupplementAddition{AttachmentID: carsAtt, Vehicles: rows})
		}
		if items > 0 {
			rows := make([]services.ItemInput, items)
			for i := range rows {
				rows[i] = services.ItemInput{Name: fmt.Sprintf("Item-%04d", i), Count: i + 1, OrderIndex: items - i}
			}
			// Same item name is allowed. Identity/order must not be reconstructed
			// by a name map or by the optional, historically unused OrderIndex.
			if items > 1 {
				rows[1].Name = rows[0].Name
			}
			req.Additions = append(req.Additions, services.SupplementAddition{AttachmentID: itemsAtt, Items: rows})
		}
		return appID, carsAtt, itemsAtt, oldCar.ID, oldItem.ID, req
	}
	for _, tc := range []struct {
		status             string
		cars, items        int
		nullDates, special bool
	}{
		{models.StatusProcessing, 500, 501, false, false}, {models.StatusInWork, 501, 500, false, false},
		{models.StatusProcessing, 2000, 0, false, false}, {models.StatusInWork, 0, 2000, false, false},
		{models.StatusProcessing, 4, 3, true, true},
	} {
		t.Run(fmt.Sprintf("success_%s_cars%d_items%d_null=%t", tc.status, tc.cars, tc.items, tc.nullDates), func(t *testing.T) {
			appID, carsAtt, itemsAtt, oldCarID, oldItemID, req := fixture(t, tc.status, tc.cars, tc.items, tc.nullDates, tc.special)
			beforeApp := suppReadApplication(t, db, appID)
			var oldCar models.Car
			var oldItem models.Item
			var oldVote models.ApplicationResponsibleUser
			require.NoError(t, db.First(&oldCar, oldCarID).Error)
			require.NoError(t, db.First(&oldItem, oldItemID).Error)
			require.NoError(t, db.Where("application_id = ?", appID).First(&oldVote).Error)
			counter := newSQLCounter("INSERT INTO cars", "INSERT INTO items")
			observedDB := db.Session(&gorm.Session{Logger: counter})
			pub := &fakePublisher{}
			response, err := transactionIntegrityService(observedDB, pub).CreateSupplement(context.Background(), "entities_batch_author", appID, false, req)
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, tc.cars, response.Counts.Vehicles)
			require.Equal(t, tc.items, response.Counts.Items)
			require.Equal(t, (tc.cars+499)/500, counter.get("INSERT INTO cars"))
			require.Equal(t, (tc.items+499)/500, counter.get("INSERT INTO items"))
			t.Logf("entity INSERT statements cars=%d items=%d; no elapsed-time claim", counter.get("INSERT INTO cars"), counter.get("INSERT INTO items"))
			round := suppReadRound(t, db, appID)
			wantRound := models.SupplementMerged
			if tc.status == models.StatusInWork {
				wantRound = models.SupplementPending
			}
			require.Equal(t, wantRound, round.Status)
			require.Equal(t, round.Status, response.Status)
			var cars []models.Car
			var items []models.Item
			require.NoError(t, db.Where("supplement_id = ?", round.ID).Order("id").Find(&cars).Error)
			require.NoError(t, db.Where("supplement_id = ?", round.ID).Order("id").Find(&items).Error)
			require.Len(t, cars, tc.cars)
			require.Len(t, items, tc.items)
			var inputCars []services.VehicleInput
			var inputItems []services.ItemInput
			for _, addition := range req.Additions {
				inputCars = append(inputCars, addition.Vehicles...)
				inputItems = append(inputItems, addition.Items...)
			}
			var carAtt models.Attachment
			require.NoError(t, db.First(&carAtt, carsAtt).Error)
			var targetLinks []models.CarTargetTable
			var unloadLinks []models.CarUnloadPlace
			ownCarIDs := db.Model(&models.Car{}).Select("id").Where("supplement_id = ?", round.ID)
			require.NoError(t, db.Where("car_id IN (?)", ownCarIDs).Find(&targetLinks).Error)
			require.NoError(t, db.Where("car_id IN (?)", ownCarIDs).Find(&unloadLinks).Error)
			postByCar, placeByCar := map[int][]int{}, map[int][]int{}
			for _, link := range targetLinks {
				require.Equal(t, "application", link.Source)
				require.NotNil(t, link.OrderIndex)
				require.Equal(t, 1, *link.OrderIndex)
				postByCar[link.CarID] = append(postByCar[link.CarID], link.TableID)
			}
			for _, link := range unloadLinks {
				require.NotNil(t, link.OrderIndex)
				require.Equal(t, 1, *link.OrderIndex)
				placeByCar[link.CarID] = append(placeByCar[link.CarID], link.UnloadPlaceID)
			}
			var history []models.AuditLog
			require.NoError(t, db.Where("entity_type = ? AND action = ? AND entity_id IN (?)", models.AuditEntityCar, "create", ownCarIDs).Find(&history).Error)
			require.Len(t, history, tc.cars)
			historyByCar := map[int]models.AuditLog{}
			for _, entry := range history {
				historyByCar[*entry.EntityID] = entry
			}
			for i, car := range cars {
				input := inputCars[i]
				require.NotEqual(t, oldCarID, car.ID)
				require.Equal(t, carsAtt, car.AttachmentID)
				require.Equal(t, round.ID, *car.SupplementID)
				require.Equal(t, input.CarNumber, *car.CarNumber)
				require.Equal(t, input.CarBrand, *car.CarBrand)
				require.Equal(t, input.UnloadPlace, car.UnloadPlace)
				require.Equal(t, 0, *car.Status)
				require.Equal(t, carAtt.EntryDateFrom, car.EntryDateFrom)
				require.Equal(t, carAtt.EntryDateTo, car.EntryDateTo)
				require.Equal(t, carAtt.EntryTimeFrom, car.EntryTimeFrom)
				require.Equal(t, carAtt.EntryTimeTo, car.EntryTimeTo)
				if input.PDConsent {
					require.NotNil(t, car.PDConsentAt)
					require.True(t, car.PDConsentAt.Equal(round.CreatedAt))
					require.Equal(t, authorID, *car.PDConsentByUserID)
				} else {
					require.Nil(t, car.PDConsentAt)
					require.Nil(t, car.PDConsentByUserID)
				}
				require.ElementsMatch(t, input.TargetTables, postByCar[car.ID])
				require.ElementsMatch(t, input.UnloadPlaces, placeByCar[car.ID])
				var details struct {
					Comment string `json:"comment"`
				}
				require.NoError(t, json.Unmarshal(historyByCar[car.ID].Details, &details))
				require.Contains(t, details.Comment, input.CarBrand, "audit must use the ID of its input row even when plates repeat")
			}
			for i, item := range items {
				require.NotEqual(t, oldItemID, item.ID)
				require.Equal(t, itemsAtt, item.AttachmentID)
				require.Equal(t, round.ID, *item.SupplementID)
				require.Equal(t, inputItems[i].Name, *item.Name)
				require.Equal(t, inputItems[i].Count, *item.Count)
				require.NotNil(t, item.DateCreated)
				require.Equal(t, round.CreatedAt.UTC().Format("2006-01-02"), item.DateCreated.UTC().Format("2006-01-02"))
			}
			if tc.cars > 0 {
				var union []int
				require.NoError(t, db.Table("attachment_unload_places").Where("attachment_id = ?", carsAtt).Pluck("unload_place_id", &union).Error)
				require.ElementsMatch(t, placeIDs, union, "repeated places are deduplicated only at attachment union level")
			}
			var currentCar models.Car
			var currentItem models.Item
			require.NoError(t, db.First(&currentCar, oldCarID).Error)
			require.NoError(t, db.First(&currentItem, oldItemID).Error)
			require.Equal(t, oldCar, currentCar)
			require.Equal(t, oldItem, currentItem)
			var oldPostLinks, oldUnloadLinks int64
			require.NoError(t, db.Model(&models.CarTargetTable{}).Where("car_id = ?", oldCarID).Count(&oldPostLinks).Error)
			require.NoError(t, db.Model(&models.CarUnloadPlace{}).Where("car_id = ?", oldCarID).Count(&oldUnloadLinks).Error)
			require.EqualValues(t, 1, oldPostLinks)
			require.EqualValues(t, 1, oldUnloadLinks)
			app := suppReadApplication(t, db, appID)
			require.Equal(t, beforeApp.Status, app.Status)
			var vote models.ApplicationResponsibleUser
			require.NoError(t, db.Where("application_id = ?", appID).First(&vote).Error)
			if tc.status == models.StatusInWork {
				require.Equal(t, oldVote, vote)
				require.Equal(t, beforeApp.Confirmation, app.Confirmation)
			} else {
				require.Equal(t, "pending", *vote.ApprovalStatus)
			}
			// Normal HTTP readers use the original DB and retain PDAudit.
			for _, endpoint := range []struct {
				attID, count int
				kind         string
			}{{carsAtt, tc.cars + 1, "cars"}, {itemsAtt, tc.items + 1, "items"}} {
				rec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/%s", endpoint.attID, endpoint.kind), testutil.AuthHeader(token))
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				require.Len(t, testutil.ParseSlice(t, rec), endpoint.count)
			}
			require.NotEmpty(t, pub.events)
		})
	}
	for _, status := range []string{models.StatusProcessing, models.StatusInWork} {
		for _, table := range []string{"cars", "items"} {
			t.Run("rollback_"+status+"_"+table, func(t *testing.T) {
				cars, items := 501, 2
				if table == "items" {
					cars, items = 2, 501
				}
				appID, _, _, _, _, req := fixture(t, status, cars, items, false, false)
				before, countsBefore := remainingTransactionSnapshot(t, db), transactionIntegritySnapshot(t, db)
				name := "test:entities_second_batch:" + strings.ReplaceAll(t.Name(), "/", ":")
				hits := 0
				require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(name, func(tx *gorm.DB) {
					sql := strings.ToLower(strings.Join(strings.Fields(tx.Statement.SQL.String()), " "))
					if !strings.HasPrefix(sql, "insert into "+table+" ") {
						return
					}
					hits++
					if hits != 2 {
						return
					}
					var count int64
					require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Table(table).Count(&count).Error)
					require.Equal(t, countsBefore[table]+500, count)
					if table == "items" {
						var carsWritten int64
						require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Table("cars").Count(&carsWritten).Error)
						require.Equal(t, countsBefore["cars"]+2, carsWritten, "late item failure follows saved cars and their links/history")
						var newLinks int64
						require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Table("car_target_tables").Count(&newLinks).Error)
						require.Equal(t, countsBefore["car_target_tables"]+4, newLinks)
						var carHistory int64
						require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Table("audit_log").Where(
							"entity_type = ? AND action = ? AND entity_id IN (SELECT id FROM cars WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?) AND supplement_id IS NOT NULL)",
							models.AuditEntityCar, "create", appID).Count(&carHistory).Error)
						require.EqualValues(t, 2, carHistory)
					}
					tx.AddError(errors.New("synthetic second entity INSERT failure"))
				}))
				t.Cleanup(func() { require.NoError(t, db.Callback().Raw().Remove(name)) })
				pub := &fakePublisher{}
				response, err := transactionIntegrityService(db, pub).CreateSupplement(context.Background(), "entities_batch_author", appID, false, req)
				require.Error(t, err)
				require.Nil(t, response)
				require.Equal(t, 2, hits)
				require.Equal(t, before, remainingTransactionSnapshot(t, db))
				require.Equal(t, countsBefore, transactionIntegritySnapshot(t, db))
				require.Empty(t, pub.events)
			})
		}
	}
	t.Run("cancel_context_before_second_car_batch_rolls_back_first", func(t *testing.T) {
		appID, _, _, _, _, req := fixture(t, models.StatusProcessing, 501, 2, false, false)
		before, countsBefore := remainingTransactionSnapshot(t, db), transactionIntegritySnapshot(t, db)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		hits := 0
		name := "test:entities_cancel_second_batch"
		require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register(name, func(tx *gorm.DB) {
			sql := strings.ToLower(strings.Join(strings.Fields(tx.Statement.SQL.String()), " "))
			if !strings.HasPrefix(sql, "insert into cars ") {
				return
			}
			hits++
			if hits != 2 {
				return
			}
			var count int64
			require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Model(&models.Car{}).Count(&count).Error)
			require.Equal(t, countsBefore["cars"]+500, count, "first entity batch must already exist in the transaction")
			cancel() // The real driver observes cancellation; no AddError failpoint.
		}))
		t.Cleanup(func() { require.NoError(t, db.Callback().Raw().Remove(name)) })
		pub := &fakePublisher{}
		response, err := transactionIntegrityService(db, pub).CreateSupplement(ctx, "entities_batch_author", appID, false, req)
		require.Error(t, err)
		require.Nil(t, response)
		require.ErrorIs(t, ctx.Err(), context.Canceled)
		require.Equal(t, 2, hits)
		require.Equal(t, before, remainingTransactionSnapshot(t, db))
		require.Equal(t, countsBefore, transactionIntegritySnapshot(t, db))
		require.Empty(t, pub.events)
	})
	t.Run("concurrent_applications_keep_sequence_ids_and_input_mapping", func(t *testing.T) {
		type work struct {
			appID, carsAtt, itemsAtt, oldCarID, oldItemID int
			req                                           services.CreateSupplementRequest
			svc                                           services.ApplicationService
		}
		type result struct {
			index    int
			response *services.CreateSupplementResponse
			err      error
		}
		jobs := make([]work, 2)
		ready := make(chan struct{}, 2)
		release := make(chan struct{})
		results := make(chan result, 2)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for i := range jobs {
			status := models.StatusProcessing
			if i == 1 {
				status = models.StatusInWork
			}
			appID, carsAtt, itemsAtt, oldCarID, oldItemID, req := fixture(t, status, 7, 9, false, false)
			for a := range req.Additions {
				for c := range req.Additions[a].Vehicles {
					req.Additions[a].Vehicles[c].CarNumber = fmt.Sprintf("C%d%06d", i, c)
					req.Additions[a].Vehicles[c].CarBrand = fmt.Sprintf("Concurrent-%d-brand-%d", i, c)
				}
				for item := range req.Additions[a].Items {
					req.Additions[a].Items[item].Name = fmt.Sprintf("Concurrent-%d-item-%d", i, item)
				}
			}
			privateDB, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{Logger: originalDB.Logger, NowFunc: func() time.Time { return now }})
			require.NoError(t, err)
			// Register before starting either worker, and remove only after both
			// workers finish. Each registry is private to one synchronous service.
			name := fmt.Sprintf("test:entities_sequence_barrier_%d", i)
			synchronized := false
			require.NoError(t, privateDB.Callback().Row().Before("gorm:row").Register(name, func(tx *gorm.DB) {
				if synchronized || !strings.Contains(strings.ToLower(tx.Statement.SQL.String()), "generate_series(") {
					return
				}
				synchronized = true
				ready <- struct{}{}
				select {
				case <-release:
				case <-ctx.Done():
					tx.AddError(ctx.Err())
				}
			}))
			t.Cleanup(func() { require.NoError(t, privateDB.Callback().Row().Remove(name)) })
			recorder := services.NewAuditRecorder(privateDB)
			svc := services.NewApplicationService(privateDB, services.NewPermissionService(privateDB), services.NewNotificationService(privateDB),
				services.NewVehicleBlacklistService(privateDB, recorder), services.NewPersonBlacklistService(privateDB, recorder), recorder)
			jobs[i] = work{appID, carsAtt, itemsAtt, oldCarID, oldItemID, req, svc}
		}
		for i, job := range jobs {
			go func(index int, job work) {
				response, err := job.svc.CreateSupplement(ctx, "entities_batch_author", job.appID, false, job.req)
				results <- result{index, response, err}
			}(i, job)
		}
		completed := []result{}
		readyCount := 0
		// An early service error releases the other worker instead of leaving
		// it blocked at the barrier. No testing assertions run in goroutines.
		for readyCount < 2 && len(completed) == 0 {
			select {
			case <-ready:
				readyCount++
			case done := <-results:
				completed = append(completed, done)
			}
		}
		close(release)
		for len(completed) < 2 {
			completed = append(completed, <-results)
		}
		require.Equal(t, 2, readyCount, "both operations must reach sequence reservation before either is released")
		carOwners, itemOwners := map[int]int{}, map[int]int{}
		for _, done := range completed {
			require.NoError(t, done.err)
			require.NotNil(t, done.response)
			job := jobs[done.index]
			var cars []models.Car
			var items []models.Item
			require.NoError(t, db.Where("supplement_id = ?", done.response.SupplementID).Order("id").Find(&cars).Error)
			require.NoError(t, db.Where("supplement_id = ?", done.response.SupplementID).Order("id").Find(&items).Error)
			require.Len(t, cars, 7)
			require.Len(t, items, 9)
			for i, car := range cars {
				_, duplicate := carOwners[car.ID]
				require.False(t, duplicate, "car ID must be unique across concurrent applications")
				carOwners[car.ID] = job.appID
				require.NotEqual(t, job.oldCarID, car.ID)
				require.Equal(t, job.carsAtt, car.AttachmentID)
				require.Equal(t, job.req.Additions[0].Vehicles[i].CarNumber, *car.CarNumber)
				require.Equal(t, job.req.Additions[0].Vehicles[i].CarBrand, *car.CarBrand)
			}
			for i, item := range items {
				_, duplicate := itemOwners[item.ID]
				require.False(t, duplicate, "item ID must be unique across concurrent applications")
				itemOwners[item.ID] = job.appID
				require.NotEqual(t, job.oldItemID, item.ID)
				require.Equal(t, job.itemsAtt, item.AttachmentID)
				require.Equal(t, job.req.Additions[1].Items[i].Name, *item.Name)
				require.Equal(t, job.req.Additions[1].Items[i].Count, *item.Count)
			}
		}
		require.Len(t, carOwners, 14)
		require.Len(t, itemOwners, 18)
	})
	t.Run("pre_cancelled_context_preserves_composition", func(t *testing.T) {
		appID, _, _, _, _, req := fixture(t, models.StatusInWork, 2, 2, false, false)
		before, countsBefore := remainingTransactionSnapshot(t, db), transactionIntegritySnapshot(t, db)
		counter := newSQLCounter("INSERT INTO cars", "INSERT INTO items")
		observedDB := db.Session(&gorm.Session{Logger: counter})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		pub := &fakePublisher{}
		response, err := transactionIntegrityService(observedDB, pub).CreateSupplement(ctx, "entities_batch_author", appID, false, req)
		require.Error(t, err)
		require.Nil(t, response)
		require.Zero(t, counter.get("INSERT INTO cars"))
		require.Zero(t, counter.get("INSERT INTO items"))
		require.Equal(t, before, remainingTransactionSnapshot(t, db))
		require.Equal(t, countsBefore, transactionIntegritySnapshot(t, db))
		require.Empty(t, pub.events)
	})
}
