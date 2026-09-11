package handlers_test

// Обезличивание заявок по истечении срока хранения (#2355).
//
// Срок задаёт владелец системы, по умолчанию его нет. Проверяется отбор: заявка в
// работе не трогается никогда, свежая - пока не вышел срок, уже обезличенная не берётся
// повторно. Ошибка в любом из трёх условий означает необратимо затёртые данные.

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

// retentionApplication заводит заявку с участником, статусом и опорной датой.
func retentionApplication(t *testing.T, db *gorm.DB, number, status string, completedAt time.Time) int {
	t.Helper()

	org := models.Organization{Name: "Срок " + number}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "ret-" + number, OrganizationID: &org.ID}
	require.NoError(t, db.Create(&user).Error)

	app := models.Application{
		OrganizationID: org.ID, SenderUserID: user.ID,
		ApplicationNumber: &number, Status: &status,
		SendingDatetime: &completedAt,
	}
	if status == "Завершено" {
		app.CompletedAt = &completedAt
	}
	require.NoError(t, db.Create(&app).Error)

	att := models.Attachment{ApplicationID: &app.ID, OrganizationID: &org.ID}
	require.NoError(t, db.Create(&att).Error)
	last, first := "Срокин", "Иван"
	passport := "4510 700700"
	require.NoError(t, db.Create(&models.Employee{
		AttachmentID: &att.ID, LastName: &last, FirstName: &first,
		PassportSeriesNumber: &passport,
	}).Error)
	return app.ID
}

func TestApplicationRetention_PicksOnlyExpiredAndFinished(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	old := retentionApplication(t, db, "старая", "Завершено", now.AddDate(-6, 0, 0))
	fresh := retentionApplication(t, db, "свежая", "Завершено", now.AddDate(0, -1, 0))
	inWork := retentionApplication(t, db, "в работе", "В обработке", now.AddDate(-6, 0, 0))

	cutoff := now.AddDate(-5, 0, 0)
	found, err := entityarchive.FindApplicationsForRetention(context.Background(), db, cutoff, 100)
	require.NoError(t, err)

	ids := map[int]bool{}
	for _, c := range found {
		ids[c.ID] = true
	}
	assert.True(t, ids[old], "заявка старше срока обязана попасть под обезличивание")
	assert.False(t, ids[fresh], "свежая заявка не трогается: срок не вышел")
	assert.False(t, ids[inWork], "заявка в работе не трогается никогда - по ней ещё ходят люди")
}

func TestApplicationRetention_SweepWipesAndDoesNotRepeat(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	now := time.Now().UTC()
	appID := retentionApplication(t, db, "просроченная", "Завершено", now.AddDate(-6, 0, 0))
	cutoff := now.AddDate(-5, 0, 0)
	recorder := services.NewAuditRecorder(db)

	// Без применения база не меняется.
	dry, err := entityarchive.SweepApplicationRetention(context.Background(), db, recorder, entityarchive.FilePaths{}, cutoff, 100, false)
	require.NoError(t, err)
	assert.Equal(t, 1, dry.Checked)
	assert.Equal(t, 0, dry.Applied)

	var last *string
	require.NoError(t, db.Raw(`SELECT last_name FROM employees
		WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)`, appID).Scan(&last).Error)
	require.NotNil(t, last)

	// С применением - обезличивает.
	applied, err := entityarchive.SweepApplicationRetention(context.Background(), db, recorder, entityarchive.FilePaths{}, cutoff, 100, true)
	require.NoError(t, err)
	assert.Equal(t, 1, applied.Applied)

	require.NoError(t, db.Raw(`SELECT last_name FROM employees
		WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)`, appID).Scan(&last).Error)
	assert.Nil(t, last, "ФИО участника обязано быть стёрто")

	var number *string
	require.NoError(t, db.Raw(`SELECT application_number FROM applications WHERE id = ?`, appID).Scan(&number).Error)
	require.NotNil(t, number, "сама заявка остаётся: по ней отвечают на запросы органов")

	// Повторный прогон не должен брать её снова: иначе каждый день копились бы
	// записи в истории об обезличивании одного и того же.
	again, err := entityarchive.SweepApplicationRetention(context.Background(), db, recorder, entityarchive.FilePaths{}, cutoff, 100, true)
	require.NoError(t, err)
	assert.Equal(t, 0, again.Checked, "уже обезличенная заявка повторно не берётся")
}
