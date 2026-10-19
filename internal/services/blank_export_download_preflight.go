package services

import (
	"io"
	"net/http"
	"os"

	"systemburo/internal/download"

	"github.com/labstack/echo/v4"
)

func archivePreparationError(err error) error {
	return echo.NewHTTPError(http.StatusServiceUnavailable, "Не удалось подготовить файлы архива").SetInternal(err)
}

// Validate every selected file before committing the ZIP response. Only one file
// is open at a time; the archive is not buffered in RAM or copied to temporary
// storage. Read failures after streaming starts retain the existing ZIP behavior.
func prepareArchiveZip(entries []download.ZipEntry) ([]download.ZipEntry, error) {
	for _, entry := range entries {
		info, err := os.Stat(entry.Path)
		if err != nil {
			return nil, archivePreparationError(err)
		}
		if !info.Mode().IsRegular() {
			return nil, archivePreparationError(os.ErrInvalid)
		}
		open := entry.Open
		if open == nil {
			path := entry.Path
			open = func() (io.ReadCloser, error) { return os.Open(path) }
		}
		reader, err := open()
		if err != nil {
			return nil, archivePreparationError(err)
		}
		if err := reader.Close(); err != nil {
			return nil, archivePreparationError(err)
		}
	}
	return entries, nil
}
