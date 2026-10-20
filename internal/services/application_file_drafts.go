package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/upload"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// A cleanup row for an existing file is created only from its locked DB row.
const existingDraftCleanupWorker = "00000000-0000-0000-0000-000000000000"

type DraftFilesPage struct {
	Items  []models.ApplicationFileItem `json:"items"`
	Usage  DraftQuotaUsage              `json:"usage"`
	Limits DraftQuotaLimits             `json:"limits"`
}

func (s *ApplicationFileQuota) ListDrafts(ctx context.Context, owner int, page, perPage int) (DraftFilesPage, models.PaginationMeta, error) {
	result := DraftFilesPage{Items: []models.ApplicationFileItem{}, Limits: s.limits}
	meta := models.PaginationMeta{Page: page, PerPage: perPage}
	offset, err := models.CheckedOffset(page, perPage)
	if err != nil {
		return result, meta, err
	}
	err = s.ownerTx(ctx, owner, func(tx *gorm.DB) error {
		usage, err := draftQuotaUsage(tx, owner)
		if err != nil {
			return err
		}
		result.Usage = usage
		meta.Total = usage.SavedCount
		var rows []models.ApplicationFile
		if err := tx.Where("uploaded_by = ? AND application_id IS NULL", owner).Order("created_at DESC, id DESC").Offset(offset).Limit(perPage).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			result.Items = append(result.Items, row.Item())
		}
		return nil
	})
	return result, meta, err
}

func (s *applicationFileService) Quota() *ApplicationFileQuota { return s.quota }

// queueDraftDelete atomically moves accounting to cleanup before detaching its row.
// Attach uses the same owner lock and can never attach a file after this point.
func (s *applicationFileService) queueDraftDelete(ctx context.Context, owner, id int, cutoff *time.Time) error {
	var reservation models.ApplicationFileReservation
	err := s.quota.ownerTx(ctx, owner, func(tx *gorm.DB) error {
		var file models.ApplicationFile
		q := tx.Where("id = ? AND uploaded_by = ? AND application_id IS NULL", id, owner)
		if cutoff != nil {
			q = q.Where("created_at < ?", *cutoff)
		}
		if err := q.First(&file).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return apperr.NotFound("Файл не найден")
		} else if err != nil {
			return err
		}
		if file.FileSize < 0 {
			return apperr.Internal("Некорректный размер файла")
		}
		if _, err := upload.StoredPath(s.quota.root, ApplicationFilesDir, file.StoredName); err != nil {
			return err
		}
		now := s.quota.now().UTC()
		reservation = models.ApplicationFileReservation{ID: uuid.NewString(), UploadedBy: owner, State: models.ApplicationFileReservationCleanup, ReservedCount: 1, ReservedBytes: file.FileSize, PreparedBytes: file.FileSize, StoredNames: []string{file.StoredName}, WorkerID: existingDraftCleanupWorker, CreatedAt: now, ExpiresAt: now}
		if err := tx.Create(&reservation).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND uploaded_by = ? AND application_id IS NULL", id, owner).Delete(&models.ApplicationFile{}).Error
	})
	if err != nil {
		return err
	}
	if err := s.quota.AbortReservation(ctx, owner, reservation.ID); err != nil {
		// The file is no longer usable, but remains charged until cleanup succeeds.
		slog.Warn("application draft cleanup pending", "reservation_id", reservation.ID, "error", err)
	}
	return nil
}
