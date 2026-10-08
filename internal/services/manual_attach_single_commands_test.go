package services

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"systemburo/internal/models"
)

func TestSingleManualAttachTargetCompatibility(t *testing.T) {
	template, other := 7, 8
	orphan := models.Attachment{RoofAccess: true, UniqueAttachmentID: &template}
	require.NoError(t, singleManualAttachTargetCompatibility(ElementCar, orphan, nil))
	target := orphan
	require.NoError(t, singleManualAttachTargetCompatibility(ElementCar, orphan, &target))
	require.NoError(t, singleManualAttachTargetCompatibility(ElementEmployee, orphan, &target))
	for _, scenario := range []string{"roof", "parking", "template"} {
		t.Run(scenario, func(t *testing.T) {
			changed := target
			switch scenario {
			case "roof":
				changed.RoofAccess = false
			case "parking":
				changed.FreeParking = true
			case "template":
				changed.UniqueAttachmentID = &other
			}
			if scenario == "template" {
				require.Error(t, singleManualAttachTargetCompatibility(ElementCar, orphan, &changed))
			} else {
				require.NoError(t, singleManualAttachTargetCompatibility(ElementCar, orphan, &changed))
			}
			require.Error(t, singleManualAttachTargetCompatibility(ElementEmployee, orphan, &changed), "employee flags must not acquire car-only inheritance semantics")
		})
	}
}

func TestSingleManualAttachCarFlagsIndependentOR(t *testing.T) {
	for bits := 0; bits < 16; bits++ {
		own := SingleManualAttachCarFlags{IndividualRoofAccess: bits&1 != 0, IndividualFreeParking: bits&2 != 0}
		parent := models.Attachment{RoofAccess: bits&4 != 0, FreeParking: bits&8 != 0}
		got := singleAttachCarFlags(own, parent)
		require.Equal(t, own.IndividualRoofAccess || parent.RoofAccess, got.RoofAccess)
		require.Equal(t, own.IndividualFreeParking || parent.FreeParking, got.FreeParking)
		require.Equal(t, own.IndividualRoofAccess, got.IndividualRoofAccess)
		require.Equal(t, own.IndividualFreeParking, got.IndividualFreeParking)
	}
}

func TestSingleManualAttachRevisionCanonicalAndScoped(t *testing.T) {
	stamp := time.Date(2030, 10, 8, 10, 0, 0, 123456789, time.UTC)
	active := 1
	orphan := models.Attachment{ID: 1, IsManual: true, Status: &active, UpdatedAt: stamp}
	row := entityPeriodSnapshot{ID: 17, AttachmentID: 1, Manual: true, UpdatedAt: stamp, AttachmentUpdatedAt: stamp, PeriodMode: models.PeriodInherit}
	appID := 7
	req := SingleManualAttachRequest{ApplicationID: &appID, Reason: " Привязка "}
	revision, err := singleAttachRevision(2, ElementEmployee, row, orphan, nil, nil, 7, models.StatusInWork, models.ConfirmationApproved, &stamp, req, nil, nil, SingleManualAttachCarFlags{})
	require.NoError(t, err)
	require.Len(t, revision, 64)
	row.UpdatedAt = canonicalEntityPeriodTime(row.UpdatedAt)
	orphan.UpdatedAt = canonicalEntityPeriodTime(orphan.UpdatedAt)
	req.ExpectedRevision = strings.Repeat("f", 64)
	req.Reason = "Привязка"
	again, err := singleAttachRevision(2, ElementEmployee, row, orphan, nil, nil, 7, models.StatusInWork, models.ConfirmationApproved, &stamp, req, nil, nil, SingleManualAttachCarFlags{})
	require.NoError(t, err)
	require.Equal(t, revision, again)
	row.PeriodMode = models.PeriodIndividual
	changed, err := singleAttachRevision(2, ElementEmployee, row, orphan, nil, nil, 7, models.StatusInWork, models.ConfirmationApproved, &stamp, req, nil, nil, SingleManualAttachCarFlags{})
	require.NoError(t, err)
	require.NotEqual(t, revision, changed)
}
