package fakedata_test

import (
	"regexp"
	"testing"

	"systemburo/internal/fakedata"

	"github.com/stretchr/testify/require"
)

// Формат: буква, три цифры, две буквы, код региона (2 или 3 цифры), буквы - только
// из PlateLetters.
var plateFormatRe = regexp.MustCompile(`^[АВЕКМНОРСТУХ]\d{3}[АВЕКМНОРСТУХ]{2}\d{2,3}$`)

func TestPlateGenerator_MatchesFormat(t *testing.T) {
	g := fakedata.NewPlateGenerator(1)
	for i := 0; i < 300; i++ {
		plate, err := g.Next()
		require.NoError(t, err)
		require.Regexp(t, plateFormatRe, plate)
	}
}

// Серийный номер "000" на настоящих знаках не выдаётся - вымышленный номер не
// должен его использовать тоже, иначе поле будет отличать сгенерированные записи
// от настоящих не только по факту, но и по этому конкретному признаку.
func TestPlateGenerator_NeverZeroSerial(t *testing.T) {
	g := fakedata.NewPlateGenerator(2)
	zeroSerial := regexp.MustCompile(`^[АВЕКМНОРСТУХ]000`)
	for i := 0; i < 500; i++ {
		plate, err := g.Next()
		require.NoError(t, err)
		require.NotRegexp(t, zeroSerial, plate)
	}
}

func TestPlateGenerator_RegionAlwaysFromRealList(t *testing.T) {
	known := map[string]bool{}
	for _, code := range fakedata.PlateRegionCodes {
		known[code] = true
	}

	g := fakedata.NewPlateGenerator(3)
	re := regexp.MustCompile(`^[АВЕКМНОРСТУХ]\d{3}[АВЕКМНОРСТУХ]{2}(\d{2,3})$`)
	for i := 0; i < 300; i++ {
		plate, err := g.Next()
		require.NoError(t, err)
		m := re.FindStringSubmatch(plate)
		require.Len(t, m, 2)
		require.True(t, known[m[1]], "код региона %q должен быть из PlateRegionCodes", m[1])
	}
}

func TestPlateGenerator_NoDuplicatesWithinBatch(t *testing.T) {
	g := fakedata.NewPlateGenerator(4)
	seen := map[string]bool{}
	for i := 0; i < 2000; i++ {
		plate, err := g.Next()
		require.NoError(t, err)
		require.False(t, seen[plate], "номер %q выдан дважды в пределах одной партии", plate)
		seen[plate] = true
	}
}

// Повтор с тем же seed обязан дать ту же последовательность номеров - в том числе
// в отдельном генераторе, а не только в исходном Stream.
func TestPlateGenerator_RepeatableBySeed(t *testing.T) {
	a := fakedata.NewPlateGenerator(99)
	b := fakedata.NewPlateGenerator(99)
	for i := 0; i < 50; i++ {
		pa, err := a.Next()
		require.NoError(t, err)
		pb, err := b.Next()
		require.NoError(t, err)
		require.Equal(t, pa, pb)
	}
}
