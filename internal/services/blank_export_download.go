package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"systemburo/internal/download"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// archiveDownloadTicketTTL - билет живёт секунды: фронт открывает ссылку скачивания
// сразу после выдачи. Долгий срок жизни не нужен и только увеличивает окно, в
// которое утёкшая ссылка остаётся рабочей.
const archiveDownloadTicketTTL = 60 * time.Second

// ErrArchiveDownloadRangeInvalid - date_from/date_to не разобрались либо date_from
// позже date_to.
var ErrArchiveDownloadRangeInvalid = errors.New("invalid archive download date range")

// ErrArchiveDownloadTooLarge - оценённый объём периода больше archive.zip_max_bytes.
var ErrArchiveDownloadTooLarge = errors.New("archive download exceeds size limit")

// archiveDownloadTicket - билет на потоковый ZIP за период (#1615, B3), по образцу
// internal/realtime/tickets.go. Привязан не только к userID (кто скачивает - в
// аудит и в лог), но и к границам периода: они зашиты в билет на выдаче, а не
// берутся заново из query GET-запроса на скачивание. Иначе владелец публичной
// ссылки (без JWT, см. Download) мог бы подменить период уже после того, как
// право и объём проверены при выдаче билета.
type archiveDownloadTicket struct {
	userID    int
	from, to  time.Time
	expiresAt time.Time
}

// ArchiveDownloadService стримит ZIP файлового архива за период или по заявке
// (#1615, B3). Отдельный сервис, а не методы BlankExportService: скачивание не
// пишет реестр и не зависит от генератора бланков, а тянет за собой билеты и
// потоковую отдачу, которые самому экспорту не нужны.
type ArchiveDownloadService struct {
	db       *gorm.DB
	writer   *ArchiveWriter
	settings SettingsService

	mu      sync.Mutex
	tickets map[string]archiveDownloadTicket
}

// NewArchiveDownloadService создаёт сервис скачивания файлового архива.
func NewArchiveDownloadService(db *gorm.DB, writer *ArchiveWriter, settings SettingsService) *ArchiveDownloadService {
	return &ArchiveDownloadService{db: db, writer: writer, settings: settings, tickets: make(map[string]archiveDownloadTicket)}
}

// ArchiveItemsQuery - фильтры списка реестра файлового архива (вкладка «Ошибки» и
// просмотр раздела, #1615 B3).
type ArchiveItemsQuery struct {
	Status        string
	ApplicationID int
	Page, PerPage int
}

// Estimate считает объём и число файлов периода до фактического скачивания -
// конструктор на фронте предупреждает о большом архиве, не запуская его.
func (s *ArchiveDownloadService) Estimate(ctx context.Context, dateFrom, dateTo string) (*models.ArchiveDownloadEstimate, error) {
	from, to, err := parseArchiveDownloadRange(dateFrom, dateTo)
	if err != nil {
		return nil, err
	}

	bytes, files, err := s.periodTotals(ctx, from, to)
	if err != nil {
		return nil, err
	}
	settings, err := s.settings.GetArchiveSettings(ctx)
	if err != nil {
		return nil, err
	}

	return &models.ArchiveDownloadEstimate{
		FileCount:    files,
		Bytes:        bytes,
		ExceedsLimit: settings.ZipMaxBytes > 0 && bytes > settings.ZipMaxBytes,
	}, nil
}

// IssueTicket проверяет период и потолок ZipMaxBytes, выдаёт билет и планирует его
// уборку по TTL. Потолок проверяется на выдаче, а не на скачивании: билет живёт
// секунды, повторная проверка на редемпшене ничего бы не добавила, кроме лишнего
// запроса к реестру на пути потоковой отдачи.
func (s *ArchiveDownloadService) IssueTicket(ctx context.Context, userID int, dateFrom, dateTo string) (string, error) {
	from, to, err := parseArchiveDownloadRange(dateFrom, dateTo)
	if err != nil {
		return "", err
	}

	bytes, _, err := s.periodTotals(ctx, from, to)
	if err != nil {
		return "", err
	}
	settings, err := s.settings.GetArchiveSettings(ctx)
	if err != nil {
		return "", err
	}
	if settings.ZipMaxBytes > 0 && bytes > settings.ZipMaxBytes {
		return "", ErrArchiveDownloadTooLarge
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("failed to generate archive download ticket: %w", err)
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw)

	now := time.Now()
	s.mu.Lock()
	s.gcTicketsLocked(now)
	s.tickets[ticket] = archiveDownloadTicket{userID: userID, from: from, to: to, expiresAt: now.Add(archiveDownloadTicketTTL)}
	s.mu.Unlock()
	return ticket, nil
}

// gcTicketsLocked чистит протухшие билеты. Вызывающий обязан держать мьютекс.
func (s *ArchiveDownloadService) gcTicketsLocked(now time.Time) {
	for k, t := range s.tickets {
		if now.After(t.expiresAt) {
			delete(s.tickets, k)
		}
	}
}

// consumeTicket проверяет билет и сразу удаляет его (одноразовость).
func (s *ArchiveDownloadService) consumeTicket(ticket string, now time.Time) (archiveDownloadTicket, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tickets[ticket]
	if !ok {
		return archiveDownloadTicket{}, false
	}
	delete(s.tickets, ticket)
	if now.After(t.expiresAt) {
		return archiveDownloadTicket{}, false
	}
	return t, true
}

// ConsumeAndCollect обменивает билет на список файлов ZIP, имя архива и того, кому
// билет выдан. Публичный GET-роут (без JWT, см. router.go) больше ничего про период
// не знает - границы пришли с билетом, а не из query-параметров запроса.
//
// Идентификатор пользователя возвращается ради журнала 152-ФЗ: на публичном роуте
// его больше взять неоткуда, а запись «кто-то выгрузил бланки сотен заявок» на
// вопрос закона не отвечает.
func (s *ArchiveDownloadService) ConsumeAndCollect(ctx context.Context, ticket string, now time.Time) ([]download.ZipEntry, string, int, error) {
	t, ok := s.consumeTicket(ticket, now)
	if !ok {
		return nil, "", 0, echo.NewHTTPError(http.StatusUnauthorized, "билет на скачивание недействителен или истёк")
	}

	entries, err := s.periodEntries(ctx, t.from, t.to)
	if err != nil {
		return nil, "", 0, err
	}
	name := fmt.Sprintf("archive_%s_%s.zip", t.from.Format("20060102"), t.to.Format("20060102"))
	return entries, name, t.userID, nil
}

// ArchiveApplicationEntries отбирает файлы заявки, доступные вызывающему, для ZIP
// GET /applications/:id/archive. canViewAttachment повторяет гейт скачивания
// одиночного бланка (attachment_blank.go, canDownloadBlank) - серверный ZIP не
// имеет права показать больше, чем показало бы скачивание файлов по одному.
//
// Слепок заявки (заявка.json) включается только при полном доступе к заявке
// (fullAccess): он несёт данные всех вложений, участников и организации разом, а
// узкий гейт охраны по одному конкретному вложению такого объёма не даёт даже при
// скачивании настоящего бланка.
func (s *ArchiveDownloadService) ArchiveApplicationEntries(
	ctx context.Context, applicationID int, fullAccess bool, canViewAttachment func(attachmentID int) (bool, error),
) ([]download.ZipEntry, error) {
	rows, err := s.rowsForApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}

	entries := make([]download.ZipEntry, 0, len(rows))
	for _, row := range rows {
		allowed := fullAccess
		if row.AttachmentID == archiveSnapshotAttachmentID {
			// Слепок - не вложение, свой гейт для него не спрашиваем.
		} else if !allowed {
			allowed, err = canViewAttachment(row.AttachmentID)
			if err != nil {
				return nil, err
			}
		}
		if !allowed {
			continue
		}

		path, err := s.resolvePath(row)
		if err != nil {
			slog.Warn("archive download: файл вне корня архива пропущен",
				"application_id", applicationID, "attachment_id", row.AttachmentID, "error", err)
			continue
		}
		entries = append(entries, s.zipEntry(path, row.FileName))
	}
	return entries, nil
}

// Get отдаёт одну строку реестра для скачивания отдельного файла (GET
// /file-archive/files/:id). Строка без записанного файла (не status=ok) отдаётся
// как 404 - на диске за ней ничего нет, отличать "нет строки" от "строка есть, но
// файла не будет" вызывающему незачем.
// zipEntry собирает запись архива с учётом шифрования: зашифрованный файл
// расшифровывается на лету, поэтому в ZIP уезжает читаемый документ, а на диске
// он остаётся закрытым.
func (s *ArchiveDownloadService) zipEntry(path, name string) download.ZipEntry {
	entry := download.ZipEntry{Path: path, Name: strings.TrimSuffix(name, EncryptedSuffix)}
	if s.writer != nil && s.writer.Crypto().Enabled() {
		crypto := s.writer.Crypto()
		entry.Open = func() (io.ReadCloser, error) { return crypto.Open(path) }
	}
	return entry
}

func (s *ArchiveDownloadService) Get(ctx context.Context, id int) (models.BlankExport, error) {
	var row models.BlankExport
	err := s.db.WithContext(ctx).First(&row, id).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return models.BlankExport{}, echo.NewHTTPError(http.StatusNotFound, "Файл не найден в архиве")
	case err != nil:
		return models.BlankExport{}, fmt.Errorf("failed to load archive item: %w", err)
	case row.Status != models.BlankExportOK:
		return models.BlankExport{}, echo.NewHTTPError(http.StatusNotFound, "Файл не найден в архиве")
	}
	return row, nil
}

// ResolveFile отдаёт абсолютный путь файла реестра для download.Serve.
func (s *ArchiveDownloadService) ResolveFile(row models.BlankExport) (string, error) {
	return s.resolvePath(row)
}

// FileForDownload собирает файл реестра к отдаче поштучно - для ленты архива и для
// кнопки «сохранённый файл» в карточке заявки.
//
// Зашифрованный файл расшифровывается на лету, а имя теряет суффикс: на диске
// лежит шифротекст, и отдать его байт в байт значит выдать администратору то, что
// он не откроет. Выгрузка ZIP за период так и делала с самого начала (zipEntry), а
// поштучное скачивание отдавало сырой файл - расхождение, которое было видно
// только на площадке с включёнными ключами.
//
// Размер берётся из реестра: там записан объём ИСХОДНОГО содержимого, а размер
// файла на диске к расшифрованному потоку отношения не имеет.
func (s *ArchiveDownloadService) FileForDownload(row models.BlankExport) (download.File, error) {
	path, err := s.resolvePath(row)
	if err != nil {
		return download.File{}, err
	}

	file := download.File{Path: path, Name: row.FileName}
	if s.writer == nil || !s.writer.Crypto().Enabled() || !strings.HasSuffix(row.FileName, EncryptedSuffix) {
		return file, nil
	}

	crypto := s.writer.Crypto()
	file.Name = strings.TrimSuffix(row.FileName, EncryptedSuffix)
	file.Size = row.SizeBytes
	file.Open = func() (io.ReadCloser, error) { return crypto.Open(path) }
	return file, nil
}

// GetByApplicationAttachment отдаёт строку реестра одного вложения заявки для
// скачивания сохранённого файла (?source=archive у AttachmentBlankHandler.Download,
// #1615 C6). Гейт доступа тот же, что у обычного скачивания бланка
// (canDownloadBlank) - отдельного права под архивный источник не заводим, это
// тот же бланк, просто с диска, а не сгенерированный заново. Строка без
// записанного файла (не status=ok) - 404, а не тихая регенерация: расхождение
// между тем, что скачал пользователь, и тем, что лежит в архиве, расследовать
// нечем.
func (s *ArchiveDownloadService) GetByApplicationAttachment(ctx context.Context, applicationID, attachmentID int) (models.BlankExport, error) {
	var row models.BlankExport
	err := s.db.WithContext(ctx).
		Where("application_id = ? AND attachment_id = ?", applicationID, attachmentID).
		First(&row).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return models.BlankExport{}, echo.NewHTTPError(http.StatusNotFound, "Файл не найден в архиве")
	case err != nil:
		return models.BlankExport{}, fmt.Errorf("failed to load archive item: %w", err)
	case row.Status != models.BlankExportOK:
		return models.BlankExport{}, echo.NewHTTPError(http.StatusNotFound, "Файл не найден в архиве")
	}
	return row, nil
}

// ListItems листает реестр файлового архива с фильтрами по статусу и заявке -
// вкладка «Ошибки» и просмотр раздела (#1615, B3).
func (s *ArchiveDownloadService) ListItems(ctx context.Context, q ArchiveItemsQuery) ([]models.ArchiveItemView, int64, error) {
	query := s.db.WithContext(ctx).Model(&models.BlankExport{})
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.ApplicationID > 0 {
		query = query.Where("application_id = ?", q.ApplicationID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count archive items: %w", err)
	}

	page, perPage := q.Page, q.PerPage
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	// Номер заявки и наименование вложения подтягиваются здесь же: без них лента
	// показывает внутренние идентификаторы, по которым человек ничего не опознаёт.
	// LEFT JOIN, потому что реестр живёт без внешних ключей и переживает удаление
	// и заявки, и вложения - строка обязана остаться видимой.
	//
	// Имя вложения считается тем же выражением, что и имя файла на диске
	// (archiveAttachmentNameExpr), иначе лента называла бы тип иначе, чем он
	// подписан в самом файле.
	sql := `
		SELECT be.*,
		       COALESCE(a.application_number, '') AS application_number,
		       COALESCE(` + archiveAttachmentNameExpr + `, '') AS attachment_name
		FROM blank_exports be
		LEFT JOIN applications a ON a.id = be.application_id
		LEFT JOIN attachments at ON at.id = be.attachment_id
		LEFT JOIN unique_attachments ua ON ua.id = at.unique_attachment_id
		WHERE 1 = 1`
	args := []any{}
	if q.Status != "" {
		sql += ` AND be.status = ?`
		args = append(args, q.Status)
	}
	if q.ApplicationID > 0 {
		sql += ` AND be.application_id = ?`
		args = append(args, q.ApplicationID)
	}
	sql += ` ORDER BY be.updated_at DESC LIMIT ? OFFSET ?`
	args = append(args, perPage, (page-1)*perPage)

	var rows []models.ArchiveItemView
	if err := s.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to load archive items: %w", err)
	}
	return rows, total, nil
}

// periodTotals считает суммарный объём и число живых файлов периода (status=ok).
// Общий запрос для Estimate и IssueTicket - оба обязаны видеть одно и то же число,
// иначе билет мог бы отказать по лимиту, который оценка перед этим не показала.
func (s *ArchiveDownloadService) periodTotals(ctx context.Context, from, to time.Time) (bytes, files int64, err error) {
	var agg struct {
		Bytes int64
		Files int64
	}
	err = s.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("status = ? AND bucket_date BETWEEN ? AND ?", models.BlankExportOK, from, to).
		Select("COALESCE(SUM(size_bytes), 0) AS bytes, COUNT(*) AS files").
		Scan(&agg).Error
	if err != nil {
		return 0, 0, fmt.Errorf("failed to estimate archive download: %w", err)
	}
	return agg.Bytes, agg.Files, nil
}

// rowsForRange читает живые строки реестра периода, отсортированные по фактическому
// пути - соседние файлы одной папки идут в архиве подряд.
func (s *ArchiveDownloadService) rowsForRange(ctx context.Context, from, to time.Time) ([]models.BlankExport, error) {
	var rows []models.BlankExport
	err := s.db.WithContext(ctx).
		Where("status = ? AND bucket_date BETWEEN ? AND ?", models.BlankExportOK, from, to).
		Order("rel_dir, file_name").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load archive rows for period: %w", err)
	}
	return rows, nil
}

// rowsForApplication читает живые строки реестра одной заявки, включая слепок.
func (s *ArchiveDownloadService) rowsForApplication(ctx context.Context, applicationID int) ([]models.BlankExport, error) {
	var rows []models.BlankExport
	err := s.db.WithContext(ctx).
		Where("application_id = ? AND status = ?", applicationID, models.BlankExportOK).
		Order("attachment_id").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to load archive rows for application: %w", err)
	}
	return rows, nil
}

// periodEntries превращает строки реестра периода в записи ZIP: имя внутри архива
// сохраняет структуру каталогов (RelDir/FileName), в отличие от ZIP заявки, где
// все файлы и так лежат в одной папке.
func (s *ArchiveDownloadService) periodEntries(ctx context.Context, from, to time.Time) ([]download.ZipEntry, error) {
	rows, err := s.rowsForRange(ctx, from, to)
	if err != nil {
		return nil, err
	}

	entries := make([]download.ZipEntry, 0, len(rows))
	for _, row := range rows {
		path, err := s.resolvePath(row)
		if err != nil {
			slog.Warn("archive download: файл вне корня архива пропущен", "id", row.ID, "error", err)
			continue
		}
		entries = append(entries, s.zipEntry(path, zipPathFor(row)))
	}
	return entries, nil
}

// zipPathFor - имя записи внутри архива за период: путь каталога заявки плюс имя
// файла, тем же разделителем "/", которым RelDir уже собран (path.Join).
func zipPathFor(row models.BlankExport) string {
	if row.RelDir == "" {
		return row.FileName
	}
	return row.RelDir + "/" + row.FileName
}

// resolvePath переводит хранимый RelDir/FileName в абсолютный путь на диске тем же
// писателем, что и запись - второй рубеж (JoinUnder) ловит любое расхождение
// раньше, чем оно превратится в чтение чужого файла.
func (s *ArchiveDownloadService) resolvePath(row models.BlankExport) (string, error) {
	return s.writer.Resolve(append(splitRelDir(row.RelDir), row.FileName)...)
}

// parseArchiveDownloadRange разбирает и проверяет границы периода.
func parseArchiveDownloadRange(dateFrom, dateTo string) (time.Time, time.Time, error) {
	from, err := time.Parse("2006-01-02", dateFrom)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_from", ErrArchiveDownloadRangeInvalid)
	}
	to, err := time.Parse("2006-01-02", dateTo)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_to", ErrArchiveDownloadRangeInvalid)
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("%w: date_to before date_from", ErrArchiveDownloadRangeInvalid)
	}
	return from, to, nil
}
