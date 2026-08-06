package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Ответ по обращению (#1748) - реальная точка ответа, а не любое обновление статуса:
// уведомление feedback_answered уходит автору обращения только когда записан
// непустой resolution_comment, и не уходит, если отвечает сам автор.

func TestFeedback_Notify_AnsweredNotifiesAuthor(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorToken := testutil.RegisterAndLogin(t, e, "fbnotify_author", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/feedback",
		`{"message":"Не получается прикрепить фото машины к заявке, ошибка 500"}`, testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	feedbackID := int(testutil.ParseMap(t, rec)["id"].(float64))

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/status", feedbackID),
		`{"status":"Решено","comment":"Починили загрузку фото, попробуйте ещё раз"}`, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusOK, rec.Code)
	found := false
	for _, n := range testutil.ParseSlice(t, rec) {
		if n["type"] == "feedback_answered" {
			found = true
			assert.Equal(t, "Ответ по обращению", n["title"])
		}
	}
	assert.True(t, found, "автор обращения должен получить уведомление об ответе")
}

func TestFeedback_Notify_ResolvedWithoutCommentDoesNotNotify(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorToken := testutil.RegisterAndLogin(t, e, "fbnotify_nocomment", "password123", 1, td.OrgID, td.CompanyID)
	rec := testutil.POST(t, e, "/feedback",
		`{"message":"Заявка висит в статусе В обработке уже неделю"}`, testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	feedbackID := int(testutil.ParseMap(t, rec)["id"].(float64))

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/status", feedbackID),
		`{"status":"Решено"}`, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusOK, rec.Code)
	for _, n := range testutil.ParseSlice(t, rec) {
		assert.NotEqual(t, "feedback_answered", n["type"],
			"перевод в 'Решено' без комментария - не ответ, уведомления быть не должно")
	}
}

func TestFeedback_Notify_SelfAnswerNotNotified(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Супер-админ (type_id=6 -> is_super_admin, см. testutil.RegisterUser) пишет
	// обращение сам себе и сам же на него отвечает - вырожденный, но возможный
	// случай (автор обращения одновременно и разборщик).
	token := testutil.RegisterAndLogin(t, e, "fbnotify_selfadmin", "password123", 6, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/feedback",
		`{"message":"Себе на память: перепроверить экспорт отчётов в следующем релизе"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	feedbackID := int(testutil.ParseMap(t, rec)["id"].(float64))

	rec = testutil.PUT(t, e, fmt.Sprintf("/feedback/%d/status", feedbackID),
		`{"status":"Решено","comment":"Перепроверил, всё в порядке"}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code)
	for _, n := range testutil.ParseSlice(t, rec) {
		assert.NotEqual(t, "feedback_answered", n["type"], "отвечая на собственное обращение, уведомлять себя не нужно")
	}
}
