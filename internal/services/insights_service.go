package services

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"systemburo/internal/models"
)

// Метрики, на которых строятся инсайты (#632, E1).
const (
	insightApps   = "applications_count"
	insightCars   = "car_entries_count"
	insightPeople = "people_entries_count"
)

// insightDeltaThreshold — порог в %, ниже которого изменение считаем «без
// динамики» (flat), чтобы шум не выглядел трендом.
const insightDeltaThreshold = 5.0

// GetInsights отдаёт инсайты за период из тёплого кэша (если включён), иначе
// считает напрямую. Результат кэшируется снимком в БД и переживает рестарт.
func (s *statisticsService) GetInsights(ctx context.Context, from, to string) (*models.InsightsResponse, error) {
	if s.insightsCache == nil {
		return s.computeInsights(ctx, from, to)
	}
	f, errF := time.Parse("2006-01-02", from)
	t, errT := time.Parse("2006-01-02", to)
	if errF != nil || errT != nil {
		return s.computeInsights(ctx, from, to) // невалидные даты - мимо кэша
	}
	return s.insightsCache.get(ctx, f, t)
}

// computeInsights собирает инсайты за период: пик нагрузки по часам, сравнение
// с предыдущим периодом, топ мест и организаций, тренды. Всё считается вызовами
// RunReport (движок отчётов), нового SQL нет.
//
// MVP: ~13 последовательных запросов на вызов. Если станет узким местом — блок
// comparison (6 запросов) сворачивается в один запрос с двумя CTE по периодам.
func (s *statisticsService) computeInsights(ctx context.Context, from, to string) (*models.InsightsResponse, error) {
	resp := &models.InsightsResponse{
		PeakHours:   []models.PeakHoursInsight{},
		Comparisons: []models.ComparisonInsight{},
		TopPlaces:   []models.TopItemInsight{},
		TopOrgs:     []models.TopItemInsight{},
		Trends:      []models.TrendInsight{},
	}

	// Пик по часам — для въездов машин и проходов людей.
	for _, metric := range []string{insightCars, insightPeople} {
		r, err := s.runInsightAgg(ctx, metric, "hour_of_day", from, to, "", 0)
		if err != nil {
			return nil, fmt.Errorf("insight peak hours %s: %w", metric, err)
		}
		if ph := buildPeakHours(metric, r); ph != nil {
			resp.PeakHours = append(resp.PeakHours, *ph)
		}
	}

	// Сравнение с предыдущим периодом равной длины.
	prevFrom, prevTo := prevPeriod(from, to)
	for _, metric := range []string{insightApps, insightCars, insightPeople} {
		cur, err := s.runInsightAgg(ctx, metric, "none", from, to, "", 0)
		if err != nil {
			return nil, fmt.Errorf("insight comparison current %s: %w", metric, err)
		}
		prev, err := s.runInsightAgg(ctx, metric, "none", prevFrom, prevTo, "", 0)
		if err != nil {
			return nil, fmt.Errorf("insight comparison previous %s: %w", metric, err)
		}
		resp.Comparisons = append(resp.Comparisons, buildComparison(metric, cur.Total, prev.Total))
	}

	// Топ мест разгрузки по въездам машин.
	places, err := s.runInsightAgg(ctx, insightCars, "unload_place", from, to, "", 5)
	if err != nil {
		return nil, fmt.Errorf("insight top places: %w", err)
	}
	resp.TopPlaces = topItems(insightCars, places, 5)

	// Топ организаций по числу заявок.
	orgs, err := s.runInsightAgg(ctx, insightApps, "organization", from, to, "", 5)
	if err != nil {
		return nil, fmt.Errorf("insight top orgs: %w", err)
	}
	resp.TopOrgs = topItems(insightApps, orgs, 5)

	// Тренды по дням.
	for _, metric := range []string{insightApps, insightCars, insightPeople} {
		r, err := s.runInsightAgg(ctx, metric, "period", from, to, "day", 0)
		if err != nil {
			return nil, fmt.Errorf("insight trend %s: %w", metric, err)
		}
		resp.Trends = append(resp.Trends, buildTrend(metric, r))
	}

	return resp, nil
}

// runInsightAgg — вызов агрегатного движка с фильтром периода.
func (s *statisticsService) runInsightAgg(ctx context.Context, metric, dimension, from, to, granularity string, limit int) (*models.ReportResponse, error) {
	req := models.ReportRequest{
		Mode:        "aggregate",
		Metric:      metric,
		Dimension:   dimension,
		Granularity: granularity,
		Limit:       limit,
		Filters:     []models.ReportFilterValue{{Key: "date_range", From: from, To: to}},
	}
	return s.RunReport(ctx, req)
}

// metricMeta достаёт человекочитаемые подпись и единицу метрики из реестра.
func metricMeta(key string) (string, string) {
	if d, ok := reportMetricRegistry[key]; ok {
		return d.label, d.unit
	}
	return key, ""
}

// buildPeakHours переводит почасовое распределение в инсайт с выделенным пиком.
// Возвращает nil, если данных за период нет.
func buildPeakHours(metric string, r *models.ReportResponse) *models.PeakHoursInsight {
	if r == nil || r.Total == 0 {
		return nil
	}
	label, unit := metricMeta(metric)
	buckets := make([]models.HourBucket, 0, len(r.Rows))
	peakHour, peakValue := 0, int64(-1)
	for _, row := range r.Rows {
		hour, err := strconv.Atoi(row.Label)
		if err != nil {
			continue
		}
		buckets = append(buckets, models.HourBucket{Hour: hour, Value: row.Value})
		if row.Value > peakValue {
			peakValue = row.Value
			peakHour = hour
		}
	}
	if len(buckets) == 0 {
		return nil
	}
	return &models.PeakHoursInsight{
		Metric:    metric,
		Label:     label,
		Unit:      unit,
		PeakHour:  peakHour,
		PeakValue: peakValue,
		Hourly:    buckets,
	}
}

// buildComparison считает дельту текущего периода к предыдущему.
func buildComparison(metric string, current, previous int64) models.ComparisonInsight {
	label, unit := metricMeta(metric)
	delta := 0.0
	if previous != 0 {
		delta = float64(current-previous) / float64(previous) * 100
	} else if current > 0 {
		delta = 100
	}
	return models.ComparisonInsight{
		Metric:    metric,
		Label:     label,
		Unit:      unit,
		Current:   current,
		Previous:  previous,
		DeltaPct:  roundOne(delta),
		Direction: classifyDelta(delta),
	}
}

// topItems превращает строки агрегата в рейтинг (движок уже отсортировал по
// убыванию значения для категориального разреза).
func topItems(metric string, r *models.ReportResponse, limit int) []models.TopItemInsight {
	out := []models.TopItemInsight{}
	if r == nil {
		return out
	}
	for i, row := range r.Rows {
		if i >= limit {
			break
		}
		out = append(out, models.TopItemInsight{Metric: metric, Label: row.Label, Value: row.Value})
	}
	return out
}

// buildTrend строит ряд по дням и определяет направление (сравнение средних
// первой и второй половины периода).
func buildTrend(metric string, r *models.ReportResponse) models.TrendInsight {
	label, _ := metricMeta(metric)
	series := make([]int64, 0, len(r.Rows))
	for _, row := range r.Rows {
		series = append(series, row.Value)
	}
	return models.TrendInsight{
		Metric:    metric,
		Label:     label,
		Direction: trendDirection(series),
		Series:    series,
	}
}

// prevPeriod возвращает предыдущий период той же длины, заканчивающийся за день до
// начала текущего. Невалидные даты возвращает как есть.
func prevPeriod(from, to string) (string, string) {
	const layout = "2006-01-02"
	f, errF := time.Parse(layout, from)
	t, errT := time.Parse(layout, to)
	if errF != nil || errT != nil || t.Before(f) {
		return from, to
	}
	days := int(t.Sub(f).Hours()/24) + 1
	prevTo := f.AddDate(0, 0, -1)
	prevFrom := prevTo.AddDate(0, 0, -(days - 1))
	return prevFrom.Format(layout), prevTo.Format(layout)
}

// classifyDelta — направление по проценту изменения с порогом.
func classifyDelta(delta float64) string {
	if delta > insightDeltaThreshold {
		return "up"
	}
	if delta < -insightDeltaThreshold {
		return "down"
	}
	return "flat"
}

// trendDirection сравнивает средние первой и второй половин ряда.
func trendDirection(series []int64) string {
	n := len(series)
	if n < 2 {
		return "flat"
	}
	mid := n / 2
	var firstSum, secondSum int64
	for i := 0; i < mid; i++ {
		firstSum += series[i]
	}
	for i := mid; i < n; i++ {
		secondSum += series[i]
	}
	firstAvg := float64(firstSum) / float64(mid)
	secondAvg := float64(secondSum) / float64(n-mid)
	if firstAvg == 0 {
		if secondAvg > 0 {
			return "up"
		}
		return "flat"
	}
	return classifyDelta((secondAvg - firstAvg) / firstAvg * 100)
}

// roundOne округляет до одного знака после запятой.
func roundOne(v float64) float64 {
	return math.Round(v*10) / 10
}
