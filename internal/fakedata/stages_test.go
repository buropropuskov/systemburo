package fakedata_test

import (
	"testing"

	"systemburo/internal/fakedata"

	"github.com/stretchr/testify/require"
)

// При числе заявок меньше числа стадий партия не должна молча схлопываться в одну
// стадию: раньше пропорциональное ужатие обнуляло сразу все меньшинства.
func TestStageBucketSizes_TinyBatchKeepsVariety(t *testing.T) {
	for total := 1; total <= 5; total++ {
		unread, approvedOnly, rejected, revoked, withdrawn := fakedata.StageBucketSizesForTest(total)
		sum := unread + approvedOnly + rejected + revoked + withdrawn
		require.LessOrEqual(t, sum, total, "заявок роздано больше, чем есть (total=%d)", total)
		require.Equal(t, min(total, 5), sum,
			"при total=%d по стадиям должно разойтись всё, что можно, а не ноль", total)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
