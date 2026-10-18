package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestS5History_DetailFamilies(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedElementScopeFixture(t, db)
	token := testutil.RegisterAndLogin(t, e, "history_reader", "pass123", 1, td.OrgID, td.CompanyID)
	uid := getUserID(t, db, "history_reader")
	require.NoError(t, db.Create(&models.ApplicationViewer{ApplicationID: fx.appID, UserID: uid}).Error)
	for _, entity := range []struct {
		kind string
		id   int
	}{{models.AuditEntityCar, fx.carID}, {models.AuditEntityEmployee, fx.empID}} {
		require.NoError(t, db.Where("entity_type=? AND entity_id=?", entity.kind, entity.id).Delete(&models.AuditLog{}).Error)
		for _, action := range []string{"entry", "exit", "update", "dates_changed", "entry_revert", "exit_revert", "passage_close", "passage_close_revert", "unknown_future_action"} {
			require.NoError(t, db.Exec(`INSERT INTO audit_log(entity_type,entity_id,action,details,created_at) VALUES(?,?,?,'{}',NOW())`, entity.kind, entity.id, action).Error)
		}
	}
	paths := []string{fmt.Sprintf("/cars/%d/history", fx.carID), fmt.Sprintf("/employees/%d/history", fx.empID), "/cars/history/unified?car_number=Р200ОС77&car_brand=Газель", "/employees/history/unified?last_name=Постовой&first_name=Пётр"}
	h := testutil.AuthHeader(token)
	for _, mode := range []struct {
		name              string
		changes, passages bool
	}{{"none", false, false}, {"changes", true, false}, {"passages", false, true}, {"both", true, true}} {
		t.Run(mode.name, func(t *testing.T) {
			require.NoError(t, db.Where("user_id=? AND permission_key IN ?", uid, []string{services.KeyDetailFullHistory, services.KeyDetailEntryExitHistory}).Delete(&models.UserPermissionOverride{}).Error)
			if mode.changes {
				testutil.GrantPermission(t, uid, services.KeyDetailFullHistory)
			}
			if mode.passages {
				testutil.GrantPermission(t, uid, services.KeyDetailEntryExitHistory)
			}
			for _, path := range paths {
				r := testutil.GET(t, e, path, h)
				if !mode.changes && !mode.passages {
					require.Equal(t, http.StatusForbidden, r.Code, r.Body.String())
					continue
				}
				require.Equal(t, http.StatusOK, r.Code, r.Body.String())
				var expected, actual []string
				if mode.changes {
					expected = append(expected, "update", "dates_changed", "unknown_future_action")
				}
				if mode.passages {
					expected = append(expected, "entry", "exit", "entry_revert", "exit_revert", "passage_close", "passage_close_revert")
				}
				for _, row := range testutil.ParseSlice(t, r) {
					actual = append(actual, row["action_type"].(string))
				}
				require.ElementsMatch(t, expected, actual, path)
			}
		})
	}
	// Object visibility remains mandatory even with both history capabilities.
	rec := testutil.GET(t, e, "/cars/999999/history", h)
	require.Equal(t, http.StatusForbidden, rec.Code)
	// Fresh checks observe a personal deny without a new login.
	require.NoError(t, db.Where("user_id=? AND permission_key=?", uid, services.KeyDetailFullHistory).Delete(&models.UserPermissionOverride{}).Error)
	testutil.DenyPermission(t, uid, services.KeyDetailFullHistory)
	rec = testutil.GET(t, e, paths[0], h)
	require.Equal(t, http.StatusOK, rec.Code)
	for _, row := range testutil.ParseSlice(t, rec) {
		require.True(t, services.HistoryReadPolicy{Passages: true}.Allows(row["action_type"].(string)))
	}
}

func TestS5History_RegistryAndAdminJournals(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterManager(t, e, "history_admin", td.OrgID, td.CompanyID)
	uid := getUserID(t, db, "history_admin")
	h := testutil.AuthHeader(token)
	for _, entity := range []string{"cars", "employees"} {
		key := "entity." + entity + ".read"
		testutil.DenyPermission(t, uid, key)
		for _, path := range []string{"/unique-" + entity + "/999999/history", "/unique-" + entity + "/history"} {
			r := testutil.GET(t, e, path, h)
			require.Equal(t, http.StatusForbidden, r.Code, r.Body.String())
		}
		require.NoError(t, db.Where("user_id=? AND permission_key=?", uid, key).Delete(&models.UserPermissionOverride{}).Error)
	}
	testutil.DenyPermission(t, uid, services.KeyDetailFullHistory)
	for _, path := range []string{"/unique-cars/history", "/unique-employees/history"} {
		r := testutil.GET(t, e, path, h)
		require.Equal(t, http.StatusForbidden, r.Code, r.Body.String())
	}
	testutil.DenyPermission(t, uid, services.KeyDetailEntryExitHistory)
	for _, entity := range []string{"cars", "employees"} {
		for _, suffix := range []string{"all", "filter-options"} {
			r := testutil.GET(t, e, "/"+entity+"/history/"+suffix, h)
			require.Equal(t, http.StatusForbidden, r.Code, r.Body.String())
		}
	}
}

func TestS5History_TableJournalAndSuper(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	fx := seedElementScopeFixture(t, db)
	token := testutil.RegisterAndLogin(t, e, "history_guard", "pass123", 1, td.OrgID, td.CompanyID)
	uid := getUserID(t, db, "history_guard")
	testutil.GrantTableVerb(t, uid, fx.tableName, "history")
	testutil.GrantTableVerb(t, uid, fx.otherTableName, "history")
	testutil.DenyPermission(t, uid, services.KeyDetailFullHistory)
	testutil.DenyPermission(t, uid, services.KeyDetailEntryExitHistory)
	for _, target := range []struct{ entity, name string }{{"cars", fx.tableName}, {"employees", fx.otherTableName}} {
		var table models.SystemTable
		require.NoError(t, db.Where("name=?", target.name).First(&table).Error)
		for _, suffix := range []string{"", "/filter-options"} {
			path := fmt.Sprintf("/%s/history/table/%d%s", target.entity, table.ID, suffix)
			rec := testutil.GET(t, e, path, testutil.AuthHeader(token))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		}
	}
	super := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	sid := getUserID(t, db, "testadmin")
	for _, key := range []string{services.KeyDetailFullHistory, services.KeyDetailEntryExitHistory, services.KeyEntityCarsRead, services.KeyEntityEmployeesRead} {
		testutil.DenyPermission(t, sid, key)
	}
	for _, path := range []string{fmt.Sprintf("/cars/%d/history", fx.carID), fmt.Sprintf("/employees/%d/history", fx.empID), "/unique-cars/history", "/unique-employees/history"} {
		rec := testutil.GET(t, e, path, testutil.AuthHeader(super))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
}
