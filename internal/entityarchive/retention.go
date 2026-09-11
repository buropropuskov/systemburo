package entityarchive

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"systemburo/internal/services"
)

// Обезличивание заявок по истечении срока хранения (#2355).
//
// Данные удаляет не человек и не кнопка, а истечение срока - и не удаляет, а
// обезличивает: заявка, её даты, статус, решения согласующих и факт прохода остаются,
// ФИО и документы участников затираются. Иначе оператор не сможет ответить на запрос
// государственного органа о том, приходил ли человек и когда, - а отвечать обязан.
//
// Срок задаёт владелец системы. По умолчанию его НЕТ: молча начать затирать данные
// установки, которая о сроке не просила, нельзя.

// ApplicationRetentionCandidate - заявка, у которой срок истёк.
type ApplicationRetentionCandidate struct {
	ID     int
	Number *string
	Status *string
	// AnchorAt - от какой даты считался срок.
	AnchorAt time.Time
}

// Опорная дата срока: когда с заявкой в последний раз что-то произошло.
//
// Столбца created_at у заявок нет вовсе, а completed_at заполнен не у всех: на стенде
// у 96 завершённых заявок он стоит у 31. Поэтому берём первую попавшуюся из трёх - от
// завершения к отзыву и к подаче. Подача есть у всех 170 записей, так что без опорной
// даты не остаётся ни одна заявка.
const applicationAnchor = "COALESCE(completed_at, withdrawn_at, sending_datetime)"

// Заявки в работе не обезличиваются: по ним ещё ходят люди, и затирать участников
// действующего пропуска нельзя. Отбираем только те, чья жизнь закончилась.
const applicationFinishedStatuses = `('Завершено', 'Отозвана', 'Отказано')`

// applicationLivePeople - признак того, что заявка ещё НЕ обезличена: у неё остались
// участники с именем или документом. Идентификатор заявки подставляется вместо %s -
// у отбора по сроку это ссылка на внешний запрос (a.id), у повторного применения
// после восстановления (#2357) именованный параметр. Предикат один на оба: разойдись
// они, повторное применение считало бы обезличенной заявку, которую суточный прогон
// обезличивать ещё собирается.
const applicationLivePeople = `EXISTS (
	SELECT 1 FROM employees e
	JOIN attachments att ON att.id = e.attachment_id
	WHERE att.application_id = %s
	  AND (e.last_name IS NOT NULL OR e.passport_series_number IS NOT NULL)
)`

// FindApplicationsForRetention возвращает заявки, чей срок хранения истёк.
//
// Уже обезличенные в выборку не попадают: признак - отсутствие живых участников с
// именем. Без этого условия каждый прогон переобезличивал бы одни и те же заявки,
// плодя записи в истории.
func FindApplicationsForRetention(ctx context.Context, db *gorm.DB, cutoff time.Time, limit int) ([]ApplicationRetentionCandidate, error) {
	q := fmt.Sprintf(`
		SELECT id, application_number AS number, status, %[1]s AS anchor_at
		FROM applications a
		WHERE status IN %[2]s
		  AND %[1]s IS NOT NULL
		  AND %[1]s < ?
		  AND %[3]s
		ORDER BY %[1]s
		LIMIT ?`, applicationAnchor, applicationFinishedStatuses,
		fmt.Sprintf(applicationLivePeople, "a.id"))

	var out []ApplicationRetentionCandidate
	if err := db.WithContext(ctx).Raw(q, cutoff, limit).Scan(&out).Error; err != nil {
		return nil, fmt.Errorf("отбор заявок с истёкшим сроком: %w", err)
	}
	return out, nil
}

// RetentionResult - итог прогона обезличивания по сроку.
type RetentionSweepResult struct {
	Checked int
	Applied int
	Rows    int
	// Files - сколько файлов заявок уничтожено на диске: приложенные документы и
	// корпоративные копии бланков. Затирание полей их не касается, и без этого
	// счётчика прогон отчитывался бы о полном обезличивании, оставив на диске
	// читаемые паспорта (#2355).
	Files int
}

// SweepApplicationRetention обезличивает заявки, чей срок истёк.
//
// apply=false - только подсчёт: оператор обязан увидеть объём до того, как необратимая
// операция пройдёт по всей базе.
func SweepApplicationRetention(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder, paths FilePaths, cutoff time.Time, limit int, apply bool) (RetentionSweepResult, error) {
	candidates, err := FindApplicationsForRetention(ctx, db, cutoff, limit)
	if err != nil {
		return RetentionSweepResult{}, err
	}

	res := RetentionSweepResult{Checked: len(candidates)}
	for _, c := range candidates {
		out, err := AnonymizeApplication(ctx, db, recorder, c.ID,
			DestructionOptions{Files: paths, Basis: BasisRetention, Apply: apply})
		if err != nil {
			// Одна сбойная заявка не должна останавливать весь прогон: остальные
			// обезличить всё равно надо, а о сбое говорим вслух.
			slog.Error("заявка не обезличена по сроку", "application", c.ID, "error", err)
			continue
		}
		if apply {
			res.Applied++
		}
		for _, t := range out.Tables {
			res.Rows += t.Rows
		}
		res.Files += out.Files.Total()
	}
	return res, nil
}
