package services

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRegistryReadScope_IndependentSections(t *testing.T) {
	uid, foreign, org, company := 1, 2, 3, 4
	for mask := 0; mask < 8; mask++ {
		set := PermissionSet{allows: map[string]string{}}
		keys := []string{KeySectionRegistryOrganization, KeySectionRegistryCompany, KeySectionRegistryAllSystem}
		for i, key := range keys {
			if mask&(1<<i) != 0 {
				set.allows[key] = SourceOverride
			}
		}
		scope := newRegistryReadScope(set, uid, &org, &company)
		require.True(t, scope.contains(&uid, nil, nil), "own, mask %d", mask)
		require.Equal(t, mask&5 != 0, scope.contains(&foreign, &org, nil), "organization, mask %d", mask)
		require.Equal(t, mask&6 != 0, scope.contains(&foreign, nil, &company), "company, mask %d", mask)
		require.Equal(t, mask&4 != 0, scope.contains(&foreign, nil, nil), "foreign, mask %d", mask)
		for i, filter := range []string{"organization", "company", "all_system"} {
			require.Equal(t, mask&(1<<i) == 0, scope.authorizeFilter(filter) != nil, filter)
		}
		require.NoError(t, scope.authorizeFilter("user"))
		require.NoError(t, scope.authorizeFilter("all"))
	}
}

func TestRegistryReadScope_AdminDenyAndSuper(t *testing.T) {
	uid, other, org := 1, 2, 3
	denies := map[string]struct{}{KeySectionRegistryOrganization: {}, KeySectionRegistryCompany: {}, KeySectionRegistryAllSystem: {}}
	admin := newRegistryReadScope(PermissionSet{adminAll: true, denies: denies}, uid, &org, nil)
	require.True(t, admin.contains(&uid, nil, nil))
	require.False(t, admin.contains(&other, &org, nil))
	super := newRegistryReadScope(PermissionSet{allowAll: true, denies: denies}, uid, &org, nil)
	require.True(t, super.contains(&other, nil, nil))
	require.NoError(t, super.authorizeFilter("all_system"))
	withoutMembership := newRegistryReadScope(PermissionSet{adminAll: true}, uid, nil, nil)
	withoutMembership.allSystem = false
	require.False(t, withoutMembership.contains(&other, &org, nil))
}
