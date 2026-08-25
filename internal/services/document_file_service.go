package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// allowedDocExtensions -- разрешённые расширения документов.
var allowedDocExtensions = map[string]bool{
	".doc":  true,
	".docx": true,
	".pdf":  true,
	".xlsx": true,
	".pptx": true,
}

// magic-bytes сигнатуры для семейств форматов
var (
	magicPDF   = []byte{0x25, 0x50, 0x44, 0x46} // %PDF
	magicOOXML = []byte{0x50, 0x4B, 0x03, 0x04} // PK..
	magicOLE2  = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
)

// docExtMagicFamily -- к какому семейству magic-bytes относится расширение.
type magicFamily int

const (
	familyPDF   magicFamily = iota
	familyOOXML             // docx, xlsx, pptx
	familyOLE2              // doc
)

var extFamily = map[string]magicFamily{
	".pdf":  familyPDF,
	".docx": familyOOXML,
	".xlsx": familyOOXML,
	".pptx": familyOOXML,
	".doc":  familyOLE2,
}

// DocumentFileService -- загрузка и удаление файлов документов.
type DocumentFileService interface {
	// Save читает файл из multipart.FileHeader, валидирует, сохраняет на диск.
	// Возвращает storedName, fileExt, mimeFamily, error.
	Save(ctx context.Context, file *multipart.FileHeader, maxSize int64) (storedName, fileExt string, err error)
	// Delete удаляет файл с диска; ошибку не возвращает если файл уже отсутствует.
	Delete(storedName string)
	// UploadDir возвращает директорию хранения документов.
	UploadDir() string
}

type documentFileService struct {
	uploadDir string
}

// NewDocumentFileService создаёт DocumentFileService для документов (uploads/documents).
// uploadPath -- корневая папка uploads.
func NewDocumentFileService(uploadPath string) DocumentFileService {
	return NewDocumentFileServiceIn(uploadPath, "documents")
}

// NewDocumentFileServiceIn создаёт DocumentFileService с произвольным подкаталогом
// внутри uploads (напр. "guide" для PDF разделов руководства). Валидация и magic-bytes
// те же, что у документов; ограничение по типу (только PDF и т.п.) накладывает вызывающий хендлер.
func NewDocumentFileServiceIn(uploadPath, subdir string) DocumentFileService {
	return &documentFileService{
		uploadDir: filepath.Join(uploadPath, subdir),
	}
}

func (s *documentFileService) UploadDir() string {
	return s.uploadDir
}

func (s *documentFileService) Save(_ context.Context, file *multipart.FileHeader, maxSize int64) (string, string, error) {
	// Проверяем размер
	if file.Size > maxSize {
		maxMB := maxSize / 1024 / 1024
		return "", "", echo.NewHTTPError(http.StatusBadRequest,
			fmt.Sprintf("Файл слишком большой. Максимальный размер: %d МБ", maxMB))
	}

	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedDocExtensions[ext] {
		return "", "", echo.NewHTTPError(http.StatusBadRequest,
			"Недопустимый тип файла. Разрешены: doc, docx, pdf, xlsx, pptx")
	}

	family, ok := extFamily[ext]
	if !ok {
		return "", "", echo.NewHTTPError(http.StatusBadRequest, "Недопустимый тип файла")
	}

	src, err := file.Open()
	if err != nil {
		return "", "", echo.NewHTTPError(http.StatusBadRequest, "Ошибка чтения файла")
	}
	defer src.Close()

	// Читаем первые 8 байт для проверки magic-bytes
	header := make([]byte, 8)
	n, err := io.ReadFull(src, header)
	if err != nil && err != io.ErrUnexpectedEOF {
		return "", "", echo.NewHTTPError(http.StatusBadRequest, "Файл повреждён или пуст")
	}
	header = header[:n]

	if !matchesMagic(header, family) {
		return "", "", echo.NewHTTPError(http.StatusBadRequest,
			"Содержимое файла не соответствует расширению. Возможно, файл повреждён или переименован")
	}

	// Перемотка через многочитаемый reader (header + остаток файла)
	combined := io.MultiReader(bytes.NewReader(header), src)

	if err := os.MkdirAll(s.uploadDir, 0o755); err != nil {
		return "", "", echo.NewHTTPError(http.StatusInternalServerError, "Ошибка создания директории загрузки")
	}

	storedName := uuid.New().String() + ext
	savePath := filepath.Join(s.uploadDir, storedName)

	dst, err := os.Create(savePath)
	if err != nil {
		return "", "", echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи файла")
	}
	defer dst.Close()

	if _, err := io.Copy(dst, combined); err != nil {
		_ = os.Remove(savePath)
		return "", "", echo.NewHTTPError(http.StatusInternalServerError, "Ошибка записи файла")
	}

	return storedName, ext, nil
}

func (s *documentFileService) Delete(storedName string) {
	if storedName == "" {
		return
	}
	filePath := filepath.Join(s.uploadDir, storedName)
	if _, err := os.Stat(filePath); err == nil {
		_ = os.Remove(filePath)
	}
}

// matchesMagic проверяет соответствие заголовка файла ожидаемому семейству.
func matchesMagic(header []byte, family magicFamily) bool {
	switch family {
	case familyPDF:
		return len(header) >= 4 && bytes.Equal(header[:4], magicPDF)
	case familyOOXML:
		return len(header) >= 4 && bytes.Equal(header[:4], magicOOXML)
	case familyOLE2:
		return len(header) >= 8 && bytes.Equal(header[:8], magicOLE2)
	}
	return false
}

// DetectMimeType возвращает MIME-тип по расширению (для документов).
func DetectMimeType(ext string) string {
	switch ext {
	case ".pdf":
		return "application/pdf"
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return "application/octet-stream"
	}
}
