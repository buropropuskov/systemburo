package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotifications_Create_Admin: админ создаёт рассылку - 200 и уведомление
// появляется у целевого пользователя.
func TestNotifications_Create_Admin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	target := models.User{
		Username:       "notif_target",
		Password:       "x",
		TypeID:         1,
		OrganizationID: &td.OrgID,
		CompanyID:      &td.CompanyID,
	}
	require.NoError(t, db.Create(&target).Error)

	body := fmt.Sprintf(`{"user_id":%d,"title":"Внимание"}`, target.ID)
	rec := testutil.POST(t, e, "/notifications", body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusOK, rec.Code)
	n := testutil.ParseMap(t, rec)
	assert.Equal(t, "Внимание", n["title"])
	assert.Equal(t, float64(target.ID), n["user_id"])
}

// TestNotifications_Create_Forbidden_NonAdmin: обычный пользователь не может
// создавать рассылку - гейт page.admin (Ф5, ранее type_id 5/6).
func TestNotifications_Create_Forbidden_NonAdmin(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "regularuser", "password123", 1, td.OrgID, td.CompanyID)

	body := `{"user_id":1,"title":"X"}`
	rec := testutil.POST(t, e, "/notifications", body, testutil.AuthHeader(token))

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// TestNotifications_Create_Unauthorized: без токена - 401.
func TestNotifications_Create_Unauthorized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	body := `{"user_id":1,"title":"X"}`
	rec := testutil.POST(t, e, "/notifications", body, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}
