package handlers_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

	"filippo.io/age"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Выгрузка графа организации в пакет (server entity export). Тесты живут здесь, а не
// рядом с пакетом entityarchive: база в тестах одна на пакет-бинарь, и второй бинарь,
// работающий с ней, даёт гонку чисток.

type exportFixture struct {
	org       models.Organization
	otherOrg  models.Organization
	app       models.Application
	file      models.ApplicationFile
	content   []byte
	uploadDir string
	dataKey   []byte
}

// setupExportFixture готовит организацию с заявкой и приложенным файлом, а рядом -
// чужую организацию со своей заявкой: выгрузка обязана унести только свою.
func setupExportFixture(t *testing.T, db *gorm.DB, uploadDir string) exportFixture {
	t.Helper()

	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	crypto.SetGlobalKey(key)
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })

	f := exportFixture{uploadDir: uploadDir, dataKey: key, content: []byte("%PDF-1.4\nсканированный пропуск")}

	f.org = models.Organization{Name: "Организация выгрузки"}
	require.NoError(t, db.Create(&f.org).Error)
	f.otherOrg = models.Organization{Name: "Организация по соседству"}
	require.NoError(t, db.Create(&f.otherOrg).Error)

	sender := models.User{Username: "export-sender", OrganizationID: &f.org.ID}
	require.NoError(t, db.Create(&sender).Error)
	stranger := models.User{Username: "export-stranger", OrganizationID: &f.otherOrg.ID}
	require.NoError(t, db.Create(&stranger).Error)

	f.app = models.Application{OrganizationID: f.org.ID, SenderUserID: sender.ID}
	require.NoError(t, db.Create(&f.app).Error)
	foreign := models.Application{OrganizationID: f.otherOrg.ID, SenderUserID: stranger.ID}
	require.NoError(t, db.Create(&foreign).Error)

	// Файл кладём на диск ровно так, как это делает загрузка: закрытым ключом системы.
	stored := "export-fixture.bin"
	dir := filepath.Join(uploadDir, "application_files")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	dst, err := os.Create(filepath.Join(dir, stored))
	require.NoError(t, err)
	w, err := crypto.NewStreamWriter(dst, key)
	require.NoError(t, err)
	_, err = w.Write(f.content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	require.NoError(t, dst.Close())

	f.file = models.ApplicationFile{
		ApplicationID: &f.app.ID,
		FileName:      "Паспорт Иванова.pdf",
		StoredName:    stored,
		MimeType:      "application/pdf",
		FileSize:      int64(len(f.content)),
		UploadedBy:    sender.ID,
		Encrypted:     true,
	}
	require.NoError(t, db.Create(&f.file).Error)
	return f
}

func testExportCrypto(t *testing.T) *services.ArchiveCrypto {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	crypt, err := services.NewArchiveCrypto(id.Recipient().String(), id.String())
	require.NoError(t, err)
	require.True(t, crypt.Enabled())
	return crypt
}

// readPackageFile отдаёт открытое содержимое файла пакета - то, что увидит импорт.
func readPackageFile(t *testing.T, crypt *services.ArchiveCrypto, dir, name string) []byte {
	t.Helper()
	rc, err := crypt.Open(filepath.Join(dir, filepath.FromSlash(name)))
	require.NoError(t, err, "открыть %s", name)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	return data
}

func readManifest(t *testing.T, crypt *services.ArchiveCrypto, dir string) entityarchive.Manifest {
	t.Helper()
	var m entityarchive.Manifest
	require.NoError(t, json.Unmarshal(readPackageFile(t, crypt, dir, "manifest.json.age"), &m))
	return m
}

// TestEntityExport_PackageRoundTrip: пакет собран, закрыт конвертами и разворачивается
// обратно в то же содержимое. Проверяется весь путь целиком - опись, строки таблиц,
// файл заявки - потому что на этом пакете позже держится инвариант «не сносить, пока
// копия не снята и не проверена»: неполный, но зелёный экспорт и есть худший исход.
func TestEntityExport_PackageRoundTrip(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	crypt := testExportCrypto(t)

	res, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root:       t.TempDir(),
			UploadPath: uploadDir,
			Crypto:     crypt,
			Recorder:   services.NewAuditRecorder(db),
			Now:        time.Now(),
		})
	require.NoError(t, err)

	m := readManifest(t, crypt, res.Dir)
	require.Equal(t, entityarchive.PackageVersion, m.Version)
	require.Equal(t, entityarchive.TypeOrganization, m.Type)
	require.Equal(t, f.org.ID, m.ID)
	require.True(t, m.Encrypted)
	require.Equal(t, "system_key", m.FieldEncryption, "паспортные поля уезжают закрытыми ключом установки")

	tables := map[string]entityarchive.TableFile{}
	for _, tf := range m.Tables {
		tables[tf.Table] = tf
	}
	for _, want := range []string{"organizations", "applications", "users", "application_files"} {
		require.Contains(t, tables, want, "таблица %s не попала в опись", want)
	}

	// Отпечаток описи обязан сходиться с тем, что реально лежит в пакете: иначе сверка
	// перед сносом подтверждала бы сама себя.
	apps := tables["applications"]
	body := readPackageFile(t, crypt, res.Dir, apps.File)
	sum := sha256.Sum256(body)
	require.Equal(t, apps.SHA256, hex.EncodeToString(sum[:]))
	require.Equal(t, int64(len(body)), apps.Bytes)
	require.Contains(t, apps.Columns, "organization_id")

	// Ровно одна строка - своя заявка. Чужая организация в пакет не заезжает.
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	require.Len(t, lines, 1, "в пакете %d строк заявок", len(lines))
	var row map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &row))
	require.EqualValues(t, f.app.ID, row["id"])
	require.EqualValues(t, f.org.ID, row["organization_id"])
	require.EqualValues(t, int64(1), apps.Rows)

	// Метаданные файлов и сами файлы читаются двумя выборками; они обязаны описывать
	// одно состояние. Строка в tables/application_files.jsonl без блоба в files/ (или
	// наоборот) и есть тот «зелёный, но неполный» пакет, ради которого выгрузка идёт
	// одним снимком базы.
	fileRows := tables["application_files"]
	fileLines := strings.Split(strings.TrimSpace(string(readPackageFile(t, crypt, res.Dir, fileRows.File))), "\n")
	require.Len(t, fileLines, len(m.Files), "строк в application_files.jsonl и файлов в описи разное число")

	// Файл заявки: открытое содержимое совпадает с исходным, отпечаток сходится.
	require.Len(t, m.Files, 1)
	entry := m.Files[0]
	require.Equal(t, f.file.ID, entry.RowID)
	require.Equal(t, "Паспорт Иванова.pdf", entry.OriginalName)
	require.NotContains(t, entry.File, "Паспорт", "имя файла в пакете не должно нести персональные данные")
	fileBody := readPackageFile(t, crypt, res.Dir, entry.File)
	require.Equal(t, f.content, fileBody)
	fileSum := sha256.Sum256(f.content)
	require.Equal(t, hex.EncodeToString(fileSum[:]), entry.SHA256)

	// На диске пакет закрыт: ни содержимое файла, ни строки таблиц не читаются без ключа.
	onDisk, err := os.ReadFile(filepath.Join(res.Dir, filepath.FromSlash(entry.File)))
	require.NoError(t, err)
	require.NotContains(t, string(onDisk), "сканированный пропуск")
	rawManifest, err := os.ReadFile(filepath.Join(res.Dir, "manifest.json.age"))
	require.NoError(t, err)
	require.NotContains(t, string(rawManifest), "Паспорт Иванова", "имя файла лежит под конвертом")
}

// TestEntityExport_DryRunWritesNothing: без -apply команда обязана только считать.
// Оператор решает по этим числам, и любая запись на этом шаге ломала бы договорённость.
func TestEntityExport_DryRunWritesNothing(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	root := t.TempDir()

	res, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{Root: root, UploadPath: uploadDir, Crypto: testExportCrypto(t), DryRun: true})
	require.NoError(t, err)
	require.True(t, res.DryRun)
	require.Greater(t, res.Rows, int64(0))
	require.Equal(t, 1, res.Files)
	require.EqualValues(t, len(f.content), res.FileBytes)
	require.NotEmpty(t, res.Manifest.Tables, "состав таблиц показывается и без записи")

	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Empty(t, entries, "пробный прогон создал каталог пакета")

	// Пробный прогон не пишет пакет - и не должен писать след в audit_log: запись о
	// выгрузке, которой не было, ввела бы в заблуждение так же, как и сам несуществующий
	// пакет.
	var auditCount int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", entityarchive.TypeOrganization, f.org.ID).
		Count(&auditCount).Error)
	require.Zero(t, auditCount, "пробный прогон оставил запись в audit_log")
}

// TestEntityExport_MissingDataKeyStopsExport: файл на диске закрыт ключом системы, а
// ключа нет. Тихо скопировать шифротекст значило бы отдать пакет, который не развернётся
// нигде и никогда, - и при этом выглядит целым. Проверяем и то, что от неудачной попытки
// не остаётся полупакета.
func TestEntityExport_MissingDataKeyStopsExport(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	crypto.SetGlobalKey(nil)
	root := t.TempDir()

	_, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: root, UploadPath: uploadDir, Crypto: testExportCrypto(t),
			Recorder: services.NewAuditRecorder(db),
		})
	require.Error(t, err)
	require.Contains(t, err.Error(), "DATA_ENCRYPTION_KEY")

	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Empty(t, entries, "после сбоя остался недописанный пакет")
}

// TestEntityExport_EmptyEntityRefused: организации нет - выгружать нечего, и пустой
// пакет заводить нельзя: по нему потом «сверят» снос несуществующих данных.
func TestEntityExport_EmptyEntityRefused(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	_, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, 999999,
		entityarchive.ExportOptions{Root: t.TempDir(), UploadPath: uploadDir, Recorder: services.NewAuditRecorder(db)})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "аудита", "пустая сущность обязана отказать по своей причине, а не по отсутствию Recorder")
}

// failingRecorder симулирует сбой записи журнала: Record всегда возвращает ошибку. Нужен,
// чтобы проверить обратную сторону инварианта «нет записи в журнале - нет пакета» -
// живую БД для этого ломать не надо, довольно подставного рецептора.
type failingRecorder struct{}

func (failingRecorder) Record(context.Context, *gorm.DB, string, *int, string, *int, interface{}) error {
	return errors.New("подставной сбой журнала аудита")
}

// TestEntityExport_RecordsAuditLog: реальная выгрузка обязана оставить в audit_log запись
// о снятии пакета - отдельным шагом ПОСЛЕ самого пакета, потому что снимок базы read-only
// и писать в него нельзя. Отпечаток в подробностях обязан сходиться с тем, что реально
// лежит в manifest.json: это единственный внешний якорь, которым позже можно отличить
// подменённый пакет от настоящего (опись внутри пакета сверяется сама с собой). В
// подробностях не должно быть персональных данных - только метаданные и хэш.
func TestEntityExport_RecordsAuditLog(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)

	res, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root:       t.TempDir(),
			UploadPath: uploadDir,
			Recorder:   services.NewAuditRecorder(db),
			Now:        time.Now(),
		})
	require.NoError(t, err)

	var audit models.AuditLog
	err = db.Where("entity_type = ? AND action = ? AND entity_id = ?",
		entityarchive.TypeOrganization, models.OrganizationActionExported, f.org.ID).First(&audit).Error
	require.NoError(t, err, "реальная выгрузка обязана оставить запись в audit_log")

	var details struct {
		Package        string `json:"package"`
		Rows           int64  `json:"rows"`
		Files          int    `json:"files"`
		Encrypted      bool   `json:"encrypted"`
		ManifestSHA256 string `json:"manifest_sha256"`
	}
	require.NoError(t, json.Unmarshal(audit.Details, &details))
	require.Equal(t, res.Dir, details.Package)
	require.Equal(t, res.Rows, details.Rows)
	require.Equal(t, res.Files, details.Files)
	require.False(t, details.Encrypted)
	require.NotEmpty(t, details.ManifestSHA256)

	body, err := os.ReadFile(filepath.Join(res.Dir, "manifest.json"))
	require.NoError(t, err)
	sum := sha256.Sum256(body)
	require.Equal(t, hex.EncodeToString(sum[:]), details.ManifestSHA256,
		"отпечаток в audit_log обязан сходиться с фактическим manifest.json")

	require.NotContains(t, string(audit.Details), "Паспорт Иванова",
		"имя файла - персональные данные, в audit_log им не место")
	require.NotContains(t, string(audit.Details), "сканированный пропуск")

	// Отпечаток считается с ОТКРЫТОГО тела манифеста (см. комментарий у ManifestSHA256 в
	// export.go) - путь общий что для открытого, что для зашифрованного пакета, но
	// сходится он с manifest.json.age только если считать ДО конверта, а не после. Не
	// перепроверить это отдельно значило бы поверить в общность пути на слово.
	crypt := testExportCrypto(t)
	encRes, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root:       t.TempDir(),
			UploadPath: uploadDir,
			Crypto:     crypt,
			Recorder:   services.NewAuditRecorder(db),
			Now:        time.Now(),
		})
	require.NoError(t, err)

	var encAudit models.AuditLog
	err = db.Where("entity_type = ? AND action = ? AND entity_id = ? AND details->>'package' = ?",
		entityarchive.TypeOrganization, models.OrganizationActionExported, f.org.ID, encRes.Dir).First(&encAudit).Error
	require.NoError(t, err, "зашифрованная выгрузка тоже обязана оставить запись в audit_log")

	var encDetails struct {
		Encrypted      bool   `json:"encrypted"`
		ManifestSHA256 string `json:"manifest_sha256"`
	}
	require.NoError(t, json.Unmarshal(encAudit.Details, &encDetails))
	require.True(t, encDetails.Encrypted)

	encManifestBody := readPackageFile(t, crypt, encRes.Dir, "manifest.json.age")
	encSum := sha256.Sum256(encManifestBody)
	require.Equal(t, hex.EncodeToString(encSum[:]), encDetails.ManifestSHA256,
		"на зашифрованном пакете отпечаток обязан сходиться с открытым содержимым конверта")
}

// TestEntityExport_RequiresRecorder: реальная выгрузка (DryRun=false) без рецептора -
// явный отказ до единого SELECT, а не тихая выгрузка без следа. Пробный прогон рецептора
// не требует - это отдельно проверяет TestEntityExport_DryRunWritesNothing.
//
// Сломать: убрать проверку "if !opt.DryRun && opt.Recorder == nil" в Export - тест
// краснеет, потому что выгрузка проходит и без Recorder оставляет пакет на диске.
func TestEntityExport_RequiresRecorder(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	root := t.TempDir()

	_, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{Root: root, UploadPath: uploadDir, Now: time.Now()})
	require.Error(t, err)
	require.Contains(t, err.Error(), "аудита")

	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Empty(t, entries, "отказ без Recorder не должен был создать каталог пакета")
}

// TestEntityExport_RecorderFailureRemovesPackage: запись в журнал не удалась - пакет,
// который выгрузка уже успела дописать на диск внутри снимка, обязан быть снесён, а не
// остаться там бесследной копией персональных данных организации. Это обратная сторона
// того же инварианта, что и в TestEntityExport_RequiresRecorder, но здесь Recorder есть,
// просто отказывает в момент записи.
//
// Сломать: убрать os.RemoveAll(res.Dir) в ветке ошибки recordExport в Export - тест
// краснеет на "пакет остался на диске после сбоя записи в журнал".
func TestEntityExport_RecorderFailureRemovesPackage(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	root := t.TempDir()

	_, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{Root: root, UploadPath: uploadDir, Recorder: failingRecorder{}, Now: time.Now()})
	require.Error(t, err)
	require.Contains(t, err.Error(), "журнал")

	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Empty(t, entries, "пакет остался на диске после сбоя записи в журнал")
}

// TestEntityExport_CommitFailureAfterSuccess: снимок дописал пакет ЦЕЛИКОМ (manifest.json
// включительно), а COMMIT самой read-only транзакции упал - обрыв соединения, рестарт
// базы. Найдено ревью как путь, на котором пакет оставался на диске без следа в
// audit_log: Transaction() возвращала ошибку commit'а ДО того, как Export вообще
// пыталась писать в журнал. Инвариант "нет записи в журнале - нет пакета" обязан
// держаться и здесь: запись в журнал идёт своим соединением независимо от того, жива ли
// транзакция снимка, и только если ОНА тоже не удалась, пакет сносится.
//
// Обрыв соединения смоделирован честно: через ExportOptions.AfterSnapshotSuccessForTest
// (export.go) тест обрывает РЕАЛЬНОЕ соединение именно этой транзакции - SELECT
// pg_terminate_backend(pg_backend_pid()) от себя же. Таймингом эту гонку не поймать (нет
// ни одной точки синхронизации между "снимок дописан" и вызовом Commit - см. комментарий
// у поля), поэтому вместо гонки - точка встраивания, живущая ровно один вызов Export.
func TestEntityExport_CommitFailureAfterSuccess(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	root := t.TempDir()

	_, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root:       root,
			UploadPath: uploadDir,
			Recorder:   services.NewAuditRecorder(db),
			Now:        time.Now(),
			AfterSnapshotSuccessForTest: func(tx *gorm.DB) {
				// Сам Exec может как отработать штатно (вернуть true), так и упасть с
				// ошибкой обрыва раньше времени - Postgres вправе прервать себя же в любой
				// момент между "функция выполнена" и "результат отдан". Оба исхода
				// одинаково ломают следующую команду на этом соединении (Commit), поэтому
				// результат самого Exec не проверяем.
				tx.Exec("SELECT pg_terminate_backend(pg_backend_pid())")
			},
		})
	require.Error(t, err, "commit снимка обязан был упасть - иначе крючок не сработал")
	require.Contains(t, err.Error(), "зафиксирован",
		"пакет записан и попал в audit_log несмотря на упавший commit - оператору не нужно повторять команду")

	// Пакет остался на диске: запись в журнал успела пройти СВОИМ соединением, снос не требуется.
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Len(t, entries, 1, "пакет должен был остаться на диске")

	var audit models.AuditLog
	err = db.Where("entity_type = ? AND action = ? AND entity_id = ?",
		entityarchive.TypeOrganization, models.OrganizationActionExported, f.org.ID).First(&audit).Error
	require.NoError(t, err, "запись в audit_log обязана появиться несмотря на упавший commit снимка")
}

// TestEntityExport_CommitFailureAndRecorderFailureRemovesPackage: commit снимка упал (как
// в TestEntityExport_CommitFailureAfterSuccess), И следующая за ним попытка записи в
// журнал тоже не удалась - инвариант "нет записи в журнале - нет пакета" обязан держаться
// без исключений и на пересечении обоих отказов, не только на каждом по отдельности.
func TestEntityExport_CommitFailureAndRecorderFailureRemovesPackage(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	root := t.TempDir()

	_, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: root, UploadPath: uploadDir, Recorder: failingRecorder{}, Now: time.Now(),
			AfterSnapshotSuccessForTest: func(tx *gorm.DB) {
				tx.Exec("SELECT pg_terminate_backend(pg_backend_pid())")
			},
		})
	require.Error(t, err)
	require.Contains(t, err.Error(), "журнал")

	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	require.Empty(t, entries, "пакет остался на диске, хотя не удались и commit снимка, и запись в журнал")
}
