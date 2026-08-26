package services

import (
	"fmt"

	"systemburo/internal/models"
)

// Метрики качества обработки заявок (#1240, B3): чем заявка закончилась и сколько
// раз её передавали из рук в руки.
//
//	refusal_rate      — доля заявок, которым отказал принимающий (статус «Отказано»)
//	                    ЛИБО которые не согласовали (confirmation «Не согласовано»).
//	rejected_rate     — только отказ принимающего (#1251 polish, п.8).
//	not_approved_rate — только несогласование согласующими (#1251 polish, п.8).
//	avg_forwards      — сколько раз заявку в среднем пересылали.
//
// Все метрики-доли ДРОБНЫЕ, а execAggregatePlan сканирует значение агрегата в int64:
// на PG16 сырое AVG/деление дают numeric, и скан падает в рантайме (go build
// такого не видит — только исполнение запроса). Поэтому SQL отдаёт значение,
// домноженное на rateScale (целое), а постпроцесс RunReport делит его обратно и
// кладёт дробь в FloatValues — тот же контракт, по которому фронт уже рисует
// avg_cars_per_day (колонка Float=true). Домноженное целое наружу не выходит.
//
// Итог таких метрик НЕ складывается из бинов (пять дней по 20% отказов — это не
// 100% отказов), его пересчитывает по всему окну execWindowTotal — см.
// metricIsDerived.

// rateScale — множитель транспорта дробного значения из SQL: 10 = один знак после
// запятой (33.3% едет как 333). Больше знаков доле отказов и среднему числу
// пересылок не нужно, а round1 в постпроцессе всё равно округляет до одного.
const rateScale = 10

// rateMetrics — метрики-доли, чьё значение SQL отдаёт домноженным на rateScale.
// Читают постпроцесс RunReport (обратное деление) и metricIsDerived.
var rateMetrics = map[string]bool{
	"refusal_rate":      true,
	"rejected_rate":     true,
	"not_approved_rate": true,
	"avg_forwards":      true,
}

// rateScaled поднимает дробное выражение до целого (x rateScale) и округляет.
// COALESCE — гард пустой выборки: агрегат по нулю строк даёт NULL, а NULL в int64
// не сканируется (тот же гард, что durationRound у длительностей).
func rateScaled(expr string) string {
	return fmt.Sprintf("COALESCE(ROUND((%s) * %d), 0)::bigint", expr, rateScale)
}

// refusalRateExpr — доля отказов в ПРОЦЕНТАХ (0..100). Отказ — это две ветки
// одного исхода «заявку не пропустили»: отказал принимающий (status) или не
// согласовали согласующие (confirmation), поэтому считаются обе. NULLIF —
// защита от деления на ноль. Значения статусов подставляются из констант кода
// (models.*), а не из пользовательского ввода.
var refusalRateExpr = fmt.Sprintf(
	"100 * COUNT(*) FILTER (WHERE app.confirmation = '%s' OR app.status = '%s')::numeric / NULLIF(COUNT(*), 0)",
	models.ConfirmationRejected, models.StatusRefused,
)

// Две ветки отказа по отдельности (#1251 polish, п.8): в сводной доле они слиты, и
// по ней не понять, кто «зарубил» заявку - согласующие или принимающий. Ветки
// НЕ взаимоисключающие (заявку могли не согласовать И получить отказ принимающего),
// поэтому их сумма не обязана равняться объединённой refusal_rate.

// rejectedRateExpr — доля заявок, которым отказал ПРИНИМАЮЩИЙ (статус «Отказано»).
var rejectedRateExpr = fmt.Sprintf(
	"100 * COUNT(*) FILTER (WHERE app.status = '%s')::numeric / NULLIF(COUNT(*), 0)",
	models.StatusRefused,
)

// notApprovedRateExpr — доля заявок, которые не согласовали СОГЛАСУЮЩИЕ
// (confirmation «Не согласовано»).
var notApprovedRateExpr = fmt.Sprintf(
	"100 * COUNT(*) FILTER (WHERE app.confirmation = '%s')::numeric / NULLIF(COUNT(*), 0)",
	models.ConfirmationRejected,
)

// avgForwardsExpr — среднее число пересылок на заявку. Коррелированный подзапрос,
// а не JOIN: join к audit_log размножил бы строку заявки по числу пересылок и
// исказил бы агрегат. Считаем сводные записи «переслал» (одна на действие
// пересылки), а не по-получательские assigned_*: иначе пересылка троим за раз
// считалась бы за три. Заявки без пересылок дают 0 и честно тянут среднее вниз.
//
// Метрика считает пересылки, добавившие НОВЫХ получателей: сводная запись пишется
// только в этом случае (ApplicationApprovalService), поэтому повторная пересылка
// тем же людям в неё не попадёт. Плюс сама запись появилась вместе с пересылкой
// вложений (#680) — у заявок, пересланных до неё, её нет. И там, и там метрика
// скорее занижена, чем завышена (как разрежен completed_at у длительностей, B1).
var avgForwardsExpr = fmt.Sprintf(
	"AVG((SELECT COUNT(*) FROM audit_log al WHERE al.entity_type = '%s' AND al.entity_id = app.id AND al.action = '%s'))",
	models.AuditEntityApplication, models.AuditActionForwarded,
)

// refusalRateDims — разрезы доли отказов: те же 1:1 join'ы, что у длительностей,
// но БЕЗ статуса. Группировка доли отказов по статусу тавтологична: группа
// «Отказано» даст 100%, остальные — почти всегда 0. Как ФИЛЬТР статус остаётся.
func refusalRateDims() map[string]aggColumn {
	dims := durationDims()
	delete(dims, "status")
	return dims
}

// Разрезы каталога. Тип вложения не даётся ни одной метрике заявки по той же
// причине, что и длительностям: attachments 1:N размножит строку заявки и
// исказит долю/среднее (см. report_duration_metrics.go).
var (
	refusalRateDimensions = []string{"organization", "company", "period"}
	avgForwardsDimensions = []string{"status", "organization", "company", "period"}
)

// init регистрирует метрики качества в движке (aggMetricRegistry) и каталоге
// (reportMetricRegistry + reportMetricOrder). Выражение агрегата считается один
// раз и кладётся в оба реестра — движок и каталог не разъедутся.
func init() {
	refusalAgg := rateScaled(refusalRateExpr)
	aggMetricRegistry["refusal_rate"] = aggMetricSchema{
		base:    "applications app",
		aggExpr: refusalAgg,
		// Окно и разрез period — по дате ПОДАЧИ, как у длительностей: «доля отказов
		// среди заявок, поданных за период», а не «среди отказов, случившихся в нём».
		tsColumn:  "app.sending_datetime",
		tsJoin:    jNone,
		unit:      "%",
		joinBlock: durationJoinBlock(),
		dims:      refusalRateDims(),
		filters:   durationFilters(),
	}
	reportMetricRegistry["refusal_rate"] = metricDef{
		// «и несогласований» в подписи: метрика объединяет обе ветки, а рядом теперь
		// живут они же по отдельности - без уточнения три доли путались бы.
		label:      "Доля отказов и несогласований",
		unit:       "%",
		group:      metricGroupProcessing,
		baseTable:  "applications",
		aggExpr:    refusalAgg,
		dimensions: refusalRateDimensions,
	}
	reportMetricOrder = append(reportMetricOrder, "refusal_rate")

	// Ветки отказа по отдельности: та же база и окно, что у объединённой. Разрезы
	// РАЗНЫЕ: отказ принимающего фильтрует по status, поэтому группировка по статусу
	// тавтологична (группа «Отказано» = 100%); несогласование фильтрует confirmation,
	// и «сколько несогласованных среди заявок каждого статуса» - осмысленный вопрос,
	// поэтому ему статус как разрез оставлен.
	for _, m := range []struct {
		key        string
		label      string
		expr       string
		dims       map[string]aggColumn
		dimensions []string
	}{
		{"rejected_rate", "Доля отказов принимающего", rejectedRateExpr, refusalRateDims(), refusalRateDimensions},
		{"not_approved_rate", "Доля несогласованных", notApprovedRateExpr, durationDims(), avgForwardsDimensions},
	} {
		agg := rateScaled(m.expr)
		aggMetricRegistry[m.key] = aggMetricSchema{
			base:      "applications app",
			aggExpr:   agg,
			tsColumn:  "app.sending_datetime",
			tsJoin:    jNone,
			unit:      "%",
			joinBlock: durationJoinBlock(),
			dims:      m.dims,
			filters:   durationFilters(),
		}
		reportMetricRegistry[m.key] = metricDef{
			label:      m.label,
			unit:       "%",
			group:      metricGroupProcessing,
			baseTable:  "applications",
			aggExpr:    agg,
			dimensions: m.dimensions,
		}
		reportMetricOrder = append(reportMetricOrder, m.key)
	}

	forwardsAgg := rateScaled(avgForwardsExpr)
	aggMetricRegistry["avg_forwards"] = aggMetricSchema{
		base:      "applications app",
		aggExpr:   forwardsAgg,
		tsColumn:  "app.sending_datetime",
		tsJoin:    jNone,
		unit:      "раз/заявку",
		joinBlock: durationJoinBlock(),
		dims:      durationDims(),
		filters:   durationFilters(),
	}
	reportMetricRegistry["avg_forwards"] = metricDef{
		label:      "Среднее число пересылок",
		unit:       "раз/заявку",
		group:      metricGroupProcessing,
		baseTable:  "applications",
		aggExpr:    forwardsAgg,
		dimensions: avgForwardsDimensions,
	}
	reportMetricOrder = append(reportMetricOrder, "avg_forwards")
}

// applyRateScale возвращает домноженные целые обратно в дробь: значение метрики
// переносится из Values в FloatValues (контракт дробных метрик — там же лежат
// avg-метрики). Строки без значения метрики не трогаются: их отсутствие означает
// «нет данных» (metricIsDerived), и дорисовывать туда 0.0 нельзя.
func applyRateScale(rows []models.ReportMetricRow, metric string) {
	for i := range rows {
		raw, ok := rows[i].Values[metric]
		if !ok {
			continue
		}
		delete(rows[i].Values, metric)
		if rows[i].FloatValues == nil {
			rows[i].FloatValues = map[string]float64{}
		}
		rows[i].FloatValues[metric] = round1(float64(raw) / rateScale)
	}
}
