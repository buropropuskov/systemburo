package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Публикация новости видна любому авторизованному без отдельного view-права
// (см. TestNews_GetActive_AnyAuthenticated) - поэтому уведомление news_published
// уходит всем активным пользователям, кроме самого автора публикации (#1748).

func TestNews_Notify_PublishedNotifiesOthersNotAuthor(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	otherToken := testutil.RegisterAndLogin(t, e, "newsnotify_other", "password123", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/news", `{"title":"Открыт новый КПП"}`, testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(otherToken))
	require.Equal(t, http.StatusOK, rec.Code)
	items := testutil.ParseSlice(t, rec)
	found := false
	for _, n := range items {
		if n["type"] == "news_published" {
			found = true
			assert.Equal(t, "Опубликована новость", n["title"])
			assert.Equal(t, "Открыт новый КПП", n["message"])
		}
	}
	assert.True(t, found, "другой активный пользователь должен получить уведомление о новости")

	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusOK, rec.Code)
	for _, n := range testutil.ParseSlice(t, rec) {
		assert.NotEqual(t, "news_published", n["type"], "автор новости не должен уведомлять сам себя")
	}
}
