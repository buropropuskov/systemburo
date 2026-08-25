package handlers_test

// Новости, объявления и документы видны любому, у кого есть раздел «Обзор и новости», --
// но только опубликованные. Обращения обратной связи закрыты своим правом. Проверяем обе
// границы: снятое с публикации не всплывает в подсказках, чужие обращения не видны без
// права на их разбор.

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearch_Content_OnlyPublished(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	live := searchDirToken("Роголевновость")
	hidden := searchDirToken("Роголевчерновик")
	require.NoError(t, db.Create(&models.News{Title: live, Description: searchStrPtr("действующая")}).Error)

	// Снятие с публикации отдельным Update: при Create false -- нулевое значение, и
	// gorm подставляет вместо него default:true.
	draft := models.News{Title: hidden, Description: searchStrPtr("снята с публикации")}
	require.NoError(t, db.Create(&draft).Error)
	require.NoError(t, db.Model(&draft).Update("is_active", false).Error)

	testutil.RegisterUser(t, e, "content_user", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "content_user")
	token, _ := testutil.LoginUser(t, e, "content_user", "password123")
	h := testutil.AuthHeader(token)

	t.Run("действующая новость находится", func(t *testing.T) {
		rec := testutil.GET(t, e, "/search?q="+live, h)
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "content")
		assert.True(t, found, "новость раздела должна находиться: %s", rec.Body.String())
	})

	t.Run("снятая с публикации не находится", func(t *testing.T) {
		rec := testutil.GET(t, e, "/search?q="+hidden, h)
		require.Equal(t, http.StatusOK, rec.Code)

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "content")
		assert.False(t, found, "снятое с публикации не место показывать в подсказках: %s", rec.Body.String())
	})
}

// Скрытый документ не должен всплывать: открыть его пользователь всё равно не сможет.
func TestSearch_Content_HiddenDocumentNotListed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	name := searchDirToken("Роголевдок")
	doc := models.Document{Title: name, FileName: name + ".pdf", StoredName: name}
	require.NoError(t, db.Create(&doc).Error)
	require.NoError(t, db.Model(&doc).Update("is_visible", false).Error)

	testutil.RegisterUser(t, e, "content_doc", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "content_doc")

	token, _ := testutil.LoginUser(t, e, "content_doc", "password123")
	rec := testutil.GET(t, e, "/search?q="+name, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	_, found := groupByType(decodeSearch(t, rec.Body.String()), "content")
	assert.False(t, found, "скрытый документ не должен попадать в выдачу: %s", rec.Body.String())
}

func TestSearch_Feedback_GatedByPermission(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "fb_author", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "fb_author")
	testutil.RegisterUser(t, e, "fb_plain", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "fb_plain")
	testutil.RegisterUser(t, e, "fb_admin", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "fb_admin")
	grantPermission(t, db, "fb_admin", "page.admin.feedback")

	authorID := userIDByName(t, db, "fb_author")
	message := searchDirToken("Роголевобращение")
	require.NoError(t, db.Create(&models.Feedback{
		UserID:  authorID,
		Message: message + ": не открывается страница",
		Status:  "Не решено",
	}).Error)

	t.Run("без права раздела нет", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "fb_plain", "password123")
		rec := testutil.GET(t, e, "/search?q="+message, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "feedback")
		assert.False(t, found, "чужие обращения закрыты правом: %s", rec.Body.String())
	})

	t.Run("с правом обращение находится", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "fb_admin", "password123")
		rec := testutil.GET(t, e, "/search?q="+message, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		count, found := groupByType(decodeSearch(t, rec.Body.String()), "feedback")
		require.True(t, found, "обращение должно находиться: %s", rec.Body.String())
		assert.Equal(t, 1, count)
	})
}
