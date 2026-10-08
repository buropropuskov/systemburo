package services

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
)

func manual2665Window(from, to, start, end string) models.EntryPeriod {
	return models.EntryPeriod{EntryDateFrom: &from, EntryDateTo: &to, EntryTimeFrom: &start, EntryTimeTo: &end}
}

func TestManualAttach2665ByFactOnlyGuardsChangedEffectiveWindow(t *testing.T) {
	now := time.Date(2031, 1, 1, 12, 0, 0, 0, MoscowLocation())
	long := manual2665Window("2031-01-01", "2031-01-06", "08:00:00", "22:00:00")
	allowed := manual2665Window("2031-01-01", ByFactMaxDate(now), "08:00:00", "22:00:00")
	empty := models.EntryPeriod{}
	for _, mode := range []models.PeriodMode{models.PeriodInherit, models.PeriodIndividual} {
		err := manualAttachValidateByFact(" По Факту ", models.PeriodInherit, empty, empty, mode, long, long, now)
		var httpErr *echo.HTTPError
		require.ErrorAs(t, err, &httpErr)
		require.Equal(t, http.StatusBadRequest, httpErr.Code)
		require.NoError(t, manualAttachValidateByFact("По факту", models.PeriodInherit, empty, empty, mode, allowed, allowed, now))
	}
	require.NoError(t, manualAttachValidateByFact("По факту", models.PeriodIndividual, long, empty, models.PeriodIndividual, long, allowed, now), "preserved individual window is not a new extension")
	require.NoError(t, manualAttachValidateByFact("По факту", models.PeriodInherit, empty, long, models.PeriodIndividual, long, allowed, now), "materialization alone does not change effective validity")
	require.NoError(t, manualAttachValidateByFact("TEST2665", models.PeriodInherit, empty, empty, models.PeriodIndividual, long, allowed, now))
}

func TestManualAttach2665RequestChoiceContract(t *testing.T) {
	app, target, source := 10, 20, 30
	period := &EntityPeriodInput{EntryDateFrom: "2031-01-01", EntryDateTo: "2031-01-02", EntryTimeFrom: "08:00", EntryTimeTo: "18:00"}
	for _, tc := range []struct {
		name  string
		req   AttachToApplicationRequest
		valid bool
	}{
		{"legacy adopt", AttachToApplicationRequest{ApplicationID: &app}, true},
		{"legacy reattach", AttachToApplicationRequest{TargetAttachmentID: &target}, true},
		{"source", AttachToApplicationRequest{ApplicationID: &app, PeriodChoice: "source", SourceAttachmentID: &source}, true},
		{"individual", AttachToApplicationRequest{TargetAttachmentID: &target, PeriodChoice: "individual", Period: period}, true},
		{"no target", AttachToApplicationRequest{}, false},
		{"two targets", AttachToApplicationRequest{ApplicationID: &app, TargetAttachmentID: &target}, false},
		{"implicit source", AttachToApplicationRequest{ApplicationID: &app, SourceAttachmentID: &source}, false},
		{"implicit dates", AttachToApplicationRequest{ApplicationID: &app, Period: period}, false},
		{"source missing", AttachToApplicationRequest{ApplicationID: &app, PeriodChoice: "source"}, false},
		{"source and dates", AttachToApplicationRequest{ApplicationID: &app, PeriodChoice: "source", SourceAttachmentID: &source, Period: period}, false},
		{"individual and source", AttachToApplicationRequest{ApplicationID: &app, PeriodChoice: "individual", SourceAttachmentID: &source, Period: period}, false},
		{"unknown choice", AttachToApplicationRequest{ApplicationID: &app, PeriodChoice: "inherit"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateManualAttachRequest(1, tc.req)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	var wire AttachToApplicationRequest
	require.NoError(t, json.Unmarshal([]byte(`{"target_attachment_id":20,"period_choice":"source","source_attachment_id":30}`), &wire))
	require.Equal(t, "source", wire.PeriodChoice)
	require.Equal(t, source, *wire.SourceAttachmentID)
}

func TestManualAttach2665UnboundedNeedsExplicitFiniteChoice(t *testing.T) {
	now := time.Date(2031, 1, 1, 9, 0, 0, 0, time.UTC)
	_, err := manualAttachChosenWindow(AttachToApplicationRequest{}, true, nil, now)
	require.Error(t, err)
	p := EntityPeriodInput{EntryDateFrom: "2031-01-01", EntryDateTo: "2031-01-05", EntryTimeFrom: "08:00", EntryTimeTo: "18:00"}
	chosen, err := manualAttachChosenWindow(AttachToApplicationRequest{PeriodChoice: "individual", Period: &p}, true, nil, now)
	require.NoError(t, err)
	require.Equal(t, "18:00:00", *chosen.EntryTimeTo)
	for _, mutate := range []func(*EntityPeriodInput){
		func(p *EntityPeriodInput) { p.EntryTimeFrom = "" },
		func(p *EntityPeriodInput) { p.EntryTimeTo = "" },
		func(p *EntityPeriodInput) { p.EntryDateFrom = "" },
		func(p *EntityPeriodInput) { p.EntryDateTo = "" },
		func(p *EntityPeriodInput) { p.EntryDateTo = "2030-12-31" },
		func(p *EntityPeriodInput) { p.EntryTimeTo = "99:00" },
	} {
		invalid := p
		mutate(&invalid)
		_, err := manualAttachChosenWindow(AttachToApplicationRequest{PeriodChoice: "individual", Period: &invalid}, true, nil, now)
		require.Error(t, err)
	}
	from, to := "2031-01-01", "2031-01-05"
	source := models.Attachment{EntryDateFrom: &from, EntryDateTo: &to}
	chosen, err = manualAttachChosenWindow(AttachToApplicationRequest{PeriodChoice: "source"}, true, &source, now)
	require.NoError(t, err)
	require.Equal(t, "00:00:00", *chosen.EntryTimeFrom)
	require.Equal(t, "23:59:59", *chosen.EntryTimeTo)
	_, err = manualAttachChosenWindow(AttachToApplicationRequest{PeriodChoice: "source"}, true, &models.Attachment{}, now)
	require.Error(t, err)
	_, err = manualAttachChosenWindow(AttachToApplicationRequest{PeriodChoice: "source"}, false, &source, now)
	require.Error(t, err, "finite manual must not acquire a different period through this operation")
}

func TestManualAttach2665PreservesIndividualBeyondParent(t *testing.T) {
	own := manual2665Window("2031-01-01", "2031-02-20", "08:00", "18:00")
	parent := manual2665Window("2031-01-01", "2031-01-10", "08:00", "18:00")
	for _, old := range []models.EntryPeriod{{}, parent} {
		mode, next, changed, err := manualAttachNextPeriod(models.PeriodIndividual, own, old, parent, parent, AttachToApplicationRequest{}, false, false)
		require.NoError(t, err)
		require.Equal(t, models.PeriodIndividual, mode)
		require.Equal(t, own, next)
		require.False(t, changed)
	}
	_, _, _, err := manualAttachNextPeriod(models.PeriodMode("bad"), own, parent, parent, parent, AttachToApplicationRequest{}, false, false)
	require.Error(t, err)
}

func TestManualAttach2665FiniteInheritKeepsWholeParentGuard(t *testing.T) {
	parent := manual2665Window("2031-01-01", "2031-01-20", "08:00", "18:00")
	narrow := manual2665Window("2031-01-01", "2031-01-10", "08:00", "18:00")
	// Own NULL after explicit inherit must not bypass the parent's bounds.
	_, _, _, err := manualAttachNextPeriod(models.PeriodInherit, models.EntryPeriod{}, parent, narrow, models.EntryPeriod{}, AttachToApplicationRequest{}, false, false)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusUnprocessableEntity, httpErr.Code)
	mode, own, changed, err := manualAttachNextPeriod(models.PeriodInherit, models.EntryPeriod{}, parent, parent, models.EntryPeriod{}, AttachToApplicationRequest{}, false, false)
	require.NoError(t, err)
	require.Equal(t, models.PeriodIndividual, mode)
	require.Equal(t, parent, own)
	require.True(t, changed)
	wider := manual2665Window("2031-01-01", "2031-02-20", "00:00", "23:59:59")
	mode, own, changed, err = manualAttachNextPeriod(models.PeriodInherit, models.EntryPeriod{}, parent, wider, models.EntryPeriod{}, AttachToApplicationRequest{}, false, false)
	require.NoError(t, err)
	require.Equal(t, models.PeriodIndividual, mode)
	require.Equal(t, parent, own, "moving to a wider parent must not extend the old effective window")
	require.True(t, changed)
	mode, own, changed, err = manualAttachNextPeriod(models.PeriodInherit, models.EntryPeriod{}, parent, parent, models.EntryPeriod{}, AttachToApplicationRequest{}, false, true)
	require.NoError(t, err)
	require.Equal(t, models.PeriodInherit, mode)
	require.Equal(t, models.EntryPeriod{}, own)
	require.False(t, changed, "adopting the same manual parent preserves inherit")
	laterStart := manual2665Window("2031-01-01", "2031-01-20", "09:00", "18:00")
	require.False(t, manualAttachWindowWithin(parent, laterStart))
	legacy := parent
	legacy.EntryTimeFrom = nil
	legacy.EntryTimeTo = nil
	fullDay := manual2665Window("2031-01-01", "2031-01-20", "00:00", "23:59:59")
	require.True(t, manualAttachWindowWithin(legacy, fullDay))
	require.True(t, manualAttachWindowWithin(parent, models.EntryPeriod{}), "legacy open target remains unlimited")
}

func TestManualAttach2665SourceAndIndividualModesAreExplicit(t *testing.T) {
	target, other := 20, 30
	chosen := manual2665Window("2031-01-01", "2031-01-20", "08:00:00", "18:00:00")
	for _, tc := range []struct {
		name  string
		req   AttachToApplicationRequest
		adopt bool
		want  models.PeriodMode
	}{
		{"target source", AttachToApplicationRequest{TargetAttachmentID: &target, SourceAttachmentID: &target, PeriodChoice: "source"}, false, models.PeriodInherit},
		{"other source equal values", AttachToApplicationRequest{TargetAttachmentID: &target, SourceAttachmentID: &other, PeriodChoice: "source"}, false, models.PeriodIndividual},
		{"adopt source", AttachToApplicationRequest{SourceAttachmentID: &other, PeriodChoice: "source"}, true, models.PeriodInherit},
		{"explicit individual equal values", AttachToApplicationRequest{TargetAttachmentID: &target, PeriodChoice: "individual"}, false, models.PeriodIndividual},
		{"adopt individual", AttachToApplicationRequest{PeriodChoice: "individual"}, true, models.PeriodIndividual},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode, next, changed, err := manualAttachNextPeriod(models.PeriodInherit, models.EntryPeriod{}, models.EntryPeriod{}, chosen, chosen, tc.req, true, tc.adopt)
			require.NoError(t, err)
			require.True(t, changed)
			require.Equal(t, tc.want, mode)
			if tc.want == models.PeriodIndividual {
				require.Equal(t, chosen, next)
			} else {
				require.Equal(t, models.EntryPeriod{}, next)
			}
		})
	}
	_, _, _, err := manualAttachNextPeriod(models.PeriodInherit, models.EntryPeriod{}, models.EntryPeriod{}, chosen, models.EntryPeriod{}, AttachToApplicationRequest{PeriodChoice: "source", TargetAttachmentID: &target, SourceAttachmentID: &target}, true, false)
	require.Error(t, err)
}
