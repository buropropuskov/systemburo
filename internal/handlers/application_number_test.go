package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/entityarchive"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
)

func TestApplicationNumbersSurvivePurgeAcrossSubmissionPaths(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	data := testutil.SeedTestData(t, db)
	token := testutil.RegisterAndLogin(t, e, "number-owner", "pass123", 1, data.OrgID, data.CompanyID)
	templateID := seedUniqueAttachment(t, db, "cars", "number_cars", "Транспорт")
	body := withApplicationConfirmation(t, db, fmt.Sprintf(`{"organization_id":%d,"data_approval":true}`, data.OrgID))
	seen := make(map[string]bool)
	create := func() int {
		t.Helper()
		rec := testutil.POST(t, e, "/applications", body, testutil.AuthHeader(token))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		app := testutil.ParseResponse[services.ApplicationCreateResponse](t, rec)
		require.NotEmpty(t, app.ApplicationNumber)
		require.False(t, seen[app.ApplicationNumber], "повтор номера %s", app.ApplicationNumber)
		seen[app.ApplicationNumber] = true
		return app.ApplicationID
	}
	purge := func(id int) {
		t.Helper()
		_, err := entityarchive.PurgeApplication(context.Background(), db, services.NewAuditRecorder(db),
			id, entityarchive.DestructionOptions{Basis: entityarchive.BasisOperator, Apply: true})
		require.NoError(t, err)
	}
	create()
	middle := create()
	last := submitCompleteApplication(t, db, e, token, "Test Organization", templateID)
	var number string
	require.NoError(t, db.Raw("SELECT application_number FROM applications WHERE id = ?", last).Row().Scan(&number))
	require.NotEmpty(t, number)
	require.False(t, seen[number])
	seen[number] = true
	purge(middle)
	newLast := create()
	purge(newLast)
	create()
}
