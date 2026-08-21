package handlers_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Скачивание из файлового архива бланков (#1615, срез B3): потоковый ZIP за период
// через одноразовый билет и ZIP сохранённых файлов заявки. Реюз archiveWorld/
// setupArchiveWorld (blank_archive_export_test.go) - тот же пакет handlers_test.

const archiveDownloadPassword = "archive_download_pass_long_enough"

// Секции живут на одном поднятом приложении - см. правило в archiveWorld.
func TestFileArchiveDownload_Period(t *testing.T) {
	w := setupArchiveWorld(t)

	uaID := w.newExportType(t, "Пропуск на людей", true, true)
	appID, attID := w.newExportApp(t, "20260731/501", uaID, "")
	res := w.reexport(t, appID)
	require.Len(t, res.Items, 1)
	require.Equal(t, models.BlankExportOK, res.Items[0].Status, res.Items[0].Error)

	row := w.registryRow(t, appID, attID)
	require.NotEmpty(t, row.FileName)
	today := row.BucketDate.Format("2006-01-02")

	t.Run("оценка отдаёт число файлов и объём", func(t *testing.T) {
		rec := testutil.POST(t, w.e, "/file-archive/estimate",
			fmt.Sprintf(`{"date_from":%q,"date_to":%q}`, today, today), w.adminH)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		got := testutil.ParseResponse[models.ArchiveDownloadEstimate](t, rec)
		assert.GreaterOrEqual(t, got.FileCount, int64(1))
		assert.Greater(t, got.Bytes, int64(0))
		assert.False(t, got.ExceedsLimit)
	})

	t.Run("date_to раньше date_from отклоняется", func(t *testing.T) {
		rec := testutil.POST(t, w.e, "/file-archive/estimate",
			fmt.Sprintf(`{"date_from":%q,"date_to":"2020-01-01"}`, today), w.adminH)
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("билет отдаёт ZIP с уцелевшей кириллицей", func(t *testing.T) {
		ticketRec := testutil.POST(t, w.e, "/file-archive/download-ticket",
			fmt.Sprintf(`{"date_from":%q,"date_to":%q}`, today, today), w.adminH)
		require.Equal(t, http.StatusOK, ticketRec.Code, ticketRec.Body.String())
		ticket := testutil.ParseResponse[models.ArchiveDownloadTicketResponse](t, ticketRec)
		require.NotEmpty(t, ticket.Ticket)

		// Публичный роут - без Authorization, билет и есть авторизация.
		zipRec := testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket.Ticket, nil)
		require.Equal(t, http.StatusOK, zipRec.Code, zipRec.Body.String())
		assert.Equal(t, "application/zip", zipRec.Header().Get("Content-Type"))

		body := zipRec.Body.Bytes()
		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		require.NoError(t, err)
		// Бланк вложения плюс машиночитаемый слепок заявки (заявка.json) - ExportApplication
		// пишет оба, и период должен забрать оба.
		require.Len(t, zr.File, 2)

		var blank *zip.File
		for i, f := range zr.File {
			if f.Name == row.RelDir+"/"+row.FileName {
				blank = zr.File[i]
			}
		}
		require.NotNil(t, blank, "бланк по пути из реестра не найден в архиве: %+v", zr.File)
		assert.True(t, strings.Contains(blank.Name, "Пропуск") || strings.ContainsAny(blank.Name, "а-яё"),
			"имя файла из шаблона раскладки должно нести кириллицу без искажений: %q", blank.Name)

		rc, err := blank.Open()
		require.NoError(t, err)
		defer rc.Close()
		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		assert.NotEmpty(t, content, "запись не должна оказаться пустой заглушкой об ошибке")
	})

	t.Run("билет одноразовый", func(t *testing.T) {
		ticketRec := testutil.POST(t, w.e, "/file-archive/download-ticket",
			fmt.Sprintf(`{"date_from":%q,"date_to":%q}`, today, today), w.adminH)
		require.Equal(t, http.StatusOK, ticketRec.Code)
		ticket := testutil.ParseResponse[models.ArchiveDownloadTicketResponse](t, ticketRec)

		first := testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket.Ticket, nil)
		require.Equal(t, http.StatusOK, first.Code)

		second := testutil.GET(t, w.e, "/file-archive/download?ticket="+ticket.Ticket, nil)
		require.Equal(t, http.StatusUnauthorized, second.Code, "повторное использование билета обязано отказать")
	})

	t.Run("без билета - 401", func(t *testing.T) {
		rec := testutil.GET(t, w.e, "/file-archive/download", nil)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("слишком большой период отказывает уже при выдаче билета", func(t *testing.T) {
		testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{ZipMaxBytes: testutil.Ptr[int64](1)})
		t.Cleanup(func() {
			testutil.SetArchiveSettings(t, w.db, models.UpdateArchiveSettingsRequest{ZipMaxBytes: testutil.Ptr[int64](2147483648)})
		})

		estRec := testutil.POST(t, w.e, "/file-archive/estimate",
			fmt.Sprintf(`{"date_from":%q,"date_to":%q}`, today, today), w.adminH)
		require.Equal(t, http.StatusOK, estRec.Code)
		assert.True(t, testutil.ParseResponse[models.ArchiveDownloadEstimate](t, estRec).ExceedsLimit,
			"оценка обязана предупредить о том же пределе, который откажет billет")

		ticketRec := testutil.POST(t, w.e, "/file-archive/download-ticket",
			fmt.Sprintf(`{"date_from":%q,"date_to":%q}`, today, today), w.adminH)
		require.Equal(t, http.StatusRequestEntityTooLarge, ticketRec.Code, ticketRec.Body.String())
	})
}

// Гейт ZIP сохранённых файлов заявки повторяет гейт скачивания одного бланка
// (canDownloadBlank): отправитель заявки скачивает архив, посторонний - нет.
func TestFileArchiveDownload_Application(t *testing.T) {
	e, db, _, cleanup := testutil.SetupTestAppWithArchive(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	adminH := testutil.AuthHeader(adminToken)
	testutil.SetArchiveSettings(t, db, models.UpdateArchiveSettingsRequest{Enabled: testutil.Ptr(true)})

	userTypeID := secUserTypeIDByCode(t, db, "user")
	senderToken := testutil.RegisterAndLogin(t, e, "archivedlappsender", archiveDownloadPassword, userTypeID, td.OrgID, td.CompanyID)
	senderID := secUserIDByUsername(t, db, "archivedlappsender")
	outsiderToken := testutil.RegisterAndLogin(t, e, "archivedlappoutsider", archiveDownloadPassword, userTypeID, td.OrgID, td.CompanyID)

	name := "app_archive_type"
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &name, IsActive: true, AutoExport: true}
	require.NoError(t, db.Create(&ua).Error)
	blankSeedTemplate(t, db, ua.ID)

	now := time.Now()
	num, status, conf := "20260731/777", models.StatusInWork, models.ConfirmationApproved
	app := models.Application{
		ApplicationNumber: &num, OrganizationID: td.OrgID, SenderUserID: senderID,
		Status: &status, Confirmation: &conf, SendingDatetime: &now,
	}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", UniqueAttachmentID: &ua.ID}
	require.NoError(t, db.Create(&att).Error)

	exportRec := testutil.POST(t, e, fmt.Sprintf("/file-archive/applications/%d/reexport", app.ID), `{}`, adminH)
	require.Equal(t, http.StatusOK, exportRec.Code, exportRec.Body.String())
	exported := testutil.ParseResponse[models.BlankExportResult](t, exportRec)
	require.Len(t, exported.Items, 1)
	require.Equal(t, models.BlankExportOK, exported.Items[0].Status, exported.Items[0].Error)

	url := fmt.Sprintf("/applications/%d/archive", app.ID)

	t.Run("посторонний не скачивает архив чужой заявки", func(t *testing.T) {
		rec := testutil.GET(t, e, url, testutil.AuthHeader(outsiderToken))
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	})

	// В ZIP лежат сохранённые бланки с документами участников, поэтому он открыт
	// инициатору заявки и носителю права на выгрузку - как и бланк поштучно.
	t.Run("отправитель скачивает ZIP своей заявки", func(t *testing.T) {
		rec := testutil.GET(t, e, url, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("админ скачивает ZIP заявки со слепком и бланком", func(t *testing.T) {
		rec := testutil.GET(t, e, url, adminH)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		body := rec.Body.Bytes()
		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(zr.File), 2, "ожидались и слепок заявки, и бланк вложения")

		var hasSnapshot, hasBlank bool
		for _, f := range zr.File {
			switch {
			case f.Name == "заявка.json":
				hasSnapshot = true
			case strings.HasSuffix(f.Name, ".xlsx"):
				hasBlank = true
			}
		}
		assert.True(t, hasSnapshot, "в архиве должен быть машиночитаемый слепок заявки")
		assert.True(t, hasBlank, "в архиве должен быть бланк вложения")
	})
}
