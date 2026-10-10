package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"
)

func TestTakeToWorkAdmissionExpiry2674(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	for _, scenario := range []struct {
		name                                   string
		parentDays                             int
		individualKind                         string
		unbounded, secondFuture, hourlyExpired bool
		want                                   int
	}{
		{name: "expired_before_worker", parentDays: -2, want: http.StatusConflict},
		{name: "future", parentDays: 2, want: http.StatusOK},
		{name: "unbounded", unbounded: true, want: http.StatusOK},
		{name: "another_attachment_survives", parentDays: -2, secondFuture: true, want: http.StatusOK},
		{name: "individual_employee_survives", parentDays: -2, individualKind: "people", want: http.StatusOK},
		{name: "individual_car_survives", parentDays: -2, individualKind: "cars", want: http.StatusOK},
		{name: "past_hour_today", hourlyExpired: true, want: http.StatusConflict},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			testutil.CleanDB(t, db)
			seed := testutil.SeedTestData(t, db)
			token := testutil.RegisterAndLogin(t, e, "expiry2674", "fixture2674", 1, seed.OrgID, seed.CompanyID)
			makeApprover(t, db, "expiry2674")
			actor := getUserID(t, db, "expiry2674")
			var clock time.Time
			require.NoError(t, db.Raw("SELECT CURRENT_TIMESTAMP").Scan(&clock).Error)
			clock = clock.In(services.MoscowLocation())
			status := models.StatusProcessing
			app := models.Application{OrganizationID: seed.OrgID, SenderUserID: actor, Status: &status}
			require.NoError(t, db.Create(&app).Error)
			from, end := clock.AddDate(0, 0, -4).Format("2006-01-02"), clock.AddDate(0, 0, scenario.parentDays).Format("2006-01-02")
			inactive := 0
			kind := "cars"
			if scenario.individualKind != "" {
				kind = scenario.individualKind
			}
			attachment := models.Attachment{ApplicationID: &app.ID, AttachmentType: kind, Status: &inactive, EntryDateFrom: &from, EntryDateTo: &end}
			if scenario.unbounded {
				attachment.EntryDateTo = nil
			}
			if scenario.hourlyExpired {
				// One database-derived reference handles midnight and month/year rollover.
				past := clock.Add(-2 * time.Minute)
				end, hour := past.Format("2006-01-02"), past.Format("15:04:05")
				attachment.EntryDateTo, attachment.EntryTimeTo = &end, &hour
			}
			require.NoError(t, db.Create(&attachment).Error)
			if scenario.secondFuture {
				future := clock.AddDate(0, 0, 2).Format("2006-01-02")
				other := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars", Status: &inactive, EntryDateTo: &future}
				require.NoError(t, db.Create(&other).Error)
			}
			if scenario.individualKind != "" {
				future := clock.AddDate(0, 0, 2).Format("2006-01-02")
				if scenario.individualKind == "cars" {
					car := models.Car{AttachmentID: attachment.ID, Status: &inactive}
					require.NoError(t, db.Create(&car).Error)
					require.NoError(t, db.Exec("UPDATE cars SET period_mode='individual', entry_date_from=?, entry_date_to=? WHERE id=?", from, future, car.ID).Error)
				} else {
					person := models.Employee{AttachmentID: &attachment.ID, Status: &inactive}
					require.NoError(t, db.Create(&person).Error)
					require.NoError(t, db.Exec("UPDATE employees SET period_mode='individual', entry_date_from=?, entry_date_to=? WHERE id=?", from, future, person.ID).Error)
				}
			}
			rec := testutil.POST(t, e, fmt.Sprintf("/applications/%d/take-to-work", app.ID), `{"action":"accept"}`, testutil.AuthHeader(token))
			require.Equal(t, scenario.want, rec.Code, rec.Body.String())
			var saved models.Application
			require.NoError(t, db.First(&saved, app.ID).Error)
			if scenario.want == http.StatusConflict {
				require.Equal(t, models.StatusProcessing, *saved.Status)
				require.Nil(t, saved.AcceptedAt)
				require.Nil(t, saved.Confirmation)
			} else {
				require.Equal(t, models.StatusInWork, *saved.Status)
			}
		})
	}
}
