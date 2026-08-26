package fakedata_test

import (
	"testing"

	"systemburo/internal/fakedata"

	"github.com/stretchr/testify/require"
)

func TestProfileByName_UnknownReports(t *testing.T) {
	_, err := fakedata.ProfileByName("gigantic")
	require.Error(t, err)
	require.Contains(t, err.Error(), "medium", "в отказе должен быть перечень доступных профилей")
}

func TestProfileByName_CaseInsensitive(t *testing.T) {
	p, err := fakedata.ProfileByName("  Medium ")
	require.NoError(t, err)
	require.Equal(t, "medium", p.Name)
}

func TestProfileNames_OrderedByVolume(t *testing.T) {
	require.Equal(t, []string{"small", "medium", "large"}, fakedata.ProfileNames())
}

// Ноль в переопределении означает «флаг не задан»: иначе отличить неуказанный флаг от
// осознанного нуля нечем, а наливка нуля организаций смысла не имеет.
func TestProfileApply_ZeroKeepsProfileValue(t *testing.T) {
	base, err := fakedata.ProfileByName("small")
	require.NoError(t, err)

	got := base.Apply(fakedata.Overrides{Applications: 77})

	require.Equal(t, 77, got.Applications, "заданное переопределение применяется")
	require.Equal(t, base.Organizations, got.Organizations, "незаданное остаётся из профиля")
	require.Equal(t, base.DaysBack, got.DaysBack)
}

func TestPlanTotal_SumsItems(t *testing.T) {
	total := fakedata.PlanTotal([]fakedata.PlanItem{
		{Entity: "organization", Title: "Организации", Count: 3},
		{Entity: "user", Title: "Пользователи", Count: 10},
	})
	require.Equal(t, 13, total)
}
