package services

import (
	"context"
	"log/slog"
	"net/http"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UploadPhoto загружает фотографию места разгрузки.
func (s *unloadPlaceService) UploadPhoto(ctx context.Context, placeID int, username string, photoURL, fileName, mimeType string, fileSize int64) (int, error) {
	// Проверяем существование места
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.UnloadPlace{}).Where("id = ?", placeID).Count(&count).Error; err != nil {
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if count == 0 {
		return 0, echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}

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

	// Определяем, должна ли быть фотография главной (первая = главная)
	var photoCount int64
	s.db.WithContext(ctx).
		Model(&models.UnloadPlacePhoto{}).
		Where("unload_place_id = ?", placeID).
		Count(&photoCount)

	isMain := photoCount == 0

	photo := models.UnloadPlacePhoto{
		UnloadPlaceID: placeID,
		PhotoURL:      photoURL,
		FileName:      &fileName,
		FileSize:      &fileSize,
		MimeType:      &mimeType,
		IsMain:        isMain,
		UploadedBy:    &userID,
	}

	if err := s.db.WithContext(ctx).Create(&photo).Error; err != nil {
		slog.Error("не удалось загрузить фото", "place_id", placeID, "error", err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	slog.Info("фото загружено", "id", photo.ID, "place_id", placeID)
	return photo.ID, nil
}

// DeletePhoto удаляет фотографию места разгрузки и возвращает URL удалённого файла.
func (s *unloadPlaceService) DeletePhoto(ctx context.Context, placeID, photoID int) (string, error) {
	var photo models.UnloadPlacePhoto
	if err := s.db.WithContext(ctx).
		Where("id = ? AND unload_place_id = ?", photoID, placeID).
		First(&photo).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", echo.NewHTTPError(http.StatusNotFound, "Фотография не найдена")
		}
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Error fetching photo")
	}

	photoURL := photo.PhotoURL
	wasMain := photo.IsMain

	result := s.db.WithContext(ctx).
		Where("id = ? AND unload_place_id = ?", photoID, placeID).
		Delete(&models.UnloadPlacePhoto{})
	if result.Error != nil {
		slog.Error("не удалось удалить фото", "photo_id", photoID, "place_id", placeID, "error", result.Error)
		return "", echo.NewHTTPError(http.StatusInternalServerError, "Error deleting photo")
	}
	if result.RowsAffected == 0 {
		return "", echo.NewHTTPError(http.StatusNotFound, "Фотография не найдена")
	}
	slog.Info("фото удалено", "photo_id", photoID, "place_id", placeID)

	// Если удалили главную, назначаем следующую
	if wasMain {
		var next models.UnloadPlacePhoto
		if err := s.db.WithContext(ctx).
			Where("unload_place_id = ? AND id != ?", placeID, photoID).
			Order("uploaded_at").
			First(&next).Error; err == nil {
			s.db.WithContext(ctx).
				Model(&models.UnloadPlacePhoto{}).
				Where("id = ?", next.ID).
				Update("is_main", true)
		}
	}

	return photoURL, nil
}

// SetMainPhoto устанавливает главную фотографию места разгрузки.
func (s *unloadPlaceService) SetMainPhoto(ctx context.Context, placeID, photoID int) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Сбрасываем is_main у всех фотографий этого места
		if err := tx.Model(&models.UnloadPlacePhoto{}).
			Where("unload_place_id = ?", placeID).
			Update("is_main", false).Error; err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Error resetting main photo")
		}

		// Устанавливаем новую главную
		result := tx.Model(&models.UnloadPlacePhoto{}).
			Where("id = ? AND unload_place_id = ?", photoID, placeID).
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
