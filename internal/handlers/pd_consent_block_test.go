package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Блокирование после отзыва согласия (#2361).
//
// Отзыв не стирает данные: они хранятся дальше и выдаются по запросам
// государственных органов. Но обработка по прежним целям прекращается, а правка
// карточки - это она и есть. Проверяем три состояния, которые легко перепутать:
// согласия не было, согласие отозвано, согласие дано заново.

// grantConsent записывает действующее согласие напрямую в базу: тест проверяет
// блокирование, а не работу окна согласия, у которого свои тесты.
func grantConsent(t *testing.T, db *gorm.DB, username string) {
	t.Helper()
	var userID int
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", username).
		Select("id").Row().Scan(&userID))
	require.NoError(t, db.Create(&models.PDConsent{
		UserID:          userID,
		ConsentType:     "pd_processing",
		Granted:         true,
		GrantedAt:       time.Now().UTC(),
		DocumentVersion: 1,
	}).Error)
}

// revokeConsent помечает согласие отозванным тем же способом, что и сервис отзыва.
func revokeConsent(t *testing.T, db *gorm.DB, username string) {
	t.Helper()
	var userID int
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", username).
		Select("id").Row().Scan(&userID))
	now := time.Now().UTC()
	require.NoError(t, db.Model(&models.PDConsent{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Updates(map[string]any{"granted": false, "revoked_at": now}).Error)
}

// Работник, который согласия ещё не давал, правится обычным порядком: отсутствие
// согласия и отзыв - разные состояния, и путать их нельзя, иначе новую учётную
// запись нельзя было бы заполнить.
func TestConsentBlock_NeverGranted_UpdateAllowed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "block_fresh", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	rec := testutil.PUT(t, e, "/users/block_fresh/info",
		`{"last_name":"Новиков","position":"Инженер"}`,
		testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// После отзыва правка отклоняется, а данные остаются на месте: блокирование, а не
// уничтожение.
func TestConsentBlock_Revoked_UpdateRejected(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "block_revoked", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK, testutil.PUT(t, e, "/users/block_revoked/info",
		`{"last_name":"Петров","position":"Слесарь"}`, testutil.AuthHeader(admin)).Code)

	grantConsent(t, db, "block_revoked")
	revokeConsent(t, db, "block_revoked")

	rec := testutil.PUT(t, e, "/users/block_revoked/info",
		`{"last_name":"Подменённый","position":"Директор"}`,
		testutil.AuthHeader(admin))
	assert.Equal(t, http.StatusConflict, rec.Code, "правка после отзыва отклоняется")

	var last, position string
	require.NoError(t, db.Model(&models.User{}).Where("username = ?", "block_revoked").
		Select("COALESCE(last_name, ''), COALESCE(position, '')").Row().Scan(&last, &position))
	assert.Equal(t, "Петров", last, "прежние данные сохранены")
	assert.Equal(t, "Слесарь", position, "отказ не применил ни одного поля запроса")
}

// Новое согласие снимает блокирование само: отдельной команды разблокировки нет,
// и появляться она не должна.
func TestConsentBlock_GrantedAgain_UpdateAllowed(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "block_again", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)

	grantConsent(t, db, "block_again")
	revokeConsent(t, db, "block_again")
	require.Equal(t, http.StatusConflict, testutil.PUT(t, e, "/users/block_again/info",
		`{"position":"Мастер"}`, testutil.AuthHeader(admin)).Code)

	grantConsent(t, db, "block_again")

	rec := testutil.PUT(t, e, "/users/block_again/info",
		`{"position":"Мастер"}`, testutil.AuthHeader(admin))
	assert.Equal(t, http.StatusOK, rec.Code, "после нового согласия правка снова возможна")
}
