package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

func TestS5Registry_ListAndSearchSections(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "registry_reader", "pass123", 1, td.OrgID, td.CompanyID)
	testutil.RegisterAndLogin(t, e, "registry_other", "pass123", 1, td.OrgID, td.CompanyID)
	uid, other := getUserID(t, db, "registry_reader"), getUserID(t, db, "registry_other")
	assignBaseRole(t, db, "registry_reader")
	h := testutil.AuthHeader(token)
	type ids struct{ car, employee int }
	rows := make([]ids, 4)
	for i, binding := range []struct{ user, org, company *int }{
		{&uid, nil, nil}, {&other, &td.OrgID, nil}, {&other, nil, &td.CompanyID}, {&other, nil, nil},
	} {
		label := fmt.Sprintf("S5Registry%d", i)
		car := models.UniqueCar{Number: &label, Mark: searchStrPtr("Учебная"), UserID: binding.user, OrganizationID: binding.org, CompanyID: binding.company}
		emp := models.UniqueEmployee{LastName: &label, FirstName: searchStrPtr("Тест"), UserID: binding.user, OrganizationID: binding.org, CompanyID: binding.company}
		require.NoError(t, db.Create(&car).Error)
		require.NoError(t, db.Create(&emp).Error)
		rows[i] = ids{car.ID, emp.ID}
	}
	keys := []string{services.KeySectionRegistryOrganization, services.KeySectionRegistryCompany, services.KeySectionRegistryAllSystem}
	for mask := 0; mask < 8; mask++ {
		for i, key := range keys {
			require.NoError(t, db.Where("user_id=? AND permission_key=?", uid, key).Delete(&models.UserPermissionOverride{}).Error)
			if mask&(1<<i) != 0 {
				testutil.GrantPermission(t, uid, key)
			} else {
				testutil.DenyPermission(t, uid, key)
			}
		}
		for _, kind := range []string{"cars", "employees"} {
			idOf := func(row ids) int {
				if kind == "cars" {
					return row.car
				}
				return row.employee
			}
			all := []int{idOf(rows[0])}
			if mask&1 != 0 {
				all = append(all, idOf(rows[1]))
			}
			if mask&2 != 0 {
				all = append(all, idOf(rows[2]))
			}
			for _, pagination := range []string{"", "&per_page=100"} {
				for _, filter := range []string{"user", "all", "organization", "company", "all_system"} {
					rec := testutil.GET(t, e, "/unique-"+kind+"?filter_type="+filter+pagination, h)
					bit := map[string]int{"organization": 1, "company": 2, "all_system": 4}[filter]
					if bit != 0 && mask&bit == 0 {
						require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
						continue
					}
					require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
					expected := []int{idOf(rows[0])}
					switch filter {
					case "all":
						expected = all
					case "organization":
						expected = []int{idOf(rows[1])}
					case "company":
						expected = []int{idOf(rows[2])}
					case "all_system":
						expected = []int{idOf(rows[0]), idOf(rows[1]), idOf(rows[2]), idOf(rows[3])}
					}
					var actual []int
					for _, row := range testutil.ParseSlice(t, rec) {
						actual = append(actual, int(row["id"].(float64)))
					}
					require.ElementsMatch(t, expected, actual, "mask=%d %s %s", mask, kind, filter)
				}
			}
			rec := testutil.GET(t, e, "/search?q=S5Registry", h)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			var actual []int
			for _, group := range decodeSearch(t, rec.Body.String()).Data.Groups {
				if group.Type == kind {
					for _, item := range group.Items {
						actual = append(actual, item.ID)
					}
				}
			}
			expected := all
			if mask&4 != 0 {
				expected = []int{idOf(rows[0]), idOf(rows[1]), idOf(rows[2]), idOf(rows[3])}
			}
			require.ElementsMatch(t, expected, actual, "search mask=%d %s", mask, kind)
		}
	}
	// The final mask permits system-wide reading, without administrative editing.
	for _, target := range []struct {
		kind string
		id   int
		body string
	}{
		{"cars", rows[3].car, `{"number":"Changed","mark":"Учебная"}`},
		{"employees", rows[3].employee, `{"pd_consent":true,"last_name":"Changed","first_name":"Тест"}`},
	} {
		path := fmt.Sprintf("/unique-%s/%d", target.kind, target.id)
		require.Equal(t, http.StatusForbidden, testutil.PUT(t, e, path, target.body, h).Code)
		require.Equal(t, http.StatusForbidden, testutil.DELETE(t, e, path, h).Code)
	}
	var mutations int64
	require.NoError(t, db.Model(&models.AuditLog{}).Where("actor_user_id=? AND action IN ?", uid, []string{"update", "delete"}).Count(&mutations).Error)
	require.Zero(t, mutations)
}

func TestS5Registry_ReadRequiredForMutation(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterManager(t, e, "registry_admin", td.OrgID, td.CompanyID)
	uid := getUserID(t, db, "registry_admin")
	h := testutil.AuthHeader(token)
	for _, kind := range []string{"cars", "employees"} {
		readKey := "entity." + kind + ".read"
		testutil.DenyPermission(t, uid, readKey)
		for _, id := range []int{1, 999999} {
			rec := testutil.PUT(t, e, fmt.Sprintf("/unique-%s/%d", kind, id), `{}`, h)
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			rec = testutil.DELETE(t, e, fmt.Sprintf("/unique-%s/%d", kind, id), h)
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		}
		rec := testutil.POST(t, e, "/unique-"+kind, `{}`, h)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		var count int64
		require.NoError(t, db.Model(&models.AuditLog{}).Where("actor_user_id=?", uid).Count(&count).Error)
		require.Zero(t, count, "denied mutations must not append audit")
	}
}
