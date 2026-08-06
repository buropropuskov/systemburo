package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"systemburo/internal/diskspace"
	"systemburo/internal/models"

	"gorm.io/gorm"
)

// Место и квота файлового архива (#1615, срез B2).
//
// Три отдельные заботы живут в одном сервисе, потому что все три считаются от
// одного и того же снимка diskspace.Statfs и было бы расточительно снимать его
// трижды: сводка для интерфейса (Stats, кэш 5 минут), мягкое предупреждение
// администраторам и жёсткая остановка очереди выгрузки при нехватке места
// (EnforceThresholds - вызывается периодически будущим фоновым воркером, срез B1).
//
// Создание заявки эта остановка не затрагивает никак: очередь выгрузки бланков -
// отдельный путь от подачи заявки, и сервис здесь её не видит и не трогает.

// Тип уведомления -- NotificationTypeArchiveQuotaWarning (каталог, notification_catalog.go):
// предупреждение администраторам архива о приближении к порогу заполнения раздела.

// Ключи устойчивых флагов "уже сказали / уже остановили" в system_settings.
// Мимо knownKeys, как и archive.* настройки - это внутреннее состояние сервиса,
// не то, что правится через PUT /settings/:key.
//
// Флаг означает "действие перехода ВЫПОЛНЕНО", а не "переход замечен", и пишется
// только после подтверждённого успеха действия - см. crossEdge.
const (
	archiveWarnFlagKey  = "archive.warn_notified"
	archiveBlockFlagKey = "archive.blocked_notified"
)

// archiveBlockRetry - через сколько заблокированная строка реестра снова
// становится кандидатом на попытку. Пятнадцать минут, а не часы бэкоффа
// transient-ошибки: место на диске освобождают в тот же день, и часовая пауза
// продержала бы бланк невыгруженным дольше, чем нужно.
const archiveBlockRetry = 15 * time.Minute

// BlankExportQuotaService считает место на разделе архива, следит за порогами
// заполнения и переводит очередь выгрузки в blocked, когда места не хватает.
type BlankExportQuotaService struct {
	db       *gorm.DB
	settings SettingsService
	notifier NotificationService
	resolver *PermissionResolver
	recorder AuditRecorder

	archivePath string
	uploadPath  string
	logDir      string

	statsMu sync.Mutex
	stats   *models.ArchiveStats
	statsAt time.Time

	thresholdMu sync.Mutex
	warnFlag    thresholdState
	blockFlag   thresholdState
}

// thresholdState - устойчивое состояние одного порога ("уже сказали" /
// "уже остановили"), лениво подгруженное из system_settings при первом
// обращении в этом процессе.
type thresholdState struct {
	loaded bool
	active bool
}

// archiveStatsCacheTTL - сводка не пересчитывается на каждый запрос вкладки
// «Обзор»: агрегаты по реестру и обход каталогов загрузок/логов не бесплатны.
const archiveStatsCacheTTL = 5 * time.Minute

// NewBlankExportQuotaService создаёт сервис места и квоты. logFilePath - тот же
// путь, что и cfg.LogFilePath (путь к файлу, не к каталогу - каталог логов
// вычисляется отсюда); пустая строка означает "логи не пишутся в файл", и
// каталог логов в состав диска не входит вовсе.
func NewBlankExportQuotaService(
	db *gorm.DB,
	settings SettingsService,
	notifier NotificationService,
	resolver *PermissionResolver,
	recorder AuditRecorder,
	archivePath, uploadPath, logFilePath string,
) *BlankExportQuotaService {
	logDir := ""
	if strings.TrimSpace(logFilePath) != "" {
		logDir = filepath.Dir(logFilePath)
	}
	return &BlankExportQuotaService{
		db: db, settings: settings, notifier: notifier, resolver: resolver, recorder: recorder,
		archivePath: archivePath, uploadPath: uploadPath, logDir: logDir,
	}
}

// Stats отдаёт сводку файлового архива для вкладки «Обзор», из кэша, если снимок
// не старше archiveStatsCacheTTL.
func (q *BlankExportQuotaService) Stats(ctx context.Context) (*models.ArchiveStats, error) {
	q.statsMu.Lock()
	if q.stats != nil && time.Since(q.statsAt) < archiveStatsCacheTTL {
		cached := *q.stats
		q.statsMu.Unlock()
		return &cached, nil
	}
	q.statsMu.Unlock()

	stats, err := q.computeStats(ctx)
	if err != nil {
		return nil, err
	}

	q.statsMu.Lock()
	q.stats, q.statsAt = stats, time.Now()
	q.statsMu.Unlock()

	out := *stats
	return &out, nil
}

// statusCounts считает строки реестра по состояниям. Известные статусы попадают в
// ответ даже с нулём: пропавший из карты ключ фронт покажет пустотой, а «ноль
// пробелов» и «про пробелы не спросили» - разные сообщения администратору.
func (q *BlankExportQuotaService) statusCounts(ctx context.Context) (map[string]int64, error) {
	var rows []struct {
		Status string
		Total  int64
	}
	err := q.db.WithContext(ctx).Model(&models.BlankExport{}).
		Select("status, COUNT(*) AS total").Group("status").Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate archive statuses: %w", err)
	}

	out := make(map[string]int64, len(models.AllBlankExportStatuses))
	for _, name := range models.AllBlankExportStatuses {
		out[name] = 0
	}
	for _, r := range rows {
		out[r.Status] = r.Total
	}
	return out, nil
}

func (q *BlankExportQuotaService) computeStats(ctx context.Context) (*models.ArchiveStats, error) {
	var agg struct {
		Bytes int64
		Files int64
	}
	err := q.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("status = ?", models.BlankExportOK).
		Select("COALESCE(SUM(size_bytes), 0) AS bytes, COUNT(*) AS files").
		Scan(&agg).Error
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate archive stats: %w", err)
	}

	var periodRows []struct {
		Month string
		Bytes int64
		Files int64
	}
	err = q.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("status = ?", models.BlankExportOK).
		Select("to_char(date_trunc('month', bucket_date), 'YYYY-MM') AS month, " +
			"COALESCE(SUM(size_bytes), 0) AS bytes, COUNT(*) AS files").
		Group("date_trunc('month', bucket_date)").
		Order("date_trunc('month', bucket_date) DESC").
		Scan(&periodRows).Error
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate archive periods: %w", err)
	}
	periods := make([]models.ArchiveStatsPeriod, 0, len(periodRows))
	for _, r := range periodRows {
		periods = append(periods, models.ArchiveStatsPeriod{Month: r.Month, Bytes: r.Bytes, FileCount: r.Files})
	}

	statuses, err := q.statusCounts(ctx)
	if err != nil {
		return nil, err
	}

	composition, lastWrittenAt, err := q.composition(ctx)
	if err != nil {
		return nil, err
	}

	attachmentTypes, err := q.attachmentTypeBreakdown(ctx)
	if err != nil {
		return nil, err
	}

	disk, err := q.diskUsage(ctx, agg.Bytes)
	if err != nil {
		return nil, err
	}

	return &models.ArchiveStats{
		UsedBytes:       agg.Bytes,
		FreeBytes:       disk.FreeBytes,
		FileCount:       agg.Files,
		Periods:         periods,
		Statuses:        statuses,
		Composition:     composition,
		AttachmentTypes: attachmentTypes,
		LastWrittenAt:   lastWrittenAt,
		Disk:            disk,
		GeneratedAt:     time.Now(),
	}, nil
}

// composition разбирает file_count на заявки, бланки и служебные слепки и заодно
// отдаёт момент последней записи. Одним запросом, а не тремя: все четыре числа
// снимаются с одной выборки status=ok, и разъехаться между собой они не должны -
// «бланков больше, чем файлов» администратор прочитает как поломку.
//
// Слепок заявки живёт в реестре строкой с attachment_id = 0 (у него нет вложения),
// бланк - строкой с настоящим идентификатором вложения.
func (q *BlankExportQuotaService) composition(ctx context.Context) (models.ArchiveStatsComposition, *time.Time, error) {
	var row struct {
		Applications  int64
		Blanks        int64
		Snapshots     int64
		LastWrittenAt *time.Time
	}
	err := q.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("status = ?", models.BlankExportOK).
		Select("COUNT(DISTINCT application_id) AS applications, " +
			"COUNT(*) FILTER (WHERE attachment_id > 0) AS blanks, " +
			"COUNT(*) FILTER (WHERE attachment_id = 0) AS snapshots, " +
			"MAX(generated_at) AS last_written_at").
		Scan(&row).Error
	if err != nil {
		return models.ArchiveStatsComposition{}, nil, fmt.Errorf("failed to aggregate archive composition: %w", err)
	}

	return models.ArchiveStatsComposition{
		Applications: row.Applications,
		Blanks:       row.Blanks,
		Snapshots:    row.Snapshots,
	}, row.LastWrittenAt, nil
}

// attachmentTypeBreakdown считает, сколько занимают бланки каждого типа вложения.
//
// LEFT JOIN, а не INNER: реестр намеренно живёт без внешних ключей, и строка
// переживает удалённое вложение. INNER молча выбросил бы такие файлы, и сумма по
// типам не сошлась бы с числом бланков - расхождение без объяснения хуже строки
// «Вложение удалено», по которой видно, что файлы на диске есть, а сущности нет.
func (q *BlankExportQuotaService) attachmentTypeBreakdown(ctx context.Context) ([]models.ArchiveStatsAttachmentType, error) {
	sql := `
		SELECT CASE
		           WHEN at.id IS NULL THEN 'Вложение удалено'
		           ELSE COALESCE(NULLIF(` + archiveAttachmentNameExpr + `, ''), 'Без наименования')
		       END AS name,
		       COALESCE(SUM(be.size_bytes), 0) AS bytes,
		       COUNT(*) AS file_count
		FROM blank_exports be
		LEFT JOIN attachments at ON at.id = be.attachment_id
		LEFT JOIN unique_attachments ua ON ua.id = at.unique_attachment_id
		WHERE be.status = ? AND be.attachment_id > 0
		GROUP BY 1
		ORDER BY bytes DESC, name ASC`

	var rows []struct {
		Name      string
		Bytes     int64
		FileCount int64
	}
	if err := q.db.WithContext(ctx).Raw(sql, models.BlankExportOK).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("failed to aggregate archive attachment types: %w", err)
	}

	out := make([]models.ArchiveStatsAttachmentType, 0, len(rows))
	for _, r := range rows {
		out = append(out, models.ArchiveStatsAttachmentType{Name: r.Name, Bytes: r.Bytes, FileCount: r.FileCount})
	}
	return out, nil
}

// diskUsage считает состав занятого места на разделе, которому принадлежит
// корень архива. archiveBytes - уже посчитанная сумма реестра (Stats и
// EnforceThresholds считают её один раз и передают сюда/рядом, а не дважды).
func (q *BlankExportQuotaService) diskUsage(ctx context.Context, archiveBytes int64) (models.ArchiveDiskUsage, error) {
	usage, err := diskspace.Statfs(q.archivePath)
	if err != nil {
		return models.ArchiveDiskUsage{}, fmt.Errorf("failed to stat archive partition: %w", err)
	}

	dirs := []diskspace.Dir{
		{Label: "Архив", Path: q.archivePath},
		{Label: "Загрузки", Path: q.uploadPath},
	}
	if q.logDir != "" {
		dirs = append(dirs, diskspace.Dir{Label: "Логи", Path: q.logDir})
	}
	partitions := diskspace.Collect(dirs)
	partitionInfos := make([]models.ArchiveDiskPartition, 0, len(partitions))
	for _, p := range partitions {
		partitionInfos = append(partitionInfos, models.ArchiveDiskPartition{
			Labels: p.Labels, TotalBytes: p.TotalBytes, FreeBytes: p.FreeBytes,
		})
	}

	// Загрузки/логи учитываются в составе ЭТОГО раздела, только если их
	// собственный статфс подтверждает то же устройство: иначе "занято" сложило бы
	// байты каталога с чужого раздела в отчёт про архивный (docker-compose.base.yml
	// монтирует архив bind-mount-ом, а загрузки - именованным томом, они не
	// обязаны совпадать физически).
	var uploadsBytes, logsBytes int64
	if uploadUsage, err := diskspace.Statfs(q.uploadPath); err == nil && uploadUsage.Device == usage.Device {
		if b, err := diskspace.DirSize(q.uploadPath); err == nil {
			uploadsBytes = b
		} else {
			slog.Warn("archive stats: не удалось посчитать размер каталога загрузок", "path", q.uploadPath, "error", err)
		}
	}
	if q.logDir != "" {
		if logUsage, err := diskspace.Statfs(q.logDir); err == nil && logUsage.Device == usage.Device {
			if b, err := diskspace.DirSize(q.logDir); err == nil {
				logsBytes = b
			} else {
				slog.Warn("archive stats: не удалось посчитать размер каталога логов", "path", q.logDir, "error", err)
			}
		}
	}

	dbBytes, err := q.databaseSizeBytes(ctx)
	if err != nil {
		slog.Warn("archive stats: не удалось получить размер базы", "error", err)
	}

	other := usage.UsedBytes() - archiveBytes - uploadsBytes - logsBytes - dbBytes
	if other < 0 {
		other = 0
	}

	return models.ArchiveDiskUsage{
		TotalBytes:    usage.TotalBytes,
		FreeBytes:     usage.FreeBytes,
		ArchiveBytes:  archiveBytes,
		UploadsBytes:  uploadsBytes,
		DatabaseBytes: dbBytes,
		LogsBytes:     logsBytes,
		OtherBytes:    other,
		Partitions:    partitionInfos,
	}, nil
}

// databaseSizeBytes - справочный размер базы (см. ArchiveDiskUsage.DatabaseBytes).
func (q *BlankExportQuotaService) databaseSizeBytes(ctx context.Context) (int64, error) {
	var bytes int64
	err := q.db.WithContext(ctx).Raw("SELECT pg_database_size(current_database())").Scan(&bytes).Error
	if err != nil {
		return 0, fmt.Errorf("failed to read database size: %w", err)
	}
	return bytes, nil
}

// QuotaEnforcementResult - что сделал один прогон EnforceThresholds.
type QuotaEnforcementResult struct {
	SoftTripped bool
	HardTripped bool
	// BlockedRows - сколько строк реестра ушло в blocked ИМЕННО этим прогоном
	// (0, если жёсткий порог уже был отмечен раньше - строки блокируются один раз
	// на переход через край, не на каждый тик).
	BlockedRows int64
}

// EnforceThresholds - периодическая проверка порогов места (#1615, B2).
// Вызывается фоновым воркером (срез B1); сам по себе таймер не заводит.
//
// Мягкий порог (WarnPercent) - предупреждение администраторам один раз на
// переход через край. Жёсткий (MinFreeBytes свободного места или QuotaBytes
// объёма архива) - строки очереди (pending/failed) переводятся в blocked с
// повтором через 15 минут; уже лежащие файлы не трогаются, создание заявки эта
// остановка не видит вовсе - речь только про очередь выгрузки бланков.
func (q *BlankExportQuotaService) EnforceThresholds(ctx context.Context) (*QuotaEnforcementResult, error) {
	settings, err := q.settings.GetArchiveSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Enabled {
		// Выключенный архив не пишет и не копит очередь - сторожить нечего.
		return &QuotaEnforcementResult{}, nil
	}

	usage, err := diskspace.Statfs(q.archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat archive partition: %w", err)
	}
	archiveBytes, err := q.archiveRegistryBytes(ctx)
	if err != nil {
		return nil, err
	}

	result := &QuotaEnforcementResult{SoftTripped: softThresholdTripped(usage, settings)}
	var reason string
	result.HardTripped, reason = hardThresholdTripped(usage, archiveBytes, settings)

	// Мягкий порог: провал уведомления не рвёт прогон - жёсткий порог важнее и
	// должен быть проверен независимо. Отметка при этом не сдвинется, и следующий
	// тик повторит попытку.
	if err := q.crossEdge(ctx, &q.warnFlag, archiveWarnFlagKey, result.SoftTripped, func(crossedIn bool) error {
		if !crossedIn {
			// Возврат ниже порога сам по себе действия не требует: снятая отметка и
			// есть всё событие - следующее пересечение снова разбудит уведомление.
			return nil
		}
		return q.notifyAdmins(ctx, usage, settings)
	}); err != nil {
		slog.Warn("archive quota: предупреждение о заполнении не отправлено, повторим на следующем тике", "error", err)
	}

	err = q.crossEdge(ctx, &q.blockFlag, archiveBlockFlagKey, result.HardTripped, func(crossedIn bool) error {
		if !crossedIn {
			// Снятие блокировки видно администратору только этой записью - она и
			// есть действие перехода, поэтому её провал откладывает отметку.
			return q.recorder.Record(ctx, nil, models.AuditEntityArchiveQuota, nil,
				models.ArchiveQuotaActionUnblocked, nil, nil)
		}
		blocked, err := q.blockQueue(ctx, reason)
		if err != nil {
			return err
		}
		result.BlockedRows = blocked
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to enforce archive hard threshold: %w", err)
	}
	return result, nil
}

// archiveRegistryBytes - SUM(size_bytes) живых записанных файлов реестра.
func (q *BlankExportQuotaService) archiveRegistryBytes(ctx context.Context) (int64, error) {
	var bytes int64
	err := q.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("status = ?", models.BlankExportOK).
		Select("COALESCE(SUM(size_bytes), 0)").Scan(&bytes).Error
	if err != nil {
		return 0, fmt.Errorf("failed to sum archive registry bytes: %w", err)
	}
	return bytes, nil
}

// blockQueue переводит строки очереди в blocked с ретраем через
// archiveBlockRetry. Уже лежащие файлы (status=ok) не трогаются - блокировка
// про то, что ЕЩЁ предстоит записать, а не про то, что уже на диске.
//
// Строки и запись журнала идут одной транзакцией (Record с exec=tx, паттерн
// blacklist): остановка очереди без следа в журнале выглядит для администратора
// как "выгрузка сама перестала работать", а откат вернёт строки в очередь, и
// следующий тик повторит блокировку с честным числом строк.
func (q *BlankExportQuotaService) blockQueue(ctx context.Context, reason string) (int64, error) {
	next := time.Now().Add(archiveBlockRetry)
	var affected int64
	err := q.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.BlankExport{}).
			Where("status IN ?", []string{models.BlankExportPending, models.BlankExportFailed}).
			Updates(map[string]any{
				"status":          models.BlankExportBlocked,
				"next_attempt_at": next,
				"last_error":      "недостаточно места в файловом архиве: " + reason,
			})
		if res.Error != nil {
			return fmt.Errorf("failed to block archive export queue: %w", res.Error)
		}
		affected = res.RowsAffected
		return q.recorder.Record(ctx, tx, models.AuditEntityArchiveQuota, nil,
			models.ArchiveQuotaActionBlocked, nil,
			map[string]any{"reason": reason, "rows": res.RowsAffected})
	})
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// crossEdge выполняет действие перехода через край порога и только после его
// подтверждённого успеха фиксирует устойчивую отметку. onCross вызывается ИМЕННО
// на переходе, а не при "условие всё ещё истинно/ложно" - иначе каждый тик
// воркера повторно уведомлял бы или заново блокировал бы уже заблокированные
// строки; crossedIn=true означает переход в нарушение порога, false - возврат.
//
// Порядок именно такой: отметка, поставленная ДО действия, при упавшем действии
// (или рестарте процесса между шагами) сказала бы следующему тику "уже сделано",
// и очередь так и не ушла бы в blocked при реально переполненном диске - перехода
// больше не будет, пока место не вернётся и снова не кончится. Провалилось
// действие - отметка не двигается, и следующий тик повторяет попытку.
//
// Отметка переживает рестарт процесса через system_settings: без этого рестарт
// при всё ещё критичном месте "забыл" бы, что уже предупредил, и продублировал
// бы уведомление на первом же тике.
//
// Мьютекс держится и на время действия: два одновременных прогона иначе оба
// увидели бы переход и оба заблокировали бы очередь (и прислали по уведомлению).
// onCross по этой же причине не имеет права вызывать crossEdge повторно.
func (q *BlankExportQuotaService) crossEdge(ctx context.Context, state *thresholdState, key string, tripped bool, onCross func(crossedIn bool) error) error {
	q.thresholdMu.Lock()
	defer q.thresholdMu.Unlock()

	if !state.loaded {
		state.active = q.loadFlag(ctx, key)
		state.loaded = true
	}
	if tripped == state.active {
		return nil
	}
	if onCross != nil {
		if err := onCross(tripped); err != nil {
			return err
		}
	}
	if err := q.saveFlag(ctx, key, tripped); err != nil {
		return fmt.Errorf("failed to persist threshold flag %s: %w", key, err)
	}
	state.active = tripped
	return nil
}

func (q *BlankExportQuotaService) loadFlag(ctx context.Context, key string) bool {
	var setting models.SystemSetting
	if err := q.db.WithContext(ctx).Where("key = ?", key).First(&setting).Error; err != nil {
		return false
	}
	return setting.Value == "true"
}

func (q *BlankExportQuotaService) saveFlag(ctx context.Context, key string, active bool) error {
	value := strconv.FormatBool(active)
	var existing models.SystemSetting
	err := q.db.WithContext(ctx).Where("key = ?", key).First(&existing).Error
	switch {
	case err == nil:
		existing.Value = value
		return q.db.WithContext(ctx).Save(&existing).Error
	case errors.Is(err, gorm.ErrRecordNotFound):
		return q.db.WithContext(ctx).Create(&models.SystemSetting{Key: key, Value: value, Type: "bool"}).Error
	default:
		return fmt.Errorf("failed to load threshold flag %s: %w", key, err)
	}
}

// notifyAdmins шлёт предупреждение о заполнении носителям права управления
// архивом (KeyActionManageFileArchive) - тем же, кто видит настройки и решает,
// что с этим делать.
//
// Ошибка возвращается только когда предупреждение не дошло НИ ДО КОГО: отметка
// "уже предупредили" ставится по успеху (см. crossEdge), и провал на одном
// администраторе из пяти заставил бы следующий тик прислать остальным четверым
// второе уведомление о том же событии. Некому предупреждать (право никому не
// выдано) - не сбой: повторять такое каждый тик бессмысленно.
func (q *BlankExportQuotaService) notifyAdmins(ctx context.Context, usage diskspace.Usage, settings *models.ArchiveSettings) error {
	if q.notifier == nil || q.resolver == nil {
		return nil
	}
	audience, err := q.archiveAdmins(ctx)
	if err != nil {
		return fmt.Errorf("failed to collect archive quota audience: %w", err)
	}
	if len(audience) == 0 {
		slog.Warn("archive quota: архив заполняется, но право управления им никому не выдано")
		return nil
	}
	title := "Файловый архив заполняется"
	body := fmt.Sprintf("Раздел архива заполнен на %.0f%% (порог %d%%), свободно %s.",
		usage.UsedPercent(), settings.WarnPercent, formatBytesRu(usage.FreeBytes))
	var delivered int
	var lastErr error
	for _, userID := range audience {
		if err := q.notifier.CreateForUser(ctx, userID, NotificationTypeArchiveQuotaWarning, title, body, nil); err != nil {
			slog.Warn("archive quota: не удалось уведомить о заполнении архива", "user_id", userID, "error", err)
			lastErr = err
			continue
		}
		delivered++
	}
	if delivered == 0 {
		return fmt.Errorf("failed to notify archive admins (%d recipients): %w", len(audience), lastErr)
	}
	return nil
}

// archiveAdmins - активные пользователи с правом управления файловым архивом.
// Кандидаты - все активные аккаунты (тот же источник, что и в table_audience.go),
// резолвер - единственный источник истины про право (super/admin/грант учтены).
func (q *BlankExportQuotaService) archiveAdmins(ctx context.Context) ([]int, error) {
	ids, err := activeUserIDs(ctx, q.db)
	if err != nil {
		return nil, err
	}
	audience := make([]int, 0, len(ids))
	for _, uid := range ids {
		set, err := q.resolver.Resolve(ctx, uid)
		if err != nil {
			slog.Warn("archive quota: резолв прав не удался", "user_id", uid, "error", err)
			continue
		}
		if set.Has(KeyActionManageFileArchive) {
			audience = append(audience, uid)
		}
	}
	return audience, nil
}

// softThresholdTripped - доля заполнения раздела архива достигла WarnPercent.
func softThresholdTripped(usage diskspace.Usage, settings *models.ArchiveSettings) bool {
	if settings.WarnPercent <= 0 {
		return false
	}
	return usage.UsedPercent() >= float64(settings.WarnPercent)
}

// hardThresholdTripped - свободного места меньше MinFreeBytes ИЛИ архив достиг
// QuotaBytes (0 у обоих полей значит "порог не задан", а не "нулевой лимит").
//
// Строгость сравнений разная намеренно, это не опечатка: MinFreeBytes - сколько
// обязано ОСТАТЬСЯ свободным, и ровно минимум ещё выполняет обещание, а QuotaBytes -
// потолок объёма архива, и достигнутый потолок уже исчерпан, писать дальше нельзя.
func hardThresholdTripped(usage diskspace.Usage, archiveBytes int64, settings *models.ArchiveSettings) (bool, string) {
	// Строго <: свободного места ровно минимум - порог ещё соблюдён.
	if settings.MinFreeBytes > 0 && usage.FreeBytes < settings.MinFreeBytes {
		return true, "insufficient_free_space"
	}
	// Нестрого >=: архив ровно на потолке квоты - место под следующий файл уже
	// выбрано, и очередь обязана встать.
	if settings.QuotaBytes > 0 && archiveBytes >= settings.QuotaBytes {
		return true, "quota_exceeded"
	}
	return false, ""
}

// formatBytesRu - минимальный форматтер для текста уведомления. Не канонический
// (тот на фронте, frontend/src/utils/download.js) - здесь достаточно человеку
// понятной прикидки внутри серверного текста.
func formatBytesRu(n int64) string {
	switch {
	case n < 1024:
		return fmt.Sprintf("%d Б", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.0f КБ", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f МБ", float64(n)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f ГБ", float64(n)/(1024*1024*1024))
	}
}
