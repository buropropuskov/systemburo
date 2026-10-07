package services

import (
	"context"
	"errors"
	"gorm.io/gorm/clause"
	"log/slog"
	"net/http"
	"systemburo/internal/upload"

	"systemburo/internal/models"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// UploadPhoto загружает фотографию места разгрузки.
func (s *unloadPlaceService) ValidatePhotoParent(ctx context.Context, placeID int) error {
	var count int64
	if err := s.db.WithContext(ctx).Model(&models.UnloadPlace{}).Where("id = ? AND is_active = ?", placeID, true).Count(&count).Error; err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
	}
	if count == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
	}
	return nil
}

func (s *unloadPlaceService) UploadPhoto(ctx context.Context, placeID int, username string, photoURL, fileName, mimeType string, fileSize int64) (int, error) {
	ids, err := s.UploadPhotos(ctx, placeID, username, []upload.SavedFile{{URL: photoURL, FileName: fileName, DetectedMime: mimeType, Size: fileSize}})
	if err != nil {
		return 0, err
	}
	return ids[0], nil
}

// UploadPhotos commits the entire metadata batch under a parent row lock.
func (s *unloadPlaceService) UploadPhotos(ctx context.Context, placeID int, username string, files []upload.SavedFile) ([]int, error) {
	ids := make([]int, 0, len(files))
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var parent models.UnloadPlace
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND is_active = ?", placeID, true).First(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "Место разгрузки не найдено")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Database error")
		}
		var userID int
		if err := tx.Table("users").Select("id").Where("username = ?", username).Row().Scan(&userID); err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "User not found")
		}
		var count int64
		if err := tx.Model(&models.UnloadPlacePhoto{}).Where("unload_place_id = ?", placeID).Count(&count).Error; err != nil {
			return err
		}
		for i, f := range files {
			name, size, mime := f.FileName, f.Size, f.DetectedMime
			row := models.UnloadPlacePhoto{UnloadPlaceID: placeID, PhotoURL: f.URL, FileName: &name, FileSize: &size, MimeType: &mime, IsMain: count == 0 && i == 0, UploadedBy: &userID}
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
