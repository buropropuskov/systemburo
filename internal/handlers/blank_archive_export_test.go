package handlers_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Ядро выгрузки бланков в файловый архив (#1615, срез A5b): запись на диск, реестр,
// дедупликация по хэшу, переименование папки, заморозка.
//
// Секции живут на одном поднятом приложении: отдельный SetupTestApp на каждый кейс
// перебивал границу go test -timeout у пакета handlers.

type archiveWorld struct {
	e        *echo.Echo
	db       *gorm.DB
	root     string
	adminH   http.Header
	orgID    int
	senderID int
}

func setupArchiveWorld(t *testing.T) archiveWorld {
	t.Helper()
	e, db, root, cleanup := testutil.SetupTestAppWithArchive(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)
	senderID := secUserIDByUsername(t, db, "testadmin")

	// Рубильник выгрузки выключен по умолчанию - без него сервис отвечает отказом.
	testutil.SetArchiveSettings(t, db, models.UpdateArchiveSettingsRequest{Enabled: testutil.Ptr(true)})

	return archiveWorld{e: e, db: db, root: root, adminH: adminH, orgID: td.OrgID, senderID: senderID}
}

// newExportApp создаёт заявку с одним вложением заданного типа вложения.
func (w archiveWorld) newExportApp(t *testing.T, number string, uaID int, entryTo string) (appID, attID int) {
	t.Helper()
	now := time.Now()
	num, status, conf := number, models.StatusInWork, models.ConfirmationApproved
	app := models.Application{
		ApplicationNumber: &num,
		OrganizationID:    w.orgID,
		SenderUserID:      w.senderID,
		Status:            &status,
		Confirmation:      &conf,
		SendingDatetime:   &now,
	}
	require.NoError(t, w.db.Create(&app).Error)

	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", UniqueAttachmentID: &uaID}
	if entryTo != "" {
		att.EntryDateTo = &entryTo
	}
	require.NoError(t, w.db.Create(&att).Error)
	return app.ID, att.ID
}

// newExportType заводит тип вложения; withTemplate решает, есть ли у него бланк.
func (w archiveWorld) newExportType(t *testing.T, name string, withTemplate, autoExport bool) int {
	t.Helper()
	title := name
	ua := models.UniqueAttachment{
		AttachmentType: "people", Name: &title, DisplayName: &title,
		IsActive: true, AutoExport: autoExport,
	}
	require.NoError(t, w.db.Create(&ua).Error)
	// Выключенный тумблер дописываем отдельно: у колонки задан default:true, и gorm
	// выбрасывает поле из INSERT, когда значение нулевое.
	if !autoExport {
		require.NoError(t, w.db.Model(&models.UniqueAttachment{}).
			Where("id = ?", ua.ID).Update("auto_export", false).Error)
	}
	if withTemplate {
		blankSeedTemplate(t, w.db, ua.ID)
	}
	return ua.ID
}

func (w archiveWorld) reexport(t *testing.T, appID int) *models.BlankExportResult {
	t.Helper()
	rec := testutil.POST(t, w.e, fmt.Sprintf("/file-archive/applications/%d/reexport", appID), `{}`, w.adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	res := testutil.ParseResponse[models.BlankExportResult](t, rec)
	return &res
}

func (w archiveWorld) registryRow(t *testing.T, appID, attID int) models.BlankExport {
	t.Helper()
	var row models.BlankExport
	require.NoError(t, w.db.Where("application_id = ? AND attachment_id = ?", appID, attID).First(&row).Error)
	return row
}

// abs собирает путь на диске из относительного пути реестра.
func (w archiveWorld) abs(rel string) string {
	return filepath.Join(w.root, filepath.FromSlash(rel))
}

func TestFileArchiveExport(t *testing.T) {
	w := setupArchiveWorld(t)
	t.Run("запись файла и реестра", func(t *testing.T) { archiveWritesFileSection(t, w) })
	t.Run("повтор не трогает файл", func(t *testing.T) { archiveDedupSection(t, w) })
	t.Run("смена организации переносит папку", func(t *testing.T) { archiveRenameSection(t, w) })
	t.Run("тип без бланка", func(t *testing.T) { archiveNoTemplateSection(t, w) })
	t.Run("слепок заявки рядом с бланками", func(t *testing.T) { archiveSnapshotSection(t, w) })
	t.Run("слепок замороженной заявки", func(t *testing.T) { archiveSnapshotFrozenSection(t, w) })
	t.Run("слепок заявки без бланков", func(t *testing.T) { archiveSnapshotFrozenWithoutBlanksSection(t, w) })
	t.Run("выключенный тумблер типа", func(t *testing.T) { archiveSkippedSection(t, w) })
	t.Run("две заявки в одну папку", func(t *testing.T) { archiveDirCollisionSection(t, w) })
	t.Run("заморозка закрытой заявки", func(t *testing.T) { archiveFreezeSection(t, w) })
	t.Run("выключенная выгрузка", func(t *testing.T) { archiveDisabledSection(t, w) })
	t.Run("тумблер типа переживает создание", func(t *testing.T) { archiveTypeToggleSection(t, w) })
}

// Тип вложения, заведённый с выключенной выгрузкой, обязан таким и сохраниться:
// значение по умолчанию у колонки перебивало ответ формы, и администратор получал
// включённую выгрузку там, где явно её выключил.
func archiveTypeToggleSection(t *testing.T, w archiveWorld) {
	body := `{"attachment_type":"people","name":"toggle_off","display_name":"Без архива","title":"Без архива","auto_export":false}`
	rec := testutil.POST(t, w.e, "/attachments", body, w.adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var ua models.UniqueAttachment
	require.NoError(t, w.db.Where("name = ?", "toggle_off").First(&ua).Error)
	assert.False(t, ua.AutoExport, "выключенный тумблер не должен превращаться во включённый")
}

// Файл обязан оказаться на диске по тому же пути, который записан в реестре: именно
// по нему сверка и скачивание потом ищут бланк.
func archiveWritesFileSection(t *testing.T, w archiveWorld) {
	uaID := w.newExportType(t, "Пропуск на людей", true, true)
	appID, attID := w.newExportApp(t, "20260731/001", uaID, "")

	res := w.reexport(t, appID)
	require.Len(t, res.Items, 1)
	item := res.Items[0]
	require.Equal(t, models.BlankExportOK, item.Status, "ошибка выгрузки: %s", item.Error)
	assert.True(t, item.Written, "первый прогон обязан записать файл")

	info, err := os.Stat(w.abs(item.RelPath))
	require.NoError(t, err, "файла нет по пути из ответа: %s", item.RelPath)
	assert.Greater(t, info.Size(), int64(0))
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm(), "бланк содержит ПД: читать его может только владелец и группа")

	row := w.registryRow(t, appID, attID)
	assert.Equal(t, models.BlankExportOK, row.Status)
	assert.Equal(t, res.RelDir, row.RelDir)
	assert.Len(t, row.ContentHash, 64, "хэш содержимого - sha256 в hex")
	assert.Equal(t, info.Size(), row.SizeBytes)
	assert.NotNil(t, row.GeneratedAt)
	assert.Nil(t, row.FrozenAt, "заявка ещё действует - замораживать нечего")
	assert.Equal(t, filepath.Base(item.RelPath), row.FileName)
}

// Совпал хэш - файл не открывается вовсе. Проверяется по времени изменения: именно
// оно решает, потянет ли инкрементальная синхронизация файл на рабочий компьютер.
func archiveDedupSection(t *testing.T, w archiveWorld) {
	uaID := w.newExportType(t, "Пропуск дедуп", true, true)
	appID, attID := w.newExportApp(t, "20260731/002", uaID, "")

	first := w.reexport(t, appID)
	require.Equal(t, models.BlankExportOK, first.Items[0].Status, first.Items[0].Error)
	path := w.abs(first.Items[0].RelPath)

	// Отматываем время файла назад: совпадение mtime «секунда в секунду» у двух
	// подряд идущих прогонов ничего бы не доказывало.
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(path, old, old))
	before := w.registryRow(t, appID, attID)

	second := w.reexport(t, appID)
	require.Equal(t, models.BlankExportOK, second.Items[0].Status, second.Items[0].Error)
	assert.False(t, second.Items[0].Written, "содержимое не изменилось - переписывать нечего")

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.WithinDuration(t, old, info.ModTime(), time.Second, "файл трогать нельзя: mtime уводит синхронизацию в перекачку")

	after := w.registryRow(t, appID, attID)
	assert.Equal(t, before.ContentHash, after.ContentHash)
	assert.Equal(t, before.RelDir, after.RelDir)
}

// Организацию в заявке поправили - папка обязана переехать, а не раздвоиться.
func archiveRenameSection(t *testing.T, w archiveWorld) {
	uaID := w.newExportType(t, "Пропуск переезд", true, true)
	appID, attID := w.newExportApp(t, "20260731/003", uaID, "")

	first := w.reexport(t, appID)
	require.Equal(t, models.BlankExportOK, first.Items[0].Status, first.Items[0].Error)
	oldDir := w.abs(first.RelDir)
	require.DirExists(t, oldDir)

	other := models.Organization{Name: "Ромашка-Строй"}
	require.NoError(t, w.db.Create(&other).Error)
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", appID).
		Update("organization_id", other.ID).Error)

	second := w.reexport(t, appID)
	require.True(t, second.Renamed, "смена организации меняет имя папки")
	assert.NotEqual(t, first.RelDir, second.RelDir)
	assert.NoDirExists(t, oldDir, "прежняя папка обязана исчезнуть, иначе бланки лежат в двух местах")
	assert.FileExists(t, w.abs(second.Items[0].RelPath))

	row := w.registryRow(t, appID, attID)
	assert.Equal(t, second.RelDir, row.RelDir, "реестр обязан знать фактический путь")
}

// Тип вложения без настроенного бланка - видимый пробел архива, а не тишина.
func archiveNoTemplateSection(t *testing.T, w archiveWorld) {
	uaID := w.newExportType(t, "Пропуск без бланка", false, true)
	appID, attID := w.newExportApp(t, "20260731/004", uaID, "")

	res := w.reexport(t, appID)
	require.Len(t, res.Items, 1)
	assert.Equal(t, models.BlankExportNoTemplate, res.Items[0].Status)
	assert.False(t, res.Items[0].Written)

	row := w.registryRow(t, appID, attID)
	assert.Equal(t, models.BlankExportNoTemplate, row.Status)
	assert.Empty(t, row.RelDir, "файла нет - фактического пути тоже")
	assert.Nil(t, row.NextAttemptAt, "повторять нечего: нужен администратор, а не время")
}

// Выключенный тумблер типа выгрузку останавливает, но в ошибки не пишет.
func archiveSkippedSection(t *testing.T, w archiveWorld) {
	uaID := w.newExportType(t, "Пропуск выключен", true, false)
	appID, attID := w.newExportApp(t, "20260731/005", uaID, "")

	res := w.reexport(t, appID)
	require.Len(t, res.Items, 1)
	assert.Equal(t, models.BlankExportSkipped, res.Items[0].Status)

	row := w.registryRow(t, appID, attID)
	assert.Equal(t, models.BlankExportSkipped, row.Status)
	assert.Empty(t, row.LastError)
	assert.Nil(t, row.NextAttemptAt)
}

// Шаблон раскладки без номера складывает две заявки одного дня в общую папку -
// вторая обязана разойтись суффиксом, иначе её файлы затрут чужие.
func archiveDirCollisionSection(t *testing.T, w archiveWorld) {
	testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{DirTemplate: testutil.Ptr("{год}/{дата} {организация}")})
	t.Cleanup(func() {
		testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{DirTemplate: testutil.Ptr("{год}/{месяц_число} {МЕСЯЦ} {год}/{дата}/{дата} №{номер} {организация}")})
	})

	uaID := w.newExportType(t, "Пропуск коллизия", true, true)
	firstApp, _ := w.newExportApp(t, "20260731/006", uaID, "")
	secondApp, _ := w.newExportApp(t, "20260731/007", uaID, "")

	first := w.reexport(t, firstApp)
	require.Equal(t, models.BlankExportOK, first.Items[0].Status, first.Items[0].Error)
	second := w.reexport(t, secondApp)
	require.Equal(t, models.BlankExportOK, second.Items[0].Status, second.Items[0].Error)

	assert.NotEqual(t, first.RelDir, second.RelDir, "две заявки не могут делить одну папку")
	assert.Contains(t, second.RelDir, fmt.Sprintf("(№%d)", secondApp), "суффикс детерминированный - по идентификатору заявки")
	assert.FileExists(t, w.abs(first.Items[0].RelPath))
	assert.FileExists(t, w.abs(second.Items[0].RelPath))
}

// Закрытая заявка со сроком в прошлом замораживается: файл становится документом и
// больше не следует за правками, а папка не переезжает.
func archiveFreezeSection(t *testing.T, w archiveWorld) {
	testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{FreezeAfterDays: testutil.Ptr(0)})
	t.Cleanup(func() {
		testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{FreezeAfterDays: testutil.Ptr(30)})
	})

	uaID := w.newExportType(t, "Пропуск заморозка", true, true)
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	appID, attID := w.newExportApp(t, "20260731/008", uaID, yesterday)
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", appID).
		Update("status", models.StatusCompleted).Error)

	first := w.reexport(t, appID)
	require.Equal(t, models.BlankExportOK, first.Items[0].Status, first.Items[0].Error)
	assert.True(t, first.Items[0].Frozen, "срок вышел - файл окончателен")
	frozenRow := w.registryRow(t, appID, attID)
	require.NotNil(t, frozenRow.FrozenAt)
	path := w.abs(first.Items[0].RelPath)

	old := time.Now().Add(-3 * time.Hour)
	require.NoError(t, os.Chtimes(path, old, old))

	// Правка, которая у живой заявки переписала бы файл и переименовала папку.
	other := models.Organization{Name: "Незабудка"}
	require.NoError(t, w.db.Create(&other).Error)
	require.NoError(t, w.db.Model(&models.Application{}).Where("id = ?", appID).
		Update("organization_id", other.ID).Error)

	second := w.reexport(t, appID)
	assert.False(t, second.Renamed, "замороженная заявка не переезжает: путь уже уехал в корпоративную копию")
	assert.True(t, second.Items[0].Frozen)
	assert.False(t, second.Items[0].Written)
	assert.Equal(t, first.RelDir, second.RelDir)

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.WithinDuration(t, old, info.ModTime(), time.Second, "замороженный файл не перезаписывается")
	after := w.registryRow(t, appID, attID)
	assert.Equal(t, frozenRow.ContentHash, after.ContentHash)
	assert.Equal(t, frozenRow.FrozenAt.Unix(), after.FrozenAt.Unix())

	// Новое вложение того же типа даёт то же имя файла. Подвинуться обязано оно:
	// замороженный файл лежит на диске под своим именем и переименован не будет.
	extra := models.Attachment{ApplicationID: &appID, AttachmentType: "people", UniqueAttachmentID: &uaID}
	require.NoError(t, w.db.Create(&extra).Error)

	third := w.reexport(t, appID)
	require.Len(t, third.Items, 2)
	fresh := third.Items[1]
	require.Equal(t, models.BlankExportOK, fresh.Status, fresh.Error)
	assert.Contains(t, fresh.RelPath, fmt.Sprintf("(№%d)", extra.ID), "новый файл разводится суффиксом вложения")
	assert.FileExists(t, w.abs(fresh.RelPath))

	kept, err := os.Stat(path)
	require.NoError(t, err)
	assert.WithinDuration(t, old, kept.ModTime(), time.Second, "замороженный файл не должен быть перезаписан соседом")
}

// Выключенная выгрузка отвечает причиной, а не пустым результатом: администратор,
// нажавший «пересоздать», должен понять, почему ничего не произошло.
func archiveDisabledSection(t *testing.T, w archiveWorld) {
	testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{Enabled: testutil.Ptr(false)})
	t.Cleanup(func() {
		testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{Enabled: testutil.Ptr(true)})
	})

	uaID := w.newExportType(t, "Пропуск выключенный архив", true, true)
	appID, _ := w.newExportApp(t, "20260731/009", uaID, "")

	res := testutil.POST(t, w.e, fmt.Sprintf("/file-archive/applications/%d/reexport", appID), `{}`, w.adminH)
	require.Equal(t, http.StatusConflict, res.Code, res.Body.String())

	var count int64
	require.NoError(t, w.db.Model(&models.BlankExport{}).Where("application_id = ?", appID).Count(&count).Error)
	assert.Zero(t, count, "выключенная выгрузка не должна заводить строк реестра")
}
