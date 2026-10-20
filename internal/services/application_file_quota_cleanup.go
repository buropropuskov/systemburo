package services

import (
	"context"
	"errors"
	"os"

	"systemburo/internal/apperr"
	"systemburo/internal/models"

	"gorm.io/gorm"
)

// AbortReservation first fences the operation in the DB, then removes its files
// outside a transaction. Failed cleanup continues to occupy the full quota.
func (s *ApplicationFileQuota) AbortReservation(ctx context.Context, owner int, id string) error {
	var row models.ApplicationFileReservation
	err := s.ownerTx(ctx, owner, func(tx *gorm.DB) error {
		var err error
		row, err = s.reservation(tx, owner, id)
		if err != nil {
			return err
		}
		if _, live := s.live.Load(row.ID); live {
			return apperr.Conflict("Запись файлов ещё не завершена")
		}
		if row.State == models.ApplicationFileReservationActive && row.WorkerID != s.workerID && s.now().Before(row.ExpiresAt) {
			return apperr.Conflict("Загрузка ещё выполняется")
		}
		return tx.Model(&row).Update("state", models.ApplicationFileReservationCleanup).Error
	})
	if err != nil {
		return err
	}
	var cleanupErr error
	for _, name := range row.StoredNames {
		file, err := s.storagePath(row.ID, name)
		if err == nil {
			err = os.Remove(file)
			if errors.Is(err, os.ErrNotExist) {
				err = nil
			}
		}
		cleanupErr = errors.Join(cleanupErr, err)
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	return s.ownerTx(ctx, owner, func(tx *gorm.DB) error {
		// No writer can add a path once cleanup_pending was persisted.
		return tx.Where("id = ? AND uploaded_by = ? AND state = ?", row.ID, owner, models.ApplicationFileReservationCleanup).
			Delete(&models.ApplicationFileReservation{}).Error
	})
}

// RecoverReservations runs only for the single backend supported by this quota.
// A rotating, bounded batch prevents an unremovable early row starving others.
// It does not run automatically until explicitly wired into the sweeper.
func (s *ApplicationFileQuota) RecoverReservations(ctx context.Context) (int, error) {
	s.recoverMu.Lock()
	defer s.recoverMu.Unlock()
	var rows []models.ApplicationFileReservation
	query := s.db.WithContext(ctx).Where("state = ? OR expires_at <= ?", models.ApplicationFileReservationCleanup, s.now().UTC())
	if s.recoverCursor != "" {
		query = query.Where("id > ?", s.recoverCursor)
	}
	if err := query.Order("id").Limit(100).Find(&rows).Error; err != nil {
		return 0, err
	}
	s.recoverCursor = ""
	if len(rows) == 100 {
		s.recoverCursor = rows[len(rows)-1].ID
	}
	var result error
	removed := 0
	for _, row := range rows {
		if _, live := s.live.Load(row.ID); live {
			continue
		}
		err := s.AbortReservation(ctx, row.UploadedBy, row.ID)
		var typed *apperr.Error
		if errors.As(err, &typed) && typed.Code == 404 {
			continue
		}
		if err != nil {
			result = errors.Join(result, err)
			continue
		}
		removed++
	}
	return removed, result
}
