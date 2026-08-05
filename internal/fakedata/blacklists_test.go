package fakedata_test

import (
	"testing"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

// Plan чёрных списков масштабируется профилем -- обе записи (машины и люди) берут
// ровно profile.Blacklists, как реестры сотрудников/машин берут profile.Employees/
// profile.Cars (см. TestPlan_RegistriesScaleWithProfile).
func TestPlan_BlacklistsScaleWithProfile(t *testing.T) {
	for _, name := range fakedata.ProfileNames() {
		t.Run(name, func(t *testing.T) {
			profile, err := fakedata.ProfileByName(name)
			require.NoError(t, err)

			counts := planCounts(fakedata.Plan(profile))

			require.Equal(t, profile.Blacklists, counts[models.AuditEntityVehicleBlacklist])
			require.Equal(t, profile.Blacklists, counts[models.AuditEntityPersonBlacklist])
		})
	}
}
