package services

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UploadPhoto сохраняет метаданные фотографии системной таблицы. Запись файла
// на диск выполняет upload-конвейер на уровне хендлера (см. internal/upload).
func (s *systemTableService) UploadPhoto(ctx context.Context, tableID int, username string, photoURL, fileName, mimeType string, fileSize int64) (int, error) {
	// Получаем ID пользователя
	var userID int
	if err := s.db.WithContext(ctx).
		Table("users").
		Select("id").
		Where("username = ?", username).
		Row().
		Scan(&userID); err != nil {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "User not found")
	}

	// Проверяем существование таблицы
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.SystemTable{}).
		Where("id = ? AND is_active = ?", tableID, true).
		Count(&count).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if count == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "Системная таблица не найдена")
	}

	// Первая фотография -- главная
	var photoCount int64
	s.db.WithContext(ctx).Model(&models.SystemTablePhoto{}).
		Where("table_id = ?", tableID).
		Count(&photoCount)

	photo := models.SystemTablePhoto{
		TableID:    tableID,
		PhotoURL:   photoURL,
		FileName:   &fileName,
		FileSize:   &fileSize,
		MimeType:   &mimeType,
		IsMain:     photoCount == 0,
		UploadedBy: &userID,
	}

	if err := s.db.WithContext(ctx).Create(&photo).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}

	return photo.ID, nil
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
