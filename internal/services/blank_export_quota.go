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

// NotificationTypeArchiveQuotaWarning - предупреждение администраторам архива о
// приближении к порогу заполнения раздела.
const NotificationTypeArchiveQuotaWarning = "archive_quota_warning"

// Ключи устойчивых флагов "уже сказали / уже остановили" в system_settings.
// Мимо knownKeys, как и archive.* настройки - это внутреннее состояние сервиса,
// не то, что правится через PUT /settings/:key.
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

	disk, err := q.diskUsage(ctx, agg.Bytes)
	if err != nil {
		return nil, err
	}

	return &models.ArchiveStats{
		UsedBytes:   agg.Bytes,
		FreeBytes:   disk.FreeBytes,
		FileCount:   agg.Files,
		Periods:     periods,
		Disk:        disk,
		GeneratedAt: time.Now(),
	}, nil
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

	if crossedIn, _, err := q.crossEdge(ctx, &q.warnFlag, archiveWarnFlagKey, result.SoftTripped); err != nil {
		// Уведомление - не тот случай, ради которого стоит рвать весь прогон:
		// жёсткий порог важнее и должен быть проверен независимо от того,
		// сохранился ли флаг мягкого.
		slog.Warn("archive quota: не удалось сохранить отметку мягкого порога", "error", err)
	} else if crossedIn {
		q.notifyAdmins(ctx, usage, settings)
	}

	blockedIn, blockedOut, err := q.crossEdge(ctx, &q.blockFlag, archiveBlockFlagKey, result.HardTripped)
	if err != nil {
		return nil, fmt.Errorf("failed to persist archive block flag: %w", err)
	}
	switch {
	case blockedIn:
		blocked, err := q.blockQueue(ctx, reason)
		if err != nil {
			return nil, err
		}
		result.BlockedRows = blocked
	case blockedOut:
		q.recorder.Log(ctx, nil, models.AuditEntityArchiveQuota, nil, models.ArchiveQuotaActionUnblocked, nil, nil)
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
func (q *BlankExportQuotaService) blockQueue(ctx context.Context, reason string) (int64, error) {
	next := time.Now().Add(archiveBlockRetry)
	res := q.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("status IN ?", []string{models.BlankExportPending, models.BlankExportFailed}).
		Updates(map[string]any{
			"status":          models.BlankExportBlocked,
			"next_attempt_at": next,
			"last_error":      "недостаточно места в файловом архиве: " + reason,
		})
	if res.Error != nil {
		return 0, fmt.Errorf("failed to block archive export queue: %w", res.Error)
	}
	q.recorder.Log(ctx, nil, models.AuditEntityArchiveQuota, nil, models.ArchiveQuotaActionBlocked, nil,
		map[string]any{"reason": reason, "rows": res.RowsAffected})
	return res.RowsAffected, nil
}

// crossEdge переводит устойчивый флаг threshold-а в новое состояние и сообщает,
// произошёл ли ИМЕННО переход через край, а не "условие всё ещё истинно/ложно" -
// иначе каждый тик воркера повторно уведомлял бы или заново блокировал бы уже
// заблокированные строки. Состояние переживает рестарт процесса через
// system_settings: без этого рестарт при всё ещё критичном месте "забыл" бы,
// что уже предупредил, и продублировал бы уведомление на первом же тике.
func (q *BlankExportQuotaService) crossEdge(ctx context.Context, state *thresholdState, key string, tripped bool) (crossedIn, crossedOut bool, err error) {
	q.thresholdMu.Lock()
	defer q.thresholdMu.Unlock()

	if !state.loaded {
		state.active = q.loadFlag(ctx, key)
		state.loaded = true
	}
	if tripped == state.active {
		return false, false, nil
	}
	if err := q.saveFlag(ctx, key, tripped); err != nil {
		return false, false, err
	}
	state.active = tripped
	return tripped, !tripped, nil
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
// что с этим делать. Best-effort: сбой уведомления не должен рвать проверку
// жёсткого порога, которая идёт следом.
func (q *BlankExportQuotaService) notifyAdmins(ctx context.Context, usage diskspace.Usage, settings *models.ArchiveSettings) {
	if q.notifier == nil || q.resolver == nil {
		return
	}
	audience, err := q.archiveAdmins(ctx)
	if err != nil {
		slog.Warn("archive quota: аудитория предупреждения о месте не собралась", "error", err)
		return
	}
	title := "Файловый архив заполняется"
	body := fmt.Sprintf("Раздел архива заполнен на %.0f%% (порог %d%%), свободно %s.",
		usage.UsedPercent(), settings.WarnPercent, formatBytesRu(usage.FreeBytes))
	for _, userID := range audience {
		if err := q.notifier.CreateForUser(ctx, userID, NotificationTypeArchiveQuotaWarning, title, body, nil); err != nil {
			slog.Warn("archive quota: не удалось уведомить о заполнении архива", "user_id", userID, "error", err)
		}
	}
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
func hardThresholdTripped(usage diskspace.Usage, archiveBytes int64, settings *models.ArchiveSettings) (bool, string) {
	if settings.MinFreeBytes > 0 && usage.FreeBytes < settings.MinFreeBytes {
		return true, "insufficient_free_space"
	}
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
