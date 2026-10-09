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

func TestOpenPassages2667TableScope(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	seed := testutil.SeedTestData(t, db)
	owner := testutil.RegisterAndLogin(t, e, "open_scope_owner", "pass123", 1, seed.OrgID, seed.CompanyID)
	stranger := testutil.RegisterAndLogin(t, e, "open_scope_stranger", "pass123", 1, seed.OrgID, seed.CompanyID)
	ownerID := getUserID(t, db, "open_scope_owner")
	for _, kind := range []string{"people", "cars"} {
		table := models.SystemTable{Name: "open_scope_" + kind, TableType: kind, IsActive: true, Status: "active"}
		require.NoError(t, db.Create(&table).Error)
		testutil.GrantTableVerb(t, ownerID, table.Name, "view")
		active, inside := 1, 1
		attachment := models.Attachment{AttachmentType: kind, IsManual: true, OrganizationID: &seed.OrgID, Status: &active}
		require.NoError(t, db.Create(&attachment).Error)
		for i, label := range []string{"BoundScope2667", "ForeignScope2667"} {
			if kind == "cars" {
				car := models.Car{AttachmentID: attachment.ID, CarNumber: &label, Status: &active, TerritoryStatus: &inside}
				require.NoError(t, db.Create(&car).Error)
				if i == 0 {
					bindPassageFixtureCar2667(t, db, car.ID, table.ID)
				}
			} else {
				employee := models.Employee{AttachmentID: &attachment.ID, LastName: &label, Status: &active, TerritoryStatus: &inside}
				require.NoError(t, db.Create(&employee).Error)
				if i == 0 {
					require.NoError(t, db.Create(&models.EmployeeTargetTable{EmployeeID: employee.ID, TableID: table.ID, Source: "manual"}).Error)
				}
			}
		}
		endpoint := "employees"
		if kind == "cars" {
			endpoint = "cars"
		}
		url := fmt.Sprintf("/%s/open-for-table/%d?attention_only=false", endpoint, table.ID)
		legitimate := testutil.GET(t, e, url, testutil.AuthHeader(owner))
		require.Equal(t, http.StatusOK, legitimate.Code, legitimate.Body.String())
		list := testutil.ParseResponse[services.PassageOpenList](t, legitimate)
		require.Len(t, list.Items, 1)
		require.EqualValues(t, 1, list.Total)
		require.Contains(t, legitimate.Body.String(), "BoundScope2667")
		require.NotContains(t, legitimate.Body.String(), "ForeignScope2667")
		foreign := testutil.GET(t, e, url, testutil.AuthHeader(stranger))
		require.Equal(t, http.StatusForbidden, foreign.Code, foreign.Body.String())
		// A missing table is also forbidden; its existence is not disclosed.
		missing := testutil.GET(t, e, fmt.Sprintf("/%s/open-for-table/999999", endpoint), testutil.AuthHeader(stranger))
		require.Equal(t, http.StatusForbidden, missing.Code, missing.Body.String())
	}
}
