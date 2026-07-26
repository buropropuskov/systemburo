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
		require.Equal(t, "+7 (910) 053 00-55, Иванов П.", cellValue(t))
	})

	t.Run("заданная пустая строка склеивает без разделителя", func(t *testing.T) {
		empty := ""
		require.NoError(t, db.Model(&models.AttachmentTemplate{}).Where("id = ?", tpl.ID).
			Update("concat_separator", &empty).Error)
		require.Equal(t, "+7 (910) 053 00-55Иванов П.", cellValue(t))
	})

	t.Run("свой разделитель уважается", func(t *testing.T) {
		sep := " / "
		require.NoError(t, db.Model(&models.AttachmentTemplate{}).Where("id = ?", tpl.ID).
			Update("concat_separator", &sep).Error)
		require.Equal(t, "+7 (910) 053 00-55 / Иванов П.", cellValue(t))
	})
}

// Места разгрузки вложения в бланке (#1454): для имущества это единственный источник
// мест, у ТМЦ своих машин нет.
func TestBlankGenerate_AttachmentUnloadPlaces(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blankplacessender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	name := "places_blank"
	ua := models.UniqueAttachment{AttachmentType: "items", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), "places.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: "places.xlsx", ListStartRow: 10, ListEndRow: 12, MaxListRows: 3,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "C5", FieldPath: "attachment.unload_places"},
		{TemplateID: tpl.ID, CellRef: "C6", FieldPath: "attachment.roof_access"},
		{TemplateID: tpl.ID, CellRef: "C7", FieldPath: "attachment.display_name"},
	}).Error)

	now := time.Now()
	conf, status := "Согласовано", models.StatusInWork
	app := models.Application{
		OrganizationID: td.OrgID, SenderUserID: sender.ID,
		Confirmation: &conf, Status: &status, SendingDatetime: &now,
	}
	require.NoError(t, db.Create(&app).Error)

	display := "Заявка на ввоз №1"
	att := models.Attachment{
		ApplicationID: &app.ID, AttachmentType: "items", UniqueAttachmentID: &ua.ID,
		AttachmentDisplayName: &display, RoofAccess: true,
	}
	require.NoError(t, db.Create(&att).Error)

	first := models.UnloadPlace{Name: "Ворота Черепашки", IsActive: true}
	second := models.UnloadPlace{Name: "Склад 4", IsActive: true}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)
	one, two := 1, 2
	require.NoError(t, db.Create(&models.AttachmentUnloadPlace{AttachmentID: att.ID, UnloadPlaceID: first.ID, OrderIndex: &one}).Error)
	require.NoError(t, db.Create(&models.AttachmentUnloadPlace{AttachmentID: att.ID, UnloadPlaceID: second.ID, OrderIndex: &two}).Error)

	reader, _, err := services.NewAttachmentBlankService(db).
		GenerateBlank(context.Background(), app.ID, att.ID)
	require.NoError(t, err)
	out, err := excelize.OpenReader(reader)
	require.NoError(t, err)
	defer func() { require.NoError(t, out.Close()) }()
	sheet := out.GetSheetName(0)

	places, err := out.GetCellValue(sheet, "C5")
	require.NoError(t, err)
	require.Equal(t, "Ворота Черепашки, Склад 4", places, "места идут в порядке привязки")

	roof, err := out.GetCellValue(sheet, "C6")
	require.NoError(t, err)
	require.Equal(t, "Да", roof)

	title, err := out.GetCellValue(sheet, "C7")
	require.NoError(t, err)
	require.Equal(t, display, title)
}

// Полный список мест разгрузки машины в бланке (#1454). Форма подачи кладёт в
// cars.unload_place строку "Первое место и др.", поэтому раньше в бланке видно
// было только первое место.
func TestBlankGenerate_CarBindings(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blankcarsender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	name := "cars_blank"
	ua := models.UniqueAttachment{AttachmentType: "cars", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), "cars.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: "cars.xlsx", ListStartRow: 19, ListEndRow: 24, MaxListRows: 6,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "B19", FieldPath: "car.car_number", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "G19", FieldPath: "car.unload_place", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "H19", FieldPath: "car.unload_places", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "I19", FieldPath: "car.passage_tables", IsListField: true},
	}).Error)

	now := time.Now()
	conf, status := "Согласовано", models.StatusInWork
	app := models.Application{
		OrganizationID: td.OrgID, SenderUserID: sender.ID,
		Confirmation: &conf, Status: &status, SendingDatetime: &now,
	}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars", UniqueAttachmentID: &ua.ID}
	require.NoError(t, db.Create(&att).Error)

	number, short := "О 593 УЕ 325", "Ворота Черепашки и др."
	car := models.Car{AttachmentID: att.ID, CarNumber: &number, UnloadPlace: &short}
	require.NoError(t, db.Create(&car).Error)

	first := models.UnloadPlace{Name: "Ворота Черепашки", IsActive: true}
	second := models.UnloadPlace{Name: "Склад 4", IsActive: true}
	require.NoError(t, db.Create(&first).Error)
	require.NoError(t, db.Create(&second).Error)
	one, two := 1, 2
	require.NoError(t, db.Create(&models.CarUnloadPlace{CarID: car.ID, UnloadPlaceID: first.ID, OrderIndex: &one}).Error)
	require.NoError(t, db.Create(&models.CarUnloadPlace{CarID: car.ID, UnloadPlaceID: second.ID, OrderIndex: &two}).Error)

	post := models.SystemTable{Name: "post-72", TableType: "cars", IsActive: true}
	require.NoError(t, db.Create(&post).Error)
	require.NoError(t, db.Create(&models.CarTargetTable{CarID: car.ID, TableID: post.ID, OrderIndex: &one}).Error)

	reader, _, err := services.NewAttachmentBlankService(db).
		GenerateBlank(context.Background(), app.ID, att.ID)
	require.NoError(t, err)
	out, err := excelize.OpenReader(reader)
	require.NoError(t, err)
	defer func() { require.NoError(t, out.Close()) }()
	sheet := out.GetSheetName(0)

	gotShort, err := out.GetCellValue(sheet, "G19")
	require.NoError(t, err)
	require.Equal(t, short, gotShort, "старое поле остаётся как было")

	gotFull, err := out.GetCellValue(sheet, "H19")
	require.NoError(t, err)
	require.Equal(t, "Ворота Черепашки, Склад 4", gotFull, "новое поле показывает все места")

	gotPost, err := out.GetCellValue(sheet, "I19")
	require.NoError(t, err)
	require.Equal(t, "post-72", gotPost)
}

// Границы строк списка правятся без перезагрузки файла (#1454): раньше их задавали
// только вместе с загрузкой .xlsx, и подвинуть диапазон значило перезалить тот же файл.
func TestTemplateParams_UpdateWithoutReupload(t *testing.T) {
	w := setupBlankWorld(t)
	db := w.h.w.db

	var uaID int
	require.NoError(t, db.Table("unique_attachments").Where("name = ?", "people_blank").
		Select("id").Scan(&uaID).Error)
	url := fmt.Sprintf("/attachments/%d/template/params", uaID)

	userTypeID := secUserTypeIDByCode(t, db, "user")
	plain := testutil.RegisterAndLogin(t, w.h.e, "blankparamsuser", blankTestPassword, userTypeID, w.h.w.orgID, 0)

	t.Run("обычный пользователь не правит границы", func(t *testing.T) {
		rec := testutil.PUT(t, w.h.e, url, `{"list_start_row":5,"list_end_row":20}`, testutil.AuthHeader(plain))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("админ двигает диапазон, максимум считается по нему", func(t *testing.T) {
		rec := testutil.PUT(t, w.h.e, url, `{"list_start_row":7,"list_end_row":20,"max_list_rows":0}`,
			testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var tpl models.AttachmentTemplate
		require.NoError(t, db.Where("unique_attachment_id = ? AND is_active = ?", uaID, true).First(&tpl).Error)
		require.Equal(t, 7, tpl.ListStartRow)
		require.Equal(t, 20, tpl.ListEndRow)
		require.Equal(t, 14, tpl.MaxListRows, "0 означает посчитать по диапазону")
	})

	t.Run("перевёрнутый диапазон отклоняется", func(t *testing.T) {
		rec := testutil.PUT(t, w.h.e, url, `{"list_start_row":30,"list_end_row":10}`,
			testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})
}

// «Инициатор заявки» и «Телефон» из шапки подачи сохраняются в заявке (#1454):
// раньше форма их требовала, а бэк отбрасывал, и в бланк они попасть не могли.
func TestSubmitApplication_StoresInitiatorAndPhone(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	uaID := seedUniqueAttachment(t, db, "cars", "initiator_cars", "Initiator Cars")
	token := testutil.RegisterAndLogin(t, e, "initiatorsender", "pass123", 1, td.OrgID, td.CompanyID)

	body := fmt.Sprintf(`{
		"message": "initiator test",
		"organization_id": %d,
		"responsible_person": "  Сидорова Анна Петровна  ",
		"contact_phone": "89100530055",
		"data_approval": true,
		"attachments": [{
			"attachment_type": "cars",
			"attachment_name": "initiator_cars",
			"attachment_display_name": "Initiator Cars",
			"unique_attachment_id": %d,
			"entry_date_from": "2026-04-01",
			"entry_date_to": "2099-12-31",
			"entry_time_from": "08:00",
			"entry_time_to": "18:00",
			"data": {"vehicles": [{"car_number": "Т777ТТ777", "car_brand": "Toyota"}]}
		}]
	}`, td.OrgID, uaID)
	rec := testutil.POST(t, e, "/applications/submit-complete-application", body, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	appID := testutil.ParseResponse[services.CompleteApplicationResponse](t, rec).ApplicationID

	var stored struct {
		InitiatorName *string
		ContactPhone  *string
	}
	require.NoError(t, db.Raw("SELECT initiator_name, contact_phone FROM applications WHERE id = ?", appID).
		Scan(&stored).Error)
	require.NotNil(t, stored.InitiatorName)
	require.Equal(t, "Сидорова Анна Петровна", *stored.InitiatorName, "пробелы по краям снимаются")
	require.NotNil(t, stored.ContactPhone)
	require.Equal(t, "89100530055", *stored.ContactPhone, "храним как ввели, форматируем при выводе")
}
