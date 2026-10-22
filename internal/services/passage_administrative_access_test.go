package services

import "testing"

func TestCanCorrectPassageRequiresAdministrativeMode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		set  PermissionSet
		want bool
	}{
		{name: "ordinary"},
		{name: "ordinary with explicit grant", set: PermissionSet{allows: map[string]string{passageCorrectionPermission: SourceOverride}}},
		{name: "admin", set: PermissionSet{adminAll: true}, want: true},
		{name: "admin denied correction", set: PermissionSet{adminAll: true, denies: map[string]struct{}{passageCorrectionPermission: {}}}},
		{name: "banned admin", set: PermissionSet{adminAll: true, banned: true}},
		{name: "inactive admin", set: PermissionSet{adminAll: true, inactive: true}},
		{name: "super", set: PermissionSet{allowAll: true}, want: true},
		{name: "super ignores personal denial", set: PermissionSet{allowAll: true, denies: map[string]struct{}{passageCorrectionPermission: {}}}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := canCorrectPassage(tc.set); got != tc.want {
				t.Fatalf("canCorrectPassage = %v, want %v", got, tc.want)
			}
		})
	}
}
