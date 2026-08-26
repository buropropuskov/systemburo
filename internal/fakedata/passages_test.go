package fakedata_test

import (
	"testing"
	"time"

	"systemburo/internal/fakedata"

	"github.com/stretchr/testify/require"
)

// При числе кандидатов меньше двух гарантировать оба состояния (остался/выехал)
// физически нельзя, но начиная с двух распределение обязано давать хотя бы одного
// в каждом состоянии -- иначе на маленьком профиле проверочный тест плавает (см.
// TestStageBucketSizes_TinyBatchKeepsVariety).
func TestPassageStayExitCounts_TinyBatchKeepsBothStates(t *testing.T) {
	for total := 0; total <= 20; total++ {
		stay, exit := fakedata.PassageStayExitCountsForTest(total)
		require.Equal(t, total, stay+exit, "total=%d: распределено не всё", total)
		require.GreaterOrEqual(t, stay, 0, "total=%d: отрицательный остаток", total)
		require.GreaterOrEqual(t, exit, 0, "total=%d: отрицательный выезд", total)
		if total >= 2 {
			require.Positive(t, stay, "total=%d: должен остаться хотя бы один", total)
			require.Positive(t, exit, "total=%d: должен выехать хотя бы один", total)
		}
	}
}

// passageMoment должен отдавать момент СТРОГО после prev всякий раз, когда окно [prev,
// hi) на это способно -- ровно та гарантия, которую предыдущий срез (стадии обработки)
// не проверил и один раз этим ошибся: слабый "не раньше" остаётся зелёным и при
// совпавших датах. Проверяем сериями окон от заведомо широкого до на грани миллисекунды
// -- узкое окно (заявка принята в работу мгновения назад) не какой-то надуманный кейс, а
// именно то, что провоцирует проблему у свежепринятых заявок.
func TestPassageMoment_StrictlyAfterPrevWhenWindowAllows(t *testing.T) {
	s := fakedata.NewStream(1, "passage-moment-test")
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	windows := []time.Duration{
		30 * 24 * time.Hour, // широкое окно, обычный случай
		time.Hour,
		time.Minute,
		time.Second,
		50 * time.Millisecond,
		2 * time.Millisecond, // на грани -- минимум, при котором ещё есть выбор
	}
	for _, w := range windows {
		hi := base.Add(w)
		for i := 0; i < 50; i++ {
			next := fakedata.PassageMomentForTest(s, base, hi)
			require.True(t, next.After(base), "окно %v: момент не строго после prev", w)
			require.False(t, next.After(hi), "окно %v: момент позже верхней границы", w)
		}
	}
}

// Нулевое или отрицательное окно -- вырожденный случай (prev>=hi): передавать в
// историю нечего различать, функция обязана вернуть hi без паники, а не считать это
// ошибкой -- сам факт вызова с prev>=hi значит, что окна для различения уже нет.
func TestPassageMoment_DegenerateWindowReturnsHi(t *testing.T) {
	s := fakedata.NewStream(2, "passage-moment-degenerate")
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	require.Equal(t, base, fakedata.PassageMomentForTest(s, base, base))
	require.Equal(t, base, fakedata.PassageMomentForTest(s, base.Add(time.Second), base))
}
