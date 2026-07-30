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
