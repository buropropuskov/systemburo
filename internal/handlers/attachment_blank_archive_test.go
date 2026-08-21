package handlers_test

import (
	"fmt"
	"net/http"
	"net/url"
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

// Источник «сохранённый файл» у скачивания бланка (?source=archive, #1615 C6).
// Ручка отдаёт файл прямо с диска, и весь гейт держится на том, что строка реестра
// ищется строго по паре application_id+attachment_id: потеряй выборка половину пары -
// участник своей заявки заберёт бланк чужой, уже не спрашивая ни у кого доступа.
const archiveSourcePassword = "archive_source_pass_long_enough"

// blankArchiveType заводит тип вложения с настроенным бланком и включённой выгрузкой.
func blankArchiveType(t *testing.T, db *gorm.DB, name string) int {
	t.Helper()
	nm := name
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &nm, IsActive: true, AutoExport: true}
	require.NoError(t, db.Create(&ua).Error)
	blankSeedTemplate(t, db, ua.ID)
	return ua.ID
}

// blankArchiveApp создаёт заявку отправителя с одним people-вложением типа uaID.
func blankArchiveApp(t *testing.T, db *gorm.DB, orgID, senderID, uaID int, number string) (int, int) {
	t.Helper()
	now := time.Now()
	num, status, conf := number, models.StatusInWork, models.ConfirmationApproved
	app := models.Application{
		ApplicationNumber: &num, OrganizationID: orgID, SenderUserID: senderID,
		Status: &status, Confirmation: &conf, SendingDatetime: &now,
	}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", UniqueAttachmentID: &uaID}
	require.NoError(t, db.Create(&att).Error)
	return app.ID, att.ID
}

// blankArchiveExport гонит выгрузку заявки в архив и возвращает строку реестра вложения.
func blankArchiveExport(t *testing.T, e *echo.Echo, db *gorm.DB, adminH http.Header, appID, attID int) models.BlankExport {
	t.Helper()
	rec := testutil.POST(t, e, fmt.Sprintf("/file-archive/applications/%d/reexport", appID), `{}`, adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	res := testutil.ParseResponse[models.BlankExportResult](t, rec)
	require.NotEmpty(t, res.Items)
	require.Equal(t, models.BlankExportOK, res.Items[0].Status, res.Items[0].Error)

	var row models.BlankExport
	require.NoError(t, db.Where("application_id = ? AND attachment_id = ?", appID, attID).First(&row).Error)
	require.NotEmpty(t, row.FileName)
	return row
}

// blankArchiveURL - адрес скачивания бланка вложения; source пустой означает прежнюю
// генерацию на лету.
func blankArchiveURL(appID, attID int, source string) string {
	u := fmt.Sprintf("/applications/%d/blank?attachment_id=%d", appID, attID)
	if source != "" {
		u += "&source=" + source
	}
	return u
}

func TestAttachmentBlankArchiveSource(t *testing.T) {
	e, db, root, cleanup := testutil.SetupTestAppWithArchive(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))
	// Выгрузку включаем сервисом: правка настроек ушла из веба в команду
	// server archive, роута записи больше нет (#1615).
	testutil.ArchiveEnabled(t, db, true)

	userTypeID := secUserTypeIDByCode(t, db, "user")
	senderH := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "blankarchsender", archiveSourcePassword, userTypeID, td.OrgID, td.CompanyID))
	senderID := secUserIDByUsername(t, db, "blankarchsender")
	outsiderH := testutil.AuthHeader(testutil.RegisterAndLogin(t, e, "blankarchoutsider", archiveSourcePassword, userTypeID, td.OrgID, td.CompanyID))
	testutil.RegisterAndLogin(t, e, "blankarchforeign", archiveSourcePassword, userTypeID, td.OrgID, td.CompanyID)
	foreignID := secUserIDByUsername(t, db, "blankarchforeign")

	uaID := blankArchiveType(t, db, "blank_archive_source")
	appID, attID := blankArchiveApp(t, db, td.OrgID, senderID, uaID, "20260801/601")
	foreignAppID, foreignAttID := blankArchiveApp(t, db, td.OrgID, foreignID, uaID, "20260801/602")

	row := blankArchiveExport(t, e, db, adminH, appID, attID)
	foreignRow := blankArchiveExport(t, e, db, adminH, foreignAppID, foreignAttID)

	// Метки вместо настоящих книг: только по содержимому и видно, что ручка отдала
	// файл из реестра, а не сгенерировала бланк заново - оба ответа иначе выглядят
	// одинаковым валидным .xlsx.
	marker, foreignMarker := []byte("ARCHIVED-BLANK-OWN"), []byte("ARCHIVED-BLANK-FOREIGN")
	path := filepath.Join(root, filepath.FromSlash(row.RelDir), row.FileName)
	foreignPath := filepath.Join(root, filepath.FromSlash(foreignRow.RelDir), foreignRow.FileName)
	require.NoError(t, os.WriteFile(path, marker, 0o640))
	require.NoError(t, os.WriteFile(foreignPath, foreignMarker, 0o640))

	// Сохранённая копия собрана с документами участников, и открыта она двоим:
	// инициатору заявки (документы он сам и вводил) и носителю права на выгрузку.
	t.Run("инициатор заявки получает сохранённый файл", func(t *testing.T) {
		rec := testutil.GET(t, e, blankArchiveURL(appID, attID, "archive"), senderH)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, string(marker), rec.Body.String())
	})

	t.Run("отдаётся файл из реестра под своим именем", func(t *testing.T) {
		rec := testutil.GET(t, e, blankArchiveURL(appID, attID, "archive"), adminH)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, string(marker), rec.Body.String(),
			"ручка обязана отдать сохранённый файл, а не собрать бланк заново")
		assert.Contains(t, rec.Header().Get("Content-Disposition"), url.PathEscape(row.FileName),
			"имя в заголовке - то же, под которым файл лежит в архиве")
		assert.Contains(t, rec.Header().Get("Content-Type"), "spreadsheetml.sheet")
	})

	t.Run("без source бланк по-прежнему собирается заново", func(t *testing.T) {
		rec := testutil.GET(t, e, blankArchiveURL(appID, attID, ""), senderH)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.NotEqual(t, string(marker), rec.Body.String(), "прежний путь не должен читать диск")
		assert.Equal(t, "PK", rec.Body.String()[:2], "живой источник отдаёт свежесобранную книгу")
	})

	t.Run("вложение чужой заявки не отдаётся", func(t *testing.T) {
		rec := testutil.GET(t, e, blankArchiveURL(appID, foreignAttID, "archive"), senderH)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), string(foreignMarker),
			"строка реестра ищется по паре: чужое вложение не должно находиться по своей заявке")
	})

	t.Run("нет строки реестра - 404", func(t *testing.T) {
		fresh := models.Attachment{ApplicationID: &appID, AttachmentType: "people", UniqueAttachmentID: &uaID}
		require.NoError(t, db.Create(&fresh).Error)

		rec := testutil.GET(t, e, blankArchiveURL(appID, fresh.ID, "archive"), senderH)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "не найден", "404 обязан объяснить себя, а не быть пустым")
	})

	t.Run("файл ещё не выгружен - 404", func(t *testing.T) {
		queued := models.Attachment{ApplicationID: &appID, AttachmentType: "people", UniqueAttachmentID: &uaID}
		require.NoError(t, db.Create(&queued).Error)
		// На диске рядом кладём заготовку под именем строки очереди: потеряйся
		// проверка статуса, ответом станет она, а не 404 - иначе кейс был бы
		// зелёным просто потому, что читать нечего.
		stale := []byte("ARCHIVED-BLANK-STALE")
		staleName := "queued_" + row.FileName
		require.NoError(t, os.WriteFile(filepath.Join(root, filepath.FromSlash(row.RelDir), staleName), stale, 0o640))
		require.NoError(t, db.Create(&models.BlankExport{
			ApplicationID: appID, AttachmentID: queued.ID, BucketDate: row.BucketDate,
			RelDir: row.RelDir, FileName: staleName,
			Status: models.BlankExportPending, QueuedAt: time.Now(),
		}).Error)

		rec := testutil.GET(t, e, blankArchiveURL(appID, queued.ID, "archive"), senderH)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), string(stale), "выгрузки ещё не было - отдавать нечего")
	})

	t.Run("посторонний не скачивает и сохранённый файл", func(t *testing.T) {
		archived := testutil.GET(t, e, blankArchiveURL(appID, attID, "archive"), outsiderH)
		require.Equal(t, http.StatusForbidden, archived.Code, archived.Body.String())
		assert.NotContains(t, archived.Body.String(), string(marker))

		live := testutil.GET(t, e, blankArchiveURL(appID, attID, ""), outsiderH)
		require.Equal(t, http.StatusForbidden, live.Code, live.Body.String(),
			"гейт у источников общий: отказ не должен зависеть от source")
	})
}
