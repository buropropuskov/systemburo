package handlers_test

// Повторное применение уничтожений после восстановления из копии (#2357).
//
// Восстановление имитируется тем, что данные заводятся заново с теми же
// идентификаторами: pg_restore возвращает строки ровно так, и на устойчивости
// идентификаторов проход и держится. Человек - исключение: своего идентификатора у
// него нет, и вернувшегося находят пересчётом отпечатка документа.
//
// Тесты стерегут не «команда отработала», а три границы: вернувшееся снимается,
// невернувшееся не трогается (иначе каждый прогон плодил бы записи в истории), а то,
// что автоматически повторить нельзя, названо вслух, а не пропущено молча.

import (
	"context"
	"testing"

	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// restoreApplicationWithPeople возвращает заявку «из копии»: те же идентификаторы
// заявки, вложения и участника, снова с именем и паспортом. Именно так выглядит база
// после pg_restore копии, снятой до уничтожения.
func restoreApplicationWithPeople(t *testing.T, db *gorm.DB, appID, attID int, number string) {
	t.Helper()

	status := "Завершено"
	require.NoError(t, db.Exec(`
		INSERT INTO applications (id, organization_id, sender_user_id, application_number, status)
		SELECT ?, o.id, u.id, ?, ?
		FROM organizations o, users u
		WHERE u.organization_id = o.id
		ORDER BY o.id LIMIT 1
		ON CONFLICT (id) DO NOTHING`, appID, number, status).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO attachments (id, application_id, organization_id)
		SELECT ?, ?, organization_id FROM applications WHERE id = ?
		ON CONFLICT (id) DO NOTHING`, attID, appID, appID).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO employees (attachment_id, last_name, first_name, passport_series_number)
		VALUES (?, 'Хранимов', 'Олег', '4510 300300')`, attID).Error)
}

// attachmentOf - вложение заявки: тест возвращает его, чтобы завести «вернувшиеся»
// строки под теми же идентификаторами.
func attachmentOf(t *testing.T, db *gorm.DB, appID int) int {
	t.Helper()
	var id int
	require.NoError(t, db.Raw(`SELECT id FROM attachments WHERE application_id = ?`, appID).Scan(&id).Error)
	return id
}

func TestReplay_AnonymizedApplicationCameBack(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	appID, _ := applicationWithPeople(t, db, "№ 20260401/801")
	attID := attachmentOf(t, db, appID)
	recorder := services.NewAuditRecorder(db)

	_, err := entityarchive.AnonymizeApplication(context.Background(), db, recorder, appID,
		entityarchive.DestructionOptions{Basis: entityarchive.BasisRetention, Apply: true})
	require.NoError(t, err)

	// Пока ничего не возвращалось, проход обязан молчать: иначе каждый прогон гонял бы
	// обезличивание по уже обезличенным заявкам, плодя записи в истории.
	quiet, err := entityarchive.ReplayDestructions(context.Background(), db, recorder,
		entityarchive.FilePaths{}, true)
	require.NoError(t, err)
	assert.Equal(t, 1, quiet.Checked)
	assert.Zero(t, quiet.Applied, "данные не возвращались - снимать нечего")
	assert.Equal(t, 1, quiet.Gone)

	// Восстановление из копии: участник заявки снова с именем и паспортом.
	require.NoError(t, db.Exec(`
		INSERT INTO employees (attachment_id, last_name, first_name, passport_series_number)
		VALUES (?, 'Хранимов', 'Олег', '4510 300300')`, attID).Error)

	dry, err := entityarchive.ReplayDestructions(context.Background(), db, recorder,
		entityarchive.FilePaths{}, false)
	require.NoError(t, err)
	assert.Equal(t, 1, dry.Applied, "вернувшаяся заявка обязана попасть в показ")
	var stillNamed int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM employees
		WHERE attachment_id = ? AND last_name IS NOT NULL`, attID).Scan(&stillNamed).Error)
	assert.EqualValues(t, 1, stillNamed, "показ без -apply базу не трогает")

	res, err := entityarchive.ReplayDestructions(context.Background(), db, recorder,
		entityarchive.FilePaths{}, true)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Applied)

	require.NoError(t, db.Raw(`SELECT count(*) FROM employees
		WHERE attachment_id = ? AND last_name IS NOT NULL`, attID).Scan(&stillNamed).Error)
	assert.Zero(t, stillNamed, "вернувшийся из копии участник обязан быть обезличен заново")

	// Запись осталась на месте и знает о повторе: следующая копия может быть старше.
	records := destructionRecords(t, db)
	require.Len(t, records, 1, "повтор не заводит второго свидетельства")
	assert.Equal(t, 1, records[0].Replays)
	require.NotNil(t, records[0].ReplayedAt)
}

func TestReplay_PurgedApplicationCameBack(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	appID, _ := applicationWithPeople(t, db, "№ 20260401/802")
	attID := attachmentOf(t, db, appID)
	recorder := services.NewAuditRecorder(db)

	_, err := entityarchive.PurgeApplication(context.Background(), db, recorder, appID,
		entityarchive.DestructionOptions{Basis: entityarchive.BasisOperator, Apply: true})
	require.NoError(t, err)

	restoreApplicationWithPeople(t, db, appID, attID, "№ 20260401/802")
	var back int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM applications WHERE id = ?`, appID).Scan(&back).Error)
	require.EqualValues(t, 1, back, "заявка вернулась из копии")

	res, err := entityarchive.ReplayDestructions(context.Background(), db, recorder,
		entityarchive.FilePaths{}, true)
	require.NoError(t, err)
	assert.Equal(t, 1, res.Applied)

	require.NoError(t, db.Raw(`SELECT count(*) FROM applications WHERE id = ?`, appID).Scan(&back).Error)
	assert.Zero(t, back, "вернувшаяся заявка обязана быть уничтожена заново")

	records := destructionRecords(t, db)
	require.Len(t, records, 1)
	assert.Equal(t, 1, records[0].Replays)
}

func TestReplay_SubjectFoundByDocumentDigest(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	passport, _ := anonymizeSubjectFixture(t, db)
	recorder := services.NewAuditRecorder(db)

	_, err := entityarchive.AnonymizeSubject(context.Background(), db, recorder,
		entityarchive.SubjectTargetFromDocuments(passport, ""),
		entityarchive.DestructionOptions{Basis: entityarchive.BasisSubjectRequest, Apply: true})
	require.NoError(t, err)

	// Человек вернулся из копии: у него нет своего идентификатора, и единственная
	// зацепка - отпечаток документа, записанный в журнал.
	var attID int
	require.NoError(t, db.Raw(`SELECT id FROM attachments ORDER BY id LIMIT 1`).Scan(&attID).Error)
	restored := models.Employee{
		AttachmentID: &attID, PassportSeriesNumber: &passport,
		LastName: testutil.Ptr("Стираев"), FirstName: testutil.Ptr("Артём"),
	}
	require.NoError(t, db.Create(&restored).Error)

	res, err := entityarchive.ReplayDestructions(context.Background(), db, recorder,
		entityarchive.FilePaths{}, true)
	require.NoError(t, err)
	require.Equal(t, 1, res.Applied, "человек обязан найтись по отпечатку документа")

	// Проверяем ту самую строку, а не поиск по фамилии: в фикстуре есть однофамилец с
	// другим документом, и счёт по имени спутал бы «не обезличили» с «правильно не
	// тронули чужого».
	var name *string
	require.NoError(t, db.Raw(`SELECT last_name FROM employees WHERE id = ?`, restored.ID).
		Scan(&name).Error)
	assert.Nil(t, name, "вернувшийся человек обязан быть обезличен заново")

	// Однофамилец с другим документом остаётся нетронутым: склейка идёт по документу,
	// а не по имени, - иначе проход обезличил бы чужого человека.
	var other int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM employees
		WHERE passport_series_number = ? AND last_name IS NOT NULL`, "4510 606060").Scan(&other).Error)
	assert.EqualValues(t, 1, other)
}

// Снос организации идёт только по проверенному пакету выгрузки, и повторить его
// автоматически нечем. Такая запись обязана быть названа вслух: пропусти её проход
// молча, и оператор решит, что после восстановления всё снято.
func TestReplay_OrganizationPurgeNeedsHands(t *testing.T) {
	_, db, uploadDir, cleanup := testutil.SetupTestAppWithUploads(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	f, dir, crypt := purgeFixture(t, db, uploadDir)
	orgID := f.org.ID
	recorder := services.NewAuditRecorder(db)
	_, err := entityarchive.Purge(context.Background(), db, entityarchive.TypeOrganization, orgID, dir,
		entityarchive.PurgeOptions{
			Basis:      entityarchive.BasisOperator,
			UploadPath: uploadDir, Decrypt: crypt, Recorder: recorder, Apply: true,
		})
	require.NoError(t, err)
	require.False(t, orgExistsInDB(t, db, orgID))

	// Организация вернулась из копии под тем же идентификатором.
	require.NoError(t, db.Exec(`INSERT INTO organizations (id, name) VALUES (?, ?)`,
		orgID, "Вернувшаяся из копии").Error)

	res, err := entityarchive.ReplayDestructions(context.Background(), db, recorder,
		entityarchive.FilePaths{UploadPath: uploadDir}, true)
	require.NoError(t, err)
	assert.Zero(t, res.Applied)
	require.Equal(t, 1, res.Manual, "снос по пакету автоматически не повторяется")
	require.Len(t, res.Outcomes, 1)
	assert.Equal(t, entityarchive.ReplayManual, res.Outcomes[0].Status)
	assert.Contains(t, res.Outcomes[0].Detail, "снесите её заново по пакету")

	assert.True(t, orgExistsInDB(t, db, orgID), "проход не сносит организацию сам")
	records := destructionRecords(t, db)
	require.Len(t, records, 1)
	assert.Zero(t, records[0].Replays, "повтора не было - отмечать нечего")
}
