package entityarchive

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"

	"systemburo/internal/crypto"
	"systemburo/internal/export"
)

// Справка о том, что система знает о человеке (#2356).
//
// Файл прикладывают к официальному ответу государственному органу либо отдают самому
// человеку, поэтому он обязан читаться без пояснений и отвечать на четыре вопроса:
// что хранится, где, с какого времени и на каком основании. Отсюда разделы вместо
// одной плоской таблицы: сведения, участие в заявках, проходы, привязки к постам.
//
// Основание обработки одно для всех строк - законный интерес оператора (п. 7 ч. 1
// ст. 6 152-ФЗ): пропускной режим на объекте. Согласие субъекта в этой схеме не
// собирается, работника уведомляют; отметка уведомления и возражение против обработки
// печатаются рядом со сведениями, потому что именно они меняют режим работы с записью.

// Основание обработки в тексте справки. Держится здесь, а не в шаблоне: формулировка
// юридическая, и менять её вместе с вёрсткой нельзя.
const subjectProcessingBasis = "Законный интерес оператора, п. 7 ч. 1 ст. 6 152-ФЗ: пропускной режим на объекте"

// SubjectProcessingBasis - основание обработки для показа на экране. Экспортируется,
// чтобы интерфейс печатал ту же формулировку, что и файл: расхождение здесь означало
// бы, что человеку и проверяющему система говорит разное.
func SubjectProcessingBasis() string { return subjectProcessingBasis }

// SubjectReport - собранные разделы справки.
type SubjectReport struct {
	Origin   string
	MadeAt   time.Time
	Sections []export.Table
}

// BuildSubjectReport собирает справку по человеку. Только чтение.
func BuildSubjectReport(ctx context.Context, db *gorm.DB, target SubjectTarget) (SubjectReport, error) {
	if target.Empty() {
		return SubjectReport{}, fmt.Errorf("цель не задана: нужен паспорт или патент")
	}

	rep := SubjectReport{Origin: target.Origin, MadeAt: time.Now().UTC()}
	subtitle := fmt.Sprintf("Составлено %s. Основание обработки: %s",
		rep.MadeAt.Format("02.01.2006 15:04 MST"), subjectProcessingBasis)

	for _, build := range []func(context.Context, *gorm.DB, SubjectTarget, string) (export.Table, error){
		buildSubjectPersonalSection,
		buildSubjectApplicationsSection,
		buildSubjectPassagesSection,
		buildSubjectPostsSection,
	} {
		section, err := build(ctx, db, target, subtitle)
		if err != nil {
			return SubjectReport{}, err
		}
		rep.Sections = append(rep.Sections, section)
	}
	return rep, nil
}

// buildSubjectPersonalSection - что о человеке хранится и с какого времени.
func buildSubjectPersonalSection(ctx context.Context, db *gorm.DB, target SubjectTarget, subtitle string) (export.Table, error) {
	type row struct {
		Source       string
		ID           int
		FullName     string
		Position     *string
		Citizenship  *string
		Organization *string
		Passport     *string
		Patent       *string
		Permission   *string
		CreatedAt    time.Time
		NoticeAt     *time.Time
		ObjectionAt  *time.Time
	}

	// Обе таблицы читаются одним запросом: получателю справки безразлично, лежит
	// строка в реестре или в заявке, ему важно, что о человеке хранится.
	q := `
		SELECT 'Реестр сотрудников' AS source, ue.id,
			TRIM(CONCAT_WS(' ', ue.last_name, ue.first_name, ue.middle_name)) AS full_name,
			ue."position", ci.name AS citizenship, o.name AS organization,
			ue.passport_series_number AS passport, ue.patent_number AS patent,
			ue.other_permission AS permission,
			ue.created_at, ue.pd_consent_at AS notice_at, ue.pd_objection_at AS objection_at
		FROM unique_employees ue
		LEFT JOIN citizenships ci ON ci.id = ue.citizenship_id
		LEFT JOIN organizations o ON o.id = ue.organization_id
		WHERE ` + subjectDocsFor("ue") + `
		UNION ALL
		SELECT 'Участник заявки', e.id,
			TRIM(CONCAT_WS(' ', e.last_name, e.first_name, e.middle_name)),
			e."position", ci.name, o.name,
			e.passport_series_number, e.patent_number, e.other_permission,
			e.created_at, e.pd_consent_at, NULL
		FROM employees e
		LEFT JOIN citizenships ci ON ci.id = e.citizenship_id
		LEFT JOIN attachments a ON a.id = e.attachment_id
		LEFT JOIN applications app ON app.id = a.application_id
		LEFT JOIN organizations o ON o.id = app.organization_id
		WHERE ` + subjectDocsFor("e") + `
		ORDER BY 1, 2`

	var rows []row
	err := db.WithContext(ctx).Raw(q,
		sql.Named("pass", target.PassportHMAC), sql.Named("patent", target.PatentHMAC)).Scan(&rows).Error
	if err != nil {
		return export.Table{}, fmt.Errorf("сведения о человеке: %w", err)
	}

	table := export.Table{
		Title:    "Сведения",
		Subtitle: subtitle,
		Headers: []string{"Источник", "Запись", "ФИО", "Должность", "Гражданство", "Организация",
			"Паспорт", "Патент", "Иное разрешение", "Хранится с", "Уведомление", "Возражение"},
	}
	for _, r := range rows {
		table.Rows = append(table.Rows, []string{
			r.Source, fmt.Sprint(r.ID), r.FullName,
			derefOrDash(r.Position), derefOrDash(r.Citizenship), derefOrDash(r.Organization),
			decryptedOrDash(r.Passport), decryptedOrDash(r.Patent), decryptedOrDash(r.Permission),
			r.CreatedAt.Format("02.01.2006"),
			timeOrDash(r.NoticeAt), timeOrDash(r.ObjectionAt),
		})
	}
	return table, nil
}

// buildSubjectApplicationsSection - в каких заявках человек участвует. Заявка
// принадлежит организации, а не ему, но именно по ней видно, куда он приходил.
func buildSubjectApplicationsSection(ctx context.Context, db *gorm.DB, target SubjectTarget, subtitle string) (export.Table, error) {
	type row struct {
		Number       *string
		Status       *string
		Organization *string
		Company      *string
		SentAt       *time.Time
		// Даты доступа во вложении хранятся строкой, а не датой (attachments.
		// entry_date_from - character varying). Читаем как есть: попытка положить их
		// в time.Time роняет весь раздел, и справка не собирается вовсе - поймано на
		// стенде, тесты этого не видели, потому что вложения в них создавались без дат.
		DateFrom *string
		DateTo   *string
	}

	q := `
		SELECT app.application_number AS number, app.status, o.name AS organization, c.name AS company,
			app.sending_datetime AS sent_at, a.entry_date_from AS date_from, a.entry_date_to AS date_to
		FROM employees e
		JOIN attachments a ON a.id = e.attachment_id
		JOIN applications app ON app.id = a.application_id
		LEFT JOIN organizations o ON o.id = app.organization_id
		LEFT JOIN companies c ON c.id = app.company_id
		WHERE ` + subjectDocsFor("e") + `
		ORDER BY app.id DESC`

	var rows []row
	err := db.WithContext(ctx).Raw(q,
		sql.Named("pass", target.PassportHMAC), sql.Named("patent", target.PatentHMAC)).Scan(&rows).Error
	if err != nil {
		return export.Table{}, fmt.Errorf("заявки человека: %w", err)
	}

	table := export.Table{
		Title:    "Заявки",
		Subtitle: subtitle,
		Headers:  []string{"Номер", "Статус", "Организация", "Компания", "Подана", "Доступ с", "Доступ по"},
	}
	for _, r := range rows {
		table.Rows = append(table.Rows, []string{
			derefOrDash(r.Number), derefOrDash(r.Status), derefOrDash(r.Organization), derefOrDash(r.Company),
			dateOrDash(r.SentAt), isoDateOrDash(r.DateFrom), isoDateOrDash(r.DateTo),
		})
	}
	return table, nil
}

// buildSubjectPassagesSection - когда человек приходил и куда. Ради этого раздела
// операцию и заводят: именно его спрашивает государственный орган.
func buildSubjectPassagesSection(ctx context.Context, db *gorm.DB, target SubjectTarget, subtitle string) (export.Table, error) {
	type row struct {
		At     time.Time
		Action string
		Post   *string
		Actor  *string
	}

	q := `
		SELECT al.created_at AS at, al.action,
			COALESCE(NULLIF(st.display_name, ''), st.name) AS post,
			TRIM(CONCAT_WS(' ', u.last_name, u.first_name)) AS actor
		FROM audit_log al
		LEFT JOIN system_tables st ON st.id = (al.details->>'table_id')::int
		LEFT JOIN users u ON u.id = al.actor_user_id
		WHERE al.entity_type = 'employee'
		  AND al.action IN ('entry', 'exit')
		  AND al.entity_id IN (SELECT id FROM employees WHERE ` + subjectDocs + `)
		ORDER BY al.created_at DESC`

	var rows []row
	err := db.WithContext(ctx).Raw(q,
		sql.Named("pass", target.PassportHMAC), sql.Named("patent", target.PatentHMAC)).Scan(&rows).Error
	if err != nil {
		return export.Table{}, fmt.Errorf("проходы человека: %w", err)
	}

	table := export.Table{
		Title:    "Проходы",
		Subtitle: subtitle,
		Headers:  []string{"Дата и время", "Событие", "Пост", "Отметил"},
	}
	for _, r := range rows {
		event := "вход на территорию"
		if r.Action == "exit" {
			event = "выход с территории"
		}
		// Пост в отметке появился не сразу: у старых записей его нет вовсе, и пустое
		// место в справке читается как «человек прошёл неизвестно где». Пишем прямо,
		// что пост не записан - это разные вещи, и в ответе органу это важно.
		post := "не записан"
		if r.Post != nil && strings.TrimSpace(*r.Post) != "" {
			post = *r.Post
		}
		table.Rows = append(table.Rows, []string{
			r.At.Format("02.01.2006 15:04"), event, post, derefOrDash(r.Actor),
		})
	}
	return table, nil
}

// buildSubjectPostsSection - к каким постам человек привязан сейчас.
func buildSubjectPostsSection(ctx context.Context, db *gorm.DB, target SubjectTarget, subtitle string) (export.Table, error) {
	type row struct {
		Post   string
		Number *string
	}

	q := `
		SELECT COALESCE(NULLIF(st.display_name, ''), st.name) AS post, app.application_number AS number
		FROM employee_target_tables ett
		JOIN system_tables st ON st.id = ett.table_id
		JOIN employees e ON e.id = ett.employee_id
		LEFT JOIN attachments a ON a.id = e.attachment_id
		LEFT JOIN applications app ON app.id = a.application_id
		WHERE ` + subjectDocsFor("e") + `
		ORDER BY post`

	var rows []row
	err := db.WithContext(ctx).Raw(q,
		sql.Named("pass", target.PassportHMAC), sql.Named("patent", target.PatentHMAC)).Scan(&rows).Error
	if err != nil {
		return export.Table{}, fmt.Errorf("посты человека: %w", err)
	}

	table := export.Table{
		Title:    "Посты",
		Subtitle: subtitle,
		Headers:  []string{"Пост", "Заявка"},
	}
	for _, r := range rows {
		table.Rows = append(table.Rows, []string{r.Post, derefOrDash(r.Number)})
	}
	return table, nil
}

func derefOrDash(v *string) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return "-"
	}
	return *v
}

func dateOrDash(v *time.Time) string {
	if v == nil {
		return "-"
	}
	return v.Format("02.01.2006")
}

func timeOrDash(v *time.Time) string {
	if v == nil {
		return "-"
	}
	return v.Format("02.01.2006 15:04")
}

// decryptedOrDash расшифровывает документ, прочитанный сырым запросом: строки справки
// читает человек, а в базе они лежат шифротекстом. Тот же приём, что в выдачах реестра
// (#2413) - в списке полей здесь обязаны быть ВСЕ шифруемые документы.
func decryptedOrDash(v *string) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return "-"
	}
	return derefOrDash(crypto.DecryptOptional(v))
}

// WriteSubjectReport кладёт справку в каталог root двумя файлами: .xlsx для работы и
// .pdf для приложения к официальному ответу. Возвращает пути записанных файлов.
//
// Каталог тот же, что у пакетов по организации (ENTITY_EXPORT_PATH), и по той же
// причине: в файлах лежат персональные данные целиком, и место их хранения выбирает
// владелец системы, а не программа.
func WriteSubjectReport(root string, rep SubjectReport) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("не задан каталог выгрузки")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("каталог выгрузки %s: %w", root, err)
	}

	base := filepath.Join(root, fmt.Sprintf("subject-%s", rep.MadeAt.Format("20060102-150405")))
	xlsx, err := export.ToXLSXMulti(rep.Sections)
	if err != nil {
		return nil, fmt.Errorf("сборка xlsx: %w", err)
	}
	pdf, err := export.ToPDFMulti(rep.Sections)
	if err != nil {
		return nil, fmt.Errorf("сборка pdf: %w", err)
	}

	written := make([]string, 0, 2)
	for _, f := range []struct {
		path string
		data []byte
	}{{base + ".xlsx", xlsx}, {base + ".pdf", pdf}} {
		// 0600: справка содержит персональные данные целиком, читать её может только
		// владелец процесса.
		if err := os.WriteFile(f.path, f.data, 0o600); err != nil {
			return nil, fmt.Errorf("запись %s: %w", f.path, err)
		}
		written = append(written, f.path)
	}
	return written, nil
}

// SubjectReportRowCount - сколько строк во всех разделах справки. Нужен команде, чтобы
// показать объём до записи файлов.
func SubjectReportRowCount(rep SubjectReport) int {
	n := 0
	for _, s := range rep.Sections {
		n += len(s.Rows)
	}
	return n
}

// isoDateOrDash приводит дату, хранящуюся строкой, к принятому в системе виду
// 01.01.2026. В базе даты доступа лежат как «2026-06-11» (столбец character varying),
// и в справке это единственное место, где формат отличался бы от остального вывода.
func isoDateOrDash(v *string) string {
	if v == nil || strings.TrimSpace(*v) == "" {
		return "-"
	}
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(*v))
	if err != nil {
		// Формат не тот, что ожидали: отдаём как есть, а не прячем значение.
		return *v
	}
	return parsed.Format("02.01.2006")
}
