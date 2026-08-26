package services

import (
	"testing"

	"systemburo/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Чистая логика бандла «Обработка заявок» (#1240, B4). Значения из БД проверяет
// интеграционный тест в internal/handlers (там же и живут DB-тесты проекта);
// здесь — правила, по которым сырые числа превращаются в KPI.

// TestBuildProcessingTrend_SentimentLowerIsBetter — тональность метрик вкладки:
// это время обработки и отказы, поэтому рост читается как ухудшение. Направление
// (куда двинулось) и тональность (как читать) — разные вещи: дашборд красит рост
// зелёным, и без этого разделения вкладка врала бы цветом.
func TestBuildProcessingTrend_SentimentLowerIsBetter(t *testing.T) {
	cases := []struct {
		name              string
		current, previous float64
		wantDelta         float64
		wantDirection     string
		wantSentiment     string
	}{
		{"рост времени — ухудшение", 7200, 3600, 100, models.ProcessingDirectionUp, models.ProcessingSentimentBad},
		{"спад времени — улучшение", 1800, 3600, -50, models.ProcessingDirectionDown, models.ProcessingSentimentGood},
		{"в пределах порога — без оценки", 3700, 3600, 2.8, models.ProcessingDirectionFlat, models.ProcessingSentimentNeutral},
		{"с нуля — рост на 100%", 3600, 0, 100, models.ProcessingDirectionUp, models.ProcessingSentimentBad},
		{"оба нуля — без изменений", 0, 0, 0, models.ProcessingDirectionFlat, models.ProcessingSentimentNeutral},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildProcessingTrend(c.current, c.previous)
			require.NotNil(t, got)
			assert.Equal(t, c.wantDelta, got.DeltaPct)
			assert.Equal(t, c.wantDirection, got.Direction)
			assert.Equal(t, c.wantSentiment, got.Sentiment)
		})
	}
}

// TestProcessingWindow_StageValue_NoDataIsNotZero — главный замок бандла: агрегат
// пустой выборки SQL приводит к нулю (durationRound — гард скана NULL в int64),
// поэтому «этап никто не прошёл» и «этап прошли мгновенно» на уровне значения
// неотличимы. Различает их размер выборки: samples=0 -> nil, а не 0.
func TestProcessingWindow_StageValue_NoDataIsNotZero(t *testing.T) {
	w := &processingWindow{
		durations: map[string]int64{"avg_approval_time": 0, "avg_completion_time": 86400},
		samples:   map[string]int64{"approval_time": 0, "completion_time": 1},
	}

	assert.Nil(t, w.stageValue(processingStageAvg, "approval_time"),
		"нулевой агрегат при пустой выборке — не значение, а заглушка скана")

	got := w.stageValue(processingStageAvg, "completion_time")
	require.NotNil(t, got)
	assert.Equal(t, int64(86400), *got)

	assert.Nil(t, w.stageValue(processingStageAvg, "acceptance_time"),
		"метрики без ключа в ответе движка — нет данных")
}

// TestProcessingWindow_RateValue_NoApplicationsIsNotZero — доля без заявок это
// «нет данных», а не 0%: за период не подавали заявок, отказывать было нечему.
func TestProcessingWindow_RateValue_NoApplicationsIsNotZero(t *testing.T) {
	empty := &processingWindow{rates: map[string]float64{"refusal_rate": 0}, applications: 0}
	assert.Nil(t, empty.rateValue("refusal_rate"))

	filled := &processingWindow{rates: map[string]float64{"refusal_rate": 0}, applications: 5}
	got := filled.rateValue("refusal_rate")
	require.NotNil(t, got, "заявки были, отказов не было — честный ноль")
	assert.Equal(t, 0.0, *got)
}

// TestSortByOptionalDesc_NoDataLast — строки без значения уходят вниз: согласующий
// без ответов не медленный, он не отвечал, и в топе медленных ему не место.
func TestSortByOptionalDesc_NoDataLast(t *testing.T) {
	sec := func(v int64) *int64 { return &v }
	rows := []models.ProcessingApproverKPI{
		{Name: "без ответов", AvgResponseTime: nil},
		{Name: "быстрый", AvgResponseTime: sec(600)},
		{Name: "медленный", AvgResponseTime: sec(7200)},
	}

	sortByOptionalDesc(rows, func(r models.ProcessingApproverKPI) *int64 { return r.AvgResponseTime })

	assert.Equal(t, []string{"медленный", "быстрый", "без ответов"},
		[]string{rows[0].Name, rows[1].Name, rows[2].Name})
}

// TestBuildProcessingStages_TrendNeedsBothPeriods — сравнивать не с чем, если в
// одном из периодов этап никто не прошёл: дельта от нулевой заглушки показала бы
// выдуманное ухудшение на 100%.
func TestBuildProcessingStages_TrendNeedsBothPeriods(t *testing.T) {
	current := &processingWindow{
		durations: map[string]int64{"avg_approval_time": 7200, "p90_approval_time": 7200},
		samples:   map[string]int64{"approval_time": 2},
	}
	previous := &processingWindow{durations: map[string]int64{}, samples: map[string]int64{}}

	stages := buildProcessingStages(current, previous)
	require.Len(t, stages, len(durationStages))

	approval := stages[0]
	assert.Equal(t, "approval_time", approval.Key)
	assert.Equal(t, "Время согласования", approval.Label, "подпись этапа, а не метрики каталога")
	require.NotNil(t, approval.Avg)
	assert.Equal(t, int64(7200), *approval.Avg)
	assert.Nil(t, approval.PrevAvg)
	assert.Nil(t, approval.Trend, "прошлого значения нет — тренда нет")
}
