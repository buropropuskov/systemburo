package services

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"systemburo/internal/models"
)

// Curated-бандл вкладки «Обработка заявок» (#1240, B4). Собирается из тех же
// метрик, что публикует конструктор отчётов (B2/B3) — своего SQL для агрегатов
// здесь нет, иначе цифры вкладки и конструктора разъехались бы. Вкладка бьёт
// один запрос, бандл внутри исполняет полтора десятка планов, поэтому результат
// кэшируется по периоду, как тяжёлая сводка дашборда.
//
// Два транспорта значений, которые нельзя путать (B2/B3):
//
//	длительности — целые СЕКУНДЫ в Totals/Values (Type="duration");
//	доли         — дробь в FloatTotals/FloatValues (Float=true).
//
// Оба при пустой выборке приходят нулём (COALESCE в SQL — гард скана NULL в
// int64), поэтому «нет данных» ловится не значением, а счётчиком выборки:
// Samples у этапа и TotalApplications у долей.

// processingTopN — сколько строк отдаём в топе медленных согласующих. Только там:
// рейтинги людей и разбивка по организациям/компаниям идут ПОЛНЫМИ списками, вкладка
// показывает их прокручиваемыми таблицами (#1251 S6 и polish п.10).
const processingTopN = 5

// processingQualityMetrics — метрики качества бандла в порядке показа. Отказ
// принимающего и несогласование показываем ПО ОТДЕЛЬНОСТИ (#1251 polish, п.8): в
// объединённой доле не видно, кто завернул заявку. Объединённая refusal_rate
// осталась в конструкторе отчётов.
var processingQualityMetrics = []string{"rejected_rate", "not_approved_rate", "avg_forwards"}

// processingStageAggs — агрегаты этапа, попадающие в KPI-плитку.
const (
	processingStageAvg = "avg"
	processingStageP90 = "p90"
)

// GetProcessingSummary возвращает бандл вкладки «Обработка заявок» за период
// [from, to] — KPI этапов со сравнением с прошлым периодом равной длины,
// качество обработки, топ медленных согласующих и разбивку по организациям.
func (s *statisticsService) GetProcessingSummary(ctx context.Context, from, to time.Time) (*models.ProcessingSummary, error) {
	if s.processingCache != nil {
		return s.processingCache.get(ctx, from, to)
	}
	return s.computeProcessingSummary(ctx, from, to)
}

// computeProcessingSummary считает бандл. Дорого (каждая метрика — свой план), но
// целиком кэшируется: realtime-показателей, как у сводки дашборда, здесь нет.
func (s *statisticsService) computeProcessingSummary(ctx context.Context, from, to time.Time) (*models.ProcessingSummary, error) {
	fromStr, toStr := dateKey(from), dateKey(to)
	prevFromStr, prevToStr := prevPeriod(fromStr, toStr)

	current, err := s.runProcessingWindow(ctx, fromStr, toStr)
	if err != nil {
		return nil, err
	}
	previous, err := s.runProcessingWindow(ctx, prevFromStr, prevToStr)
	if err != nil {
		return nil, err
	}

	out := &models.ProcessingSummary{
		From:              fromStr,
		To:                toStr,
		TotalApplications: current.applications,
		Stages:            buildProcessingStages(current, previous),
		Quality:           buildProcessingQuality(current, previous),
	}

	// Согласующих и принимающих забираем одним набором каждого и ранжируем в Go:
	// топ медленных (SlowApprovers) и полный рейтинг по скорости (Approvers) — из
	// одних строк, разница только в сортировке.
	approvers, err := s.runApproverKPIs(ctx, fromStr, toStr)
	if err != nil {
		return nil, err
	}
	out.SlowApprovers = slowApprovers(approvers)
	out.Approvers = rankApproversBySpeed(approvers)

	acceptors, err := s.runAcceptorKPIs(ctx, fromStr, toStr)
	if err != nil {
		return nil, err
	}
	out.Acceptors = rankAcceptorsBySpeed(acceptors)

	if out.ByOrganization, err = s.runProcessingBreakdown(ctx, "organization", fromStr, toStr); err != nil {
		return nil, err
	}
	if out.ByCompany, err = s.runProcessingBreakdown(ctx, "company", fromStr, toStr); err != nil {
		return nil, err
	}
	return out, nil
}

// processingWindow — сырые значения метрик за одно окно: длительности этапов,
// доли качества, число заявок и размер выборки каждого этапа.
type processingWindow struct {
	durations    map[string]int64   // ключ метрики (avg_approval_time...) -> секунды
	rates        map[string]float64 // refusal_rate / avg_forwards -> дробь
	applications int64
	samples      map[string]int64 // ключ этапа (approval_time...) -> заявок в выборке
}

// runProcessingWindow считает метрики бандла за окно без разреза: одна строка
// «Итого» на метрику, значения забираем из итогов ответа.
func (s *statisticsService) runProcessingWindow(ctx context.Context, from, to string) (*processingWindow, error) {
	metrics := make([]string, 0, len(durationStages)*2+len(processingQualityMetrics)+1)
	for _, st := range durationStages {
		metrics = append(metrics, processingStageAvg+"_"+st.key, processingStageP90+"_"+st.key)
	}
	metrics = append(metrics, processingQualityMetrics...)
	metrics = append(metrics, "applications_count")

	resp, err := s.RunReport(ctx, models.ReportRequest{
		Mode:      "aggregate",
		Metrics:   metrics,
		Dimension: dimNone,
		Filters:   []models.ReportFilterValue{{Key: "date_range", From: from, To: to}},
	})
	if err != nil {
		return nil, fmt.Errorf("statistics: processing window %s..%s: %w", from, to, err)
	}

	w := &processingWindow{
		durations: make(map[string]int64, len(durationStages)*2),
		rates:     make(map[string]float64, len(processingQualityMetrics)),
	}
	for k, v := range resp.Totals {
		w.durations[k] = v
	}
	for _, m := range processingQualityMetrics {
		if v, ok := resp.FloatTotals[m]; ok {
			w.rates[m] = v
		}
	}
	w.applications = resp.Totals["applications_count"]

	if w.samples, err = s.stageSamples(ctx, from, to); err != nil {
		return nil, err
	}
	return w, nil
}

// stageSamples считает, сколько заявок периода реально прошли каждый этап. Без
// этого счётчика пустой этап неотличим от мгновенного: агрегат по пустой выборке
// SQL приводит к нулю (durationRound), и «никто не завершил» читалось бы как
// «завершают за 0 секунд». Условия этапов и окно берём из тех же durationStages и
// tsColumn, по которым движок строит сами метрики, чтобы выборки совпадали.
func (s *statisticsService) stageSamples(ctx context.Context, from, to string) (map[string]int64, error) {
	selects := make([]string, 0, len(durationStages))
	for _, st := range durationStages {
		selects = append(selects, fmt.Sprintf("COUNT(*) FILTER (WHERE %s) AS %s", st.baseWhere, st.key))
	}

	tx := s.db.WithContext(ctx).Table("applications app").Select(selects)
	if t, ok := parseReportDate(from, false); ok {
		tx = tx.Where("app.sending_datetime >= ?", t)
	}
	if t, ok := parseReportDate(to, true); ok {
		tx = tx.Where("app.sending_datetime <= ?", t)
	}

	row := map[string]any{}
	if err := tx.Take(&row).Error; err != nil {
		return nil, fmt.Errorf("statistics: processing stage samples: %w", err)
	}

	out := make(map[string]int64, len(durationStages))
	for _, st := range durationStages {
		if v, ok := row[st.key].(int64); ok {
			out[st.key] = v
		}
	}
	return out, nil
}

// buildProcessingStages собирает KPI этапов в порядке пути заявки.
func buildProcessingStages(current, previous *processingWindow) []models.ProcessingStageKPI {
	stages := make([]models.ProcessingStageKPI, 0, len(durationStages))
	for _, st := range durationStages {
		kpi := models.ProcessingStageKPI{
			Key:     st.key,
			Label:   stageLabel(st),
			Samples: current.samples[st.key],
			Avg:     current.stageValue(processingStageAvg, st.key),
			P90:     current.stageValue(processingStageP90, st.key),
			PrevAvg: previous.stageValue(processingStageAvg, st.key),
		}
		if kpi.Avg != nil && kpi.PrevAvg != nil {
			kpi.Trend = buildProcessingTrend(float64(*kpi.Avg), float64(*kpi.PrevAvg))
		}
		stages = append(stages, kpi)
	}
	return stages
}

// stageValue — значение агрегата этапа или nil, если этап никто не прошёл (см.
// stageSamples: ноль от SQL в этом случае не значение, а заглушка скана).
func (w *processingWindow) stageValue(agg, stageKey string) *int64 {
	if w.samples[stageKey] == 0 {
		return nil
	}
	v, ok := w.durations[agg+"_"+stageKey]
	if !ok {
		return nil
	}
	return &v
}

// stageLabel — подпись этапа с большой буквы: в каталоге метрик подписи вида
// «Среднее время согласования», а плитке нужен сам этап («Время согласования»).
func stageLabel(st durationStage) string {
	r := []rune(st.label)
	if len(r) == 0 {
		return st.key
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

// buildProcessingQuality собирает KPI качества. Доли считаются от заявок периода:
// нет заявок — нет и доли (0% отказов при нуле заявок ввело бы в заблуждение).
func buildProcessingQuality(current, previous *processingWindow) []models.ProcessingQualityKPI {
	out := make([]models.ProcessingQualityKPI, 0, len(processingQualityMetrics))
	for _, m := range processingQualityMetrics {
		label, unit := metricMeta(m)
		kpi := models.ProcessingQualityKPI{
			Key:       m,
			Label:     label,
			Unit:      unit,
			Value:     current.rateValue(m),
			PrevValue: previous.rateValue(m),
		}
		if kpi.Value != nil && kpi.PrevValue != nil {
			kpi.Trend = buildProcessingTrend(*kpi.Value, *kpi.PrevValue)
		}
		out = append(out, kpi)
	}
	return out
}

// rateValue — доля или nil, если за период не подавали заявок.
func (w *processingWindow) rateValue(metric string) *float64 {
	if w.applications == 0 {
		return nil
	}
	v, ok := w.rates[metric]
	if !ok {
		return nil
	}
	return &v
}

// buildProcessingTrend сравнивает KPI с прошлым периодом. Тональность считается
// от направления: все метрики вкладки — время и отказы, для них рост это
// ухудшение (см. ProcessingSentimentBad).
func buildProcessingTrend(current, previous float64) *models.ProcessingTrend {
	delta := 0.0
	switch {
	case previous != 0:
		delta = (current - previous) / previous * 100
	case current > 0:
		delta = 100
	}
	direction := classifyDelta(delta)
	return &models.ProcessingTrend{
		DeltaPct:  roundOne(delta),
		Direction: direction,
		Sentiment: lowerIsBetterSentiment(direction),
	}
}

// lowerIsBetterSentiment — тональность метрики, которой лучше быть меньше.
func lowerIsBetterSentiment(direction string) string {
	switch direction {
	case models.ProcessingDirectionUp:
		return models.ProcessingSentimentBad
	case models.ProcessingDirectionDown:
		return models.ProcessingSentimentGood
	default:
		return models.ProcessingSentimentNeutral
	}
}

// runApproverKPIs — согласующие с временем реакции и нагрузкой, БЕЗ сортировки:
// порядок задаёт вызывающий (топ медленных и полный рейтинг по скорости строятся
// из одного набора). Движком не сортируем: при мультиметриках он упорядочивает
// строки по СУММЕ значений метрик, и секунды времени реакции смешались бы со
// штуками нагрузки.
func (s *statisticsService) runApproverKPIs(ctx context.Context, from, to string) ([]models.ProcessingApproverKPI, error) {
	resp, err := s.RunReport(ctx, models.ReportRequest{
		Mode:      "aggregate",
		Metrics:   []string{"avg_approver_response_time", "approver_votes_count"},
		Dimension: dimByApprover,
		Filters:   []models.ReportFilterValue{{Key: "date_range", From: from, To: to}},
		// Забираем строки широко: свой лимит движок применил бы ПОСЛЕ упорядочивания
		// по сумме метрик, и самый медленный согласующий мог бы не дожить до нашей
		// сортировки, не попав в первую сотню строк.
		Limit: maxReportLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("statistics: processing approvers: %w", err)
	}

	rows := make([]models.ProcessingApproverKPI, 0, len(resp.MetricRows))
	for _, r := range resp.MetricRows {
		rows = append(rows, models.ProcessingApproverKPI{
			Name: r.Label,
			// Ключ отсутствует — согласующий не отдал ни одного голоса за период
			// (движок не дорисовывает ноль производным метрикам, metricOmitsFakeZero).
			AvgResponseTime: optionalValue(r.Values, "avg_approver_response_time"),
			VotesCount:      r.Values["approver_votes_count"],
		})
	}
	return rows, nil
}

// runAcceptorKPIs — принимающие с временем принятия в работу и нагрузкой (#1251 S3),
// БЕЗ сортировки (симметрично согласующим). Строка = один принимающий: база метрики
// — первое принятие каждой заявки, разрез by_acceptor заявки не размножает.
func (s *statisticsService) runAcceptorKPIs(ctx context.Context, from, to string) ([]models.ProcessingAcceptorKPI, error) {
	resp, err := s.RunReport(ctx, models.ReportRequest{
		Mode:      "aggregate",
		Metrics:   []string{"avg_acceptor_response_time", "acceptor_accepts_count"},
		Dimension: dimByAcceptor,
		Filters:   []models.ReportFilterValue{{Key: "date_range", From: from, To: to}},
		Limit:     maxReportLimit, // как у согласующих: топ отбираем сами, движок сортирует по сумме
	})
	if err != nil {
		return nil, fmt.Errorf("statistics: processing acceptors: %w", err)
	}

	rows := make([]models.ProcessingAcceptorKPI, 0, len(resp.MetricRows))
	for _, r := range resp.MetricRows {
		rows = append(rows, models.ProcessingAcceptorKPI{
			Name:              r.Label,
			AvgAcceptanceTime: optionalValue(r.Values, "avg_acceptor_response_time"),
			AcceptsCount:      r.Values["acceptor_accepts_count"],
		})
	}
	return rows, nil
}

// slowApprovers — топ медленных согласующих: по убыванию времени реакции, первые N.
// Согласующие без ответов (время nil) уходят вниз — они не медленные, не отвечали.
func slowApprovers(rows []models.ProcessingApproverKPI) []models.ProcessingApproverKPI {
	out := append([]models.ProcessingApproverKPI(nil), rows...)
	sortByOptionalDesc(out, func(r models.ProcessingApproverKPI) *int64 { return r.AvgResponseTime })
	return topN(out, processingTopN)
}

// rankApproversBySpeed — полный рейтинг согласующих по скорости реакции: быстрые
// сверху, без ответов — в конце. Копия, чтобы не мутировать общий набор.
func rankApproversBySpeed(rows []models.ProcessingApproverKPI) []models.ProcessingApproverKPI {
	out := append([]models.ProcessingApproverKPI(nil), rows...)
	sortByOptionalAsc(out, func(r models.ProcessingApproverKPI) *int64 { return r.AvgResponseTime })
	return out
}

// rankAcceptorsBySpeed — полный рейтинг принимающих по скорости принятия: быстрые
// сверху, без валидной длительности — в конце.
func rankAcceptorsBySpeed(rows []models.ProcessingAcceptorKPI) []models.ProcessingAcceptorKPI {
	out := append([]models.ProcessingAcceptorKPI(nil), rows...)
	sortByOptionalAsc(out, func(r models.ProcessingAcceptorKPI) *int64 { return r.AvgAcceptanceTime })
	return out
}

// runProcessingBreakdown — разбивка этапов обработки в заданном разрезе
// (организация или компания), дольше всего сверху. Возвращает ПОЛНЫЙ список:
// вкладка показывает его прокручиваемой таблицей, а не топом (#1251 polish, п.10).
// Разреза по типу вложения здесь нет намеренно: вложения к заявке 1:N, join
// размножил бы её строку и взвесил среднее по числу вложений (см.
// report_duration_metrics.go).
func (s *statisticsService) runProcessingBreakdown(ctx context.Context, dimension, from, to string) ([]models.ProcessingBreakdownRow, error) {
	resp, err := s.RunReport(ctx, models.ReportRequest{
		Mode: "aggregate",
		Metrics: []string{
			"avg_approval_time", "avg_acceptance_time", "avg_processing_time", "applications_count",
		},
		Dimension: dimension,
		Filters:   []models.ReportFilterValue{{Key: "date_range", From: from, To: to}},
		Limit:     maxReportLimit, // сортируем в Go, движок упорядочил бы по сумме метрик
	})
	if err != nil {
		return nil, fmt.Errorf("statistics: processing by %s: %w", dimension, err)
	}

	rows := make([]models.ProcessingBreakdownRow, 0, len(resp.MetricRows))
	for _, r := range resp.MetricRows {
		rows = append(rows, models.ProcessingBreakdownRow{
			Label:             r.Label,
			AvgApprovalTime:   optionalValue(r.Values, "avg_approval_time"),
			AvgAcceptanceTime: optionalValue(r.Values, "avg_acceptance_time"),
			AvgProcessingTime: optionalValue(r.Values, "avg_processing_time"),
			ApplicationsCount: r.Values["applications_count"],
		})
	}
	// Сортируем по полному времени обработки: вопрос разбивки - «где дольше всего
	// тянут», поэтому медленные сверху (в отличие от рейтингов людей, где сверху
	// быстрые). Строки без данных уходят вниз.
	sortByOptionalDesc(rows, func(r models.ProcessingBreakdownRow) *int64 { return r.AvgProcessingTime })
	return rows, nil
}

// optionalValue — значение метрики строки или nil, если движок не дал ключа
// («нет данных» у производных метрик, metricOmitsFakeZero).
func optionalValue(values map[string]int64, metric string) *int64 {
	v, ok := values[metric]
	if !ok {
		return nil
	}
	return &v
}

// sortByOptionalDesc — по убыванию значения, строки без значения в конец
// (стабильно: движок уже упорядочил их между собой).
func sortByOptionalDesc[T any](rows []T, value func(T) *int64) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := value(rows[i]), value(rows[j])
		if a == nil || b == nil {
			return a != nil && b == nil
		}
		return *a > *b
	})
}

// sortByOptionalAsc — по возрастанию значения (быстрые сверху), строки без значения
// в конец: nil это «нет данных», а не «мгновенно», в начале рейтинга ему не место.
func sortByOptionalAsc[T any](rows []T, value func(T) *int64) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := value(rows[i]), value(rows[j])
		if a == nil || b == nil {
			return a != nil && b == nil
		}
		return *a < *b
	})
}

func topN[T any](rows []T, n int) []T {
	if len(rows) > n {
		return rows[:n]
	}
	return rows
}

// dateKey — дата периода в формате фильтров движка (окно уже в МСК: границы
// собрал parseDateRange хендлера).
func dateKey(t time.Time) string {
	return t.Format("2006-01-02")
}
