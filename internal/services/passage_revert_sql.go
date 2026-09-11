package services

import (
	"fmt"

	"systemburo/internal/models"
)

// Отмена ошибочной отметки прохода (#2437) не удаляет и не переписывает запись
// журнала: рядом с ней ложится отдельная запись, чей details.reverts_id указывает на
// аннулированную отметку. Читателям нужен готовый признак, иначе каждый собирал бы
// его своим подзапросом и разошёлся бы с соседом.
//
// Признак подмешивается в carsHistoryUnion и employeesHistoryUnion, то есть в
// единственную точку, через которую историю берут все: журнал таблицы проходной,
// карточка, статистика, конструктор отчётов, суточный отчёт. Дальше читатели
// расходятся: цифры отменённую отметку не считают (passageNotReverted), показ
// оставляет её видимой с пометкой.
const (
	// passageRevertJoin - самоджойн журнала на записи отмены. Алиас источника
	// обязан быть `a`, как в обоих union-запросах. Опирается на частичный уникальный
	// индекс idx_audit_passage_revert (database.createPassageRevertIndex): без него
	// join читает весь журнал, а без уникальности вторая отмена той же отметки
	// размножила бы строку истории.
	passageRevertJoin = `
	LEFT JOIN audit_log rv
		ON rv.action IN ('` + models.AuditActionEntryRevert + `', '` + models.AuditActionExitRevert + `')
		AND rv.details->>'reverts_id' = a.id::text`

	// passageRevertColumn - сам признак. Сравнение по тексту, а не приведением
	// details.reverts_id к int: журнал переживает любые данные, и каст свалил бы
	// чтение истории целиком, встретив одну нечисловую строку.
	passageRevertColumn = `rv.id IS NOT NULL AS reverted`
)

// passageNotReverted - условие «отметка в силе» для читателей, считающих проходы.
// Отдельной функцией, а не литералом по месту: мест больше десятка (статистика,
// суточный отчёт, обе группы конструктора отчётов, подзапросы последнего выхода), и
// пропуск любого из них означает, что раздел показывает свою цифру, расходящуюся с
// соседним разделом на те же данные.
func passageNotReverted(alias string) string {
	return fmt.Sprintf("NOT %s.reverted", alias)
}

// passageRevertWindowSQL отдаёт окно охранника в виде интервала для SQL: признак
// «отменить можно» считается в запросе текущего статуса, и повторять там число
// пятнадцать нельзя - разъедется с проверкой в самом механизме отмены.
func passageRevertWindowSQL() string {
	return fmt.Sprintf("%d minutes", int(passageRevertWindow.Minutes()))
}

// passageRevertNotExists - то же условие для читателей, идущих в audit_log напрямую,
// мимо union. Такой ровно один: суточный отчёт считает машины и людей одним
// COUNT(*) FILTER по entity_type, а union-проекции этот столбец стирают.
func passageRevertNotExists(alias string) string {
	return fmt.Sprintf(
		`NOT EXISTS (SELECT 1 FROM audit_log rv WHERE rv.action IN ('%s', '%s') `+
			`AND rv.details->>'reverts_id' = %s.id::text)`,
		models.AuditActionEntryRevert, models.AuditActionExitRevert, alias)
}
