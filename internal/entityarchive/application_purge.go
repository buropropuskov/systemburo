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

// Уничтожение ошибочно заведённой заявки (#2355, пункт 5).
//
// Это редкий ручной инструмент, а не исполнение требований субъекта: по истечении
// срока хранения заявки обезличиваются (AnonymizeApplication), и данные при этом не
// теряются. Здесь заявка исчезает целиком - вместе со вложениями, участниками,
// согласованиями, файлами и собственной историей. Нужно это тогда, когда записи не
// должно было существовать вовсе: заявку завели по ошибке, не на того человека или
// дважды.
//
// История заявки уходит вместе с ней намеренно. В деталях записей лежат ФИО (тема
// записи пишется словами), и уничтожение, оставляющее историю, оставляло бы на месте
// ровно те сведения, ради которых его затевали. Отметки прохода - обратный случай:
// они доказывают, что человек был на объекте, и заявка с ними ошибочной уже не
// выглядит, поэтому команда называет их число перед -apply отдельной строкой.

// applicationPurgeNode - таблица заявки и то, как отбираются её строки.
type applicationPurgeNode struct {
	Table string
	// ByAttachment - строки привязаны к вложению, а не к самой заявке.
	ByAttachment bool
	// Cascade - строки унесёт внешний ключ при удалении заявки; DELETE по ним не
	// нужен, но в счёт они входят: оператор обязан видеть весь объём, а не ту его
	// часть, которую система удаляет явным запросом.
	Cascade bool
}

// applicationPurgeNodes - граф заявки. Полнота держится замком
// TestApplicationGraph_NoUnaccountedTables: таблица, появившаяся позже и ссылающаяся
// на заявку или вложение, роняет тест, а не тихо переживает уничтожение.
var applicationPurgeNodes = []applicationPurgeNode{
	{Table: "employees", ByAttachment: true, Cascade: true},
	{Table: "application_employees", ByAttachment: true, Cascade: true},
	{Table: "cars", ByAttachment: true, Cascade: true},
	{Table: "items", ByAttachment: true, Cascade: true},
	{Table: "application_items", ByAttachment: true, Cascade: true},
	{Table: "attachment_unload_places", ByAttachment: true, Cascade: true},
	{Table: "application_question_attachments", ByAttachment: true, Cascade: true},
	{Table: "attachments", Cascade: true},
	{Table: "forward_attachments", Cascade: true},
	{Table: "application_questions", Cascade: true},
	{Table: "application_question_views", Cascade: true},
	{Table: "application_status_history", Cascade: true},
	{Table: "application_status_views", Cascade: true},
	{Table: "application_responsible_users", Cascade: true},
	{Table: "application_viewers", Cascade: true},
	{Table: "application_supplements", Cascade: true},
	{Table: "application_files", Cascade: true},
	// Ниже - таблицы без внешнего ключа (философия audit_log и blank_exports: каскад
	// снёс бы строку и оставил файл-сироту). Их уносит явный DELETE.
	//
	// attachment_custom_values попала сюда не по замыслу, а по находке замка: внешнего
	// ключа у неё нет ни на вложение, ни на заявку, и значения дополнительных полей
	// пережили бы удаление заявки строками-сиротами.
	{Table: "attachment_custom_values", ByAttachment: true},
	{Table: "application_answers"},
	{Table: "application_reads"},
	{Table: blacklistFlagsTable},
	{Table: blacklistOverridesTable},
	{Table: "blank_exports"},
}

// ApplicationGraphTables - имена таблиц графа заявки. Экспортируется ради замка
// TestApplicationGraph_NoUnaccountedTables: тесту нужна карта графа, а заводить ради
// него вторую копию перечня хуже, чем узкий геттер (тот же приём, что у GraphTables).
func ApplicationGraphTables() []string {
	out := make([]string, 0, len(applicationPurgeNodes))
	for _, n := range applicationPurgeNodes {
		out = append(out, n.Table)
	}
	return out
}

// ApplicationPurgeResult - что уничтожено (или будет уничтожено при показе).
type ApplicationPurgeResult struct {
	ID     int
	Number string
	Tables []PurgeTableCount
	// History - записи истории заявки и её участников.
	History int64
	// Files - файлы заявки: приложенные документы и копии в файловом архиве.
	Files FilePurgeResult
	// Warnings - то, что оператор обязан прочитать до -apply.
	Warnings []string
}

// TotalRows - сколько строк уничтожается во всём графе, включая историю и саму заявку.
func (r ApplicationPurgeResult) TotalRows() int64 {
	n := int64(1) + r.History
	for _, t := range r.Tables {
		n += t.Rows
	}
	return n
}

// PurgeApplication уничтожает заявку целиком.
// apply=false - только подсчёт: ни база, ни диск не меняются.
func PurgeApplication(ctx context.Context, db *gorm.DB, recorder services.AuditRecorder,
	id int, opt DestructionOptions) (ApplicationPurgeResult, error) {
	if id <= 0 {
		return ApplicationPurgeResult{}, fmt.Errorf("не указана заявка")
	}
	if err := opt.validate(); err != nil {
		return ApplicationPurgeResult{}, err
	}

	res := ApplicationPurgeResult{ID: id}
	number, err := applicationNumber(ctx, db, id)
	if err != nil {
		return ApplicationPurgeResult{}, err
	}
	res.Number = number

	res.Tables, err = applicationNodeCounts(ctx, db, id)
	if err != nil {
		return ApplicationPurgeResult{}, err
	}
	res.History, err = applicationHistoryCount(ctx, db, id)
	if err != nil {
		return ApplicationPurgeResult{}, err
	}
	passages, err := applicationPassageCount(ctx, db, id)
	if err != nil {
		return ApplicationPurgeResult{}, err
	}
	res.Warnings = applicationPurgeWarnings(passages)

	if !opt.Apply {
		files, err := purgeApplicationFiles(ctx, db, opt.Files, id, false)
		if err != nil {
			return ApplicationPurgeResult{}, err
		}
		res.Files = files
		res.Warnings = append(res.Warnings, files.Skipped...)
		return res, nil
	}

	// Диск первым, по тому же доводу, что и у обезличивания: удаление файла откатить
	// нечем, и сбой посередине чинится повторным прогоном.
	res.Files, err = purgeApplicationFiles(ctx, db, opt.Files, id, true)
	if err != nil {
		return ApplicationPurgeResult{}, err
	}
	res.Warnings = append(res.Warnings, res.Files.Skipped...)

	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := deleteApplicationHistory(ctx, tx, id); err != nil {
			return err
		}
		// Порядок важен: строки, привязанные к вложению, отбираются по живым
		// attachments, а их унесёт каскад от самой заявки следующим шагом.
		for _, node := range applicationPurgeNodes {
			if node.Cascade {
				continue
			}
			q := fmt.Sprintf(`DELETE FROM %s WHERE application_id = @app`, node.Table)
			if node.ByAttachment {
				q = fmt.Sprintf(`DELETE FROM %s WHERE attachment_id IN (
					SELECT id FROM attachments WHERE application_id = @app)`, node.Table)
			}
			if err := tx.WithContext(ctx).Exec(q, sql.Named("app", id)).Error; err != nil {
				return fmt.Errorf("удаление строк %s: %w", node.Table, err)
			}
		}

		out := tx.WithContext(ctx).Exec(`DELETE FROM applications WHERE id = ?`, id)
		if out.Error != nil {
			return fmt.Errorf("удаление заявки: %w", out.Error)
		}
		if out.RowsAffected == 0 {
			return errApplicationNotFound
		}

		// Запись журнала - последним шагом и уже после удаления: она единственное, что
		// остаётся от заявки, и появиться обязана только если уничтожение состоялось.
		if err := recorder.Record(ctx, tx, models.AuditEntityApplication, &id,
			models.OrganizationActionPurged, opt.ActorID, applicationPurgeDetails{
				Number:  res.Number,
				Rows:    res.TotalRows(),
				History: res.History,
				Files:   res.Files.Total(),
			}); err != nil {
			return err
		}

		// Свидетельство для повторного применения после восстановления (#2357).
		// Отдельно от audit_log намеренно: историю самой заявки этот же шаг удаляет,
		// и запись об уничтожении в ней не пережила бы восстановление из копии,
		// снятой до уничтожения.
		rec := opt.destructionRecord(models.AuditEntityApplication, &id, DestructionPurged)
		rec.ApplicationNumber = res.Number
		rec.Rows = int(res.TotalRows())
		rec.Files = res.Files.Total()
		return writeDestruction(ctx, tx, opt, rec)
	})
	switch {
	case errors.Is(err, errApplicationNotFound):
		return ApplicationPurgeResult{}, errApplicationNotFound
	case err != nil:
		return ApplicationPurgeResult{}, fmt.Errorf("уничтожение заявки #%d: %w", id, err)
	}
	return res, nil
}

// applicationPurgeDetails - содержимое записи журнала. Ни ФИО, ни документов: запись
// переживает заявку и не должна становиться местом, где сведения о человеке уцелели.
// Номер заявки остаётся - по нему уничтожение и опознают при разборе.
type applicationPurgeDetails struct {
	Number  string `json:"number,omitempty"`
	Rows    int64  `json:"rows"`
	History int64  `json:"history_rows"`
	Files   int    `json:"files"`
}

// applicationNumber - номер заявки; заодно проверяет, что заявка существует.
func applicationNumber(ctx context.Context, db *gorm.DB, id int) (string, error) {
	var rows []sql.NullString
	if err := db.WithContext(ctx).
		Raw(`SELECT application_number FROM applications WHERE id = ?`, id).Scan(&rows).Error; err != nil {
		return "", fmt.Errorf("проверка заявки %d: %w", id, err)
	}
	if len(rows) == 0 {
		return "", errApplicationNotFound
	}
	return rows[0].String, nil
}

// applicationNodeCounts считает строки графа заявки по каждой таблице.
func applicationNodeCounts(ctx context.Context, db *gorm.DB, id int) ([]PurgeTableCount, error) {
	out := make([]PurgeTableCount, 0, len(applicationPurgeNodes))
	for _, node := range applicationPurgeNodes {
		q := fmt.Sprintf(`SELECT count(*) FROM %s WHERE application_id = @app`, node.Table)
		if node.ByAttachment {
			q = fmt.Sprintf(`SELECT count(*) FROM %s WHERE attachment_id IN (
				SELECT id FROM attachments WHERE application_id = @app)`, node.Table)
		}
		var n int64
		if err := db.WithContext(ctx).Raw(q, sql.Named("app", id)).Scan(&n).Error; err != nil {
			return nil, fmt.Errorf("подсчёт строк %s: %w", node.Table, err)
		}
		out = append(out, PurgeTableCount{Table: node.Table, Rows: n})
	}
	return out, nil
}

// applicationHistoryCondition - записи истории самой заявки и её участников.
// Сотрудники живут в истории по собственному идентификатору, и без второй части
// условия комментарии вида «Сотрудник Иванов Иван прошёл» пережили бы уничтожение.
const applicationHistoryCondition = `(
	(entity_type = 'application' AND entity_id = @app)
	OR (entity_type = 'employee' AND entity_id IN (
		SELECT e.id FROM employees e
		JOIN attachments att ON att.id = e.attachment_id
		WHERE att.application_id = @app))
)`

func applicationHistoryCount(ctx context.Context, db *gorm.DB, id int) (int64, error) {
	var n int64
	q := `SELECT count(*) FROM audit_log WHERE ` + applicationHistoryCondition
	if err := db.WithContext(ctx).Raw(q, sql.Named("app", id)).Scan(&n).Error; err != nil {
		return 0, fmt.Errorf("подсчёт истории заявки: %w", err)
	}
	return n, nil
}

func deleteApplicationHistory(ctx context.Context, tx *gorm.DB, id int) error {
	q := `DELETE FROM audit_log WHERE ` + applicationHistoryCondition
	if err := tx.WithContext(ctx).Exec(q, sql.Named("app", id)).Error; err != nil {
		return fmt.Errorf("удаление истории заявки: %w", err)
	}
	return nil
}

// applicationPassageCount - отметки входа и выхода людей заявки.
func applicationPassageCount(ctx context.Context, db *gorm.DB, id int) (int64, error) {
	var n int64
	q := `SELECT count(*) FROM audit_log
		WHERE entity_type = 'employee' AND action IN ('entry', 'exit')
		  AND entity_id IN (
			SELECT e.id FROM employees e
			JOIN attachments att ON att.id = e.attachment_id
			WHERE att.application_id = @app)`
	if err := db.WithContext(ctx).Raw(q, sql.Named("app", id)).Scan(&n).Error; err != nil {
		return 0, fmt.Errorf("подсчёт отметок прохода: %w", err)
	}
	return n, nil
}

func applicationPurgeWarnings(passages int64) []string {
	out := []string{
		"уничтожение необратимо: откат не предусмотрен, а по истечении срока хранения " +
			"заявки обезличиваются (anonymize) - там данные не теряются",
		"история заявки и её участников уходит вместе с ней: в пояснениях записей " +
			"стоят ФИО, и оставить их значило бы оставить то, ради чего всё затевалось",
	}
	if passages > 0 {
		out = append(out, fmt.Sprintf(
			"по людям этой заявки есть отметки прохода (%d): заявка отработала на объекте, "+
				"и ошибочно заведённой она не выглядит - проверьте, ту ли запись уничтожаете",
			passages))
	}
	return out
}
