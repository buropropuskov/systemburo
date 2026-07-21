package services

import "systemburo/internal/models"

// Метрики согласующих (#1240, B3): как быстро согласующий отвечает и сколько
// заявок через него проходит.
//
//	avg_approver_response_time — от назначения согласующим до его голоса.
//	approver_votes_count       — нагрузка: сколько голосов на согласующем.
//
// База — application_responsible_users (aru): одна строка = ОДИН голос одного
// согласующего по одной заявке. Поэтому разрез by_approver здесь безопасен —
// строка уже per-согласующий, join ничего не размножает. Метрикам с базой
// applications этот разрез принципиально не даётся: там 1 заявка : N
// согласующих, и join размножил бы заявку, взвесив среднее по числу голосов (та
// же причина, по которой длительностям не дан attachment_type).
//
// Окно периода и разрез period — по дате ПОДАЧИ заявки (app.sending_datetime),
// как у остальных метрик обработки: тогда плитки curated-вкладки (B4) считаются
// по одному набору заявок и сопоставимы между собой.

// approverNameExpr — подпись согласующего: ФИО, а при пустых частях — логин
// (пользователь без заполненного ФИО не должен схлопываться в пустую группу).
const approverNameExpr = "COALESCE(NULLIF(TRIM(BOTH ' ' FROM CONCAT_WS(' ', u.last_name, u.first_name, u.middle_name)), ''), u.username)"

// approverJoinBlock — 1:1 join'ы голоса: заявка (окно и разрезы заявки),
// пользователь (подпись), организация/компания. Ни один не размножает голоса.
// applications — INNER JOIN: голос без заявки невозможен (FK).
func approverJoinBlock() []string {
	return []string{
		"JOIN applications app ON app.id = aru.application_id",
		"LEFT JOIN users u ON u.id = aru.user_id",
		"LEFT JOIN organizations org ON org.id = app.organization_id",
		"LEFT JOIN companies comp ON comp.id = app.company_id",
	}
}

// approverDims/approverFilters — всё за пределами aru лежит за join-блоком
// (jChain), поэтому без разреза и фильтров план сводится к одной таблице голосов.
func approverDims() map[string]aggColumn {
	return map[string]aggColumn{
		dimByApprover:  {expr: approverNameExpr, join: jChain},
		"status":       {expr: "app.status", join: jChain},
		"organization": {expr: "COALESCE(org.name, '(без организации)')", join: jChain},
		"company":      {expr: "COALESCE(comp.name, '(без компании)')", join: jChain},
	}
}

func approverFilters() map[string]aggColumn {
	return map[string]aggColumn{
		"status":       {expr: "app.status", join: jChain},
		"organization": {expr: "org.name", join: jChain},
		"company":      {expr: "comp.name", join: jChain},
	}
}

var approverDimensions = []string{dimByApprover, "status", "organization", "company", "period"}

// init регистрирует метрики согласующих в движке и каталоге (как и остальные
// метрики #1240 — выражение агрегата считается один раз на оба реестра).
func init() {
	// Время реакции: от назначения согласующим (aru.created_at) до его голоса
	// (aru.approval_datetime), по РАБОЧЕМУ времени Бюро (#1251 S2): ночь и выходные
	// вычитаются, иначе «согласующему на ответ дали двое суток» на заявке, поданной в
	// пятницу вечером, — несправедливо к человеку (нет графика ИЛИ событие вне его
	// рабочих часов -> календарный фолбэк, см. bureauWorkingDuration). Голоса без
	// ответа в среднее не попадают —
	// иначе «ещё думает» считалось бы мгновенным ответом и занижало метрику. Гард
	// approval_datetime >= created_at отсекает битые пары (голос раньше назначения
	// на исторических данных) — иначе отрицательная длительность утянула бы среднее
	// в минус, как у этапов заявки (см. durationStage.baseWhere).
	responseAgg := durationRound("AVG(" + bureauWorkingDuration("aru.created_at", "aru.approval_datetime") + ")")
	aggMetricRegistry["avg_approver_response_time"] = aggMetricSchema{
		base:      "application_responsible_users aru",
		aggExpr:   responseAgg,
		baseWhere: "aru.approval_datetime IS NOT NULL AND aru.approval_datetime >= aru.created_at",
		tsColumn:  "app.sending_datetime",
		tsJoin:    jChain,
		unit:      "", // длительность форматируется целиком, суффикс единицы не нужен
		valueType: models.ReportValueDuration,
		joinBlock: approverJoinBlock(),
		dims:      approverDims(),
		filters:   approverFilters(),
	}
	reportMetricRegistry["avg_approver_response_time"] = metricDef{
		label:      "Среднее время реакции согласующего",
		unit:       "",
		group:      metricGroupApprovers,
		baseTable:  "application_responsible_users",
		aggExpr:    responseAgg,
		baseFilter: "approval_datetime IS NOT NULL AND approval_datetime >= created_at",
		dimensions: approverDimensions,
	}
	reportMetricOrder = append(reportMetricOrder, "avg_approver_response_time")

	// Нагрузка: считаем ВСЕ назначения, включая ещё не отданные голоса — нагрузка
	// это сколько заявок на согласующего завели, а не сколько он успел разобрать.
	aggMetricRegistry["approver_votes_count"] = aggMetricSchema{
		base:      "application_responsible_users aru",
		aggExpr:   "COUNT(*)",
		tsColumn:  "app.sending_datetime",
		tsJoin:    jChain,
		unit:      "шт",
		joinBlock: approverJoinBlock(),
		dims:      approverDims(),
		filters:   approverFilters(),
	}
	reportMetricRegistry["approver_votes_count"] = metricDef{
		label:      "Нагрузка согласующего",
		unit:       "шт",
		group:      metricGroupApprovers,
		baseTable:  "application_responsible_users",
		aggExpr:    "COUNT(*)",
		dimensions: approverDimensions,
	}
	reportMetricOrder = append(reportMetricOrder, "approver_votes_count")
}
