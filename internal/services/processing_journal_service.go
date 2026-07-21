package services

import (
	"context"
	"fmt"
	"time"

	"systemburo/internal/models"
)

// Сквозная лента событий обработки (#1251 S4): согласования (голос согласующего с
// проставленным approval_datetime) и принятия в работу (первое take_to_work,
// acceptorBase) одним UNION ALL, отсортированным по времени убыванием. Рабочая
// длительность события — bureauWorkingDuration (#1251 S2, с фолбэком на календарь
// при пустом графике Бюро). Кэша нет: лента «реального времени», глубину ограничивает
// лимит. Момент начала (назначение / согласование) может отсутствовать или стоять
// позже действия — тогда длительность NULL (событие в ленте остаётся).

const (
	processingJournalDefaultDepth = 50
	processingJournalMaxDepth     = 200
)

// journalActorName — подпись актора события: ФИО, при пустых частях — логин. На
// обеих ветках UNION users подключён под алиасом u, выражение общее.
const journalActorName = acceptorNameExpr

// NormalizeProcessingJournalPaging приводит глубину и смещение к допустимому
// диапазону: limit<=0 -> дефолт, выше потолка -> потолок, отрицательный offset -> 0.
// Экспортирована, чтобы handler отдавал в meta РЕАЛЬНО применённые значения, а не
// сырые из query (иначе per_page в ответе разойдётся с размером страницы).
func NormalizeProcessingJournalPaging(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = processingJournalDefaultDepth
	}
	if limit > processingJournalMaxDepth {
		limit = processingJournalMaxDepth
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// processingJournalSource — UNION ALL обеих веток ленты за период [from, to] без
// сортировки и пагинации. Страница и счётчик строят его одним вызовом, чтобы
// предикаты окна не разъехались. event_id (id голоса / id заявки) в выдачу не
// попадает — он нужен как тай-брейк сортировки: без него страницы могли бы
// пересечься на событиях с одинаковым временем.
func processingJournalSource() string {
	// bureauWorkingDuration даёт SQL-выражение из имён колонок (без пользовательского
	// ввода); границы окна, лимит и смещение идут именованными параметрами.
	approvalDur := bureauWorkingDuration("aru.created_at", "aru.approval_datetime")
	acceptDur := bureauWorkingDuration("app.confirmation_datetime", "app.accepted_at")

	return fmt.Sprintf(`
		SELECT
			app.id AS application_id,
			COALESCE(app.application_number, '') AS application_number,
			%[1]s AS actor_name,
			'%[2]s' AS role,
			aru.approval_datetime AS occurred_at,
			CASE WHEN aru.approval_datetime >= aru.created_at
				 THEN ROUND(%[3]s)::bigint END AS working_seconds,
			aru.id AS event_id
		FROM application_responsible_users aru
		JOIN applications app ON app.id = aru.application_id
		LEFT JOIN users u ON u.id = aru.user_id
		WHERE aru.approval_datetime IS NOT NULL
		  AND aru.approval_datetime >= @from AND aru.approval_datetime <= @to
		UNION ALL
		SELECT
			app.id,
			COALESCE(app.application_number, ''),
			%[1]s,
			'%[4]s',
			app.accepted_at,
			CASE WHEN app.confirmation_datetime IS NOT NULL
				  AND app.accepted_at >= app.confirmation_datetime
				 THEN ROUND(%[5]s)::bigint END,
			app.id
		FROM %[6]s acc
		JOIN applications app ON app.id = acc.application_id
		LEFT JOIN users u ON u.id = acc.acceptor_user_id
		WHERE app.accepted_at IS NOT NULL
		  AND app.accepted_at >= @from AND app.accepted_at <= @to`,
		journalActorName,                       // %[1]s — подпись актора
		models.ProcessingJournalRoleApproval,   // %[2]s
		approvalDur,                            // %[3]s — рабочее время согласования
		models.ProcessingJournalRoleAcceptance, // %[4]s
		acceptDur,                              // %[5]s — рабочее время принятия
		acceptorBase,                           // %[6]s — первое принятие каждой заявки (алиас acc в шаблоне)
	)
}

// GetProcessingJournal возвращает страницу событий согласования/принятия за период
// [from, to] по времени убыванием (limit событий, начиная с offset) и общее число
// событий периода для постраничной навигации. Окно бьёт по времени САМОГО события
// (согласование/принятие), а не по дате подачи: лента показывает, что происходило в
// выбранный период.
func (s *statisticsService) GetProcessingJournal(ctx context.Context, from, to time.Time, limit, offset int) ([]models.ProcessingJournalEntry, int64, error) {
	limit, offset = NormalizeProcessingJournalPaging(limit, offset)

	source := processingJournalSource()
	args := map[string]any{"from": from, "to": to, "limit": limit, "offset": offset}

	var total int64
	if err := s.db.WithContext(ctx).Raw(
		fmt.Sprintf(`SELECT COUNT(*) FROM (%s) j`, source), args).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("statistics: processing journal count: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT * FROM (%s) j
		ORDER BY j.occurred_at DESC, j.application_id DESC, j.role, j.event_id DESC
		LIMIT @limit OFFSET @offset`, source)

	entries := make([]models.ProcessingJournalEntry, 0, limit)
	if err := s.db.WithContext(ctx).Raw(query, args).Scan(&entries).Error; err != nil {
		return nil, 0, fmt.Errorf("statistics: processing journal: %w", err)
	}
	return entries, total, nil
}
