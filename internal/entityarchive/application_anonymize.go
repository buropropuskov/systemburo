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

// Необратимое обезличивание одной заявки (#2355).
//
// Данные удаляет не человек и не кнопка, а истечение срока хранения - и удаляет не
// полностью: заявка, её даты, организация, статусы, решения согласующих и факт прохода
// остаются, а ФИО, документы и контакты затираются. Обезличенная заявка перестаёт быть
// персональными данными, хранить её можно дальше, и статистика не рушится.
//
// Отсюда разница с обезличиванием человека (#2356): там цель - конкретный субъект во
// всех заявках сразу, здесь - все люди одной заявки. Механизм затирания общий, меняется
// только отбор строк.
//
// Журнал истории и проходов не трогается намеренно: он доказывает, кто и когда был на
// объекте, и обезличив его, оператор лишится доказательства. У журнала свой срок
// хранения (группа audit в уборке), по нему записи и уходят.

// TypeApplication - цель «заявка».
const TypeApplication = "application"

// errApplicationNotFound - заявки с таким идентификатором нет.
var errApplicationNotFound = errors.New("заявка не найдена")

// applicationAnonymizeTargets - что затирается у заявки.
func applicationAnonymizeTargets() []AnonymizeTableResult {
	fields := employeeLikeFields()
	return []AnonymizeTableResult{
		{Table: "employees", Fields: fields},
		{Table: "application_employees", Fields: fields},
		{Table: "applications", Fields: []string{
			"initiator_name", "contact_phone",
		}},
		{Table: blacklistFlagsTable, Fields: []string{
			"element_normalized (только у строк своего сотрудника - element_type=employee)",
		}},
		{Table: blacklistOverridesTable, Fields: []string{
			"element_normalized (только у строк своего сотрудника)",
		}},
	}
}

// AnonymizeApplication затирает персональные поля одной заявки и уничтожает её файлы.
// apply=false - только подсчёт, ни база, ни диск не меняются.
//
// Файлы уходят ПЕРЕД транзакцией затирания: удаление с диска откатить нечем, и порядок
// выбран так, чтобы сбой посередине чинился сам собой. Упало после диска - файлов уже
// нет, повторный прогон уберёт остальное; упади оно наоборот, на диске остался бы
// читаемый паспорт заявки, которая в базе выглядит обезличенной.
func AnonymizeApplication(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder, id int, opt DestructionOptions) (AnonymizeResult, error) {
	if id <= 0 {
		return AnonymizeResult{}, fmt.Errorf("не указана заявка")
	}
	if err := opt.validate(); err != nil {
		return AnonymizeResult{}, err
	}

	if !opt.Apply {
		exists, err := applicationExists(ctx, db, id)
		if err != nil {
			return AnonymizeResult{}, err
		}
		if !exists {
			return AnonymizeResult{}, errApplicationNotFound
		}
		res := AnonymizeResult{Type: TypeApplication, ID: id, Tables: applicationAnonymizeTargets()}
		for i := range res.Tables {
			ids, err := applicationRowIDs(ctx, db, res.Tables[i].Table, id)
			if err != nil {
				return AnonymizeResult{}, err
			}
			res.Tables[i].Rows = len(ids)
		}
		files, err := purgeApplicationFiles(ctx, db, opt.Files, id, false)
		if err != nil {
			return AnonymizeResult{}, err
		}
		res.Files = files
		res.Warnings = applicationAnonymizeWarnings(files)
		return res, nil
	}

	// Существование проверяется до уничтожения файлов: ошибиться идентификатором
	// заявки можно, а вернуть снятый с диска скан паспорта - нет.
	exists, err := applicationExists(ctx, db, id)
	if err != nil {
		return AnonymizeResult{}, err
	}
	if !exists {
		return AnonymizeResult{}, errApplicationNotFound
	}

	res := AnonymizeResult{Type: TypeApplication, ID: id, Tables: applicationAnonymizeTargets()}
	res.Files, err = purgeApplicationFiles(ctx, db, opt.Files, id, true)
	if err != nil {
		return AnonymizeResult{}, err
	}

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Номер читается внутри той же транзакции и заодно проверяет существование:
		// снаружи заявка могла бы исчезнуть между проверкой и записью. Нужен он
		// журналу уничтожения - по идентификатору заявку в акте не опознать.
		number, err := applicationNumber(ctx, tx, id)
		if err != nil {
			return err
		}

		for i := range res.Tables {
			table := res.Tables[i].Table
			ids, err := applicationRowIDs(ctx, tx, table, id)
			if err != nil {
				return err
			}
			n, err := anonymizeRows(ctx, tx, table, ids)
			if err != nil {
				return err
			}
			res.Tables[i].Rows = n
		}
		res.Warnings = applicationAnonymizeWarnings(res.Files)

		details := make([]anonymizeAuditTable, len(res.Tables))
		for i, t := range res.Tables {
			details[i] = anonymizeAuditTable{Table: t.Table, Rows: t.Rows}
		}
		// Запись в историю последним шагом транзакции: не выполнилось затирание - не
		// появится и метка «сделано».
		if err := recorder.Record(ctx, tx, models.AuditEntityApplication, &id,
			models.OrganizationActionAnonymized, opt.ActorID, anonymizeDetails{
				Tables: details,
				Files: &anonymizeAuditFiles{
					Attached: res.Files.Attached,
					Archive:  res.Files.Archive,
					Bytes:    res.Files.Bytes(),
				},
			}); err != nil {
			return err
		}

		// Свидетельство для повторного применения после восстановления из копии
		// (#2357). Идёт той же транзакцией: восстановленная заявка вернётся с тем же
		// идентификатором, и снять её будет по чему.
		rec := opt.destructionRecord(models.AuditEntityApplication, &id, DestructionAnonymized)
		rec.ApplicationNumber = number
		rec.Rows = res.Total()
		rec.Files = res.Files.Total()
		return writeDestruction(ctx, tx, opt, rec)
	})
	switch {
	case errors.Is(err, errApplicationNotFound):
		return AnonymizeResult{}, errApplicationNotFound
	case err != nil:
		return AnonymizeResult{}, fmt.Errorf("обезличивание заявки #%d: %w", id, err)
	}
	return res, nil
}

// applicationExists - есть ли такая заявка.
func applicationExists(ctx context.Context, exec *gorm.DB, id int) (bool, error) {
	var n int64
	if err := exec.WithContext(ctx).Raw(`SELECT count(*) FROM applications WHERE id = ?`, id).
		Scan(&n).Error; err != nil {
		return false, fmt.Errorf("проверка заявки %d: %w", id, err)
	}
	return n > 0, nil
}

// applicationRowIDs - строки таблицы, принадлежащие заявке.
func applicationRowIDs(ctx context.Context, exec *gorm.DB, table string, id int) ([]int, error) {
	var q string
	switch table {
	case "applications":
		q = `SELECT id FROM applications WHERE id = @app`
	case "employees", "application_employees":
		q = fmt.Sprintf(`SELECT id FROM %s WHERE attachment_id IN (
			SELECT id FROM attachments WHERE application_id = @app)`, table)
	case blacklistFlagsTable, blacklistOverridesTable:
		// Только строки своего сотрудника: у element_type=car это номер машины, и
		// обезличивание его не касается - тот же разбор, что у организации.
		q = fmt.Sprintf(`SELECT id FROM %s WHERE application_id = @app AND element_type = 'employee'`, table)
	default:
		return nil, fmt.Errorf("отбор строк %s для заявки не реализован", table)
	}

	var ids []int
	if err := exec.WithContext(ctx).Raw(q, sql.Named("app", id)).Scan(&ids).Error; err != nil {
		return nil, fmt.Errorf("отбор строк %s: %w", table, err)
	}
	return ids, nil
}

// applicationAnonymizeWarnings - что происходит с данными за пределами затираемых полей.
func applicationAnonymizeWarnings(files FilePurgeResult) []string {
	out := []string{
		"журнал истории и проходов не трогается: он доказывает, кто и когда был на " +
			"объекте, и уходит по собственному сроку хранения (группа audit в уборке)",
		"записи журнала выдач по людям этой заявки остаются с именами: журнал " +
			"доказывает законность уже состоявшегося раскрытия",
	}
	if files.Total() > 0 {
		out = append(out, fmt.Sprintf(
			"файлы заявки уничтожаются, а не затираются: приложенных документов %d, "+
				"файлов архива (бланки и слепок заявка.json) %d - скан паспорта "+
				"обезличить по полям нельзя",
			files.Attached, files.Archive))
	}
	// Непустой Skipped означает, что каталог установке не настроен: оператор обязан
	// узнать, что часть копий осталась лежать, а не считать уничтожение полным.
	out = append(out, files.Skipped...)
	return out
}
