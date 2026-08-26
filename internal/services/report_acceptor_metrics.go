package services

import "systemburo/internal/models"

// Метрики принимающих (#1251 S3): как быстро принимающий забирает заявку в работу
// после согласования и сколько заявок он принимает.
//
//	avg_acceptor_response_time — рабочее время от согласования (confirmation_datetime)
//	                             до первого принятия в работу (accepted_at).
//	acceptor_accepts_count     — нагрузка: сколько заявок принял.
//
// «Принимающий» = актор ПЕРВОГО действия take_to_work по заявке (audit_log). База —
// подзапрос acc: DISTINCT ON (заявка) первое принятие, поэтому строка = одна заявка
// на её принимающего, разрез by_acceptor ничего не размножает (как aru у
// согласующих). accepted_at пишется COALESCE'ом (первое принятие,
// application_workflow_service.go), поэтому пара (первый актор, accepted_at)
// согласована: длительность считается до того же момента, что и app-level
// acceptance_time. Время реакции — по рабочему времени Бюро (#1251 S2,
// bureauWorkingDuration): ночь и выходные вычитаются.
//
// Окно периода и разрез period — по дате ПОДАЧИ заявки (app.sending_datetime), как у
// остальных метрик обработки, чтобы плитки вкладки считались по одному набору
// заявок и были сопоставимы.

// acceptorNameExpr — подпись принимающего: ФИО, а при пустых частях — логин.
const acceptorNameExpr = "COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name, u.middle_name)), ''), u.username)"

// acceptorBase — первое принятие каждой заявки из audit_log. DISTINCT ON по заявке
// с сортировкой по времени берёт самое раннее take_to_work; actor_user_id — кто
// принял. entity_type/action — через константы (как carsHistoryUnion), чтобы литерал
// не разъехался с писателем. Подставляется движком как FROM (...) acc (алиас снаружи).
const acceptorBase = `(SELECT DISTINCT ON (al.entity_id)
		al.entity_id AS application_id,
		al.actor_user_id AS acceptor_user_id
	FROM audit_log al
	WHERE al.entity_type = '` + models.AuditEntityApplication + `'
	  AND al.action = '` + models.AuditActionTakeToWork + `'
	  AND al.actor_user_id IS NOT NULL
	ORDER BY al.entity_id, al.created_at ASC)`

// acceptorJoinBlock — 1:1 join'ы принятия: заявка (окно, разрезы, моменты
// согласования и принятия), пользователь (подпись), организация/компания. Ни один
// не размножает строки. applications — INNER JOIN: у записи о принятии заявка есть.
func acceptorJoinBlock() []string {
	return []string{
		"JOIN applications app ON app.id = acc.application_id",
		"LEFT JOIN users u ON u.id = acc.acceptor_user_id",
		"LEFT JOIN organizations org ON org.id = app.organization_id",
		"LEFT JOIN companies comp ON comp.id = app.company_id",
	}
}

// acceptorDims/acceptorFilters — всё за пределами acc лежит за join-блоком (jChain).
func acceptorDims() map[string]aggColumn {
	return map[string]aggColumn{
		dimByAcceptor:  {expr: acceptorNameExpr, join: jChain},
		"status":       {expr: "app.status", join: jChain},
		"organization": {expr: "COALESCE(org.name, '(без организации)')", join: jChain},
		"company":      {expr: "COALESCE(comp.name, '(без компании)')", join: jChain},
	}
}

func acceptorFilters() map[string]aggColumn {
	return map[string]aggColumn{
		"status":       {expr: "app.status", join: jChain},
		"organization": {expr: "org.name", join: jChain},
		"company":      {expr: "comp.name", join: jChain},
	}
}

var acceptorDimensions = []string{dimByAcceptor, "status", "organization", "company", "period"}

// init регистрирует метрики принимающих в движке и каталоге (как и остальные
// метрики обработки — выражение агрегата считается один раз на оба реестра).
func init() {
	// Время принятия: рабочее время от согласования (app.confirmation_datetime) до
	// первого принятия (app.accepted_at). Гард accepted_at >= confirmation_datetime
	// отсекает битые пары (принятие раньше согласования на исторических данных) —
	// иначе отрицательная длительность утянула бы среднее в минус, как у этапов
	// заявки (см. durationStage.baseWhere).
	responseAgg := durationRound("AVG(" + bureauWorkingDuration("app.confirmation_datetime", "app.accepted_at") + ")")
	aggMetricRegistry["avg_acceptor_response_time"] = aggMetricSchema{
		base:      acceptorBase + " acc",
		aggExpr:   responseAgg,
		baseWhere: "app.confirmation_datetime IS NOT NULL AND app.accepted_at IS NOT NULL AND app.accepted_at >= app.confirmation_datetime",
		tsColumn:  "app.sending_datetime",
		tsJoin:    jChain,
		unit:      "", // длительность форматируется целиком, суффикс единицы не нужен
		valueType: models.ReportValueDuration,
		joinBlock: acceptorJoinBlock(),
		dims:      acceptorDims(),
		filters:   acceptorFilters(),
	}
	reportMetricRegistry["avg_acceptor_response_time"] = metricDef{
		// «реакции принимающего», а не «время принятия»: последнее уже занято метрикой
		// этапа avg_acceptance_time (группа «Обработка заявок»); здесь разрез по
		// принимающему, симметрично «время реакции согласующего».
		label:      "Среднее время реакции принимающего",
		unit:       "",
		group:      metricGroupAcceptors,
		baseTable:  "applications", // подпись в гиде; реальный источник — подзапрос acc
		aggExpr:    responseAgg,
		baseFilter: "confirmation_datetime IS NOT NULL AND accepted_at IS NOT NULL AND accepted_at >= confirmation_datetime",
		dimensions: acceptorDimensions,
	}
	reportMetricOrder = append(reportMetricOrder, "avg_acceptor_response_time")

	// Нагрузка: считаем ВСЕ принятия за период (acc — одна строка на заявку), без
	// гарда длительности: нагрузка это сколько заявок принял, а не только валидных.
	aggMetricRegistry["acceptor_accepts_count"] = aggMetricSchema{
		base:      acceptorBase + " acc",
		aggExpr:   "COUNT(*)",
		tsColumn:  "app.sending_datetime",
		tsJoin:    jChain,
		unit:      "шт",
		joinBlock: acceptorJoinBlock(),
		dims:      acceptorDims(),
		filters:   acceptorFilters(),
	}
	reportMetricRegistry["acceptor_accepts_count"] = metricDef{
		label:      "Нагрузка принимающего",
		unit:       "шт",
		group:      metricGroupAcceptors,
		baseTable:  "applications",
		aggExpr:    "COUNT(*)",
		dimensions: acceptorDimensions,
	}
	reportMetricOrder = append(reportMetricOrder, "acceptor_accepts_count")
}
