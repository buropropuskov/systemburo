package handlers_test

import (
	"fmt"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Тумблер автосохранения на типе вложения (#1615): включён по умолчанию и не гаснет
// от сохранения формы, которая про архив ничего не знает.
func TestAttachments_AutoExport_DefaultsOnAndSurvivesUnrelatedUpdate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	adminH := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	created := testutil.POST(t, e, "/attachments", `{
		"attachment_type": "cars",
		"name": "archive-toggle",
		"display_name": "Архивный тип",
		"title": "Архивный тип"
	}`, adminH)
	require.Equal(t, http.StatusOK, created.Code, created.Body.String())
	id := int(testutil.ParseMap(t, created)["id"].(float64))

	assert.True(t, autoExportOf(t, db, id), "новый тип вложения выгружается по умолчанию")

	off := testutil.PUT(t, e, fmt.Sprintf("/attachments/%d", id), `{
		"attachment_type": "cars",
		"name": "archive-toggle",
		"display_name": "Архивный тип",
		"title": "Архивный тип",
		"auto_export": false
	}`, adminH)
	require.Equal(t, http.StatusOK, off.Code, off.Body.String())
	assert.False(t, autoExportOf(t, db, id))

	// Запрос без ключа auto_export (форма шаблона вложения) не трогает тумблер.
	renamed := testutil.PUT(t, e, fmt.Sprintf("/attachments/%d", id), `{
		"attachment_type": "cars",
		"name": "archive-toggle",
		"display_name": "Переименованный тип",
		"title": "Архивный тип"
	}`, adminH)
	require.Equal(t, http.StatusOK, renamed.Code, renamed.Body.String())
	assert.False(t, autoExportOf(t, db, id), "сохранение имени не должно включать выгрузку обратно")

	// Переключение видно в истории типа вложения: она ведётся общим журналом.
	history := testutil.GET(t, e, fmt.Sprintf("/attachments/%d/history", id), adminH)
	require.Equal(t, http.StatusOK, history.Code, history.Body.String())
	assert.Contains(t, history.Body.String(), "auto_export")
}

func autoExportOf(t *testing.T, db *gorm.DB, id int) bool {
	t.Helper()
	var attachment models.UniqueAttachment
	require.NoError(t, db.First(&attachment, id).Error)
	return attachment.AutoExport
}
