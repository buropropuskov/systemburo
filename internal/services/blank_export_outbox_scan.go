package services

import (
	"context"
	"time"

	"systemburo/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const archiveRecheckCohortSQL = `SELECT id AS application_id FROM applications WHERE sending_datetime >= ?
	UNION SELECT application_id FROM blank_exports WHERE bucket_date >= ?`

func lockArchiveScan(ctx context.Context, tx *gorm.DB) (*models.ArchiveExportScanState, error) {
	state := models.ArchiveExportScanState{ID: 1}
	if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
		return nil, err
	}
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&state, 1).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

// BeginRecheck не обнуляет незавершённый проход, в том числе после рестарта.
func (s *ArchiveExportOutbox) BeginRecheck(ctx context.Context, cutoff time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := lockArchiveScan(ctx, tx)
		if err != nil {
			return err
		}
		if state.Active {
			return nil
		}
		var upper int
		if err := tx.Raw("SELECT COALESCE(MAX(application_id), 0) FROM ("+archiveRecheckCohortSQL+") AS cohort", cutoff, cutoff).Scan(&upper).Error; err != nil {
			return err
		}
		state.Cutoff, state.UpperID, state.AfterID, state.Active = cutoff, upper, 0, upper > 0
		return tx.Save(state).Error
	})
}

// AdvanceRecheck атомарно ставит ограниченную страницу и сохраняет её курсор.
func (s *ArchiveExportOutbox) AdvanceRecheck(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > archiveRecheckBatchLimit {
		limit = archiveRecheckBatchLimit
	}
	count := 0
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		state, err := lockArchiveScan(ctx, tx)
		if err != nil {
			return err
		}
		if !state.Active {
			return nil
		}
		var ids []int
		if err := tx.Raw("SELECT application_id FROM ("+archiveRecheckCohortSQL+`) AS cohort
			WHERE application_id > ? AND application_id <= ? ORDER BY application_id LIMIT ?`,
			state.Cutoff, state.Cutoff, state.AfterID, state.UpperID, limit).Scan(&ids).Error; err != nil {
			return err
		}
		if err := s.EnqueueTx(ctx, tx, ids, BlankExportReasonRecheck); err != nil {
			return err
		}
		count = len(ids)
		if count > 0 {
			state.AfterID = ids[count-1]
		}
		state.Active = count == limit && state.AfterID < state.UpperID
		return tx.Save(state).Error
	})
	if err != nil {
		return 0, err
	}
	if count > 0 {
		s.Nudge()
	}
	return count, nil
}
