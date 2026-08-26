package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"systemburo/internal/entityarchive"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Проверка снятого пакета (server entity verify). Тесты живут здесь, а не в
// internal/entityarchive, по тому же правилу, что и у export (#706): второй DB-бинарь
// на ту же тест-БД даёт гонку миграций и чисток. Чистые проверки формата - в
// internal/entityarchive/verify_test.go, здесь - всё, для чего нужна реальная схема базы
// (сверка колонок) или реальный пакет, снятый Export.
//
// Фикстура для мутаций - ОДИН пакет, снятый Export без шифрования: setupExportFixture и
// testExportCrypto уже определены рядом в entity_export_test.go (тот же пакет
// handlers_test), reuse before create. Открытый текст выбран нарочно - мутировать байты и
// манифест проще, чем перешифровывать конверт на каждый негативный кейс; шифрованный путь
// целиком уже покрыт TestEntityExport_PackageRoundTrip и отдельно перепроверяется здесь
// позитивным кейсом TestEntityVerify_EncryptedRoundTrip.

// buildVerifyFixture готовит организацию с заявкой и файлом (setupExportFixture) и сразу
// снимает с неё открытый пакет - возвращает каталог, готовый к мутациям, и id организации
// (нужен тестам сверки личности пакета).
func buildVerifyFixture(t *testing.T, db *gorm.DB, uploadDir string) (string, int) {
	t.Helper()
	f := setupExportFixture(t, db, uploadDir)
	res, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: t.TempDir(), UploadPath: uploadDir,
			Recorder: services.NewAuditRecorder(db), Now: time.Now(),
		})
	require.NoError(t, err)
	return res.Dir, f.org.ID
}

// mutateManifest читает manifest.json открытого пакета, применяет правку и пишет обратно.
func mutateManifest(t *testing.T, dir string, fn func(*entityarchive.Manifest)) {
	t.Helper()
	path := filepath.Join(dir, "manifest.json")
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	var m entityarchive.Manifest
	require.NoError(t, json.Unmarshal(body, &m))
	fn(&m)
	out, err := json.MarshalIndent(m, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, out, 0o600))
}

func problemsText(res entityarchive.VerifyResult) string {
	return strings.Join(res.Problems, "\n")
}

// TestEntityVerify_ValidExportedPackage: пакет, только что снятый Export, обязан пройти
// проверку без единой причины отказа - иначе verify рассинхронизирован с собственным
// форматом export.
func TestEntityVerify_ValidExportedPackage(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.True(t, res.OK, "problems: %v", res.Problems)
	require.Empty(t, res.Problems)
	require.NotEmpty(t, res.Files)
}

// TestEntityVerify_EncryptedRoundTrip: пакет под age-конвертами разворачивается той же
// проверкой, что и открытый, - расшифровку делает переданный Decryptor.
func TestEntityVerify_EncryptedRoundTrip(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	crypt := testExportCrypto(t)
	res, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: t.TempDir(), UploadPath: uploadDir, Crypto: crypt,
			Recorder: services.NewAuditRecorder(db), Now: time.Now(),
		})
	require.NoError(t, err)

	vres, err := entityarchive.Verify(context.Background(), db, res.Dir, crypt, "", 0)
	require.NoError(t, err)
	require.True(t, vres.OK, "problems: %v", vres.Problems)
}

// TestEntityVerify_EncryptedWithoutKeyFails: тот же зашифрованный пакет без ключа - явный
// отказ, а не попытка прочитать конверт как открытый JSON.
func TestEntityVerify_EncryptedWithoutKeyFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	res, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: t.TempDir(), UploadPath: uploadDir, Crypto: testExportCrypto(t),
			Recorder: services.NewAuditRecorder(db), Now: time.Now(),
		})
	require.NoError(t, err)

	vres, err := entityarchive.Verify(context.Background(), db, res.Dir, nil, "", 0)
	require.NoError(t, err)
	require.False(t, vres.OK)
}

// TestEntityVerify_CorruptedByteFails: содержимое таблицы подменено (ключ переименован,
// длина файла та же), отпечаток в манифесте остался прежним - отказ по sha256.
func TestEntityVerify_CorruptedByteFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)

	target := filepath.Join(dir, "tables", "applications.jsonl")
	body, err := os.ReadFile(target)
	require.NoError(t, err)
	corrupted := bytes.Replace(body, []byte(`"organization_id"`), []byte(`"organization_ie"`), 1)
	require.NotEqual(t, body, corrupted, "подмена не нашла образец в реальном формате export")
	require.NoError(t, os.WriteFile(target, corrupted, 0o600))

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Contains(t, problemsText(res), "отпечаток")
}

// TestEntityVerify_CorruptedAttachmentFails: вложение заявки в files/ подменено - это
// другая ветка кода (verifyDataFile), чем таблицы в tables/, и нуждается в своей проверке.
func TestEntityVerify_CorruptedAttachmentFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)

	entries, err := os.ReadDir(filepath.Join(dir, "files"))
	require.NoError(t, err)
	require.NotEmpty(t, entries, "фикстура обязана нести файл заявки")
	target := filepath.Join(dir, "files", entries[0].Name())
	body, err := os.ReadFile(target)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(target, append(body, 'x'), 0o600))

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Contains(t, problemsText(res), "отпечаток")
}

// TestEntityVerify_SchemaExtraColumnWarns: в схеме базы есть колонка, которой нет в
// пакете - при развороте она получит значение по умолчанию, это предупреждение, а не отказ.
func TestEntityVerify_SchemaExtraColumnWarns(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)
	mutateManifest(t, dir, func(m *entityarchive.Manifest) {
		for i := range m.Tables {
			if m.Tables[i].Table != "organizations" {
				continue
			}
			kept := m.Tables[i].Columns[:0]
			for _, c := range m.Tables[i].Columns {
				if c != "name" {
					kept = append(kept, c)
				}
			}
			m.Tables[i].Columns = kept
		}
	})

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.True(t, res.OK, "problems: %v", res.Problems)
	require.NotEmpty(t, res.Warnings)
}

// TestEntityVerify_BothManifestsPresentFails: рядом с настоящим manifest.json появляется
// второй кандидат manifest.json.age (в атаке, которую поймало ревью, - открытая подделка с
// чужим id рядом с настоящим зашифрованным манифестом). Отказ обязан сработать по самому
// факту, что в каталоге присутствуют ОБА имени - до попытки что-либо из них прочитать или
// расшифровать: содержимое второго файла здесь намеренно не JSON и не валидный конверт,
// чтобы доказать, что причина отказа - именно двойное присутствие, а не побочный сбой при
// разборе. Раньше перебор кандидатов по порядку читал бы manifest.json и принял бы такой
// пакет как годный, а настоящий манифест ушёл бы в список предупреждений про лишний файл.
func TestEntityVerify_BothManifestsPresentFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json.age"), []byte("не важно что здесь"), 0o600))

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.False(t, res.OK, "пакет с двумя манифестами сочтён годным")
	require.Empty(t, res.Files, "с двумя манифестами дальше проверять нечего")
}

// TestEntityVerify_IdentityMatches: заданные -type/-id совпадают с манифестом - обычный
// годный пакет, сверка личности ничего не портит.
func TestEntityVerify_IdentityMatches(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, orgID := buildVerifyFixture(t, db, uploadDir)

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, entityarchive.TypeOrganization, orgID)
	require.NoError(t, err)
	require.True(t, res.OK, "problems: %v", res.Problems)
}

// TestEntityVerify_IdentityMismatchIDFails: пакет внутренне целый (все отпечатки сошлись),
// но id манифеста - не тот, который ожидал вызывающий. Ровно та проверка, которую спрашивает
// снос: "это точно пакет той сущности, которую я сношу".
func TestEntityVerify_IdentityMismatchIDFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, orgID := buildVerifyFixture(t, db, uploadDir)

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, entityarchive.TypeOrganization, orgID+999)
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Contains(t, problemsText(res), "не тот пакет")
}

// TestEntityVerify_IdentityMismatchTypeFails: тип в манифесте не совпадает с ожидаемым.
func TestEntityVerify_IdentityMismatchTypeFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, orgID := buildVerifyFixture(t, db, uploadDir)

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "company", orgID)
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Contains(t, problemsText(res), "не тот пакет")
}

// TestEntityVerify_SchemaTableMissingEntirelyFails: таблица описи не существует в текущей
// схеме базы вовсе (len(cols)==0) - ветка, отдельная от "колонки не хватает у существующей
// таблицы" (см. SchemaMissingColumnFails). Table переименован в манифесте на несуществующее
// имя, а File/SHA256/Rows/Columns оставлены как есть - файл по-прежнему читается и целостен,
// единственная причина отказа именно отсутствие таблицы в схеме.
func TestEntityVerify_SchemaTableMissingEntirelyFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)
	mutateManifest(t, dir, func(m *entityarchive.Manifest) {
		for i := range m.Tables {
			if m.Tables[i].Table == "organizations" {
				m.Tables[i].Table = "не_существующая_таблица_вовсе"
			}
		}
	})

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Contains(t, problemsText(res), "не_существующая_таблица_вовсе")
	require.Contains(t, problemsText(res), "такой таблицы нет")
}

// TestEntityVerify_DeletedFileFails: файл, обещанный описью, пропал с диска.
func TestEntityVerify_DeletedFileFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)
	require.NoError(t, os.Remove(filepath.Join(dir, "tables", "applications.jsonl")))

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.False(t, res.OK)
}

// TestEntityVerify_ForeignVersionFails: манифест собран версией формата новее той, что
// понимает текущая система.
func TestEntityVerify_ForeignVersionFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)
	mutateManifest(t, dir, func(m *entityarchive.Manifest) { m.Version = entityarchive.PackageVersion + 1 })

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Contains(t, problemsText(res), "более новой версией")
}

// TestEntityVerify_ExtraFileWarnsButOK: посторонний файл в каталоге пакета - это
// предупреждение оператору, а не причина отказа.
func TestEntityVerify_ExtraFileWarnsButOK(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tables", "note.txt"), []byte("случайный файл"), 0o600))

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.True(t, res.OK, "problems: %v", res.Problems)
	require.NotEmpty(t, res.Warnings)
}

// TestEntityVerify_SchemaMissingColumnFails: колонка в описи, которой нет в текущей схеме
// базы - разворачивать её действительно некуда.
func TestEntityVerify_SchemaMissingColumnFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)
	mutateManifest(t, dir, func(m *entityarchive.Manifest) {
		for i := range m.Tables {
			if m.Tables[i].Table == "organizations" {
				m.Tables[i].Columns = append(m.Tables[i].Columns, "не_существующая_колонка")
			}
		}
	})

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.False(t, res.OK)
	require.Contains(t, problemsText(res), "не_существующая_колонка")
}

// TestEntityVerify_ForgedEncryptedFlagFails - находка ревью среза purge (12.08): манифест
// НЕ под конвертом (обычный открытый manifest.json из buildVerifyFixture), но его ТЕЛО
// заявляет "encrypted": true. Раньше решение "зашифрован ли пакет" бралось ровно из этого
// поля - открытый пакет с подделанным флагом проходил бы гейт "пакет обязан быть
// зашифрован" (Purge), не будучи зашифрованным ни единым байтом. checkEncryptionConsistency
// обязана поймать расхождение между заявленным полем и тем, каким файлом манифест реально
// лежал на диске - раньше её не было.
func TestEntityVerify_ForgedEncryptedFlagFails(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	dir, _ := buildVerifyFixture(t, db, uploadDir)
	mutateManifest(t, dir, func(m *entityarchive.Manifest) { m.Encrypted = true })

	res, err := entityarchive.Verify(context.Background(), db, dir, nil, "", 0)
	require.NoError(t, err)
	require.False(t, res.OK, "открытый манифест с подделанным encrypted=true не должен пройти проверку")
	require.False(t, res.ManifestEncrypted, "факт остаётся честным независимо от подделанного поля тела манифеста")
	require.Contains(t, problemsText(res), "расхождение")
}

// TestEntityVerify_ManifestEncryptedReflectsRealFile: зеркало теста выше на честном
// зашифрованном пакете - ManifestEncrypted обязан быть true, когда манифест РЕАЛЬНО лежит
// конвертом, а не только когда об этом просят поверить полю Manifest.Encrypted.
func TestEntityVerify_ManifestEncryptedReflectsRealFile(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	crypt := testExportCrypto(t)
	res, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: t.TempDir(), UploadPath: uploadDir, Crypto: crypt,
			Recorder: services.NewAuditRecorder(db), Now: time.Now(),
		})
	require.NoError(t, err)

	vres, err := entityarchive.Verify(context.Background(), db, res.Dir, crypt, "", 0)
	require.NoError(t, err)
	require.True(t, vres.OK, "problems: %v", vres.Problems)
	require.True(t, vres.ManifestEncrypted)
}
