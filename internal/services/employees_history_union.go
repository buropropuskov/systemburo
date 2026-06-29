package services

import (
	"systemburo/internal/models"
)

// employeesHistoryUnion - подзапрос-мост (#870, срез 1.13a): отдаёт строки
// истории сотрудников в форме таблицы employees_history, объединяя замороженную
// employees_history и новые записи из audit_log[employee]. employees_history -
// не фид одной модалки, а общий журнал событий: его читают история сотрудника
// (/:id/history, /unified, /all, /table/:id), текущий статус (last_exit),
// корзина (скоуп по action='delete'+table_id), статистика и конструктор отчётов.
// Поэтому union подставляется ВО ВСЕ читатели - чтобы после переноса записи на
// recorder (срез 1.13b) ни один потребитель не терял новые события.
//
// Плоские поля старой схемы (field_name/old_value/new_value/comment/table_id) у
// audit_log лежат внутри details jsonb, metadata - вложенным объектом
// details->'metadata'. Колонки результата совпадают с employees_history:
// id, employee_id, user_id, action_type, field_name, old_value, new_value,
// comment, metadata(jsonb), table_id, created_at.
//
// Подставлять как `FROM ` + employeesHistoryUnion + ` <alias>` вместо
// `FROM employees_history <alias>`.
const employeesHistoryUnion = `(
	SELECT id, employee_id, user_id, action_type, field_name, old_value, new_value,
		comment, metadata, table_id, created_at
	FROM employees_history
	UNION ALL
	SELECT a.id, a.entity_id AS employee_id, a.actor_user_id AS user_id,
		a.action AS action_type,
		a.details->>'field_name' AS field_name,
		a.details->>'old_value' AS old_value,
		a.details->>'new_value' AS new_value,
		a.details->>'comment' AS comment,
		a.details->'metadata' AS metadata,
		(a.details->>'table_id')::int AS table_id,
		a.created_at
	FROM audit_log a
	WHERE a.entity_type = '` + models.AuditEntityEmployee + `'
)`
