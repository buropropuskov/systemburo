package handlers_test

// Обезличивание одного человека (server subject anonymize).
//
// Операция необратима, поэтому проверяется не «функция отработала», а границы: что
// затёрто, что осталось нетронутым и что человек после этого действительно перестал
// находиться - последнее и есть смысл операции, а не побочный эффект.

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

func anonymizeSubjectFixture(t *testing.T, db *gorm.DB) (passport string, otherID int) {
	t.Helper()

	org := models.Organization{Name: "Обезличивание-эксперимент"}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "anon-owner", OrganizationID: &org.ID}
	require.NoError(t, db.Create(&user).Error)
	app := models.Application{OrganizationID: org.ID, SenderUserID: user.ID}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, OrganizationID: &org.ID}
	require.NoError(t, db.Create(&att).Error)

	passport = "4510 505050"
	last, first := "Стираев", "Артём"
	require.NoError(t, db.Create(&models.UniqueEmployee{
		LastName: &last, FirstName: &first, PassportSeriesNumber: &passport,
		OrganizationID: &org.ID,
	}).Error)
	require.NoError(t, db.Create(&models.Employee{
		AttachmentID: &att.ID, LastName: &last, FirstName: &first,
		PassportSeriesNumber: &passport,
	}).Error)

	// Однофамилец с другим документом - его команда трогать не должна.
	otherPassport := "4510 606060"
	other := models.Employee{
		AttachmentID: &att.ID, LastName: &last, FirstName: &first,
		PassportSeriesNumber: &otherPassport,
	}
	require.NoError(t, db.Create(&other).Error)
	return passport, other.ID
}

func TestAnonymizeSubject_DryRunChangesNothing(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	passport, _ := anonymizeSubjectFixture(t, db)

	target := entityarchive.SubjectTargetFromDocuments(passport, "")
	res, err := entityarchive.AnonymizeSubject(context.Background(), db,
		services.NewAuditRecorder(db), target, entityarchive.DestructionOptions{Basis: entityarchive.BasisOperator})
	require.NoError(t, err)
	assert.Equal(t, 2, res.Total(), "две строки: запись реестра и участник заявки")
	assert.NotEmpty(t, res.Warnings, "оператор обязан узнать, что остаётся после затирания")

	var stillThere int64
	require.NoError(t, db.Model(&models.UniqueEmployee{}).
		Where("last_name = ?", "Стираев").Count(&stillThere).Error)
	assert.Equal(t, int64(1), stillThere, "без -apply база не меняется")
}

func TestAnonymizeSubject_WipesNameAndDocumentWithHash(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	passport, otherID := anonymizeSubjectFixture(t, db)

	target := entityarchive.SubjectTargetFromDocuments(passport, "")
	res, err := entityarchive.AnonymizeSubject(context.Background(), db,
		services.NewAuditRecorder(db), target, entityarchive.DestructionOptions{Basis: entityarchive.BasisOperator, Apply: true})
	require.NoError(t, err)
	require.Equal(t, 2, res.Total())

	t.Run("имя и документ стёрты", func(t *testing.T) {
		var row models.UniqueEmployee
		require.NoError(t, db.Where("organization_id IS NOT NULL").First(&row).Error)
		assert.Nil(t, row.LastName)
		assert.Nil(t, row.PassportSeriesNumber)
	})

	t.Run("отпечаток стёрт вместе со значением", func(t *testing.T) {
		// Свёртка детерминирована: оставив её, можно проверить гипотезу «это паспорт
		// такого-то» даже после затирания значения - обезличиванием это не является.
		var hashes int64
		require.NoError(t, db.Model(&models.UniqueEmployee{}).
			Where("passport_series_number_hmac IS NOT NULL AND passport_series_number_hmac <> ''").
			Count(&hashes).Error)
		assert.Equal(t, int64(0), hashes)
	})

	t.Run("однофамилец с другим документом не тронут", func(t *testing.T) {
		var other models.Employee
		require.NoError(t, db.First(&other, otherID).Error)
		require.NotNil(t, other.LastName)
		assert.Equal(t, "Стираев", *other.LastName)
	})

	t.Run("человек больше не находится по документу", func(t *testing.T) {
		graph, err := entityarchive.CollectSubject(context.Background(), db, target)
		require.NoError(t, err)
		assert.Equal(t, int64(0), graph.Total(),
			"свёртка стёрта вместе со значением - собрать сведения больше не по чему")
	})

	t.Run("обезличивание попало в историю", func(t *testing.T) {
		var logged int64
		require.NoError(t, db.Table("audit_log").
			Where("entity_type = ? AND action = ?", "unique_employee", "anonymized").
			Count(&logged).Error)
		assert.Equal(t, int64(1), logged, "необратимая операция без следа в истории не считается выполненной")
	})
}

// TestAnonymizeSubject_NotFound - под цель не подошло ни одной строки: команда обязана
// сказать об этом, а не отчитаться об успешном затирании нуля записей.
func TestAnonymizeSubject_NotFound(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	target := entityarchive.SubjectTargetFromDocuments("0000 000000", "")
	_, err := entityarchive.AnonymizeSubject(context.Background(), db,
		services.NewAuditRecorder(db), target, entityarchive.DestructionOptions{Basis: entityarchive.BasisOperator})
	require.Error(t, err)
}
