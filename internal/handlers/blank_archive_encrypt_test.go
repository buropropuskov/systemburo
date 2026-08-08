package handlers_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"

	"filippo.io/age"
	"github.com/stretchr/testify/require"
)

// Ключи архива задают не всегда сразу, и файлы, записанные до них, лежат открытыми.
// Бэкфилл такие не догоняет: он пересобирает бланк заново, а замороженному это
// запрещено. Поэтому есть проход, который закрывает уже лежащий файл как есть.
func TestArchiveEncryptExisting(t *testing.T) {
	w := setupArchiveWorld(t)
	t.Run("замороженный открытый файл закрывается", func(t *testing.T) { archiveEncryptFrozenSection(t, w) })
	t.Run("повторный прогон ничего не делает", func(t *testing.T) { archiveEncryptIdempotentSection(t, w) })
	t.Run("пробный прогон не трогает диск", func(t *testing.T) { archiveEncryptDryRunSection(t, w) })
	t.Run("без ключей проход отказывается", func(t *testing.T) { archiveEncryptNoKeysSection(t, w) })
}

// encryptWorld - сервис выгрузки с включённым шифрованием и ключ для проверки того,
// что получатель действительно прочитает результат.
type encryptWorld struct {
	svc      *services.BlankExportService
	identity *age.X25519Identity
}

func newEncryptWorld(t *testing.T, w archiveWorld) encryptWorld {
	t.Helper()
	ident, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	own, err := age.GenerateX25519Identity()
	require.NoError(t, err)

	crypto, err := services.NewArchiveCrypto(ident.Recipient().String(), own.String())
	require.NoError(t, err)

	svc := w.newWorkerExport(t)
	svc.Writer().SetCrypto(crypto)
	return encryptWorld{svc: svc, identity: ident}
}

// seedPlainArchiveFile кладёт открытый файл на диск и заводит под него строку
// реестра - ровно то состояние, в котором архив оказался до включения ключей.
func seedPlainArchiveFile(t *testing.T, w archiveWorld, appID, attID int, relDir, name, content string, frozen bool) models.BlankExport {
	t.Helper()
	dir := w.abs(relDir)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o640))

	now := time.Now()
	row := models.BlankExport{
		ApplicationID: appID, AttachmentID: attID,
		BucketDate: now, RelDir: relDir, FileName: name,
		SizeBytes: int64(len(content)), ContentHash: "hash-" + name,
		Status: models.BlankExportOK, QueueReason: services.BlankExportReasonSubmit,
		QueuedAt: now, GeneratedAt: &now,
	}
	if frozen {
		row.FrozenAt = &now
	}
	require.NoError(t, w.db.Create(&row).Error)
	return row
}

func archiveEncryptFrozenSection(t *testing.T, w archiveWorld) {
	ew := newEncryptWorld(t, w)
	uaID := w.newExportType(t, "Шифровка заморозки", true, true)
	appID, attID := w.newExportApp(t, "20260301/001", uaID, "")
	const secret = "Пхакадзе В., паспорт 4509 123456"
	row := seedPlainArchiveFile(t, w, appID, attID, "2026/03/заявка-1", "Автозаявка.xlsx", secret, true)

	res, err := ew.svc.EncryptExisting(context.Background(), false)
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.Encrypted, 1)
	require.Zero(t, res.Failed)

	// Открытого файла на диске не осталось, рядом лежит закрытый.
	require.NoFileExists(t, w.abs("2026/03/заявка-1/Автозаявка.xlsx"))
	sealed := w.abs("2026/03/заявка-1/Автозаявка.xlsx" + services.EncryptedSuffix)
	require.FileExists(t, sealed)

	onDisk, err := os.ReadFile(sealed)
	require.NoError(t, err)
	require.NotContains(t, string(onDisk), "паспорт", "содержимое обязано перестать читаться глазами")

	// Получатель архива открывает файл своим ключом и видит прежнее содержимое.
	f, err := os.Open(sealed)
	require.NoError(t, err)
	defer f.Close()
	r, err := age.Decrypt(f, ew.identity)
	require.NoError(t, err)
	plain, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, secret, string(plain))

	// Реестр знает новое имя, а описание содержимого не тронуто: документ тот же,
	// и подмена хэша увела бы ночную сверку в перезапись.
	var after models.BlankExport
	require.NoError(t, w.db.First(&after, row.ID).Error)
	require.True(t, strings.HasSuffix(after.FileName, services.EncryptedSuffix))
	require.Equal(t, row.ContentHash, after.ContentHash)
	require.Equal(t, row.SizeBytes, after.SizeBytes)
	require.NotNil(t, after.FrozenAt, "заморозка не снимается")
}

func archiveEncryptIdempotentSection(t *testing.T, w archiveWorld) {
	ew := newEncryptWorld(t, w)
	uaID := w.newExportType(t, "Шифровка повтор", true, true)
	appID, attID := w.newExportApp(t, "20260301/002", uaID, "")
	seedPlainArchiveFile(t, w, appID, attID, "2026/03/заявка-2", "Автозаявка.xlsx", "содержимое", true)

	first, err := ew.svc.EncryptExisting(context.Background(), false)
	require.NoError(t, err)
	require.GreaterOrEqual(t, first.Encrypted, 1)

	sealed := w.abs("2026/03/заявка-2/Автозаявка.xlsx" + services.EncryptedSuffix)
	before, err := os.Stat(sealed)
	require.NoError(t, err)

	second, err := ew.svc.EncryptExisting(context.Background(), false)
	require.NoError(t, err)
	require.Zero(t, second.Candidates, "закрытые файлы в выборку больше не попадают")

	after, err := os.Stat(sealed)
	require.NoError(t, err)
	require.Equal(t, before.ModTime(), after.ModTime(),
		"повторный прогон не двигает mtime: иначе синхронизация перекачает весь архив")
}

func archiveEncryptDryRunSection(t *testing.T, w archiveWorld) {
	ew := newEncryptWorld(t, w)
	uaID := w.newExportType(t, "Шифровка проба", true, true)
	appID, attID := w.newExportApp(t, "20260301/003", uaID, "")
	row := seedPlainArchiveFile(t, w, appID, attID, "2026/03/заявка-3", "Автозаявка.xlsx", "как есть", true)

	res, err := ew.svc.EncryptExisting(context.Background(), true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, res.Encrypted, 1, "пробный прогон обязан показать объём работы")

	require.FileExists(t, w.abs("2026/03/заявка-3/Автозаявка.xlsx"))
	require.NoFileExists(t, w.abs("2026/03/заявка-3/Автозаявка.xlsx"+services.EncryptedSuffix))

	var after models.BlankExport
	require.NoError(t, w.db.First(&after, row.ID).Error)
	require.Equal(t, row.FileName, after.FileName)
}

func archiveEncryptNoKeysSection(t *testing.T, w archiveWorld) {
	svc := w.newWorkerExport(t)
	_, err := svc.EncryptExisting(context.Background(), false)
	require.ErrorIs(t, err, services.ErrArchiveCryptoDisabled,
		"молчаливый успех без ключей читался бы как «архив закрыт»")
}
