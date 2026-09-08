package entityarchive

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"systemburo/internal/crypto"
	"systemburo/internal/models"
	"systemburo/internal/normalize"
)

// Граф данных одного человека - субъекта персональных данных (#2356).
//
// Зачем: на запрос государственного органа («был ли этот человек на объекте и когда»)
// оператор обязан ответить полно и доказать полноту. Сегодня для этого нужно вручную
// обойти заявки, реестр, посты, отметки прохода и историю.
//
// Главная трудность не в обходе таблиц, а в том, что единого идентификатора у человека
// НЕТ. Он рассыпан по записи реестра, участникам заявок и учётной записи, и склеить эти
// строки можно только по документу или по имени. Отсюда правило пакета:
//
//   - склейка идёт по свёртке документа (`passport_series_number_hmac`,
//     `patent_number_hmac`). Свёртка детерминирована и заведена ровно для такого
//     сравнения: находит точное совпадение, не раскрывая самого значения;
//   - по ФИО записи НЕ склеиваются. Однофамильцы существуют, а приписать человеку
//     чужие проходы в ответе органу - это ошибка, которую потом никто не заметит.
//     Совпадения по имени возвращаются отдельным списком кандидатов, и решает человек.
//
// Учётная запись в граф не входит: связи «работник - пользователь системы» в базе нет
// (`unique_employees.user_id` - это владелец записи, тот кто её завёл, а не сам
// работник). Появится связь - появится и узел.

// TypeSubject - цель «человек».
const TypeSubject = "subject"

// SubjectTarget - ключ субъекта: свёртки его документов. Пустая свёртка не совпадает
// ни с чем - условие узлов проверяет это явно, иначе цель без патента собрала бы все
// строки, где патента нет вовсе.
type SubjectTarget struct {
	PassportHMAC string
	PatentHMAC   string
	// Origin - как цель была найдена, для вывода команды: «по паспорту», «по записи
	// реестра 416». Значения документов сюда не попадают.
	Origin string
}

// Empty - цель не задана ни одним документом.
func (t SubjectTarget) Empty() bool {
	return t.PassportHMAC == "" && t.PatentHMAC == ""
}

// SubjectTargetFromDocuments строит цель по открытым значениям документов.
func SubjectTargetFromDocuments(passport, patent string) SubjectTarget {
	t := SubjectTarget{}
	key := crypto.GetGlobalKey()
	origins := make([]string, 0, 2)
	if v := strings.TrimSpace(passport); v != "" {
		t.PassportHMAC = crypto.ComputeHMAC(v, key)
		origins = append(origins, "по паспорту")
	}
	if v := strings.TrimSpace(patent); v != "" {
		t.PatentHMAC = crypto.ComputeHMAC(v, key)
		origins = append(origins, "по патенту")
	}
	t.Origin = strings.Join(origins, " и ")
	return t
}

// SubjectTargetFromRegistry строит цель по записи реестра: читает готовые свёртки, не
// расшифровывая документы. Так оператор ищет человека, найденного в интерфейсе, не
// вводя номер паспорта руками.
func SubjectTargetFromRegistry(ctx context.Context, db *gorm.DB, registryID int) (SubjectTarget, error) {
	var row struct {
		PassportSeriesNumberHMAC *string
		PatentNumberHMAC         *string
	}
	err := db.WithContext(ctx).Table("unique_employees").
		Select("passport_series_number_hmac, patent_number_hmac").
		Where("id = ?", registryID).Scan(&row).Error
	if err != nil {
		return SubjectTarget{}, fmt.Errorf("чтение записи реестра %d: %w", registryID, err)
	}

	t := SubjectTarget{Origin: fmt.Sprintf("по записи реестра %d", registryID)}
	if row.PassportSeriesNumberHMAC != nil {
		t.PassportHMAC = *row.PassportSeriesNumberHMAC
	}
	if row.PatentNumberHMAC != nil {
		t.PatentHMAC = *row.PatentNumberHMAC
	}
	return t, nil
}

// SubjectCandidate - запись, похожая на цель по имени. Не часть графа: список нужен,
// чтобы человек решил, тот ли это работник, и запустил сбор по его документу.
type SubjectCandidate struct {
	Source   string
	ID       int
	FullName string
	// HasDocument - есть ли у записи свёртка паспорта. Без документа склеить человека
	// не по чему: команда show по такой записи работать не сможет.
	HasDocument bool
}

// FindSubjectCandidatesByFIO ищет записи с таким же именем в реестре и среди участников
// заявок. Именно ищет, а не склеивает: см. правило пакета выше.
//
// Отчество необязательно: в запросе государственного органа его часто нет вовсе, и
// поиск, требующий полного совпадения тройки, не нашёл бы человека, который в системе
// заведён с отчеством. Поэтому сравниваются те части, что заданы.
func FindSubjectCandidatesByFIO(ctx context.Context, db *gorm.DB, last, first, middle string) ([]SubjectCandidate, error) {
	last, first, middle = normalize.Name(last), normalize.Name(first), normalize.Name(middle)
	if last == "" || first == "" {
		return nil, nil
	}

	// Нормализация повторяет normalize.Name: нижний регистр, «ё» как «е», обрезанные
	// края. Иначе «Пётр» не нашёл бы «Петра», а список кандидатов пустел бы ровно там,
	// где он нужнее всего.
	const norm = "TRIM(LOWER(REPLACE(COALESCE(%s, ''), 'ё', 'е')))"
	where := fmt.Sprintf(norm, "last_name") + " = @last AND " + fmt.Sprintf(norm, "first_name") + " = @first"
	if middle != "" {
		where += " AND " + fmt.Sprintf(norm, "middle_name") + " = @middle"
	}

	out := make([]SubjectCandidate, 0)
	for _, src := range []struct{ table, label string }{
		{"unique_employees", "реестр"},
		{"employees", "участник заявки"},
	} {
		var rows []struct {
			ID                       int
			FullName                 string
			PassportSeriesNumberHMAC *string
		}
		q := fmt.Sprintf(`SELECT id,
				TRIM(CONCAT_WS(' ', last_name, first_name, middle_name)) AS full_name,
				passport_series_number_hmac
			FROM %s WHERE %s ORDER BY id`, src.table, where)
		err := db.WithContext(ctx).Raw(q,
			sql.Named("last", last), sql.Named("first", first), sql.Named("middle", middle)).
			Scan(&rows).Error
		if err != nil {
			return nil, fmt.Errorf("поиск по имени в %s: %w", src.table, err)
		}
		for _, r := range rows {
			out = append(out, SubjectCandidate{
				Source:      src.label,
				ID:          r.ID,
				FullName:    r.FullName,
				HasDocument: r.PassportSeriesNumberHMAC != nil && *r.PassportSeriesNumberHMAC != "",
			})
		}
	}
	return out, nil
}

// Условие принадлежности строки субъекту. Пустая свёртка отсекается явно: без
// проверки `@pass <> ”` цель, у которой известен только патент, собрала бы все
// строки с пустым паспортом.
const subjectDocs = `((passport_series_number_hmac = @pass AND @pass <> '') ` +
	`OR (patent_number_hmac = @patent AND @patent <> ''))`

// subjectDocsFor - то же условие для таблицы под псевдонимом: в запросах справки
// участвует несколько таблиц сразу, и без псевдонима имя столбца неоднозначно.
func subjectDocsFor(alias string) string {
	if alias == "" {
		return subjectDocs
	}
	return fmt.Sprintf(`((%[1]s.passport_series_number_hmac = @pass AND @pass <> '') `+
		`OR (%[1]s.patent_number_hmac = @patent AND @patent <> ''))`, alias)
}

const (
	subjEmployees = "SELECT id FROM employees WHERE " + subjectDocs
	subjUnique    = "SELECT id FROM unique_employees WHERE " + subjectDocs
)

// subjectNodes - граф субъекта в порядке «дети раньше родителей».
//
// Список ссылающихся таблиц проверен по information_schema: на `employees.id`
// смотрят ровно две (`employee_files`, `employee_target_tables`), на
// `unique_employees.id` и `application_employees.id` - ни одна. Гарантия держится
// тестом TestSubjectGraph_NoUnaccountedCascades.
func subjectNodes() []Node {
	return []Node{
		// Файлы работника и его привязки к постам - дети строки заявки.
		{"employee_files", "employee_id IN (" + subjEmployees + ")"},
		{"employee_target_tables", "employee_id IN (" + subjEmployees + ")"},
		// Сами строки: участник заявки, участник в шапке заявки, запись реестра.
		{"employees", subjectDocs},
		{"application_employees", subjectDocs},
		{"unique_employees", subjectDocs},
		// История и отметки прохода. Связь не по внешнему ключу, а по паре
		// (entity_type, entity_id) - именно здесь лежат entry и exit, то самое
		// «когда приходил», ради чего операцию и заводят.
		{"audit_log", "(entity_type = 'employee' AND entity_id IN (" + subjEmployees + ")) " +
			"OR (entity_type = 'unique_employee' AND entity_id IN (" + subjUnique + "))"},
	}
}

// CollectSubject считает строки графа субъекта. Только SELECT count(*), база не
// меняется. Пустые узлы в результат не попадают.
func CollectSubject(ctx context.Context, db *gorm.DB, target SubjectTarget) (Graph, error) {
	if target.Empty() {
		return Graph{}, fmt.Errorf("цель не задана: нужен паспорт или патент")
	}

	g := Graph{Type: TypeSubject}
	for _, node := range subjectNodes() {
		var rows int64
		q := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s", node.Table, node.Where)
		err := db.WithContext(ctx).Raw(q,
			sql.Named("pass", target.PassportHMAC),
			sql.Named("patent", target.PatentHMAC)).Scan(&rows).Error
		if err != nil {
			return Graph{}, fmt.Errorf("подсчёт строк %s: %w", node.Table, err)
		}
		if rows > 0 {
			g.Tables = append(g.Tables, TableCount{Table: node.Table, Rows: rows})
		}
	}
	return g, nil
}

// SubjectApplications - заявки, в которых человек участвует. Не часть графа: заявка
// принадлежит организации, а не работнику, и уносить её с человеком нельзя. Но в
// ответе органу она главное - именно по ней видно, когда и куда человек приходил.
func SubjectApplications(ctx context.Context, db *gorm.DB, target SubjectTarget) ([]models.Application, error) {
	if target.Empty() {
		return nil, fmt.Errorf("цель не задана: нужен паспорт или патент")
	}

	var apps []models.Application
	err := db.WithContext(ctx).
		Where("id IN (SELECT a.application_id FROM attachments a WHERE a.id IN ("+
			"SELECT attachment_id FROM employees WHERE "+subjectDocs+"))",
			sql.Named("pass", target.PassportHMAC), sql.Named("patent", target.PatentHMAC)).
		Order("id DESC").
		Find(&apps).Error
	if err != nil {
		return nil, fmt.Errorf("заявки субъекта: %w", err)
	}
	return apps, nil
}
