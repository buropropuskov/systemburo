package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"systemburo/internal/models"
	"systemburo/internal/upload"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UploadPhoto загружает фотографию системной таблицы.
func (s *systemTableService) UploadPhoto(ctx context.Context, tableID int, username string, file *multipart.FileHeader) (int, error) {
	// Получаем ID пользователя
	var userID int
	err := s.db.WithContext(ctx).
		Table("users").
		Select("id").
		Where("username = ?", username).
		Row().
		Scan(&userID)
	if err != nil {
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

	if file.Size > s.maxFileSize {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "File too large. Max 10MB")
	}

	// Создаём директорию
	uploadDir := filepath.Join(s.uploadDir, "system_tables")
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Failed to create upload directory")
	}

	src, err := file.Open()
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Error reading file")
	}
	defer src.Close()

	// Валидация типа файла по magic bytes
	detectedType, err := upload.ValidateFileType(src, allowedImageTypes)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Invalid file type. Allowed: JPEG, PNG, GIF, WebP")
	}
	// Перематываем файл после чтения заголовка
	if seeker, ok := src.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return 0, echo.NewHTTPError(http.StatusInternalServerError, "Failed to process file")
		}
	}

	ext := upload.MimeToExt(detectedType)
	uniqueName := fmt.Sprintf("%s_%d%s", uuid.New().String(), tableID, ext)
	savePath := filepath.Join(uploadDir, uniqueName)
	fileURL := fmt.Sprintf("/uploads/system_tables/%s", uniqueName)

	dst, err := os.Create(savePath)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Failed to write file")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Failed to write file")
	}

	// Определяем MIME
	mimeType := file.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	// Первая фотография -- главная
	var photoCount int64
	s.db.WithContext(ctx).Model(&models.SystemTablePhoto{}).
		Where("table_id = ?", tableID).
		Count(&photoCount)

	isMain := photoCount == 0
	fileSize := file.Size

	photo := models.SystemTablePhoto{
		TableID:  tableID,
		PhotoURL: fileURL,
		FileName: &file.Filename,
		FileSize: &fileSize,
		MimeType: &mimeType,
		IsMain:   isMain,
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
