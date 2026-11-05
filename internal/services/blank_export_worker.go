package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm"
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

// ArchiveQuotaGuard - проверка порогов места перед записью в архив (#1615, B2).
// Интерфейс, а не *BlankExportQuotaService: фоновому прогону нужен ровно один метод,
// а сводка диска и рассылка предупреждений администраторам ему не нужны вовсе.
type ArchiveQuotaGuard interface {
	EnforceThresholds(ctx context.Context) (*QuotaEnforcementResult, error)
}

// spaceAvailable прогоняет пороги места и отвечает, можно ли писать в архив прямо
// сейчас. Зовётся ПЕРЕД каждым фоновым прогоном записи: смысл жёсткого порога в том,
// чтобы узнать про нехватку места до записи, а не по факту сбоя записи. Он же
// уводит очередь в blocked и предупреждает администраторов - без этого вызова вся
// защита B2 остаётся мёртвым кодом.
//
// Сбой самой проверки прогон не отменяет: сломанный statfs или недоступная таблица
// настроек иначе тихо остановили бы выгрузку целиком. Ошибка уходит в лог громко, а
// решение принимается в пользу записи - реальная нехватка места всё равно вернётся
// ошибкой записи и уведёт строку в повтор по backoff.
func (s *BlankExportService) spaceAvailable(ctx context.Context, stage string) bool {
	if s.quota == nil {
		return true
	}
	res, err := s.quota.EnforceThresholds(ctx)
	if err != nil {
		slog.Error("файловый архив: проверка порогов места не удалась, продолжаем запись",
			"stage", stage, "error", err)
		return true
	}
	if res.HardTripped {
		slog.Warn("файловый архив: места недостаточно, запись пропущена",
			"stage", stage, "blocked_rows", res.BlockedRows)
		return false
	}
	return true
}

// ProcessQueue advances at most one persisted scan page and processes one job batch.
// A concurrent call returns immediately; each job remains durable until its ACK.
func (s *BlankExportService) ProcessQueue(ctx context.Context) (processed, failed int) {
	if s == nil || s.outbox == nil || s.writer == nil || ctx.Err() != nil || !s.processing.CompareAndSwap(false, true) {
		return 0, 0
	}
	defer s.processing.Store(false)
	jobs := s.prepareArchiveJobs(ctx)
	for _, job := range jobs {
		if ctx.Err() != nil {
			break
		}
		done, unsuccessful, stop := s.processArchiveJob(ctx, job)
		processed += done
		failed += unsuccessful
		if stop {
			break
		}
	}
	return processed, failed
}

// Preparation does not lend an expiring context to the whole 200-job batch.
// Its deadline covers the existing scan transaction and bounded queue read.
func (s *BlankExportService) prepareArchiveJobs(parent context.Context) []models.ArchiveExportJob {
	ctx, cancel := ResourcePreparationContext(parent)
	defer cancel()
	settings, err := s.settings.GetArchiveSettings(ctx)
	if err != nil {
		slog.Error("archive worker settings unavailable", "error", err)
		return nil
	}
	if !settings.Enabled {
		return nil
	}
	if _, err := s.outbox.AdvanceRecheck(ctx, archiveRecheckBatchLimit); err != nil {
		slog.Error("archive scan page failed", "error", err)
		// Already queued work can still progress after a scan failure.
	}
	if ctx.Err() != nil || !s.spaceAvailable(ctx, "очередь") {
		return nil
	}
	jobs, err := s.outbox.ListDue(ctx, archiveSweepBatchLimit)
	if err != nil {
		slog.Error("archive job selection failed", "error", err)
		return nil
	}
	return jobs
}

// Each job gets its own finite lifetime. Settlement starts from the live worker
// parent, so a timed-out export can still persist retry/backoff independently.
func (s *BlankExportService) processArchiveJob(parent context.Context, job models.ArchiveExportJob) (processed, failed int, stop bool) {
	ctx, cancel := ResourcePreparationContext(parent)
	defer cancel()
	// A missing dependency or an arbitrary 404 must not discard a job.
	// Only a successful query proving the application absent is terminal.
	var applications int64
	err := s.db.WithContext(ctx).Model(&models.Application{}).Where("id = ?", job.ApplicationID).Count(&applications).Error
	if parent.Err() != nil {
		return 0, 0, true
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err == nil && applications == 0 {
		err = s.db.WithContext(ctx).Model(&models.BlankExport{}).
			Where("application_id = ? AND status <> ?", job.ApplicationID, models.BlankExportPurged).
			Updates(map[string]interface{}{"status": models.BlankExportOrphan, "next_attempt_at": nil}).Error
		if parent.Err() != nil {
			return 0, 0, true
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		if err == nil {
			cancel()
			if err := settleArchiveJob(parent, func(ctx context.Context) error { return s.outbox.Ack(ctx, job) }); err != nil {
				failed++
				slog.Error("archive missing application ACK failed", "error", err)
			} else {
				processed++
			}
			return processed, failed, parent.Err() != nil
		}
	}
	var result *models.BlankExportResult
	if err == nil {
		result, err = s.ExportApplication(ctx, job.ApplicationID, job.Reason)
	}
	if parent.Err() != nil {
		return 0, 0, true
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	// File/SQL work is finished; release its timer before fresh settlement.
	cancel()
	if errors.Is(err, ErrArchiveDisabled) {
		return 0, 0, true
	}
	if errors.Is(err, ErrArchivePurged) || errors.Is(err, ErrArchiveDestructionPending) {
		if err := settleArchiveJob(parent, func(ctx context.Context) error { return s.outbox.Suppress(ctx, job.ApplicationID) }); err != nil {
			failed++
			slog.Error("archive terminal job suppression failed", "error", err)
		}
		return processed, failed, parent.Err() != nil
	}
	if err != nil || !archiveExportComplete(result) {
		failed++
		slog.Error("archive job incomplete", "application_id", job.ApplicationID, "error", err)
		if retryErr := settleArchiveJob(parent, func(ctx context.Context) error { return s.outbox.Retry(ctx, job) }); retryErr != nil {
			slog.Error("archive job retry scheduling failed", "error", retryErr)
		}
		return processed, failed, parent.Err() != nil
	}
	if err := settleArchiveJob(parent, func(ctx context.Context) error { return s.outbox.Ack(ctx, job) }); err != nil {
		failed++
		slog.Error("archive job ACK failed", "error", err)
		return processed, failed, parent.Err() != nil
	}
	processed++
	return processed, failed, parent.Err() != nil
}

func settleArchiveJob(parent context.Context, action func(context.Context) error) error {
	ctx, cancel := ResourcePreparationContext(parent)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	return action(ctx)
}

// Successful outcomes include intentional skips whose registry writes succeeded.
// Snapshot is mandatory even for applications with no ordinary attachments.
func archiveExportComplete(result *models.BlankExportResult) bool {
	if result == nil || result.Snapshot.Status != models.BlankExportOK {
		return false
	}
	for _, item := range result.Items {
		switch item.Status {
		case models.BlankExportOK, models.BlankExportSkipped, models.BlankExportNoTemplate, models.BlankExportOrphan:
		default:
			return false
		}
	}
	return true
}

// Sweep imports one bounded batch of legacy failed/blocked registry entries.
// Existing durable jobs retain their revision and backoff.
func (s *BlankExportService) Sweep(ctx context.Context) (processed, failed int) {
	if s == nil || s.outbox == nil || ctx.Err() != nil {
		return 0, 0
	}
	prepCtx, cancelPreparation := ResourcePreparationContext(ctx)
	defer cancelPreparation()
	settings, err := s.settings.GetArchiveSettings(prepCtx)
	if err != nil || !settings.Enabled {
		if err != nil {
			slog.Error("archive sweep settings unavailable", "error", err)
		}
		return 0, 0
	}
	err = s.db.WithContext(prepCtx).Transaction(func(tx *gorm.DB) error {
		_, err := s.outbox.EnqueueSelectionTx(prepCtx, tx, `
			SELECT DISTINCT b.application_id
			FROM blank_exports b
			WHERE b.status IN ? AND b.next_attempt_at IS NOT NULL AND b.next_attempt_at <= ?
			AND NOT EXISTS (SELECT 1 FROM archive_export_jobs j WHERE j.application_id = b.application_id)
			ORDER BY b.application_id LIMIT ?`,
			[]interface{}{[]string{models.BlankExportFailed, models.BlankExportBlocked}, time.Now(), archiveSweepBatchLimit}, BlankExportReasonSweep)
		return err
	})
	if err != nil {
		slog.Error("archive legacy sweep enqueue failed", "error", err)
		return 0, 0
	}
	s.outbox.Nudge()
	cancelPreparation()
	return s.ProcessQueue(ctx)
}

// Recheck starts a persisted bounded scan. ProcessQueue continues an active scan
// on later ticks, including after restart; a second nightly signal cannot reset it.
func (s *BlankExportService) Recheck(ctx context.Context) (processed, failed int) {
	if s == nil || s.outbox == nil || ctx.Err() != nil {
		return 0, 0
	}
	prepCtx, cancelPreparation := ResourcePreparationContext(ctx)
	defer cancelPreparation()
	settings, err := s.settings.GetArchiveSettings(prepCtx)
	if err != nil || !settings.Enabled {
		if err != nil {
			slog.Error("archive recheck settings unavailable", "error", err)
		}
		return 0, 0
	}
	days := settings.RecheckDays
	if days <= 0 {
		days = 1
	}
	if err := s.outbox.BeginRecheck(prepCtx, time.Now().AddDate(0, 0, -days)); err != nil {
		slog.Error("archive scan initialization failed", "error", err)
		return 0, 0
	}
	cancelPreparation()
	return s.ProcessQueue(ctx)
}
