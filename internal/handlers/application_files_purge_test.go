package handlers_test

// Уничтожение файлов заявки при обезличивании по сроку хранения (#2355, S4).
//
// Затирание полей в базе диска не касается, а на диске те же сведения лежат дважды:
// приложенный к заявке документ и корпоративная копия в файловом архиве - бланк со
// слепком заявка.json, где паспорт хранится ОТКРЫТЫМ текстом. Тест сначала убеждается,
// что паспорт в слепке действительно читается (ради этого срез и делался), и только
// потом проверяет, что обезличивание уносит файлы с диска.

import (
	"context"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"testing"

	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// purgeWorldFiles кладёт на диск документ, приложенный к заявке, и заводит его строку.
func purgeWorldFiles(t *testing.T, w archiveWorld, appID int) (uploadRoot, filePath string) {
	t.Helper()
	uploadRoot = t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(uploadRoot, "application_files"), 0o750))
	filePath = filepath.Join(uploadRoot, "application_files", "scan-passport.pdf")
	require.NoError(t, os.WriteFile(filePath, []byte("скан паспорта"), 0o600))

	require.NoError(t, w.db.Create(&models.ApplicationFile{
		ApplicationID: &appID, FileName: "Паспорт Хранимова.pdf",
		StoredName: "scan-passport.pdf", MimeType: "application/pdf",
		FileSize: 13, UploadedBy: w.senderID,
	}).Error)
	return uploadRoot, filePath
}

func TestAnonymizeApplication_PurgesFilesFromDisk(t *testing.T) {
	w := setupArchiveWorld(t)
	uaID := w.newExportType(t, "Пропуск срок хранения", false, true)
	appID, attID := w.newExportApp(t, "20260731/900", uaID, "")

	passport := "45 03 987654"
	require.NoError(t, w.db.Create(&models.Employee{
		AttachmentID: &attID, LastName: snapStrPtr("Хранимов"), FirstName: snapStrPtr("Олег"),
		PassportSeriesNumber: &passport,
	}).Error)

	res := w.reexport(t, appID)
	require.Equal(t, models.BlankExportOK, res.Snapshot.Status, res.Snapshot.Error)
	snapPath := w.abs(path.Join(res.RelDir, archiveSnapshotFileName))
	snapshot, err := os.ReadFile(snapPath)
	require.NoError(t, err)
	// Ради этой строки срез и делается: в базе поле зашифровано, а в слепке рядом с
	// бланком тот же паспорт лежит читаемым.
	require.Contains(t, string(snapshot), passport,
		"слепок хранит паспорт открытым текстом - обезличивание базы его не касается")

	uploadRoot, attachedPath := purgeWorldFiles(t, w, appID)
	paths := entityarchive.FilePaths{UploadPath: uploadRoot, ArchivePath: w.root}
	recorder := services.NewAuditRecorder(w.db)

	dry, err := entityarchive.AnonymizeApplication(context.Background(), w.db, recorder, paths, appID, nil, false)
	require.NoError(t, err)
	assert.Equal(t, 1, dry.Files.Attached, "приложенный документ обязан попасть в счёт")
	assert.GreaterOrEqual(t, dry.Files.Archive, 1, "слепок заявки обязан попасть в счёт")
	require.FileExists(t, snapPath, "показ без -apply диска не касается")
	require.FileExists(t, attachedPath, "показ без -apply диска не касается")

	out, err := entityarchive.AnonymizeApplication(context.Background(), w.db, recorder, paths, appID, nil, true)
	require.NoError(t, err)
	assert.Equal(t, dry.Files.Total(), out.Files.Total(), "показ обязан совпасть с делом")

	assert.NoFileExists(t, snapPath, "слепок с паспортом остался на диске")
	assert.NoFileExists(t, attachedPath, "скан паспорта остался на диске")

	var rows []models.BlankExport
	require.NoError(t, w.db.Where("application_id = ?", appID).Find(&rows).Error)
	require.NotEmpty(t, rows)
	for _, r := range rows {
		assert.Equal(t, models.BlankExportPurged, r.Status)
		assert.Empty(t, r.FileName, "реестр не должен обещать файл, которого нет")
		assert.Empty(t, r.RelDir)
	}

	var attached int64
	require.NoError(t, w.db.Model(&models.ApplicationFile{}).
		Where("application_id = ?", appID).Count(&attached).Error)
	assert.Zero(t, attached, "строка файла с именем «Паспорт Хранимова.pdf» - те же данные")
}

// Уничтоженную по сроку заявку выгрузка обязана оставить в покое: иначе ночная сверка
// или кнопка «пересоздать» вернули бы на диск бланк и слепок заявки, которую срок
// хранения велел уничтожить.
func TestAnonymizeApplication_ArchiveDoesNotResurrect(t *testing.T) {
	w := setupArchiveWorld(t)
	uaID := w.newExportType(t, "Пропуск без возврата", false, true)
	appID, attID := w.newExportApp(t, "20260731/901", uaID, "")

	passport := "45 03 111222"
	require.NoError(t, w.db.Create(&models.Employee{
		AttachmentID: &attID, LastName: snapStrPtr("Возвратов"), FirstName: snapStrPtr("Иван"),
		PassportSeriesNumber: &passport,
	}).Error)

	res := w.reexport(t, appID)
	require.Equal(t, models.BlankExportOK, res.Snapshot.Status, res.Snapshot.Error)
	snapPath := w.abs(path.Join(res.RelDir, archiveSnapshotFileName))

	paths := entityarchive.FilePaths{ArchivePath: w.root}
	_, err := entityarchive.AnonymizeApplication(context.Background(), w.db,
		services.NewAuditRecorder(w.db), paths, appID, nil, true)
	require.NoError(t, err)
	require.NoFileExists(t, snapPath)

	rec := testutil.POST(t, w.e, "/file-archive/applications/"+itoa(appID)+"/reexport", `{}`, w.adminH)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.NoFileExists(t, snapPath, "пересоздание вернуло файл уничтоженной заявки")
}
