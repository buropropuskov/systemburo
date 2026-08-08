package handlers_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"filippo.io/age"
	"github.com/stretchr/testify/require"
)

// Поштучное скачивание файла архива обязано отдавать читаемый документ. Выгрузка
// ZIP за период расшифровывала с самого начала, а кнопка «скачать файл» в ленте
// отдавала сырой шифротекст: администратор получал .xlsx.age, который не открывает
// ни Excel, ни просмотрщик. Расхождение видно только на площадке с включёнными
// ключами, поэтому его стережёт тест.
func TestArchiveDownloadFileDecrypts(t *testing.T) {
	ident, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	own, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	// Ключи ставятся ДО подъёма приложения: писатель архива читает их так же, как
	// в рабочем процессе, и включённым шифрование должно быть у самого сервиса, а
	// не только у тестового экземпляра рядом.
	t.Setenv("ARCHIVE_AGE_RECIPIENT", ident.Recipient().String())
	t.Setenv("ARCHIVE_AGE_IDENTITY", own.String())

	w := setupArchiveWorld(t)
	crypto, err := services.NewArchiveCrypto(ident.Recipient().String(), own.String())
	require.NoError(t, err)

	const content = "содержимое бланка со сведениями работника"
	const relDir = "2026/04/заявка-скачивание"
	name := "Автозаявка.xlsx" + services.EncryptedSuffix
	require.NoError(t, os.MkdirAll(filepath.Join(w.root, relDir), 0o750))
	sealed, err := crypto.Encrypt([]byte(content))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(w.root, relDir, name), sealed, 0o640))

	uaID := w.newExportType(t, "Скачивание закрытого", true, true)
	appID, attID := w.newExportApp(t, "20260401/001", uaID, "")
	now := time.Now()
	row := models.BlankExport{
		ApplicationID: appID, AttachmentID: attID,
		BucketDate: now, RelDir: relDir, FileName: name,
		// В реестре хранится размер ИСХОДНОГО содержимого - его и ждём в ответе.
		SizeBytes: int64(len(content)), ContentHash: "hash",
		Status: models.BlankExportOK, QueueReason: services.BlankExportReasonSubmit,
		QueuedAt: now, GeneratedAt: &now,
	}
	require.NoError(t, w.db.Create(&row).Error)

	rec := testutil.GET(t, w.e, "/file-archive/files/"+strconv.Itoa(row.ID), w.adminH)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, content, rec.Body.String(), "скачанный файл обязан быть читаемым")
	require.NotContains(t, rec.Header().Get("Content-Disposition"), services.EncryptedSuffix,
		"имя файла не должно предлагать пользователю сохранить .age")
}
