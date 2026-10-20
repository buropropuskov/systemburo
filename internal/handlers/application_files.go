package handlers

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"systemburo/internal/apperr"
	"systemburo/internal/crypto"
	"systemburo/internal/download"
	"systemburo/internal/imaging"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/upload"

	"github.com/labstack/echo/v4"
)

// ApplicationFileHandler -- файлы, приложенные к заявке (#1721).
type ApplicationFileHandler struct {
	files        services.ApplicationFileService
	apps         services.ApplicationService
	maxFileSize  int64
	maxCount     int
	maxTotal     int64
	allowed      []string
	imageMaxSide int
	jpegQuality  int
}

// NewApplicationFileHandler создаёт обработчик файлов заявки.
func NewApplicationFileHandler(
	files services.ApplicationFileService,
	apps services.ApplicationService,
	maxFileSize int64, maxCount int, maxTotal int64, allowed []string,
	imageMaxSide, jpegQuality int,
) *ApplicationFileHandler {
	return &ApplicationFileHandler{
		files: files, apps: apps,
		maxFileSize: maxFileSize, maxCount: maxCount, maxTotal: maxTotal, allowed: allowed,
		imageMaxSide: imageMaxSide, jpegQuality: jpegQuality,
	}
}

// UploadDraft godoc
// @Summary      Загрузка файла к будущей заявке
// @Description  Принимает файлы поля files и держит их черновиками до подачи заявки. Полученные id передаются в подаче полем file_ids; непривязанные файлы убирает суточный уборщик.
// @Tags         applications
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        files formData file true "Файлы"
// @Success      200 {array} models.ApplicationFileItem
// @Failure      400 {object} models.HTTPError
// @Failure      401 {object} models.HTTPError
// @Router       /applications/files [post]
func (h *ApplicationFileHandler) UploadDraft(c echo.Context) error {
	userID := GetUserID(c)
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Minute)
	defer cancel()
	c.SetRequest(c.Request().WithContext(ctx))
	form, err := c.MultipartForm()
	if err != nil {
		var httpError *echo.HTTPError
		if errors.As(err, &httpError) && httpError.Code == http.StatusRequestEntityTooLarge {
			return err
		}
		return apperr.Validation("Не удалось прочитать форму загрузки")
	}
	defer form.RemoveAll()
	files := form.File["files"]
	if len(files) == 0 {
		return apperr.Validation("Файл не выбран")
	}
	quota := h.files.Quota()
	var total int64
	sizes := make([]int64, len(files))
	for i, file := range files {
		size, err := upload.MeasureFile(ctx, file, h.maxFileSize)
		if err != nil {
			return err
		}
		if size > math.MaxInt64-total {
			return apperr.Validation("Слишком большой объём загрузки")
		}
		sizes[i] = size
		total += size
	}
	reservation, err := quota.ReserveDrafts(ctx, userID, int64(len(files)), total)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		quota.FinishWriting(reservation.ID)
		if !committed {
			cleanupCtx, done := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer done()
			if err := quota.AbortReservation(cleanupCtx, userID, reservation.ID); err != nil {
				slog.Error("application upload cleanup pending", "reservation_id", reservation.ID, "error", err)
			}
		}
	}()
	opts := upload.Options{
		Dir:          h.files.Dir(),
		URLPrefix:    "",
		MaxFileSize:  h.maxFileSize,
		AllowedTypes: h.allowed,
		// Снимок с телефона ужимается и перекодируется, вместе с этим уходит
		// EXIF с координатами съёмки. Документы проходят мимо нетронутыми.
		Normalize: &imaging.Options{MaxPixels: 25_000_000, MaxSide: h.imageMaxSide, JPEGQuality: h.jpegQuality},
		// Файл заявки ложится на диск зашифрованным: номер паспорта в базе
		// защищён, и его снимок не может лежать рядом открытым.
		EncryptionKey: crypto.GetGlobalKey(),
	}
	saved := make([]upload.SavedFile, 0, len(files))
	for i, file := range files {
		prepared, err := upload.PrepareFile(ctx, file, opts, sizes[i])
		if err != nil {
			return err
		}
		name, err := quota.PrepareReservedFile(ctx, userID, reservation.ID, prepared.Metadata.Size, upload.MimeToExt(prepared.Metadata.DetectedMime, file.Filename))
		if err != nil {
			return err
		}
		written, err := upload.WritePrepared(ctx, prepared, opts, name)
		if err != nil {
			return err
		}
		saved = append(saved, written)
	}
	quota.FinishWriting(reservation.ID)
	if err := ctx.Err(); err != nil {
		return err
	}
	items, err := quota.CommitDrafts(ctx, userID, reservation.ID, saved)
	if err != nil {
		return err
	}
	committed = true
	return RespondSuccess(c, items)
}

func (h *ApplicationFileHandler) ListDrafts(c echo.Context) error {
	if _, ok := services.ImpersonatorFromContext(c.Request().Context()); ok {
		return echo.NewHTTPError(http.StatusForbidden, "Список файлов недоступен в этом режиме")
	}
	params := models.PaginationParams{}
	if err := c.Bind(&params); err != nil {
		return apperr.Validation("Некорректные параметры списка")
	}
	params.Normalize()
	page, meta, err := h.files.Quota().ListDrafts(c.Request().Context(), GetUserID(c), params.Page, params.PerPage)
	if err != nil {
		return err
	}
	return RespondPaginated(c, page, meta)
}

// DeleteDraft godoc
// @Summary      Удаление ещё не приложенного файла
// @Description  Убирает файл, загруженный до подачи заявки. Приложенный к заявке файл этим методом не удаляется.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID файла"
// @Success      200 {object} map[string]string
// @Failure      404 {object} models.HTTPError
// @Router       /applications/files/{id} [delete]
func (h *ApplicationFileHandler) DeleteDraft(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid file ID")
	}
	if err := h.files.DeleteDraft(c.Request().Context(), GetUserID(c), id); err != nil {
		return err
	}
	return RespondMessage(c, "Файл удалён")
}

// List godoc
// @Summary      Файлы заявки
// @Description  Возвращает список файлов, приложенных к заявке. Доступен тем же, кому доступна сама заявка.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id path int true "ID заявки"
// @Success      200 {array} models.ApplicationFileItem
// @Failure      403 {object} models.HTTPError
// @Router       /applications/{id}/files [get]
func (h *ApplicationFileHandler) List(c echo.Context) error {
	appID, err := h.accessibleApplicationID(c)
	if err != nil {
		return err
	}
	items, err := h.files.ListByApplication(c.Request().Context(), appID)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
}

// Download godoc
// @Summary      Скачивание файла заявки
// @Description  Отдаёт файл заявки. Доступен тем же, кому доступна сама заявка.
// @Tags         applications
// @Produce      octet-stream
// @Security     BearerAuth
// @Param        id      path int true "ID заявки"
// @Param        file_id path int true "ID файла"
// @Success      200 {file} binary
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /applications/{id}/files/{file_id} [get]
func (h *ApplicationFileHandler) Download(c echo.Context) error {
	appID, err := h.accessibleApplicationID(c)
	if err != nil {
		return err
	}
	fileID, err := strconv.Atoi(c.Param("file_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid file ID")
	}

	file, path, err := h.files.Locate(c.Request().Context(), appID, fileID)
	if err != nil {
		return err
	}
	// Content-Type берётся из базы, куда записан тип по magic bytes: заголовок
	// формы задаёт клиент, и text/html в нём сделал бы из вложения страницу.
	// Ключ передаётся только для файлов, записанных зашифрованными: до среза B
	// они писались открытыми и читаются как есть.
	var key []byte
	if file.Encrypted {
		key = crypto.GetGlobalKey()
	}
	return download.ServeEncrypted(c, download.Encrypted{
		File: download.File{Path: path, Name: file.FileName, Mime: file.MimeType},
		Key:  key,
		Size: file.FileSize,
	})
}

// DeleteAttached godoc
// @Summary      Удаление файла заявки
// @Description  Убирает приложенный к заявке файл. Доступно носителям права администрирования: состав заявки после подачи неизменен.
// @Tags         applications
// @Produce      json
// @Security     BearerAuth
// @Param        id      path int true "ID заявки"
// @Param        file_id path int true "ID файла"
// @Success      200 {object} map[string]string
// @Failure      403 {object} models.HTTPError
// @Failure      404 {object} models.HTTPError
// @Router       /applications/{id}/files/{file_id} [delete]
func (h *ApplicationFileHandler) DeleteAttached(c echo.Context) error {
	appID, err := h.accessibleApplicationID(c)
	if err != nil {
		return err
	}
	fileID, err := strconv.Atoi(c.Param("file_id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid file ID")
	}

	if err := h.files.DeleteAttached(c.Request().Context(), GetUserID(c), appID, fileID); err != nil {
		return err
	}
	return RespondMessage(c, "Файл удалён")
}

// accessibleApplicationID разбирает id заявки из пути и проверяет доступ к ней.
// Отдельного права у файлов нет намеренно: они видны ровно тем, кому видна заявка.
func (h *ApplicationFileHandler) accessibleApplicationID(c echo.Context) (int, error) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Invalid application ID")
	}
	username := c.Get("username").(string)
	if !h.apps.CanAccessApplication(c.Request().Context(), id, username, IsSuperAdmin(c)) {
		return 0, echo.NewHTTPError(http.StatusForbidden, "Access denied")
	}
	return id, nil
}

// discard убирает с диска файлы, не прошедшие лимиты заявки.
func (h *ApplicationFileHandler) discard(saved []upload.SavedFile) {
	for _, f := range saved {
		h.files.DiscardStored(f.StoredName)
	}
}
