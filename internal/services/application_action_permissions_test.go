package services

import "testing"

func TestS5Action_PermissionMatrix(t *testing.T) {
	for _, key := range []string{KeyActionApproveApplication, KeyActionForwardApplication} {
		for _, tc := range []struct {
			name string
			set  PermissionSet
			want bool
		}{
			{"assignment_without_grant", PermissionSet{}, true},
			{"allow", PermissionSet{allows: map[string]string{key: "role"}}, true},
			{"deny", PermissionSet{denies: map[string]struct{}{key: {}}}, false},
			{"admin_deny", PermissionSet{adminAll: true, denies: map[string]struct{}{key: {}}}, false},
			{"super", PermissionSet{allowAll: true, denies: map[string]struct{}{key: {}}}, true},
			{"inactive", PermissionSet{inactive: true}, false},
			{"banned", PermissionSet{banned: true}, false},
		} {
			t.Run(key+"/"+tc.name, func(t *testing.T) {
				if got := applicationActionAllowed(tc.set, key); got != tc.want {
					t.Fatalf("allowed=%v, want %v", got, tc.want)
				}
			})
		}
	}
	if applicationActionAllowed(PermissionSet{allowAll: true}, KeyPageAdmin) {
		t.Fatal("application entitlement must not grant unrelated actions")
	}
}
