package fakedata_test

import (
	"testing"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

// Plan реестров сотрудников и машин масштабируется профилем -- в отличие от
// lookupsStep/postsStep, эти сущности не фиксированный кандидатский список, а
// ровно столько записей, сколько просит Profile.Employees/Profile.Cars.
func TestPlan_RegistriesScaleWithProfile(t *testing.T) {
	for _, name := range fakedata.ProfileNames() {
		t.Run(name, func(t *testing.T) {
			profile, err := fakedata.ProfileByName(name)
			require.NoError(t, err)

			counts := planCounts(fakedata.Plan(profile))

			require.Equal(t, profile.Employees, counts[models.AuditEntityUniqueEmployee])
			require.Equal(t, profile.Cars, counts[models.AuditEntityUniqueCar])
		})
	}
}
