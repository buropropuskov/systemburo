package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/models"
	"systemburo/internal/upload"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const draftReservationLifetime = 5 * time.Minute

type DraftQuotaLimits struct {
	MaxCount int64
	MaxBytes int64
}

type DraftQuotaUsage struct {
	SavedCount    int64
	SavedBytes    int64
	ReservedCount int64
	ReservedBytes int64
}

// ApplicationFileQuota is an unwired foundation until upload, Attach and the
// sweepers all share its owner lock. One instance represents one backend worker.
type ApplicationFileQuota struct {
	db            *gorm.DB
	root          string
	limits        DraftQuotaLimits
	now           func() time.Time
	workerID      string
	live          sync.Map
	recoverMu     sync.Mutex
	recoverCursor string
}

func NewApplicationFileQuota(db *gorm.DB, uploadRoot string, limits DraftQuotaLimits, now func() time.Time) (*ApplicationFileQuota, error) {
	if db == nil || uploadRoot == "" || limits.MaxCount <= 0 || limits.MaxBytes <= 0 {
		return nil, fmt.Errorf("invalid application file quota configuration")
	}
	if now == nil {
		now = time.Now
	}
	return &ApplicationFileQuota{db: db, root: uploadRoot, limits: limits, now: now, workerID: uuid.NewString()}, nil
}

// WithDraftOwnerLock is also used by Attach/Delete/Sweep once quota is enabled.
// Hash collisions can only serialize unrelated owners, never mix their queries.
func WithDraftOwnerLock(tx *gorm.DB, owner int) error {
	if owner <= 0 {
		return apperr.Validation("Некорректный владелец файлов")
	}
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 2708))", "application_file_quota:"+strconv.Itoa(owner)).Error
}

func (s *ApplicationFileQuota) ownerTx(ctx context.Context, owner int, fn func(*gorm.DB) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := WithDraftOwnerLock(tx, owner); err != nil {
			return err
		}
		return fn(tx)
	})
}

func draftQuotaUsage(tx *gorm.DB, owner int) (DraftQuotaUsage, error) {
	var result DraftQuotaUsage
	if err := tx.Model(&models.ApplicationFile{}).
		Select("COUNT(*) AS saved_count, COALESCE(SUM(file_size), 0) AS saved_bytes").
		Where("uploaded_by = ? AND application_id IS NULL", owner).Scan(&result).Error; err != nil {
		return result, err
	}
	var reserved struct{ ReservedCount, ReservedBytes int64 }
	err := tx.Model(&models.ApplicationFileReservation{}).
		Select("COALESCE(SUM(reserved_count), 0) AS reserved_count, COALESCE(SUM(reserved_bytes), 0) AS reserved_bytes").
		Where("uploaded_by = ?", owner).Scan(&reserved).Error
	result.ReservedCount, result.ReservedBytes = reserved.ReservedCount, reserved.ReservedBytes
	if result.SavedCount < 0 || result.SavedBytes < 0 || result.ReservedCount < 0 || result.ReservedBytes < 0 ||
		result.ReservedCount > math.MaxInt64-result.SavedCount || result.ReservedBytes > math.MaxInt64-result.SavedBytes {
		return result, fmt.Errorf("invalid application file quota accounting")
	}
	return result, err
}

func (s *ApplicationFileQuota) Usage(ctx context.Context, owner int) (DraftQuotaUsage, error) {
	var result DraftQuotaUsage
	err := s.ownerTx(ctx, owner, func(tx *gorm.DB) error {
		var err error
		result, err = draftQuotaUsage(tx, owner)
		return err
	})
	return result, err
}

func (s *ApplicationFileQuota) fits(usage DraftQuotaUsage, count, size int64) bool {
	return count >= 0 && size >= 0 && count <= s.limits.MaxCount-usage.SavedCount-usage.ReservedCount &&
		size <= s.limits.MaxBytes-usage.SavedBytes-usage.ReservedBytes
}

func draftQuotaExceeded() error {
	return apperr.Conflict("Достигнут лимит неприкреплённых файлов. Удалите выбранные файлы или подайте подготовленную заявку.")
}

// ReserveDrafts accounts for the entire incoming batch before permanent writes.
// Its live marker is removed only after all file writers have been closed.
func (s *ApplicationFileQuota) ReserveDrafts(ctx context.Context, owner int, count, size int64) (models.ApplicationFileReservation, error) {
	var row models.ApplicationFileReservation
	if count <= 0 || size < 0 {
		return row, apperr.Validation("Некорректный размер загрузки")
	}
	now := s.now().UTC()
	row = models.ApplicationFileReservation{ID: uuid.NewString(), UploadedBy: owner,
		State: models.ApplicationFileReservationActive, ReservedCount: count, ReservedBytes: size,
		StoredNames: []string{}, WorkerID: s.workerID, CreatedAt: now, ExpiresAt: now.Add(draftReservationLifetime)}
	s.live.Store(row.ID, struct{}{})
	err := s.ownerTx(ctx, owner, func(tx *gorm.DB) error {
		usage, err := draftQuotaUsage(tx, owner)
		if err != nil {
			return err
		}
		if !s.fits(usage, count, size) {
			return draftQuotaExceeded()
		}
		return tx.Create(&row).Error
	})
	if err != nil {
		s.live.Delete(row.ID)
	}
	return row, err
}

func (s *ApplicationFileQuota) reservation(tx *gorm.DB, owner int, id string) (models.ApplicationFileReservation, error) {
	var row models.ApplicationFileReservation
	if _, err := uuid.Parse(id); err != nil {
		return row, apperr.NotFound("Загрузка не найдена")
	}
	err := tx.Where("id = ? AND uploaded_by = ?", id, owner).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = apperr.NotFound("Загрузка не найдена")
	}
	return row, err
}

func (s *ApplicationFileQuota) writable(row models.ApplicationFileReservation) error {
	if row.State != models.ApplicationFileReservationActive || row.WorkerID != s.workerID || !s.now().Before(row.ExpiresAt) {
		return apperr.Conflict("Загрузка завершена или отменена. Выберите файлы заново.")
	}
	return nil
}

// PrepareReservedFile records a server path and any growth after normalization
// before the caller opens that path. Other upload callers retain their naming.
func (s *ApplicationFileQuota) PrepareReservedFile(ctx context.Context, owner int, id string, size int64, extension string) (string, error) {
	if size <= 0 || len(extension) < 2 || len(extension) > 12 || extension[0] != '.' ||
		strings.IndexFunc(extension[1:], func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') }) >= 0 {
		return "", apperr.Validation("Некорректный файл")
	}
	var name string
	err := s.ownerTx(ctx, owner, func(tx *gorm.DB) error {
		row, err := s.reservation(tx, owner, id)
		if err != nil {
			return err
		}
		if err := s.writable(row); err != nil {
			return err
		}
		if _, live := s.live.Load(id); !live {
			return apperr.Conflict("Запись загрузки уже завершена")
		}
		if int64(len(row.StoredNames)) >= row.ReservedCount || size > math.MaxInt64-row.PreparedBytes {
			return apperr.Validation("Некорректный состав загрузки")
		}
		total := row.PreparedBytes + size
		if total > row.ReservedBytes {
			usage, err := draftQuotaUsage(tx, owner)
			if err != nil {
				return err
			}
			if !s.fits(usage, 0, total-row.ReservedBytes) {
				return draftQuotaExceeded()
			}
			row.ReservedBytes = total
		}
		name = row.ID + "-" + uuid.NewString() + extension
		row.StoredNames = append(row.StoredNames, name)
		row.PreparedBytes = total
		return tx.Save(&row).Error
	})
	return name, err
}

// FinishWriting must be called only after all writers of this operation closed.
// Expiry alone cannot establish that a writer stopped using its file descriptor.
func (s *ApplicationFileQuota) FinishWriting(id string) { s.live.Delete(id) }

// CommitDrafts swaps a live reservation for all its file rows in one transaction.
// Replays fail without creating duplicates; the upload response remains data[].
func (s *ApplicationFileQuota) CommitDrafts(ctx context.Context, owner int, id string, saved []upload.SavedFile) ([]models.ApplicationFileItem, error) {
	var items []models.ApplicationFileItem
	err := s.ownerTx(ctx, owner, func(tx *gorm.DB) error {
		row, err := s.reservation(tx, owner, id)
		if err != nil {
			return err
		}
		if err := s.writable(row); err != nil {
			return err
		}
		if _, live := s.live.Load(id); live {
			return apperr.Conflict("Запись файлов ещё не завершена")
		}
		if int64(len(saved)) != row.ReservedCount || len(saved) != len(row.StoredNames) {
			return apperr.Validation("Неполный комплект файлов")
		}
		var size int64
		rows := make([]models.ApplicationFile, 0, len(saved))
		for i, file := range saved {
			if file.StoredName != row.StoredNames[i] || file.Size <= 0 || file.Size > math.MaxInt64-size {
				return apperr.Validation("Некорректный состав загрузки")
			}
			size += file.Size
			rows = append(rows, models.ApplicationFile{UploadedBy: owner, StoredName: file.StoredName, FileName: file.FileName,
				MimeType: file.DetectedMime, FileSize: file.Size, Encrypted: file.Encrypted})
		}
		if size != row.PreparedBytes || size > row.ReservedBytes {
			return apperr.Validation("Размер загрузки изменился")
		}
		if err := tx.Create(&rows).Error; err != nil {
			return err
		}
		if err := tx.Delete(&row).Error; err != nil {
			return err
		}
		for _, file := range rows {
			items = append(items, file.Item())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (s *ApplicationFileQuota) storagePath(id, name string) (string, error) {
	if !strings.HasPrefix(name, id+"-") {
		return "", fmt.Errorf("reservation file ownership mismatch")
	}
	return upload.StoredPath(s.root, ApplicationFilesDir, name)
}
