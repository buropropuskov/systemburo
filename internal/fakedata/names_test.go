package fakedata_test

import (
	"testing"

	"systemburo/internal/fakedata"

	"github.com/stretchr/testify/require"
)

// Согласование пола - главное требование к словарю: женское ФИО обязано нести
// женскую фамилию и женское отчество, иначе получится "Иванова Мария Петрович".
func TestFullNameForSex_FemaleFormsAgree(t *testing.T) {
	s := fakedata.NewStream(1, "names")
	for i := 0; i < 300; i++ {
		name := fakedata.FullNameForSex(s, fakedata.Female)
		require.Equal(t, fakedata.Female, name.Sex)
		require.True(t, hasSuffix(name.MiddleName, "на"), "женское отчество должно оканчиваться на -на: %q", name.MiddleName)
		require.NotEmpty(t, name.FirstName)
		require.NotEmpty(t, name.LastName)
	}
}

func TestFullNameForSex_MaleFormsAgree(t *testing.T) {
	s := fakedata.NewStream(1, "names")
	for i := 0; i < 300; i++ {
		name := fakedata.FullNameForSex(s, fakedata.Male)
		require.Equal(t, fakedata.Male, name.Sex)
		require.False(t, hasSuffix(name.MiddleName, "на"), "мужское отчество не должно оканчиваться на -на: %q", name.MiddleName)
		require.NotEmpty(t, name.FirstName)
		require.NotEmpty(t, name.LastName)
	}
}

// Фамилия и отчество берутся из согласованных пар/форм, а не собираются из двух
// независимых списков: женская фамилия должна встречаться только с женским полом.
func TestFullNameForSex_LastNameMatchesSex(t *testing.T) {
	s := fakedata.NewStream(2, "names")
	maleLastNames := map[string]bool{}
	femaleLastNames := map[string]bool{}
	for i := 0; i < 400; i++ {
		male := fakedata.FullNameForSex(s, fakedata.Male)
		female := fakedata.FullNameForSex(s, fakedata.Female)
		maleLastNames[male.LastName] = true
		femaleLastNames[female.LastName] = true
	}
	for name := range maleLastNames {
		require.False(t, hasSuffix(name, "а"), "мужская фамилия не должна оканчиваться на -а: %q", name)
	}
	// Проверка симметрична намеренно: односторонняя пропустила бы фамилию-исключение,
	// у которой женская форма совпала с мужской, а словарь ещё будет расти.
	for name := range femaleLastNames {
		require.True(t, hasSuffix(name, "а"), "женская фамилия должна оканчиваться на -а: %q", name)
	}
}

func TestRandomFullName_BothSexesAppear(t *testing.T) {
	s := fakedata.NewStream(3, "names")
	seenMale, seenFemale := false, false
	for i := 0; i < 200; i++ {
		name := fakedata.RandomFullName(s)
		if name.Sex == fakedata.Male {
			seenMale = true
		} else {
			seenFemale = true
		}
	}
	require.True(t, seenMale, "за 200 попыток должен встретиться хотя бы один мужчина")
	require.True(t, seenFemale, "за 200 попыток должна встретиться хотя бы одна женщина")
}

// Объём словаря: на профиле "large" (3000 сотрудников) сочетания фамилия+имя+
// отчество не должны быть сплошными повторами.
func TestFullNameForSex_EnoughCombinationsForLargeProfile(t *testing.T) {
	s := fakedata.NewStream(4, "names")
	seen := map[string]bool{}
	for i := 0; i < 3000; i++ {
		name := fakedata.FullNameForSex(s, fakedata.Male)
		seen[name.LastName+" "+name.FirstName+" "+name.MiddleName] = true
	}
	require.Greater(t, len(seen), 2500, "словарь должен давать разнообразные сочетания, а не горстку повторяющихся ФИО")
}

func TestRandomPosition_ReturnsFromDictionary(t *testing.T) {
	s := fakedata.NewStream(1, "positions")
	for i := 0; i < 30; i++ {
		require.Contains(t, fakedata.Positions, fakedata.RandomPosition(s))
	}
}

func hasSuffix(s, suffix string) bool {
	r := []rune(s)
	sr := []rune(suffix)
	if len(r) < len(sr) {
		return false
	}
	return string(r[len(r)-len(sr):]) == suffix
}
