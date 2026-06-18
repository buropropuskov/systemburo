package services

import (
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestPrevPeriod(t *testing.T) {
	// 7-дневный период -> предыдущие 7 дней, заканчивающиеся за день до начала.
	from, to := prevPeriod("2026-06-08", "2026-06-14")
	assert.Equal(t, "2026-06-01", from)
	assert.Equal(t, "2026-06-07", to)

	// Один день -> предыдущий день.
	from, to = prevPeriod("2026-06-10", "2026-06-10")
	assert.Equal(t, "2026-06-09", from)
	assert.Equal(t, "2026-06-09", to)

	// Невалидные даты возвращаются как есть.
	from, to = prevPeriod("xx", "yy")
	assert.Equal(t, "xx", from)
	assert.Equal(t, "yy", to)
}

func TestRoundOne(t *testing.T) {
	assert.Equal(t, 18.0, roundOne(18.0))
	assert.Equal(t, 0.0, roundOne(0.0))
	// дробь без .5-границы (float-устойчиво): 100/3 = 33.333..., -200/3 = -66.666...
	assert.Equal(t, 33.3, roundOne(100.0/3.0))
	assert.Equal(t, -66.7, roundOne(-200.0/3.0))
}

func TestClassifyDelta(t *testing.T) {
	assert.Equal(t, "up", classifyDelta(18))
	assert.Equal(t, "down", classifyDelta(-12))
	assert.Equal(t, "flat", classifyDelta(3))
	assert.Equal(t, "flat", classifyDelta(-4))
}

func TestTrendDirection(t *testing.T) {
	assert.Equal(t, "up", trendDirection([]int64{1, 1, 5, 5}))
	assert.Equal(t, "down", trendDirection([]int64{5, 5, 1, 1}))
	assert.Equal(t, "flat", trendDirection([]int64{3, 3, 3, 3}))
	assert.Equal(t, "flat", trendDirection([]int64{7}))
	assert.Equal(t, "up", trendDirection([]int64{0, 0, 4, 4}))
}

func TestBuildComparison(t *testing.T) {
	c := buildComparison("applications_count", 118, 100)
	assert.Equal(t, int64(118), c.Current)
	assert.Equal(t, int64(100), c.Previous)
	assert.Equal(t, 18.0, c.DeltaPct)
	assert.Equal(t, "up", c.Direction)
	assert.NotEmpty(t, c.Label)

	// previous = 0, current > 0 -> 100% рост.
	z := buildComparison("car_entries_count", 5, 0)
	assert.Equal(t, 100.0, z.DeltaPct)
	assert.Equal(t, "up", z.Direction)

	// оба ноля -> flat.
	zz := buildComparison("car_entries_count", 0, 0)
	assert.Equal(t, 0.0, zz.DeltaPct)
	assert.Equal(t, "flat", zz.Direction)
}

func TestBuildPeakHours(t *testing.T) {
	r := &models.ReportResponse{
		Total: 10,
		Rows: []models.ReportAggregateRow{
			{Label: "9", Value: 2},
			{Label: "10", Value: 5},
			{Label: "11", Value: 3},
		},
	}
	ph := buildPeakHours("car_entries_count", r)
	if assert.NotNil(t, ph) {
		assert.Equal(t, 10, ph.PeakHour)
		assert.Equal(t, int64(5), ph.PeakValue)
		assert.Len(t, ph.Hourly, 3)
	}

	// Нет данных -> nil.
	assert.Nil(t, buildPeakHours("car_entries_count", &models.ReportResponse{Total: 0}))
}

func TestTopItems(t *testing.T) {
	r := &models.ReportResponse{
		Rows: []models.ReportAggregateRow{
			{Label: "Дебаркадер №1", Value: 12},
			{Label: "Склад A", Value: 7},
			{Label: "Склад B", Value: 3},
		},
	}
	top := topItems("car_entries_count", r, 2)
	assert.Len(t, top, 2)
	assert.Equal(t, "Дебаркадер №1", top[0].Label)
	assert.Equal(t, int64(12), top[0].Value)
}
