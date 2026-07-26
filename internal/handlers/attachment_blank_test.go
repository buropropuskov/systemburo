package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/services"
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

// Список бланка заполняется по типу вложения (#1454). Боевой шаблон "Заявка на ввоз"
// (items) несёт привязки car.* к номеру машины: раньше они перехватывали определение
// типа списка, и таблица ТМЦ уезжала пустой.
func TestBlankGenerate_ListTypeFromAttachmentType(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blanklistsender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	name := "items_blank"
	ua := models.UniqueAttachment{AttachmentType: "items", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), "items.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: "items.xlsx", ListStartRow: 30, ListEndRow: 47, MaxListRows: 18,
	}
	require.NoError(t, db.Create(&tpl).Error)
	// Порядок как в боевом шаблоне: привязки к машине созданы раньше привязок к ТМЦ.
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "I21", FieldPath: "car.car_number", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "I22", FieldPath: "car.mark_name", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "B30", FieldPath: "item.name", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "R30", FieldPath: "item.count", IsListField: true},
	}).Error)

	now := time.Now()
	conf, status := "Согласовано", models.StatusInWork
	app := models.Application{
		OrganizationID: td.OrgID, SenderUserID: sender.ID,
		Confirmation: &conf, Status: &status, SendingDatetime: &now,
	}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "items", UniqueAttachmentID: &ua.ID}
	require.NoError(t, db.Create(&att).Error)

	cargo, count := "Груз-2", 3
	require.NoError(t, db.Create(&models.Item{AttachmentID: att.ID, Name: &cargo, Count: &count}).Error)

	reader, filename, err := services.NewAttachmentBlankService(db).
		GenerateBlank(context.Background(), app.ID, att.ID)
	require.NoError(t, err)
	require.Contains(t, filename, ".xlsx")

	out, err := excelize.OpenReader(reader)
	require.NoError(t, err)
	defer func() { require.NoError(t, out.Close()) }()
	sheet := out.GetSheetName(0)

	gotName, err := out.GetCellValue(sheet, "B30")
	require.NoError(t, err)
	require.Equal(t, cargo, gotName, "наименование ТМЦ должно попасть в первую строку списка")

	gotCount, err := out.GetCellValue(sheet, "R30")
	require.NoError(t, err)
	require.Equal(t, "3", gotCount)

	// Привязка чужой группы остаётся пустой: машин у items-вложения нет.
	gotCar, err := out.GetCellValue(sheet, "I30")
	require.NoError(t, err)
	require.Empty(t, gotCar, "привязка car.* не должна писать в строки списка ТМЦ")
}

// Разделитель совмещённых полей (#1454): nil - настройки нет, берём ", ";
// заданная пустая строка - осознанный выбор склеивать без разделителя.
func TestBlankGenerate_ConcatSeparator(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userTypeID := secUserTypeIDByCode(t, db, "user")
	phone := "89100530055"
	last, first := "Иванов", "Пётр"
	sender := models.User{
		Username: "blankconcatsender", Password: "x", TypeID: userTypeID,
		OrganizationID: secPtrInt(td.OrgID), Phone: &phone, LastName: &last, FirstName: &first,
	}
	require.NoError(t, db.Create(&sender).Error)

	name := "concat_blank"
	ua := models.UniqueAttachment{AttachmentType: "items", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), "concat.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: "concat.xlsx", ListStartRow: 10, ListEndRow: 12, MaxListRows: 3,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "A43", FieldPath: "application.sender.phone"},
		{TemplateID: tpl.ID, CellRef: "A43", FieldPath: "application.sender.short_name"},
	}).Error)

	now := time.Now()
	conf, status := "Согласовано", models.StatusInWork
	app := models.Application{
		OrganizationID: td.OrgID, SenderUserID: sender.ID,
		Confirmation: &conf, Status: &status, SendingDatetime: &now,
	}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "items", UniqueAttachmentID: &ua.ID}
	require.NoError(t, db.Create(&att).Error)

	cellValue := func(t *testing.T) string {
		t.Helper()
		reader, _, err := services.NewAttachmentBlankService(db).
			GenerateBlank(context.Background(), app.ID, att.ID)
		require.NoError(t, err)
		out, err := excelize.OpenReader(reader)
		require.NoError(t, err)
		defer func() { require.NoError(t, out.Close()) }()
		v, err := out.GetCellValue(out.GetSheetName(0), "A43")
		require.NoError(t, err)
		return v
	}

	t.Run("без настройки склеиваем запятой", func(t *testing.T) {
		require.Equal(t, "89100530055, Иванов П.", cellValue(t))
	})

	t.Run("заданная пустая строка склеивает без разделителя", func(t *testing.T) {
		empty := ""
		require.NoError(t, db.Model(&models.AttachmentTemplate{}).Where("id = ?", tpl.ID).
			Update("concat_separator", &empty).Error)
		require.Equal(t, "89100530055Иванов П.", cellValue(t))
	})

	t.Run("свой разделитель уважается", func(t *testing.T) {
		sep := " / "
		require.NoError(t, db.Model(&models.AttachmentTemplate{}).Where("id = ?", tpl.ID).
			Update("concat_separator", &sep).Error)
		require.Equal(t, "89100530055 / Иванов П.", cellValue(t))
	})
}
