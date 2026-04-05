package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"systemburo/internal/services"
	"systemburo/internal/upload"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// allowedImageTypes -- допустимые MIME-типы для загрузки фотографий мест разгрузки.
var allowedImageTypes = []string{
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
}

// UnloadPlaceHandler -- HTTP-обработчики мест разгрузки.
type UnloadPlaceHandler struct {
	service     services.UnloadPlaceService
	maxFileSize int64
	uploadDir   string
}

// NewUnloadPlaceHandler создаёт новый экземпляр обработчика мест разгрузки.
func NewUnloadPlaceHandler(service services.UnloadPlaceService, maxFileSize int64, uploadDir string) *UnloadPlaceHandler {
	return &UnloadPlaceHandler{
		service:     service,
		maxFileSize: maxFileSize,
		uploadDir:   filepath.Join(uploadDir, "unload_places"),
	}
}

// GetAll возвращает все места разгрузки с деталями.
// @Summary      Получение всех мест разгрузки
// @Description  Возвращает список мест разгрузки с расписанием, фото и текущим статусом (open/closed)
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Success      200 {array} services.UnloadPlaceWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places [get]
func (h *UnloadPlaceHandler) GetAll(c echo.Context) error {
	places, err := h.service.GetAll(c.Request().Context())
	if err != nil {
		return err
	}
	return RespondSuccess(c, places)
}

// GetByID возвращает место разгрузки по ID.
// @Summary      Получение места разгрузки по ID
// @Description  Возвращает место разгрузки с расписанием, фото и текущим статусом
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {object} services.UnloadPlaceWithDetails
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id} [get]
func (h *UnloadPlaceHandler) GetByID(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	place, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return RespondSuccess(c, place)
}

// Create создаёт новое место разгрузки.
// @Summary      Создание места разгрузки
// @Description  Создаёт новое место разгрузки с указанными параметрами
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request body services.CreateUnloadPlaceRequest true "Данные места разгрузки"
// @Success      200 {object} map[string]interface{} "id и message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places [post]
func (h *UnloadPlaceHandler) Create(c echo.Context) error {
	var req services.CreateUnloadPlaceRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.Create(c.Request().Context(), req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Место разгрузки успешно создано",
	})
}

// Update обновляет место разгрузки.
// @Summary      Обновление места разгрузки
// @Description  Обновляет поля места разгрузки (только переданные)
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Param        request body services.UpdateUnloadPlaceRequest true "Обновляемые поля"
// @Success      200 {string} string "Место разгрузки успешно обновлено"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id} [put]
func (h *UnloadPlaceHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req services.UpdateUnloadPlaceRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.Update(c.Request().Context(), id, req); err != nil {
		return err
	}
	return RespondMessage(c, "Место разгрузки успешно обновлено")
}

// Delete удаляет место разгрузки.
// @Summary      Удаление места разгрузки
// @Description  Удаляет место разгрузки. Нельзя удалить если привязано к организациям или компаниям
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {string} string "Место разгрузки успешно удалено"
// @Failure      400 {object} models.HTTPError "Привязано к организациям/компаниям"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id} [delete]
func (h *UnloadPlaceHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return err
	}
	return RespondMessage(c, "Место разгрузки успешно удалено")
}

// --- Временные слоты ---

// GetTimeSlots возвращает временные слоты места разгрузки.
// @Summary      Получение временных слотов
// @Description  Возвращает все временные слоты для указанного места разгрузки
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Success      200 {array} models.UnloadPlaceTimeSlot
// @Failure      401 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/time-slots [get]
func (h *UnloadPlaceHandler) GetTimeSlots(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	slots, err := h.service.GetTimeSlots(c.Request().Context(), placeID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, slots)
}

// AddTimeSlot добавляет временной слот к месту разгрузки.
// @Summary      Добавление временного слота
// @Description  Создаёт новый временной слот для места разгрузки
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Param        request body services.CreateTimeSlotRequest true "Данные временного слота"
// @Success      200 {object} map[string]interface{} "id и message"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/time-slots [post]
func (h *UnloadPlaceHandler) AddTimeSlot(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	var req services.CreateTimeSlotRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	id, err := h.service.AddTimeSlot(c.Request().Context(), placeID, req)
	if err != nil {
		return err
	}
	return RespondSuccess(c, map[string]interface{}{
		"id":      id,
		"message": "Временной слот успешно добавлен",
	})
}

// UpdateTimeSlot обновляет временной слот.
// @Summary      Обновление временного слота
// @Description  Обновляет поля временного слота (только переданные)
// @Tags         unload-places
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        place_id path int true "ID места разгрузки"
// @Param        slot_id path int true "ID временного слота"
// @Param        request body services.UpdateTimeSlotRequest true "Обновляемые поля"
// @Success      200 {string} string "Временной слот успешно обновлен"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{place_id}/time-slots/{slot_id} [put]
func (h *UnloadPlaceHandler) UpdateTimeSlot(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("place_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid place_id")
	}
	slotID, err := strconv.Atoi(c.Param("slot_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid slot_id")
	}
	var req services.UpdateTimeSlotRequest
	if err := BindAndValidate(c, &req); err != nil {
		return err
	}
	if err := h.service.UpdateTimeSlot(c.Request().Context(), placeID, slotID, req); err != nil {
		return err
	}
	return RespondMessage(c, "Временной слот успешно обновлен")
}

// DeleteTimeSlot удаляет временной слот.
// @Summary      Удаление временного слота
// @Description  Удаляет временной слот из места разгрузки
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        place_id path int true "ID места разгрузки"
// @Param        slot_id path int true "ID временного слота"
// @Success      200 {string} string "Временной слот успешно удален"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{place_id}/time-slots/{slot_id} [delete]
func (h *UnloadPlaceHandler) DeleteTimeSlot(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("place_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid place_id")
	}
	slotID, err := strconv.Atoi(c.Param("slot_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid slot_id")
	}
	if err := h.service.DeleteTimeSlot(c.Request().Context(), placeID, slotID); err != nil {
		return err
	}
	return RespondMessage(c, "Временной слот успешно удален")
}

// --- Фотографии ---

// UploadPhoto загружает фотографию для места разгрузки.
// @Summary      Загрузка фотографии
// @Description  Загружает фотографию (multipart/form-data). Первая фотография становится главной
// @Tags         unload-places
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID места разгрузки"
// @Param        file formData file true "Файл фотографии (макс. 10MB)"
// @Success      200 {object} map[string]interface{} "message и photo_ids"
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{id}/photos [post]
func (h *UnloadPlaceHandler) UploadPhoto(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	username := c.Get("username").(string)

	// Создаём директорию для загрузок
	if err := os.MkdirAll(h.uploadDir, 0o755); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create upload directory")
	}

	form, err := c.MultipartForm()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Error reading multipart")
	}

	var insertedIDs []int

	files := form.File["file"]
	for _, fh := range files {
		if fh.Size > h.maxFileSize {
			return echo.NewHTTPError(http.StatusBadRequest, "File too large. Max 10MB")
		}

		src, err := fh.Open()
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Error reading file")
		}
		defer src.Close()

		// Валидация типа файла по magic bytes
		detectedType, err := upload.ValidateFileType(src, allowedImageTypes)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid file type. Allowed: JPEG, PNG, GIF, WebP")
		}
		// Перематываем файл после чтения заголовка
		if seeker, ok := src.(io.Seeker); ok {
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to process file")
			}
		}

		ext := upload.MimeToExt(detectedType)
		uniqueName := fmt.Sprintf("%s_%d%s", uuid.New().String(), placeID, ext)
		dstPath := filepath.Join(h.uploadDir, uniqueName)
		fileURL := fmt.Sprintf("/uploads/unload_places/%s", uniqueName)

		dst, err := os.Create(dstPath)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to write file")
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to write file")
		}

		mimeType := fh.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}

		id, err := h.service.UploadPhoto(
			c.Request().Context(), placeID, username,
			fileURL, fh.Filename, mimeType, fh.Size,
		)
		if err != nil {
			return err
		}
		insertedIDs = append(insertedIDs, id)
	}

	return RespondSuccess(c, map[string]interface{}{
		"message":   "Фотографии успешно загружены",
		"photo_ids": insertedIDs,
	})
}

// DeletePhoto удаляет фотографию места разгрузки.
// @Summary      Удаление фотографии
// @Description  Удаляет фотографию. Если удалена главная, следующая по дате загрузки становится главной
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        place_id path int true "ID места разгрузки"
// @Param        photo_id path int true "ID фотографии"
// @Success      200 {string} string "Фотография успешно удалена"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{place_id}/photos/{photo_id} [delete]
func (h *UnloadPlaceHandler) DeletePhoto(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("place_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid place_id")
	}
	photoID, err := strconv.Atoi(c.Param("photo_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid photo_id")
	}

	photoURL, err := h.service.DeletePhoto(c.Request().Context(), placeID, photoID)
	if err != nil {
		return err
	}

	// Удаляем файл с диска
	filePath := fmt.Sprintf("./%s", photoURL)
	if _, statErr := os.Stat(filePath); statErr == nil {
		_ = os.Remove(filePath)
	}

	return RespondMessage(c, "Фотография успешно удалена")
}

// SetMainPhoto устанавливает главную фотографию для места разгрузки.
// @Summary      Установка главной фотографии
// @Description  Устанавливает указанную фотографию как главную, сбрасывая флаг у остальных
// @Tags         unload-places
// @Produce      json
// @Security     BearerAuth
// @Param        place_id path int true "ID места разгрузки"
// @Param        photo_id path int true "ID фотографии"
// @Success      200 {string} string "Главная фотография успешно установлена"
// @Failure      401 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Failure      500 {object} models.HTTPError
// @Router       /unload-places/{place_id}/photos/{photo_id}/main [post]
func (h *UnloadPlaceHandler) SetMainPhoto(c echo.Context) error {
	placeID, err := strconv.Atoi(c.Param("place_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid place_id")
	}
	photoID, err := strconv.Atoi(c.Param("photo_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid photo_id")
	}
	if err := h.service.SetMainPhoto(c.Request().Context(), placeID, photoID); err != nil {
		return err
	}
	return RespondMessage(c, "Главная фотография успешно установлена")
}
