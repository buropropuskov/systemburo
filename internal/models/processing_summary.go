package models

// Бандл curated-вкладки «Обработка заявок» (#1240, B4): всё, что вкладка рисует,
// одним запросом — KPI этапов пути заявки, качество обработки, медленные
// согласующие и разбивка по организациям. Конструктор отчётов те же метрики
// отдаёт по одной; вкладке нужен готовый набор, иначе она бьёт полтора десятка
// запросов на открытие.
//
// Длительности здесь — СЕКУНДЫ (как их отдаёт движок, см. ReportValueDuration),
// доли — уже дробь (движок везёт их домноженными, бандл делит обратно).
//
// «Нет данных» != 0. Значения этапов и качества — указатели: nil означает, что
// считать было не по чему (этап не прошла ни одна заявка периода), и вкладка
// рисует прочерк. Без этого пустой период показывал бы «0 мин» — SQL-агрегат по
// пустой выборке приводится к нулю, чтобы скан не падал на NULL.

// Направление изменения KPI к прошлому периоду.
const (
	ProcessingDirectionUp   = "up"
	ProcessingDirectionDown = "down"
	ProcessingDirectionFlat = "flat"
)

// Как читать изменение KPI. Направление — факт (значение выросло), тональность —
// смысл (для времени обработки и доли отказов рост это ухудшение). Разделены
// намеренно: дашборд красит рост зелёным, и для метрик этой вкладки такая
// раскраска врала бы — цвет выбирается по Sentiment, а не по Direction.
const (
	ProcessingSentimentGood    = "good"
	ProcessingSentimentBad     = "bad"
	ProcessingSentimentNeutral = "neutral"
)

// ProcessingSummary — ответ GET /statistics/processing-summary.
type ProcessingSummary struct {
	From string `json:"from"`
	To   string `json:"to"`
	// TotalApplications — заявки, поданные за период: знаменатель качества и
	// признак «данные вообще есть» (0 -> доли не считаются, а не рисуются нулём).
	TotalApplications int64 `json:"total_applications"`

	Stages        []ProcessingStageKPI    `json:"stages"`
	Quality       []ProcessingQualityKPI  `json:"quality"`
	SlowApprovers []ProcessingApproverKPI `json:"slow_approvers"`
	// Разбивки по этапам: один и тот же набор колонок в двух разрезах, вкладка
	// переключает их одной таблицей (#1251 polish, п.10). Дольше всего сверху.
	ByOrganization []ProcessingBreakdownRow `json:"by_organization"`
	ByCompany      []ProcessingBreakdownRow `json:"by_company"`

	// Approvers/Acceptors — полные рейтинги по скорости (#1251 S3), которые вкладка
	// (S6) рисует таблицами. Approvers — согласующие по времени реакции (полный
	// список, не только топ-5 медленных из SlowApprovers). Acceptors — принимающие
	// по времени принятия в работу. Оба ранжированы по скорости: быстрые сверху, у
	// кого длительности нет (не отвечал / принятых с валидной парой нет) — в конце.
	Approvers []ProcessingApproverKPI `json:"approvers"`
	Acceptors []ProcessingAcceptorKPI `json:"acceptors"`
}

// ProcessingTrend — сравнение KPI с предыдущим периодом равной длины.
type ProcessingTrend struct {
	DeltaPct  float64 `json:"delta_pct"`
	Direction string  `json:"direction"`
	Sentiment string  `json:"sentiment"`
}

// ProcessingStageKPI — этап пути заявки: сколько он занимает в среднем и по
// 90-му перцентилю. Перцентиль рядом со средним намеренно: одна зависшая заявка
// тянет среднее вверх, и без p90 не видно, норма это или выброс.
type ProcessingStageKPI struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Samples — по скольким заявкам посчитан этап (прошли его в этом периоде).
	// 0 -> Avg/P90 nil: этап не проходил никто, а не «прошёл мгновенно».
	Samples int64  `json:"samples"`
	Avg     *int64 `json:"avg"`
	P90     *int64 `json:"p90"`
	// PrevAvg и Trend — сравнение среднего с прошлым периодом. nil, если сравнивать
	// не с чем (в одном из периодов этап никто не прошёл).
	PrevAvg *int64           `json:"prev_avg"`
	Trend   *ProcessingTrend `json:"trend,omitempty"`
}

// ProcessingQualityKPI — качество обработки: доля отказов, среднее число пересылок.
type ProcessingQualityKPI struct {
	Key       string           `json:"key"`
	Label     string           `json:"label"`
	Unit      string           `json:"unit,omitempty"`
	Value     *float64         `json:"value"`
	PrevValue *float64         `json:"prev_value"`
	Trend     *ProcessingTrend `json:"trend,omitempty"`
}

// ProcessingApproverKPI — согласующий в топе медленных: время реакции и нагрузка.
// AvgResponseTime nil — согласующий не отдал ни одного голоса за период (нагрузка
// при этом ненулевая: заявки на него завели, он их ещё не разобрал).
type ProcessingApproverKPI struct {
	Name            string `json:"name"`
	AvgResponseTime *int64 `json:"avg_response_time"`
	VotesCount      int64  `json:"votes_count"`
}

// ProcessingAcceptorKPI — принимающий в рейтинге (#1251 S3): как быстро он
// забирает заявку в работу после согласования и сколько заявок принял.
// AvgAcceptanceTime nil — принятых им заявок с валидной длительностью за период
// нет (нагрузка при этом может быть ненулевой).
type ProcessingAcceptorKPI struct {
	Name              string `json:"name"`
	AvgAcceptanceTime *int64 `json:"avg_acceptance_time"`
	AcceptsCount      int64  `json:"accepts_count"`
}

// ProcessingBreakdownRow — строка разбивки по организации или компании: сколько у
// неё занимают этапы обработки и сколько заявок она подала за период (#1251 polish,
// п.10 - раньше здесь было только общее время обработки и только по организациям).
// Длительности — указатели: nil означает, что этап не прошла ни одна её заявка, и
// вкладка рисует прочерк, а не «0 секунд».
type ProcessingBreakdownRow struct {
	Label             string `json:"label"`
	AvgApprovalTime   *int64 `json:"avg_approval_time"`
	AvgAcceptanceTime *int64 `json:"avg_acceptance_time"`
	AvgProcessingTime *int64 `json:"avg_processing_time"`
	ApplicationsCount int64  `json:"applications_count"`
}
