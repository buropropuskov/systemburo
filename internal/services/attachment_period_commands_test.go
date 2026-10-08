package services

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func attachmentPeriod2665Fixture() (attachmentPeriodSnapshot, ChangeAttachmentPeriodRequest, time.Time) {
	now := time.Date(2030, 10, 8, 10, 0, 0, 987654321, MoscowLocation())
	appID, active := 9, 1
	old := models.EntryPeriod{EntryDateFrom: ptrString("2030-10-08"), EntryDateTo: ptrString("2030-10-09"), EntryTimeFrom: ptrString("08:00:00"), EntryTimeTo: ptrString("22:00:00")}
	row := attachmentPeriodRow{ID: 2, ApplicationID: &appID, Status: &active, UpdatedAt: now, EntryDateFrom: old.EntryDateFrom, EntryDateTo: old.EntryDateTo, EntryTimeFrom: old.EntryTimeFrom, EntryTimeTo: old.EntryTimeTo}
	entity := func(id int, mode models.PeriodMode) attachmentPeriodEntity {
		return attachmentPeriodEntity{ID: id, AttachmentID: row.ID, PeriodMode: mode, Status: &active, UpdatedAt: now, EntryDateFrom: old.EntryDateFrom, EntryDateTo: old.EntryDateTo, EntryTimeFrom: old.EntryTimeFrom, EntryTimeTo: old.EntryTimeTo}
	}
	snapshot := attachmentPeriodSnapshot{ApplicationID: appID, Status: models.StatusUnread, Confirmation: models.ConfirmationApproved, StatusUpdatedAt: &now, Attachments: []attachmentPeriodRow{row}, Cars: []attachmentPeriodEntity{entity(3, models.PeriodInherit), entity(4, models.PeriodIndividual)}, Employees: []attachmentPeriodEntity{entity(5, models.PeriodInherit), entity(6, models.PeriodIndividual)}}
	req := ChangeAttachmentPeriodRequest{AttachmentIDs: []int{2}, Period: &EntityPeriodInput{EntryDateFrom: "2030-10-08", EntryDateTo: "2030-10-15", EntryTimeFrom: "08:00", EntryTimeTo: "22:00"}, Reason: "  Исправление срока  ", ExpectedRevision: strings.Repeat("a", 64)}
	return snapshot, req, now
}

func TestAttachmentPeriod2665ValidateRequest(t *testing.T) {
	_, req, now := attachmentPeriod2665Fixture()
	req.AttachmentIDs = []int{9, 2}
	validated, next, err := validateAttachmentPeriodRequest(req, true, now)
	require.NoError(t, err)
	require.Equal(t, []int{2, 9}, validated.AttachmentIDs)
	require.Equal(t, []int{9, 2}, req.AttachmentIDs, "validation must not reorder caller's slice")
	require.Equal(t, IndividualPolicyPreserve, validated.IndividualPolicy)
	require.Equal(t, "Исправление срока", validated.Reason)
	require.Equal(t, "08:00:00", *next.EntryTimeFrom)
	require.Equal(t, "2030-10-15", *next.EntryDateTo)
	// Preview requires the exact selection/period/policy/reason, but no token yet.
	req.ExpectedRevision = ""
	_, _, err = validateAttachmentPeriodRequest(req, false, now)
	require.NoError(t, err)
	for _, scenario := range []struct {
		name   string
		mutate func(*ChangeAttachmentPeriodRequest)
	}{
		{"empty IDs", func(r *ChangeAttachmentPeriodRequest) { r.AttachmentIDs = nil }},
		{"duplicate IDs", func(r *ChangeAttachmentPeriodRequest) { r.AttachmentIDs = []int{2, 2} }},
		{"zero ID", func(r *ChangeAttachmentPeriodRequest) { r.AttachmentIDs = []int{0} }},
		{"negative ID", func(r *ChangeAttachmentPeriodRequest) { r.AttachmentIDs = []int{-2} }},
		{"unknown policy", func(r *ChangeAttachmentPeriodRequest) { r.IndividualPolicy = "reset" }},
		{"missing period", func(r *ChangeAttachmentPeriodRequest) { r.Period = nil }},
		{"missing date", func(r *ChangeAttachmentPeriodRequest) { r.Period.EntryDateFrom = "" }},
		{"missing clock", func(r *ChangeAttachmentPeriodRequest) { r.Period.EntryTimeTo = "" }},
		{"invalid calendar date", func(r *ChangeAttachmentPeriodRequest) { r.Period.EntryDateTo = "2030-02-30" }},
		{"expired", func(r *ChangeAttachmentPeriodRequest) {
			r.Period.EntryDateTo = "2030-10-08"
			r.Period.EntryTimeTo = "10:00"
		}},
		{"blank reason", func(r *ChangeAttachmentPeriodRequest) { r.Reason = " \n" }},
		{"long reason", func(r *ChangeAttachmentPeriodRequest) { r.Reason = strings.Repeat("я", 1001) }},
		{"missing revision", func(r *ChangeAttachmentPeriodRequest) { r.ExpectedRevision = "" }},
		{"invalid revision", func(r *ChangeAttachmentPeriodRequest) { r.ExpectedRevision = strings.Repeat("z", 64) }},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			_, input, clock := attachmentPeriod2665Fixture()
			scenario.mutate(&input)
			_, _, err := validateAttachmentPeriodRequest(input, true, clock)
			var httpErr *echo.HTTPError
			require.ErrorAs(t, err, &httpErr)
			require.Equal(t, http.StatusBadRequest, httpErr.Code)
		})
	}
}

func TestAttachmentPeriod2665PlanPreserveReplace(t *testing.T) {
	for _, policy := range []string{IndividualPolicyPreserve, IndividualPolicyReplace} {
		t.Run(policy, func(t *testing.T) {
			snapshot, req, now := attachmentPeriod2665Fixture()
			req.IndividualPolicy = policy
			req, next, err := validateAttachmentPeriodRequest(req, true, now)
			require.NoError(t, err)
			parents, writes, err := planAttachmentPeriodWrites(snapshot, next, policy, now)
			require.NoError(t, err)
			require.Equal(t, []int{2}, parents)
			wantWrites := 2
			if policy == IndividualPolicyReplace {
				wantWrites = 4
			}
			require.Len(t, writes, wantWrites)
			for _, write := range writes {
				require.True(t, equalEntityPeriod(next, write.NewEffective))
				if write.ID == 3 {
					require.True(t, write.WriteOwn, "legacy inherited Car fields stay synchronized")
				}
				if write.ID == 5 {
					require.False(t, write.WriteOwn, "inherited Employee follows parent without materializing own dates")
				}
				if write.ID == 4 || write.ID == 6 {
					require.Equal(t, models.PeriodIndividual, write.Mode)
					require.True(t, write.WriteOwn)
				}
			}
			result, err := attachmentPeriodResult(11, snapshot, req, next, parents, writes)
			require.NoError(t, err)
			require.False(t, result.ApprovalsReset)
			require.Equal(t, 1, result.AttachmentCount)
			require.Equal(t, 2, result.CarCount)
			require.Equal(t, 2, result.EmployeeCount)
			require.Equal(t, 2, result.IndividualCount)
			require.Equal(t, []int{3, 4}, result.Attachments[0].CarIDs)
			require.Equal(t, []int{6}, result.Attachments[0].IndividualEmployeeIDs)
			require.Equal(t, "2030-10-09", *snapshot.Attachments[0].EntryDateTo, "pure planning must not change parent")
			require.Equal(t, "2030-10-09", *snapshot.Cars[1].EntryDateTo, "preserved/old snapshot must not be mutated")
		})
	}
}

func TestAttachmentPeriod2665EqualExplicitIndividualAndByFact(t *testing.T) {
	snapshot, req, now := attachmentPeriod2665Fixture()
	req, next, err := validateAttachmentPeriodRequest(req, true, now)
	require.NoError(t, err)
	// A preserved explicit individual is unchanged even when it has an old
	// ByFact deadline. The unrelated inherited admission may still be changed.
	snapshot.Cars[1].ByFact = true
	_, writes, err := planAttachmentPeriodWrites(snapshot, next, IndividualPolicyPreserve, now)
	require.NoError(t, err)
	require.Len(t, writes, 2)
	_, _, err = planAttachmentPeriodWrites(snapshot, next, IndividualPolicyReplace, now)
	require.Error(t, err, "replacing the actual ByFact individual window must enforce its deadline")
	snapshot.Cars[1].ByFact = false
	snapshot.Cars[0].ByFact = true
	_, _, err = planAttachmentPeriodWrites(snapshot, next, IndividualPolicyPreserve, now)
	require.Error(t, err, "changing a ByFact inherited effective window must enforce its deadline")
	// Explicit individual equal to the parent remains individual; a no-op
	// replacement does not turn it into inheritance or manufacture a write.
	snapshot.Cars[0].ByFact = false
	parents, writes, err := planAttachmentPeriodWrites(snapshot, snapshot.Attachments[0].period(), IndividualPolicyReplace, now)
	require.NoError(t, err)
	require.Empty(t, parents)
	require.Empty(t, writes)
	require.Equal(t, models.PeriodIndividual, snapshot.Cars[1].PeriodMode)
}

func TestAttachmentPeriod2665RevisionCompositionAndPrecision(t *testing.T) {
	snapshot, req, now := attachmentPeriod2665Fixture()
	req, next, err := validateAttachmentPeriodRequest(req, true, now)
	require.NoError(t, err)
	beforeUpdated := snapshot.Cars[0].UpdatedAt
	base, err := attachmentPeriodRevision(11, snapshot, req)
	require.NoError(t, err)
	require.Len(t, base, 64)
	require.Equal(t, beforeUpdated, snapshot.Cars[0].UpdatedAt, "hashing must not mutate a shared snapshot slice")
	snapshot.Attachments[0].UpdatedAt = time.UnixMicro(snapshot.Attachments[0].UpdatedAt.UnixMicro())
	for i := range snapshot.Cars {
		snapshot.Cars[i].UpdatedAt = time.UnixMicro(snapshot.Cars[i].UpdatedAt.UnixMicro())
	}
	for i := range snapshot.Employees {
		snapshot.Employees[i].UpdatedAt = time.UnixMicro(snapshot.Employees[i].UpdatedAt.UnixMicro())
	}
	revision, err := attachmentPeriodRevision(11, snapshot, req)
	require.NoError(t, err)
	require.Equal(t, base, revision, "PostgreSQL microsecond/UTC roundtrip must preserve revision")
	req.ExpectedRevision, req.Reason = strings.Repeat("b", 64), "Free text is excluded from composition hash"
	revision, err = attachmentPeriodRevision(11, snapshot, req)
	require.NoError(t, err)
	require.Equal(t, base, revision)
	otherActor, err := attachmentPeriodRevision(12, snapshot, req)
	require.NoError(t, err)
	require.NotEqual(t, base, otherActor)
	for _, scenario := range []struct {
		name   string
		mutate func(*attachmentPeriodSnapshot, *ChangeAttachmentPeriodRequest)
	}{
		{"membership", func(s *attachmentPeriodSnapshot, _ *ChangeAttachmentPeriodRequest) { s.Employees = s.Employees[:1] }},
		{"mode", func(s *attachmentPeriodSnapshot, _ *ChangeAttachmentPeriodRequest) {
			s.Cars[0].PeriodMode = models.PeriodIndividual
		}},
		{"own period", func(s *attachmentPeriodSnapshot, _ *ChangeAttachmentPeriodRequest) {
			s.Cars[0].EntryTimeTo = ptrString("23:00:00")
		}},
		{"parent", func(s *attachmentPeriodSnapshot, _ *ChangeAttachmentPeriodRequest) {
			s.Attachments[0].EntryTimeTo = ptrString("23:00:00")
		}},
		{"status", func(s *attachmentPeriodSnapshot, _ *ChangeAttachmentPeriodRequest) { s.Status = models.StatusInWork }},
		{"policy", func(_ *attachmentPeriodSnapshot, r *ChangeAttachmentPeriodRequest) {
			r.IndividualPolicy = IndividualPolicyReplace
		}},
		{"selection", func(_ *attachmentPeriodSnapshot, r *ChangeAttachmentPeriodRequest) { r.AttachmentIDs = []int{2, 8} }},
		{"next period", func(_ *attachmentPeriodSnapshot, r *ChangeAttachmentPeriodRequest) {
			r.Period.EntryDateTo = "2030-10-16"
		}},
		{"microsecond", func(s *attachmentPeriodSnapshot, _ *ChangeAttachmentPeriodRequest) {
			s.Cars[0].UpdatedAt = s.Cars[0].UpdatedAt.Add(time.Microsecond)
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			current, input, clock := attachmentPeriod2665Fixture()
			input, _, err := validateAttachmentPeriodRequest(input, true, clock)
			require.NoError(t, err)
			scenario.mutate(&current, &input)
			revision, err := attachmentPeriodRevision(11, current, input)
			require.NoError(t, err)
			require.NotEqual(t, base, revision)
		})
	}
	_, writes, err := planAttachmentPeriodWrites(snapshot, next, IndividualPolicyPreserve, now)
	require.NoError(t, err)
	require.Len(t, writes, 2)
}

func TestAttachmentPeriod2665LifecycleScope(t *testing.T) {
	for _, confirmation := range []string{"", models.ConfirmationPending, models.ConfirmationApproved} {
		require.NoError(t, validateAttachmentPeriodConfirmation(confirmation))
	}
	for _, confirmation := range []string{"rejected", "unknown"} {
		var rejected *echo.HTTPError
		require.ErrorAs(t, validateAttachmentPeriodConfirmation(confirmation), &rejected)
		require.Equal(t, http.StatusBadRequest, rejected.Code)
	}
	set := PermissionSet{allows: map[string]string{KeyApplicationPeriodChange: "override"}}
	for _, status := range []string{models.StatusUnread, models.StatusProcessing} {
		require.NoError(t, AuthorizeAttachmentPeriodChange(set, PeriodChangeTarget{Visible: true, ApplicationStatus: status}))
	}
	for _, status := range []string{models.StatusInWork, models.StatusCompleted, models.StatusWithdrawn, models.StatusRefused, ""} {
		require.Error(t, AuthorizeAttachmentPeriodChange(set, PeriodChangeTarget{Visible: true, ApplicationStatus: status}))
	}
	var forbidden *echo.HTTPError
	require.ErrorAs(t, AuthorizeAttachmentPeriodChange(set, PeriodChangeTarget{Visible: false, Archived: true}), &forbidden)
	require.Equal(t, http.StatusForbidden, forbidden.Code, "visibility must precede lifecycle")
}

func TestAttachmentPeriod2665MultipleAttachmentsAndInvalidMode(t *testing.T) {
	snapshot, req, now := attachmentPeriod2665Fixture()
	second := snapshot.Attachments[0]
	second.ID = 8
	second.EntryDateTo = ptrString("2030-10-12")
	snapshot.Attachments = append(snapshot.Attachments, second)
	neighbor := snapshot.Employees[0]
	neighbor.ID, neighbor.AttachmentID = 20, second.ID
	snapshot.Employees = append(snapshot.Employees, neighbor)
	req.AttachmentIDs = []int{8, 2}
	req, next, err := validateAttachmentPeriodRequest(req, true, now)
	require.NoError(t, err)
	parents, writes, err := planAttachmentPeriodWrites(snapshot, next, req.IndividualPolicy, now)
	require.NoError(t, err)
	require.Equal(t, []int{2, 8}, parents)
	result, err := attachmentPeriodResult(11, snapshot, req, next, parents, writes)
	require.NoError(t, err)
	require.Equal(t, []int{2, 8}, result.AttachmentIDs)
	require.Equal(t, []int{5, 20}, result.ChangedEmployeeIDs)
	require.Equal(t, []int{3}, result.ChangedCarIDs)
	require.Equal(t, 2, result.AttachmentCount)
	require.Equal(t, 3, result.EmployeeCount)
	snapshot.Cars[0].PeriodMode = models.PeriodMode("unknown")
	_, _, err = planAttachmentPeriodWrites(snapshot, next, IndividualPolicyPreserve, now)
	require.Error(t, err)
	snapshot.Cars[0].PeriodMode = models.PeriodIndividual
	snapshot.Cars[0].EntryDateTo = nil
	_, _, err = planAttachmentPeriodWrites(snapshot, next, IndividualPolicyPreserve, now)
	require.Error(t, err, "invalid individual must not inherit from a valid parent")
}
