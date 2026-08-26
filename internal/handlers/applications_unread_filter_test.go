package handlers_test

import (
	"net/http"
	"net/url"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplicationsUnreadAndMultiStatusFilter: фильтр "Непрочитано" (псевдо-статус через
// application_reads, не колонка a.status) и мультивыбор статусов (IN), включая их OR-комбо.
// Регресс на баг: "Непрочитано" уходил как status='Непрочитано' -> бэк отдавал пусто.
func TestApplicationsUnreadAndMultiStatusFilter(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "unread_sender", "pass123", 1, td.OrgID, td.CompanyID)
	senderID := getUserID(t, db, "unread_sender")

	// Три заявки этого отправителя (без вложений -> не архивные, видны по дефолту).
	appRead := seedAttachableApp(t, db, td.OrgID, senderID, "UNR-READ", "Согласование", "В обработке")
	appWork := seedAttachableApp(t, db, td.OrgID, senderID, "UNR-WORK", "Согласование", "В работе")
	appDone := seedAttachableApp(t, db, td.OrgID, senderID, "UNR-DONE", "Согласование", "Завершено")

	// Помечаем ОДНУ прочитанной этим пользователем (unread = отсутствие такой записи).
	require.NoError(t, db.Create(&models.ApplicationRead{ApplicationID: appRead, UserID: senderID}).Error)

	idsFor := func(t *testing.T, query string) map[int]bool {
		t.Helper()
		rec := testutil.GET(t, e, "/applications?per_page=50"+query, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		ids := map[int]bool{}
		for _, row := range testutil.ParseSlice(t, rec) {
			if id, ok := row["id"].(float64); ok {
				ids[int(id)] = true
			}
		}
		return ids
	}

	t.Run("без фильтра - все три видны", func(t *testing.T) {
		ids := idsFor(t, "")
		assert.True(t, ids[appRead] && ids[appWork] && ids[appDone], "ожидались все три: %v", ids)
	})

	t.Run("unread=true - только непрочитанные, прочитанная исключена", func(t *testing.T) {
		ids := idsFor(t, "&unread=true")
		assert.False(t, ids[appRead], "прочитанная не должна попасть в unread")
		assert.True(t, ids[appWork], "непрочитанная В работе должна быть")
		assert.True(t, ids[appDone], "непрочитанная Завершено должна быть")
	})

	t.Run("status=В работе,Завершено - IN по нескольким статусам", func(t *testing.T) {
		ids := idsFor(t, "&status="+url.QueryEscape("В работе,Завершено"))
		assert.False(t, ids[appRead], "В обработке не входит в набор")
		assert.True(t, ids[appWork])
		assert.True(t, ids[appDone])
	})

	t.Run("status=В работе + unread=true - OR (статус ИЛИ непрочитано)", func(t *testing.T) {
		// appRead: В обработке + прочитана -> не проходит ни статус, ни unread.
		// appWork: В работе -> проходит по статусу. appDone: Завершено + непрочитана -> по unread.
		ids := idsFor(t, "&status="+url.QueryEscape("В работе")+"&unread=true")
		assert.False(t, ids[appRead], "прочитанная В обработке не должна проходить OR")
		assert.True(t, ids[appWork], "В работе проходит по статусу")
		assert.True(t, ids[appDone], "непрочитанная Завершено проходит по unread")
	})
}
