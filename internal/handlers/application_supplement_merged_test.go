package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Дополнение заявки, которая ещё не принята в работу (#1685): раунда согласования у него
// нет, добавка вливается в основной круг и получает статус merged.
//
// Тест сторожит путь, который на этом и ломался: строки merged-раунда обязаны активироваться
// приёмом заявки в работу наравне с исходным составом. Своего принятия у merged нет и не
// будет - перевести его в accepted некому, - поэтому выпади он из предиката допуска, и
// добавленный человек не попал бы на пост никогда, а карточка автора при этом показывала бы
// его ожидающим решения.
func TestSupplementMerged_ReachesPostsWithApplication(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	authorID, _ := suppVoteUser(t, e, db, "merged_author", td.OrgID, td.CompanyID)
	acceptorID, acceptorToken := suppVoteUser(t, e, db, "merged_acceptor", td.OrgID, td.CompanyID)
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: acceptorID}).Error)

	// Заявка согласована, но в работу ещё не принята - терять на постах нечего, поэтому
	// добавка идёт в текущий круг, а не отдельным раундом.
	appID := suppApp(t, db, td.OrgID, authorID, "MERGED-1", models.ConfirmationApproved, models.StatusProcessing)
	attID := suppAttachment(t, db, appID, "people", "2030-01-01")

	original := models.Employee{AttachmentID: &attID, LastName: testutil.Ptr("Исходный"), Status: testutil.Ptr(0)}
	require.NoError(t, db.Create(&original).Error)

	merged := models.ApplicationSupplement{
		ApplicationID: appID, Number: 1, Status: models.SupplementMerged, CreatedByUserID: authorID,
	}
	require.NoError(t, db.Create(&merged).Error)
	added := models.Employee{
		AttachmentID: &attID, SupplementID: &merged.ID,
		LastName: testutil.Ptr("Дописанный"), Status: testutil.Ptr(0),
	}
	require.NoError(t, db.Create(&added).Error)

	// Карточка автора не должна обещать решение по влитой добавке: принимать её отдельно
	// никто не будет.
	rec := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/employees", attID), testutil.AuthHeader(acceptorToken))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, row := range testutil.ParseResponse[[]services.EmployeeWithTables](t, rec) {
		if row.LastName == "Дописанный" {
			assert.False(t, row.IsPending, "влитая в основной круг добавка не ждёт отдельного решения")
		}
	}

	body := fmt.Sprintf(`{"user_id":%d,"action":"accept"}`, acceptorID)
	rec = testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", appID), body, testutil.AuthHeader(acceptorToken))
	require.Equal(t, http.StatusOK, rec.Code, "принять заявку в работу: %s", rec.Body.String())

	var originalAfter, addedAfter models.Employee
	require.NoError(t, db.First(&originalAfter, original.ID).Error)
	require.NoError(t, db.First(&addedAfter, added.ID).Error)
	require.NotNil(t, originalAfter.Status)
	require.NotNil(t, addedAfter.Status)
	assert.Equal(t, 1, *originalAfter.Status, "исходный состав активируется приёмом в работу")
	assert.Equal(t, 1, *addedAfter.Status, "влитая добавка встаёт на пост вместе с исходным составом")
}
