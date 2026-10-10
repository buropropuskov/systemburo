package services

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"net/http"
	"regexp"
	"systemburo/internal/models"
)

// EffectivePeriodSQL is a read-model adapter, not an access/territory predicate.
// Consumers must combine ValidMode with object visibility and lifecycle checks.
type EffectivePeriodSQL struct {
	DateFrom  string
	DateTo    string
	TimeFrom  string
	TimeTo    string
	Bounded   string
	Source    string
	ValidMode string
}

var periodAlias = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// EntityEffectivePeriodSQL takes developer-owned SQL aliases only. Every field
// selects the same explicit mode; individual null fields never inherit piecemeal.
func EntityEffectivePeriodSQL(entityAlias, attachmentAlias string) (EffectivePeriodSQL, error) {
	if !periodAlias.MatchString(entityAlias) || !periodAlias.MatchString(attachmentAlias) {
		return EffectivePeriodSQL{}, fmt.Errorf("invalid effective period SQL alias")
	}
	mode := "COALESCE(NULLIF(" + entityAlias + ".period_mode, ''), 'inherit')"
	field := func(name string) string {
		return "(CASE WHEN " + mode + " = 'individual' THEN " + entityAlias + "." + name + " WHEN " + mode + " = 'inherit' THEN " + attachmentAlias + "." + name + " ELSE NULL END)"
	}
	dateTo := field("entry_date_to")
	manualEmpty := attachmentAlias + ".is_manual AND " + mode + " = 'inherit'"
	for _, name := range []string{"entry_date_from", "entry_date_to", "entry_time_from", "entry_time_to"} {
		manualEmpty += " AND NULLIF(BTRIM(" + attachmentAlias + "." + name + "), '') IS NULL"
	}
	return EffectivePeriodSQL{
		DateFrom: field("entry_date_from"), DateTo: dateTo,
		TimeFrom: field("entry_time_from"), TimeTo: field("entry_time_to"),
		Bounded:   "(NULLIF(BTRIM(" + dateTo + "), '') IS NOT NULL)",
		Source:    "(CASE WHEN " + mode + " = 'individual' THEN 'individual' WHEN " + manualEmpty + " THEN 'manual_unbounded' WHEN " + mode + " = 'inherit' THEN 'attachment' ELSE 'invalid' END)",
		ValidMode: "(" + mode + " IN ('inherit', 'individual'))",
	}, nil
}

// PeriodChangeTarget must be assembled from server reads in the transaction, never
// from a client can_edit flag. A granted operation does not grant object visibility.
type PeriodChangeTarget struct {
	Visible           bool
	Manual            bool
	Active            bool
	Archived          bool
	Removed           bool
	ApplicationStatus string
}

// AuthorizeEntityPeriodChange deliberately has no receiver/role bypass. Existing
// PermissionResolver merges managed grants and personal denies before this check.
func AuthorizeEntityPeriodChange(permissions PermissionSet, target PeriodChangeTarget) error {
	return authorizePeriodChange(permissions, target, KeyDetailPeriodChange, true)
}

func AuthorizeAttachmentPeriodChange(permissions PermissionSet, target PeriodChangeTarget) error {
	return authorizePeriodChange(permissions, target, KeyApplicationPeriodChange, false)
}

func authorizePeriodChange(permissions PermissionSet, target PeriodChangeTarget, key string, individual bool) error {
	if !permissions.Has(key) || !target.Visible {
		return echo.NewHTTPError(http.StatusForbidden, "Недостаточно прав")
	}
	if target.Archived || target.Removed || (individual && !target.Active) {
		return echo.NewHTTPError(http.StatusBadRequest, "Срок недействующей записи изменять нельзя")
	}
	if individual {
		if target.Manual || target.ApplicationStatus == models.StatusInWork {
			return nil
		}
	} else if !target.Manual && (target.ApplicationStatus == models.StatusUnread || target.ApplicationStatus == models.StatusProcessing || target.ApplicationStatus == models.StatusInWork) {
		// The managed group operation also preserves approvals while in work.
		return nil
	}
	return echo.NewHTTPError(http.StatusBadRequest, "В этом состоянии срок изменять нельзя")
}
