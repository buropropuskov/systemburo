package handlers_test

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// Гейт скачивания Excel-бланка (#1454). До фикса ручка висела на голой JWT-группе:
// посторонний получал 403 на /details и /attachments, но забирал .xlsx с ФИО и паспортами.
// Реюз DB-хелперов secWorld/setupSecurityHTTP (тот же пакет handlers_test).

const blankTestPassword = "blankpass_long_enough_for_login"

// blankSeedTemplate кладёт на диск пустой .xlsx и делает его активным шаблоном типа вложения.
func blankSeedTemplate(t *testing.T, db *gorm.DB, uaID int) {
	t.Helper()
	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), "template.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: uaID,
		IsActive:           true,
		FilePath:           path,
		OriginalFileName:   "template.xlsx",
		ListStartRow:       5,
		ListEndRow:         10,
		MaxListRows:        6,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "A1", FieldPath: "application.application_number"},
		{TemplateID: tpl.ID, CellRef: "B5", FieldPath: "employee.last_name", IsListField: true},
	}).Error)
}

// blankWorld - заявка отправителя с people-вложением, у которого настроен бланк,
// плюс сотрудник на посту tableID (по нему охрана получает доступ).
type blankWorld struct {
	h       secHTTPWorld
	appID   int
	attID   int
	tableID int
	url     string
}

func setupBlankWorld(t *testing.T) blankWorld {
	t.Helper()
	h := setupSecurityHTTP(t)
	db := h.w.db

	name := "people_blank"
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)
	blankSeedTemplate(t, db, ua.ID)

	appID := h.w.newApp(t, "Согласовано")
	att := models.Attachment{ApplicationID: &appID, AttachmentType: "people", UniqueAttachmentID: &ua.ID}
	require.NoError(t, db.Create(&att).Error)

	tableID := h.w.newPeopleTable(t, "blank-post")
	h.w.attachEmployeeWithTable(t, att.ID, tableID)

	return blankWorld{
		h: h, appID: appID, attID: att.ID, tableID: tableID,
		url: fmt.Sprintf("/applications/%d/blank?attachment_id=%d", appID, att.ID),
	}
}

func TestBlankDownload_AccessGate(t *testing.T) {
	w := setupBlankWorld(t)
	userTypeID := secUserTypeIDByCode(t, w.h.w.db, "user")
	outsider := testutil.RegisterAndLogin(t, w.h.e, "blankoutsider", blankTestPassword, userTypeID, w.h.w.orgID, 0)

	t.Run("посторонний не скачивает чужой бланк", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, w.url, testutil.AuthHeader(outsider))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("отправитель скачивает свой бланк", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, w.url, testutil.AuthHeader(w.h.userToken))
		require.Equal(t, http.StatusOK, rec.Code)
		require.NotEmpty(t, rec.Body.Bytes())
		require.Contains(t, rec.Header().Get("Content-Disposition"), "attachment;")
	})

	t.Run("админ скачивает любой бланк", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, w.url, testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusOK, rec.Code)
	})

	// Порядок важен: сначала охрана без назначенного поста, потом с ним.
	t.Run("охрана без своего поста не скачивает", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, w.url, testutil.AuthHeader(w.h.guardToken))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("охрана скачивает бланк своего поста", func(t *testing.T) {
		w.h.w.assignTable(t, w.tableID)
		rec := testutil.GET(t, w.h.e, w.url, testutil.AuthHeader(w.h.guardToken))
		require.Equal(t, http.StatusOK, rec.Code)
	})
}

// Настройка бланка - инструмент справочника: правку шаблона и полей вложения
// пускаем только по page.admin.directories (#1454).
func TestAttachmentTemplateRoutes_RequireDirectories(t *testing.T) {
	w := setupBlankWorld(t)
	userTypeID := secUserTypeIDByCode(t, w.h.w.db, "user")
	plain := testutil.RegisterAndLogin(t, w.h.e, "blankplainuser", blankTestPassword, userTypeID, w.h.w.orgID, 0)

	var uaID int
	require.NoError(t, w.h.w.db.Table("unique_attachments").Where("name = ?", "people_blank").
		Select("id").Scan(&uaID).Error)

	t.Run("обычный пользователь не правит привязки", func(t *testing.T) {
		rec := testutil.PUT(t, w.h.e, fmt.Sprintf("/attachments/%d/template/mappings", uaID),
			`{"mappings":[]}`, testutil.AuthHeader(plain))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("обычный пользователь не правит настройку полей", func(t *testing.T) {
		rec := testutil.PUT(t, w.h.e, fmt.Sprintf("/attachments/%d/field-config", uaID),
			`{"base":[]}`, testutil.AuthHeader(plain))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("обычный пользователь не качает файл шаблона", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/template/file", uaID), testutil.AuthHeader(plain))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("форма подачи читает настройку полей без права", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/field-config", uaID), testutil.AuthHeader(plain))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("список активных типов читается без права", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, "/attachments", testutil.AuthHeader(plain))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})

	t.Run("админ правит привязки", func(t *testing.T) {
		rec := testutil.PUT(t, w.h.e, fmt.Sprintf("/attachments/%d/template/mappings", uaID),
			`{"mappings":[{"cell_ref":"A2","field_path":"application.status","is_list_field":false}]}`,
			testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	})
}
