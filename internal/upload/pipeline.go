package upload

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"systemburo/internal/apperr"
	"systemburo/internal/crypto"
	"systemburo/internal/imaging"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// SavedFile -- метаданные одного сохранённого на диск файла.
type SavedFile struct {
	URL        string // публичный URL, напр. /api/uploads/unload_places/<name>
	StoredName string // имя файла на диске
	FileName   string // оригинальное имя файла из формы
	Size       int64
	MimeType   string // Content-Type из формы: пришёл от клиента, доверять нельзя
	// DetectedMime -- тип, определённый по magic bytes. Именно его следует
	// сохранять и отдавать в Content-Type: заголовок формы задаёт клиент, и
	// text/html в нём превращает скачивание картинки в исполняемую страницу.
	DetectedMime string
	// Encrypted -- файл записан зашифрованным и читается только через
	// crypto.NewStreamReader.
	Encrypted bool
}

// Options -- параметры сохранения загруженных файлов.
type Options struct {
	Dir          string   // абсолютная/относительная директория записи (создаётся при отсутствии)
	URLPrefix    string   // публичный префикс URL без хвостового слэша (напр. /api/uploads/unload_places)
	MaxFileSize  int64    // макс размер одного файла; 0 -- без ограничения
	AllowedTypes []string // допустимые MIME-типы (определяются по magic bytes)
	NameSuffix   string   // опц. суффикс перед расширением (напр. ID сущности)
	// Normalize -- приведение изображений: уменьшение и перекодирование, которое
	// заодно срезает EXIF. nil оставляет файл как прислали. Документы проходят
	// мимо независимо от настройки.
	Normalize *imaging.Options
	// EncryptionKey -- ключ шифрования файла на диске. nil пишет открытым: так
	// работают фото мест разгрузки и таблиц, которые раздаются статикой и
	// расшифровать их было бы некому.
	EncryptionKey []byte
}

// SaveMultipart читает файлы из multipart-поля field, валидирует каждый
// (непустота поля, размер, MIME по magic bytes), сохраняет на диск под
// уникальным именем и возвращает метаданные. Это единая точка приёма файлов:
// пустое поле или неверное имя поля дают 400, а не «успех без файла».
func SaveMultipart(c echo.Context, field string, opts Options) ([]SavedFile, error) {
	form, err := c.MultipartForm()
	if err != nil {
		return nil, apperr.Validation("Не удалось прочитать форму загрузки")
	}

	files := form.File[field]
	if len(files) == 0 {
		return nil, apperr.Validation("Файл не выбран")
	}

	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return nil, apperr.Internal("Не удалось создать директорию загрузки")
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
		return SavedFile{}, apperr.Validation("Файл слишком большой")
	}

	src, err := fh.Open()
	if err != nil {
		return SavedFile{}, apperr.Validation("Не удалось прочитать файл")
	}
	defer src.Close()

	detected, err := ValidateFileType(src, opts.AllowedTypes)
	if err != nil {
		return SavedFile{}, apperr.Validation("Недопустимый тип файла")
	}
	if seeker, ok := src.(io.Seeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			return SavedFile{}, apperr.Internal("Не удалось обработать файл")
		}
	}

	// Содержимое и итоговый тип после нормализации. Тип может смениться: webp
	// уходит в jpeg, потому что кодера webp нет.
	var content io.Reader = src
	size := fh.Size
	// Офисные форматы неразличимы по сигнатуре (все они zip), поэтому тип
	// уточняется по имени: иначе таблица сохраняется как текстовый документ.
	stored := OfficeMimeByName(detected, fh.Filename)
	if opts.Normalize != nil && imaging.Normalizable(detected) {
		data, outMime, err := imaging.Normalize(src, detected, *opts.Normalize)
		if err != nil {
			return SavedFile{}, apperr.Validation("Не удалось обработать изображение")
		}
		content = bytes.NewReader(data)
		size = int64(len(data))
		stored = outMime
	}

	name := uuid.New().String()
	if opts.NameSuffix != "" {
		name += "_" + opts.NameSuffix
	}
	name += MimeToExt(stored, fh.Filename)

	path := filepath.Join(opts.Dir, name)
	dst, err := os.Create(path)
	if err != nil {
		return SavedFile{}, apperr.Internal("Не удалось записать файл")
	}
	defer dst.Close()

	writer, err := crypto.NewStreamWriter(dst, opts.EncryptionKey)
	if err != nil {
		os.Remove(path)
		return SavedFile{}, apperr.Internal("Не удалось записать файл")
	}
	if _, err := io.Copy(writer, content); err != nil {
		os.Remove(path)
		return SavedFile{}, apperr.Internal("Не удалось записать файл")
	}
	// Close дописывает последний чанк: без него файл не прочитается, поэтому его
	// ошибка обязана дойти до вызывающего, а не потеряться в defer.
	if err := writer.Close(); err != nil {
		os.Remove(path)
		return SavedFile{}, apperr.Internal("Не удалось записать файл")
	}

	mimeType := fh.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	return SavedFile{
		URL:          fmt.Sprintf("%s/%s", opts.URLPrefix, name),
		StoredName:   name,
		FileName:     fh.Filename,
		Size:         size,
		MimeType:     mimeType,
		DetectedMime: stored,
		Encrypted:    opts.EncryptionKey != nil,
	}, nil
}
