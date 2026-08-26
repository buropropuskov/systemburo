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

// applicationInactiveSinceExpr - SQL-выражение «момент, с которого заявка перестала
// действовать», для указанного алиаса таблицы applications. NULL означает «ещё
// действует»: у заявки нет ни одного истёкшего срока либо есть бессрочное вложение.
//
// Это тот же расчёт, по которому заявка уходит в архив, вынесенный отдельно: от него
// же отсчитывается заморозка файлов в файловом архиве (#1615), а срок заморозки
// настраивается. Две копии семантики «заявка закончилась» разъехались бы на первой
// правке - и тогда заявка, активная в Центре, замораживала бы свои бланки.
//
// MAX по вложениям возвращает NULL и когда вложений нет вовсе, и когда все они без
// даты окончания, поэтому отдельный EXISTS не нужен: бессрочное вложение гасит
// расчёт через NOT EXISTS, а пустая выборка даёт NULL сама.
func applicationInactiveSinceExpr(alias string) (string, []any) {
	expr := fmt.Sprintf(`(CASE
		WHEN COALESCE(%[1]s.status, '') = ? AND %[1]s.withdrawn_at IS NOT NULL THEN %[1]s.withdrawn_at
		ELSE (
			SELECT MAX(CAST(att.entry_date_to AS DATE))::timestamptz
			FROM attachments att
			WHERE att.application_id = %[1]s.id
			AND NULLIF(att.entry_date_to, '') IS NOT NULL
			AND NOT EXISTS (
				SELECT 1 FROM attachments live
				WHERE live.application_id = %[1]s.id
				AND NULLIF(live.entry_date_to, '') IS NULL
			)
		)
	END)`, alias)
	return expr, []any{models.StatusWithdrawn}
}

// archivedApplicationCond возвращает SQL-условие «заявка архивная» для указанного
// алиаса таблицы applications вместе с аргументами плейсхолдеров.
//
// COALESCE по статусу обязателен: status в БД nullable, а `NULL IN (...)` даёт NULL,
// и заявка не прошла бы ни `WHERE cond` (архив), ни `WHERE NOT cond` (активные) -
// пропала бы из обоих списков. Тот же приём рядом в application_workflow_service.go.
// По той же причине сравнение с NOW() завёрнуто в COALESCE: у ещё действующей заявки
// момент окончания NULL, и без него всё условие стало бы NULL вместо FALSE.
func archivedApplicationCond(alias string) (string, []any) {
	since, sinceArgs := applicationInactiveSinceExpr(alias)
	cond := fmt.Sprintf(`(
		COALESCE(%[1]s.status, '') IN ?
		AND COALESCE(%[2]s + INTERVAL '1 month' < NOW(), FALSE)
	)`, alias, since)
	return cond, append([]any{models.ArchivableStatuses}, sinceArgs...)
}

// activeApplicationCond - отрицание archivedApplicationCond (заявка не в архиве).
func activeApplicationCond(alias string) (string, []any) {
	cond, args := archivedApplicationCond(alias)
	return "NOT " + cond, args
}
