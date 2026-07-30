package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLockoutDuration_Ladder(t *testing.T) {
	cases := []struct {
		level int
		want  time.Duration
	}{
		{0, time.Minute},
		{1, 5 * time.Minute},
		{2, 15 * time.Minute},
		{3, 30 * time.Minute},
		{4, 60 * time.Minute},
		{5, 60 * time.Minute},
		{99, 60 * time.Minute},
		{-1, time.Minute},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, lockoutDuration(c.level), "ступень %d", c.level)
	}
}

// TestLockoutLadder_SharedByAccountAndGuard - учётная запись и счётчик пары
// «адрес + логин» обязаны расти по ОДНОЙ лестнице. На этом держится то, что
// выдуманный логин запирается на те же сроки, что настоящий: разойдись формы -
// и по длительности блокировки снова можно отличать существующие учётки.
func TestLockoutLadder_SharedByAccountAndGuard(t *testing.T) {
	base := 20 * time.Millisecond
	for level := 0; level <= len(lockoutSteps)+2; level++ {
		account := lockoutDuration(level)
		guard := stepDuration(base, level)
		assert.Equal(t, account/accountLockDuration, guard/base,
			"ступень %d: множитель учётки и пары разошёлся", level)
	}
}
