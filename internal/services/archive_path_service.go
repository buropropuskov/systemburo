package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"systemburo/internal/blankpath"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// archiveRelPathMaxBytes - предел относительного пути бланка внутри архива. Взят от
// windows-ского MAX_PATH в 260 символов с запасом на корень сетевой папки
// (\\сервер\архив\...): файл, который не открывается по сети двойным щелчком, не
// выполняет требования, ради которого архив и заводится.
const archiveRelPathMaxBytes = 200

// blankFileExt - расширение выгружаемого бланка. Генератор отдаёт xlsx и только его.
const blankFileExt = ".xlsx"

// ArchivePathService собирает значения плейсхолдеров из заявки и раскладывает по ним
// шаблоны файлового архива (#1615). Отделён от записи на диск: тем же расчётом
// пользуются и живое превью в настройках, и писатель, и ночная сверка - иначе
// показанный администратору путь и путь на диске разъехались бы.
type ArchivePathService struct {
	db *gorm.DB
	// loc - рабочая таймзона. Каталог дня считается по местной дате: заявка, поданная
	// в 23:30 МСК, хранится в UTC уже следующим днём и без приведения уехала бы в
	// каталог завтрашнего числа.
	loc *time.Location
}

// NewArchivePathService создаёт сервис путей архива. Таймзона - та же, по которой
// живут суточные операции системы (RESET_TIMEZONE).
func NewArchivePathService(db *gorm.DB, loc *time.Location) *ArchivePathService {
	if loc == nil {
		loc = time.UTC
	}
	return &ArchivePathService{db: db, loc: loc}
}

// archiveValuesRow - плоский приёмник запроса значений. Плоский намеренно: gorm молча
// не заполняет анонимно встроенные структуры, и поля пришли бы нулями.
type archiveValuesRow struct {
	ApplicationID     int        `gorm:"column:application_id"`
	ApplicationNumber string     `gorm:"column:application_number"`
	SendingDatetime   *time.Time `gorm:"column:sending_datetime"`
	Status            string     `gorm:"column:status"`
	Confirmation      string     `gorm:"column:confirmation"`
	InitiatorName     string     `gorm:"column:initiator_name"`
	Organization      string     `gorm:"column:organization"`
	Company           string     `gorm:"column:company"`
	Sender            string     `gorm:"column:sender"`
}

// archiveAttachmentRow - данные вложения для имени файла.
type archiveAttachmentRow struct {
	AttachmentID   int    `gorm:"column:attachment_id"`
	AttachmentType string `gorm:"column:attachment_type"`
	EntryDateFrom  string `gorm:"column:entry_date_from"`
	EntryDateTo    string `gorm:"column:entry_date_to"`
}

// Values собирает значения плейсхолдеров для пары «заявка + вложение». attachmentID
// равный нулю означает, что нужны только поля заявки (уровни каталогов).
func (s *ArchivePathService) Values(ctx context.Context, applicationID, attachmentID int) (blankpath.Values, error) {
	const sql = `
		SELECT a.id AS application_id,
		       COALESCE(a.application_number, '') AS application_number,
		       a.sending_datetime AS sending_datetime,
		       COALESCE(a.status, '') AS status,
		       COALESCE(a.confirmation, '') AS confirmation,
		       COALESCE(a.initiator_name, '') AS initiator_name,
		       COALESCE(o.name, '') AS organization,
		       COALESCE(c.name, '') AS company,
		       COALESCE(format_short_name(u.last_name, u.first_name, u.middle_name), '') AS sender
		FROM applications a
		LEFT JOIN organizations o ON o.id = a.organization_id
		LEFT JOIN companies c ON c.id = a.company_id
		LEFT JOIN users u ON u.id = a.sender_user_id
		WHERE a.id = ?`

	var row archiveValuesRow
	if err := s.db.WithContext(ctx).Raw(sql, applicationID).Scan(&row).Error; err != nil {
		return blankpath.Values{}, fmt.Errorf("failed to load archive values: %w", err)
	}
	if row.ApplicationID == 0 {
		return blankpath.Values{}, echo.NewHTTPError(http.StatusNotFound, "Заявка не найдена")
	}

	values := blankpath.Values{
		Date:          s.bucketDate(row.SendingDatetime),
		Number:        row.ApplicationNumber,
		ApplicationID: row.ApplicationID,
		Sender:        row.Sender,
		Initiator:     row.InitiatorName,
		Status:        row.Status,
		Confirmation:  row.Confirmation,
		Organization:  row.Organization,
		Company:       row.Company,
	}
	if attachmentID <= 0 {
		return values, nil
	}

	att, err := s.attachmentValues(ctx, applicationID, attachmentID)
	if err != nil {
		return blankpath.Values{}, err
	}
	values.AttachmentID = att.AttachmentID
	values.AttachmentType = att.AttachmentType
	values.Period = formatArchivePeriod(att.EntryDateFrom, att.EntryDateTo)
	return values, nil
}

// archiveAttachmentNameExpr - наименование вложения так, как оно попадает в имя
// файла: сперва справочник (администратор видит именно его), затем копия имени,
// осевшая на строке заявки в момент подачи. Вынесено в константу, потому что по
// этой же формуле считается разбивка сводки по типам вложений: две копии
// разъехались бы при первой правке, и раздел показывал бы одни названия, а диск
// хранил другие. Требует алиасов `at` (attachments) и `ua` (unique_attachments).
const archiveAttachmentNameExpr = `COALESCE(NULLIF(ua.display_name, ''), NULLIF(ua.title, ''),
	                NULLIF(at.attachment_display_name, ''), NULLIF(at.attachment_name, ''), '')`

// attachmentValues читает тип и срок вложения. Тип берётся с шаблона вложения, а не
// с самого вложения: в имени файла нужно то же название, которое администратор видит
// в справочнике, а на строке заявки лежит его копия времени подачи.
func (s *ArchivePathService) attachmentValues(ctx context.Context, applicationID, attachmentID int) (archiveAttachmentRow, error) {
	sql := `
		SELECT at.id AS attachment_id,
		       ` + archiveAttachmentNameExpr + ` AS attachment_type,
		       COALESCE(at.entry_date_from, '') AS entry_date_from,
		       COALESCE(at.entry_date_to, '') AS entry_date_to
		FROM attachments at
		LEFT JOIN unique_attachments ua ON ua.id = at.unique_attachment_id
		WHERE at.id = ? AND at.application_id = ?`

	var row archiveAttachmentRow
	if err := s.db.WithContext(ctx).Raw(sql, attachmentID, applicationID).Scan(&row).Error; err != nil {
		return archiveAttachmentRow{}, fmt.Errorf("failed to load archive attachment values: %w", err)
	}
	if row.AttachmentID == 0 {
		return archiveAttachmentRow{}, echo.NewHTTPError(http.StatusNotFound, "Вложение не найдено")
	}
	return row, nil
}

// Location отдаёт рабочую таймзону раскладки. Нужна тем, кто задаёт границы
// периода по календарным датам: реестр и пути на диске живут по местной дате, и
// граница, посчитанная в UTC, отрезала бы не тот кусок суток.
func (s *ArchivePathService) Location() *time.Location { return s.loc }

// BucketDate возвращает дату каталога заявки - местную дату подачи.
func (s *ArchivePathService) BucketDate(sending *time.Time) time.Time {
	return s.bucketDate(sending)
}

// bucketDate приводит момент подачи к местной дате. У заявок без даты подачи (черновик
// в базе) берётся сегодняшняя: файл всё равно должен куда-то лечь, а «нулевой год» в
// пути читался бы как поломка.
func (s *ArchivePathService) bucketDate(sending *time.Time) time.Time {
	base := time.Now()
	if sending != nil && !sending.IsZero() {
		base = *sending
	}
	local := base.In(s.loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.loc)
}

// RelPath раскладывает шаблоны в уровни каталогов и имя файла. Возвращает то же, что
// потом окажется на диске: санитайзер и подгонка длины уже применены.
func (s *ArchivePathService) RelPath(dirTemplate, fileTemplate string, values blankpath.Values) ([]string, string) {
	levels := blankpath.RenderPath(dirTemplate, values)
	fileName := blankpath.RenderName(fileTemplate, values, blankFileExt)
	return blankpath.FitRelPath(levels, fileName, archiveRelPathMaxBytes), fileName
}

// Preview строит живое превью пути для конструктора шаблонов. Работает и на пустой
// базе: заявки может ещё не быть, а настроить раскладку администратору нужно до
// первой подачи - тогда превью собирается из значений-образцов и помечается synthetic.
func (s *ArchivePathService) Preview(ctx context.Context, dirTemplate, fileTemplate string, applicationID int) (*models.ArchivePreviewResponse, error) {
	resp := &models.ArchivePreviewResponse{
		DirProblems:  toTemplateIssues(blankpath.Check(dirTemplate, blankpath.ScopeDir)),
		FileProblems: toTemplateIssues(blankpath.Check(fileTemplate, blankpath.ScopeFile)),
	}

	values, number, synthetic, err := s.previewValues(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	resp.Synthetic = synthetic
	resp.ApplicationNumber = number

	levels, fileName := s.RelPath(dirTemplate, fileTemplate, values)
	resp.Levels = levels
	resp.FileName = fileName
	resp.RelPath = path.Join(append(append([]string{}, levels...), fileName)...)
	return resp, nil
}

// previewValues выбирает, на чём показывать превью: на явно указанной заявке, на
// последней поданной или на образце. Пустая база - штатный случай, а не ошибка.
func (s *ArchivePathService) previewValues(ctx context.Context, applicationID int) (blankpath.Values, string, bool, error) {
	if applicationID <= 0 {
		found, err := s.latestApplicationID(ctx)
		if err != nil {
			return blankpath.Values{}, "", false, err
		}
		if found == 0 {
			return syntheticValues(time.Now().In(s.loc)), "", true, nil
		}
		applicationID = found
	}

	attachmentID, err := s.firstAttachmentID(ctx, applicationID)
	if err != nil {
		return blankpath.Values{}, "", false, err
	}
	values, err := s.Values(ctx, applicationID, attachmentID)
	if err != nil {
		return blankpath.Values{}, "", false, err
	}
	// У заявки без вложений тип и срок показать неоткуда - подставляем образцовые,
	// иначе конструктор имени файла выглядел бы сломанным.
	if attachmentID == 0 {
		sample := syntheticValues(values.Date)
		values.AttachmentType = sample.AttachmentType
		values.Period = sample.Period
		values.AttachmentID = sample.AttachmentID
	}
	return values, values.Number, false, nil
}

func (s *ArchivePathService) latestApplicationID(ctx context.Context) (int, error) {
	var id int
	err := s.db.WithContext(ctx).
		Raw(`SELECT id FROM applications ORDER BY COALESCE(sending_datetime, NOW()) DESC, id DESC LIMIT 1`).
		Scan(&id).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("failed to find latest application: %w", err)
	}
	return id, nil
}

func (s *ArchivePathService) firstAttachmentID(ctx context.Context, applicationID int) (int, error) {
	var id int
	err := s.db.WithContext(ctx).
		Raw(`SELECT id FROM attachments WHERE application_id = ? ORDER BY id LIMIT 1`, applicationID).
		Scan(&id).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, fmt.Errorf("failed to find application attachment: %w", err)
	}
	return id, nil
}

// syntheticValues собирает пример из подписей реестра плейсхолдеров: администратор
// видит ровно те значения, которые перечислены в палитре конструктора.
func syntheticValues(date time.Time) blankpath.Values {
	values := blankpath.Values{Date: date}
	for _, t := range blankpath.Tokens() {
		switch t.Key {
		case "номер":
			values.Number = t.Example
		case "id":
			values.ApplicationID = 4821
		case "заявитель":
			values.Sender = t.Example
		case "инициатор":
			values.Initiator = t.Example
		case "статус":
			values.Status = t.Example
		case "согласование":
			values.Confirmation = t.Example
		case "организация":
			values.Organization = t.Example
		case "компания":
			values.Company = t.Example
		case "тип":
			values.AttachmentType = t.Example
		case "период":
			values.Period = t.Example
		case "вложение_id":
			values.AttachmentID = 9134
		}
	}
	return values
}

// formatArchivePeriod складывает срок действия вложения в человеческий вид. Даты
// хранятся строками ISO; неразобранное значение отдаётся как есть - в имени файла
// лучше странная строка, чем пустое место там, где оператор ждёт срок.
func formatArchivePeriod(from, to string) string {
	fromRu, toRu := formatArchiveDate(from), formatArchiveDate(to)
	switch {
	case fromRu != "" && toRu != "":
		return fromRu + " - " + toRu
	case fromRu != "":
		return fromRu
	default:
		return toRu
	}
}

func formatArchiveDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) < 10 {
		return s
	}
	t, err := time.Parse("2006-01-02", s[:10])
	if err != nil {
		return s
	}
	return t.Format("02.01.2006")
}

func toTemplateIssues(problems []blankpath.Problem) []models.ArchiveTemplateIssue {
	out := make([]models.ArchiveTemplateIssue, 0, len(problems))
	for _, p := range problems {
		out = append(out, models.ArchiveTemplateIssue{Token: p.Token, Reason: p.Reason})
	}
	return out
}
