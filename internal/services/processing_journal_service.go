package services

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"systemburo/internal/models"
)

// Сквозная лента событий обработки (#1251 S4, роли расширены в P7): голоса
// согласующих (approval_datetime; согласование и несогласование — разные роли),
// принятия в работу (первое take_to_work, acceptorBase), отказы принимающего и
// отзывы инициатором (audit_log) одним UNION ALL, отсортированным по времени
// убыванием. Рабочая длительность события — bureauWorkingDuration (#1251 S2, с
// фолбэком на календарь, когда график Бюро пуст или событие в него не попало).
// Кэша нет: лента «реального времени», глубину ограничивает лимит. Момент начала (назначение / согласование)
// может отсутствовать или стоять позже действия — тогда длительность NULL (событие
// в ленте остаётся).
//
// Ветка audit_log показывает КАЖДОЕ действие, а не первое: отказать после возврата
// в обработку можно повторно, и каждый отказ — самостоятельное событие ленты. У
// принятия иначе — оно привязано к accepted_at (первое принятие), от которого
// считается рабочее время, поэтому там acceptorBase с DISTINCT ON.

const (
	processingJournalDefaultDepth = 50
	processingJournalMaxDepth     = 200
)

// journalActorName — подпись актора события: ФИО, при пустых частях — логин. На
// всех ветках UNION users подключён под алиасом u, выражение общее.
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

// ProcessingJournalFilter — отбор событий ленты поверх окна периода (#1251 P5c):
// роль события и поиск по номеру заявки или подписи актора. Пустое поле — фильтр
// не применяется.
type ProcessingJournalFilter struct {
	Role   string // одна из models.ProcessingJournalRoles, иное значение отсекает NormalizeProcessingJournalFilter
	Search string // подстрока номера заявки или ФИО/логина актора, регистр не важен
}

// NormalizeProcessingJournalFilter приводит фильтр к применимому виду: неизвестная
// роль превращается в ошибку (её отдаёт handler как 400 — молча показывать ВСЕ
// события на опечатку в параметре значило бы врать пользователю о выборке), поиск
// обрезается по краям.
func NormalizeProcessingJournalFilter(f ProcessingJournalFilter) (ProcessingJournalFilter, error) {
	f.Role = strings.TrimSpace(f.Role)
	f.Search = strings.TrimSpace(f.Search)
	if f.Role != "" && !slices.Contains(models.ProcessingJournalRoles, f.Role) {
		return f, fmt.Errorf("unknown processing journal role %q", f.Role)
	}
	return f, nil
}

// journalSearchPattern — подстрочный шаблон для ILIKE. Спецсимволы самого LIKE
// экранируются: без этого «%» из поля поиска матчил бы всё подряд, а «_» — любой
// символ, и выборка молча переставала бы соответствовать запросу.
func journalSearchPattern(search string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search)
	return "%" + escaped + "%"
}

// processingJournalSource — UNION ALL всех веток ленты за период [from, to] без
// сортировки и пагинации. Страница и счётчик строят его одним вызовом, чтобы
// предикаты окна не разъехались. event_id (id голоса / заявки / записи аудита) в
// выдачу не попадает — он нужен как тай-брейк сортировки: без него страницы могли бы
// пересечься на событиях с одинаковым временем.
func processingJournalSource() string {
	// bureauWorkingDuration даёт SQL-выражение из имён колонок (без пользовательского
	// ввода); границы окна, лимит и смещение идут именованными параметрами.
	approvalDur := bureauWorkingDuration("aru.created_at", "aru.approval_datetime")
	acceptDur := bureauWorkingDuration("app.confirmation_datetime", "app.accepted_at")
	rejectDur := bureauWorkingDuration("app.confirmation_datetime", "al.created_at")

	// Роль голоса согласующего: явный отказ — несогласование, всё остальное —
	// согласование. Голос без 'rejected', но с датой, — только 'approved': снятие
	// согласования (application_approval_service) чистит и статус, и дату разом,
	// поэтому pending с проставленной датой в данных не встречается.
	approvalRole := fmt.Sprintf(`CASE WHEN aru.approval_status = 'rejected'
			THEN '%s' ELSE '%s' END`,
		models.ProcessingJournalRoleNotApproved, models.ProcessingJournalRoleApproval)

	// Отказ принимающего и отзыв инициатором живут в audit_log. Действие 'reject'
	// делят принимающий и согласующий (см. models.AuditActionReject), поэтому берём
	// только записи со сменой статуса на «Отказано» — иначе несогласования приехали
	// бы в ленту дважды: и из голоса, и из аудита.
	auditRole := fmt.Sprintf(`CASE WHEN al.action = '%s'
			THEN '%s' ELSE '%s' END`,
		models.AuditActionWithdraw,
		models.ProcessingJournalRoleWithdrawal, models.ProcessingJournalRoleRejection)

	return fmt.Sprintf(`
		SELECT
			app.id AS application_id,
			COALESCE(app.application_number, '') AS application_number,
			%[1]s AS actor_name,
			%[2]s AS role,
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
		  AND app.accepted_at >= @from AND app.accepted_at <= @to
		UNION ALL
		SELECT
			app.id,
			COALESCE(app.application_number, ''),
			%[1]s,
			%[7]s,
			al.created_at,
			CASE WHEN al.action = '%[8]s'
				  AND app.confirmation_datetime IS NOT NULL
				  AND al.created_at >= app.confirmation_datetime
				 THEN ROUND(%[9]s)::bigint END,
			al.id
		FROM audit_log al
		JOIN applications app ON app.id = al.entity_id
		LEFT JOIN users u ON u.id = al.actor_user_id
		WHERE al.entity_type = '%[10]s'
		  AND (
			(al.action = '%[8]s' AND al.details->>'new_value' = '%[11]s')
			OR al.action = '%[12]s'
		  )
		  AND al.created_at >= @from AND al.created_at <= @to`,
		journalActorName,                       // %[1]s — подпись актора
		approvalRole,                           // %[2]s — согласование или несогласование
		approvalDur,                            // %[3]s — рабочее время голоса согласующего
		models.ProcessingJournalRoleAcceptance, // %[4]s
		acceptDur,                              // %[5]s — рабочее время принятия
		acceptorBase,                           // %[6]s — первое принятие каждой заявки (алиас acc в шаблоне)
		auditRole,                              // %[7]s — отказ или отзыв
		models.AuditActionReject,               // %[8]s
		rejectDur,                              // %[9]s — рабочее время отказа принимающего
		models.AuditEntityApplication,          // %[10]s
		models.StatusRefused,                   // %[11]s — маркер отказа ИМЕННО принимающего
		models.AuditActionWithdraw,             // %[12]s
	)
}

// GetProcessingJournal возвращает страницу событий обработки (голоса согласующих,
// принятия, отказы принимающего, отзывы инициатором) за период [from, to] по
// времени убыванием (limit событий, начиная с offset) и общее число
// подходящих событий для постраничной навигации. Окно бьёт по времени САМОГО события
// (согласование/принятие), а не по дате подачи: лента показывает, что происходило в
// выбранный период. Фильтр роли и поиска применяются ПОВЕРХ окна одним предикатом и
// для страницы, и для счётчика — иначе «Всего» разошлось бы с содержимым страниц.
func (s *statisticsService) GetProcessingJournal(ctx context.Context, from, to time.Time, filter ProcessingJournalFilter, limit, offset int) ([]models.ProcessingJournalEntry, int64, error) {
	// HTTP-слой уже нормализовал (ему значения нужны для meta), но метод публичный:
	// нормализуем и здесь, иначе прямой вызов из другого места ушёл бы в БД с
	// limit<=0. Функция идемпотентна, повторный вызов ничего не меняет.
	limit, offset = NormalizeProcessingJournalPaging(limit, offset)
	filter, err := NormalizeProcessingJournalFilter(filter)
	if err != nil {
		return nil, 0, err
	}

	source := fmt.Sprintf(`
		SELECT * FROM (%s) j
		WHERE (@role = '' OR j.role = @role)
		  AND (@search = ''
			   OR j.application_number ILIKE @pattern ESCAPE '\'
			   OR j.actor_name ILIKE @pattern ESCAPE '\')`, processingJournalSource())
	args := map[string]any{
		"from": from, "to": to, "limit": limit, "offset": offset,
		"role": filter.Role, "search": filter.Search,
		"pattern": journalSearchPattern(filter.Search),
	}

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
