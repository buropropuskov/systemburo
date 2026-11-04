package services

import (
	"context"
	"sort"
	"strings"
	"time"

	"systemburo/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ArchiveExportOutbox сохраняет задания независимо от доступности файлового писателя.
type ArchiveExportOutbox struct {
	db   *gorm.DB
	wake chan struct{}
}

func NewArchiveExportOutbox(db *gorm.DB) *ArchiveExportOutbox {
	return &ArchiveExportOutbox{db: db, wake: make(chan struct{}, 1)}
}

func (s *ArchiveExportOutbox) Wake() <-chan struct{} { return s.wake }

func (s *ArchiveExportOutbox) Nudge() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *ArchiveExportOutbox) Enqueue(ctx context.Context, ids []int, reason string) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.EnqueueTx(ctx, tx, ids, reason)
	})
	if err == nil && len(ids) > 0 {
		s.Nudge()
	}
	return err
}

// EnqueueTx блокирует строки в одном порядке; количество параметров SQL ограничено.
// Пробуждение после commit остаётся обязанностью вызывающего кода.
func (s *ArchiveExportOutbox) EnqueueTx(ctx context.Context, tx *gorm.DB, ids []int, reason string) error {
	ordered := make([]int, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			ordered = append(ordered, id)
		}
	}
	sort.Ints(ordered)
	unique := ordered[:0]
	for _, id := range ordered {
		if len(unique) == 0 || unique[len(unique)-1] != id {
			unique = append(unique, id)
		}
	}
	const batchSize = 1000
	for start := 0; start < len(unique); start += batchSize {
		end := start + batchSize
		if end > len(unique) {
			end = len(unique)
		}
		placeholders := make([]string, end-start)
		args := make([]interface{}, 0, end-start)
		for i, id := range unique[start:end] {
			placeholders[i] = "(?::bigint)"
			args = append(args, id)
		}
		selection := "SELECT column1 AS application_id FROM (VALUES " + strings.Join(placeholders, ",") + ") AS selected"
		if _, err := s.EnqueueSelectionTx(ctx, tx, selection, args, reason); err != nil {
			return err
		}
	}
	return nil
}

// EnqueueSelectionTx принимает только внутренний SQL, возвращающий application_id.
// Выборка и UPSERT выполняются одним statement, без полного списка ID в памяти Go.
func (s *ArchiveExportOutbox) EnqueueSelectionTx(ctx context.Context, tx *gorm.DB, selectSQL string, args []interface{}, reason string) (int64, error) {
	query := `INSERT INTO archive_export_jobs
		(application_id, revision, state, reason, queued_at, next_attempt_at, attempts)
		SELECT selected.application_id, ?, 'pending', ?, ?, ?, 0
		FROM (SELECT DISTINCT application_id FROM (` + selectSQL + `) AS source
		      WHERE application_id > 0) AS selected
		ORDER BY selected.application_id
		ON CONFLICT (application_id) DO UPDATE SET revision = EXCLUDED.revision,
		state = EXCLUDED.state, reason = EXCLUDED.reason, queued_at = EXCLUDED.queued_at,
		next_attempt_at = EXCLUDED.next_attempt_at, attempts = 0
		WHERE archive_export_jobs.state <> 'suppressed'`
	now := time.Now()
	params := []interface{}{uuid.NewString(), reason, now, now}
	params = append(params, args...)
	result := tx.WithContext(ctx).Exec(query, params...)
	return result.RowsAffected, result.Error
}

func (s *ArchiveExportOutbox) ListDue(ctx context.Context, limit int) ([]models.ArchiveExportJob, error) {
	if limit <= 0 || limit > archiveSweepBatchLimit {
		limit = archiveSweepBatchLimit
	}
	var jobs []models.ArchiveExportJob
	err := s.db.WithContext(ctx).Where("state = ? AND next_attempt_at <= ?", models.ArchiveExportJobPending, time.Now()).
		Order("next_attempt_at, queued_at, application_id").Limit(limit).Find(&jobs).Error
	return jobs, err
}

func (s *ArchiveExportOutbox) Ack(ctx context.Context, job models.ArchiveExportJob) error {
	return s.db.WithContext(ctx).Where("application_id = ? AND revision = ? AND state = ?", job.ApplicationID, job.Revision, models.ArchiveExportJobPending).
		Delete(&models.ArchiveExportJob{}).Error
}

func (s *ArchiveExportOutbox) Retry(ctx context.Context, job models.ArchiveExportJob) error {
	attempts := job.Attempts + 1
	return s.db.WithContext(ctx).Model(&models.ArchiveExportJob{}).
		Where("application_id = ? AND revision = ? AND state = ?", job.ApplicationID, job.Revision, models.ArchiveExportJobPending).
		Updates(map[string]interface{}{"attempts": attempts, "next_attempt_at": time.Now().Add(blankExportBackoff(attempts))}).Error
}

func (s *ArchiveExportOutbox) Suppress(ctx context.Context, id int) error {
	return s.SuppressTx(ctx, s.db, id)
}

// SuppressTx меняет revision даже у уже терминальной записи: старые ACK/Retry бессильны.
func (s *ArchiveExportOutbox) SuppressTx(ctx context.Context, tx *gorm.DB, id int) error {
	if id <= 0 {
		return nil
	}
	now := time.Now()
	return tx.WithContext(ctx).Exec(`INSERT INTO archive_export_jobs
		(application_id, revision, state, reason, queued_at, next_attempt_at, attempts)
		VALUES (?, ?, 'suppressed', '', ?, ?, 0)
		ON CONFLICT (application_id) DO UPDATE SET revision = EXCLUDED.revision,
		state = 'suppressed', reason = '', queued_at = EXCLUDED.queued_at,
		next_attempt_at = EXCLUDED.next_attempt_at, attempts = 0`, id, uuid.NewString(), now, now).Error
}

func (s *ArchiveExportOutbox) IsSuppressed(ctx context.Context, id int) (bool, error) {
	var exists bool
	err := s.db.WithContext(ctx).Raw("SELECT EXISTS (SELECT 1 FROM archive_export_jobs WHERE application_id = ? AND state = ?)", id, models.ArchiveExportJobSuppressed).Scan(&exists).Error
	return exists, err
}
