package services

import (
	"fmt"

	"systemburo/internal/models"
)

/*
Единственное определение «архивной заявки» на весь сервис.

Заявка уходит в архив, когда её статус закрытый (models.ArchivableStatuses) и
прошёл месяц с момента, когда заявка перестала действовать:

  - отозванная (`Отозвана`) - месяц с момента отзыва (`withdrawn_at`): отзыв гасит
    вложения сразу, их сроки после этого ничего не значат. У отозванных до
    появления колонки `withdrawn_at` пусто - они архивируются по общему правилу;
  - остальные закрытые - месяц с окончания ПОСЛЕДНЕГО вложения, то есть пока
    действует хотя бы одно вложение, заявка остаётся активной.

Вложение без даты окончания сроком не ограничено - считать его истёкшим нельзя,
поэтому заявка с таким вложением в архив не уходит. По той же причине заявка
совсем без вложений остаётся активной: архивировать нечего.

Предикат обязаны использовать все потребители - листинг Центра, счётчик
непрочитанных и гейт «архивная заявка только для чтения». Разъехавшиеся копии
дают заявку, которая в списке активна, но недоступна на запись (и наоборот).
*/

// archivedApplicationCond возвращает SQL-условие «заявка архивная» для указанного
// алиаса таблицы applications вместе с аргументами плейсхолдеров.
//
// COALESCE по статусу обязателен: status в БД nullable, а `NULL IN (...)` даёт NULL,
// и заявка не прошла бы ни `WHERE cond` (архив), ни `WHERE NOT cond` (активные) -
// пропала бы из обоих списков. Тот же приём рядом в application_workflow_service.go.
func archivedApplicationCond(alias string) (string, []any) {
	cond := fmt.Sprintf(`(
		COALESCE(%[1]s.status, '') IN ?
		AND (
			(
				COALESCE(%[1]s.status, '') = ?
				AND %[1]s.withdrawn_at IS NOT NULL
				AND %[1]s.withdrawn_at + INTERVAL '1 month' < NOW()
			)
			OR (
				(COALESCE(%[1]s.status, '') <> ? OR %[1]s.withdrawn_at IS NULL)
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
			)
		)
	)`, alias)
	return cond, []any{models.ArchivableStatuses, models.StatusWithdrawn, models.StatusWithdrawn}
}

// activeApplicationCond - отрицание archivedApplicationCond (заявка не в архиве).
func activeApplicationCond(alias string) (string, []any) {
	cond, args := archivedApplicationCond(alias)
	return "NOT " + cond, args
}
