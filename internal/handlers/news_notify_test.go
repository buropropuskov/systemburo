package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Уведомление news_published уходит активным пользователям, кроме автора.
// Подробная проекция требует текущего page.news; отзыв оставляет generic запись.

func TestNews_Notify_PublishedNotifiesOthersNotAuthor(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	otherToken := testutil.RegisterAndLogin(t, e, "newsnotify_other", "password123", 1, td.OrgID, td.CompanyID)
	otherID := getUserID(t, db, "newsnotify_other")
	ctx := context.Background()
	resolver := services.NewPermissionResolver(db)
	permissions, err := resolver.ResolveFresh(ctx, otherID)
	require.NoError(t, err)
	require.False(t, permissions.Has(services.KeyPageNews), "plain Type1 fixture must not implicitly grant %s: %+v", services.KeyPageNews, permissions)
	newsAccess := models.UserPermissionOverride{UserID: otherID, PermissionKey: services.KeyPageNews, Value: "allow"}
	require.NoError(t, db.Create(&newsAccess).Error)
	permissions, err = resolver.ResolveFresh(ctx, otherID)
	require.NoError(t, err)
	require.True(t, permissions.Has(services.KeyPageNews), "positive projection fixture requires %s: %+v", services.KeyPageNews, permissions)

	rec := testutil.POST(t, e, "/news", `{"title":"Открыт новый КПП"}`, testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := testutil.ParseMap(t, rec)
	var stored models.Notification
	require.NoError(t, db.Where("user_id = ? AND type = ?", otherID, services.NotificationTypeNewsPublished).Take(&stored).Error)
	assert.Equal(t, "Есть обновление", *stored.Title)
	assert.Equal(t, "Есть обновление", *stored.Message)
	require.NotNil(t, stored.ProjectionRef)
	var reference struct {
		NewsID int `json:"news_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(*stored.ProjectionRef), &reference))
	assert.EqualValues(t, created["id"], reference.NewsID)
	assert.NotContains(t, *stored.ProjectionRef, "Открыт новый КПП")

	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(otherToken))
	require.Equal(t, http.StatusOK, rec.Code)
	items := testutil.ParseSlice(t, rec)
	found := false
	for _, n := range items {
		if n["type"] == "news_published" {
			found = true
			assert.Equal(t, "Опубликована новость", n["title"])
			assert.Equal(t, "На сайте бюро вышла новая новость.", n["message"])
			assert.NotContains(t, n, "projection_ref")
			assert.Nil(t, n["data"], "free-form news preview is not an authenticated event reference")
			assert.EqualValues(t, stored.ID, n["id"])
		}
	}
	assert.True(t, found, "другой активный пользователь должен получить уведомление о новости")
	require.NoError(t, db.Model(&newsAccess).Update("value", "deny").Error)
	permissions, err = resolver.ResolveFresh(ctx, otherID)
	require.NoError(t, err)
	require.False(t, permissions.Has(services.KeyPageNews), "fresh explicit deny must remove %s: %+v", services.KeyPageNews, permissions)
	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(otherToken))
	require.Equal(t, http.StatusOK, rec.Code)
	found = false
	for _, n := range testutil.ParseSlice(t, rec) {
		if n["id"] == float64(stored.ID) {
			found = true
			assert.Equal(t, "Есть обновление", n["title"])
			assert.Equal(t, "Есть обновление", n["message"])
			assert.Nil(t, n["data"])
			assert.NotContains(t, n, "group_key")
			assert.NotContains(t, n, "projection_ref")
		}
	}
	assert.True(t, found, "revocation preserves the stored notification")

	rec = testutil.GET(t, e, "/notifications", testutil.AuthHeader(authorToken))
	require.Equal(t, http.StatusOK, rec.Code)
	for _, n := range testutil.ParseSlice(t, rec) {
		assert.NotEqual(t, "news_published", n["type"], "автор новости не должен уведомлять сам себя")
	}
}
