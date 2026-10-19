package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestS6History_ApplicationHTTPMasksAllActorParts(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seed := testutil.SeedTestData(t, db)
	sender := testutil.RegisterAndLogin(t, e, "s6_history_sender", "password123", 1, seed.OrgID, seed.CompanyID)
	admin := testutil.RegisterAdmin(t, e, seed.OrgID, seed.CompanyID)
	// RegisterUser(type6) also sets the super flag; this receiver must be an
	// ordinary account so it participates in the independent consent policy.
	testutil.RegisterUser(t, e, "s6_history_operator", "password123", 1, seed.OrgID, seed.CompanyID)
	testutil.RegisterUser(t, e, "s6_history_admin", "password123", 1, seed.OrgID, seed.CompanyID)
	require.NoError(t, db.Model(&models.User{}).Where("username=?", "s6_history_admin").Update("is_admin", true).Error)
	ordinaryAdmin, _ := testutil.LoginUser(t, e, "s6_history_admin", "password123")
	actor := setUserName(t, db, "s6_history_operator", "PrivateLastS6", "PrivateFirstS6", "PrivateMiddleS6")
	label := "Оператор бюро"
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: actor, DisplayName: &label}).Error)
	appID := createSimpleApplication(t, e, sender, seed.OrgID)
	// Administrative identity disclosure does not grant access to an unrelated
	// application. Give this viewer the existing explicit viewing relationship.
	require.NoError(t, db.Create(&models.ApplicationViewer{ApplicationID: appID, UserID: getUserID(t, db, "s6_history_admin")}).Error)
	require.NoError(t, services.NewAuditRecorder(db).Record(context.Background(), nil, models.AuditEntityApplication, &appID, "take_to_work", &actor, nil))
	path := fmt.Sprintf("/applications/%d/history", appID)
	for _, scenario := range []struct {
		name, token string
		real        bool
	}{{"sender", sender, false}, {"admin", ordinaryAdmin, true}, {"super", admin, true}} {
		t.Run(scenario.name, func(t *testing.T) {
			response := testutil.GET(t, e, path, testutil.AuthHeader(scenario.token))
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			items := testutil.ParseSlice(t, response)
			found := false
			for _, item := range items {
				if item["action_type"] != "take_to_work" {
					continue
				}
				found = true
				if scenario.real {
					require.Contains(t, item["user_name"], "PrivateLastS6")
					require.Equal(t, "PrivateLastS6", item["last_name"])
				} else {
					require.Equal(t, label, item["user_name"])
					require.Nil(t, item["last_name"])
					require.Nil(t, item["first_name"])
					require.Nil(t, item["middle_name"])
					require.NotContains(t, response.Body.String(), "PrivateLastS6")
				}
			}
			require.True(t, found)
		})
	}
	// Consent hides the same name from the administrative viewer as well.
	enableConsent(t, e, admin, "<p>Synthetic consent</p>")
	response := testutil.GET(t, e, path, testutil.AuthHeader(admin))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.NotContains(t, response.Body.String(), "PrivateLastS6")
	require.NotContains(t, response.Body.String(), "PrivateFirstS6")
	require.NotContains(t, response.Body.String(), "PrivateMiddleS6")
	// Opening the main card can mark an application read. A failed name-policy
	// lookup must happen before that incidental mutation, in the same transaction.
	require.NoError(t, db.Model(&models.Application{}).Where("id=?", appID).
		Updates(map[string]any{"status": "Непрочитано", "reading_datetime": nil}).Error)
	var before, after models.Application
	require.NoError(t, db.First(&before, appID).Error)
	const callbackName = "s6_history_card_policy_failure"
	injected := false
	require.NoError(t, db.Callback().Row().Before("gorm:row").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement.Table == "system_settings" {
			injected = true
			tx.AddError(errors.New("synthetic card policy failure"))
		}
	}))
	defer db.Callback().Row().Remove(callbackName)
	ctx := services.WithSessionIdentity(context.Background(), models.SessionIdentity{EffectiveUserID: getUserID(t, db, "testadmin")})
	card, err := services.NewApplicationService(db, nil, nil, nil, nil, services.NewAuditRecorder(db)).GetApplicationByID(ctx, "testadmin", appID, true)
	require.True(t, injected)
	require.Nil(t, card)
	var unavailable *echo.HTTPError
	require.ErrorAs(t, err, &unavailable)
	require.Equal(t, http.StatusServiceUnavailable, unavailable.Code)
	require.NoError(t, db.First(&after, appID).Error)
	require.Equal(t, before.Status, after.Status)
	require.Equal(t, before.ReadingDatetime, after.ReadingDatetime)
}
