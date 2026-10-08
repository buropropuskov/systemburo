package handlers_test

import (
	"context"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
)

type archivePeriod2665Fixture struct {
	world period2665World
	apps  services.ApplicationService
	orgID int
}

func setupArchivePeriod2665Fixture(t *testing.T) archivePeriod2665Fixture {
	t.Helper()
	w := setupPeriod2665World(t, services.ElementEmployee)
	var orgID int
	require.NoError(t, w.db.Table("applications").Select("organization_id").Where("id=?", w.application).Scan(&orgID).Error)
	return archivePeriod2665Fixture{world: w, orgID: orgID, apps: services.NewApplicationService(w.db, services.NewPermissionService(w.db), nil, nil, nil, services.NewAuditRecorder(w.db))}
}

func (f archivePeriod2665Fixture) app(t *testing.T) int {
	t.Helper()
	status, confirmation := models.StatusCompleted, models.ConfirmationApproved
	app := models.Application{OrganizationID: f.orgID, SenderUserID: f.world.actor, Status: &status, Confirmation: &confirmation, SendingDatetime: &f.world.clock}
	require.NoError(t, f.world.db.Create(&app).Error)
	return app.ID
}

func (f archivePeriod2665Fixture) window(from, to int) models.EntryPeriod {
	start, end := f.world.clock.AddDate(0, 0, from).Format("2006-01-02"), f.world.clock.AddDate(0, 0, to).Format("2006-01-02")
	open, close := "08:00:00", "22:00:00"
	return models.EntryPeriod{EntryDateFrom: &start, EntryDateTo: &end, EntryTimeFrom: &open, EntryTimeTo: &close}
}

func (f archivePeriod2665Fixture) attachment(t *testing.T, appID *int, kind string, period models.EntryPeriod, manual bool) int {
	t.Helper()
	active := 1
	attachment := models.Attachment{ApplicationID: appID, AttachmentType: kind, IsManual: manual, Status: &active,
		EntryDateFrom: period.EntryDateFrom, EntryDateTo: period.EntryDateTo, EntryTimeFrom: period.EntryTimeFrom, EntryTimeTo: period.EntryTimeTo}
	require.NoError(t, f.world.db.Create(&attachment).Error)
	return attachment.ID
}

func (f archivePeriod2665Fixture) entity(t *testing.T, kind services.ElementKind, attID int, mode models.PeriodMode, own models.EntryPeriod, active int) {
	t.Helper()
	if kind == services.ElementEmployee {
		require.NoError(t, f.world.db.Create(&models.Employee{AttachmentID: &attID, PeriodMode: mode, Status: &active,
			EntryDateFrom: own.EntryDateFrom, EntryDateTo: own.EntryDateTo, EntryTimeFrom: own.EntryTimeFrom, EntryTimeTo: own.EntryTimeTo}).Error)
	} else {
		plate := "ARCHIVE-PERIOD2665"
		require.NoError(t, f.world.db.Create(&models.Car{AttachmentID: attID, CarNumber: &plate, PeriodMode: mode, Status: &active,
			EntryDateFrom: own.EntryDateFrom, EntryDateTo: own.EntryDateTo, EntryTimeFrom: own.EntryTimeFrom, EntryTimeTo: own.EntryTimeTo}).Error)
	}
}

// GetApplications is the production service behind both listing endpoints.
// The same reader must partition archive/default and independently apply today.
func (f archivePeriod2665Fixture) assertLists(t *testing.T, appID int, archived, today bool) {
	t.Helper()
	snapshot := func() string {
		var value string
		require.NoError(t, f.world.db.Raw(`SELECT jsonb_build_object(
			'app', (SELECT to_jsonb(a) FROM applications a WHERE a.id=?),
			'attachments', (SELECT jsonb_agg(to_jsonb(a) ORDER BY a.id) FROM attachments a WHERE a.application_id=?),
			'people', (SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM employees e JOIN attachments a ON a.id=e.attachment_id WHERE a.application_id=?),
			'cars', (SELECT jsonb_agg(to_jsonb(c) ORDER BY c.id) FROM cars c JOIN attachments a ON a.id=c.attachment_id WHERE a.application_id=?)
		)::text`, appID, appID, appID, appID).Scan(&value).Error)
		return value
	}
	before := snapshot()
	yes := true
	for _, check := range []struct {
		name   string
		filter services.ApplicationFilter
		want   bool
	}{
		{"archive", services.ApplicationFilter{Archive: &yes}, archived},
		{"default", services.ApplicationFilter{}, !archived},
		{"today", services.ApplicationFilter{Archive: &archived, ActiveToday: &yes}, today},
	} {
		rows, err := f.apps.GetApplications(context.Background(), "period2665_actor", check.filter)
		require.NoError(t, err, check.name)
		found := false
		for _, row := range rows {
			found = found || row.ID == appID
		}
		require.Equal(t, check.want, found, check.name)
	}
	require.Equal(t, before, snapshot(), "listing must preserve historical source rows")
}

func TestEntityPeriod2665DBArchiveAndActiveTodayEffectiveWindows(t *testing.T) {
	f := setupArchivePeriod2665Fixture(t)
	for _, kind := range []services.ElementKind{services.ElementEmployee, services.ElementCar} {
		for _, scenario := range []string{"individual_beyond_expired_parent", "expired_individual_future_parent", "inherit_cleared", "historical_inactive_individual"} {
			t.Run(string(kind)+"/"+scenario, func(t *testing.T) {
				appID := f.app(t)
				parent, own := f.window(-100, -60), f.window(-2, 8)
				mode, active, archived, today := models.PeriodIndividual, 1, false, true
				switch scenario {
				case "expired_individual_future_parent":
					parent, own, archived, today = f.window(-2, 8), f.window(-100, -60), true, false
				case "inherit_cleared":
					parent, own, mode = f.window(-2, 8), models.EntryPeriod{}, models.PeriodInherit
				case "historical_inactive_individual":
					active = 0
				}
				attachmentType := "people"
				if kind == services.ElementCar {
					attachmentType = "cars"
				}
				attID := f.attachment(t, &appID, attachmentType, parent, false)
				f.entity(t, kind, attID, mode, own, active)
				f.assertLists(t, appID, archived, today)
			})
		}
	}
}

func TestEntityPeriod2665DBArchiveAndActiveTodayFallbacks(t *testing.T) {
	f := setupArchivePeriod2665Fixture(t)
	for _, scenario := range []string{"empty_people", "empty_cars", "items", "mixed_fallback", "inherit_unbounded", "manual_unbounded_orphan"} {
		t.Run(scenario, func(t *testing.T) {
			appID := f.app(t)
			archived, today := false, true
			switch scenario {
			case "empty_people", "empty_cars", "items":
				kind := "items"
				if scenario == "empty_people" {
					kind = "people"
				} else if scenario == "empty_cars" {
					kind = "cars"
				}
				f.attachment(t, &appID, kind, f.window(-2, 8), false)
			case "mixed_fallback":
				attID := f.attachment(t, &appID, "people", f.window(-100, -60), false)
				f.entity(t, services.ElementEmployee, attID, models.PeriodIndividual, f.window(-100, -60), 0)
				f.attachment(t, &appID, "items", f.window(-2, 8), false)
			case "inherit_unbounded":
				attID := f.attachment(t, &appID, "cars", models.EntryPeriod{}, false)
				f.entity(t, services.ElementCar, attID, models.PeriodInherit, models.EntryPeriod{}, 1)
				today = false // ActiveToday keeps its existing finite-calendar rule.
			case "manual_unbounded_orphan":
				f.attachment(t, &appID, "items", f.window(-100, -60), false)
				orphan := f.attachment(t, nil, "people", models.EntryPeriod{}, true)
				f.entity(t, services.ElementEmployee, orphan, models.PeriodInherit, models.EntryPeriod{}, 1)
				archived, today = true, false // An unrelated manual orphan cannot hold this app open.
			}
			f.assertLists(t, appID, archived, today)
		})
	}
}
