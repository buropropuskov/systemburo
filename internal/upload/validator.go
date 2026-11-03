package upload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"systemburo/internal/apperr"
)

// ValidateFileType reads the first 512 bytes from r, detects the MIME type
// via http.DetectContentType, and checks it against allowedTypes.
// Office results are provisional ZIP-family matches. Saving an Office document
// also requires ValidateOOXML; magic bytes do not establish its format.
func ValidateFileType(r io.Reader, allowedTypes []string) (string, error) {
	buf := make([]byte, 512)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read file header: %w", err)
	}
	detected := http.DetectContentType(buf[:n])
	for _, allowed := range allowedTypes {
		if detected == allowed {
			return detected, nil
		}
		if detected == "application/zip" && isOfficeType(allowed) {
			return allowed, nil
		}
	}
	return "", fmt.Errorf("file type %q not allowed (allowed: %v)", detected, allowedTypes)
}

// validateOfficeFile checks the package before applying the MIME allowlist.
// The first Office entry in allowedTypes must not determine the stored format.
func validateOfficeFile(ctx context.Context, src io.ReaderAt, size int64, filename string, allowedTypes []string) (string, error) {
	mime, err := ValidateOOXML(ctx, src, size, strings.ToLower(filepath.Ext(filename)))
	if err != nil {
		return "", err
	}
	for _, allowed := range allowedTypes {
		if allowed == mime {
			return mime, nil
		}
	}
	return "", apperr.Validation("Недопустимый тип файла")
}

// OfficeMimeByName уточняет тип офисного документа по имени файла. Нужен потому,
// что docx, xlsx и pptx - это zip: по сигнатуре они неразличимы, и определение
// возвращает первый допустимый офисный тип из списка. Без уточнения таблица
// попадала в базу как текстовый документ, и интерфейс красил её не тем цветом.
func OfficeMimeByName(detected, originalName string) string {
	if !isOfficeType(detected) {
		return detected
	}
	switch strings.ToLower(filepath.Ext(originalName)) {
	case ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	}
	return detected
}

func isOfficeType(mime string) bool {
	switch mime {
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return true
	}
	return false
}

// MimeToExt возвращает расширение файла по MIME-типу, определённому из magic bytes,
// и оригинальному имени.
//
// Имя нужно для форматов, неразличимых по сигнатуре: docx, xlsx и pptx - это zip,
// и по одним magic bytes таблица легла бы на диск с расширением документа. Для
// таких типов расширение берётся из имени и только из белого списка, чтобы имя из
// формы не задавало произвольное расширение файла на диске.
func MimeToExt(mime, originalName string) string {
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	}
	if isOfficeType(mime) {
		if ext := officeExtFromName(originalName); ext != "" {
			return ext
		}
		return officeDefaultExt(mime)
	}
	return ".bin"
}

// officeExtFromName возвращает расширение офисного документа из имени файла, если
// оно входит в белый список.
func officeExtFromName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".docx", ".xlsx", ".pptx":
		return ext
	}
	return ""
}

// officeDefaultExt -- расширение по MIME-типу, когда имя файла его не подсказало.
func officeDefaultExt(mime string) string {
	switch mime {
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	default:
		return ".docx"
	}
}

// ValidateFileSize checks that size does not exceed maxSize.
func ValidateFileSize(size, maxSize int64) error {
	if size > maxSize {
		return fmt.Errorf("file too large: %d bytes (max %d)", size, maxSize)
	}
	return nil
}
