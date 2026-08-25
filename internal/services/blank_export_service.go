package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"

	"systemburo/internal/blankpath"
	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Причины постановки в очередь. Пишутся в реестр и отвечают на вопрос «почему файл
// переписался», когда заявку никто руками не трогал.
const (
	BlankExportReasonReexport = "reexport"
	BlankExportReasonSubmit   = "submit"
	BlankExportReasonUpdate   = "update"
	// BlankExportReasonSweep - подметатель повторов (B1): заявка, у которой прошлая
	// попытка выгрузки провалилась транзиентно и подошёл срок next_attempt_at.
	BlankExportReasonSweep = "sweep"
	// BlankExportReasonRecheck - ночная сверка реестра с диском (B1): полный прогон
	// заявок в окне recheck_days, независимо от того, звала ли их очередь.
	BlankExportReasonRecheck = "recheck"
	// BlankExportReasonBackfill - ручной бэкфилл за период (B4): администратор просит
	// пересобрать бланки диапазона, не дожидаясь ночной сверки, либо пересоздаёт файлы
	// одного типа после правки маппингов шаблона.
	BlankExportReasonBackfill = "backfill"
)

// Паузы перед повтором неудачной выгрузки: минута с удвоением до шести часов.
// Верхняя граница не бесконечная - место на диске освобождают в тот же день, и
// сутки ожидания превратили бы поправимый сбой в пропажу бланка.
const (
	blankExportRetryBase = time.Minute
	blankExportRetryMax  = 6 * time.Hour
)

// ErrArchiveDisabled - выгрузка выключена глобальным рубильником. Отдельная ошибка,
// а не молчаливый выход: администратор, нажавший «пересоздать», должен увидеть
// причину, а не пустой результат.
var ErrArchiveDisabled = errors.New("file archive is disabled")

// BlankExportService пишет заполненные бланки заявки в файловый архив (#1615).
//
// Единица обработки - заявка целиком: папка принадлежит ей, и переименование,
// запущенное из нескольких строк реестра одновременно, было бы гонкой.
type BlankExportService struct {
	db       *gorm.DB
	blanks   AttachmentBlankService
	paths    *ArchivePathService
	writer   *ArchiveWriter
	settings SettingsService
	// queue - набор заявок, ожидающих разбора фоновым воркером (B1). Отдельная
	// маленькая структура, а не поле-канал прямо здесь: push/drain/nudge должны
	// быть безопасны без блокировки остального сервиса.
	queue *blankExportQueue
	// quota - пороги места (B2). Спрашивается перед каждой фоновой записью: узнать
	// про нехватку места надо ДО записи, а не по факту сбоя.
	quota ArchiveQuotaGuard
	// files - хранилище файлов, приложенных к заявке (#1721). Опционально: без него
	// в архив уезжают только бланки.
	files ApplicationFileService
}

// NewBlankExportService собирает сервис выгрузки поверх готовых частей: генератор
// бланков, расчёт путей, писатель на диск и сторож места на разделе.
//
// quota может быть nil - тогда фоновые прогоны идут без проверки порогов (полезно
// в тестах ядра выгрузки). В рабочем процессе сторож подключён всегда: без него
// жёсткий порог места остаётся мёртвым кодом.
func NewBlankExportService(
	db *gorm.DB,
	blanks AttachmentBlankService,
	paths *ArchivePathService,
	writer *ArchiveWriter,
	settings SettingsService,
	quota ArchiveQuotaGuard,
) *BlankExportService {
	return &BlankExportService{
		db: db, blanks: blanks, paths: paths, writer: writer, settings: settings,
		queue: newBlankExportQueue(), quota: quota,
	}
}

// Wake отдаёт канал пробуждения воркера - сигнал приходит по каждому enqueue и
// по явному Nudge. Воркер (cmd/server) селектит его рядом с тикерами.
func (s *BlankExportService) Wake() <-chan struct{} {
	return s.queue.wake
}

// Writer отдаёт писателя архива - воркеру (cmd/server) он нужен для уборки
// временного мусора (.tmp-*) на своём тике, отдельном от разбора очереди.
// SetApplicationFiles подключает хранилище файлов заявки: без него приложенные
// файлы в архив не уезжают, а бланки выкладываются как прежде.
//
// Проверка на nil обязательна: когда каталог архива не поднялся, сервис остаётся
// типизированным nil - соседние Enqueue* на нём тоже no-op, и присваивание поля
// уронило бы старт вместо тихого отказа от выгрузки.
func (s *BlankExportService) SetApplicationFiles(f ApplicationFileService) {
	if s == nil {
		return
	}
	s.files = f
}

func (s *BlankExportService) Writer() *ArchiveWriter {
	return s.writer
}

// Location - рабочая таймзона раскладки архива. Границы бэкфилла считаются по ней,
// иначе они разъедутся с bucket_date: заявка, поданная вечером по местному времени,
// в UTC относится уже к следующим суткам.
func (s *BlankExportService) Location() *time.Location { return s.paths.Location() }

// blankExportTarget - вложение заявки вместе с тем, что решает его судьбу: тумблер
// типа и наличие активного бланка.
type blankExportTarget struct {
	AttachmentID       int    `gorm:"column:attachment_id"`
	UniqueAttachmentID *int   `gorm:"column:unique_attachment_id"`
	AutoExport         bool   `gorm:"column:auto_export"`
	TemplateID         *int   `gorm:"column:template_id"`
	FileName           string `gorm:"-"`
	// SourceFileID - файл, приложенный к заявке (#1721). Заполнен у целей, которые
	// не генерируются из бланка, а копируются из хранилища загрузок. Такая цель
	// живёт в реестре под отрицательным attachment_id: столбец без внешнего ключа,
	// пары (заявка, отрицательный id) с настоящими вложениями не пересекаются.
	SourceFileID *int `gorm:"-"`
}

// archiveAttachmentsDir - подпапка заявки, куда кладутся приложенные к ней файлы.
// Отдельная от бланков: имена приносит заявитель, и совпадение с именем бланка
// перезаписало бы документ.
const archiveAttachmentsDir = "Приложения"

// targetLevels отдаёт каталог цели: бланки лежат в папке заявки, приложенные к ней
// файлы - в подпапке.
func targetLevels(levels []string, target blankExportTarget) []string {
	if target.SourceFileID == nil {
		return levels
	}
	return append(append([]string{}, levels...), archiveAttachmentsDir)
}

// ExportApplication выгружает бланки заявки в архив и приводит реестр в соответствие
// с диском. Порядок шагов - «диск, потом база»: падение между ними лечится следующим
// прогоном (фактического каталога уже нет, файлы просто перепишутся), обратный
// порядок оставил бы каталог-сироту, про который система забыла.
func (s *BlankExportService) ExportApplication(ctx context.Context, applicationID int, reason string) (*models.BlankExportResult, error) {
	settings, err := s.settings.GetArchiveSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		return nil, ErrArchiveDisabled
	}

	appValues, err := s.paths.Values(ctx, applicationID, 0)
	if err != nil {
		return nil, err
	}
	targets, err := s.loadTargets(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	registry, err := s.loadRegistry(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	frozenAt, err := s.freezeMoment(ctx, applicationID, settings.FreezeAfterDays)
	if err != nil {
		return nil, err
	}

	levels, err := s.layout(ctx, settings, applicationID, appValues, targets, registry)
	if err != nil {
		return nil, err
	}

	result := &models.BlankExportResult{ApplicationID: applicationID, Items: make([]models.BlankExportItem, 0, len(targets))}
	if err := s.markOrphans(ctx, registry, targets); err != nil {
		return nil, err
	}

	// Замороженная заявка остаётся там, где лежит: путь уже уехал в корпоративную
	// копию, и переименование развело бы копии по разным папкам. Новые вложения
	// такой заявки лягут в ту же папку - она и есть папка этой заявки.
	frozenApplicationDir := frozenDir(registry)
	if frozenApplicationDir != "" {
		levels = splitRelDir(frozenApplicationDir)
	} else {
		levels, result.Renamed, err = s.relocate(ctx, registry, levels)
		if err != nil {
			return nil, err
		}
	}
	result.RelDir = path.Join(levels...)

	for _, target := range targets {
		item := s.exportOne(ctx, exportRequest{
			applicationID: applicationID,
			reason:        reason,
			bucketDate:    appValues.Date,
			levels:        targetLevels(levels, target),
			target:        target,
			row:           registry[target.AttachmentID],
			frozenAt:      frozenAt,
		})
		result.Items = append(result.Items, item)
	}

	// Слепок замораживается по сроку самой заявки, а не по тому, есть ли рядом
	// замороженный бланк. У заявки, где ни одному типу вложения не настроен бланк,
	// замороженных строк реестра не появляется вовсе - и слепок такой заявки
	// переписывался бы вечно, хотя обещан окончательным.
	//
	// Слепок ведёт строку реестра под attachment_id=archiveSnapshotAttachmentID
	// (B1, долг A5c): без неё currentDir/frozenDir/relocate не видят его фактический
	// путь, и заявка без единого бланка после смены организации оставляет заявка.json
	// сиротой в прежней папке - переезжать в системе было бы нечему.
	result.Snapshot = s.exportSnapshot(ctx, exportRequest{
		applicationID: applicationID,
		reason:        reason,
		bucketDate:    appValues.Date,
		levels:        levels,
		target:        blankExportTarget{AttachmentID: archiveSnapshotAttachmentID, FileName: s.writer.Crypto().FileName(archiveSnapshotFileName)},
		row:           registry[archiveSnapshotAttachmentID],
		frozenAt:      frozenAt,
	})

	return result, nil
}

// exportSnapshot пишет машиночитаемый слепок заявки рядом с бланками и докладывает
// исход в ответе. Молчать нельзя: без ретрая и без записи в реестр администратор,
// нажавший «пересоздать», обязан узнать о несостоявшейся записи сразу, а не
// обнаружить пропажу файла годы спустя.
//
// Решение «писать или нет» остаётся диск-ориентированным (writeApplicationSnapshot
// сама проверяет exists+frozen), а не читает req.row.FrozenAt из реестра: гейт по
// персистентному признаку оставил бы без слепка навсегда заявки, у которых первая
// запись сорвалась ровно на прогоне заморозки, - реестр здесь только запоминает
// ФАКТИЧЕСКИЙ путь для переезда, не решает, перезаписывать ли содержимое.
func (s *BlankExportService) exportSnapshot(ctx context.Context, req exportRequest) models.BlankExportSnapshotResult {
	frozen := req.frozenAt != nil
	out := models.BlankExportSnapshotResult{
		RelPath: path.Join(path.Join(req.levels...), req.target.FileName),
		Frozen:  frozen,
	}

	written, hash, size, err := writeApplicationSnapshot(ctx, s.db, s.writer, req.applicationID, req.levels, frozen)
	if err != nil {
		out.Status, out.Error = models.BlankExportFailed, err.Error()
		slog.Error("не удалось записать слепок заявки в архив", "application_id", req.applicationID, "error", err)
		if saveErr := s.saveState(ctx, req, out.Status, out.Error, nil); saveErr != nil {
			slog.Error("не удалось записать состояние слепка заявки", "application_id", req.applicationID, "error", saveErr)
		}
		return out
	}

	// Замороженный слепок пропускает чтение файла (writeApplicationSnapshot экономит
	// I/O раз содержимое всё равно неприкосновенно) - хэш тогда не пересчитан, берём
	// его из прежней строки реестра. RelDir всё равно перезаписывается на req.levels:
	// без этого переезд заявки при смене организации некому заметить (долг A5c).
	if hash == "" && req.row != nil {
		hash, size = req.row.ContentHash, req.row.SizeBytes
	}

	out.Status, out.Written = models.BlankExportOK, written
	state := &exportedFile{hash: hash, size: size, frozenAt: req.frozenAt}
	if err := s.saveState(ctx, req, models.BlankExportOK, "", state); err != nil {
		out.Status, out.Error = models.BlankExportFailed, err.Error()
	}
	return out
}

// exportRequest - вход одной строки реестра. Собран структурой, чтобы список
// параметров не разъезжался между вызовом и сигнатурой.
type exportRequest struct {
	applicationID int
	reason        string
	// bucketDate - день подачи заявки в рабочей таймзоне. Считается один раз на
	// заявку: по нему строится раскладка каталогов, и «сегодня» вместо дня подачи
	// увело бы строку реестра в чужой период при первом же пересоздании.
	bucketDate time.Time
	levels     []string
	target     blankExportTarget
	row        *models.BlankExport
	frozenAt   *time.Time
}

// exportOne обрабатывает один бланк: решает, писать ли его вообще, генерирует,
// сравнивает хэш и приводит строку реестра в соответствие с диском.
func (s *BlankExportService) exportOne(ctx context.Context, req exportRequest) models.BlankExportItem {
	item := models.BlankExportItem{AttachmentID: req.target.AttachmentID}

	// Уже замороженный файл не трогаем ни при каких правках заявки: он документ, а
	// не отражение текущего состояния.
	if req.row != nil && req.row.FrozenAt != nil {
		item.Status, item.Frozen = req.row.Status, true
		item.RelPath = path.Join(req.row.RelDir, req.row.FileName)
		return item
	}

	// Выключенный тумблер и отсутствующий бланк останавливают запись, но уже
	// лежащий файл не удаляют: смена настройки не повод стирать документ, который
	// мог уехать в корпоративную копию. К приложенным файлам это не относится:
	// у них нет ни тумблера, ни шаблона.
	switch {
	case req.target.SourceFileID != nil:
	case !req.target.AutoExport:
		item.Status = models.BlankExportSkipped
	case req.target.UniqueAttachmentID == nil || req.target.TemplateID == nil:
		item.Status = models.BlankExportNoTemplate
	}
	if item.Status != "" {
		if err := s.saveState(ctx, req, item.Status, "", nil); err != nil {
			item.Status, item.Error = models.BlankExportFailed, err.Error()
		}
		return item
	}

	data, err := s.contentFor(ctx, req)
	if err != nil {
		item.Status, item.Error = classifyBlankExportError(err), err.Error()
		// Не сложившаяся запись в реестр не должна подменять собой причину отказа:
		// администратор чинит шаблон, а не базу, и текст ему нужен исходный.
		if saveErr := s.saveState(ctx, req, item.Status, item.Error, nil); saveErr != nil {
			slog.Error("не удалось записать состояние выгрузки бланка",
				"application_id", req.applicationID, "attachment_id", req.target.AttachmentID, "error", saveErr)
		}
		return item
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	item.RelPath = path.Join(path.Join(req.levels...), req.target.FileName)

	written, err := s.placeFile(req, hash, data)
	if err != nil {
		item.Status, item.Error = models.BlankExportFailed, err.Error()
		if saveErr := s.saveState(ctx, req, item.Status, item.Error, nil); saveErr != nil {
			slog.Error("не удалось записать состояние выгрузки бланка",
				"application_id", req.applicationID, "attachment_id", req.target.AttachmentID, "error", saveErr)
		}
		return item
	}

	item.Status, item.Written = models.BlankExportOK, written
	item.Frozen = req.frozenAt != nil
	state := &exportedFile{hash: hash, size: int64(len(data)), frozenAt: req.frozenAt}
	if err := s.saveState(ctx, req, models.BlankExportOK, "", state); err != nil {
		item.Status, item.Error = models.BlankExportFailed, err.Error()
	}
	return item
}

// contentFor отдаёт содержимое цели: сгенерированный бланк либо приложенный к
// заявке файл.
//
// Файл читается через расшифровку: в хранилище загрузок он лежит закрытым, а в
// архив должен уехать пригодным для чтения принимающей стороной - там его закроет
// уже шифрование архива, ключами получателя.
func (s *BlankExportService) contentFor(ctx context.Context, req exportRequest) ([]byte, error) {
	if req.target.SourceFileID == nil {
		return s.generate(ctx, req.applicationID, req.target.AttachmentID)
	}
	if s.files == nil {
		return nil, fmt.Errorf("хранилище файлов заявки не подключено")
	}
	return s.files.ReadContent(ctx, *req.target.SourceFileID)
}

// placeFile кладёт файл на диск, если его содержимое или положение изменились.
//
// Совпал хэш и файл на месте - не открываем его вовсе: это экономит запись и, что
// важнее, не двигает mtime, поэтому инкрементальная синхронизация на рабочий
// компьютер не перекачивает неизменившееся.
func (s *BlankExportService) placeFile(req exportRequest, hash string, data []byte) (bool, error) {
	row := req.row
	sameContent := row != nil && row.ContentHash == hash &&
		row.RelDir == path.Join(req.levels...) && row.FileName == req.target.FileName
	if sameContent {
		exists, err := s.writer.Exists(req.levels, req.target.FileName)
		if err != nil {
			return false, err
		}
		if exists {
			return false, nil
		}
	}

	if err := s.writer.WriteFile(req.levels, req.target.FileName, data); err != nil {
		return false, err
	}
	// Имя файла сменилось внутри той же папки (поправили шаблон или тип вложения) -
	// прежний файл иначе остался бы лежать рядом вторым экземпляром тех же данных.
	if row != nil && row.FileName != "" && row.FileName != req.target.FileName && row.RelDir == path.Join(req.levels...) {
		if err := s.writer.RemoveFile(req.levels, row.FileName); err != nil {
			return true, err
		}
	}
	return true, nil
}

// exportedFile - то, что удалось записать: чем заполнять строку реестра при успехе.
type exportedFile struct {
	hash     string
	size     int64
	frozenAt *time.Time
}

// saveState приводит строку реестра к результату прогона. Upsert по паре
// (application_id, attachment_id): строку мог не создать никто - ручное пересоздание
// работает и на заявке, которой очередь ещё не касалась.
func (s *BlankExportService) saveState(ctx context.Context, req exportRequest, status, lastError string, file *exportedFile) error {
	now := time.Now()
	// ID строки не подставляем даже когда он известен: вставка с готовым ключом
	// конфликтовала бы по первичному индексу, а не по паре «заявка + вложение»,
	// и вместо обновления пришла бы ошибка.
	row := models.BlankExport{
		ApplicationID:      req.applicationID,
		AttachmentID:       req.target.AttachmentID,
		UniqueAttachmentID: req.target.UniqueAttachmentID,
		TemplateID:         req.target.TemplateID,
		BucketDate:         req.bucketDate,
		Status:             status,
		LastError:          lastError,
		QueueReason:        req.reason,
		QueuedAt:           now,
	}
	if req.row != nil {
		row.RelDir, row.FileName = req.row.RelDir, req.row.FileName
		row.ContentHash, row.SizeBytes = req.row.ContentHash, req.row.SizeBytes
		row.GeneratedAt, row.FrozenAt = req.row.GeneratedAt, req.row.FrozenAt
	}

	switch {
	case file != nil:
		row.RelDir, row.FileName = path.Join(req.levels...), req.target.FileName
		row.ContentHash, row.SizeBytes = file.hash, file.size
		row.GeneratedAt, row.FrozenAt = &now, file.frozenAt
		row.Attempts, row.NextAttemptAt = 0, nil
	case status == models.BlankExportFailed:
		// Ретраить имеет смысл только транзиентные отказы; счётчик растёт от
		// предыдущего значения, поэтому пауза удваивается, а не начинается заново.
		row.Attempts = 1
		if req.row != nil {
			row.Attempts = req.row.Attempts + 1
		}
		next := now.Add(blankExportBackoff(row.Attempts))
		row.NextAttemptAt = &next
	default:
		// skipped/no_template ждут действия человека, а не времени.
		row.Attempts, row.NextAttemptAt = 0, nil
	}

	return s.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "application_id"}, {Name: "attachment_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"unique_attachment_id", "template_id", "rel_dir", "file_name", "size_bytes",
			"content_hash", "status", "attempts", "last_error", "next_attempt_at",
			"queue_reason", "generated_at", "frozen_at", "updated_at",
		}),
	}).Create(&row).Error
}

// generate вызывает единственный генератор бланков и складывает его вывод в память:
// хэш считается по тем же байтам, которые лягут на диск, а не по отдельному прогону.
func (s *BlankExportService) generate(ctx context.Context, applicationID, attachmentID int) ([]byte, error) {
	// Копия на диске - полная: архив читает внешняя сторона, ради которой он и заведён,
	// а сам файл шифруется на её ключ. Гейт документов стоит на выдаче (см.
	// canExportBlankDocuments), а не на записи: обезличенный архив пришлось бы держать
	// вторым комплектом файлов, и первый же запрос «а где паспорта» вернул бы всё назад.
	reader, _, err := s.blanks.GenerateBlank(ctx, applicationID, attachmentID,
		BlankOptions{IncludeDocuments: true})
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read generated blank: %w", err)
	}
	return data, nil
}

// layout считает раскладку заявки: общие уровни каталогов и имя файла на каждое
// вложение. Подгонка длины пути делается по самому длинному имени - иначе у двух
// вложений одной заявки уровни разошлись бы и папка раздвоилась.
func (s *BlankExportService) layout(
	ctx context.Context,
	settings *models.ArchiveSettings,
	applicationID int,
	appValues blankpath.Values,
	targets []blankExportTarget,
	registry map[int]*models.BlankExport,
) ([]string, error) {
	longest := ""
	for i := range targets {
		// У приложенного файла имя своё - его принёс заявитель, шаблон к нему не
		// применяется. И вложения с таким идентификатором не существует, поэтому
		// разбор его полей закончился бы отказом «вложение не найдено».
		if targets[i].SourceFileID != nil {
			continue
		}
		// Поля заявки уже загружены и для всех её вложений одинаковы - дочитываем
		// только тип и срок вложения, а не повторяем тяжёлый разбор заявки.
		att, err := s.paths.attachmentValues(ctx, applicationID, targets[i].AttachmentID)
		if err != nil {
			return nil, err
		}
		values := appValues
		values.AttachmentID = att.AttachmentID
		values.AttachmentType = att.AttachmentType
		values.Period = formatArchivePeriod(att.EntryDateFrom, att.EntryDateTo)

		targets[i].FileName = blankpath.RenderName(settings.FileTemplate, values, blankFileExt)
		if len(targets[i].FileName) > len(longest) {
			longest = targets[i].FileName
		}
	}
	resolveNameCollisions(targets, registry)
	s.applyArchiveEncryptionSuffix(targets)

	levels := blankpath.FitRelPath(blankpath.RenderPath(settings.DirTemplate, appValues), longest, archiveRelPathMaxBytes)
	return s.resolveDirCollision(ctx, applicationID, levels)
}

// resolveNameCollisions разводит вложения, чьи имена файлов совпали: у заявки
// бывает два вложения одного типа, а шаблон имени про них не знает.
//
// Суффикс получают ВСЕ участники коллизии и по своему attachment_id, а не второй по
// счёту: нумерация «(2)», «(3)» перетасовывалась бы при каждом пересоздании и файл
// прыгал бы между именами, ломая синхронизацию на рабочий компьютер.
//
// Имена замороженных файлов берутся из реестра, а не из свежего расчёта: такой файл
// лежит на диске под старым именем и переименован уже не будет. Не учти мы его,
// новое вложение после смены шаблона имени попало бы ровно на замороженный файл и
// переписало бы документ, который обещан неизменным.
func resolveNameCollisions(targets []blankExportTarget, registry map[int]*models.BlankExport) {
	seen := make(map[string]int, len(targets))
	for _, t := range targets {
		seen[t.FileName]++
	}
	// Занятое замороженное имя делает конфликтующим любое совпадение с ним, даже
	// единственное: подвинуться обязан новый файл, старый уже документ.
	for _, row := range registry {
		if row.FrozenAt != nil && row.FileName != "" {
			seen[row.FileName] += 2
		}
	}

	for i := range targets {
		// Замороженному вложению имя не меняем: на диске оно уже под своим.
		if row := registry[targets[i].AttachmentID]; row != nil && row.FrozenAt != nil {
			continue
		}
		if seen[targets[i].FileName] < 2 {
			continue
		}
		base := strings.TrimSuffix(targets[i].FileName, blankFileExt)
		targets[i].FileName = blankpath.FileName(fmt.Sprintf("%s (№%d)", base, targets[i].AttachmentID), blankFileExt)
	}
}

// applyArchiveEncryptionSuffix дописывает имени признак шифрования. Делается
// последним шагом, после разведения одинаковых имён: суффикс попадает и в реестр,
// и на диск, иначе сверка каталога с реестром считала бы файлы пропавшими.
func (s *BlankExportService) applyArchiveEncryptionSuffix(targets []blankExportTarget) {
	if s.writer == nil || !s.writer.Crypto().Enabled() {
		return
	}
	for i := range targets {
		targets[i].FileName = s.writer.Crypto().FileName(targets[i].FileName)
	}
}

// resolveDirCollision разводит две заявки, попавшие в одну папку: шаблон может не
// содержать номера, и тогда две заявки одной организации за один день сложились бы
// в общий каталог, а сверка увидела бы чужие файлы как пропавшие.
//
// Суффикс детерминированный - по application_id, не по счётчику. Длина после
// суффикса может выйти за расчётный предел пути: это осознанный размен, потому что
// обрезка съела бы сам суффикс и коллизия вернулась бы.
func (s *BlankExportService) resolveDirCollision(ctx context.Context, applicationID int, levels []string) ([]string, error) {
	if len(levels) == 0 {
		return levels, nil
	}
	relDir := path.Join(levels...)

	var taken int64
	err := s.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("rel_dir = ? AND application_id <> ?", relDir, applicationID).
		Count(&taken).Error
	if err != nil {
		return nil, fmt.Errorf("failed to check archive directory collision: %w", err)
	}
	if taken == 0 {
		return levels, nil
	}

	out := append([]string(nil), levels...)
	last := len(out) - 1
	out[last] = blankpath.ComponentOr(fmt.Sprintf("%s (№%d)", out[last], applicationID), blankpath.FallbackName)
	return out, nil
}

// relocate переносит папку заявки на желаемое место, если фактическое разошлось с
// расчётным: организацию в заявке поправили, и дерево обязано это отразить, иначе
// рядом появится вторая папка с теми же бланками.
//
// Пропажа источника (папку удалили руками) отказом не считается: фактический путь
// забывается, и файлы просто пишутся заново.
func (s *BlankExportService) relocate(ctx context.Context, registry map[int]*models.BlankExport, levels []string) ([]string, bool, error) {
	current := currentDir(registry)
	target := path.Join(levels...)
	if current == "" || current == target {
		return levels, false, nil
	}

	switch err := s.writer.MoveDir(splitRelDir(current), levels); {
	case err == nil:
	case errors.Is(err, ErrArchiveSourceMissing):
		if err := s.forgetDir(ctx, registry); err != nil {
			return nil, false, err
		}
		return levels, false, nil
	default:
		return nil, false, err
	}

	if err := s.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("application_id = ? AND rel_dir = ?", applicationIDOf(registry), current).
		Update("rel_dir", target).Error; err != nil {
		return nil, false, fmt.Errorf("failed to update archive directory: %w", err)
	}
	for _, row := range registry {
		if row.RelDir == current {
			row.RelDir = target
		}
	}
	return levels, true, nil
}

// forgetDir забывает фактический путь строк заявки. Хэш обнуляется вместе с ним:
// иначе дедупликация решила бы, что файл на месте, и папка осталась бы пустой.
func (s *BlankExportService) forgetDir(ctx context.Context, registry map[int]*models.BlankExport) error {
	err := s.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("application_id = ?", applicationIDOf(registry)).
		Updates(map[string]any{"rel_dir": "", "content_hash": ""}).Error
	if err != nil {
		return fmt.Errorf("failed to reset archive directory: %w", err)
	}
	for _, row := range registry {
		row.RelDir, row.ContentHash = "", ""
	}
	return nil
}

// markOrphans помечает строки реестра, чьи вложения из заявки исчезли. Файл при этом
// не удаляется: сирота докладывается человеку, а не подчищается молча - удалять
// документ по расхождению, причину которого никто не разобрал, опаснее, чем хранить
// лишний файл.
func (s *BlankExportService) markOrphans(ctx context.Context, registry map[int]*models.BlankExport, targets []blankExportTarget) error {
	alive := make(map[int]struct{}, len(targets))
	for _, t := range targets {
		alive[t.AttachmentID] = struct{}{}
	}
	orphans := make([]int, 0)
	for id, row := range registry {
		// Строка слепка (attachment_id=archiveSnapshotAttachmentID) не вложение и
		// никогда не встретится среди targets - без этой оговорки её отмечало бы
		// сиротой на каждом прогоне.
		if id == archiveSnapshotAttachmentID {
			continue
		}
		if _, ok := alive[id]; !ok && row.Status != models.BlankExportOrphan {
			orphans = append(orphans, row.ID)
		}
	}
	if len(orphans) == 0 {
		return nil
	}

	err := s.db.WithContext(ctx).Model(&models.BlankExport{}).Where("id IN ?", orphans).
		Updates(map[string]any{"status": models.BlankExportOrphan, "next_attempt_at": nil}).Error
	if err != nil {
		return fmt.Errorf("failed to mark orphan blank exports: %w", err)
	}
	return nil
}

// loadTargets читает вложения заявки вместе с тумблером типа и активным бланком.
// Шаблон берётся подзапросом, а не соединением: неактивных версий бланка у типа
// сколько угодно, и JOIN размножил бы строки вложений.
func (s *BlankExportService) loadTargets(ctx context.Context, applicationID int) ([]blankExportTarget, error) {
	const sql = `
		SELECT at.id AS attachment_id,
		       at.unique_attachment_id AS unique_attachment_id,
		       COALESCE(ua.auto_export, TRUE) AS auto_export,
		       (
		           SELECT tpl.id FROM attachment_templates tpl
		           WHERE tpl.unique_attachment_id = at.unique_attachment_id AND tpl.is_active = TRUE
		           ORDER BY tpl.id DESC LIMIT 1
		       ) AS template_id
		FROM attachments at
		LEFT JOIN unique_attachments ua ON ua.id = at.unique_attachment_id
		WHERE at.application_id = ?
		ORDER BY at.id`

	var rows []blankExportTarget
	if err := s.db.WithContext(ctx).Raw(sql, applicationID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load application attachments for export: %w", err)
	}

	files, err := s.loadFileTargets(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	return append(rows, files...), nil
}

// loadFileTargets собирает цели по файлам, приложенным к заявке (#1721).
//
// Отрицательный attachment_id - способ уложить их в тот же реестр, что и бланки:
// пара (заявка, вложение) остаётся уникальной, а весь остальной механизм
// (заморозка, сверка с диском, пометка сирот, переезд каталога) начинает работать
// для них даром. Настоящих вложений с отрицательным id не бывает.
func (s *BlankExportService) loadFileTargets(ctx context.Context, applicationID int) ([]blankExportTarget, error) {
	var files []models.ApplicationFile
	if err := s.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("id").Find(&files).Error; err != nil {
		return nil, fmt.Errorf("failed to load application files for export: %w", err)
	}

	targets := make([]blankExportTarget, 0, len(files))
	for i := range files {
		id := files[i].ID
		targets = append(targets, blankExportTarget{
			AttachmentID: -id,
			// Приложенные файлы выкладываются всегда: тумблер auto_export
			// настраивает типы вложений, а к ним эти файлы не относятся.
			AutoExport:   true,
			FileName:     blankpath.FileName(strings.TrimSuffix(files[i].FileName, filepath.Ext(files[i].FileName)), filepath.Ext(files[i].FileName)),
			SourceFileID: &id,
		})
	}
	return targets, nil
}

// loadRegistry читает строки реестра заявки по attachment_id.
func (s *BlankExportService) loadRegistry(ctx context.Context, applicationID int) (map[int]*models.BlankExport, error) {
	var rows []models.BlankExport
	if err := s.db.WithContext(ctx).Where("application_id = ?", applicationID).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to load blank export registry: %w", err)
	}
	out := make(map[int]*models.BlankExport, len(rows))
	for i := range rows {
		out[rows[i].AttachmentID] = &rows[i]
	}
	return out, nil
}

// freezeMoment решает, стал ли файл заявки окончательным. Отсчёт идёт от момента,
// когда заявка перестала действовать, и той же формулой, по которой заявка уходит в
// архив Центра: иначе активная заявка замораживала бы свои бланки.
//
// Возвращает момент заморозки для записи в реестр либо nil, если срок ещё не вышел.
func (s *BlankExportService) freezeMoment(ctx context.Context, applicationID, freezeAfterDays int) (*time.Time, error) {
	since, args := applicationInactiveSinceExpr("a")
	sql := fmt.Sprintf(`
		SELECT CASE WHEN COALESCE(a.status, '') IN ? THEN %s END AS inactive_since
		FROM applications a WHERE a.id = ?`, since)

	var row struct {
		InactiveSince *time.Time `gorm:"column:inactive_since"`
	}
	params := append([]any{models.ArchivableStatuses}, args...)
	params = append(params, applicationID)
	if err := s.db.WithContext(ctx).Raw(sql, params...).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("failed to load application inactivity moment: %w", err)
	}
	if row.InactiveSince == nil {
		return nil, nil
	}

	now := time.Now()
	if now.Before(row.InactiveSince.AddDate(0, 0, freezeAfterDays)) {
		return nil, nil
	}
	return &now, nil
}

// classifyBlankExportError отделяет «нечего выгружать» от «не получилось сейчас».
//
// Генератор отвечает ошибкой клиента, когда у вложения нет шаблона или он не
// настроен - повторять такое бессмысленно, это видимый пробел архива и ждёт он
// действия администратора. Остальное (недоступный файл шаблона, сбой записи)
// транзиентно и уходит в повтор.
func classifyBlankExportError(err error) string {
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) && httpErr.Code >= http.StatusBadRequest && httpErr.Code < http.StatusInternalServerError {
		return models.BlankExportNoTemplate
	}
	return models.BlankExportFailed
}

// blankExportBackoff - пауза перед следующей попыткой: удвоение от минуты до потолка.
func blankExportBackoff(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := blankExportRetryBase
	for i := 1; i < attempts; i++ {
		delay *= 2
		if delay >= blankExportRetryMax {
			return blankExportRetryMax
		}
	}
	return delay
}

// currentDir возвращает фактический каталог заявки по реестру.
func currentDir(registry map[int]*models.BlankExport) string {
	for _, row := range registry {
		if row.RelDir != "" {
			return row.RelDir
		}
	}
	return ""
}

// frozenDir возвращает каталог замороженных файлов заявки, если такие есть.
func frozenDir(registry map[int]*models.BlankExport) string {
	for _, row := range registry {
		if row.FrozenAt != nil && row.RelDir != "" {
			return row.RelDir
		}
	}
	return ""
}

func applicationIDOf(registry map[int]*models.BlankExport) int {
	for _, row := range registry {
		return row.ApplicationID
	}
	return 0
}

// splitRelDir разбирает хранимый относительный путь обратно на уровни. Разделитель
// всегда прямой слэш: путь собран path.Join, а не filepath.Join, и в базе он лежит
// в одном виде независимо от операционной системы.
func splitRelDir(relDir string) []string {
	if relDir == "" {
		return nil
	}
	return strings.Split(relDir, "/")
}
