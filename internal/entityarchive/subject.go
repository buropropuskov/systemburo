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

// SubjectTargetFromEmployee строит цель по строке заявки. Нужна тем, у кого записи
// реестра нет вовсе: человек попал в систему одной подачей и живёт только в заявке.
// Таких на стенде 22, и запрос государственного органа может прийти именно о нём.
func SubjectTargetFromEmployee(ctx context.Context, db *gorm.DB, employeeID int) (SubjectTarget, error) {
	var row struct {
		PassportSeriesNumberHMAC *string
		PatentNumberHMAC         *string
	}
	err := db.WithContext(ctx).Table("employees").
		Select("passport_series_number_hmac, patent_number_hmac").
		Where("id = ?", employeeID).Scan(&row).Error
	if err != nil {
		return SubjectTarget{}, fmt.Errorf("чтение строки заявки %d: %w", employeeID, err)
	}

	t := SubjectTarget{Origin: fmt.Sprintf("по строке заявки %d", employeeID)}
	if row.PassportSeriesNumberHMAC != nil {
		t.PassportHMAC = *row.PassportSeriesNumberHMAC
	}
	if row.PatentNumberHMAC != nil {
		t.PatentHMAC = *row.PatentNumberHMAC
	}
	return t, nil
}

// SubjectCandidate - один человек, найденный по имени: строки склеены по документу.
//
// Раньше список отдавал каждую строку отдельно, и по одному человеку выходило десять
// пунктов: три записи реестра и семь упоминаний в заявках. Разбираться в них негде -
// решение принимают про ЧЕЛОВЕКА, а не про строку. Поэтому одна запись списка = один
// документ, а сколько за ней строк, видно счётчиками.
type SubjectCandidate struct {
	FullName string
	// RegistryID - запись реестра, если она есть. Ноль означает, что человек
	// встречается только в заявках; собирать тогда нужно от строки заявки.
	RegistryID int
	// EmployeeID - строка заявки, от которой можно собрать сведения, когда записи
	// реестра нет. Таких людей на стенде 22 - без них по ним нельзя ответить органу.
	EmployeeID int
	// RegistryRows/ApplicationRows - сколько строк за этим человеком.
	RegistryRows    int
	ApplicationRows int
	// Fuzzy - запись найдена по ПОХОЖЕМУ написанию, а не точному совпадению имени.
	// Показывать это обязательно: оператор должен знать, что система домыслила за
	// него, иначе в ответ государственному органу уедет однофамилец с другой фамилией.
	Fuzzy bool
	// HasDocument - есть ли документ. Без него склеить человека не по чему, и такая
	// строка остаётся в списке лишь затем, чтобы человек увидел: запись есть, но
	// собрать по ней нечего.
	HasDocument bool
	// Organization и Position - чем эти люди отличаются друг от друга. Без них список
	// однофамильцев выглядит как три одинаковые строки, и выбрать не из чего.
	Organization string
	Position     string
	// DocumentTail - последние четыре знака документа. Весь номер в списке показывать
	// незачем: различить людей хватает хвоста, а список видят все, у кого есть доступ
	// к разделу.
	DocumentTail string
}

// FindSubjectCandidatesByFIO ищет людей с таким именем, склеивая строки по документу.
//
// Именно ищет, а не решает за человека: см. правило пакета выше. Отчество
// необязательно - в запросе государственного органа его часто нет вовсе, и поиск,
// требующий полного совпадения тройки, не нашёл бы человека, заведённого с отчеством.
func FindSubjectCandidatesByFIO(ctx context.Context, db *gorm.DB, last, first, middle string) ([]SubjectCandidate, error) {
	last, first, middle = normalize.Name(last), normalize.Name(first), normalize.Name(middle)
	if last == "" || first == "" {
		return nil, nil
	}

	whereFor := func(alias string) string { return nameWhere(alias, middle != "") }

	type row struct {
		Source       string
		ID           int
		FullName     string
		DocKey       string
		Document     *string
		Organization *string
		Position     *string
	}
	var rows []row
	q := fmt.Sprintf(`
		SELECT 'registry' AS source, ue.id,
			TRIM(CONCAT_WS(' ', ue.last_name, ue.first_name, ue.middle_name)) AS full_name,
			COALESCE(NULLIF(ue.passport_series_number_hmac, ''), NULLIF(ue.patent_number_hmac, ''), '') AS doc_key,
			COALESCE(ue.passport_series_number, ue.patent_number) AS document,
			o.name AS organization, ue."position" AS position
		FROM unique_employees ue
		LEFT JOIN organizations o ON o.id = ue.organization_id
		WHERE %[1]s
		UNION ALL
		SELECT 'application', e.id,
			TRIM(CONCAT_WS(' ', e.last_name, e.first_name, e.middle_name)),
			COALESCE(NULLIF(e.passport_series_number_hmac, ''), NULLIF(e.patent_number_hmac, ''), ''),
			COALESCE(e.passport_series_number, e.patent_number),
			o2.name, e."position"
		FROM employees e
		LEFT JOIN attachments a ON a.id = e.attachment_id
		LEFT JOIN applications app ON app.id = a.application_id
		LEFT JOIN organizations o2 ON o2.id = app.organization_id
		WHERE %[2]s
		ORDER BY 1, 2`, whereFor("ue"), whereFor("e"))
	err := db.WithContext(ctx).Raw(q,
		sql.Named("last", last), sql.Named("first", first), sql.Named("middle", middle)).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("поиск по имени: %w", err)
	}

	// Ключ склейки - свёртка документа. Строку без документа склеивать не по чему,
	// поэтому она остаётся сама по себе.
	order := make([]string, 0, len(rows))
	byKey := make(map[string]*SubjectCandidate, len(rows))
	for _, r := range rows {
		key := r.DocKey
		if key == "" {
			key = fmt.Sprintf("%s-%d", r.Source, r.ID)
		}
		c, ok := byKey[key]
		if !ok {
			c = &SubjectCandidate{
				FullName:     r.FullName,
				HasDocument:  r.DocKey != "",
				Organization: derefOrEmpty(r.Organization),
				Position:     derefOrEmpty(r.Position),
				DocumentTail: documentTail(r.Document),
			}
			byKey[key] = c
			order = append(order, key)
		}
		// Организацию и должность берём у первой строки, где они есть: у записи
		// реестра их может не быть, а у строки заявки они всегда от самой заявки.
		if c.Organization == "" {
			c.Organization = derefOrEmpty(r.Organization)
		}
		if c.Position == "" {
			c.Position = derefOrEmpty(r.Position)
		}
		if c.DocumentTail == "" {
			c.DocumentTail = documentTail(r.Document)
		}
		if r.Source == "registry" {
			c.RegistryRows++
			if c.RegistryID == 0 {
				c.RegistryID = r.ID
			}
			continue
		}
		c.ApplicationRows++
		if c.EmployeeID == 0 {
			c.EmployeeID = r.ID
		}
	}

	out := make([]SubjectCandidate, 0, len(order))
	for _, key := range order {
		out = append(out, *byKey[key])
	}
	if len(out) > 0 {
		return out, nil
	}

	// Точных совпадений нет - пробуем похожее написание. Опечатка в фамилии («Мякотних»
	// вместо «Мякотных») иначе означает «человека в системе нет», а он есть, и запрос
	// государственного органа остаётся без ответа. Домысливать молча нельзя: у каждой
	// такой записи стоит признак Fuzzy, и интерфейс говорит об этом прямо.
	return findSubjectCandidatesFuzzy(ctx, db, last, first, middle)
}

// findSubjectCandidatesFuzzy ищет по похожему написанию имени.
//
// Порог тот же, что у сквозного поиска (0.3): ниже начинают проходить общие триграммы,
// выше перестают ловиться настоящие опечатки в короткой фамилии.
func findSubjectCandidatesFuzzy(ctx context.Context, db *gorm.DB, last, first, middle string) ([]SubjectCandidate, error) {
	similar := func(alias string) string {
		return fmt.Sprintf("(@last %%>> %[1]s.last_name AND @first %%>> %[1]s.first_name)", alias)
	}

	type row struct {
		Source       string
		ID           int
		FullName     string
		DocKey       string
		Document     *string
		Organization *string
		Position     *string
	}
	var rows []row
	q := fmt.Sprintf(`
		SELECT 'registry' AS source, ue.id,
			TRIM(CONCAT_WS(' ', ue.last_name, ue.first_name, ue.middle_name)) AS full_name,
			COALESCE(NULLIF(ue.passport_series_number_hmac, ''), NULLIF(ue.patent_number_hmac, ''), '') AS doc_key,
			COALESCE(ue.passport_series_number, ue.patent_number) AS document,
			o.name AS organization, ue."position" AS position
		FROM unique_employees ue
		LEFT JOIN organizations o ON o.id = ue.organization_id
		WHERE %[1]s
		UNION ALL
		SELECT 'application', e.id,
			TRIM(CONCAT_WS(' ', e.last_name, e.first_name, e.middle_name)),
			COALESCE(NULLIF(e.passport_series_number_hmac, ''), NULLIF(e.patent_number_hmac, ''), ''),
			COALESCE(e.passport_series_number, e.patent_number),
			o2.name, e."position"
		FROM employees e
		LEFT JOIN attachments a ON a.id = e.attachment_id
		LEFT JOIN applications app ON app.id = a.application_id
		LEFT JOIN organizations o2 ON o2.id = app.organization_id
		WHERE %[2]s
		ORDER BY 1, 2
		LIMIT 50`, similar("ue"), similar("e"))

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Порог ставится на транзакцию: значение из postgresql.conf не подействует на
		// уже открытые соединения пула, и поиск вёл бы себя по-разному.
		if err := tx.Exec("SET LOCAL pg_trgm.strict_word_similarity_threshold = 0.3").Error; err != nil {
			return fmt.Errorf("выставить порог нечёткого поиска: %w", err)
		}
		return tx.Raw(q, sql.Named("last", last), sql.Named("first", first)).Scan(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("поиск по похожему имени: %w", err)
	}

	order := make([]string, 0, len(rows))
	byKey := make(map[string]*SubjectCandidate, len(rows))
	for _, r := range rows {
		key := r.DocKey
		if key == "" {
			key = fmt.Sprintf("%s-%d", r.Source, r.ID)
		}
		c, ok := byKey[key]
		if !ok {
			c = &SubjectCandidate{
				FullName:     r.FullName,
				HasDocument:  r.DocKey != "",
				Fuzzy:        true,
				Organization: derefOrEmpty(r.Organization),
				Position:     derefOrEmpty(r.Position),
				DocumentTail: documentTail(r.Document),
			}
			byKey[key] = c
			order = append(order, key)
		}
		if r.Source == "registry" {
			c.RegistryRows++
			if c.RegistryID == 0 {
				c.RegistryID = r.ID
			}
			continue
		}
		c.ApplicationRows++
		if c.EmployeeID == 0 {
			c.EmployeeID = r.ID
		}
	}

	out := make([]SubjectCandidate, 0, len(order))
	for _, key := range order {
		out = append(out, *byKey[key])
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

// nameWhere - условие совпадения по имени для таблицы под псевдонимом.
//
// Нормализация повторяет normalize.Name: нижний регистр, «ё» как «е», обрезанные
// края. Иначе «Пётр» не нашёл бы «Петра». Отчество сравнивается, только если задано:
// в запросе государственного органа его часто нет.
func nameWhere(alias string, withMiddle bool) string {
	norm := func(col string) string {
		return fmt.Sprintf("TRIM(LOWER(REPLACE(COALESCE(%s.%s, ''), 'ё', 'е')))", alias, col)
	}
	where := norm("last_name") + " = @last AND " + norm("first_name") + " = @first"
	if withMiddle {
		where += " AND " + norm("middle_name") + " = @middle"
	}
	return where
}

func derefOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

// documentTail - последние четыре знака документа, чтобы отличить однофамильцев.
// Номер целиком в списке не нужен: список открывают, чтобы выбрать человека, а не
// чтобы прочитать его паспорт.
func documentTail(v *string) string {
	if v == nil {
		return ""
	}
	plain := crypto.DecryptOptional(v)
	if plain == nil {
		return ""
	}
	digits := []rune(strings.TrimSpace(*plain))
	if len(digits) <= 4 {
		return string(digits)
	}
	return string(digits[len(digits)-4:])
}
