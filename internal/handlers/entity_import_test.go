package handlers_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Разворот пакета выгрузки на стенд (server entity import). Тесты живут здесь, а не рядом
// с пакетом entityarchive, по тому же правилу, что export и verify (#706): второй DB-бинарь
// на ту же тест-БД даёт гонку миграций и чисток. Чистые проверки формата (decodeCell,
// quoteIdent, rowID и т.д.) - в internal/entityarchive/import_test.go.
//
// Фикстура - та же setupExportFixture, что у export/verify (entity_export_test.go, тот же
// пакет handlers_test): организация с заявкой и приложенным файлом плюс чужая организация
// рядом. Ключ шифрования файла ставится ею же и держится глобально до конца теста -
// достаточно для основного круга (импорт на "тот же" стенд с тем же DATA_ENCRYPTION_KEY);
// сценарий с ДРУГИМ ключом назначения здесь не разбирается отдельно.

// importFixture готовит организацию через setupExportFixture, снимает с неё пакет и
// возвращает каталог пакета вместе с самой фикстурой - переиспользуют его все тесты этого
// файла, кроме тех, что нарочно оставляют стенд грязным (конфликт id).
func importFixture(t *testing.T, db *gorm.DB, uploadDir string) (exportFixture, string) {
	t.Helper()
	f := setupExportFixture(t, db, uploadDir)
	eres, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: t.TempDir(), UploadPath: uploadDir,
			Recorder: services.NewAuditRecorder(db), Now: time.Now(),
		})
	require.NoError(t, err)
	return f, eres.Dir
}

// TestEntityImport_RoundTrip: главный круговой тест - Export организации, полная очистка
// базы (имитация чистого стенда), Import обратно. Организация, заявка и файл заявки
// обязаны вернуться теми же id и тем же содержимым, файл - читаться байт в байт, а
// последовательность и журнал - быть в порядке.
func TestEntityImport_RoundTrip(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir := importFixture(t, db, uploadDir)
	testutil.CleanDB(t, db)

	destUploadDir := t.TempDir()
	res, err := entityarchive.Import(context.Background(), db, dir, entityarchive.ImportOptions{
		UploadPath: destUploadDir,
		Recorder:   services.NewAuditRecorder(db),
		Apply:      true,
	})
	require.NoError(t, err, "problems: %v, conflicts: %v", res.Problems, res.Conflicts)
	require.True(t, res.Apply)
	require.Empty(t, res.Conflicts)
	require.Positive(t, res.Rows)

	var org models.Organization
	require.NoError(t, db.First(&org, f.org.ID).Error)
	require.Equal(t, f.org.Name, org.Name)

	var app models.Application
	require.NoError(t, db.First(&app, f.app.ID).Error)
	require.Equal(t, f.org.ID, app.OrganizationID)

	var file models.ApplicationFile
	require.NoError(t, db.First(&file, f.file.ID).Error)
	require.Equal(t, f.file.StoredName, file.StoredName)
	require.Equal(t, f.file.FileName, file.FileName)
	require.True(t, file.Encrypted, "ключ на этом стенде задан - строка обязана описывать файл закрытым")

	// Содержимое на диске - байт в байт то, что было в исходном (ключ в рамках теста не
	// менялся, поэтому расшифровывается тем же crypto.GetGlobalKey()).
	disk, err := os.Open(filepath.Join(destUploadDir, "application_files", file.StoredName))
	require.NoError(t, err)
	defer disk.Close()
	reader, err := crypto.NewStreamReader(disk, crypto.GetGlobalKey())
	require.NoError(t, err)
	content, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, f.content, content)

	// Импорт ничего не насоздавал сверх графа: организация ровно одна.
	var orgCount int64
	require.NoError(t, db.Model(&models.Organization{}).Count(&orgCount).Error)
	require.Equal(t, int64(1), orgCount)

	var audit models.AuditLog
	err = db.Where("entity_type = ? AND action = ? AND entity_id = ?",
		entityarchive.TypeOrganization, models.OrganizationActionImported, f.org.ID).First(&audit).Error
	require.NoError(t, err, "успешный импорт обязан оставить запись в audit_log")
}

func TestEntityImport_NotificationIntentRoundTrip(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	f := setupExportFixture(t, db, uploadDir)
	now := time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	intent := models.NotificationIntent{
		EventKey: "submit:archive-fixture", Kind: "application_submitted",
		ApplicationID: f.app.ID, ActorUserID: f.app.SenderUserID,
		State: "pending", OccurredAt: now, NextAttemptAt: now,
	}
	require.NoError(t, db.Create(&intent).Error)
	require.Positive(t, intent.ID)
	var foreign models.Application
	require.NoError(t, db.Where("organization_id = ?", f.otherOrg.ID).Take(&foreign).Error)
	foreignIntent := models.NotificationIntent{
		EventKey: "submit:foreign-archive-fixture", Kind: "application_submitted",
		ApplicationID: foreign.ID, ActorUserID: foreign.SenderUserID,
		State: "pending", OccurredAt: now, NextAttemptAt: now,
	}
	require.NoError(t, db.Create(&foreignIntent).Error)
	exported, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{Root: t.TempDir(), UploadPath: uploadDir, Recorder: services.NewAuditRecorder(db), Now: now})
	require.NoError(t, err)
	testutil.CleanDB(t, db)
	// Prove that the ordinary numeric archive path also repairs this sequence.
	require.NoError(t, db.Exec(`SELECT setval(pg_get_serial_sequence('notification_intents', 'id'), ?, false)`, intent.ID).Error)
	res, err := entityarchive.Import(context.Background(), db, exported.Dir, entityarchive.ImportOptions{
		UploadPath: t.TempDir(), Recorder: services.NewAuditRecorder(db), Apply: true,
	})
	require.NoError(t, err, "problems: %v, conflicts: %v", res.Problems, res.Conflicts)
	require.Empty(t, res.Conflicts)
	var restored models.NotificationIntent
	require.NoError(t, db.Where("event_key = ?", intent.EventKey).Take(&restored).Error)
	require.Equal(t, intent.ID, restored.ID)
	require.Equal(t, intent.ApplicationID, restored.ApplicationID)
	require.Equal(t, intent.ActorUserID, restored.ActorUserID)
	require.Equal(t, "pending", restored.State)
	require.True(t, now.Equal(restored.OccurredAt))
	var count int64
	require.NoError(t, db.Model(&models.NotificationIntent{}).Count(&count).Error)
	require.EqualValues(t, 1, count, "another organization's intent must not enter the package")
	next := models.NotificationIntent{
		EventKey: "submit:after-archive-fixture", Kind: intent.Kind,
		ApplicationID: restored.ApplicationID, ActorUserID: restored.ActorUserID,
		State: "pending", OccurredAt: now, NextAttemptAt: now,
	}
	require.NoError(t, db.Create(&next).Error)
	require.Greater(t, next.ID, restored.ID)
}

// TestEntityImport_FixesSequenceAfterExplicitIDs: последовательность organizations нарочно
// сдвинута так, чтобы БЕЗ починки следующая обычная вставка получила ровно тот id, который
// только что занял импорт, - и упала бы на дубле ключа. Сломать: закомментировать цикл
// fixSequence в insertPackage - тест краснеет ровно на db.Create ниже с "duplicate key
// value violates unique constraint organizations_pkey".
func TestEntityImport_FixesSequenceAfterExplicitIDs(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir := importFixture(t, db, uploadDir)
	testutil.CleanDB(t, db)

	// Имитация "молодого" стенда: следующий nextval() дал бы РОВНО f.org.ID - то же
	// значение, которое сейчас вставит импорт явным INSERT (он не трогает sequence).
	require.NoError(t, db.Exec(
		`SELECT setval(pg_get_serial_sequence('organizations', 'id'), ?, false)`, f.org.ID).Error)

	res, err := entityarchive.Import(context.Background(), db, dir, entityarchive.ImportOptions{
		UploadPath: t.TempDir(),
		Recorder:   services.NewAuditRecorder(db),
		Apply:      true,
	})
	require.NoError(t, err, "problems: %v, conflicts: %v", res.Problems, res.Conflicts)

	after := models.Organization{Name: "после импорта"}
	require.NoError(t, db.Create(&after).Error, "последовательность organizations не поправлена после импорта")
	require.Greater(t, after.ID, f.org.ID)
}

// TestEntityImport_RejectsUnverifiedPackage: битый пакет (тот же приём, что в
// entity_verify_test.go - подмена байт таблицы) не должен привести ни к одной записи в базе.
func TestEntityImport_RejectsUnverifiedPackage(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir := importFixture(t, db, uploadDir)

	target := filepath.Join(dir, "tables", "applications.jsonl")
	body, err := os.ReadFile(target)
	require.NoError(t, err)
	corrupted := bytes.Replace(body, []byte(`"organization_id"`), []byte(`"organization_ie"`), 1)
	require.NotEqual(t, body, corrupted, "подмена не нашла образец в реальном формате export")
	require.NoError(t, os.WriteFile(target, corrupted, 0o600))

	testutil.CleanDB(t, db)

	res, err := entityarchive.Import(context.Background(), db, dir, entityarchive.ImportOptions{
		UploadPath: t.TempDir(),
		Recorder:   services.NewAuditRecorder(db),
		Apply:      true,
	})
	require.Error(t, err)
	require.NotEmpty(t, res.Problems)

	var orgCount int64
	require.NoError(t, db.Model(&models.Organization{}).Where("name = ?", f.org.Name).Count(&orgCount).Error)
	require.Zero(t, orgCount, "битый пакет не должен был ничего вставить")
}

// TestEntityImport_RejectsTableOutsideGraph: манифест ссылается на РЕАЛЬНУЮ таблицу схемы
// (citizenships существует, колонка id совпадает, sha256 верный) вне графа organization -
// общий справочник, которым организация только пользуется, ей не принадлежит и в
// organizationNodes() не входит. Проверка со схемой (verifySchema) такую таблицу бы
// пропустила - белый список графа (checkTablesInGraph в Import, checkGraphMembership в
// Verify) обязан отказать сам, до вставки.
func TestEntityImport_RejectsTableOutsideGraph(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir := importFixture(t, db, uploadDir)

	body := []byte(`{"id":1}` + "\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tables", "citizenships.jsonl"), body, 0o600))
	sum := sha256.Sum256(body)
	mutateManifest(t, dir, func(m *entityarchive.Manifest) {
		m.Tables = append(m.Tables, entityarchive.TableFile{
			Table:   "citizenships",
			Rows:    1,
			Columns: []string{"id"},
			File:    "tables/citizenships.jsonl",
			Bytes:   int64(len(body)),
			SHA256:  hex.EncodeToString(sum[:]),
		})
	})

	testutil.CleanDB(t, db)
	var citBefore int64
	require.NoError(t, db.Model(&models.Citizenship{}).Count(&citBefore).Error)

	res, err := entityarchive.Import(context.Background(), db, dir, entityarchive.ImportOptions{
		UploadPath: t.TempDir(),
		Recorder:   services.NewAuditRecorder(db),
		Apply:      true,
	})
	require.Error(t, err)
	require.NotEmpty(t, res.Problems, "проверка обязана была отказать раньше вставки")
	require.Condition(t, func() bool {
		for _, p := range res.Problems {
			if strings.Contains(p, "citizenships") {
				return true
			}
		}
		return false
	}, "причина отказа не называет таблицу вне графа: %v", res.Problems)

	var orgCount int64
	require.NoError(t, db.Model(&models.Organization{}).Where("name = ?", f.org.Name).Count(&orgCount).Error)
	require.Zero(t, orgCount, "пакет с таблицей вне графа не должен был ничего вставить")

	var citAfter int64
	require.NoError(t, db.Model(&models.Citizenship{}).Count(&citAfter).Error)
	require.Equal(t, citBefore, citAfter, "чужая таблица тоже не должна была получить строку от отклонённого импорта")
}

// TestEntityImport_RejectsIDConflicts: стенд НЕ очищен - организация фикстуры всё ещё
// занимает свой id. Импорт обязан отказать ДО вставки и назвать занятую таблицу и id.
func TestEntityImport_RejectsIDConflicts(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir := importFixture(t, db, uploadDir)

	res, err := entityarchive.Import(context.Background(), db, dir, entityarchive.ImportOptions{
		UploadPath: t.TempDir(),
		Recorder:   services.NewAuditRecorder(db),
		Apply:      true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "заняты")
	require.NotEmpty(t, res.Conflicts)

	found := false
	for _, c := range res.Conflicts {
		if c.Table == "organizations" {
			found = true
			require.Contains(t, c.Examples, f.org.ID)
		}
	}
	require.True(t, found, "конфликт по organizations не обнаружен: %v", res.Conflicts)

	var orgCount int64
	require.NoError(t, db.Model(&models.Organization{}).Where("name = ?", f.org.Name).Count(&orgCount).Error)
	require.Equal(t, int64(1), orgCount, "конфликт обязан был остановить импорт ДО вставки, а не дать дубль")
}

// TestEntityImport_CleansUpFailedFileCopy: сбой ВНУТРИ копирования файла заявки (не
// "уже существует", а отказ crypto.NewStreamWriter/io.Copy на этой попытке) не должен
// оставлять огрызок на диске - иначе повторный import -apply того же пакета упирается в
// Stat-проверку на файле, который сам импорт и не дописал (до часового уборщика сирот).
// Ломаем ключ шифрования на невалидную для AES длину (3 байта - валидный []byte,
// aes.NewCipher его отвергнет) - crypto.NewStreamWriter падает СРАЗУ после os.OpenFile,
// ровно в нужном месте copyPackageFile.
func TestEntityImport_CleansUpFailedFileCopy(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	_, dir := importFixture(t, db, uploadDir)
	testutil.CleanDB(t, db)

	crypto.SetGlobalKey([]byte{1, 2, 3})
	defer crypto.SetGlobalKey(nil)

	destUploadDir := t.TempDir()
	_, err := entityarchive.Import(context.Background(), db, dir, entityarchive.ImportOptions{
		UploadPath: destUploadDir,
		Recorder:   services.NewAuditRecorder(db),
		Apply:      true,
	})
	require.Error(t, err)

	entries, statErr := os.ReadDir(filepath.Join(destUploadDir, "application_files"))
	require.NoError(t, statErr)
	require.Empty(t, entries, "недописанный файл должен был убраться сам, а не ждать часового уборщика")
}

// TestEntityImport_DryRunWritesNothing: без -apply команда только считает - ни базы, ни
// диска трогать не должна, хотя посчитанные строки в результате уже видны оператору.
func TestEntityImport_DryRunWritesNothing(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	_, dir := importFixture(t, db, uploadDir)
	testutil.CleanDB(t, db)
	destUploadDir := t.TempDir()

	res, err := entityarchive.Import(context.Background(), db, dir, entityarchive.ImportOptions{
		UploadPath: destUploadDir,
		Recorder:   services.NewAuditRecorder(db),
		Apply:      false,
	})
	require.NoError(t, err)
	require.False(t, res.Apply)
	require.Positive(t, res.Rows)

	var orgCount int64
	require.NoError(t, db.Model(&models.Organization{}).Count(&orgCount).Error)
	require.Zero(t, orgCount, "пробный прогон не должен был писать в базу")

	_, statErr := os.Stat(filepath.Join(destUploadDir, "application_files"))
	require.True(t, os.IsNotExist(statErr), "пробный прогон не должен был писать файлы на диск")
}

// TestEntityImport_RequiresRecorder: Apply без журнала аудита - явный отказ, а не тихий
// импорт без следа (успешный импорт обязан попасть в audit_log).
func TestEntityImport_RequiresRecorder(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	_, dir := importFixture(t, db, uploadDir)
	testutil.CleanDB(t, db)

	_, err := entityarchive.Import(context.Background(), db, dir, entityarchive.ImportOptions{
		UploadPath: t.TempDir(),
		Apply:      true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "аудита")

	var orgCount int64
	require.NoError(t, db.Model(&models.Organization{}).Count(&orgCount).Error)
	require.Zero(t, orgCount)
}
