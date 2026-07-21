package services

import "fmt"

/*
Единственное определение «архивной заявки» на весь сервис.

Заявка уходит в архив, когда выполняются оба условия:
  - статус закрытый (models.ArchivableStatuses);
  - у ВСЕХ её вложений срок действия истёк больше месяца назад, т.е. месяц
    отсчитывается от ПОСЛЕДНЕГО заканчивающегося вложения.

Вложение без даты окончания (NULL или пустая строка) сроком не ограничено -
считать его истёкшим нельзя, поэтому такая заявка в архив не уходит. По той же
причине заявка совсем без вложений остаётся активной: архивировать нечего.

Предикат обязаны использовать все потребители - листинг Центра, счётчик
непрочитанных и гейт «архивная заявка только для чтения». Разъехавшиеся копии
дают заявку, которая в списке активна, но недоступна на запись (и наоборот).
*/

// archivedApplicationCond возвращает SQL-условие «заявка архивная» для указанного
// алиаса таблицы applications. Условие содержит ОДИН плейсхолдер - список
// закрытых статусов (models.ArchivableStatuses).
//
// COALESCE по статусу обязателен: status в БД nullable, а `NULL IN (...)` даёт NULL,
// и заявка не прошла бы ни `WHERE cond` (архив), ни `WHERE NOT cond` (активные) -
// пропала бы из обоих списков. Тот же приём рядом в application_workflow_service.go.
func archivedApplicationCond(alias string) string {
	return fmt.Sprintf(`(
		COALESCE(%[1]s.status, '') IN ?
		AND EXISTS (
			SELECT 1 FROM attachments att
			WHERE att.application_id = %[1]s.id
			AND NULLIF(att.entry_date_to, '') IS NOT NULL
		)
		AND NOT EXISTS (
			SELECT 1 FROM attachments att
			WHERE att.application_id = %[1]s.id
			AND (
				NULLIF(att.entry_date_to, '') IS NULL
				OR CAST(att.entry_date_to AS DATE) + INTERVAL '1 month' >= NOW()
			)
		)
	)`, alias)
}

// activeApplicationCond - отрицание archivedApplicationCond (заявка не в архиве).
func activeApplicationCond(alias string) string {
	return "NOT " + archivedApplicationCond(alias)
}
