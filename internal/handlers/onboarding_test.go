package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// tourStatus читает прогресс туров текущего пользователя. Значения JSON: число или null.
func tourStatus(t *testing.T, e *echo.Echo, token string) map[string]any {
	t.Helper()
	rec := testutil.GET(t, e, "/onboarding", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	completed, ok := testutil.ParseMap(t, rec)["completed"].(map[string]any)
	require.True(t, ok, "ответ обязан содержать объект completed: %s", rec.Body.String())
	return completed
}

func completeTour(t *testing.T, e *echo.Echo, token, tour string, version int) *httptest.ResponseRecorder {
	t.Helper()
	body := fmt.Sprintf(`{"tour":%q,"version":%d}`, tour, version)
	return testutil.POST(t, e, "/onboarding/complete", body, testutil.AuthHeader(token))
}

// tourProgress отдаёт строки прогресса пользователя как tour_key -> версия.
func tourProgress(t *testing.T, db *gorm.DB, username string) map[string]int {
	t.Helper()
	var rows []models.UserOnboardingProgress
	require.NoError(t, db.Model(&models.UserOnboardingProgress{}).
		Joins("JOIN users u ON u.id = user_onboarding_progress.user_id").
		Where("u.username = ?", username).
		Find(&rows).Error)
	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.TourKey] = row.CompletedVersion
	}
	return out
}

// setLegacyTourVersion заполняет старую колонку - ту, из которой переносит миграция.
func setLegacyTourVersion(t *testing.T, db *gorm.DB, username string, version int) {
	t.Helper()
	res := db.Model(&models.User{}).
		Where("username = ?", username).
		Update("onboarding_completed_version", version)
	require.NoError(t, res.Error)
	require.EqualValues(t, 1, res.RowsAffected, "пользователь %q не найден", username)
}

// clearMigrationMarker снимает отметку разового переноса: она ставится ещё на первом
// AutoMigrate тестовой базы и переживает чистку, а тесту нужно, чтобы перенос отработал
// на его собственных данных.
func clearMigrationMarker(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Where("key = ?", database.OnboardingProgressMigratedMarker).
		Delete(&models.SystemSetting{}).Error)
}

func userTypeIDByCode(t *testing.T, db *gorm.DB, code string) int {
	t.Helper()
	var id int
	require.NoError(t, db.Raw("SELECT id FROM user_types WHERE code = ?", code).Scan(&id).Error)
	require.NotZero(t, id, "тип пользователя %q обязан быть засеян", code)
	return id
}

func TestOnboarding_GetStatus_NewUserHasAllToursNull(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	completed := tourStatus(t, e, token)
	assert.Len(t, completed, len(models.TourKeys), "ответ отдаёт ровно известные туры")
	for _, key := range models.TourKeys {
		value, ok := completed[key]
		assert.True(t, ok, "ключ %q обязан присутствовать", key)
		assert.Nil(t, value, "новый пользователь тур %q не проходил", key)
	}
}

// Ради этого срез и делался: у администратора туров несколько, и отметка одного не
// должна выглядеть как прохождение остальных.
func TestOnboarding_ToursDoNotOverlap(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK, completeTour(t, e, token, models.TourUser, 2).Code)
	completed := tourStatus(t, e, token)
	assert.Equal(t, float64(2), completed[models.TourUser])
	for _, key := range []string{models.TourGuard, models.TourApprove, models.TourAccept, models.TourAdmin} {
		assert.Nil(t, completed[key], "тур %q не проходили", key)
	}

	require.Equal(t, http.StatusOK, completeTour(t, e, token, models.TourAdmin, 5).Code)
	completed = tourStatus(t, e, token)
	assert.Equal(t, float64(2), completed[models.TourUser], "отметка соседнего тура не трогает версию первого")
	assert.Equal(t, float64(5), completed[models.TourAdmin])
	assert.Nil(t, completed[models.TourApprove])

	assert.Equal(t, map[string]int{models.TourUser: 2, models.TourAdmin: 5}, tourProgress(t, db, "testadmin"),
		"на каждый пройденный тур ровно одна строка")
}

func TestOnboarding_VersionNeverGoesDown(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK, completeTour(t, e, token, models.TourUser, 3).Code)
	// Отметка со старой вкладки приходит меньшей версией - прогресс от неё не падает.
	require.Equal(t, http.StatusOK, completeTour(t, e, token, models.TourUser, 1).Code)
	assert.Equal(t, 3, tourProgress(t, db, "testadmin")[models.TourUser])

	require.Equal(t, http.StatusOK, completeTour(t, e, token, models.TourUser, 4).Code)
	assert.Equal(t, 4, tourProgress(t, db, "testadmin")[models.TourUser], "новая версия тур перезаписывает")
}

func TestOnboarding_MarkComplete_RejectsUnknownTour(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := completeTour(t, e, token, "hacker", 1)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "неизвестный ключ тура не должен заводить строку")

	rec = testutil.POST(t, e, "/onboarding/complete", `{"version":1}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "тур обязателен")

	assert.Empty(t, tourProgress(t, db, "testadmin"), "отклонённые запросы ничего не записывают")
}

func TestOnboarding_MarkComplete_RejectsVersionBelowOne(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := completeTour(t, e, token, models.TourUser, 0)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "version<1 must be rejected")
	assert.Empty(t, tourProgress(t, db, "testadmin"), "rejected request must not write progress")
}

func TestOnboarding_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	rec := testutil.GET(t, e, "/onboarding", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = testutil.POST(t, e, "/onboarding/complete", `{"tour":"user","version":1}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestOnboarding_AdminReset_SingleTourLeavesOthers(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	userToken := testutil.RegisterAndLogin(t, e, "tour_user", "Password123!", 1, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK, completeTour(t, e, userToken, models.TourUser, 1).Code)
	require.Equal(t, http.StatusOK, completeTour(t, e, userToken, models.TourAccept, 2).Code)

	rec := testutil.POST(t, e, "/users/tour_user/onboarding/reset",
		fmt.Sprintf(`{"tour":%q}`, models.TourUser), testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, "Onboarding reset for user", testutil.ParseMessage(t, rec))

	assert.Equal(t, map[string]int{models.TourAccept: 2}, tourProgress(t, db, "tour_user"),
		"сброс одного тура не трогает остальные")
}

func TestOnboarding_AdminReset_WithoutTourClearsAll(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	userToken := testutil.RegisterAndLogin(t, e, "tour_user", "Password123!", 1, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK, completeTour(t, e, userToken, models.TourUser, 1).Code)
	require.Equal(t, http.StatusOK, completeTour(t, e, userToken, models.TourAdmin, 1).Code)
	require.Len(t, tourProgress(t, db, "tour_user"), 2)

	// Пустое тело - тот же смысл, что отсутствующий ключ: сбросить всё.
	rec := testutil.POST(t, e, "/users/tour_user/onboarding/reset", ``, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Empty(t, tourProgress(t, db, "tour_user"))

	// Прогресс администратора при этом на месте - сброс адресный.
	require.Equal(t, http.StatusOK, completeTour(t, e, adminToken, models.TourAdmin, 1).Code)
	rec = testutil.POST(t, e, "/users/tour_user/onboarding/reset", `{}`, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, map[string]int{models.TourAdmin: 1}, tourProgress(t, db, "testadmin"))
}

func TestOnboarding_AdminReset_RejectsUnknownTour(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	userToken := testutil.RegisterAndLogin(t, e, "tour_user", "Password123!", 1, td.OrgID, td.CompanyID)
	require.Equal(t, http.StatusOK, completeTour(t, e, userToken, models.TourUser, 1).Code)

	rec := testutil.POST(t, e, "/users/tour_user/onboarding/reset", `{"tour":"hacker"}`, testutil.AuthHeader(adminToken))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "мусорный ключ не должен читаться как «сбросить всё»")
	assert.Equal(t, map[string]int{models.TourUser: 1}, tourProgress(t, db, "tour_user"))
}

func TestOnboarding_AdminReset_UnknownUser_Returns404(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/users/nosuchuser/onboarding/reset", ``, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusNotFound, rec.Code, "reset of unknown user must be 404")
}

func TestOnboarding_AdminReset_NonAdmin_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	// Обычный пользователь без прав page.admin.
	userToken := testutil.RegisterAndLogin(t, e, "regular_user", "Password123!", 1, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/users/regular_user/onboarding/reset", ``, testutil.AuthHeader(userToken))
	assert.Equal(t, http.StatusForbidden, rec.Code, "non-admin must not reset onboarding")
}

// Перенос старой колонки: охраннику доставался сценарий охраны, всем остальным - общий.
func TestOnboarding_LegacyMigration_TourByUserType(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	securityTypeID := userTypeIDByCode(t, db, "security")
	testutil.RegisterUser(t, e, "legacy_guard", "Password123!", securityTypeID, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "legacy_user", "Password123!", 1, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "legacy_never", "Password123!", 1, td.OrgID, td.CompanyID)
	setLegacyTourVersion(t, db, "legacy_guard", 2)
	setLegacyTourVersion(t, db, "legacy_user", 3)

	clearMigrationMarker(t, db)
	require.NoError(t, database.MigrateOnboardingProgress(db))

	assert.Equal(t, map[string]int{models.TourGuard: 2}, tourProgress(t, db, "legacy_guard"))
	assert.Equal(t, map[string]int{models.TourUser: 3}, tourProgress(t, db, "legacy_user"))
	assert.Empty(t, tourProgress(t, db, "legacy_never"), "пустая колонка не заводит строку прогресса")

	// Повторный прогон переноса (маркер снят руками) дублей не создаёт.
	clearMigrationMarker(t, db)
	require.NoError(t, database.MigrateOnboardingProgress(db))
	assert.Equal(t, map[string]int{models.TourGuard: 2}, tourProgress(t, db, "legacy_guard"))
	assert.Equal(t, map[string]int{models.TourUser: 3}, tourProgress(t, db, "legacy_user"))
}

// Старая колонка остаётся заполненной, поэтому без отметки о выполненном переносе
// каждый старт сервера воскрешал бы прохождение, снятое администратором.
func TestOnboarding_LegacyMigration_MarkerSurvivesAdminReset(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	testutil.RegisterUser(t, e, "legacy_user", "Password123!", 1, td.OrgID, td.CompanyID)
	setLegacyTourVersion(t, db, "legacy_user", 3)

	clearMigrationMarker(t, db)
	require.NoError(t, database.MigrateOnboardingProgress(db))
	require.Equal(t, map[string]int{models.TourUser: 3}, tourProgress(t, db, "legacy_user"))

	rec := testutil.POST(t, e, "/users/legacy_user/onboarding/reset", ``, testutil.AuthHeader(adminToken))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	require.Empty(t, tourProgress(t, db, "legacy_user"))

	// Рестарт сервера: AutoMigrate снова зовёт перенос.
	require.NoError(t, database.MigrateOnboardingProgress(db))
	assert.Empty(t, tourProgress(t, db, "legacy_user"),
		"перенос выполняется один раз - сброшенный тур не должен возвращаться при рестарте")
}
