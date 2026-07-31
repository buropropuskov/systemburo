package services

import (
	"context"
	"log/slog"
	"time"

	"systemburo/internal/models"
)

// Пределы одного прогона подметателя/сверки (B1). Без потолка единичная аномалия
// (огромный recheck_days, лавина failed-строк) держала бы воркер в многочасовом
// прогоне и не давала бы отвечать на Wake() между тиками ArchiveWorkerTick.
// Остаток разбирается следующим прогоном - оба джоба идемпотентны.
const (
	archiveSweepBatchLimit   = 200
	archiveRecheckBatchLimit = 2000
)

// ProcessQueue разбирает очередь enqueue: вызывается воркером на каждом тике
// ArchiveWorkerTick и по Nudge. Ошибка одной заявки не останавливает разбор
// остальных - иначе проблемная заявка держала бы очередь от всех, кто встал за ней.
func (s *BlankExportService) ProcessQueue(ctx context.Context) (processed, failed int) {
	pending := s.queue.drain()
	if len(pending) == 0 {
		return 0, 0
	}

	settings, err := s.settings.GetArchiveSettings(ctx)
	if err != nil {
		slog.Error("очередь файлового архива: не удалось прочитать настройки", "error", err)
		return 0, 0
	}
	if !settings.Enabled {
		// Выключенный рубильник не повод копить очередь до бесконечности молча:
		// заявки, вставшие в неё, просто пропадают - следующий реальный триггер
		// (правка, ручное пересоздание) поставит их заново, когда архив включат.
		return 0, 0
	}

	for id, reason := range pending {
		if _, err := s.ExportApplication(ctx, id, reason); err != nil {
			failed++
			slog.Error("выгрузка заявки из очереди файлового архива завершилась ошибкой",
				"application_id", id, "reason", reason, "error", err)
			continue
		}
		processed++
	}
	return processed, failed
}

// Sweep подбирает заявки, чья прошлая попытка выгрузки провалилась транзиентно и
// подошёл срок повтора (next_attempt_at по backoff из ExportApplication). Дополняет
// очередь на короткой дистанции: очередь ловит новые заявки сразу после commit,
// подметатель - те, что уже пробовались и не удались (интервал ArchiveSweepInterval).
//
// Известное ограничение: заявка, чья САМАЯ ПЕРВАЯ попытка выгрузки потерялась вместе
// с очередью (процесс перезапущен между commit и разбором), строки в blank_exports
// не оставляет вовсе - подметателю нечего подбирать. Такую заявку донесёт только
// ночная сверка (Recheck), с задержкой до recheck_days. Постановка в очередь и так
// намеренно best-effort (см. blank_export_queue.go) - полный диск не должен ронять
// подачу заявки, а редкая гонка перезапуска дешевле полноценного персистентного outbox.
func (s *BlankExportService) Sweep(ctx context.Context) (processed, failed int) {
	settings, err := s.settings.GetArchiveSettings(ctx)
	if err != nil {
		slog.Error("подметатель файлового архива: не удалось прочитать настройки", "error", err)
		return 0, 0
	}
	if !settings.Enabled {
		return 0, 0
	}

	var ids []int
	err = s.db.WithContext(ctx).Model(&models.BlankExport{}).
		Where("status = ? AND next_attempt_at IS NOT NULL AND next_attempt_at <= ?", models.BlankExportFailed, time.Now()).
		Distinct("application_id").
		Limit(archiveSweepBatchLimit).
		Pluck("application_id", &ids).Error
	if err != nil {
		slog.Error("подметатель файлового архива: не удалось выбрать заявки на повтор", "error", err)
		return 0, 0
	}

	return s.exportBatch(ctx, ids, BlankExportReasonSweep)
}

// Recheck - ночная сверка реестра с диском (03:00 по ResetTimezone). Полный прогон
// заявок в окне recheck_days независимо от состояния очереди: ловит заявки, чья
// самая первая постановка потерялась вместе с процессом, и заодно чинит расхождение
// реестра с диском (файл подменили или удалили руками) - ExportApplication
// перепроверяет каждую заявку с нуля и хэш-дедуп решает сам, писать ли заново.
//
// Тот же прогон закрывает долг A5c про слепок-сироту: заявка без единого бланка не
// заводит строк реестра для своих обычных вложений, но её заявка.json с B1 ведёт
// строку сам (archiveSnapshotAttachmentID), и relocate внутри ExportApplication
// подхватывает фактический путь слепка наравне с бланками - специального обхода
// каталога на диске не требуется.
func (s *BlankExportService) Recheck(ctx context.Context) (processed, failed int) {
	settings, err := s.settings.GetArchiveSettings(ctx)
	if err != nil {
		slog.Error("ночная сверка файлового архива: не удалось прочитать настройки", "error", err)
		return 0, 0
	}
	if !settings.Enabled {
		return 0, 0
	}
	days := settings.RecheckDays
	if days <= 0 {
		days = 1
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	// UNION вместо одного условия по applications: заявка могла лишиться строк
	// реестра (заморозка/сирота-статус не убирает их, но перестраховка не лишняя),
	// а строка реестра - пережить окно sending_datetime у самой заявки (bucket_date
	// у неё считается по дню подачи и не двигается вместе с правками).
	var ids []int
	err = s.db.WithContext(ctx).Raw(`
		SELECT id FROM applications WHERE sending_datetime >= ?
		UNION
		SELECT application_id FROM blank_exports WHERE bucket_date >= ?
		ORDER BY 1
		LIMIT ?
	`, cutoff, cutoff, archiveRecheckBatchLimit).Scan(&ids).Error
	if err != nil {
		slog.Error("ночная сверка файлового архива: не удалось выбрать заявки окна", "error", err)
		return 0, 0
	}

	return s.exportBatch(ctx, ids, BlankExportReasonRecheck)
}

// exportBatch прогоняет ExportApplication по списку заявок, не давая сбою одной
// прервать остальные - это фоновый прогон, а не запрос администратора, которому
// нужно вернуть ошибку ответом.
func (s *BlankExportService) exportBatch(ctx context.Context, applicationIDs []int, reason string) (processed, failed int) {
	for _, id := range applicationIDs {
		if _, err := s.ExportApplication(ctx, id, reason); err != nil {
			failed++
			slog.Error("фоновая выгрузка заявки в архив завершилась ошибкой",
				"application_id", id, "reason", reason, "error", err)
			continue
		}
		processed++
	}
	return processed, failed
}
