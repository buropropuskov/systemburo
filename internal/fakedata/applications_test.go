package fakedata_test

import (
	"testing"

	"systemburo/internal/fakedata"
	"systemburo/internal/models"

	"github.com/stretchr/testify/require"
)

// Plan заявок масштабируется профилем -- в отличие от шаблонов вложений (фиксированный
// кандидатский список, см. TestPlan_DictionariesAndPostsAreFixedRegardlessOfProfile),
// заявок ровно столько, сколько просит Profile.Applications.
func TestPlan_ApplicationsScaleWithProfile(t *testing.T) {
	for _, name := range fakedata.ProfileNames() {
		t.Run(name, func(t *testing.T) {
			profile, err := fakedata.ProfileByName(name)
			require.NoError(t, err)

			counts := planCounts(fakedata.Plan(profile))

			require.Equal(t, profile.Applications, counts[models.AuditEntityApplication])
		})
	}
}
