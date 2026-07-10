package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeedback_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	rec := testutil.POST(t, e, "/feedback", `{"message":"test"}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestFeedback_Create(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "fbuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	body := `{"message":"This is a test feedback message for the system"}`
	rec := testutil.POST(t, e, "/feedback", body, h)
	require.Equal(t, http.StatusOK, rec.Code)

	resp := testutil.ParseMap(t, rec)
	assert.NotNil(t, resp["id"])
	assert.Greater(t, int(resp["id"].(float64)), 0)
}

func TestFeedback_Create_TooShort(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "fbuser2", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Less than 10 characters
	rec := testutil.POST(t, e, "/feedback", `{"message":"short"}`, h)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFeedback_GetMy(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "myuser", "password123", 1, td.OrgID, td.CompanyID)
	h := testutil.AuthHeader(token)

	// Create feedback
	rec := testutil.POST(t, e, "/feedback",
		`{"message":"My feedback message for testing purposes"}`, h)
	require.Equal(t, http.StatusOK, rec.Code)

	// Get my feedback
	rec = testutil.GET(t, e, "/feedback/my", h)
	require.Equal(t, http.StatusOK, rec.Code)

	feedbacks := testutil.ParseSlice(t, rec)
	assert.GreaterOrEqual(t, len(feedbacks), 1)

	fb := feedbacks[0]
	assert.Contains(t, fb, "id")
	assert.Contains(t, fb, "message")
	assert.Contains(t, fb, "status")
	assert.Contains(t, fb, "is_read")
	assert.Contains(t, fb, "created_at")
	assert.Equal(t, "Не решено", fb["status"])
	assert.Equal(t, false, fb["is_read"])
}

func TestFeedback_GetAll_AdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Regular user should get 403
	userToken := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/feedback/all", testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Admin should succeed
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, "/feedback/all", testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestFeedback_GetStats_AdminOnly(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Regular user should get 403
	userToken := testutil.RegisterAndLogin(t, e, "statsuser", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/feedback/stats", testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Admin should succeed
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.GET(t, e, "/feedback/stats", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	stats := testutil.ParseMap(t, rec)
	assert.Contains(t, stats, "total")
	assert.Contains(t, stats, "resolved")
	assert.Contains(t, stats, "unresolved")
	assert.Contains(t, stats, "unread")
}

func TestFeedback_UpdateStatus(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Create feedback as regular user
	userToken := testutil.RegisterAndLogin(t, e, "statususer", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/feedback",
		`{"message":"Feedback to update status on"}`,
		testutil.AuthHeader(userToken))
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	fbID := int(createResp["id"].(float64))

	// Admin updates status
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	// Решаем с ответом -- ответ и дата решения сохраняются и возвращаются в списке.
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/status", fbID),
		`{"status":"Решено","comment":"Готово, проверьте раздел Транспорт"}`, adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	got := findFeedbackByID(testutil.ParseSlice(t, testutil.GET(t, e, "/feedback/all", adminH)), fbID)
	require.NotNil(t, got)
	assert.Equal(t, "Решено", got["status"])
	assert.Equal(t, "Готово, проверьте раздел Транспорт", got["resolution_comment"])
	assert.NotNil(t, got["resolved_at"])

	// Возврат в работу очищает ответ и дату решения.
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/status", fbID),
		`{"status":"Не решено"}`, adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	got = findFeedbackByID(testutil.ParseSlice(t, testutil.GET(t, e, "/feedback/all", adminH)), fbID)
	require.NotNil(t, got)
	assert.Nil(t, got["resolution_comment"])
	assert.Nil(t, got["resolved_at"])

	// Regular user cannot update status
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/status", fbID),
		`{"status":"Не решено"}`, testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestFeedback_UpdateStatus_InvalidStatus(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Create feedback
	userToken := testutil.RegisterAndLogin(t, e, "invalidstatus", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/feedback",
		`{"message":"Feedback for invalid status test"}`,
		testutil.AuthHeader(userToken))
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	fbID := int(createResp["id"].(float64))

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/status", fbID),
		`{"status":"InvalidStatus"}`, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestFeedback_UpdateStatus_NotFound(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/feedback/99999/status",
		`{"status":"Решено"}`, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestFeedback_MarkAsRead(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Create feedback
	userToken := testutil.RegisterAndLogin(t, e, "readuser", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/feedback",
		`{"message":"Feedback to mark as read later"}`,
		testutil.AuthHeader(userToken))
	require.Equal(t, http.StatusOK, rec.Code)

	createResp := testutil.ParseMap(t, rec)
	fbID := int(createResp["id"].(float64))

	// Admin marks as read (тело не требуется - персональная фиксация прочтения)
	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)

	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/read", fbID), ``, adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	// Идемпотентно: повторный вызов тоже 200
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/read", fbID), ``, adminH)
	require.Equal(t, http.StatusOK, rec.Code)

	// В списке админа обращение теперь прочитано.
	got := findFeedbackByID(testutil.ParseSlice(t, testutil.GET(t, e, "/feedback/all", adminH)), fbID)
	require.NotNil(t, got)
	assert.Equal(t, true, got["is_read"])

	// Regular user cannot mark as read
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/read", fbID), ``, testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Несуществующее обращение -> 404
	rec = testutil.PUT(t, e, "/feedback/99999/read", ``, adminH)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// TestFeedback_ReadIsPerUser проверяет, что прочтение и счётчик непрочитанных
// персональны: прочтение одним админом не гасит непрочитанность у другого.
func TestFeedback_ReadIsPerUser(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Заявитель создаёт два обращения.
	userToken := testutil.RegisterAndLogin(t, e, "peruser", "password123", 1, td.OrgID, td.CompanyID)
	var ids []int
	for i := 0; i < 2; i++ {
		rec := testutil.POST(t, e, "/feedback",
			fmt.Sprintf(`{"message":"Per-user read feedback number %d here"}`, i),
			testutil.AuthHeader(userToken))
		require.Equal(t, http.StatusOK, rec.Code)
		ids = append(ids, int(testutil.ParseMap(t, rec)["id"].(float64)))
	}

	// Два разных администратора (is_admin, как keeq0).
	adminA := testutil.AuthHeader(testutil.RegisterManager(t, e, "fbadmin_a", td.OrgID, td.CompanyID))
	adminB := testutil.AuthHeader(testutil.RegisterManager(t, e, "fbadmin_b", td.OrgID, td.CompanyID))

	unread := func(h http.Header) float64 {
		rec := testutil.GET(t, e, "/feedback/stats", h)
		require.Equal(t, http.StatusOK, rec.Code)
		return testutil.ParseMap(t, rec)["unread"].(float64)
	}

	beforeA := unread(adminA)
	beforeB := unread(adminB)
	require.GreaterOrEqual(t, beforeA, float64(2))
	assert.Equal(t, beforeA, beforeB, "изначально у обоих админов одинаковый unread")

	// adminA читает первое обращение.
	rec := testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/read", ids[0]), ``, adminA)
	require.Equal(t, http.StatusOK, rec.Code)

	// У adminA непрочитанных на одно меньше, у adminB - без изменений.
	assert.Equal(t, beforeA-1, unread(adminA), "прочтение снижает счётчик читателя")
	assert.Equal(t, beforeB, unread(adminB), "у другого админа счётчик не меняется")

	// В списке is_read персональный: adminA видит #1 прочитанным, adminB - нет.
	gotA := findFeedbackByID(testutil.ParseSlice(t, testutil.GET(t, e, "/feedback/all", adminA)), ids[0])
	gotB := findFeedbackByID(testutil.ParseSlice(t, testutil.GET(t, e, "/feedback/all", adminB)), ids[0])
	require.NotNil(t, gotA)
	require.NotNil(t, gotB)
	assert.Equal(t, true, gotA["is_read"])
	assert.Equal(t, false, gotB["is_read"])
}

// TestFeedback_StatsVisibleToAdmin фиксирует, что администратор (is_admin, не супер)
// имеет доступ к статистике - счётчик обращений в нав-меню должен работать и у него.
func TestFeedback_StatsVisibleToAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminH := testutil.AuthHeader(testutil.RegisterManager(t, e, "fbadmin_stats", td.OrgID, td.CompanyID))
	rec := testutil.GET(t, e, "/feedback/stats", adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, testutil.ParseMap(t, rec), "unread")
}

// TestFeedback_Flag проверяет общий флажок "важное / взять в работу":
// он виден всем администраторам и переключается.
func TestFeedback_Flag(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userToken := testutil.RegisterAndLogin(t, e, "flaguser", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/feedback",
		`{"message":"Feedback to flag as important here"}`,
		testutil.AuthHeader(userToken))
	require.Equal(t, http.StatusOK, rec.Code)
	fbID := int(testutil.ParseMap(t, rec)["id"].(float64))

	adminA := testutil.AuthHeader(testutil.RegisterManager(t, e, "flagadmin_a", td.OrgID, td.CompanyID))
	adminB := testutil.AuthHeader(testutil.RegisterManager(t, e, "flagadmin_b", td.OrgID, td.CompanyID))

	// adminA ставит флажок.
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/flag", fbID), `{"flagged":true}`, adminA)
	require.Equal(t, http.StatusOK, rec.Code)

	// Флажок общий - виден и adminA, и adminB.
	for _, h := range []http.Header{adminA, adminB} {
		got := findFeedbackByID(testutil.ParseSlice(t, testutil.GET(t, e, "/feedback/all", h)), fbID)
		require.NotNil(t, got)
		assert.Equal(t, true, got["flagged"])
	}

	// Снятие флажка.
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/flag", fbID), `{"flagged":false}`, adminB)
	require.Equal(t, http.StatusOK, rec.Code)
	got := findFeedbackByID(testutil.ParseSlice(t, testutil.GET(t, e, "/feedback/all", adminA)), fbID)
	require.NotNil(t, got)
	assert.Equal(t, false, got["flagged"])

	// Обычный пользователь не может ставить флажок.
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/flag", fbID), `{"flagged":true}`, testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code)

	// Несуществующее обращение -> 404.
	rec = testutil.PUT(t, e, "/feedback/99999/flag", `{"flagged":true}`, adminA)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func findFeedbackByID(items []map[string]interface{}, id int) map[string]interface{} {
	for _, it := range items {
		if v, ok := it["id"].(float64); ok && int(v) == id {
			return it
		}
	}
	return nil
}

func TestFeedback_Stats_AfterCreation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userToken := testutil.RegisterAndLogin(t, e, "statscount", "password123", 1, td.OrgID, td.CompanyID)

	// Create 2 feedbacks
	for i := 0; i < 2; i++ {
		rec := testutil.POST(t, e, "/feedback",
			fmt.Sprintf(`{"message":"Stats feedback number %d for testing"}`, i),
			testutil.AuthHeader(userToken))
		require.Equal(t, http.StatusOK, rec.Code)
	}

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/feedback/stats", testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code)

	stats := testutil.ParseMap(t, rec)
	assert.GreaterOrEqual(t, stats["total"].(float64), float64(2))
	assert.GreaterOrEqual(t, stats["unresolved"].(float64), float64(2))
	assert.GreaterOrEqual(t, stats["unread"].(float64), float64(2))
}
