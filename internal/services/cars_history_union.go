package services

import (
	"encoding/json"

	"systemburo/internal/models"
)

// carAuditDetails - форма details jsonb для записей car в audit_log (#870, срез
// 1.12c). Ключи ОБЯЗАНЫ совпадать с тем, что извлекает carsHistoryUnion
// (field_name/old_value/new_value/comment/table_id/metadata), иначе читатель
// потеряет поля. omitempty + указатели дают семантику старой nullable cars_history:
// nil -> ключ опущен -> details->>'key' = NULL (как незаполненная колонка); &"" ->
// пустая строка (сохраняет не-nil Comment старого write-path). metadata пишется
// вложенным объектом, чтобы details->'metadata' вернул тот же jsonb, что давала
// колонка cars_history.metadata.
type carAuditDetails struct {
	FieldName *string `json:"field_name,omitempty"`
	OldValue  *string `json:"old_value,omitempty"`
	NewValue  *string `json:"new_value,omitempty"`
	Comment   *string `json:"comment,omitempty"`
	// Subject -- к чему относится событие: ФИО работника либо номер и марка машины на
	// момент действия. Снимок, а не ссылка: строку могут удалить, и тогда по entity_id
	// уже не узнать, о ком речь, а в журнале реестра именно это и нужно прочитать.
	Subject  *string         `json:"subject,omitempty"`
	TableID  *int            `json:"table_id,omitempty"`
	Metadata json.RawMessage `json:"metadata,omitempty"`
}

// carsHistoryUnion - подзапрос-источник (#870, срез 1.12 -> read-switch F.5):
// отдаёт строки истории машин в форме таблицы cars_history. cars_history - не фид
// одной модалки, а общий журнал событий: его читают история машины, корзина (скоуп
// по action='delete'+table_id), текущий статус (last_exit), статистика и конструктор
// отчётов. Поэтому единый источник подставляется ВО ВСЕ читатели разом.
//
// До F.5 это был UNION замороженной cars_history и новых записей audit_log[car].
// После F.5 до-cutover строки cars_history разово перенесены в audit_log
// (BackfillAuditFromLegacy, форма carAuditDetails), поэтому читаем ТОЛЬКО audit_log[car]
// - он уже содержит и исторические, и новые события. cars_history дропнута в
// дроп-sweep (F.8). Имя с "Union" сохранено: подзапрос по-прежнему
// единственная точка, через которую все 5 читателей берут историю машин.
//
// Плоские поля старой схемы (field_name/old_value/new_value/comment/table_id) у
// audit_log лежат внутри details jsonb, metadata - вложенным объектом
// details->'metadata'. Колонки результата совпадают с cars_history:
// id, car_id, user_id, action_type, field_name, old_value, new_value, comment,
// metadata(jsonb), table_id, created_at.
//
// Подставлять как `FROM ` + carsHistoryUnion + ` <alias>` вместо `FROM cars_history <alias>`.
const carsHistoryUnion = `(
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
