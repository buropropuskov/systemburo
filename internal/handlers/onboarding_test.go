package handlers_test

import (
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnboarding_GetStatus_NewUserIsNull(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.GET(t, e, "/onboarding", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	resp := testutil.ParseMap(t, rec)
	// JSON null десериализуется в nil; ключ присутствует.
	val, ok := resp["completed_version"]
	assert.True(t, ok, "completed_version key must be present")
	assert.Nil(t, val, "new user must have null completed_version")

	var u models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&u).Error)
	assert.Nil(t, u.OnboardingCompletedVersion, "DB column must be null for new user")
}

func TestOnboarding_MarkComplete_ThenGetReturnsVersion(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/onboarding/complete", `{"version":1}`, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Equal(t, "Onboarding marked as completed", testutil.ParseMessage(t, rec))

	// Значение записалось в БД.
	var u models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&u).Error)
	require.NotNil(t, u.OnboardingCompletedVersion)
	assert.Equal(t, 1, *u.OnboardingCompletedVersion)

	// И отдаётся через GET.
	recGet := testutil.GET(t, e, "/onboarding", testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, recGet.Code, "body: %s", recGet.Body.String())
	resp := testutil.ParseMap(t, recGet)
	// JSON-число приходит как float64.
	assert.Equal(t, float64(1), resp["completed_version"])
}

func TestOnboarding_MarkComplete_UpdatesToNewVersion(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	require.Equal(t, http.StatusOK,
		testutil.POST(t, e, "/onboarding/complete", `{"version":1}`, testutil.AuthHeader(token)).Code)
	require.Equal(t, http.StatusOK,
		testutil.POST(t, e, "/onboarding/complete", `{"version":3}`, testutil.AuthHeader(token)).Code)

	var u models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&u).Error)
	require.NotNil(t, u.OnboardingCompletedVersion)
	assert.Equal(t, 3, *u.OnboardingCompletedVersion, "later version must overwrite earlier")
}

func TestOnboarding_MarkComplete_RejectsVersionBelowOne(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	rec := testutil.POST(t, e, "/onboarding/complete", `{"version":0}`, testutil.AuthHeader(token))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "version<1 must be rejected")

	// Поле не должно было записаться.
	var u models.User
	require.NoError(t, db.Where("username = ?", "testadmin").First(&u).Error)
	assert.Nil(t, u.OnboardingCompletedVersion, "rejected request must not write the column")
}

func TestOnboarding_Unauthorized(t *testing.T) {
	e, _, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	rec := testutil.GET(t, e, "/onboarding", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	rec = testutil.POST(t, e, "/onboarding/complete", `{"version":1}`, nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
