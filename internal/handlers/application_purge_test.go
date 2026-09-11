package handlers_test

// Уничтожение ошибочно заведённой заявки (#2355, пункт 5).
//
// Это редкий ручной инструмент: по истечении срока хранения заявки обезличиваются, а
// здесь запись исчезает целиком - её не должно было существовать вовсе. Тест стережёт
// три вещи: не остаётся ни строк, ни файлов, ни истории с ФИО; соседняя заявка цела;
// перечень таблиц графа полон - таблица, добавленная позже, роняет замок, а не тихо
// переживает уничтожение.

import (
	"context"
	"path"
	"testing"

	"systemburo/internal/entityarchive"
	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPurgeApplication_RemovesEverything(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	appID, otherAppID := applicationWithPeople(t, db, "№ 20260101/500")

	recorder := services.NewAuditRecorder(db)
	paths := entityarchive.FilePaths{}

	dry, err := entityarchive.PurgeApplication(context.Background(), db, recorder, paths, appID, nil, false)
	require.NoError(t, err)
	assert.Equal(t, "№ 20260101/500", dry.Number)
	assert.NotEmpty(t, dry.Warnings, "оператор обязан прочитать, что откат не предусмотрен")
	assert.Positive(t, dry.History, "история заявки и её участников входит в объём")

	var stillThere int64
	require.NoError(t, db.Raw(`SELECT count(*) FROM applications WHERE id = ?`, appID).Scan(&stillThere).Error)
	require.EqualValues(t, 1, stillThere, "показ без -apply базу не меняет")

	out, err := entityarchive.PurgeApplication(context.Background(), db, recorder, paths, appID, nil, true)
	require.NoError(t, err)
	assert.Positive(t, out.TotalRows())

	t.Run("заявки и её строк не осталось", func(t *testing.T) {
		var apps, atts, people int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM applications WHERE id = ?`, appID).Scan(&apps).Error)
		require.NoError(t, db.Raw(`SELECT count(*) FROM attachments WHERE application_id = ?`, appID).Scan(&atts).Error)
		require.NoError(t, db.Raw(`SELECT count(*) FROM employees e
			JOIN attachments att ON att.id = e.attachment_id WHERE att.application_id = ?`, appID).Scan(&people).Error)
		assert.Zero(t, apps)
		assert.Zero(t, atts)
		assert.Zero(t, people)
	})

	t.Run("история с ФИО ушла вместе с заявкой", func(t *testing.T) {
		// В пояснениях записей стоят имена словами («Сотрудник Хранимов Олег прошёл»):
		// уничтожение, оставляющее историю, оставляло бы ровно то, ради чего затевалось.
		var withName int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM audit_log
			WHERE details::text LIKE '%Хранимов%'`).Scan(&withName).Error)
		assert.Zero(t, withName)
	})

	t.Run("запись об уничтожении осталась", func(t *testing.T) {
		var n int64
		require.NoError(t, db.Raw(`SELECT count(*) FROM audit_log
			WHERE entity_type = 'application' AND entity_id = ? AND action = ?`,
			appID, models.OrganizationActionPurged).Scan(&n).Error)
		assert.EqualValues(t, 1, n, "журнал - единственное, что остаётся от заявки")

		var details string
		require.NoError(t, db.Raw(`SELECT details::text FROM audit_log
			WHERE entity_type = 'application' AND entity_id = ? AND action = ?`,
			appID, models.OrganizationActionPurged).Scan(&details).Error)
		assert.Contains(t, details, "20260101/500", "номер опознаёт уничтожение при разборе")
		assert.NotContains(t, details, "Хранимов", "в журнале уничтожения имён быть не должно")
	})

	t.Run("соседняя заявка цела", func(t *testing.T) {
		var last *string
		require.NoError(t, db.Raw(`SELECT last_name FROM employees
			WHERE attachment_id IN (SELECT id FROM attachments WHERE application_id = ?)`, otherAppID).
			Scan(&last).Error)
		require.NotNil(t, last)
		assert.Equal(t, "Соседов", *last)
	})
}

// Уничтоженная заявка не должна оставлять в реестре архива строк, указывающих на
// запись, которой больше нет: обезличивание их помечает и хранит, а здесь исчезает
// сама заявка, и указывать строке не на что.
func TestPurgeApplication_TakesArchiveRowsWithIt(t *testing.T) {
	w := setupArchiveWorld(t)
	uaID := w.newExportType(t, "Пропуск уничтожение целиком", false, true)
	appID, attID := w.newExportApp(t, "20260731/902", uaID, "")

	passport := "45 03 555666"
	require.NoError(t, w.db.Create(&models.Employee{
		AttachmentID: &attID, LastName: snapStrPtr("Ошибкин"), FirstName: snapStrPtr("Пётр"),
		PassportSeriesNumber: &passport,
	}).Error)

	res := w.reexport(t, appID)
	require.Equal(t, models.BlankExportOK, res.Snapshot.Status, res.Snapshot.Error)
	snapPath := w.abs(path.Join(res.RelDir, archiveSnapshotFileName))
	require.FileExists(t, snapPath)

	_, err := entityarchive.PurgeApplication(context.Background(), w.db,
		services.NewAuditRecorder(w.db), entityarchive.FilePaths{ArchivePath: w.root},
		appID, nil, true)
	require.NoError(t, err)

	assert.NoFileExists(t, snapPath, "слепок с паспортом остался на диске")
	var rows int64
	require.NoError(t, w.db.Model(&models.BlankExport{}).
		Where("application_id = ?", appID).Count(&rows).Error)
	assert.Zero(t, rows, "реестр указывает на заявку, которой больше нет")
}

func TestPurgeApplication_NotFound(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)

	_, err := entityarchive.PurgeApplication(context.Background(), db,
		services.NewAuditRecorder(db), entityarchive.FilePaths{}, 999999, nil, true)
	require.Error(t, err)
}

// Перечень таблиц графа заявки обязан покрывать всё, что на неё ссылается. Новая
// таблица с application_id или attachment_id появляется вместе с новой возможностью, и
// без этого замка она молча пережила бы уничтожение заявки - вместе со сведениями о
// людях, ради которых уничтожение и делалось.
func TestApplicationGraph_NoUnaccountedTables(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()

	known := map[string]bool{}
	for _, table := range entityarchive.ApplicationGraphTables() {
		known[table] = true
	}
	// Сама заявка и журнал уносятся отдельными шагами, а не узлами графа.
	known["applications"] = true
	known["audit_log"] = true

	var tables []string
	require.NoError(t, db.Raw(`
		SELECT DISTINCT c.table_name FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.table_schema = 'public' AND t.table_type = 'BASE TABLE'
		  AND c.column_name IN ('application_id', 'attachment_id')
		ORDER BY 1`).Scan(&tables).Error)
	require.NotEmpty(t, tables)

	var missed []string
	for _, table := range tables {
		if !known[table] {
			missed = append(missed, table)
		}
	}
	assert.Empty(t, missed,
		"таблицы ссылаются на заявку или вложение, но в графе уничтожения их нет: %v", missed)
}
