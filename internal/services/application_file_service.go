package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/crypto"
	"systemburo/internal/models"
	"systemburo/internal/upload"

	"gorm.io/gorm"
)

// ApplicationFileService -- файлы, приложенные к заявке (#1721).
//
// Порядок работы задан требованием «прикреплять только при подаче»: файлы
// загружаются черновиками до создания заявки (application_id NULL), а подача
// привязывает их к заявке в своей транзакции. Проверять вместо этого статус уже
// созданной заявки нельзя - бюро открывает её в ту же секунду по SSE, и
// загрузка проигрывала бы гонку.
type ApplicationFileService interface {
	// DeleteDraft удаляет непривязанный файл пользователя вместе с файлом на диске.
	DeleteDraft(ctx context.Context, userID, fileID int) error
	// DraftUsage возвращает число и суммарный размер черновиков пользователя.
	DraftUsage(ctx context.Context, userID int) (count int64, totalSize int64, err error)
	// Attach привязывает черновики пользователя к созданной заявке. Вызывается
	// внутри транзакции подачи: файл, не найденный среди черновиков автора,
	// откатывает подачу целиком. Здесь же проверяются пределы заявки - считать их
	// при загрузке нельзя, черновики копятся от всех незавершённых подач.
	Attach(tx *gorm.DB, userID, applicationID int, fileIDs []int, maxCount int, maxTotal int64) error
	// ListByApplication возвращает файлы заявки. Доступ проверяет вызывающий.
	ListByApplication(ctx context.Context, applicationID int) ([]models.ApplicationFileItem, error)
	// Locate возвращает строку файла заявки и путь к нему на диске.
	Locate(ctx context.Context, applicationID, fileID int) (models.ApplicationFile, string, error)
	// DeleteAttached убирает приложенный к заявке файл. Право проверяет роутер
	// (page.admin): состав заявки после подачи неизменен, а удаление нужно как
	// способ вычистить приложенное вопреки подписи поля (скан паспорта). Решение
	// владельца - заявителю после отправки состав не менять.
	DeleteAttached(ctx context.Context, userID int, applicationID, fileID int) error
	// SweepOrphans убирает черновики старше olderThan вместе с файлами на диске.
	SweepOrphans(ctx context.Context, olderThan time.Duration) (int, error)
	// SweepDiskOrphans убирает файлы на диске, которым не соответствует ни одна
	// строка: удаление заявки уносит строки каскадом, а файлы остаются лежать.
	SweepDiskOrphans(ctx context.Context) (int, error)
	// DiscardStored убирает с диска файл, для которого строка так и не появилась
	// (например, отказ по лимитам заявки уже после записи).
	DiscardStored(storedName string)
	// ReadContent отдаёт расшифрованное содержимое файла. Нужен выгрузке в архив:
	// там файл закрывается заново, уже ключами получателя.
	ReadContent(ctx context.Context, fileID int) ([]byte, error)
	// Dir -- каталог хранения файлов заявок.
	Dir() string
	Quota() *ApplicationFileQuota
}

type applicationFileService struct {
	db       *gorm.DB
	dir      string
	recorder AuditRecorder
	quota    *ApplicationFileQuota
}

// ApplicationFilesDir -- подкаталог UPLOAD_PATH, где лежат приложенные к заявкам
// сканы. Вынесено в константу: то же имя нужно гейту принадлежности в раздаче
// статики, и разъехаться эти два места не должны.
const ApplicationFilesDir = "application_files"

// NewApplicationFileService создаёт сервис поверх каталога uploads/application_files.
func NewApplicationFileService(db *gorm.DB, uploadPath string, recorder AuditRecorder, limits ...DraftQuotaLimits) ApplicationFileService {
	limit := DraftQuotaLimits{MaxCount: 50, MaxBytes: 209715200}
	if len(limits) > 0 {
		limit = limits[0]
	}
	quota, err := NewApplicationFileQuota(db, uploadPath, limit, nil)
	if err != nil {
		panic("invalid application file service configuration")
	}
	return &applicationFileService{
		db:       db,
		dir:      filepath.Join(uploadPath, ApplicationFilesDir),
		recorder: recorder,
		quota:    quota,
	}
}

func (s *applicationFileService) Dir() string { return s.dir }

func (s *applicationFileService) DeleteDraft(ctx context.Context, userID, fileID int) error {
	return s.queueDraftDelete(ctx, userID, fileID, nil)
}

func (s *applicationFileService) DraftUsage(ctx context.Context, userID int) (int64, int64, error) {
	var row struct {
		Count int64
		Total int64
	}
	err := s.db.WithContext(ctx).Model(&models.ApplicationFile{}).
		Select("COUNT(*) AS count, COALESCE(SUM(file_size), 0) AS total").
		Where("uploaded_by = ? AND application_id IS NULL", userID).
		Scan(&row).Error
	if err != nil {
		return 0, 0, apperr.Internal("Не удалось посчитать загруженные файлы")
	}
	return row.Count, row.Total, nil
}

func (s *applicationFileService) Attach(tx *gorm.DB, userID, applicationID int, fileIDs []int, maxCount int, maxTotal int64) error {
	if len(fileIDs) == 0 {
		return nil
	}
	if err := WithDraftOwnerLock(tx, userID); err != nil {
		return err
	}
	if maxCount > 0 && len(fileIDs) > maxCount {
		return apperr.Validation(fmt.Sprintf("К заявке можно приложить не больше %d файлов", maxCount))
	}
	if maxTotal > 0 {
		var total int64
		if err := tx.Model(&models.ApplicationFile{}).
			Where("id IN ? AND uploaded_by = ?", fileIDs, userID).
			Select("COALESCE(SUM(file_size), 0)").Scan(&total).Error; err != nil {
			return apperr.Internal("Не удалось посчитать размер файлов")
		}
		if total > maxTotal {
			return apperr.Validation(fmt.Sprintf("Общий размер файлов заявки не больше %d МБ", maxTotal/1024/1024))
		}
	}

	res := tx.Model(&models.ApplicationFile{}).
		Where("id IN ? AND uploaded_by = ? AND application_id IS NULL", fileIDs, userID).
		Update("application_id", applicationID)
	if res.Error != nil {
		return apperr.Internal("Не удалось приложить файлы к заявке")
	}
	// Расхождение значит, что id чужой, уже привязан или убран уборщиком. Молча
	// потерять файл нельзя: заявитель считает, что документ приложен.
	if int(res.RowsAffected) != len(fileIDs) {
		return apperr.Validation("Часть файлов не найдена, загрузите их заново")
	}
	return nil
}

func (s *applicationFileService) ListByApplication(ctx context.Context, applicationID int) ([]models.ApplicationFileItem, error) {
	var rows []models.ApplicationFile
	err := s.db.WithContext(ctx).
		Where("application_id = ?", applicationID).
		Order("created_at, id").
		Find(&rows).Error
	if err != nil {
		return nil, apperr.Internal("Не удалось получить файлы заявки")
	}

	items := make([]models.ApplicationFileItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, r.Item())
	}
	return items, nil
}

func (s *applicationFileService) Locate(ctx context.Context, applicationID, fileID int) (models.ApplicationFile, string, error) {
	var file models.ApplicationFile
	err := s.db.WithContext(ctx).
		Where("id = ? AND application_id = ?", fileID, applicationID).
		First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return models.ApplicationFile{}, "", apperr.NotFound("Файл не найден")
	}
	if err != nil {
		return models.ApplicationFile{}, "", apperr.Internal("Не удалось прочитать файл")
	}
	path, err := upload.StoredPath(filepath.Dir(s.dir), ApplicationFilesDir, file.StoredName)
	if err != nil {
		return models.ApplicationFile{}, "", apperr.Internal("Некорректное имя файла")
	}
	return file, path, nil
}

func (s *applicationFileService) DeleteAttached(ctx context.Context, userID int, applicationID, fileID int) error {
	var file models.ApplicationFile
	err := s.db.WithContext(ctx).
		Where("id = ? AND application_id = ?", fileID, applicationID).
		First(&file).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound("Файл не найден")
	}
	if err != nil {
		return apperr.Internal("Не удалось прочитать файл")
	}

	if err := s.db.WithContext(ctx).Delete(&models.ApplicationFile{}, file.ID).Error; err != nil {
		return apperr.Internal("Не удалось удалить файл")
	}
	if s.recorder != nil {
		s.recorder.Log(ctx, nil, models.AuditEntityApplication, &applicationID, "file_delete", &userID,
			map[string]any{"old_value": file.FileName})
	}
	s.removeFile(file.StoredName)
	return nil
}

func (s *applicationFileService) SweepOrphans(ctx context.Context, olderThan time.Duration) (int, error) {
	cutoff := s.quota.now().Add(-olderThan)
	var rows []models.ApplicationFile
	err := s.db.WithContext(ctx).
		Where("application_id IS NULL AND created_at < ?", cutoff).
		Order("uploaded_by, id").
		Find(&rows).Error
	if err != nil {
		return 0, fmt.Errorf("select orphan application files: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	removed := 0
	for _, r := range rows {
		if err := s.queueDraftDelete(ctx, r.UploadedBy, r.ID, &cutoff); err != nil {
			var typed *apperr.Error
			if errors.As(err, &typed) && typed.Code == 404 {
				continue
			}
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// SweepDiskOrphans сверяет каталог с базой. Нужен потому, что каскад от
// applications уносит строки, но не файлы: заявку удаляют редко, а место такой
// файл занимает вечно и никакому владельцу уже не принадлежит.
func (s *applicationFileService) SweepDiskOrphans(ctx context.Context) (int, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read application files dir: %w", err)
	}

	var known []string
	if err := s.db.WithContext(ctx).Model(&models.ApplicationFile{}).
		Pluck("stored_name", &known).Error; err != nil {
		return 0, fmt.Errorf("select known application files: %w", err)
	}
	index := make(map[string]struct{}, len(known))
	for _, name := range known {
		index[name] = struct{}{}
	}
	var reservations []models.ApplicationFileReservation
	if err := s.db.WithContext(ctx).Select("stored_names").Find(&reservations).Error; err != nil {
		return 0, err
	}
	for _, row := range reservations {
		for _, name := range row.StoredNames {
			index[name] = struct{}{}
		}
	}

	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, ok := index[e.Name()]; ok {
			continue
		}
		// Свежий файл не трогаем: между записью на диск и появлением строки есть
		// зазор, и уборщик не должен унести файл прямо из-под загрузки.
		info, err := e.Info()
		if err != nil || time.Since(info.ModTime()) < time.Hour {
			continue
		}
		if err := os.Remove(filepath.Join(s.dir, e.Name())); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return removed, fmt.Errorf("remove application disk orphan: %w", err)
		}
		removed++
	}
	return removed, nil
}

func (s *applicationFileService) ReadContent(ctx context.Context, fileID int) ([]byte, error) {
	var file models.ApplicationFile
	err := s.db.WithContext(ctx).First(&file, fileID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, apperr.NotFound("Файл не найден")
	}
	if err != nil {
		return nil, apperr.Internal("Не удалось прочитать файл")
	}

	path, err := upload.StoredPath(filepath.Dir(s.dir), ApplicationFilesDir, file.StoredName)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open application file %d: %w", fileID, err)
	}
	defer f.Close()

	var key []byte
	if file.Encrypted {
		key = crypto.GetGlobalKey()
	}
	reader, err := crypto.NewStreamReader(f, key)
	if err != nil {
		return nil, fmt.Errorf("decrypt application file %d: %w", fileID, err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read application file %d: %w", fileID, err)
	}
	return data, nil
}

func (s *applicationFileService) DiscardStored(storedName string) { s.removeFile(storedName) }

// removeFile убирает файл с диска. Отсутствие файла ошибкой не считается: строку
// в базе всё равно надо снять, иначе она будет ссылаться в пустоту вечно.
func (s *applicationFileService) removeFile(storedName string) {
	if storedName == "" {
		return
	}
	path, err := upload.StoredPath(filepath.Dir(s.dir), ApplicationFilesDir, storedName)
	if err != nil {
		slog.Error("некорректное имя файла заявки", "error", err)
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Error("не удалось удалить файл заявки", "error", err, "path", path)
	}
}
