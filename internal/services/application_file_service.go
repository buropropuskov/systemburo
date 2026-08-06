package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"systemburo/internal/apperr"
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
	// SaveDrafts записывает метаданные уже сохранённых на диск файлов как черновики
	// пользователя. Возвращает их представления для ответа.
	SaveDrafts(ctx context.Context, userID int, saved []upload.SavedFile) ([]models.ApplicationFileItem, error)
	// DeleteDraft удаляет непривязанный файл пользователя вместе с файлом на диске.
	DeleteDraft(ctx context.Context, userID, fileID int) error
	// DraftUsage возвращает число и суммарный размер черновиков пользователя.
	DraftUsage(ctx context.Context, userID int) (count int64, totalSize int64, err error)
	// Attach привязывает черновики пользователя к созданной заявке. Вызывается
	// внутри транзакции подачи: файл, не найденный среди черновиков автора,
	// откатывает подачу целиком.
	Attach(tx *gorm.DB, userID, applicationID int, fileIDs []int) error
	// ListByApplication возвращает файлы заявки. Доступ проверяет вызывающий.
	ListByApplication(ctx context.Context, applicationID int) ([]models.ApplicationFileItem, error)
	// Locate возвращает строку файла заявки и путь к нему на диске.
	Locate(ctx context.Context, applicationID, fileID int) (models.ApplicationFile, string, error)
	// SweepOrphans убирает черновики старше olderThan вместе с файлами на диске.
	SweepOrphans(ctx context.Context, olderThan time.Duration) (int, error)
	// DiscardStored убирает с диска файл, для которого строка так и не появилась
	// (например, отказ по лимитам заявки уже после записи).
	DiscardStored(storedName string)
	// Dir -- каталог хранения файлов заявок.
	Dir() string
}

type applicationFileService struct {
	db  *gorm.DB
	dir string
}

// NewApplicationFileService создаёт сервис поверх каталога uploads/application_files.
func NewApplicationFileService(db *gorm.DB, uploadPath string) ApplicationFileService {
	return &applicationFileService{db: db, dir: filepath.Join(uploadPath, "application_files")}
}

func (s *applicationFileService) Dir() string { return s.dir }

func (s *applicationFileService) SaveDrafts(ctx context.Context, userID int, saved []upload.SavedFile) ([]models.ApplicationFileItem, error) {
	rows := make([]models.ApplicationFile, 0, len(saved))
	for _, f := range saved {
		rows = append(rows, models.ApplicationFile{
			FileName:   f.FileName,
			StoredName: f.StoredName,
			// Тип берём определённый по magic bytes, а не Content-Type из формы:
			// его задаёт клиент, и text/html в нём превратил бы скачивание
			// картинки в исполняемую страницу.
			MimeType:   f.DetectedMime,
			FileSize:   f.Size,
			UploadedBy: userID,
			Encrypted:  f.Encrypted,
		})
	}

	if err := s.db.WithContext(ctx).Create(&rows).Error; err != nil {
		// Файлы уже на диске: без строк в базе их никто не подберёт, кроме
		// уборщика сирот, поэтому убираем сразу.
		for _, f := range saved {
			s.removeFile(f.StoredName)
		}
		return nil, apperr.Internal("Не удалось сохранить файлы")
	}

	items := make([]models.ApplicationFileItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, r.Item())
	}
	return items, nil
}

func (s *applicationFileService) DeleteDraft(ctx context.Context, userID, fileID int) error {
	var file models.ApplicationFile
	err := s.db.WithContext(ctx).
		Where("id = ? AND uploaded_by = ? AND application_id IS NULL", fileID, userID).
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
	s.removeFile(file.StoredName)
	return nil
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

func (s *applicationFileService) Attach(tx *gorm.DB, userID, applicationID int, fileIDs []int) error {
	if len(fileIDs) == 0 {
		return nil
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
	return file, filepath.Join(s.dir, file.StoredName), nil
}

func (s *applicationFileService) SweepOrphans(ctx context.Context, olderThan time.Duration) (int, error) {
	var rows []models.ApplicationFile
	err := s.db.WithContext(ctx).
		Where("application_id IS NULL AND created_at < ?", time.Now().Add(-olderThan)).
		Find(&rows).Error
	if err != nil {
		return 0, fmt.Errorf("select orphan application files: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	ids := make([]int, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	if err := s.db.WithContext(ctx).Delete(&models.ApplicationFile{}, ids).Error; err != nil {
		return 0, fmt.Errorf("delete orphan application files: %w", err)
	}
	for _, r := range rows {
		s.removeFile(r.StoredName)
	}
	return len(rows), nil
}

func (s *applicationFileService) DiscardStored(storedName string) { s.removeFile(storedName) }

// removeFile убирает файл с диска. Отсутствие файла ошибкой не считается: строку
// в базе всё равно надо снять, иначе она будет ссылаться в пустоту вечно.
func (s *applicationFileService) removeFile(storedName string) {
	if storedName == "" {
		return
	}
	path := filepath.Join(s.dir, storedName)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Error("не удалось удалить файл заявки", "error", err, "path", path)
	}
}
