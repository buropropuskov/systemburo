package handlers_test

// Обезличивание одной заявки по истечении срока хранения (#2355).
//
// Данные не удаляются, а обезличиваются: заявка, её даты, статус и факт прохода
// остаются, ФИО и документы затираются необратимо. Проверяется и то, и другое -
// «затёрлось» без «осталось» означало бы потерю сведений, по которым отвечают на
// запросы государственных органов.

import (
	"context"
	"testing"
	"time"

	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// applicationWithPeople заводит заявку с двумя участниками и отметкой прохода.
func applicationWithPeople(t *testing.T, db *gorm.DB, number string) (appID, otherAppID int) {
	t.Helper()

	org := models.Organization{Name: "Срок-эксперимент " + number}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "retention-owner-" + number, OrganizationID: &org.ID}
	require.NoError(t, db.Create(&user).Error)

	status := "Завершено"
	app := models.Application{
		OrganizationID: org.ID, SenderUserID: user.ID,
		ApplicationNumber: &number, Status: &status,
	}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, OrganizationID: &org.ID}
	require.NoError(t, db.Create(&att).Error)

	last, first, position := "Хранимов", "Олег", "Монтажник"
	passport := "4510 300300"
	require.NoError(t, db.Create(&models.Employee{
		AttachmentID: &att.ID, LastName: &last, FirstName: &first, Position: &position,
		PassportSeriesNumber: &passport,
	}).Error)

	// Отметка прохода: журнал не трогается, и это надо подтвердить.
	var empID int
	require.NoError(t, db.Raw(`SELECT id FROM employees WHERE attachment_id = ?`, att.ID).Scan(&empID).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO audit_log (entity_type, entity_id, action, details, created_at)
		VALUES ('employee', ?, 'entry', '{"comment": "Сотрудник Хранимов Олег прошёл"}'::jsonb, ?)`,
		empID, time.Now().UTC()).Error)

	// Соседняя заявка с другим человеком - её обезличивание касаться не должно.
	otherNumber := number + "-сосед"
	otherApp := models.Application{
		OrganizationID: org.ID, SenderUserID: user.ID,
		ApplicationNumber: &otherNumber, Status: &status,
	}
	require.NoError(t, db.Create(&otherApp).Error)
	otherAtt := models.Attachment{ApplicationID: &otherApp.ID, OrganizationID: &org.ID}
	require.NoError(t, db.Create(&otherAtt).Error)
	otherLast, otherFirst := "Соседов", "Пётр"
	otherPassport := "4510 400400"
	require.NoError(t, db.Create(&models.Employee{
		AttachmentID: &otherAtt.ID, LastName: &otherLast, FirstName: &otherFirst,
		PassportSeriesNumber: &otherPassport,
	}).Error)

	return app.ID, otherApp.ID
}

func TestAnonymizeApplication_DryRunChangesNothing(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	appID, _ := applicationWithPeople(t, db, "№ 20260101/001")

	res, err := entityarchive.AnonymizeApplication(context.Background(), db,
		services.NewAuditRecorder(db), entityarchive.FilePaths{}, appID, nil, false)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Warnings, "оператор обязан узнать, что остаётся после затирания")

	var last *string
	require.NoError(t, db.Raw(`SELECT last_name FROM employees
		WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)`, appID).
		Scan(&last).Error)
	require.NotNil(t, last, "без -apply база не меняется")
	assert.Equal(t, "Хранимов", *last)
}

func TestAnonymizeApplication_WipesPeopleKeepsApplication(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	appID, otherAppID := applicationWithPeople(t, db, "№ 20260101/002")

	_, err := entityarchive.AnonymizeApplication(context.Background(), db,
		services.NewAuditRecorder(db), entityarchive.FilePaths{}, appID, nil, true)
	require.NoError(t, err)

	t.Run("ФИО и документ участника стёрты", func(t *testing.T) {
		var last, passport, hmac *string
		require.NoError(t, db.Raw(`SELECT last_name, passport_series_number, passport_series_number_hmac
			FROM employees WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)`, appID).
			Row().Scan(&last, &passport, &hmac))
		assert.Nil(t, last)
		assert.Nil(t, passport)
		// Отпечаток стирается вместе со значением: по нему можно проверить гипотезу
		// «это паспорт такого-то» и после затирания.
		assert.Nil(t, hmac)
	})

	t.Run("сама заявка остаётся: номер, статус, организация", func(t *testing.T) {
		var number, status *string
		var orgID int
		require.NoError(t, db.Raw(`SELECT application_number, status, organization_id FROM applications WHERE id = ?`, appID).
			Row().Scan(&number, &status, &orgID))
		require.NotNil(t, number)
		assert.Equal(t, "№ 20260101/002", *number)
		require.NotNil(t, status)
		assert.Equal(t, "Завершено", *status, "статистика по заявкам обязана пережить обезличивание")
		assert.NotZero(t, orgID)
	})

	t.Run("журнал проходов не тронут", func(t *testing.T) {
		var passages int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM audit_log WHERE entity_type = 'employee' AND action = 'entry'`).
			Scan(&passages).Error)
		assert.Greater(t, passages, int64(0),
			"журнал доказывает, кто и когда был на объекте - обезличив его, оператор лишится доказательства")
	})

	t.Run("соседняя заявка не тронута", func(t *testing.T) {
		var last *string
		require.NoError(t, db.Raw(`SELECT last_name FROM employees
			WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)`, otherAppID).
			Scan(&last).Error)
		require.NotNil(t, last)
		assert.Equal(t, "Соседов", *last)
	})

	t.Run("обезличивание попало в историю заявки", func(t *testing.T) {
		var logged int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM audit_log WHERE entity_type = 'application' AND entity_id = ? AND action = 'anonymized'`, appID).
			Scan(&logged).Error)
		assert.Equal(t, int64(1), logged, "необратимая операция без следа в истории не считается выполненной")
	})
}

func TestAnonymizeApplication_NotFound(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	_, err := entityarchive.AnonymizeApplication(context.Background(), db,
		services.NewAuditRecorder(db), entityarchive.FilePaths{}, 999999, nil, false)
	require.Error(t, err, "несуществующая заявка - отказ, а не отчёт об успешном затирании нуля строк")
}
