package services

import (
	"strings"

	"systemburo/internal/models"
)

// Движок исполнения list-отчётов (B2b): выгрузка строк сущности с фиксированным
// whitelist столбцов и фильтров. Как и агрегатный движок (report_engine.go), все
// SQL-выражения здесь — статические константы кода; пользовательский ввод сверяется
// по ключам реестров и передаётся только через ? плейсхолдеры. Подписи и порядок
// столбцов берутся из каталога B1 (reportListEntityRegistry) — единый источник
// правды для UI и движка; тест ловит расхождение ключей каталога и схем исполнения.

// workAttachmentDisplayName — отображаемое имя типа вложения «Заявка на работы».
// Сущность work_applications = вложения этого типа. Значение data-driven (имена
// типов задаются оператором в справочнике unique_attachments); при иной
// формулировке на проде — поправить здесь.
const workAttachmentDisplayName = "Заявка на работы"

// workNameFieldPattern — ILIKE-шаблон подписи кастомного поля «Наименование работ».
// «Наименование работ» — это кастомное поле типа вложения (attachment_custom_fields),
// его значение хранится в attachment_custom_values. Подбираем поле по подписи, т.к.
// id поля зависит от данных конкретного стенда. Передаётся в SQL как аргумент-плейсхолдер.
const workNameFieldPattern = "наименование работ%"

// colExprDef — SQL-выражение одного столбца list-режима (алиасится AS <key>).
// args заполняется только для выражений с ? плейсхолдером (передаются в Select).
type colExprDef struct {
	expr string
	args []any
}

// listExecSchema — план сборки list-запроса для одной сущности. joins применяются
// всегда (1:1 LEFT JOIN — ограничены лимитом выборки; агрегаты по дочерним строкам
// считаются скоррелированными подзапросами, чтобы не плодить fan-out).
type listExecSchema struct {
	base         string
	baseWhere    whereClause           // безопасная константа (+ опц. аргумент)
	joins        []string              // всегда добавляются
	tsColumn     string                // колонка для фильтра date_range ("" — фильтр недоступен)
	colExpr      map[string]colExprDef // ключ столбца -> SQL-выражение
	filterExpr   map[string]string     // ключ фильтра -> SQL-выражение (для IN)
	defaultOrder string
}

var listExecRegistry = map[string]listExecSchema{
	"work_applications": {
		base:      "attachments att",
		baseWhere: whereClause{expr: "ua.display_name = ?", args: []any{workAttachmentDisplayName}},
		joins: []string{
			"JOIN applications app ON app.id = att.application_id",
			"LEFT JOIN unique_attachments ua ON ua.id = att.unique_attachment_id",
			"LEFT JOIN organizations org ON org.id = app.organization_id",
			"LEFT JOIN companies comp ON comp.id = app.company_id",
			"LEFT JOIN users ru ON ru.id = app.responsible_user_id",
		},
		tsColumn: "app.sending_datetime",
		colExpr: map[string]colExprDef{
			"number":         {expr: "COALESCE(app.application_number, '')"},
			"org_or_company": {expr: "COALESCE(NULLIF(org.name, ''), comp.name, '')"},
			"work_name": {
				expr: "COALESCE((SELECT acv.value FROM attachment_custom_values acv " +
					"JOIN attachment_custom_fields acf ON acf.id = acv.custom_field_id " +
					"WHERE acv.attachment_id = att.id AND acf.label ILIKE ? " +
					"ORDER BY acf.sort_order, acf.id LIMIT 1), '')",
				args: []any{workNameFieldPattern},
			},
			"responsible": {expr: "COALESCE(NULLIF(TRIM(CONCAT_WS(' ', ru.last_name, ru.first_name, ru.middle_name)), ''), '') " +
				"|| CASE WHEN COALESCE(ru.phone, '') <> '' THEN ', тел. ' || ru.phone ELSE '' END"},
			"work_period": {expr: "CASE WHEN COALESCE(att.entry_date_from, att.entry_date_to, '') = '' THEN '' " +
				"ELSE COALESCE(att.entry_date_from, '') || ' - ' || COALESCE(att.entry_date_to, '') END"},
			"work_time": {expr: "CASE WHEN COALESCE(att.entry_time_from, att.entry_time_to, '') = '' THEN '' " +
				"ELSE COALESCE(att.entry_time_from, '') || ' - ' || COALESCE(att.entry_time_to, '') END"},
			// Люди непринятого дополнения в счёт не идут (#1685): отчёт показывает, сколько
			// человек по заявке реально работает, а решения по добавке ещё нет.
			"people_count": {expr: "(SELECT COUNT(*) FROM employees emp WHERE emp.attachment_id = att.id AND emp.is_purged = false" +
				" AND " + admittedSupplementCond("emp") + ")"},
		},
		filterExpr: map[string]string{
			"organization": "org.name",
			"status":       "app.status",
		},
		defaultOrder: "att.id DESC",
	},
	"applications": {
		base: "applications app",
		joins: []string{
			"LEFT JOIN organizations org ON org.id = app.organization_id",
			"LEFT JOIN companies comp ON comp.id = app.company_id",
		},
		tsColumn: "app.sending_datetime",
		colExpr: map[string]colExprDef{
			"number":            {expr: "COALESCE(app.application_number, '')"},
			"status":            {expr: "COALESCE(app.status, '')"},
			"organization":      {expr: "COALESCE(org.name, '')"},
			"company":           {expr: "COALESCE(comp.name, '')"},
			"sending_datetime":  {expr: "COALESCE(to_char(app.sending_datetime, 'YYYY-MM-DD HH24:MI'), '')"},
			"attachments_count": {expr: "(SELECT COUNT(*) FROM attachments a2 WHERE a2.application_id = app.id)"},
		},
		filterExpr: map[string]string{
			"status":       "app.status",
			"organization": "org.name",
			"company":      "comp.name",
		},
		defaultOrder: "app.sending_datetime DESC NULLS LAST",
	},
	"cars": {
		base:      "cars c",
		baseWhere: whereClause{expr: "c.is_purged = false"},
		joins: []string{
			"LEFT JOIN attachments att ON att.id = c.attachment_id",
			"LEFT JOIN applications app ON app.id = att.application_id",
			"LEFT JOIN organizations org ON org.id = app.organization_id",
			"LEFT JOIN companies comp ON comp.id = app.company_id",
		},
		colExpr: map[string]colExprDef{
			"car_number":       {expr: "COALESCE(c.car_number, '')"},
			"mark":             {expr: "COALESCE(NULLIF(c.mark_name, ''), c.car_brand, '')"},
			"organization":     {expr: "COALESCE(org.name, comp.name, '')"},
			"place":            {expr: "COALESCE(c.unload_place, '')"},
			"territory_status": {expr: "CASE WHEN c.territory_status = 1 THEN 'На территории' ELSE 'Нет' END"},
		},
		filterExpr: map[string]string{
			"organization": "org.name",
			"unload_place": "c.unload_place",
		},
		defaultOrder: "c.id DESC",
	},
	"people": {
		base:      "employees e",
		baseWhere: whereClause{expr: "e.is_purged = false"},
		joins: []string{
			"LEFT JOIN citizenships cz ON cz.id = e.citizenship_id",
			"LEFT JOIN attachments att ON att.id = e.attachment_id",
			"LEFT JOIN applications app ON app.id = att.application_id",
			"LEFT JOIN organizations org ON org.id = app.organization_id",
			"LEFT JOIN companies comp ON comp.id = app.company_id",
		},
		colExpr: map[string]colExprDef{
			"full_name":    {expr: "TRIM(CONCAT_WS(' ', e.last_name, e.first_name, e.middle_name))"},
			"organization": {expr: "COALESCE(org.name, comp.name, '')"},
			"citizenship":  {expr: "COALESCE(cz.name, '')"},
			"place": {expr: "COALESCE((SELECT STRING_AGG(DISTINCT st.display_name, ', ' ORDER BY st.display_name) " +
				"FROM employee_target_tables ett JOIN system_tables st ON st.id = ett.table_id " +
				"WHERE ett.employee_id = e.id), '')"},
			"territory_status": {expr: "CASE WHEN e.territory_status = 1 THEN 'На территории' ELSE 'Нет' END"},
		},
		filterExpr: map[string]string{
			"organization": "org.name",
			"citizenship":  "cz.name",
		},
		defaultOrder: "e.id DESC",
	},
}

// listPlan — резолвленный план list-запроса (только whitelist-выражения).
// selectArgs — аргументы плейсхолдеров SELECT (в порядке появления столбцов).
type listPlan struct {
	table      string
	selectStr  string
	selectArgs []any
	joins      []string
	wheres     []whereClause
	orderStr   string
	limit      int
	columns    []models.ReportColumnInfo
}

// buildListPlan собирает план list-отчёта из whitelist-схем. Чистая функция (без
// БД) — тестируется напрямую. Порядок и подписи столбцов берутся из каталога B1,
// SQL-выражения — из listExecRegistry. Неизвестная сущность/фильтр или столбец
// каталога без выражения -> ErrInvalidReportRequest.
// responsibleMaskedExpr - колонка «принимающий», когда персональные данные скрыты
// до согласия: вместо ФИО и телефона показывается логин. Условие повторяет
// consentMasksWithState, но в SQL: строку собирает база, и подменить её после
// выборки нечем - идентификатора работника в выдаче отчёта нет.
//
// Скрываем только тех, кого запрос согласия реально касается (та же мерка, что у
// гейта и у gatedUsersWhere): супер-администратор проходит гейт всегда, архивных и
// заблокированных отбивают раньше, и согласия у них нет не потому, что они его не
// дали. Без этой оговорки отчёт обезличивал бы тех, кто во всей остальной системе
// показывается открыто.
const responsibleMaskedExpr = `CASE
	WHEN ru.id IS NULL THEN ''
	WHEN ru.is_super_admin OR NOT ru.is_active OR ru.is_banned OR EXISTS (
		SELECT 1 FROM pd_consents c
		WHERE c.user_id = ru.id AND c.consent_type = 'pd_processing'
		  AND c.granted = true AND c.revoked_at IS NULL
	) THEN COALESCE(NULLIF(TRIM(CONCAT_WS(' ', ru.last_name, ru.first_name, ru.middle_name)), ''), '')
		|| CASE WHEN COALESCE(ru.phone, '') <> '' THEN ', тел. ' || ru.phone ELSE '' END
	ELSE '@' || COALESCE(ru.username, '')
END`

func buildListPlan(req models.ReportRequest, maskPD bool) (*listPlan, error) {
	exec, ok := listExecRegistry[req.Entity]
	if !ok {
		return nil, errInvalidReport("entity")
	}
	catalog, ok := reportListEntityRegistry[req.Entity]
	if !ok {
		return nil, errInvalidReport("entity")
	}

	plan := &listPlan{
		table: exec.base,
		joins: append([]string{}, exec.joins...),
	}

	selects := make([]string, 0, len(catalog.columns))
	cols := make([]models.ReportColumnInfo, 0, len(catalog.columns))
	for _, c := range catalog.columns {
		def, ok := exec.colExpr[c.key]
		if !ok {
			// Каталог объявил столбец, для которого нет SQL-выражения — баг конфигурации.
			return nil, errInvalidReport("column")
		}
		expr := def.expr
		if maskPD && c.key == "responsible" {
			expr = responsibleMaskedExpr
		}
		selects = append(selects, expr+" AS "+c.key)
		plan.selectArgs = append(plan.selectArgs, def.args...)
		cols = append(cols, models.ReportColumnInfo{Key: c.key, Label: c.label, Type: c.format})
	}
	plan.selectStr = strings.Join(selects, ", ")
	plan.columns = cols

	if exec.baseWhere.expr != "" {
		plan.wheres = append(plan.wheres, exec.baseWhere)
	}

	allowed := make(map[string]bool, len(catalog.filters))
	for _, f := range catalog.filters {
		allowed[f] = true
	}
	for _, f := range req.Filters {
		if !allowed[f.Key] {
			return nil, errInvalidReport("filter")
		}
		wc, err := resolveListFilter(exec, f)
		if err != nil {
			return nil, err
		}
		if wc == nil { // пустой фильтр (нет значений/границ) — пропускаем
			continue
		}
		plan.wheres = append(plan.wheres, *wc)
	}

	plan.orderStr = exec.defaultOrder
	plan.limit = clampLimit(req.Limit)
	return plan, nil
}

// resolveListFilter превращает поле фильтра в WHERE-фрагмент. Возвращает nil без
// ошибки, если фильтр пустой (нет значений/границ). date_range применяется к
// tsColumn сущности; остальные — по выражению из filterExpr (IN со срезом-аргументом).
func resolveListFilter(exec listExecSchema, f models.ReportFilterValue) (*whereClause, error) {
	if f.Key == "date_range" {
		if exec.tsColumn == "" {
			return nil, errInvalidReport("filter")
		}
		var parts []string
		var args []any
		if t, ok := parseReportDate(f.From, false); ok {
			parts = append(parts, exec.tsColumn+" >= ?")
			args = append(args, t)
		}
		if t, ok := parseReportDate(f.To, true); ok {
			parts = append(parts, exec.tsColumn+" <= ?")
			args = append(args, t)
		}
		if len(parts) == 0 {
			return nil, nil
		}
		return &whereClause{expr: strings.Join(parts, " AND "), args: args}, nil
	}

	expr, ok := exec.filterExpr[f.Key]
	if !ok {
		return nil, errInvalidReport("filter")
	}
	vals := nonEmpty(f.Values)
	if len(vals) == 0 {
		return nil, nil
	}
	return &whereClause{expr: expr + " IN ?", args: []any{vals}}, nil
}
