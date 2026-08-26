package handlers_test

// Видимость заявок устроена иначе, чем у реестров: она не про организацию, а про
// участие -- своя заявка, заявка где ты ответственный или наблюдатель. Плюс принимающий
// видит все. Каждое из трёх правил проверяется отдельно: провайдер поиска обязан
// повторять applyApplicationAccessFilter целиком, а не его половину.

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// seedSearchApplication кладёт заявку напрямую: провести её через API значило бы
// собрать вложения, машины и согласования, а проверяем мы видимость.
func seedSearchApplication(t *testing.T, db *gorm.DB, number string, senderID, orgID int) int {
	t.Helper()
	app := models.Application{
		ApplicationNumber: searchStrPtr(number),
		Message:           searchStrPtr("Пропуск для Роголева"),
		Status:            searchStrPtr("В обработке"),
		SenderUserID:      senderID,
		OrganizationID:    orgID,
	}
	require.NoError(t, db.Create(&app).Error)
	return app.ID
}

// Автор находит свою заявку, посторонний -- нет.
func TestSearch_Applications_OnlyParticipantsSee(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "app_author", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "app_author")
	testutil.RegisterUser(t, e, "app_outsider", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "app_outsider")

	authorID := userIDByName(t, db, "app_author")
	seedSearchApplication(t, db, "20260731/777", authorID, td.OrgID)

	t.Run("автор видит свою заявку", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "app_author", "password123")
		rec := testutil.GET(t, e, "/search?q=20260731/777", testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

		count, found := groupByType(decodeSearch(t, rec.Body.String()), "applications")
		require.True(t, found, "своя заявка обязана находиться: %s", rec.Body.String())
		assert.Equal(t, 1, count)
	})

	t.Run("посторонний из той же организации не видит", func(t *testing.T) {
		token, _ := testutil.LoginUser(t, e, "app_outsider", "password123")
		rec := testutil.GET(t, e, "/search?q=20260731/777", testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code)

		_, found := groupByType(decodeSearch(t, rec.Body.String()), "applications")
		assert.False(t, found, "заявка чужая: видимость заявок идёт по участию, не по организации: %s", rec.Body.String())
	})
}

// Принимающий видит все заявки -- ровно как в Центре.
func TestSearch_Applications_ApproverSeesAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "app_sender", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "app_sender")
	testutil.RegisterUser(t, e, "app_approver", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "app_approver")

	senderID := userIDByName(t, db, "app_sender")
	approverID := userIDByName(t, db, "app_approver")
	seedSearchApplication(t, db, "20260731/888", senderID, td.OrgID)
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: approverID}).Error)

	token, _ := testutil.LoginUser(t, e, "app_approver", "password123")
	rec := testutil.GET(t, e, "/search?q=20260731/888", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	count, found := groupByType(decodeSearch(t, rec.Body.String()), "applications")
	require.True(t, found, "принимающий видит все заявки: %s", rec.Body.String())
	assert.Equal(t, 1, count)
}

// Наблюдатель заявки тоже участник -- вторая ветка фильтра доступа.
func TestSearch_Applications_ViewerSees(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "app_owner2", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "app_owner2")
	testutil.RegisterUser(t, e, "app_viewer", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "app_viewer")

	ownerID := userIDByName(t, db, "app_owner2")
	viewerID := userIDByName(t, db, "app_viewer")
	appID := seedSearchApplication(t, db, "20260731/999", ownerID, td.OrgID)
	require.NoError(t, db.Create(&models.ApplicationViewer{ApplicationID: appID, UserID: viewerID}).Error)

	token, _ := testutil.LoginUser(t, e, "app_viewer", "password123")
	rec := testutil.GET(t, e, "/search?q=20260731/999", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	_, found := groupByType(decodeSearch(t, rec.Body.String()), "applications")
	assert.True(t, found, "наблюдатель заявки должен её находить: %s", rec.Body.String())
}

// Марка машины в заявке лежит в устаревшей колонке car_brand -- снимок mark_name
// появился позже и заполнен у единиц записей. Заявка обязана находиться по марке,
// иначе запрос «номер + марка» не работает именно там, где его чаще всего вводят.
func TestSearch_Applications_FoundByCarBrand(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "app_brand", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "app_brand")
	senderID := userIDByName(t, db, "app_brand")
	appID := seedSearchApplication(t, db, "20260731/222", senderID, td.OrgID)

	att := models.Attachment{ApplicationID: &appID, AttachmentType: "cars"}
	require.NoError(t, db.Create(&att).Error)
	require.NoError(t, db.Create(&models.Car{
		AttachmentID: att.ID,
		CarNumber:    searchStrPtr("В 543 НЕ 654"),
		CarBrand:     searchStrPtr("Мерседес"),
	}).Error)

	token, _ := testutil.LoginUser(t, e, "app_brand", "password123")

	for _, q := range []string{"Мерседес", "В 543 НЕ 654 Мерседес"} {
		t.Run(q, func(t *testing.T) {
			rec := testutil.GET(t, e, "/search?q="+urlQuery(q), testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

			_, found := groupByType(decodeSearch(t, rec.Body.String()), "applications")
			assert.True(t, found, "заявка должна находиться по марке своей машины: %s", rec.Body.String())
		})
	}
}

// Заявку можно найти по номеру машины из вложения -- ради этого поиск по заявкам и нужен.
func TestSearch_Applications_FoundByCarNumber(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	testutil.RegisterUser(t, e, "app_car", "password123", 1, td.OrgID, td.CompanyID)
	assignBaseRole(t, db, "app_car")
	senderID := userIDByName(t, db, "app_car")
	appID := seedSearchApplication(t, db, "20260731/111", senderID, td.OrgID)

	att := models.Attachment{ApplicationID: &appID, AttachmentType: "cars"}
	require.NoError(t, db.Create(&att).Error)
	require.NoError(t, db.Create(&models.Car{
		AttachmentID: att.ID,
		CarNumber:    searchStrPtr("А777АА"),
		MarkName:     searchStrPtr("BMW"),
	}).Error)

	token, _ := testutil.LoginUser(t, e, "app_car", "password123")
	rec := testutil.GET(t, e, "/search?q=А777АА", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "тело: %s", rec.Body.String())

	_, found := groupByType(decodeSearch(t, rec.Body.String()), "applications")
	assert.True(t, found, "заявка должна находиться по номеру машины из вложения: %s", rec.Body.String())
}
