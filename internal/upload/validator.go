package upload

import (
	"fmt"
	"io"
	"net/http"
)

// ValidateFileType reads the first 512 bytes from r, detects the MIME type
// via http.DetectContentType, and checks it against allowedTypes.
// Returns the detected MIME type or an error if the type is not allowed.
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

func isOfficeType(mime string) bool {
	switch mime {
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return true
	}
	return false
}

// ValidateFileSize checks that size does not exceed maxSize.
func ValidateFileSize(size, maxSize int64) error {
	if size > maxSize {
		return fmt.Errorf("file too large: %d bytes (max %d)", size, maxSize)
	}
	return nil
}
