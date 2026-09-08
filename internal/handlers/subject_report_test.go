package handlers_test

// Справка о человеке (server subject export). Её прикладывают к официальному ответу
// государственному органу, поэтому проверяется не «функция отработала», а содержимое:
// попал ли в неё проход с постом, расшифрованы ли документы, не втянулся ли однофамилец.

import (
	"context"
	"os"
	"testing"
	"time"

	"systemburo/internal/crypto"
	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reportSection достаёт раздел справки по заголовку.
func reportSection(t *testing.T, rep entityarchive.SubjectReport, title string) []([]string) {
	t.Helper()
	for _, s := range rep.Sections {
		if s.Title == title {
			return s.Rows
		}
	}
	t.Fatalf("в справке нет раздела %q", title)
	return nil
}

func TestBuildSubjectReport_CollectsSections(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Справка-эксперимент"}
	require.NoError(t, db.Create(&org).Error)
	user := models.User{Username: "report-owner", OrganizationID: &org.ID}
	require.NoError(t, db.Create(&user).Error)
	number := "№ 20260908/001"
	status := "Согласовано"
	app := models.Application{OrganizationID: org.ID, SenderUserID: user.ID,
		ApplicationNumber: &number, Status: &status}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, OrganizationID: &org.ID}
	require.NoError(t, db.Create(&att).Error)

	post := models.SystemTable{Name: "КПП-1"}
	require.NoError(t, db.Create(&post).Error)

	last, first, position := "Справкин", "Олег", "Монтажник"
	passport := "4510 424242"
	employee := models.Employee{
		AttachmentID: &att.ID, LastName: &last, FirstName: &first, Position: &position,
		PassportSeriesNumber: &passport,
	}
	require.NoError(t, db.Create(&employee).Error)
	require.NoError(t, db.Create(&models.UniqueEmployee{
		LastName: &last, FirstName: &first, Position: &position,
		PassportSeriesNumber: &passport, OrganizationID: &org.ID,
	}).Error)
	require.NoError(t, db.Exec(`INSERT INTO employee_target_tables (employee_id, table_id) VALUES (?, ?)`,
		employee.ID, post.ID).Error)

	// Отметка прохода: именно она отвечает на вопрос «когда приходил и куда».
	require.NoError(t, db.Exec(`
		INSERT INTO audit_log (entity_type, entity_id, action, actor_user_id, details, created_at)
		VALUES ('employee', ?, 'entry', ?, ?::jsonb, ?)`,
		employee.ID, user.ID, `{"table_id": `+itoa(post.ID)+`}`, time.Now().UTC()).Error)

	// Однофамилец с другим документом - в справку попасть не должен.
	otherPassport := "4510 999999"
	require.NoError(t, db.Create(&models.Employee{
		AttachmentID: &att.ID, LastName: &last, FirstName: &first,
		PassportSeriesNumber: &otherPassport,
	}).Error)

	target := entityarchive.SubjectTargetFromDocuments(passport, "")
	rep, err := entityarchive.BuildSubjectReport(context.Background(), db, target)
	require.NoError(t, err)

	t.Run("сведения: обе записи человека и ни одной чужой", func(t *testing.T) {
		rows := reportSection(t, rep, "Сведения")
		require.Len(t, rows, 2, "запись реестра и участие в заявке: однофамилец не в счёт")
		for _, r := range rows {
			assert.Contains(t, r[2], "Справкин")
			assert.Equal(t, passport, r[6], "паспорт в справке обязан быть читаемым")
		}
	})

	t.Run("заявки: номер и статус", func(t *testing.T) {
		rows := reportSection(t, rep, "Заявки")
		require.Len(t, rows, 1)
		assert.Equal(t, number, rows[0][0])
		assert.Equal(t, status, rows[0][1])
	})

	t.Run("проходы: событие и пост", func(t *testing.T) {
		rows := reportSection(t, rep, "Проходы")
		require.Len(t, rows, 1, "отметка прохода - главное, ради чего справку и составляют")
		assert.Equal(t, "вход на территорию", rows[0][1])
		assert.Equal(t, "КПП-1", rows[0][2], "без поста ответ «когда приходил» неполон")
	})

	t.Run("посты: действующая привязка", func(t *testing.T) {
		rows := reportSection(t, rep, "Посты")
		require.Len(t, rows, 1)
		assert.Equal(t, "КПП-1", rows[0][0])
	})

	t.Run("основание обработки в каждом разделе", func(t *testing.T) {
		for _, s := range rep.Sections {
			assert.Contains(t, s.Subtitle, "п. 7 ч. 1 ст. 6 152-ФЗ",
				"раздел %q без основания обработки: получатель справки не поймёт, на чём она стоит", s.Title)
		}
	})
}

// TestBuildSubjectReport_DecryptsDocuments - документы лежат в базе шифротекстом, а
// справку читает человек. Забытая расшифровка даёт файл с набором символов вместо
// паспорта - ровно то, что уже случилось с «иным разрешением» (#2413).
func TestBuildSubjectReport_DecryptsDocuments(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Шифр-эксперимент"}
	require.NoError(t, db.Create(&org).Error)

	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 29)
	}
	crypto.SetGlobalKey(key)
	t.Cleanup(func() { crypto.SetGlobalKey(nil) })

	last, first := "Шифров", "Иван"
	passport := "4510 314159"
	permission := "Разрешение на временное проживание"
	require.NoError(t, db.Create(&models.UniqueEmployee{
		LastName: &last, FirstName: &first,
		PassportSeriesNumber: &passport, OtherPermission: &permission,
		OrganizationID: &org.ID,
	}).Error)

	target := entityarchive.SubjectTargetFromDocuments(passport, "")
	rep, err := entityarchive.BuildSubjectReport(context.Background(), db, target)
	require.NoError(t, err)

	rows := reportSection(t, rep, "Сведения")
	require.Len(t, rows, 1)
	assert.Equal(t, passport, rows[0][6], "паспорт в справке обязан быть расшифрован")
	assert.Equal(t, permission, rows[0][8], "иное разрешение тоже")
}

// TestWriteSubjectReport_WritesBothFormats - справка кладётся двумя файлами: .xlsx для
// работы и .pdf для приложения к официальному ответу.
func TestWriteSubjectReport_WritesBothFormats(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	org := models.Organization{Name: "Файлы-эксперимент"}
	require.NoError(t, db.Create(&org).Error)
	last, first := "Файлов", "Пётр"
	passport := "4510 271828"
	require.NoError(t, db.Create(&models.UniqueEmployee{
		LastName: &last, FirstName: &first, PassportSeriesNumber: &passport,
		OrganizationID: &org.ID,
	}).Error)

	rep, err := entityarchive.BuildSubjectReport(context.Background(),
		db, entityarchive.SubjectTargetFromDocuments(passport, ""))
	require.NoError(t, err)

	written, err := entityarchive.WriteSubjectReport(t.TempDir(), rep)
	require.NoError(t, err)
	require.Len(t, written, 2)
	assert.Contains(t, written[0], ".xlsx")
	assert.Contains(t, written[1], ".pdf")
	for _, f := range written {
		fi, err := os.Stat(f)
		require.NoError(t, err)
		assert.Greater(t, fi.Size(), int64(0), "файл %s пуст", f)
	}
}
