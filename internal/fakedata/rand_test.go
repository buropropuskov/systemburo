package fakedata_test

import (
	"testing"

	"systemburo/internal/fakedata"

	"github.com/stretchr/testify/require"
)

// Повторяемость по seed - обещание, которое команда "server fake" даёт в справке
// ("Тот же seed даёт ту же партию"): повтор с тем же (seed, domain) обязан выдать
// ту же последовательность значений.
func TestNewStream_SameSeedSameSequence(t *testing.T) {
	a := fakedata.NewStream(12345, "names")
	b := fakedata.NewStream(12345, "names")

	for i := 0; i < 20; i++ {
		require.Equal(t, fakedata.IntRange(a, 0, 1_000_000), fakedata.IntRange(b, 0, 1_000_000))
	}
}

func TestNewStream_DifferentSeedDifferentSequence(t *testing.T) {
	a := fakedata.NewStream(1, "names")
	b := fakedata.NewStream(2, "names")

	same := true
	for i := 0; i < 20; i++ {
		if fakedata.IntRange(a, 0, 1_000_000) != fakedata.IntRange(b, 0, 1_000_000) {
			same = false
			break
		}
	}
	require.False(t, same, "разные seed не должны давать одну и ту же последовательность")
}

// Ключевое свойство потоков доменов: домен не зависит от того, сколько вызовов
// сделано в ДРУГОМ домене между открытием потока и чтением из него. Без этого
// добавление одного Pick в середину шага "plates" сдвинуло бы последующие
// значения "names", и повтор с тем же -seed перестал бы давать ту же партию при
// следующей правке кода где угодно раньше по потоку.
func TestNewStream_DomainsAreIndependent(t *testing.T) {
	seed := int64(777)

	names1 := fakedata.NewStream(seed, "names")
	firstPick := fakedata.IntRange(names1, 0, 1_000_000)

	// Между открытием потока "names" и повторным чтением его же значения
	// открываем и расходуем поток "plates" - на "names" это повлиять не должно.
	plates := fakedata.NewStream(seed, "plates")
	_ = fakedata.IntRange(plates, 0, 1_000_000)
	_ = fakedata.IntRange(plates, 0, 1_000_000)
	_ = fakedata.IntRange(plates, 0, 1_000_000)

	names2 := fakedata.NewStream(seed, "names")
	secondPick := fakedata.IntRange(names2, 0, 1_000_000)

	require.Equal(t, firstPick, secondPick,
		"поток домена names не должен зависеть от того, сколько значений взято из потока plates")
}

func TestNewStream_DifferentDomainsDifferentSequence(t *testing.T) {
	seed := int64(42)
	a := fakedata.NewStream(seed, "names")
	b := fakedata.NewStream(seed, "plates")

	same := true
	for i := 0; i < 20; i++ {
		if fakedata.IntRange(a, 0, 1_000_000) != fakedata.IntRange(b, 0, 1_000_000) {
			same = false
			break
		}
	}
	require.False(t, same, "разные домены одного seed не должны давать одну и ту же последовательность")
}

func TestPick_ReturnsElementOfSlice(t *testing.T) {
	s := fakedata.NewStream(1, "test")
	items := []string{"a", "b", "c"}
	for i := 0; i < 30; i++ {
		require.Contains(t, items, fakedata.Pick(s, items))
	}
}

func TestPick_PanicsOnEmptySlice(t *testing.T) {
	s := fakedata.NewStream(1, "test")
	require.Panics(t, func() { fakedata.Pick(s, []string{}) })
}

func TestIntRange_StaysWithinBounds(t *testing.T) {
	s := fakedata.NewStream(1, "test")
	for i := 0; i < 200; i++ {
		v := fakedata.IntRange(s, 5, 9)
		require.GreaterOrEqual(t, v, 5)
		require.LessOrEqual(t, v, 9)
	}
}

func TestIntRange_SingleValueRange(t *testing.T) {
	s := fakedata.NewStream(1, "test")
	require.Equal(t, 7, fakedata.IntRange(s, 7, 7))
}

func TestChance_ZeroAndOneAreExtremes(t *testing.T) {
	s := fakedata.NewStream(1, "test")
	for i := 0; i < 10; i++ {
		require.False(t, fakedata.Chance(s, 0))
	}
	for i := 0; i < 10; i++ {
		require.True(t, fakedata.Chance(s, 1))
	}
}
