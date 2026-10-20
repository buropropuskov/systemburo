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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNotificationOwnership(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "notif_owner", "password123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "notif_other", "password123", 1, td.OrgID, td.CompanyID)
	owner := getUserID(t, db, "notif_owner")
	other := getUserID(t, db, "notif_other")
	headers := testutil.AuthHeader(token)
	create := func(userID int) models.Notification {
		n := models.Notification{UserID: userID}
		require.NoError(t, db.Create(&n).Error)
		return n
	}
	foreign := create(other)
	own := create(owner)
	absent := create(owner)
	require.NoError(t, db.Delete(&absent).Error)

	t.Run("unavailable_responses_match", func(t *testing.T) {
		foreignRead := testutil.PUT(t, e, fmt.Sprintf("/notifications/%d/read", foreign.ID), `{"is_read":true}`, headers)
		absentRead := testutil.PUT(t, e, fmt.Sprintf("/notifications/%d/read", absent.ID), `{"is_read":true}`, headers)
		require.Equal(t, http.StatusNotFound, foreignRead.Code)
		require.Equal(t, http.StatusNotFound, absentRead.Code)
		assert.JSONEq(t, absentRead.Body.String(), foreignRead.Body.String())
		foreignDelete := testutil.DELETE(t, e, fmt.Sprintf("/notifications/%d", foreign.ID), headers)
		absentDelete := testutil.DELETE(t, e, fmt.Sprintf("/notifications/%d", absent.ID), headers)
		require.Equal(t, http.StatusNotFound, foreignDelete.Code)
		require.Equal(t, http.StatusNotFound, absentDelete.Code)
		assert.JSONEq(t, absentDelete.Body.String(), foreignDelete.Body.String())
		var unchanged models.Notification
		require.NoError(t, db.First(&unchanged, foreign.ID).Error)
		assert.False(t, unchanged.IsRead)
		assert.Equal(t, other, unchanged.UserID)
	})
	t.Run("own_actions", func(t *testing.T) {
		for _, read := range []bool{true, true, false} {
			rec := testutil.PUT(t, e, fmt.Sprintf("/notifications/%d/read", own.ID), fmt.Sprintf(`{"is_read":%t}`, read), headers)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Equal(t, read, testutil.ParseMap(t, rec)["is_read"])
		}
		rec := testutil.DELETE(t, e, fmt.Sprintf("/notifications/%d", own.ID), headers)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Equal(t, http.StatusNotFound, testutil.DELETE(t, e, fmt.Sprintf("/notifications/%d", own.ID), headers).Code)
	})
	t.Run("bulk_is_personal", func(t *testing.T) {
		n := create(owner)
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, "/notifications/read-all", "", headers).Code)
		var rows []models.Notification
		require.NoError(t, db.Where("id IN ?", []int{n.ID, foreign.ID}).Order("id").Find(&rows).Error)
		require.Len(t, rows, 2)
		for _, row := range rows {
			assert.Equal(t, row.UserID == owner, row.IsRead)
		}
		require.Equal(t, http.StatusOK, testutil.DELETE(t, e, "/notifications", headers).Code)
		require.ErrorIs(t, db.First(&models.Notification{}, n.ID).Error, gorm.ErrRecordNotFound)
		require.NoError(t, db.First(&models.Notification{}, foreign.ID).Error)
	})
	t.Run("authentication_required", func(t *testing.T) {
		require.Equal(t, http.StatusUnauthorized, testutil.PUT(t, e, fmt.Sprintf("/notifications/%d/read", foreign.ID), `{"is_read":true}`, nil).Code)
		require.Equal(t, http.StatusUnauthorized, testutil.DELETE(t, e, fmt.Sprintf("/notifications/%d", foreign.ID), nil).Code)
	})
}

func TestNotificationOwnershipDatabaseFailures(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	user := models.User{Username: "notif_db_owner", Password: "x", TypeID: 1, OrganizationID: &td.OrgID, CompanyID: &td.CompanyID}
	require.NoError(t, db.Create(&user).Error)
	svc := services.NewNotificationService(db)
	ctx := context.Background()
	for _, operation := range []string{"read", "delete"} {
		for _, phase := range []string{"query", "write", "disappear"} {
			t.Run(operation+"_"+phase, func(t *testing.T) {
				n := models.Notification{UserID: user.ID}
				require.NoError(t, db.Create(&n).Error)
				name := "test:notification_ownership_" + operation + "_" + phase
				callback := func(tx *gorm.DB) {
					if tx.Statement.Table == "notifications" {
						tx.AddError(errors.New("synthetic database failure: private detail"))
					}
				}
				var remove func() error
				switch phase {
				case "query":
					require.NoError(t, db.Callback().Query().Before("gorm:query").Register(name, callback))
					remove = func() error { return db.Callback().Query().Remove(name) }
				case "write":
					if operation == "read" {
						require.NoError(t, db.Callback().Update().Before("gorm:update").Register(name, callback))
						remove = func() error { return db.Callback().Update().Remove(name) }
					} else {
						require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(name, callback))
						remove = func() error { return db.Callback().Delete().Remove(name) }
					}
				case "disappear":
					require.NoError(t, db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
						if tx.Statement.Table == "notifications" && tx.Error == nil {
							require.NoError(t, db.Session(&gorm.Session{NewDB: true}).Delete(&models.Notification{}, n.ID).Error)
						}
					}))
					remove = func() error { return db.Callback().Query().Remove(name) }
				}
				defer func() { require.NoError(t, remove()) }()
				var err error
				if operation == "read" {
					_, err = svc.MarkRead(ctx, user.ID, n.ID, models.MarkNotificationReadRequest{IsRead: true})
				} else {
					err = svc.Delete(ctx, user.ID, n.ID)
				}
				var httpErr *echo.HTTPError
				require.ErrorAs(t, err, &httpErr)
				expected := http.StatusInternalServerError
				if phase == "disappear" {
					expected = http.StatusNotFound
				}
				assert.Equal(t, expected, httpErr.Code)
				assert.NotContains(t, fmt.Sprint(httpErr.Message), "private detail")
			})
		}
	}
}
