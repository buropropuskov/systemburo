package services

import "systemburo/internal/models"

// Метрики длительностей обработки заявки (#1240, B2): сколько времени заявка
// проводит на каждом этапе пути (моменты заведены в B1):
//
//	T0 sending_datetime      - подача
//	T1 confirmation_datetime - согласование
//	T2 accepted_at           - принятие в работу
//	T3 completed_at          - завершение по истечении срока вложений
//
// Этапы, зависящие от РАБОТЫ людей Бюро (согласование T0->T1, принятие T1->T2,
// обработка T0->T2), считаются по рабочему времени Бюро (#1251 S2): ночь и выходные
// из длительности вычитаются, иначе метрика завышала сроки и была несправедлива к
// согласующим (поданная в пятницу вечером заявка «согласовывалась двое суток», хотя
// Бюро всё это время не работало). См. bureauWorkingDuration. «Время до завершения»
// (T0->T3) остаётся КАЛЕНДАРНЫМ: это срок действия пропуска (реальные сутки до
// истечения вложений), а не работа человека -- рабочие часы к нему неприменимы.
//
// Значение метрики - СЕКУНДЫ (bigint), колонка ответа несёт Type="duration", по
// нему фронт показывает «2 ч 15 мин» (форматируем по типу колонки, не по виду
// значения). Целые секунды, а не дробь: execAggregatePlan сканирует значение в
// int64, а на PG16 EXTRACT(EPOCH ...) возвращает numeric - без округления в SQL
// скан падает в рантайме (go build такое не ловит, только исполнение запроса).
// Секунды на длительностях в часах - точность с запасом.
//
// Каждый этап даёт три метрики: среднее, медиану (p50) и 90-й перцентиль.
// Перцентили честнее среднего на хвостах: одна зависшая на неделю заявка тянет
// avg вверх, но p50 остаётся показательным.

// durationStage - этап пути заявки: длительность и условие, при котором этап
// реально пройден. Незавершённые этапы (момент NULL) в агрегат не попадают:
// иначе «ещё не приняли» считалось бы нулевой длительностью и занижало метрику.
// baseWhere требует и <момент конца> >= <момент начала>: у части исторических
// заявок конечный момент раньше начального (напр. confirmation_datetime до
// sending_datetime) - такая пара даёт отрицательную длительность и утягивает
// среднее в минус («-2 сут 2 ч согласования»). Битую пару отсекаем из выборки
// (samples тоже её не считает), а не клампим в ноль: ноль исказил бы среднее,
// исключение - честно убирает некорректную строку. Гард нужен и рабочему времени:
// на календарном фолбэке битая пара даёт минус, а bureau_working_seconds для неё
// вернул бы 0 (to<=from) - тоже фейковая нулевая выборка, которую надо исключить.
type durationStage struct {
	key       string // суффикс ключа метрики: <agg>_<key>
	label     string // именительный падеж: «Среднее <label>»
	labelGen  string // родительный падеж: «Медиана <labelGen>»
	expr      string // длительность этапа в секундах
	baseWhere string
}

// durationBetween - длительность между моментами в секундах. Отдельный хелпер,
// чтобы порядок аргументов (позже минус раньше) читался в одном месте, а не
// переписывался руками в каждом этапе.
func durationBetween(from, to string) string {
	return "EXTRACT(EPOCH FROM (" + to + " - " + from + "))"
}

var durationStages = []durationStage{
	{
		key:       "approval_time",
		label:     "время согласования",
		labelGen:  "времени согласования",
		expr:      bureauWorkingDuration("app.sending_datetime", "app.confirmation_datetime"),
		baseWhere: "app.sending_datetime IS NOT NULL AND app.confirmation_datetime IS NOT NULL AND app.confirmation_datetime >= app.sending_datetime",
	},
	{
		key:       "acceptance_time",
		label:     "время принятия в работу",
		labelGen:  "времени принятия в работу",
		expr:      bureauWorkingDuration("app.confirmation_datetime", "app.accepted_at"),
		baseWhere: "app.confirmation_datetime IS NOT NULL AND app.accepted_at IS NOT NULL AND app.accepted_at >= app.confirmation_datetime",
	},
	{
		key:       "processing_time",
		label:     "время обработки",
		labelGen:  "времени обработки",
		expr:      bureauWorkingDuration("app.sending_datetime", "app.accepted_at"),
		baseWhere: "app.sending_datetime IS NOT NULL AND app.accepted_at IS NOT NULL AND app.accepted_at >= app.sending_datetime",
	},
	{
		key:      "completion_time",
		label:    "время до завершения",
		labelGen: "времени до завершения",
		// КАЛЕНДАРНОЕ, а не рабочее (#1251 S2): срок действия пропуска идёт реальными
		// сутками, Бюро в его течении не участвует. durationBetween, не bureauWorkingDuration.
		expr: durationBetween("app.sending_datetime", "app.completed_at"),
		// completed_at пишется только живым заявкам (белый список статусов, B1), а
		// у завершённых до появления колонки он NULL - на исторических данных
		// метрика разрежена. Это честнее нуля: момента завершения там не было.
		baseWhere: "app.sending_datetime IS NOT NULL AND app.completed_at IS NOT NULL AND app.completed_at >= app.sending_datetime",
	},
}

// durationRound приводит агрегат длительности к bigint-секундам. COALESCE - гард
// пустой выборки: AVG/PERCENTILE_CONT по нулю строк дают NULL, а NULL в int64 не
// сканируется. Пустой агрегат -> 0 («нет данных» рисует фронт).
func durationRound(agg string) string {
	return "COALESCE(ROUND(" + agg + ")::numeric, 0)::bigint"
}

// durationPercentile - перцентиль длительности. fraction подставляется только из
// констант kind'ов ниже (не из ввода). Явный ::double precision: EXTRACT даёт
// numeric, а percentile_cont разрешается по double precision.
func durationPercentile(expr, fraction string) string {
	return "PERCENTILE_CONT(" + fraction + ") WITHIN GROUP (ORDER BY (" + expr + ")::double precision)"
}

// durationAggKind - агрегат над длительностью этапа: префикс ключа, подпись и
// SQL-выражение.
type durationAggKind struct {
	prefix string
	label  func(s durationStage) string
	agg    func(expr string) string
}

var durationAggKinds = []durationAggKind{
	{
		prefix: "avg",
		label:  func(s durationStage) string { return "Среднее " + s.label },
		agg:    func(expr string) string { return durationRound("AVG(" + expr + ")") },
	},
	{
		prefix: "p50",
		label:  func(s durationStage) string { return "Медиана " + s.labelGen },
		agg:    func(expr string) string { return durationRound(durationPercentile(expr, "0.5")) },
	},
	{
		prefix: "p90",
		label:  func(s durationStage) string { return "90-й перцентиль " + s.labelGen },
		agg:    func(expr string) string { return durationRound(durationPercentile(expr, "0.9")) },
	},
}

// durationDimensions - разрезы длительностей (каталог). Только поля самой заявки
// и 1:1 join'ы. Типа вложения здесь намеренно НЕТ: attachments это 1:N, и join
// размножил бы строку заявки по числу вложений - заявка с пятью вложениями
// весила бы в среднем/перцентиле впятеро больше одиночной. Разрез по типу
// требует дедупа пар (заявка, тип) подзапросом - отдельная задача, а не молча
// взвешенное среднее. По той же причине его нет и в durationFilters.
var durationDimensions = []string{"status", "organization", "company", "period"}

// durationJoinBlock - 1:1 join'ы для разрезов/фильтров по организации и компании
// (строки заявок не размножают).
func durationJoinBlock() []string {
	return []string{
		"LEFT JOIN organizations org ON org.id = app.organization_id",
		"LEFT JOIN companies comp ON comp.id = app.company_id",
	}
}

func durationDims() map[string]aggColumn {
	return map[string]aggColumn{
		"status":       {expr: "app.status", join: jNone},
		"organization": {expr: "COALESCE(org.name, '(без организации)')", join: jChain},
		"company":      {expr: "COALESCE(comp.name, '(без компании)')", join: jChain},
	}
}

func durationFilters() map[string]aggColumn {
	return map[string]aggColumn{
		"status":       {expr: "app.status", join: jNone},
		"organization": {expr: "org.name", join: jChain},
		"company":      {expr: "comp.name", join: jChain},
	}
}

// init регистрирует метрики длительностей в движке (aggMetricRegistry) и каталоге
// (reportMetricRegistry + reportMetricOrder). Генерация вместо двенадцати
// рукописных блоков на реестр: выражение этапа задано ОДИН раз, поэтому avg/p50/p90
// одного этапа не могут разъехаться между собой, а движок с каталогом - друг с
// другом (тесты синхрона реестров идут по reportMetricOrder и это видят).
// Порядок детерминирован: этапы x агрегаты обходятся слайсами, не картами.
func init() {
	for _, s := range durationStages {
		for _, k := range durationAggKinds {
			key := k.prefix + "_" + s.key
			aggExpr := k.agg(s.expr)

			aggMetricRegistry[key] = aggMetricSchema{
				base:      "applications app",
				aggExpr:   aggExpr,
				baseWhere: s.baseWhere,
				// Окно периода и разрез period - по дате ПОДАЧИ: метрики этапов
				// читаются как «по заявкам, поданным за период», а не «по событиям
				// этапа за период» (иначе бины разных метрик несопоставимы).
				tsColumn:  "app.sending_datetime",
				tsJoin:    jNone,
				unit:      "", // значение форматируется как длительность, суффикс единицы не нужен
				valueType: models.ReportValueDuration,
				joinBlock: durationJoinBlock(),
				dims:      durationDims(),
				filters:   durationFilters(),
			}

			reportMetricRegistry[key] = metricDef{
				label:      k.label(s),
				unit:       "",
				group:      metricGroupProcessing,
				baseTable:  "applications",
				aggExpr:    aggExpr,
				baseFilter: s.baseWhere,
				dimensions: durationDimensions,
			}
			reportMetricOrder = append(reportMetricOrder, key)
		}
	}
}
