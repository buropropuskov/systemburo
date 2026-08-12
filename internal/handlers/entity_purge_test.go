package handlers_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Физический снос данных организации по проверенному пакету (server entity purge). Тесты
// живут здесь, а не рядом с пакетом entityarchive, по тому же правилу, что export/verify/
// import (#706): второй DB-бинарь на одну тест-БД даёт гонку миграций и чисток.
//
// Фикстура - setupExportFixture (entity_export_test.go, тот же пакет handlers_test):
// организация с заявкой и приложенным файлом плюс чужая организация рядом - снос обязан
// унести только свою.

// purgeFixture готовит организацию через setupExportFixture и сразу снимает с неё
// ЗАШИФРОВАННЫЙ пакет: purge отказывает по открытому пакету (инвариант 2), поэтому тестам
// этого файла по умолчанию нужен конверт, а не открытый пакет verify/import-фикстур. crypt
// возвращается отдельно - тем же ключом снятый пакет и открывается обратно.
func purgeFixture(t *testing.T, db *gorm.DB, uploadDir string) (exportFixture, string, *services.ArchiveCrypto) {
	t.Helper()
	f := setupExportFixture(t, db, uploadDir)
	crypt := testExportCrypto(t)
	res, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: t.TempDir(), UploadPath: uploadDir, Crypto: crypt,
			Recorder: services.NewAuditRecorder(db), Now: time.Now(),
		})
	require.NoError(t, err)
	return f, res.Dir, crypt
}

func orgExistsInDB(t *testing.T, db *gorm.DB, id int) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.Raw("SELECT EXISTS(SELECT 1 FROM organizations WHERE id = ?)", id).Scan(&exists).Error)
	return exists
}

// seedCascadeFixture заполняет строки в таблицах, которые ревью среза purge (12.08) нашло
// каскадно висящими на графе организации БЕЗ собственного узла (см. комментарий пакета в
// registry.go и TestOrganizationGraph_NoUnaccountedCascades в entity_graph_test.go). Без
// этой фикстуры тест ничего не доказывает - setupExportFixture их не населяет, и ровно
// поэтому находка осталась незамеченной прежними тестами. Покрывает все три формы находки:
// прямой каскад от users.id (notifications/pd_consents/application_approvers) и два
// двухходовых - от application_questions.id и application_supplements.id.
func seedCascadeFixture(t *testing.T, db *gorm.DB, f exportFixture) {
	t.Helper()
	uid := f.app.SenderUserID

	require.NoError(t, db.Create(&models.Notification{UserID: uid, Title: strPtr("тест")}).Error)
	require.NoError(t, db.Create(&models.PDConsent{
		UserID: uid, ConsentType: "pd_processing", Granted: true, GrantedAt: time.Now(),
	}).Error)
	require.NoError(t, db.Create(&models.ApplicationApprover{UserID: uid}).Error)

	question := models.ApplicationQuestion{ApplicationID: f.app.ID, AuthorUserID: uid, Subject: "Тест", Text: "Тест"}
	require.NoError(t, db.Create(&question).Error)
	require.NoError(t, db.Create(&models.ApplicationQuestionRead{QuestionID: question.ID, UserID: uid, ReadAt: time.Now()}).Error)

	supplement := models.ApplicationSupplement{ApplicationID: f.app.ID, Number: 1, Status: "pending", CreatedByUserID: uid}
	require.NoError(t, db.Create(&supplement).Error)
	require.NoError(t, db.Create(&models.ApplicationSupplementApproval{SupplementID: supplement.ID, UserID: uid}).Error)
}

// cascadeOnlyTables - подмножество seedCascadeFixture, которое проверяют оба среза теста
// ниже (экспорт и снос): по одной таблице каждой из трёх форм находки.
var cascadeOnlyTables = []string{
	"notifications", "pd_consents", "application_approvers",
	"application_question_reads", "application_supplement_approvals",
}

// TestEntityPurge_ExportsAndDeletesCascadeOnlyTables - пробой находки 1 ревью (12.08):
// таблицы без узла графа, но с ON DELETE CASCADE от users.id/application_questions.id/
// application_supplements.id, обязаны И попасть в экспортированный пакет ДО сноса, И быть
// физически удалены, И попасть в счётчики результата (audit_log.details) - три места, где
// раньше они были невидимы одновременно.
func TestEntityPurge_ExportsAndDeletesCascadeOnlyTables(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	seedCascadeFixture(t, db, f)
	uid := f.app.SenderUserID

	crypt := testExportCrypto(t)
	eres, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: t.TempDir(), UploadPath: uploadDir, Crypto: crypt,
			Recorder: services.NewAuditRecorder(db), Now: time.Now(),
		})
	require.NoError(t, err)

	exported := make(map[string]int64, len(eres.Manifest.Tables))
	for _, tbl := range eres.Manifest.Tables {
		exported[tbl.Table] = tbl.Rows
	}
	for _, table := range cascadeOnlyTables {
		require.EqualValues(t, 1, exported[table], "таблица %s не попала в экспортированный пакет", table)
	}

	res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, eres.Dir,
		entityarchive.PurgeOptions{
			UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db), Apply: true,
		})
	require.NoError(t, err, "tables: %+v", res.Tables)

	deleted := make(map[string]int64, len(res.Tables))
	for _, tbl := range res.Tables {
		deleted[tbl.Table] = tbl.Rows
	}
	for _, table := range cascadeOnlyTables {
		require.EqualValues(t, 1, deleted[table], "снос не посчитал таблицу %s в результате", table)

		// Все пять - user_id-адресуемые (application_question_reads/
		// application_supplement_approvals несут user_id колонкой напрямую, не только
		// через question_id/supplement_id), поэтому одна и та же проверка годится для всех.
		var count int64
		require.NoError(t, db.Table(table).Where("user_id = ?", uid).Count(&count).Error)
		require.Zero(t, count, "строки таблицы %s должны были физически исчезнуть", table)
	}

	var audit models.AuditLog
	require.NoError(t, db.Where("entity_type = ? AND action = ? AND entity_id = ?",
		models.AuditEntityOrganization, models.OrganizationActionPurged, f.org.ID).First(&audit).Error)
	var details map[string]any
	require.NoError(t, json.Unmarshal(audit.Details, &details))
	tablesJSON, err := json.Marshal(details["tables"])
	require.NoError(t, err)
	for _, table := range cascadeOnlyTables {
		require.Contains(t, string(tablesJSON), table, "запись в audit_log не назвала таблицу %s", table)
	}
}

// TestEntityPurge_DeletesGraphAndFiles: главный круговой тест - снос по годному
// зашифрованному пакету физически удаляет строки графа и файл заявки с диска, пишет запись
// в audit_log без персональных данных и не задевает соседнюю организацию.
func TestEntityPurge_DeletesGraphAndFiles(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir, crypt := purgeFixture(t, db, uploadDir)
	diskPath := filepath.Join(uploadDir, "application_files", f.file.StoredName)
	_, err := os.Stat(diskPath)
	require.NoError(t, err, "файл заявки должен лежать на диске до сноса")

	res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, dir,
		entityarchive.PurgeOptions{
			UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db), Apply: true,
		})
	require.NoError(t, err, "tables: %+v", res.Tables)
	require.True(t, res.Apply)
	require.Positive(t, res.TotalRows())
	require.Equal(t, 1, res.Files)
	require.NotEmpty(t, res.ManifestSHA256)

	require.False(t, orgExistsInDB(t, db, f.org.ID), "организация должна быть удалена физически")
	var appCount int64
	require.NoError(t, db.Model(&models.Application{}).Where("id = ?", f.app.ID).Count(&appCount).Error)
	require.Zero(t, appCount)
	var fileCount int64
	require.NoError(t, db.Model(&models.ApplicationFile{}).Where("id = ?", f.file.ID).Count(&fileCount).Error)
	require.Zero(t, fileCount)

	_, statErr := os.Stat(diskPath)
	require.True(t, os.IsNotExist(statErr), "файл заявки должен быть снесён с диска")

	var audit models.AuditLog
	err = db.Where("entity_type = ? AND action = ? AND entity_id = ?",
		models.AuditEntityOrganization, models.OrganizationActionPurged, f.org.ID).First(&audit).Error
	require.NoError(t, err, "успешный снос обязан оставить запись в audit_log")

	var details map[string]any
	require.NoError(t, json.Unmarshal(audit.Details, &details))
	require.Equal(t, res.ManifestSHA256, details["manifest_sha256"], "отпечаток в журнале обязан совпасть с отпечатком результата")
	require.Equal(t, dir, details["package"])
	require.NotContains(t, string(audit.Details), f.org.Name, "details не должны нести персональные данные организации")
	require.NotContains(t, string(audit.Details), f.file.FileName, "details не должны нести имя файла")

	// Соседняя организация не задета.
	require.True(t, orgExistsInDB(t, db, f.otherOrg.ID))
	var strangerCount int64
	require.NoError(t, db.Model(&models.User{}).Where("organization_id = ?", f.otherOrg.ID).Count(&strangerCount).Error)
	require.Positive(t, strangerCount, "пользователь соседней организации не задет")
}

// TestEntityPurge_AuditHistorySurvivesPurge: audit_log не входит в граф организации и не
// имеет внешнего ключа на entity_id (models.AuditLog - "аудит должен пережить удаление
// родителя") - и более ранняя запись истории организации, и сама запись о сносе обязаны
// остаться читаемыми после того, как organizations-строка физически удалена.
func TestEntityPurge_AuditHistorySurvivesPurge(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir, crypt := purgeFixture(t, db, uploadDir)
	rec := services.NewAuditRecorder(db)
	require.NoError(t, rec.Record(context.Background(), nil, models.AuditEntityOrganization, &f.org.ID, "created", nil, nil))

	_, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, dir,
		entityarchive.PurgeOptions{UploadPath: uploadDir, Decrypt: crypt, Recorder: rec, Apply: true})
	require.NoError(t, err)
	require.False(t, orgExistsInDB(t, db, f.org.ID))

	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND entity_id = ?", models.AuditEntityOrganization, f.org.ID).
		Count(&count).Error)
	require.GreaterOrEqual(t, count, int64(2), "и запись 'created', и запись 'purged' обязаны пережить снос")
}

// TestEntityPurge_RejectsUnverifiedPackage: конверт таблицы повреждён - любой бит,
// перевёрнутый внутри age-потока, ломает аутентификацию конверта целиком, поэтому Verify
// обязан отказать раньше, чем снос коснётся базы.
func TestEntityPurge_RejectsUnverifiedPackage(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir, crypt := purgeFixture(t, db, uploadDir)

	target := filepath.Join(dir, "tables", "applications.jsonl.age")
	body, err := os.ReadFile(target)
	require.NoError(t, err)
	corrupted := append([]byte(nil), body...)
	corrupted[len(corrupted)/2] ^= 0xFF
	require.NoError(t, os.WriteFile(target, corrupted, 0o600))

	res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, dir,
		entityarchive.PurgeOptions{UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db), Apply: true})
	require.Error(t, err)
	require.NotEmpty(t, res.Package)
	require.True(t, orgExistsInDB(t, db, f.org.ID), "битый пакет не должен был ничего удалить")
}

// TestEntityPurge_RejectsPlaintextPackage: пакет без шифрования сверяет опись сам с собой -
// снос по такому пакету запрещён без исключений (в отличие от export, у purge нет флага
// -plaintext, который позволял бы обойти этот отказ).
func TestEntityPurge_RejectsPlaintextPackage(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	eres, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: t.TempDir(), UploadPath: uploadDir,
			Recorder: services.NewAuditRecorder(db), Now: time.Now(),
		})
	require.NoError(t, err)
	require.False(t, eres.Manifest.Encrypted, "фикстура обязана дать открытый пакет без Crypto")

	res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, eres.Dir,
		entityarchive.PurgeOptions{UploadPath: uploadDir, Recorder: services.NewAuditRecorder(db), Apply: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "открытым текстом")
	require.Empty(t, res.ManifestSHA256, "отпечаток не считается для отклонённого на этом шаге пакета")
	require.True(t, orgExistsInDB(t, db, f.org.ID))
}

// TestEntityPurge_RejectsCoverageMismatch: между снятием пакета и сносом у организации
// появилась новая заявка - пакет больше не покрывает текущее состояние графа, снос обязан
// отказать, а не удалить старое, оставив новую заявку сиротой без организации.
func TestEntityPurge_RejectsCoverageMismatch(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir, crypt := purgeFixture(t, db, uploadDir)

	extra := models.Application{OrganizationID: f.org.ID, SenderUserID: f.app.SenderUserID}
	require.NoError(t, db.Create(&extra).Error, "новая заявка появилась ПОСЛЕ снятия пакета")

	res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, dir,
		entityarchive.PurgeOptions{UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db), Apply: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "не покрывает текущее состояние")
	require.Empty(t, res.Tables, "отказ по покрытию не должен был дойти до удаления")

	require.True(t, orgExistsInDB(t, db, f.org.ID))
	var appCount int64
	require.NoError(t, db.Model(&models.Application{}).Where("organization_id = ?", f.org.ID).Count(&appCount).Error)
	require.EqualValues(t, 2, appCount, "ни старая, ни новая заявка не должны были пропасть")
}

// TestEntityPurge_RejectsWrongOrganizationPackage: пакет снят для одной организации, а
// снос запрошен для другой - тот же гейт Verify(wantType, wantID), на котором уже держится
// import, отказывает до того, как снос коснётся базы.
func TestEntityPurge_RejectsWrongOrganizationPackage(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir, crypt := purgeFixture(t, db, uploadDir)

	res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.otherOrg.ID, dir,
		entityarchive.PurgeOptions{UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db), Apply: true})
	require.Error(t, err)
	require.Empty(t, res.Tables)

	require.True(t, orgExistsInDB(t, db, f.org.ID), "пакет своей организации не тронут")
	require.True(t, orgExistsInDB(t, db, f.otherOrg.ID), "чужая организация, указанная по ошибке, тоже не тронута")
}

// TestEntityPurge_RejectsWrongOrganizationPackage_SameShape: пакет снят для одной
// организации, снос запрошен для другой, но графы двух организаций СОВПАДАЮТ по форме
// (по одному пользователю, по одной заявке, без файлов) - в отличие от соседнего теста
// выше, здесь сверка счётчиков покрытия ничего не поймает: единственное, что может
// остановить снос не той цели, это гейт идентичности пакета (checkIdentity внутри Verify).
func TestEntityPurge_RejectsWrongOrganizationPackage_SameShape(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	orgA := models.Organization{Name: "Организация A"}
	require.NoError(t, db.Create(&orgA).Error)
	userA := models.User{Username: "purge-identity-a", OrganizationID: &orgA.ID}
	require.NoError(t, db.Create(&userA).Error)
	appA := models.Application{OrganizationID: orgA.ID, SenderUserID: userA.ID}
	require.NoError(t, db.Create(&appA).Error)

	orgB := models.Organization{Name: "Организация B"}
	require.NoError(t, db.Create(&orgB).Error)
	userB := models.User{Username: "purge-identity-b", OrganizationID: &orgB.ID}
	require.NoError(t, db.Create(&userB).Error)
	appB := models.Application{OrganizationID: orgB.ID, SenderUserID: userB.ID}
	require.NoError(t, db.Create(&appB).Error)

	crypt := testExportCrypto(t)
	eres, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, orgA.ID,
		entityarchive.ExportOptions{
			Root: t.TempDir(), UploadPath: uploadDir, Crypto: crypt,
			Recorder: services.NewAuditRecorder(db), Now: time.Now(),
		})
	require.NoError(t, err)

	res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, orgB.ID, eres.Dir,
		entityarchive.PurgeOptions{UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db), Apply: true})
	require.Error(t, err)
	require.Empty(t, res.Tables)
	require.True(t, orgExistsInDB(t, db, orgA.ID), "пакет своей организации не тронут")
	require.True(t, orgExistsInDB(t, db, orgB.ID), "чужая организация той же формы не тронута")
}

// TestEntityPurge_DryRunDeletesNothing: без -apply команда только считает и сверяет
// покрытие - ни строк, ни файлов, ни записи в audit_log быть не должно.
func TestEntityPurge_DryRunDeletesNothing(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir, crypt := purgeFixture(t, db, uploadDir)
	diskPath := filepath.Join(uploadDir, "application_files", f.file.StoredName)

	res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, dir,
		entityarchive.PurgeOptions{UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db), Apply: false})
	require.NoError(t, err)
	require.False(t, res.Apply)
	require.Positive(t, res.TotalRows())
	require.Equal(t, 1, res.Files)

	require.True(t, orgExistsInDB(t, db, f.org.ID), "пробный прогон не должен был удалить организацию")
	_, statErr := os.Stat(diskPath)
	require.NoError(t, statErr, "пробный прогон не должен был трогать файл на диске")

	var count int64
	require.NoError(t, db.Model(&models.AuditLog{}).
		Where("entity_type = ? AND action = ? AND entity_id = ?", models.AuditEntityOrganization, models.OrganizationActionPurged, f.org.ID).
		Count(&count).Error)
	require.Zero(t, count, "пробный прогон не должен был писать в audit_log")
}

// TestEntityPurge_RequiresRecorder: Apply без журнала аудита - явный отказ, тот же приём,
// что у export/import: снос без следа в audit_log запрещён.
func TestEntityPurge_RequiresRecorder(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir, crypt := purgeFixture(t, db, uploadDir)

	_, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, dir,
		entityarchive.PurgeOptions{UploadPath: uploadDir, Decrypt: crypt, Apply: true})
	require.Error(t, err)
	require.Contains(t, err.Error(), "аудита")
	require.True(t, orgExistsInDB(t, db, f.org.ID))
}

// TestEntityPurge_RejectsForgedEncryptedFlag - находка ревью среза purge (12.08): пакет
// снят БЕЗ шифрования (обычный открытый manifest.json), но кто-то подделал в его теле
// "encrypted": true. Раньше это трогало ровно то поле, которое проверял единственный гейт
// purge на шифрование ("пакет обязан быть зашифрован") - подмена проходила его мимо, не
// будучи зашифрованной ни единым байтом. Verify теперь ловит расхождение сам
// (checkEncryptionConsistency), но проверяем именно со стороны purge - той защиты, которую
// подмена и должна была обойти.
func TestEntityPurge_RejectsForgedEncryptedFlag(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f := setupExportFixture(t, db, uploadDir)
	eres, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
		entityarchive.ExportOptions{
			Root: t.TempDir(), UploadPath: uploadDir,
			Recorder: services.NewAuditRecorder(db), Now: time.Now(),
		})
	require.NoError(t, err)
	require.False(t, eres.Manifest.Encrypted, "фикстура обязана дать открытый пакет без Crypto")

	mutateManifest(t, eres.Dir, func(m *entityarchive.Manifest) { m.Encrypted = true })

	res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, eres.Dir,
		entityarchive.PurgeOptions{UploadPath: uploadDir, Recorder: services.NewAuditRecorder(db), Apply: true})
	require.Error(t, err)
	require.Empty(t, res.ManifestSHA256, "отпечаток не считается для пакета, отклонённого на проверке")
	require.True(t, orgExistsInDB(t, db, f.org.ID), "поддельный флаг шифрования не должен был снести организацию")
}

// TestEntityPurge_SharedReportTemplatesSurviveDetached: report_templates каскадится от
// пользователя молча (FK OwnerUserID OnDelete:CASCADE) - без отдельного шага строка ушла бы
// вместе с автором штатным узлом графа. Но is_shared делает шаблон видимым ВСЕМ, кто им
// пользуется, не только автору, а снос отвечает только за пользователей СВОЕЙ организации -
// общий шаблон обязан пережить снос своего автора (owner_user_id -> NULL ДО удаления узлов
// графа), а личный - обязан уйти вместе с ним, как и раньше. Оператор обязан увидеть
// предупреждение о предстоящей отвязке ДО -apply, а не после.
func TestEntityPurge_SharedReportTemplatesSurviveDetached(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	t.Run("общий шаблон переживает снос и виден чужому пользователю, личный уходит с автором", func(t *testing.T) {
		testutil.CleanDB(t, db)

		// Шаблоны заводятся ДО снятия пакета, а не после (в отличие от purgeFixture) - иначе
		// снимок не покрыл бы текущее состояние графа, и Purge отказал бы сверкой покрытия
		// раньше, чем дошёл бы до отвязки.
		f := setupExportFixture(t, db, uploadDir)
		uid := f.app.SenderUserID
		shared := models.ReportTemplate{
			Name: "Общий отчёт", Config: json.RawMessage(`{}`), IsShared: true, OwnerUserID: &uid,
		}
		require.NoError(t, db.Create(&shared).Error)
		personal := models.ReportTemplate{
			Name: "Личный отчёт", Config: json.RawMessage(`{}`), OwnerUserID: &uid,
		}
		require.NoError(t, db.Create(&personal).Error)

		// Пользователь чужой, не сносимой организации - именно ради него шаблон и
		// расшаривали; после сноса он не должен его лишиться.
		otherOrgUser := models.User{Username: "purge-tpl-other-user", OrganizationID: &f.otherOrg.ID}
		require.NoError(t, db.Create(&otherOrgUser).Error)

		crypt := testExportCrypto(t)
		eres, err := entityarchive.Export(context.Background(), db, entityarchive.TypeOrganization, f.org.ID,
			entityarchive.ExportOptions{
				Root: t.TempDir(), UploadPath: uploadDir, Crypto: crypt,
				Recorder: services.NewAuditRecorder(db), Now: time.Now(),
			})
		require.NoError(t, err)

		dry, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, eres.Dir,
			entityarchive.PurgeOptions{UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db)})
		require.NoError(t, err, "tables: %+v", dry.Tables)
		require.Len(t, dry.Warnings, 1)
		require.Contains(t, dry.Warnings[0], "report_templates")
		require.True(t, orgExistsInDB(t, db, f.org.ID), "пробный прогон не должен был удалить организацию")

		res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, eres.Dir,
			entityarchive.PurgeOptions{UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db), Apply: true})
		require.NoError(t, err, "tables: %+v", res.Tables)
		require.Len(t, res.Warnings, 1)
		require.Contains(t, res.Warnings[0], "report_templates")
		require.False(t, orgExistsInDB(t, db, f.org.ID))

		// Счётчики различают удалённое и отвязанное: личный - в Tables (удалён), общий - в
		// отдельном DetachedReportTemplates, а не смешаны в одну цифру.
		require.EqualValues(t, 1, res.DetachedReportTemplates, "отвязан ровно общий шаблон")
		var reportTemplatesDeleted int64 = -1
		for _, tbl := range res.Tables {
			if tbl.Table == "report_templates" {
				reportTemplatesDeleted = tbl.Rows
			}
		}
		require.EqualValues(t, 1, reportTemplatesDeleted, "report_templates: удалён только личный, общий отвязан, а не удалён")

		// Общий шаблон физически остался в базе осиротевшим (owner_user_id -> NULL), личный
		// исчез вместе с автором.
		var survivor models.ReportTemplate
		require.NoError(t, db.First(&survivor, shared.ID).Error, "общий шаблон обязан пережить снос организации-автора")
		require.Nil(t, survivor.OwnerUserID)
		require.True(t, survivor.IsShared)
		require.ErrorIs(t, db.First(&models.ReportTemplate{}, personal.ID).Error, gorm.ErrRecordNotFound,
			"личный шаблон обязан уйти вместе с автором штатным каскадом")

		// И виден в выборке чужому пользователю - ровно тому, ради кого его расшаривали и
		// кто заявку на снос своей организации не подавал.
		svc := services.NewStatisticsService(db, 0)
		list, err := svc.ListReportTemplates(context.Background(), otherOrgUser.ID)
		require.NoError(t, err)
		found := false
		for _, tpl := range list {
			if tpl.ID == shared.ID {
				found = true
			}
		}
		require.True(t, found, "общий шаблон обязан остаться виден чужому пользователю после сноса автора")

		// Журнал различает удалённое и отвязанное тем же приёмом, что и результат команды.
		var entry models.AuditLog
		require.NoError(t, db.Where("entity_type = ? AND action = ? AND entity_id = ?",
			models.AuditEntityOrganization, models.OrganizationActionPurged, f.org.ID).First(&entry).Error)
		var details struct {
			DetachedReportTemplates int64 `json:"detached_report_templates"`
		}
		require.NoError(t, json.Unmarshal(entry.Details, &details))
		require.EqualValues(t, 1, details.DetachedReportTemplates)
	})

	t.Run("без общих шаблонов - предупреждения нет и отвязывать нечего", func(t *testing.T) {
		testutil.CleanDB(t, db)

		f, dir, crypt := purgeFixture(t, db, uploadDir)

		res, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, dir,
			entityarchive.PurgeOptions{UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db)})
		require.NoError(t, err, "tables: %+v", res.Tables)
		require.Empty(t, res.Warnings)
		require.Zero(t, res.DetachedReportTemplates)
	})
}
