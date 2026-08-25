package handlers_test

import (
	"net/http"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Лента раздела администрирования отдаёт не только строку реестра, но и то, чем
// её опознаёт человек: номер заявки и наименование вложения. Без них на экране
// оставались внутренний идентификатор заявки и имя файла на диске - по ним
// дежурный не поймёт ни какая это заявка, ни какому вложению не хватает бланка.
func TestFileArchiveItems_CarryApplicationNumberAndAttachmentName(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	day := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	appID := statsSeedApplication(t, db, td.OrgID)
	attID := statsSeedAttachment(t, db, appID, "Автозаявка", true)

	quotaSeedRow(t, db, appID, attID, models.BlankExportOK, 1000, day)
	// Служебное описание заявки: вложения у него нет вовсе.
	quotaSeedRow(t, db, appID, 0, models.BlankExportOK, 500, day)

	rec := testutil.GET(t, e, "/file-archive/items?page=1&per_page=20", adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	items := testutil.ParseResponse[[]models.ArchiveItemView](t, rec)
	require.Len(t, items, 2)

	var number, attachmentName string
	var snapshotName = "не найдено"
	for _, item := range items {
		number = item.ApplicationNumber
		if item.AttachmentID > 0 {
			attachmentName = item.AttachmentName
		} else {
			snapshotName = item.AttachmentName
		}
	}

	assert.NotEmpty(t, number, "номер заявки обязан приезжать: по идентификатору её не опознать")
	assert.Equal(t, "Автозаявка", attachmentName,
		"наименование берётся из справочника - тем же выражением, что и имя файла на диске")
	assert.Empty(t, snapshotName, "у описания заявки вложения нет, имени взяться неоткуда")
}

// Строка, пережившая свою заявку и своё вложение, из ленты не пропадает: реестр
// намеренно живёт без внешних ключей, и файл на диске остаётся существовать.
func TestFileArchiveItems_SurviveDeletedApplicationAndAttachment(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	quotaSeedRow(t, db, 8801, 7701, models.BlankExportFailed, 0,
		time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC))

	rec := testutil.GET(t, e, "/file-archive/items?page=1&per_page=20&status=failed", adminH)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	items := testutil.ParseResponse[[]models.ArchiveItemView](t, rec)
	require.Len(t, items, 1, "осиротевшая строка обязана остаться в ленте")
	assert.Equal(t, 8801, items[0].ApplicationID)
	assert.Empty(t, items[0].ApplicationNumber, "номера нет - заявки уже нет")
	assert.Empty(t, items[0].AttachmentName)
}
