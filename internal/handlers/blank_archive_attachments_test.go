package handlers_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Файлы, приложенные к заявке, уезжают в архив вместе с её бланками: владелец
// потребовал, чтобы всё по заявке лежало в одной папке и жило её сроком. Кладутся
// они в отдельную подпапку - имена приносит заявитель, и совпадение с именем бланка
// перезаписало бы документ.
func TestArchiveApplicationFiles(t *testing.T) {
	w := setupArchiveWorld(t)
	t.Run("приложенный файл уезжает в подпапку заявки", func(t *testing.T) { archiveAttachedFileSection(t, w) })
	t.Run("удалённый из заявки файл помечается сиротой", func(t *testing.T) { archiveAttachedOrphanSection(t, w) })
}

// seedApplicationFile кладёт файл в хранилище загрузок и заводит строку в базе -
// ровно так, как это делает подача заявки.
func seedApplicationFile(t *testing.T, db *gorm.DB, uploads string, appID int, name string, content []byte) models.ApplicationFile {
	t.Helper()

	dir := filepath.Join(uploads, "application_files")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	stored := "seed-" + name
	require.NoError(t, os.WriteFile(filepath.Join(dir, stored), content, 0o600))

	row := models.ApplicationFile{
		ApplicationID: &appID,
		FileName:      name,
		StoredName:    stored,
		MimeType:      "application/pdf",
		FileSize:      int64(len(content)),
		UploadedBy:    1,
	}
	require.NoError(t, db.Create(&row).Error)
	return row
}

func archiveAttachedFileSection(t *testing.T, w archiveWorld) {
	ctx := context.Background()
	svc := w.newWorkerExport(t)
	uploads := t.TempDir()
	svc.SetApplicationFiles(services.NewApplicationFileService(w.db, uploads, services.NewAuditRecorder(w.db)))

	uaID := w.newExportType(t, "Пропуск с файлом", true, true)
	appID, _ := w.newExportApp(t, "20260801/010", uaID, "")
	file := seedApplicationFile(t, w.db, uploads, appID, "разрешение.pdf", []byte("%PDF-1.4 разрешение на работы"))

	_, err := svc.ExportApplication(ctx, appID, services.BlankExportReasonBackfill)
	require.NoError(t, err)

	// Строка реестра заведена под отрицательным идентификатором: пара с настоящими
	// вложениями не пересекается, а весь механизм архива работает для файла даром.
	var row models.BlankExport
	require.NoError(t, w.db.Where("application_id = ? AND attachment_id = ?", appID, -file.ID).First(&row).Error)
	require.Equal(t, models.BlankExportOK, row.Status)
	require.Contains(t, row.RelDir, "Приложения")
	require.Contains(t, row.FileName, "разрешение")

	// На диске лежит содержимое, которое приложил заявитель.
	onDisk, err := os.ReadFile(w.filePath(row))
	require.NoError(t, err)
	require.Contains(t, string(onDisk), "разрешение на работы")

	// Бланк вложения при этом лежит в самой папке заявки, а не в подпапке.
	var blank models.BlankExport
	require.NoError(t, w.db.Where("application_id = ? AND attachment_id > 0", appID).First(&blank).Error)
	require.NotContains(t, blank.RelDir, "Приложения")
}

func archiveAttachedOrphanSection(t *testing.T, w archiveWorld) {
	ctx := context.Background()
	svc := w.newWorkerExport(t)
	uploads := t.TempDir()
	svc.SetApplicationFiles(services.NewApplicationFileService(w.db, uploads, services.NewAuditRecorder(w.db)))

	uaID := w.newExportType(t, "Пропуск сирота", true, true)
	appID, _ := w.newExportApp(t, "20260801/011", uaID, "")
	file := seedApplicationFile(t, w.db, uploads, appID, "лишний.pdf", []byte("%PDF-1.4 лишний документ"))

	_, err := svc.ExportApplication(ctx, appID, services.BlankExportReasonBackfill)
	require.NoError(t, err)

	// Администратор убрал файл из заявки: строка реестра остаётся, но помечается -
	// файл на диске при этом не трогается, он мог уехать в корпоративную копию.
	require.NoError(t, w.db.Delete(&models.ApplicationFile{}, file.ID).Error)
	_, err = svc.ExportApplication(ctx, appID, services.BlankExportReasonBackfill)
	require.NoError(t, err)

	var row models.BlankExport
	require.NoError(t, w.db.Where("application_id = ? AND attachment_id = ?", appID, -file.ID).First(&row).Error)
	require.Equal(t, models.BlankExportOrphan, row.Status)
	require.FileExists(t, w.filePath(row))
}
