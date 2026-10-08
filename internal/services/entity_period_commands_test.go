package services

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func periodCommandFixture() (entityPeriodSnapshot, ChangeEntityPeriodRequest, time.Time) {
	now := time.Date(2030, 10, 8, 10, 0, 0, 0, MoscowLocation())
	active, applicationID := 1, 9
	p := models.EntryPeriod{EntryDateFrom: ptrString("2030-10-08"), EntryDateTo: ptrString("2030-10-09"), EntryTimeFrom: ptrString("08:00:00"), EntryTimeTo: ptrString("22:00:00")}
	snapshot := entityPeriodSnapshot{ID: 7, AttachmentID: 8, ApplicationID: &applicationID, ApplicationStatus: models.StatusInWork, Status: &active, AttachmentStatus: &active, PeriodMode: models.PeriodInherit, Parent: p, UpdatedAt: now.UTC()}
	req := ChangeEntityPeriodRequest{PeriodMode: models.PeriodIndividual, Period: &EntityPeriodInput{EntryDateFrom: "2030-10-08", EntryDateTo: "2030-10-15", EntryTimeFrom: "08:00", EntryTimeTo: "22:00"}, Reason: "  Продление по обращению  ", ExpectedRevision: strings.Repeat("a", 64)}
	return snapshot, req, now
}

func TestValidateEntityPeriodCommandIndividualBeyondParent(t *testing.T) {
	snapshot, req, now := periodCommandFixture()
	next, reason, err := validateEntityPeriodCommand(req, snapshot, now)
	require.NoError(t, err)
	require.Equal(t, "2030-10-15", *next.EntryDateTo)
	require.Equal(t, "08:00:00", *next.EntryTimeFrom)
	require.Equal(t, "Продление по обращению", reason)
	require.Equal(t, "2030-10-09", *snapshot.Parent.EntryDateTo)
	// Multi-day hours define one continuous window, not a daily prohibition.
	req.Period.EntryTimeFrom, req.Period.EntryTimeTo = "22:00", "06:00"
	_, _, err = validateEntityPeriodCommand(req, snapshot, now)
	require.NoError(t, err)
}

func TestValidateEntityPeriodCommandExplicitInheritance(t *testing.T) {
	snapshot, req, now := periodCommandFixture()
	snapshot.PeriodMode, snapshot.Own = models.PeriodIndividual, snapshot.Parent
	req.PeriodMode, req.Period = models.PeriodInherit, nil
	next, _, err := validateEntityPeriodCommand(req, snapshot, now)
	require.NoError(t, err)
	require.Equal(t, models.EntryPeriod{}, next, "inherit clears the complete own window")
	req.Period = &EntityPeriodInput{}
	_, _, err = validateEntityPeriodCommand(req, snapshot, now)
	require.Error(t, err, "inherit must not silently discard a supplied individual window")

	req.Period = nil
	snapshot.Manual, snapshot.ApplicationID, snapshot.Parent = true, nil, models.EntryPeriod{}
	_, _, err = validateEntityPeriodCommand(req, snapshot, now)
	require.NoError(t, err, "explicit inheritance can restore an existing unbounded manual parent")
	snapshot.Manual = false
	_, _, err = validateEntityPeriodCommand(req, snapshot, now)
	require.Error(t, err, "an invalid nonmanual parent must not become unlimited")
}

func TestValidateEntityPeriodCommandRejectsInvalidPayloads(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*ChangeEntityPeriodRequest)
	}{
		{"unknown mode", func(r *ChangeEntityPeriodRequest) { r.PeriodMode = "other" }},
		{"missing period", func(r *ChangeEntityPeriodRequest) { r.Period = nil }},
		{"missing start clock", func(r *ChangeEntityPeriodRequest) { r.Period.EntryTimeFrom = "" }},
		{"missing end clock", func(r *ChangeEntityPeriodRequest) { r.Period.EntryTimeTo = "" }},
		{"reversed dates", func(r *ChangeEntityPeriodRequest) { r.Period.EntryDateTo = "2030-10-07" }},
		{"equal moments", func(r *ChangeEntityPeriodRequest) {
			r.Period.EntryDateTo = "2030-10-08"
			r.Period.EntryTimeTo = "08:00"
		}},
		{"exact expired end", func(r *ChangeEntityPeriodRequest) {
			r.Period.EntryDateTo = "2030-10-08"
			r.Period.EntryTimeTo = "10:00"
		}},
		{"empty reason", func(r *ChangeEntityPeriodRequest) { r.Reason = " \n " }},
		{"long Unicode reason", func(r *ChangeEntityPeriodRequest) { r.Reason = strings.Repeat("я", 1001) }},
		{"missing revision", func(r *ChangeEntityPeriodRequest) { r.ExpectedRevision = "" }},
		{"invalid revision", func(r *ChangeEntityPeriodRequest) { r.ExpectedRevision = strings.Repeat("z", 64) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, req, now := periodCommandFixture()
			test.mutate(&req)
			_, _, err := validateEntityPeriodCommand(req, snapshot, now)
			require.Error(t, err)
		})
	}
}

func TestValidateEntityPeriodCommandByFactAppliesToEffectiveWindow(t *testing.T) {
	snapshot, req, now := periodCommandFixture()
	snapshot.ByFact = true
	_, _, err := validateEntityPeriodCommand(req, snapshot, now)
	require.Error(t, err)
	req.Period.EntryDateTo = ByFactMaxDate(now)
	_, _, err = validateEntityPeriodCommand(req, snapshot, now)
	require.NoError(t, err)
	snapshot.Parent.EntryDateTo = ptrString("2030-10-15")
	req.PeriodMode, req.Period = models.PeriodInherit, nil
	_, _, err = validateEntityPeriodCommand(req, snapshot, now)
	require.Error(t, err, "inherit cannot bypass a ByFact deadline")
}

func TestEntityPeriodRevisionBindsActorKindAndState(t *testing.T) {
	snapshot, _, _ := periodCommandFixture()
	first, err := entityPeriodRevision(1, ElementEmployee, snapshot)
	require.NoError(t, err)
	same, err := entityPeriodRevision(1, ElementEmployee, snapshot)
	require.NoError(t, err)
	require.Equal(t, first, same)
	actor, _ := entityPeriodRevision(2, ElementEmployee, snapshot)
	kind, _ := entityPeriodRevision(1, ElementCar, snapshot)
	require.NotEqual(t, first, actor)
	require.NotEqual(t, first, kind)
	snapshot.PeriodMode, snapshot.Own = models.PeriodIndividual, snapshot.Parent
	mode, _ := entityPeriodRevision(1, ElementEmployee, snapshot)
	require.NotEqual(t, first, mode, "equal effective values must not hide an explicit mode change")
	snapshot.ApplicationStatus = models.StatusCompleted
	state, _ := entityPeriodRevision(1, ElementEmployee, snapshot)
	require.NotEqual(t, mode, state)
}

func TestEntityPeriodCommandFailsClosedWithoutDependencies(t *testing.T) {
	var service *EntityPeriodCommandService
	_, err := service.Change(context.Background(), 1, ElementEmployee, 1, ChangeEntityPeriodRequest{})
	require.Error(t, err)
	service = NewEntityPeriodCommandService(nil, nil)
	_, err = service.Inspect(context.Background(), 1, ElementEmployee, 1, nil)
	require.Error(t, err)
}

func TestEntityPeriodRevisionPreservesPostgresTimestampPrecision(t *testing.T) {
	snapshot, _, _ := periodCommandFixture()
	raw := time.Date(2030, 10, 8, 10, 15, 20, 987654321, MoscowLocation())
	writtenAt := canonicalEntityPeriodTime(raw)
	require.Equal(t, 987654000, writtenAt.Nanosecond())
	require.Equal(t, time.UTC, writtenAt.Location())
	snapshot.UpdatedAt = writtenAt
	snapshot.AttachmentUpdatedAt = raw
	snapshot.ApplicationStatusUpdatedAt = &raw
	returned, err := entityPeriodRevision(1, ElementEmployee, snapshot)
	require.NoError(t, err)
	// Simulate pgx/PostgreSQL's microsecond round trip, including UTC decoding
	// of related timestamps. Root integration tests verify the real DB path.
	persisted := time.UnixMicro(writtenAt.UnixMicro()).UTC()
	snapshot.UpdatedAt = persisted
	snapshot.AttachmentUpdatedAt = persisted
	snapshot.ApplicationStatusUpdatedAt = &persisted
	inspected, err := entityPeriodRevision(1, ElementEmployee, snapshot)
	require.NoError(t, err)
	require.Equal(t, returned, inspected, "an unchanged persisted row must not produce a false revision conflict")
	snapshot.UpdatedAt = persisted.Add(time.Microsecond)
	changed, err := entityPeriodRevision(1, ElementEmployee, snapshot)
	require.NoError(t, err)
	require.NotEqual(t, returned, changed, "a real persisted timestamp change must still invalidate revision")
}

func TestEntityPeriodCommandAuthorizationKeepsWorkOnly(t *testing.T) {
	set := PermissionSet{allows: map[string]string{KeyDetailPeriodChange: SourceOverride}}
	for _, status := range []string{models.StatusUnread, models.StatusProcessing, models.StatusCompleted, models.StatusWithdrawn} {
		err := AuthorizeEntityPeriodChange(set, PeriodChangeTarget{Visible: true, Active: true, ApplicationStatus: status})
		var httpErr *echo.HTTPError
		require.ErrorAs(t, err, &httpErr)
		require.Equal(t, http.StatusBadRequest, httpErr.Code)
	}
	require.NoError(t, AuthorizeEntityPeriodChange(set, PeriodChangeTarget{Visible: true, Active: true, ApplicationStatus: models.StatusInWork}))
	require.NoError(t, AuthorizeEntityPeriodChange(set, PeriodChangeTarget{Visible: true, Active: true, Manual: true}))
}
