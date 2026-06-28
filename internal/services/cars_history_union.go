package services

import "systemburo/internal/models"

// carsHistoryUnion - подзапрос-мост (#870, срез 1.12): отдаёт строки истории
// машин в форме таблицы cars_history, объединяя замороженную cars_history и
// новые записи из audit_log[car]. cars_history - не фид одной модалки, а общий
// журнал событий: его читают история машины, корзина (скоуп по action='delete'+
// table_id), текущий статус (last_exit), статистика и конструктор отчётов.
// Поэтому union подставляется ВО ВСЕ читатели - чтобы после переноса записи на
// recorder (срез 1.12c) ни один потребитель не терял новые события.
//
// Плоские поля старой схемы (field_name/old_value/new_value/comment/table_id) у
// audit_log лежат внутри details jsonb, metadata - вложенным объектом
// details->'metadata'. Колонки результата совпадают с cars_history:
// id, car_id, user_id, action_type, field_name, old_value, new_value, comment,
// metadata(jsonb), table_id, created_at.
//
// Подставлять как `FROM ` + carsHistoryUnion + ` <alias>` вместо `FROM cars_history <alias>`.
const carsHistoryUnion = `(
	SELECT id, car_id, user_id, action_type, field_name, old_value, new_value,
		comment, metadata, table_id, created_at
	FROM cars_history
	UNION ALL
	SELECT a.id, a.entity_id AS car_id, a.actor_user_id AS user_id,
		a.action AS action_type,
		a.details->>'field_name' AS field_name,
		a.details->>'old_value' AS old_value,
		a.details->>'new_value' AS new_value,
		a.details->>'comment' AS comment,
		a.details->'metadata' AS metadata,
		(a.details->>'table_id')::int AS table_id,
		a.created_at
	FROM audit_log a
	WHERE a.entity_type = '` + models.AuditEntityCar + `'
)`
