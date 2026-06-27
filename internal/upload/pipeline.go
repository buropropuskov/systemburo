package upload

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// SavedFile -- метаданные одного сохранённого на диск файла.
type SavedFile struct {
	URL      string // публичный URL, напр. /api/uploads/unload_places/<name>
	FileName string // оригинальное имя файла из формы
	Size     int64
	MimeType string
}

// Options -- параметры сохранения загруженных файлов.
type Options struct {
	Dir          string   // абсолютная/относительная директория записи (создаётся при отсутствии)
	URLPrefix    string   // публичный префикс URL без хвостового слэша (напр. /api/uploads/unload_places)
	MaxFileSize  int64    // макс размер одного файла; 0 -- без ограничения
	AllowedTypes []string // допустимые MIME-типы (определяются по magic bytes)
	NameSuffix   string   // опц. суффикс перед расширением (напр. ID сущности)
}

// SaveMultipart читает файлы из multipart-поля field, валидирует каждый
// (непустота поля, размер, MIME по magic bytes), сохраняет на диск под
// уникальным именем и возвращает метаданные. Это единая точка приёма файлов:
// пустое поле или неверное имя поля дают 400, а не «успех без файла».
func SaveMultipart(c echo.Context, field string, opts Options) ([]SavedFile, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Не удалось прочитать форму загрузки")
	}

	files := form.File[field]
	if len(files) == 0 {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "Файл не выбран")
	}

	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось создать директорию загрузки")
	}

	saved := make([]SavedFile, 0, len(files))
	for _, fh := range files {
		file, err := saveOne(fh, opts)
		if err != nil {
			return nil, err
		}
		saved = append(saved, file)
	}
	return saved, nil
}

// saveOne сохраняет один файл; вынесен отдельно, чтобы defer Close срабатывал
// на каждой итерации, а не копился до конца функции (утечка дескрипторов).
func saveOne(fh *multipart.FileHeader, opts Options) (SavedFile, error) {
	if opts.MaxFileSize > 0 && fh.Size > opts.MaxFileSize {
		return SavedFile{}, echo.NewHTTPError(http.StatusBadRequest, "Файл слишком большой")
	}

	src, err := fh.Open()
	if err != nil {
		return SavedFile{}, echo.NewHTTPError(http.StatusBadRequest, "Не удалось прочитать файл")
	}
	defer src.Close()

	detected, err := ValidateFileType(src, opts.AllowedTypes)
	if err != nil {
		return SavedFile{}, echo.NewHTTPError(http.StatusBadRequest, "Недопустимый тип файла")
	}
	if seeker, ok := src.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return SavedFile{}, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось обработать файл")
		}
	}

	name := uuid.New().String()
	if opts.NameSuffix != "" {
		name += "_" + opts.NameSuffix
	}
	name += MimeToExt(detected)

	dst, err := os.Create(filepath.Join(opts.Dir, name))
	if err != nil {
		return SavedFile{}, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось записать файл")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return SavedFile{}, echo.NewHTTPError(http.StatusInternalServerError, "Не удалось записать файл")
	}

	mimeType := fh.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return SavedFile{
		URL:      fmt.Sprintf("%s/%s", opts.URLPrefix, name),
		FileName: fh.Filename,
		Size:     fh.Size,
		MimeType: mimeType,
	}, nil
}
