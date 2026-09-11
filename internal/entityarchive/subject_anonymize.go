package entityarchive

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"systemburo/internal/models"
	"systemburo/internal/services"
)

// Необратимое обезличивание одного человека (#2356).
//
// Отличие от обезличивания организации не в механике - функции затирания те же, - а в
// том, как выбирается цель: не по идентификатору владельца, а по свёртке документа.
// Отсюда и главное следствие: ПОСЛЕ обезличивания человек перестаёт находиться. Свёртка
// стирается вместе со значением, и собрать по нему сведения больше нельзя - это не
// побочный эффект, а смысл операции.
//
// Порядок работы поэтому такой: сперва справка (subject export), потом обезличивание.
// Наоборот не выйдет: справку будет уже не по чему собрать.

// SubjectAnonymizeResult - что затёрто (или было бы затёрто без -apply).
type SubjectAnonymizeResult struct {
	Origin   string
	Tables   []AnonymizeTableResult
	Warnings []string
}

// Total - сколько строк затронуто.
func (r SubjectAnonymizeResult) Total() int {
	n := 0
	for _, t := range r.Tables {
		n += t.Rows
	}
	return n
}

// errSubjectNotFound - под цель не подошло ни одной строки.
var errSubjectNotFound = errors.New("человек не найден")

// subjectAnonymizeTargets - таблицы, где живут персональные поля человека.
//
// Учётной записи здесь нет намеренно: связи «работник - пользователь системы» в базе
// не существует, и затирать пользователя по совпадению ФИО значило бы обезличить
// однофамильца. Заявки тоже не трогаются: initiator_name и contact_phone в шапке - это
// данные заявителя, а он к субъекту отношения может не иметь.
func subjectAnonymizeTargets() []AnonymizeTableResult {
	fields := employeeLikeFields()
	return []AnonymizeTableResult{
		{Table: "employees", Fields: fields},
		{Table: "unique_employees", Fields: fields},
		{Table: "application_employees", Fields: fields},
	}
}

// AnonymizeSubject затирает персональные поля человека во всех трёх таблицах.
// apply=false - только подсчёт, база не меняется.
func AnonymizeSubject(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder, target SubjectTarget, opt DestructionOptions) (SubjectAnonymizeResult, error) {
	if target.Empty() {
		return SubjectAnonymizeResult{}, fmt.Errorf("цель не задана: нужен паспорт или патент")
	}
	if err := opt.validate(); err != nil {
		return SubjectAnonymizeResult{}, err
	}

	if !opt.Apply {
		res := SubjectAnonymizeResult{Origin: target.Origin, Tables: subjectAnonymizeTargets()}
		for i := range res.Tables {
			ids, err := subjectRowIDs(ctx, db, res.Tables[i].Table, target)
			if err != nil {
				return SubjectAnonymizeResult{}, err
			}
			res.Tables[i].Rows = len(ids)
		}
		if res.Total() == 0 {
			return SubjectAnonymizeResult{}, errSubjectNotFound
		}
		warnings, err := subjectAnonymizeWarnings(ctx, db, target)
		if err != nil {
			return SubjectAnonymizeResult{}, err
		}
		res.Warnings = warnings
		return res, nil
	}

	res := SubjectAnonymizeResult{Origin: target.Origin, Tables: subjectAnonymizeTargets()}
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Предупреждения считаются ДО затирания: после него строк по свёртке уже нет,
		// и число проходов, оставшихся с именем в журнале, посчитать будет нечем.
		warnings, err := subjectAnonymizeWarnings(ctx, tx, target)
		if err != nil {
			return err
		}
		res.Warnings = warnings

		var anchorID int
		total := 0
		for i := range res.Tables {
			table := res.Tables[i].Table
			ids, err := subjectRowIDs(ctx, tx, table, target)
			if err != nil {
				return err
			}
			if table == "unique_employees" && len(ids) > 0 {
				anchorID = ids[0]
			}
			n, err := anonymizeRows(ctx, tx, table, ids)
			if err != nil {
				return err
			}
			res.Tables[i].Rows = n
			total += n
		}
		if total == 0 {
			return errSubjectNotFound
		}

		details := make([]anonymizeAuditTable, len(res.Tables))
		for i, t := range res.Tables {
			details[i] = anonymizeAuditTable{Table: t.Table, Rows: t.Rows}
		}
		// Запись аудита последним шагом транзакции: не выполнилось затирание - не
		// появится и метка «сделано». Привязываем к записи реестра, если она была:
		// у человека своего идентификатора нет, а запись без сущности не найти потом
		// в истории.
		var entityID *int
		if anchorID > 0 {
			entityID = &anchorID
		}
		if err := recorder.Record(ctx, tx, models.AuditEntityUniqueEmployee, entityID,
			models.OrganizationActionAnonymized, opt.ActorID,
			anonymizeDetails{Tables: details}); err != nil {
			return err
		}

		// Свидетельство для повторного применения после восстановления (#2357).
		// Идентификатор здесь ненадёжен - записи реестра у человека может не быть
		// вовсе, - поэтому цель опознаётся отпечатками документов: вернувшегося из
		// копии человека находят пересчётом того же отпечатка.
		rec := opt.destructionRecord(models.AuditEntityUniqueEmployee, entityID, DestructionAnonymized)
		rec.PassportDigest = documentDigest(target.PassportHMAC)
		rec.PatentDigest = documentDigest(target.PatentHMAC)
		rec.Rows = total
		return writeDestruction(ctx, tx, opt, rec)
	})
	switch {
	case errors.Is(err, errSubjectNotFound):
		return SubjectAnonymizeResult{}, errSubjectNotFound
	case err != nil:
		return SubjectAnonymizeResult{}, fmt.Errorf("обезличивание человека: %w", err)
	}
	return res, nil
}

// subjectRowIDs - идентификаторы строк таблицы, принадлежащих человеку.
func subjectRowIDs(ctx context.Context, exec *gorm.DB, table string, target SubjectTarget) ([]int, error) {
	var ids []int
	q := fmt.Sprintf("SELECT id FROM %s WHERE %s ORDER BY id", table, subjectDocs)
	err := exec.WithContext(ctx).Raw(q,
		sql.Named("pass", target.PassportHMAC), sql.Named("patent", target.PatentHMAC)).Scan(&ids).Error
	if err != nil {
		return nil, fmt.Errorf("отбор строк %s: %w", table, err)
	}
	return ids, nil
}

// subjectAnonymizeWarnings - что остаётся с именем человека после затирания.
//
// Молчать об этом нельзя: оператор, запустивший команду, вправе считать, что данных
// больше нет, а они есть - в журнале проходов, в файлах и в журнале выдач.
func subjectAnonymizeWarnings(ctx context.Context, exec *gorm.DB, target SubjectTarget) ([]string, error) {
	var passages int64
	q := `SELECT count(*) FROM audit_log
		WHERE entity_type = 'employee'
		  AND entity_id IN (SELECT id FROM employees WHERE ` + subjectDocs + `)`
	if err := exec.WithContext(ctx).Raw(q,
		sql.Named("pass", target.PassportHMAC), sql.Named("patent", target.PatentHMAC)).
		Scan(&passages).Error; err != nil {
		return nil, fmt.Errorf("подсчёт записей истории: %w", err)
	}

	return []string{
		fmt.Sprintf("в журнале истории и проходов остаётся %d записей об этом человеке, и в их "+
			"пояснении его имя записано открытым текстом («Сотрудник Иванов Иван прошёл на "+
			"территорию»). Команда журнал НЕ трогает: он доказывает, кто и когда был на объекте, "+
			"и чистится только по сроку хранения", passages),
		"файлы, приложенные к заявкам (сканы документов), и слепки бланков в файловом архиве " +
			"остаются как есть - это те же персональные данные в другом виде, решение по ним " +
			"отдельное, за владельцем системы",
		"записи журнала выдач сведений об этом человеке остаются с его именем: журнал " +
			"доказывает законность уже состоявшегося раскрытия, и обезличивать его нельзя",
		"после обезличивания человек перестаёт находиться по документу - справку о нём " +
			"собрать будет уже не по чему. Если она нужна, снимите её ДО обезличивания",
	}, nil
}
