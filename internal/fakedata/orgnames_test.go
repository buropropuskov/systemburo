package fakedata_test

import (
	"regexp"
	"testing"

	"systemburo/internal/fakedata"

	"github.com/stretchr/testify/require"
)

func TestOrgNameGenerator_MatchesFormat(t *testing.T) {
	legalForms := map[string]bool{}
	for _, f := range fakedata.OrgLegalForms {
		legalForms[f] = true
	}
	re := regexp.MustCompile(`^(\S+) (.+)$`)

	g := fakedata.NewOrgNameGenerator(1, "orgnames")
	for i := 0; i < 50; i++ {
		name, err := g.Next()
		require.NoError(t, err)
		m := re.FindStringSubmatch(name)
		require.Len(t, m, 3, "название %q должно быть в виде 'форма название'", name)
		require.True(t, legalForms[m[1]], "форма %q должна быть из OrgLegalForms", m[1])
		require.NotEmpty(t, m[2])
	}
}

func TestOrgNameGenerator_NoDuplicatesWithinBatch(t *testing.T) {
	g := fakedata.NewOrgNameGenerator(2, "orgnames")
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		name, err := g.Next()
		require.NoError(t, err)
		require.False(t, seen[name], "название %q выдано дважды в пределах одной партии", name)
		seen[name] = true
	}
}

// Организации и компании - разные сущности на разных потоках (domain "orgnames" /
// "companynames"): последовательность одного генератора не должна зависеть от
// того, сколько имён к этому моменту выдал другой.
func TestOrgNameGenerator_DomainsAreIndependent(t *testing.T) {
	seed := int64(3)

	orgs1 := fakedata.NewOrgNameGenerator(seed, "orgnames")
	firstOrgName, err := orgs1.Next()
	require.NoError(t, err)

	// Между открытием генератора организаций и повторным чтением его первого
	// имени расходуем генератор компаний - на организации это влиять не должно.
	companies := fakedata.NewOrgNameGenerator(seed, "companynames")
	_, err = companies.Next()
	require.NoError(t, err)
	_, err = companies.Next()
	require.NoError(t, err)

	orgs2 := fakedata.NewOrgNameGenerator(seed, "orgnames")
	secondOrgName, err := orgs2.Next()
	require.NoError(t, err)

	require.Equal(t, firstOrgName, secondOrgName, "домен orgnames не должен зависеть от использования companynames")
}

func TestOrgNameGenerator_RepeatableBySeed(t *testing.T) {
	a := fakedata.NewOrgNameGenerator(42, "orgnames")
	b := fakedata.NewOrgNameGenerator(42, "orgnames")
	for i := 0; i < 30; i++ {
		na, err := a.Next()
		require.NoError(t, err)
		nb, err := b.Next()
		require.NoError(t, err)
		require.Equal(t, na, nb)
	}
}
