package services

import (
	"context"
	"errors"
	"gorm.io/gorm/clause"
	"net/http"
	"os"
	"path/filepath"
	"systemburo/internal/upload"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UploadPhoto сохраняет метаданные фотографии системной таблицы. Запись файла
// на диск выполняет upload-конвейер на уровне хендлера (см. internal/upload).
func (s *systemTableService) ValidatePhotoParent(ctx context.Context, tableID int) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.SystemTable{}).Where("id = ? AND is_active = ?", tableID, true).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}
	return nil
}

func (s *systemTableService) UploadPhoto(ctx context.Context, tableID int, username string, photoURL, fileName, mimeType string, fileSize int64) (int, error) {
	ids, err := s.UploadPhotos(ctx, tableID, username, []upload.SavedFile{{URL: photoURL, FileName: fileName, DetectedMime: mimeType, Size: fileSize}})
	if err != nil {
		return 0, err
	}
	return ids[0], nil
}

// UploadPhotos commits the entire metadata batch under a parent row lock.
func (s *systemTableService) UploadPhotos(ctx context.Context, tableID int, username string, files []upload.SavedFile) ([]int, error) {
	ids := make([]int, 0, len(files))
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var parent models.SystemTable
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_active = ?", tableID, true).First(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}
		var userID int
		if err := tx.Table("users").Select("id").Where("username = ?", username).Row().Scan(&userID); err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
		}
		var count int64
		if err := tx.Model(&models.SystemTablePhoto{}).Where("table_id = ?", tableID).Count(&count).Error; err != nil {
			return err
		}
		for i, f := range files {
			name, size, mime := f.FileName, f.Size, f.DetectedMime
			row := models.SystemTablePhoto{TableID: tableID, PhotoURL: f.URL, FileName: &name, FileSize: &size, MimeType: &mime, IsMain: count == 0 && i == 0, UploadedBy: &userID}
			if err := tx.Create(&row).Error; err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Database error").SetInternal(err)
			}
			ids = append(ids, row.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// DeletePhoto удаляет фотографию системной таблицы с файлом.
func (s *systemTableService) DeletePhoto(ctx context.Context, tableID, photoID int) error {
	var photo models.SystemTablePhoto
	if err := s.db.WithContext(ctx).
		Where("id = ? AND table_id = ?", photoID, tableID).
		First(&photo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "Фотография не найдена")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Error fetching photo")
	}

	// Удаляем файл
	fileName := filepath.Base(photo.PhotoURL)
	filePath := filepath.Join(s.uploadDir, "system_tables", fileName)
	if _, err := os.Stat(filePath); err == nil {
		_ = os.Remove(filePath)
	}

	// Удаляем запись
	result := s.db.WithContext(ctx).
		Where("id = ? AND table_id = ?", photoID, tableID).
		Delete(&models.SystemTablePhoto{})
	if result.Error != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Error deleting photo")
	}
	if result.RowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Фотография не найдена")
	}

	// Если удалили главную -- назначаем следующую
	if photo.IsMain {
		var next models.SystemTablePhoto
		if err := s.db.WithContext(ctx).
			Where("table_id = ? AND id != ?", tableID, photoID).
			Order("uploaded_at").
			First(&next).Error; err == nil {
			s.db.WithContext(ctx).
				Model(&models.SystemTablePhoto{}).
				Where("id = ?", next.ID).
				Update("is_main", true)
		}
	}

	return nil
}

// SetMainPhoto устанавливает главную фотографию системной таблицы.
func (s *systemTableService) SetMainPhoto(ctx context.Context, tableID, photoID int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Сбрасываем is_main для всех фото таблицы
		if err := tx.Model(&models.SystemTablePhoto{}).
			Where("table_id = ?", tableID).
			Update("is_main", false).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error resetting main photo")
		}

		// Устанавливаем новую главную
		result := tx.Model(&models.SystemTablePhoto{}).
			Where("id = ? AND table_id = ?", photoID, tableID).
			Update("is_main", true)
		if result.Error != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error setting main photo")
		}
		if result.RowsAffected == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "Фотография не найдена")
		}

		return nil
	})
}
