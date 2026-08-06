package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"systemburo/internal/apperr"
	"systemburo/internal/download"
	"systemburo/internal/services"
	"systemburo/internal/upload"

	"github.com/labstack/echo/v4"
)

// ApplicationFileHandler -- файлы, приложенные к заявке (#1721).
type ApplicationFileHandler struct {
	files       services.ApplicationFileService
	apps        services.ApplicationService
	maxFileSize int64
	maxCount    int
	maxTotal    int64
	allowed     []string
}

// NewApplicationFileHandler создаёт обработчик файлов заявки.
func NewApplicationFileHandler(
	files services.ApplicationFileService,
	apps services.ApplicationService,
	maxFileSize int64, maxCount int, maxTotal int64, allowed []string,
) *ApplicationFileHandler {
	return &ApplicationFileHandler{
		files: files, apps: apps,
		maxFileSize: maxFileSize, maxCount: maxCount, maxTotal: maxTotal, allowed: allowed,
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
	ctx := c.Request().Context()

	count, total, err := h.files.DraftUsage(ctx, userID)
	if err != nil {
		return err
	}

	saved, err := upload.SaveMultipart(c, "files", upload.Options{
		Dir:          h.files.Dir(),
		URLPrefix:    "",
		MaxFileSize:  h.maxFileSize,
		AllowedTypes: h.allowed,
	})
	if err != nil {
		return err
	}

	var addedSize int64
	for _, f := range saved {
		addedSize += f.Size
	}
	// Лимиты проверяются после записи на диск: до чтения формы размер файлов
	// неизвестен, а лишнее сразу убирается вместе с отказом.
	if int(count)+len(saved) > h.maxCount {
		h.discard(saved)
		return apperr.Validation(fmt.Sprintf("К заявке можно приложить не больше %d файлов", h.maxCount))
	}
	if total+addedSize > h.maxTotal {
		h.discard(saved)
		return apperr.Validation(fmt.Sprintf("Общий размер файлов заявки не больше %d МБ", h.maxTotal/1024/1024))
	}

	items, err := h.files.SaveDrafts(ctx, userID, saved)
	if err != nil {
		return err
	}
	return RespondSuccess(c, items)
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
	return download.Serve(c, download.File{Path: path, Name: file.FileName, Mime: file.MimeType})
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
