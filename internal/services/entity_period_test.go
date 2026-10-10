package services

import (
	"github.com/labstack/echo/v4"
	"net/http"
	"strings"
	"systemburo/internal/models"
	"testing"
)

func TestEntityEffectivePeriodSQL(t *testing.T) {
	got, err := EntityEffectivePeriodSQL("e", "a")
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{got.DateFrom, got.DateTo, got.TimeFrom, got.TimeTo} {
		if !strings.Contains(field, "e.period_mode") || !strings.Contains(field, "'individual'") || !strings.Contains(field, "'inherit'") {
			t.Fatalf("window did not select explicit mode: %s", field)
		}
	}
	if !strings.Contains(got.Source, "manual_unbounded") || !strings.Contains(got.ValidMode, "IN ('inherit', 'individual')") {
		t.Fatal("missing source/mode guard")
	}
	if _, err := EntityEffectivePeriodSQL("e; SELECT 1", "a"); err == nil {
		t.Fatal("unsafe alias accepted")
	}
}

func TestPeriodChangeAuthorization(t *testing.T) {
	allowed := PermissionSet{allows: map[string]string{KeyDetailPeriodChange: SourceOverride, KeyApplicationPeriodChange: SourceOverride}}
	active := PeriodChangeTarget{Visible: true, Active: true, ApplicationStatus: models.StatusInWork}
	for _, tc := range []struct {
		name       string
		set        PermissionSet
		target     PeriodChangeTarget
		attachment bool
		status     int
	}{
		{"granted entity in work", allowed, active, false, 0},
		{"group in work allowed", allowed, active, true, 0},
		{"group missing permission", PermissionSet{}, active, true, http.StatusForbidden},
		{"group personal deny", PermissionSet{adminAll: true, denies: map[string]struct{}{KeyApplicationPeriodChange: {}}}, active, true, http.StatusForbidden},
		{"group hidden", allowed, PeriodChangeTarget{ApplicationStatus: models.StatusInWork}, true, http.StatusForbidden},
		{"group archived", allowed, PeriodChangeTarget{Visible: true, Archived: true, ApplicationStatus: models.StatusInWork}, true, http.StatusBadRequest},
		{"group removed", allowed, PeriodChangeTarget{Visible: true, Removed: true, ApplicationStatus: models.StatusInWork}, true, http.StatusBadRequest},
		{"group completed", allowed, PeriodChangeTarget{Visible: true, ApplicationStatus: models.StatusCompleted}, true, http.StatusBadRequest},
		{"individual unread not approved", allowed, PeriodChangeTarget{Visible: true, Active: true, ApplicationStatus: models.StatusUnread}, false, http.StatusBadRequest},
		{"individual processing not approved", allowed, PeriodChangeTarget{Visible: true, Active: true, ApplicationStatus: models.StatusProcessing}, false, http.StatusBadRequest},
		{"group unread preactivation", allowed, PeriodChangeTarget{Visible: true, ApplicationStatus: models.StatusUnread}, true, 0},
		{"group processing preactivation", allowed, PeriodChangeTarget{Visible: true, ApplicationStatus: models.StatusProcessing}, true, 0},
		{"manual group prohibited", allowed, PeriodChangeTarget{Visible: true, Active: true, Manual: true}, true, http.StatusBadRequest},
		{"view only", PermissionSet{}, active, false, http.StatusForbidden},
		{"admin personal deny", PermissionSet{adminAll: true, denies: map[string]struct{}{KeyDetailPeriodChange: {}}}, active, false, http.StatusForbidden},
		{"blocked super", PermissionSet{allowAll: true, banned: true}, active, false, http.StatusForbidden},
		{"invisible before state", allowed, PeriodChangeTarget{Visible: false, ApplicationStatus: models.StatusCompleted}, false, http.StatusForbidden},
		{"completed", allowed, PeriodChangeTarget{Visible: true, Active: true, ApplicationStatus: models.StatusCompleted}, false, http.StatusBadRequest},
		{"withdrawn", allowed, PeriodChangeTarget{Visible: true, Active: true, ApplicationStatus: models.StatusWithdrawn}, false, http.StatusBadRequest},
		{"manual active", allowed, PeriodChangeTarget{Visible: true, Active: true, Manual: true}, false, 0},
		{"manual inactive", allowed, PeriodChangeTarget{Visible: true, Manual: true}, false, http.StatusBadRequest},
		{"removed", allowed, PeriodChangeTarget{Visible: true, Active: true, Manual: true, Removed: true}, false, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.attachment {
				err = AuthorizeAttachmentPeriodChange(tc.set, tc.target)
			} else {
				err = AuthorizeEntityPeriodChange(tc.set, tc.target)
			}
			if tc.status == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			httpErr, ok := err.(*echo.HTTPError)
			if !ok || httpErr.Code != tc.status {
				t.Fatalf("got %v, want HTTP%d", err, tc.status)
			}
		})
	}
}

func TestPeriodKeysAreManagedCatalogPermissions(t *testing.T) {
	for _, key := range []string{KeyApplicationPeriodChange, KeyDetailPeriodChange} {
		if !IsCatalogKey(key) || IsSuperOnly(key) {
			t.Fatalf("key %s not grantable", key)
		}
		found := false
		for _, known := range AllStaticKeys() {
			if known == key {
				found = true
			}
		}
		if !found {
			t.Fatalf("key %s absent from static keys", key)
		}
		admin := PermissionSet{adminAll: true}
		if !admin.Has(key) {
			t.Fatalf("admin missing %s", key)
		}
	}
}
