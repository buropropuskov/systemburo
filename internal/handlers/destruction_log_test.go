package handlers_test

// Журнал уничтожения персональных данных (#2357).
//
// Удаление из работающей системы не удаляет данные из снятых ранее копий: восстановление
// вернёт и то, что оператор обязан был уничтожить. Перечень уничтоженного - единственное,
// по чему удаления применяются заново, поэтому тесты стерегут не «функция дописала строку»,
// а три свойства, без которых журнал бесполезен или вреден:
//
//   - запись появляется В КАЖДОЙ точке уничтожения и в той же транзакции, что само
//     уничтожение: нет действия - нет свидетельства, и наоборот;
//   - персональных данных в записи нет. Журнал живёт дольше всего остального, и стать
//     последним местом, где уцелел паспорт, он не должен;
//   - уборка журналов его не трогает. Ушёл бы он по сроку - ушёл бы раньше копий, ради
//     которых заведён.

import (
	"context"
	"strings"
	"testing"
	"time"

	"systemburo/internal/database"
	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// destructionRecords - записи журнала уничтожения, старые раньше.
func destructionRecords(t *testing.T, db *gorm.DB) []models.DestructionRecord {
	t.Helper()
	var out []models.DestructionRecord
	require.NoError(t, db.Order("id").Find(&out).Error)
	return out
}

// destructionDump - все записи журнала одной строкой текста, со всеми столбцами.
// Именно строкой: проверка «нет персональных данных» обязана смотреть на ВСЕ поля, а
// не на те, о которых вспомнил автор теста, - новое поле иначе прошло бы мимо неё.
func destructionDump(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var rows []string
	require.NoError(t, db.Raw(`SELECT row_to_json(d)::text FROM destruction_log d`).Scan(&rows).Error)
	return strings.Join(rows, "\n")
}

func TestDestructionLog_ApplicationAnonymize(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	appID, _ := applicationWithPeople(t, db, "№ 20260301/701")

	_, err := entityarchive.AnonymizeApplication(context.Background(), db,
		services.NewAuditRecorder(db), appID,
		entityarchive.DestructionOptions{Basis: entityarchive.BasisRetention, Apply: true})
	require.NoError(t, err)

	records := destructionRecords(t, db)
	require.Len(t, records, 1)
	rec := records[0]
	assert.Equal(t, models.AuditEntityApplication, rec.EntityType)
	require.NotNil(t, rec.EntityID)
	assert.Equal(t, appID, *rec.EntityID)
	assert.Equal(t, entityarchive.DestructionAnonymized, rec.Action)
	assert.Equal(t, entityarchive.BasisRetention, rec.Basis)
	assert.Equal(t, "№ 20260301/701", rec.ApplicationNumber,
		"номер опознаёт заявку в акте: идентификатор человеку ничего не говорит")
	assert.Positive(t, rec.Rows)
	assert.Equal(t, "суточный прогон по сроку хранения", rec.ActorNote,
		"по чьему решению - обязательная графа акта")
	assert.Zero(t, rec.Replays)
	assert.Nil(t, rec.ReplayedAt)
}

func TestDestructionLog_ApplicationPurge(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	appID, _ := applicationWithPeople(t, db, "№ 20260301/702")

	_, err := entityarchive.PurgeApplication(context.Background(), db,
		services.NewAuditRecorder(db), appID,
		entityarchive.DestructionOptions{Basis: entityarchive.BasisOperator, Apply: true})
	require.NoError(t, err)

	records := destructionRecords(t, db)
	require.Len(t, records, 1)
	assert.Equal(t, entityarchive.DestructionPurged, records[0].Action)
	assert.Equal(t, entityarchive.BasisOperator, records[0].Basis)
	assert.Equal(t, "№ 20260301/702", records[0].ApplicationNumber)
	assert.Equal(t, "оператор сервера (консоль)", records[0].ActorNote)

	// Уничтожение заявки удаляет её историю из audit_log - в пояснениях записей стоят
	// ФИО. Значит, свидетельство обязано лежать не там: иначе восстановление из копии,
	// снятой ДО уничтожения, вернуло бы заявку и не оставило следа, по которому её
	// уничтожают заново.
	var history int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM audit_log
		WHERE entity_type = 'application' AND entity_id = ? AND action <> ?`,
		appID, models.OrganizationActionPurged).Scan(&history).Error)
	assert.Zero(t, history)
}

func TestDestructionLog_SubjectAnonymizeKeepsDocumentDigest(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	passport, _ := anonymizeSubjectFixture(t, db)

	target := entityarchive.SubjectTargetFromDocuments(passport, "")
	_, err := entityarchive.AnonymizeSubject(context.Background(), db,
		services.NewAuditRecorder(db), target,
		entityarchive.DestructionOptions{Basis: entityarchive.BasisSubjectRequest, Apply: true})
	require.NoError(t, err)

	records := destructionRecords(t, db)
	require.Len(t, records, 1)
	rec := records[0]
	assert.Equal(t, models.AuditEntityUniqueEmployee, rec.EntityType)
	assert.Equal(t, entityarchive.BasisSubjectRequest, rec.Basis)
	require.NotEmpty(t, rec.PassportDigest, "без отпечатка вернувшегося человека не найти")
	assert.Len(t, rec.PassportDigest, 64, "отпечаток - sha256 в шестнадцатеричном виде")
	assert.NotContains(t, rec.PassportDigest, "505050",
		"в журнал уходит отпечаток, а не сам документ")
	assert.Empty(t, rec.PatentDigest, "патента у цели не было")
}

func TestDestructionLog_OrganizationAnonymize(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	f := setupAnonymizeFixture(t, db, "Уничтожение-журнал")

	_, err := entityarchive.Anonymize(context.Background(), db, services.NewAuditRecorder(db),
		entityarchive.TypeOrganization, f.org.ID,
		entityarchive.DestructionOptions{Basis: entityarchive.BasisOperator, Apply: true})
	require.NoError(t, err)

	records := destructionRecords(t, db)
	require.Len(t, records, 1)
	require.NotNil(t, records[0].EntityID)
	assert.Equal(t, models.AuditEntityOrganization, records[0].EntityType)
	assert.Equal(t, f.org.ID, *records[0].EntityID)
	assert.Equal(t, entityarchive.DestructionAnonymized, records[0].Action)
}

func TestDestructionLog_OrganizationPurge(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir, crypt := purgeFixture(t, db, uploadDir)
	_, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, f.org.ID, dir,
		entityarchive.PurgeOptions{
			Basis:      entityarchive.BasisOperator,
			UploadPath: uploadDir, Decrypt: crypt, Recorder: services.NewAuditRecorder(db), Apply: true,
		})
	require.NoError(t, err)

	records := destructionRecords(t, db)
	require.Len(t, records, 1)
	assert.Equal(t, models.AuditEntityOrganization, records[0].EntityType)
	assert.Equal(t, entityarchive.DestructionPurged, records[0].Action)
	assert.Positive(t, records[0].Rows)

	// Организации больше нет, а свидетельство о её сносе осталось: журнал не входит в
	// граф организации и уйти вместе с ней не может.
	assert.False(t, orgExistsInDB(t, db, f.org.ID))
}

// Уничтожение без основания не является свидетельством, и подставлять умолчание вместо
// пропущенного основания нельзя: запись в журнале выглядела бы законной, ничего при этом
// не подтверждая. Показ объёма без -apply основания не требует - иначе оператор вписывал
// бы первое попавшееся, только чтобы увидеть числа.
func TestDestructionLog_RefusesWithoutBasis(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	appID, _ := applicationWithPeople(t, db, "№ 20260301/703")

	_, err := entityarchive.AnonymizeApplication(context.Background(), db,
		services.NewAuditRecorder(db), appID,
		entityarchive.DestructionOptions{Apply: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "основание")

	_, err = entityarchive.AnonymizeApplication(context.Background(), db,
		services.NewAuditRecorder(db), appID,
		entityarchive.DestructionOptions{Apply: true, Basis: "по звонку"})
	require.Error(t, err, "основание берётся из перечня, а не из фантазии вызывающего")

	_, err = entityarchive.AnonymizeApplication(context.Background(), db,
		services.NewAuditRecorder(db), appID, entityarchive.DestructionOptions{})
	require.NoError(t, err, "показ объёма основания не требует")

	assert.Empty(t, destructionRecords(t, db), "ни одна отказавшая попытка не оставила следа")
}

// Главное требование задачи: персональных данных в журнале нет. Проверяется по всем
// столбцам сразу (row_to_json), а не по перечню полей - новое поле иначе прошло бы мимо.
func TestDestructionLog_KeepsNoPersonalData(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	appID, _ := applicationWithPeople(t, db, "№ 20260301/704")
	_, err := entityarchive.AnonymizeApplication(context.Background(), db,
		services.NewAuditRecorder(db), appID,
		entityarchive.DestructionOptions{Basis: entityarchive.BasisRetention, Apply: true})
	require.NoError(t, err)

	passport, _ := anonymizeSubjectFixture(t, db)
	_, err = entityarchive.AnonymizeSubject(context.Background(), db,
		services.NewAuditRecorder(db), entityarchive.SubjectTargetFromDocuments(passport, ""),
		entityarchive.DestructionOptions{Basis: entityarchive.BasisSubjectRequest, Apply: true})
	require.NoError(t, err)

	require.Len(t, destructionRecords(t, db), 2)
	dump := destructionDump(t, db)
	for _, secret := range []string{
		"Хранимов", "Олег", "Монтажник", "4510 300300", // участник заявки
		"Стираев", "Артём", passport, // субъект
		"505050", "300300", // номера документов без пробела: свёртка не должна их нести
	} {
		assert.NotContains(t, dump, secret,
			"персональные данные в журнале уничтожения: %q", secret)
	}
}

// Журнал уничтожения обязан пережить уборку. Ушёл бы он по сроку группы audit - ушёл бы
// раньше копий, ради повторного применения к которым он и заведён.
func TestDestructionLog_SurvivesCleanup(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	appID, _ := applicationWithPeople(t, db, "№ 20260301/705")
	_, err := entityarchive.AnonymizeApplication(context.Background(), db,
		services.NewAuditRecorder(db), appID,
		entityarchive.DestructionOptions{Basis: entityarchive.BasisRetention, Apply: true})
	require.NoError(t, err)
	require.Len(t, destructionRecords(t, db), 1)

	// Состариваем запись сильнее любого разумного срока: уборка обязана не трогать её
	// не потому, что запись свежая, а потому, что группы для неё нет вовсе.
	require.NoError(t, db.Exec(`UPDATE destruction_log SET created_at = ?`,
		time.Now().UTC().AddDate(-20, 0, 0)).Error)

	// Суточная уборка с назначенным сроком истории.
	database.SweepRoutine(context.Background(), db, 1, 1, 1, 1, 1, 1)
	// И ручная чистка всех групп разом, границей в будущем - жёстче не бывает.
	targets, err := database.SelectRetentionTargets("all", "")
	require.NoError(t, err)
	for _, target := range targets {
		_, err := database.SweepRetention(context.Background(), db, target,
			database.SweepOptions{Cutoff: time.Now().UTC().AddDate(1, 0, 0), Apply: true})
		require.NoError(t, err)
	}

	assert.Len(t, destructionRecords(t, db), 1,
		"перечень уничтоженного обязан пережить любую уборку журналов")
}
