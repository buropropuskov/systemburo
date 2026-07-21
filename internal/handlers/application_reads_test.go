package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func makeApprover(t *testing.T, db *gorm.DB, username string) {
	t.Helper()
	var user models.User
	require.NoError(t, db.Where("username = ?", username).First(&user).Error)
	db.Create(&models.ApplicationApprover{UserID: user.ID})
}

// --- Part 1: Individual Reads ---

func TestMarkAsRead_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	msg := testutil.ParseMessage(t, rec)
	assert.Equal(t, "Application marked as read", msg)
}

func TestMarkAsRead_Idempotent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	// First call
	rec1 := testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec1.Code)

	// Second call — should also succeed (idempotent)
	rec2 := testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID), "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec2.Code)

	// Verify only one read record exists
	var count int64
	db.Model(&models.ApplicationRead{}).Where("application_id = ?", appID).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestMarkAsRead_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/applications/99999/read", "", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetReads_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, adminToken, td.OrgID)

	// Mark as read
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID), "", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	// Get reads
	rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/reads", appID), testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, rec.Code)

	reads := testutil.ParseResponse[[]models.ApplicationReadResponse](t, rec)
	assert.Len(t, reads, 1)
	assert.Equal(t, "testadmin", reads[0].Username)
}

func TestGetReads_Empty(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	appID := createSimpleApplication(t, e, token, td.OrgID)

	rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/reads", appID), testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	reads := testutil.ParseResponse[[]models.ApplicationReadResponse](t, rec)
	assert.Empty(t, reads)
}

func TestGetUnreadCount_Success(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Create two applications
	appID1 := createSimpleApplication(t, e, token, td.OrgID)
	_ = createSimpleApplication(t, e, token, td.OrgID)

	// Check unread count before reading
	rec := testutil.GET(t, e, "/applications/unread-count", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseResponse[models.UnreadCountResponse](t, rec)
	assert.Equal(t, 2, resp.Count)

	// Mark one as read
	testutil.POST(t, e, fmt.Sprintf("/applications/%d/read", appID1), "", testutil.AuthHeader(token))

	// Check unread count after reading one
	rec = testutil.GET(t, e, "/applications/unread-count", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	resp = testutil.ParseResponse[models.UnreadCountResponse](t, rec)
	assert.Equal(t, 1, resp.Count)
}

// --- Part 2: Archive ---

func TestGetApplications_ArchiveDefault_ExcludesArchived(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	// Create an application that will be archived
	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl", "Cars")
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)

	// Make it archived: set status to Завершено and entry_date_to to > 1 month ago
	db.Model(&models.Application{}).Where("id = ?", appID).Update("status", models.StatusCompleted)
	db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01")

	// Create a normal active application
	_ = createSimpleApplication(t, e, token, td.OrgID)

	// Default (no archive param) should exclude archived
	rec := testutil.GET(t, e, "/applications", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	apps := testutil.ParseSlice(t, rec)

	// The archived application should NOT be in the results
	for _, app := range apps {
		assert.NotEqual(t, float64(appID), app["id"], "archived application should be excluded by default")
	}
}

func TestGetApplications_ArchiveTrue_ShowsOnlyArchived(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	// Create an archived application
	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl2", "Cars2")
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
	db.Model(&models.Application{}).Where("id = ?", appID).Update("status", models.StatusCompleted)
	db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01")

	// Create a normal active application
	_ = createSimpleApplication(t, e, token, td.OrgID)

	// archive=true should show only archived
	rec := testutil.GET(t, e, "/applications?archive=true", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	apps := testutil.ParseSlice(t, rec)

	// Should contain only the archived application
	assert.Len(t, apps, 1)
	assert.Equal(t, float64(appID), apps[0]["id"])
}

// Архив должен включать все закрытые статусы (Завершено, Отказано), а не только
// Завершено. Заявка "В работе" с истёкшим вложением остаётся активной.
func TestGetApplications_Archive_IncludesRefused_ExcludesInWork(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	archive := func(name string, status string) int {
		uaID := seedUniqueAttachment(t, db, "cars", "tmpl_"+name, "Disp_"+name)
		appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
		db.Model(&models.Application{}).Where("id = ?", appID).Update("status", status)
		db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01")
		return appID
	}

	completedID := archive("done", models.StatusCompleted)
	refusedID := archive("refused", models.StatusRefused)
	inWorkID := archive("inwork", models.StatusInWork)

	ids := func(apps []map[string]interface{}) []float64 {
		out := make([]float64, 0, len(apps))
		for _, a := range apps {
			if id, ok := a["id"].(float64); ok {
				out = append(out, id)
			}
		}
		return out
	}

	// archive=true: закрытые (Завершено, Отказано), но не "В работе".
	rec := testutil.GET(t, e, "/applications?archive=true", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)
	archived := ids(testutil.ParseSlice(t, rec))
	assert.Contains(t, archived, float64(completedID))
	assert.Contains(t, archived, float64(refusedID))
	assert.NotContains(t, archived, float64(inWorkID))

	// Активные (по умолчанию): "В работе" есть, закрытые исключены.
	recActive := testutil.GET(t, e, "/applications", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, recActive.Code)
	active := ids(testutil.ParseSlice(t, recActive))
	assert.Contains(t, active, float64(inWorkID))
	assert.NotContains(t, active, float64(completedID))
	assert.NotContains(t, active, float64(refusedID))
}

// Правила архива, завязанные на сроки вложений и момент отзыва (#1097 follow-up).
// Один SetupTestApp на все кейсы: пакет handlers - единственный DB-бинарь и уже
// упирается в CI-таймаут, отдельный сетап на каждый кейс стоит дороже самого теста.
func TestGetApplications_ArchiveRules(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")
	adminID := getUserID(t, db, "testadmin")

	// Заявка с одним вложением (по умолчанию действует до 2099) в заданном статусе.
	closedApp := func(name, status string) int {
		uaID := seedUniqueAttachment(t, db, "cars", "tmpl_"+name, "Disp_"+name)
		appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
		require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).
			Update("status", status).Error)
		return appID
	}
	// Второе вложение заявки с заданной датой окончания (nil - бессрочное).
	addAttachment := func(appID int, name string, dateTo *string) {
		label := "Disp " + name
		require.NoError(t, db.Create(&models.Attachment{
			ApplicationID:         &appID,
			AttachmentType:        "cars",
			AttachmentName:        &name,
			AttachmentDisplayName: &label,
			EntryDateTo:           dateTo,
		}).Error)
	}
	expireFirst := func(appID int, dateTo string) {
		require.NoError(t, db.Model(&models.Attachment{}).Where("application_id = ?", appID).
			Update("entry_date_to", dateTo).Error)
	}
	ids := func(query string) []float64 {
		rec := testutil.GET(t, e, query, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)
		apps := testutil.ParseSlice(t, rec)
		out := make([]float64, 0, len(apps))
		for _, a := range apps {
			if id, ok := a["id"].(float64); ok {
				out = append(out, id)
			}
		}
		return out
	}
	ptr := func(s string) *string { return &s }

	t.Run("месяц считается от последнего вложения", func(t *testing.T) {
		appID := closedApp("last_att", models.StatusCompleted)
		expireFirst(appID, "2025-01-01")
		addAttachment(appID, "cars_second", ptr(time.Now().AddDate(0, 0, -1).Format("2006-01-02")))

		assert.NotContains(t, ids("/applications?archive=true"), float64(appID),
			"второе вложение истекло вчера - месяц не прошёл, заявка не архивная")
		assert.Contains(t, ids("/applications"), float64(appID), "пока не архивная - в активных")

		expireFirst(appID, time.Now().AddDate(0, 0, -40).Format("2006-01-02"))
		assert.Contains(t, ids("/applications?archive=true"), float64(appID),
			"все вложения просрочены больше месяца - заявка архивная")
	})

	t.Run("бессрочное вложение держит заявку активной", func(t *testing.T) {
		appID := closedApp("open_ended", models.StatusCompleted)
		expireFirst(appID, "2025-01-01")
		addAttachment(appID, "cars_no_end", nil)

		assert.NotContains(t, ids("/applications?archive=true"), float64(appID))
		assert.Contains(t, ids("/applications"), float64(appID))
	})

	t.Run("отозванная архивируется через месяц после отзыва", func(t *testing.T) {
		// Вложение действует до 2099 - по общему правилу заявка не архивная никогда.
		withdrawn := func(name string, at time.Time) int {
			appID := closedApp(name, models.StatusWithdrawn)
			require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).
				Update("withdrawn_at", at).Error)
			return appID
		}
		freshID := withdrawn("wd_fresh", time.Now().AddDate(0, 0, -3))
		oldID := withdrawn("wd_old", time.Now().AddDate(0, 0, -40))

		archived := ids("/applications?archive=true")
		assert.Contains(t, archived, float64(oldID), "отозвана больше месяца назад - в архиве")
		assert.NotContains(t, archived, float64(freshID), "отозвана 3 дня назад - ещё не в архиве")
		assert.Contains(t, ids("/applications"), float64(freshID))
	})

	t.Run("отозванная без withdrawn_at - по срокам вложений", func(t *testing.T) {
		appID := closedApp("wd_legacy", models.StatusWithdrawn)
		require.NoError(t, db.Model(&models.Application{}).Where("id = ?", appID).
			Update("withdrawn_at", nil).Error)
		expireFirst(appID, "2025-01-01")

		assert.Contains(t, ids("/applications?archive=true"), float64(appID),
			"без withdrawn_at работает старое правило по срокам вложений")
	})

	t.Run("гейт read-only не срабатывает у неархивной заявки", func(t *testing.T) {
		appID := closedApp("gate_mixed", models.StatusCompleted)
		expireFirst(appID, "2025-01-01")
		addAttachment(appID, "cars_gate_second", ptr(time.Now().AddDate(0, 1, 0).Format("2006-01-02")))

		// Проверяем именно архивный гейт по тексту ошибки: 403 у approve может прийти и
		// по другой причине (пользователь не ответственный), она к архиву не относится.
		body := fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, adminID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), body, testutil.AuthHeader(token))
		assert.NotContains(t, rec.Body.String(), "Архивная заявка",
			"заявка с ещё действующим вложением не архивная - гейт срабатывать не должен")
	})
}

// --- Part 3: Active today ---

func TestGetApplications_ActiveToday_FiltersByAttachmentPeriod(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	// Заявка, активная сегодня: период действия вложения охватывает текущую дату.
	uaID := seedUniqueAttachment(t, db, "cars", "cars_today", "CarsToday")
	todayAppID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
	db.Model(&models.Attachment{}).Where("application_id = ?", todayAppID).Updates(map[string]interface{}{
		"entry_date_from": "2025-01-01",
		"entry_date_to":   "2099-12-31",
	})

	// Заявка с прошедшим периодом — попадает в общий список, но не в active_today.
	uaID2 := seedUniqueAttachment(t, db, "cars", "cars_past", "CarsPast")
	pastAppID := submitCompleteApplication(t, e, token, "Test Organization", uaID2)
	db.Model(&models.Attachment{}).Where("application_id = ?", pastAppID).Updates(map[string]interface{}{
		"entry_date_from": "2025-01-01",
		"entry_date_to":   "2025-01-02",
	})

	rec := testutil.GET(t, e, "/applications?active_today=true", testutil.AuthHeader(token))
	assert.Equal(t, http.StatusOK, rec.Code)

	apps := testutil.ParseSlice(t, rec)

	assert.Len(t, apps, 1)
	assert.Equal(t, float64(todayAppID), apps[0]["id"])
}

// Пересылка архивной заявки разрешена с #869 - позитивный кейс в
// applications_forward_archive_test.go (TestForwardApplication_Archived_Allowed).
// Read-only архива остаётся на approve/take-to-work (тесты ниже).

func TestApproveApplication_ArchivedReturns403(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminID := getUserID(t, db, "testadmin")

	// Create and archive an application
	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl4", "Cars4")
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
	db.Model(&models.Application{}).Where("id = ?", appID).Update("status", models.StatusCompleted)
	db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01")

	// Try to approve — should get 403
	body := fmt.Sprintf(`{"user_id":%d,"status":"approved"}`, adminID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/approve", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestTakeToWork_ArchivedReturns403(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminID := getUserID(t, db, "testadmin")

	// Create and archive an application
	uaID := seedUniqueAttachment(t, db, "cars", "cars_tmpl5", "Cars5")
	appID := submitCompleteApplication(t, e, token, "Test Organization", uaID)
	db.Model(&models.Application{}).Where("id = ?", appID).Update("status", models.StatusCompleted)
	db.Model(&models.Attachment{}).Where("application_id = ?", appID).Update("entry_date_to", "2025-01-01")

	// Try to take to work — should get 403
	body := fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, adminID)
	rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), body, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestForwardAttachmentsRead проверяет фильтрацию чтения вложений по получателю пересылки
// (#680): получатель видит только пересланные ему вложения и их теги, отправитель и
// получатель без ограничений - все.
func TestForwardAttachmentsRead(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	// createAppWithTwoCars создаёт заявку с двумя cars-вложениями, возвращает их ID.
	createAppWithTwoCars := func(t *testing.T, token, prefix string) (appID, attID1, attID2 int) {
		t.Helper()
		uaID1 := seedUniqueAttachment(t, db, "cars", prefix+"_a", prefix+" A")
		uaID2 := seedUniqueAttachment(t, db, "cars", prefix+"_b", prefix+" B")
		body := fmt.Sprintf(`{
			"message": "read filter test",
			"organization": "Test Organization",
			"responsible_person": "Test Person",
			"contact_phone": "+79001234567",
			"data_approval": true,
			"attachments": [
				{"attachment_type":"cars","attachment_name":"cars_a","attachment_display_name":"Cars A",
				 "unique_attachment_id":%d,"entry_date_from":"2026-04-01","entry_date_to":"2099-12-31",
				 "entry_time_from":"08:00","entry_time_to":"18:00",
				 "data":{"vehicles":[{"car_number":"B001BB777","car_brand":"Honda"}]}},
				{"attachment_type":"cars","attachment_name":"cars_b","attachment_display_name":"Cars B",
				 "unique_attachment_id":%d,"entry_date_from":"2026-04-01","entry_date_to":"2099-12-31",
				 "entry_time_from":"08:00","entry_time_to":"18:00",
				 "data":{"vehicles":[{"car_number":"C001CC777","car_brand":"Mazda"}]}}
			]
		}`, uaID1, uaID2)
		rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "create: %s", rec.Body.String())
		resp := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec)

		rec = testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", resp.ApplicationID), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)
		atts := testutil.ParseSlice(t, rec)
		require.Len(t, atts, 2)
		return resp.ApplicationID, int(atts[0]["id"].(float64)), int(atts[1]["id"].(float64))
	}

	attachmentIDs := func(t *testing.T, token string, appID int) []int {
		t.Helper()
		rec := testutil.GET(t, e, fmt.Sprintf("/applications/%d/attachments", appID), testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)
		out := make([]int, 0)
		for _, a := range testutil.ParseSlice(t, rec) {
			out = append(out, int(a["id"].(float64)))
		}
		return out
	}

	findApp := func(apps []map[string]interface{}, id int) map[string]interface{} {
		for _, a := range apps {
			if int(a["id"].(float64)) == id {
				return a
			}
		}
		return nil
	}

	t.Run("recipient_sees_only_forwarded_attachments", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fr_sender1", "pass123", 1, td.OrgID, td.CompanyID)
		appID, attID1, attID2 := createAppWithTwoCars(t, senderToken, "fr_cars1")

		respToken := testutil.RegisterAndLogin(t, e, "fr_resp1", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "fr_resp1")

		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}],"attachment_ids":[%d]}`, respID, attID1)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		assert.Equal(t, []int{attID1}, attachmentIDs(t, respToken, appID), "получатель видит только пересланное вложение")
		assert.ElementsMatch(t, []int{attID1, attID2}, attachmentIDs(t, senderToken, appID), "отправитель видит все")
	})

	t.Run("recipient_without_subset_sees_all", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fr_sender2", "pass123", 1, td.OrgID, td.CompanyID)
		appID, attID1, attID2 := createAppWithTwoCars(t, senderToken, "fr_cars2")

		respToken := testutil.RegisterAndLogin(t, e, "fr_resp2", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "fr_resp2")

		// Пересыл без attachment_ids -> строк нет -> получатель видит все (обратная совместимость).
		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}]}`, respID)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		assert.ElementsMatch(t, []int{attID1, attID2}, attachmentIDs(t, respToken, appID))
	})

	t.Run("detail_endpoints_403_for_hidden_attachment", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fr_sender3", "pass123", 1, td.OrgID, td.CompanyID)
		appID, attID1, attID2 := createAppWithTwoCars(t, senderToken, "fr_cars3")

		respToken := testutil.RegisterAndLogin(t, e, "fr_resp3", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "fr_resp3")

		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}],"attachment_ids":[%d]}`, respID, attID1)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		recHidden := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID2), testutil.AuthHeader(respToken))
		assert.Equal(t, http.StatusForbidden, recHidden.Code, "скрытое вложение -> 403")

		recVisible := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID1), testutil.AuthHeader(respToken))
		assert.Equal(t, http.StatusOK, recVisible.Code, "пересланное вложение доступно")

		recSender := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/cars", attID2), testutil.AuthHeader(senderToken))
		assert.Equal(t, http.StatusOK, recSender.Code, "отправитель видит любое вложение")
	})

	t.Run("tags_reflect_only_visible_attachments", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fr_sender4", "pass123", 1, td.OrgID, td.CompanyID)
		appID, attID1, attID2 := createAppWithTwoCars(t, senderToken, "fr_cars4")

		// Теги только на СКРЫТОМ вложении: получатель не должен их увидеть.
		require.NoError(t, db.Model(&models.Attachment{}).Where("id = ?", attID2).
			Updates(map[string]interface{}{"roof_access": true, "free_parking": true}).Error)

		respToken := testutil.RegisterAndLogin(t, e, "fr_resp4", "pass123", 1, td.OrgID, td.CompanyID)
		respID := getUserID(t, db, "fr_resp4")

		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}],"attachment_ids":[%d]}`, respID, attID1)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		recResp := testutil.GET(t, e, "/applications", testutil.AuthHeader(respToken))
		require.Equal(t, http.StatusOK, recResp.Code)
		respApp := findApp(testutil.ParseSlice(t, recResp), appID)
		require.NotNil(t, respApp, "получатель видит заявку в Центре")
		assert.Equal(t, false, respApp["has_roof_access"], "тег скрытого вложения не виден получателю")
		assert.Equal(t, false, respApp["has_free_parking"], "тег скрытого вложения не виден получателю")

		recSender := testutil.GET(t, e, "/applications", testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, recSender.Code)
		senderApp := findApp(testutil.ParseSlice(t, recSender), appID)
		require.NotNil(t, senderApp)
		assert.Equal(t, true, senderApp["has_roof_access"], "отправитель видит тег")
		assert.Equal(t, true, senderApp["has_free_parking"], "отправитель видит тег")
	})

	t.Run("superadmin_recipient_sees_all_tags", func(t *testing.T) {
		testutil.CleanDB(t, db)
		td := testutil.SeedTestData(t, db)

		senderToken := testutil.RegisterAndLogin(t, e, "fr_sender5", "pass123", 1, td.OrgID, td.CompanyID)
		appID, attID1, attID2 := createAppWithTwoCars(t, senderToken, "fr_cars5")

		require.NoError(t, db.Model(&models.Attachment{}).Where("id = ?", attID2).
			Updates(map[string]interface{}{"roof_access": true, "free_parking": true}).Error)

		// Получатель с ограниченным набором, но он же супер-админ -> видит все теги (bypass).
		superToken := testutil.RegisterAndLogin(t, e, "fr_super5", "pass123", 1, td.OrgID, td.CompanyID)
		superID := getUserID(t, db, "fr_super5")
		require.NoError(t, db.Model(&models.User{}).Where("id = ?", superID).Update("is_super_admin", true).Error)

		body := fmt.Sprintf(`{"users":[{"user_id":%d,"required_approval":true,"can_view":false}],"attachment_ids":[%d]}`, superID, attID1)
		rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/forward", appID), body, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "forward: %s", rec.Body.String())

		recSuper := testutil.GET(t, e, "/applications", testutil.AuthHeader(superToken))
		require.Equal(t, http.StatusOK, recSuper.Code)
		superApp := findApp(testutil.ParseSlice(t, recSuper), appID)
		require.NotNil(t, superApp, "супер-админ видит заявку")
		assert.Equal(t, true, superApp["has_roof_access"], "супер-админ не ограничивается пересылом - видит все теги")
		assert.Equal(t, true, superApp["has_free_parking"], "супер-админ не ограничивается пересылом - видит все теги")
	})
}

// --- Unauthorized tests for new endpoints ---

func TestReads_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	rec := testutil.POST(t, e, "/applications/1/read", "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = testutil.GET(t, e, "/applications/1/reads", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = testutil.GET(t, e, "/applications/unread-count", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
