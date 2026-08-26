package handlers_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
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

// Гейты и параметры шаблона живут секциями на одном поднятом приложении: отдельные
// SetupTestApp с CleanDB перебивали границу go test -timeout у пакета handlers.
func TestBlankAccessAndTemplateRoutes(t *testing.T) {
	w := setupBlankWorld(t)
	t.Run("скачивание бланка", func(t *testing.T) { blankDownloadGateSection(t, w) })
	t.Run("настройка шаблона под правом", func(t *testing.T) { templateRoutesSection(t, w) })
	t.Run("пустой бланк для заполнения", func(t *testing.T) { blankTemplateDownloadSection(t, w) })
	t.Run("границы списка", func(t *testing.T) { templateParamsSection(t, w) })
	t.Run("журнал доступа к персональным данным", func(t *testing.T) { pdAuditSection(t, w) })
	t.Run("копирование привязок", func(t *testing.T) { copyMappingsSection(t, w) })
	t.Run("удаление бланка", func(t *testing.T) { deleteTemplateSection(t, w) })
}

// Удаление активного бланка не выключает генерацию: у вложения обычно несколько
// файлов, активным становится самый свежий из оставшихся, и настройка продолжает
// работать. Удаление неактивного файла активный не трогает.
func deleteTemplateSection(t *testing.T, w blankWorld) {
	db := w.h.w.db
	admin := testutil.AuthHeader(w.h.adminToken)

	uaID, oldTpl := copySeedTemplate(t, db, "delete_blank", "cars", 5, 8)
	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), "delete_blank_2.xlsx")
	require.NoError(t, f.SaveAs(path))
	require.NoError(t, db.Model(&models.AttachmentTemplate{}).Where("id = ?", oldTpl).
		Update("is_active", false).Error)
	newTpl := models.AttachmentTemplate{
		UniqueAttachmentID: uaID, IsActive: true, FilePath: path,
		OriginalFileName: "delete_blank_2.xlsx", ListStartRow: 5, ListEndRow: 8, MaxListRows: 4,
	}
	require.NoError(t, db.Create(&newTpl).Error)

	t.Run("удаление неактивного файла активный не трогает", func(t *testing.T) {
		rec := testutil.DELETE(t, w.h.e, fmt.Sprintf("/attachments/%d/template/%d", uaID, oldTpl), admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		got := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/template", uaID), admin)
		require.Equal(t, http.StatusOK, got.Code, "активный шаблон должен остаться")
		require.Contains(t, got.Body.String(), "delete_blank_2.xlsx")
	})

	t.Run("удаление активного файла оставляет генерацию на соседнем", func(t *testing.T) {
		spare := models.AttachmentTemplate{
			UniqueAttachmentID: uaID, FilePath: path,
			OriginalFileName: "delete_blank_spare.xlsx", ListStartRow: 5, ListEndRow: 8, MaxListRows: 4,
		}
		require.NoError(t, db.Create(&spare).Error)
		// is_active в модели с default:true, поэтому нулевое значение при Create не
		// сохраняется - гасим флаг явным апдейтом.
		require.NoError(t, db.Model(&models.AttachmentTemplate{}).Where("id = ?", spare.ID).
			Update("is_active", false).Error)

		rec := testutil.DELETE(t, w.h.e, fmt.Sprintf("/attachments/%d/template/%d", uaID, newTpl.ID), admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		got := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/template", uaID), admin)
		require.Equal(t, http.StatusOK, got.Code, "генерация должна остаться включённой")
		require.Contains(t, got.Body.String(), "delete_blank_spare.xlsx", "активным становится оставшийся файл")
	})

	t.Run("удаление последнего файла оставляет вложение без шаблона", func(t *testing.T) {
		uaOne, only := copySeedTemplate(t, db, "delete_blank_one", "cars", 5, 8)
		rec := testutil.DELETE(t, w.h.e, fmt.Sprintf("/attachments/%d/template/%d", uaOne, only), admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		got := testutil.GET(t, w.h.e, fmt.Sprintf("/attachments/%d/template", uaOne), admin)
		require.Equal(t, http.StatusNotFound, got.Code, "активировать больше нечего")
	})
}

// copySeedTemplate создаёт тип вложения с активным шаблоном и возвращает их id.
func copySeedTemplate(t *testing.T, db *gorm.DB, name, attachmentType string, start, end int) (int, int) {
	t.Helper()
	nm := name
	ua := models.UniqueAttachment{AttachmentType: attachmentType, Name: &nm, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), name+".xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: name + ".xlsx",
		ListStartRow:     start, ListEndRow: end, MaxListRows: end - start + 1,
	}
	require.NoError(t, db.Create(&tpl).Error)
	return ua.ID, tpl.ID
}

// Перенос привязок с одного бланка на другой: настраивая новый тип вложения, админ
// набивал те же пары ячейка-поле заново.
func copyMappingsSection(t *testing.T, w blankWorld) {
	db := w.h.w.db
	admin := testutil.AuthHeader(w.h.adminToken)
	userTypeID := secUserTypeIDByCode(t, db, "user")
	plain := testutil.RegisterAndLogin(t, w.h.e, "blankcopyuser", blankTestPassword, userTypeID, w.h.w.orgID, 0)

	srcUA, srcTpl := copySeedTemplate(t, db, "copy_source", "cars", 19, 33)
	dstUA, dstTpl := copySeedTemplate(t, db, "copy_target", "cars", 1, 1)
	itemsUA, itemsTpl := copySeedTemplate(t, db, "copy_items", "items", 4, 8)

	// Кастомное поле есть у обоих типов, но со своими id: переносится по названию.
	srcField := models.AttachmentCustomField{UniqueAttachmentID: srcUA, Label: "Номер договора", IsActive: true}
	require.NoError(t, db.Create(&srcField).Error)
	dstField := models.AttachmentCustomField{UniqueAttachmentID: dstUA, Label: "номер  договора", IsActive: true}
	require.NoError(t, db.Create(&dstField).Error)
	// Поле, которого у цели нет вовсе.
	orphanField := models.AttachmentCustomField{UniqueAttachmentID: srcUA, Label: "Пропуск охраны", IsActive: true}
	require.NoError(t, db.Create(&orphanField).Error)

	sep := " / "
	require.NoError(t, db.Model(&models.AttachmentTemplate{}).Where("id = ?", srcTpl).
		Update("concat_separator", &sep).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: srcTpl, CellRef: "A1", FieldPath: "application.application_number"},
		{TemplateID: srcTpl, CellRef: "F19", FieldPath: "application.organization"},
		{TemplateID: srcTpl, CellRef: "B19", FieldPath: "car.car_number", IsListField: true},
		{TemplateID: srcTpl, CellRef: "C19", FieldPath: fmt.Sprintf("custom.%d", srcField.ID)},
		{TemplateID: srcTpl, CellRef: "D19", FieldPath: fmt.Sprintf("custom.%d", orphanField.ID)},
	}).Error)

	mappingsOf := func(t *testing.T, tplID int) []models.AttachmentTemplateMapping {
		t.Helper()
		var out []models.AttachmentTemplateMapping
		require.NoError(t, db.Where("template_id = ?", tplID).Order("cell_ref").Find(&out).Error)
		return out
	}
	copyURL := func(uaID int) string {
		return fmt.Sprintf("/attachments/%d/template/copy-mappings", uaID)
	}

	t.Run("обычный пользователь не копирует", func(t *testing.T) {
		rec := testutil.POST(t, w.h.e, copyURL(dstUA),
			fmt.Sprintf(`{"source_template_id":%d}`, srcTpl), testutil.AuthHeader(plain))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("обычный пользователь не видит список источников", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, "/attachments/template-sources", testutil.AuthHeader(plain))
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("список источников отдаёт шаблоны со счётчиком привязок", func(t *testing.T) {
		rec := testutil.GET(t, w.h.e, "/attachments/template-sources", admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `"template_id":`+fmt.Sprint(srcTpl))
		require.Contains(t, rec.Body.String(), `"mappings_count":5`)
	})

	t.Run("свой же шаблон источником не берём", func(t *testing.T) {
		rec := testutil.POST(t, w.h.e, copyURL(dstUA),
			fmt.Sprintf(`{"source_template_id":%d}`, dstTpl), admin)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("перенос с заменой и границами списка", func(t *testing.T) {
		rec := testutil.POST(t, w.h.e, copyURL(dstUA),
			fmt.Sprintf(`{"source_template_id":%d,"replace":true,"copy_params":true}`, srcTpl), admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `"copied":4`)
		require.Contains(t, rec.Body.String(), `"skipped_custom":1`)
		require.Contains(t, rec.Body.String(), `"remapped_custom":1`)

		got := mappingsOf(t, dstTpl)
		require.Len(t, got, 4, "поле без пары у цели не переносится")
		paths := map[string]bool{}
		for _, m := range got {
			paths[m.CellRef+"="+m.FieldPath] = true
		}
		require.True(t, paths["B19=car.car_number"], "привязка списка своей группы переносится")
		require.True(t, paths["F19=application.organization"])
		require.True(t, paths[fmt.Sprintf("C19=custom.%d", dstField.ID)],
			"кастомное поле сопоставлено по названию с полем цели")

		var updated models.AttachmentTemplate
		require.NoError(t, db.First(&updated, dstTpl).Error)
		require.Equal(t, 19, updated.ListStartRow)
		require.Equal(t, 33, updated.ListEndRow)
		require.Equal(t, 15, updated.MaxListRows)
		require.NotNil(t, updated.ConcatSeparator)
		require.Equal(t, " / ", *updated.ConcatSeparator)
	})

	t.Run("повторный перенос без замены пропускает дубли", func(t *testing.T) {
		rec := testutil.POST(t, w.h.e, copyURL(dstUA),
			fmt.Sprintf(`{"source_template_id":%d,"replace":false}`, srcTpl), admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `"copied":0`)
		require.Contains(t, rec.Body.String(), `"skipped_duplicates":4`)
		require.Len(t, mappingsOf(t, dstTpl), 4, "дубли не удваивают привязки")
	})

	t.Run("привязки списка чужого типа не переносятся", func(t *testing.T) {
		rec := testutil.POST(t, w.h.e, copyURL(itemsUA),
			fmt.Sprintf(`{"source_template_id":%d,"replace":true}`, srcTpl), admin)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `"skipped_foreign_list":1`)

		for _, m := range mappingsOf(t, itemsTpl) {
			require.False(t, m.IsListField, "у бланка имущества привязок car.* быть не должно")
		}
		var params models.AttachmentTemplate
		require.NoError(t, db.First(&params, itemsTpl).Error)
		require.Equal(t, 4, params.ListStartRow, "без copy_params границы цели не трогаем")
	})
}

func blankDownloadGateSection(t *testing.T, w blankWorld) {
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
func templateRoutesSection(t *testing.T, w blankWorld) {
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
func TestBlankGenerate(t *testing.T) {
	_, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	t.Run("список по типу вложения", func(t *testing.T) { listTypeSection(t, db, td) })
	t.Run("разделитель совмещённых полей", func(t *testing.T) { concatSeparatorSection(t, db, td) })
	t.Run("места разгрузки вложения", func(t *testing.T) { attachmentPlacesSection(t, db, td) })
	t.Run("привязки машины", func(t *testing.T) { carBindingsSection(t, db, td) })
	t.Run("переполнение списка", func(t *testing.T) { listOverflowSection(t, db, td) })
	t.Run("поле заявки в строках списка", func(t *testing.T) { listRepeatedFieldSection(t, db, td) })
	t.Run("условное форматирование при переполнении", func(t *testing.T) { listOverflowConditionalSection(t, db, td) })
	t.Run("повтор шапки перенесённой таблицы", func(t *testing.T) { printTitlesSection(t, db, td) })
	t.Run("ТМЦ соседних вложений заявки", func(t *testing.T) { crossAttachmentItemsSection(t, db, td) })
	t.Run("таблица ТМЦ в бланке работ", func(t *testing.T) { itemsTableSection(t, db, td) })
	t.Run("подпись согласовавших", func(t *testing.T) { approverSignatureSection(t, db, td) })
	t.Run("повторная генерация побайтово совпадает", func(t *testing.T) { determinismSection(t, db, td) })
	t.Run("транспорт заявки в бланке ввоза", func(t *testing.T) { crossAttachmentCarsSection(t, db, td) })
}

// determinismSection - гейт дедупликации файлового архива (#1615). Архив не будет
// перезаписывать файл, если sha256 нового бланка совпал с сохранённым: это экономит
// запись и, что важнее, не двигает mtime - иначе инкрементальная выгрузка на рабочий
// ПК каждый раз тянула бы архив целиком, а ночная сверка переписывала бы его весь.
//
// Проверяется самый рискованный путь: переполненный список (excelize дублирует
// строки) вместе с условным форматированием (после excelize бланк пересобирается из
// zip вручную). Появится здесь текущее время - дедупликация не сработает никогда.
func determinismSection(t *testing.T, db *gorm.DB, td testutil.TestData) {
	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blankdetsender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	name := "det_blank"
	ua := models.UniqueAttachment{AttachmentType: "cars", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	sheet := f.GetSheetName(0)
	style, err := f.NewConditionalStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFC7CE"}}})
	require.NoError(t, err)
	require.NoError(t, f.SetConditionalFormat(sheet, "A20:D20", []excelize.ConditionalFormatOptions{
		{Type: "formula", Criteria: `$A$20=""`, Format: &style},
	}))
	path := filepath.Join(t.TempDir(), "det.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: "det.xlsx", ListStartRow: 10, ListEndRow: 12, MaxListRows: 3,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "A1", FieldPath: "application.application_number"},
		{TemplateID: tpl.ID, CellRef: "B10", FieldPath: "car.car_number", IsListField: true},
	}).Error)

	now := time.Now()
	conf, status := "Согласовано", models.StatusInWork
	number := "№ 20260731/077"
	app := models.Application{
		OrganizationID: td.OrgID, SenderUserID: sender.ID, ApplicationNumber: &number,
		Confirmation: &conf, Status: &status, SendingDatetime: &now,
	}
	require.NoError(t, db.Create(&app).Error)
	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars", UniqueAttachmentID: &ua.ID}
	require.NoError(t, db.Create(&att).Error)

	const carCount = 5 // на две больше, чем строк списка в шаблоне
	firstNumber := ""
	for i := 0; i < carCount; i++ {
		carNumber := fmt.Sprintf("Д %03d ДД 777", 401+i)
		if i == 0 {
			firstNumber = carNumber
		}
		require.NoError(t, db.Create(&models.Car{AttachmentID: att.ID, CarNumber: &carNumber}).Error)
	}

	svc := services.NewAttachmentBlankService(db)
	first := generateBlankBytes(t, svc, app.ID, att.ID)
	second := generateBlankBytes(t, svc, app.ID, att.ID)

	// Сначала убеждаемся, что бланк содержательный: два одинаково пустых результата
	// тоже "совпадают побайтово", и без этой проверки гейт был бы зелёным впустую.
	require.NotEmpty(t, first)
	out, err := excelize.OpenReader(bytes.NewReader(first))
	require.NoError(t, err)
	defer func() { require.NoError(t, out.Close()) }()
	gotNumber, err := out.GetCellValue(out.GetSheetName(0), "A1")
	require.NoError(t, err)
	require.Equal(t, number, gotNumber)
	gotCar, err := out.GetCellValue(out.GetSheetName(0), "B10")
	require.NoError(t, err)
	require.Equal(t, firstNumber, gotCar)
	gotOverflow, err := out.GetCellValue(out.GetSheetName(0), "B14")
	require.NoError(t, err)
	require.NotEmpty(t, gotOverflow, "список должен был расшириться, иначе рискованный путь не проверен")

	require.Equal(t, sha256.Sum256(first), sha256.Sum256(second),
		"два бланка по одним данным разошлись побайтово: дедупликация архива по хешу работать не будет")

	// Страховка на будущее, а не проверка сегодняшнего риска: excelize v2.11 пишет в
	// docProps/core.xml фиксированную дату и своего времени не проставляет, так что
	// сейчас условие выполняется всегда. Сторожит она апгрейд библиотеки или чей-то
	// SetDocProps(time.Now()) в сервисе - равенство двух прогонов в одном процессе
	// такую утечку пропустит, а архив сравнивает хеш с сохранённым другим процессом
	// и днями раньше, и там момент генерации разошёлся бы.
	core := blankCoreProps(t, first)
	require.NotEmpty(t, core, "в книге нет docProps/core.xml - проверка ниже стала бы пустой")
	require.NotContains(t, core, time.Now().Format("2006-01-02"),
		"в свойства книги попал момент генерации - хеш будет меняться от прогона к прогону")
}

func generateBlankBytes(t *testing.T, svc services.AttachmentBlankService, appID, attID int) []byte {
	t.Helper()
	reader, _, err := svc.GenerateBlank(context.Background(), appID, attID, services.BlankOptions{IncludeDocuments: true})
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	return data
}

// blankCoreProps возвращает docProps/core.xml готовой книги - именно туда офисные
// форматы пишут время изменения. Пустая строка, если раздела в книге нет.
func blankCoreProps(t *testing.T, data []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	for _, file := range zr.File {
		if file.Name != "docProps/core.xml" {
			continue
		}
		rc, err := file.Open()
		require.NoError(t, err)
		defer func() { require.NoError(t, rc.Close()) }()
		content, err := io.ReadAll(rc)
		require.NoError(t, err)
		return string(content)
	}
	return ""
}

// Подпись «СОГЛАСОВАНО» в бланке: обязательные согласования перечисляются все,
// необязательные представляет первый согласовавший.
func approverSignatureSection(t *testing.T, db *gorm.DB, td testutil.TestData) {
	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blankapproversender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	makeUser := func(t *testing.T, login, last, first, middle string) models.User {
		t.Helper()
		l, f, m := last, first, middle
		u := models.User{
			Username: login, Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID),
			LastName: &l, FirstName: &f, MiddleName: &m,
		}
		require.NoError(t, db.Create(&u).Error)
		return u
	}
	first := makeUser(t, "blankappr_first", "Иванов", "Иван", "Иванович")
	req1 := makeUser(t, "blankappr_req1", "Петров", "Пётр", "Петрович")
	req2 := makeUser(t, "blankappr_req2", "Сидорова", "Анна", "Сергеевна")

	name := "approver_blank"
	ua := models.UniqueAttachment{AttachmentType: "cars", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), "approver.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path, OriginalFileName: "approver.xlsx",
		ListStartRow: 10, ListEndRow: 12, MaxListRows: 3,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "B4", FieldPath: "application.approver_short_name"},
		{TemplateID: tpl.ID, CellRef: "B5", FieldPath: "application.approver_name"},
		{TemplateID: tpl.ID, CellRef: "B6", FieldPath: "application.approvers_short"},
	}).Error)

	// approve - кто согласовал заявку и было ли согласование обязательным.
	type approve struct {
		user     models.User
		required bool
		at       time.Time
	}
	makeApp := func(t *testing.T, approvals ...approve) (int, int) {
		t.Helper()
		now := time.Now()
		conf, status := "Согласовано", models.StatusInWork
		app := models.Application{
			OrganizationID: td.OrgID, SenderUserID: sender.ID,
			Confirmation: &conf, Status: &status, SendingDatetime: &now,
		}
		require.NoError(t, db.Create(&app).Error)
		att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars", UniqueAttachmentID: &ua.ID}
		require.NoError(t, db.Create(&att).Error)
		number := "Т 100 ТТ 777"
		require.NoError(t, db.Create(&models.Car{AttachmentID: att.ID, CarNumber: &number}).Error)
		approved := "approved"
		for _, a := range approvals {
			at := a.at
			require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
				ApplicationID: app.ID, UserID: a.user.ID, RequiredApproval: a.required,
				ApprovalStatus: &approved, ApprovalDatetime: &at,
			}).Error)
		}
		return app.ID, att.ID
	}

	cells := func(t *testing.T, appID, attID int, refs ...string) []string {
		t.Helper()
		reader, _, err := services.NewAttachmentBlankService(db).
			GenerateBlank(context.Background(), appID, attID, services.BlankOptions{IncludeDocuments: true})
		require.NoError(t, err)
		out, err := excelize.OpenReader(reader)
		require.NoError(t, err)
		defer func() { require.NoError(t, out.Close()) }()
		got := make([]string, 0, len(refs))
		for _, ref := range refs {
			v, err := out.GetCellValue(out.GetSheetName(0), ref)
			require.NoError(t, err)
			got = append(got, v)
		}
		return got
	}

	base := time.Now().Add(-3 * time.Hour)

	t.Run("обязательные согласования печатаются все", func(t *testing.T) {
		appID, attID := makeApp(t,
			approve{user: first, at: base},
			approve{user: req1, required: true, at: base.Add(time.Hour)},
			approve{user: req2, required: true, at: base.Add(2 * time.Hour)})
		got := cells(t, appID, attID, "B4", "B5", "B6")
		require.Equal(t, "Петров П. П., Сидорова А. С.", got[0])
		require.Equal(t, "Петров Пётр Петрович, Сидорова Анна Сергеевна", got[1])
		require.Equal(t, "Иванов И. И., Петров П. П., Сидорова А. С.", got[2],
			"поле «все согласовавшие» включает и необязательных")
	})

	t.Run("без обязательных подписывает первый согласовавший", func(t *testing.T) {
		appID, attID := makeApp(t,
			approve{user: req1, at: base.Add(time.Hour)},
			approve{user: first, at: base})
		got := cells(t, appID, attID, "B4", "B6")
		require.Equal(t, "Иванов И. И.", got[0], "первый по времени согласования")
		require.Equal(t, "Иванов И. И., Петров П. П.", got[1])
	})

	t.Run("никто не согласовал - ячейки пустые", func(t *testing.T) {
		appID, attID := makeApp(t)
		require.Equal(t, []string{"", "", ""}, cells(t, appID, attID, "B4", "B5", "B6"))
	})
}

// Вторая таблица бланка: ввозимый товар «Заявок на ввоз» заявки идёт строками, теми же
// привязками item.*, что и бланк самого ввоза. Списочная секция при этом остаётся за
// собственным типом вложения - у заявки на работы за сотрудниками.
func itemsTableSection(t *testing.T, db *gorm.DB, td testutil.TestData) {
	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blankitemstablesender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	name := "items_table_blank"
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	// Сотрудники 10-12, таблица ТМЦ 20-23, под обеими подпись в 30-й строке с правилом
	// условного форматирования: по ней видно, что вставки сдвинули разметку и формулы.
	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	sheet := f.GetSheetName(0)
	require.NoError(t, f.SetCellValue(sheet, "A9", "СОТРУДНИКИ"))
	require.NoError(t, f.SetCellValue(sheet, "A19", "ОБОРУДОВАНИЕ"))
	require.NoError(t, f.SetCellValue(sheet, "A30", "ПОДПИСЬ"))
	style, err := f.NewConditionalStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFC7CE"}}})
	require.NoError(t, err)
	require.NoError(t, f.SetConditionalFormat(sheet, "A30:D30", []excelize.ConditionalFormatOptions{
		{Type: "formula", Criteria: `$A$30=""`, Format: &style},
	}))
	path := filepath.Join(t.TempDir(), "items_table.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path, OriginalFileName: "items_table.xlsx",
		ListStartRow: 10, ListEndRow: 12, MaxListRows: 3,
		ItemsMaxListRows: 4,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "B10", FieldPath: "employee.last_name", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "A20", FieldPath: "item.row_number", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "B20", FieldPath: "item.name", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "F20", FieldPath: "item.count", IsListField: true},
	}).Error)

	// makeApp собирает заявку: сотрудники в своём вложении, ТМЦ - в «Заявках на ввоз».
	makeApp := func(t *testing.T, employees int, imports ...[]models.Item) (int, int) {
		t.Helper()
		now := time.Now()
		conf, status := "Согласовано", models.StatusInWork
		app := models.Application{
			OrganizationID: td.OrgID, SenderUserID: sender.ID,
			Confirmation: &conf, Status: &status, SendingDatetime: &now,
		}
		require.NoError(t, db.Create(&app).Error)
		works := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", UniqueAttachmentID: &ua.ID}
		require.NoError(t, db.Create(&works).Error)
		for i := 0; i < employees; i++ {
			last := fmt.Sprintf("Сотрудник%d", i+1)
			require.NoError(t, db.Create(&models.Employee{AttachmentID: &works.ID, LastName: &last}).Error)
		}
		for _, items := range imports {
			imp := models.Attachment{ApplicationID: &app.ID, AttachmentType: "items"}
			require.NoError(t, db.Create(&imp).Error)
			for i := range items {
				items[i].AttachmentID = imp.ID
				require.NoError(t, db.Create(&items[i]).Error)
			}
		}
		return app.ID, works.ID
	}

	item := func(name string, count int) models.Item {
		n, c := name, count
		return models.Item{Name: &n, Count: &c}
	}

	blank := func(t *testing.T, appID, attID int) *excelize.File {
		t.Helper()
		reader, _, err := services.NewAttachmentBlankService(db).
			GenerateBlank(context.Background(), appID, attID, services.BlankOptions{IncludeDocuments: true})
		require.NoError(t, err)
		out, err := excelize.OpenReader(reader)
		require.NoError(t, err)
		return out
	}
	cell := func(t *testing.T, out *excelize.File, ref string) string {
		t.Helper()
		v, err := out.GetCellValue(out.GetSheetName(0), ref)
		require.NoError(t, err)
		return v
	}
	signRow := func(t *testing.T, out *excelize.File) int {
		t.Helper()
		for row := 25; row <= 45; row++ {
			if cell(t, out, fmt.Sprintf("A%d", row)) == "ПОДПИСЬ" {
				return row
			}
		}
		return 0
	}

	t.Run("ввозимый товар идёт строками таблицы", func(t *testing.T) {
		appID, attID := makeApp(t, 1,
			[]models.Item{item("Кабель ВВГнг 3х2.5", 200), item("Щит распределительный", 2)},
			[]models.Item{item("Лестница-стремянка", 1)})
		out := blank(t, appID, attID)
		defer func() { require.NoError(t, out.Close()) }()

		require.Equal(t, "Сотрудник1", cell(t, out, "B10"), "список сотрудников на своём месте")
		require.Empty(t, cell(t, out, "B11"))

		require.Equal(t, []string{"1", "Кабель ВВГнг 3х2.5", "200"},
			[]string{cell(t, out, "A20"), cell(t, out, "B20"), cell(t, out, "F20")})
		require.Equal(t, []string{"2", "Щит распределительный", "2"},
			[]string{cell(t, out, "A21"), cell(t, out, "B21"), cell(t, out, "F21")})
		require.Equal(t, []string{"3", "Лестница-стремянка", "1"},
			[]string{cell(t, out, "A22"), cell(t, out, "B22"), cell(t, out, "F22")},
			"позиции второго вложения продолжают ту же таблицу")
		require.Empty(t, cell(t, out, "B23"), "лишняя строка таблицы остаётся пустой")
		require.Equal(t, 30, signRow(t, out), "разметка под таблицами не двигалась")
	})

	t.Run("расширение списка сотрудников сдвигает таблицу ТМЦ", func(t *testing.T) {
		appID, attID := makeApp(t, 5, []models.Item{item("Кабель", 10), item("Щит", 1)})
		out := blank(t, appID, attID)
		defer func() { require.NoError(t, out.Close()) }()

		require.Equal(t, "Сотрудник5", cell(t, out, "B14"), "список вырос на две строки")
		require.Equal(t, "Кабель", cell(t, out, "B22"), "таблица ТМЦ уехала вниз на столько же")
		require.Equal(t, "Щит", cell(t, out, "B23"))
		require.Equal(t, 32, signRow(t, out))
	})

	t.Run("переполнение таблицы ТМЦ добавляет строки", func(t *testing.T) {
		items := make([]models.Item, 0, 6)
		for i := 1; i <= 6; i++ {
			items = append(items, item(fmt.Sprintf("Позиция %d", i), i))
		}
		appID, attID := makeApp(t, 1, items)
		out := blank(t, appID, attID)
		defer func() { require.NoError(t, out.Close()) }()

		for i := 0; i < 6; i++ {
			require.Equal(t, fmt.Sprintf("Позиция %d", i+1), cell(t, out, fmt.Sprintf("B%d", 20+i)))
		}
		require.Equal(t, 32, signRow(t, out), "подпись сдвинулась на две добавленные строки")
	})

	t.Run("формулы условного форматирования сдвигаются на обе таблицы", func(t *testing.T) {
		items := make([]models.Item, 0, 6)
		for i := 1; i <= 6; i++ {
			items = append(items, item(fmt.Sprintf("Позиция %d", i), i))
		}
		appID, attID := makeApp(t, 5, items)
		out := blank(t, appID, attID)
		defer func() { require.NoError(t, out.Close()) }()

		// Сотрудников на 2 больше шаблона и ТМЦ на 2 - подпись уезжает на 4 строки.
		require.Equal(t, 34, signRow(t, out))
		formats, err := out.GetConditionalFormats(out.GetSheetName(0))
		require.NoError(t, err)
		var criteria string
		for ref, opts := range formats {
			if strings.HasPrefix(ref, "A34") && len(opts) > 0 {
				criteria = opts[0].Criteria
			}
		}
		require.Equal(t, `$A$34=""`, criteria, "формула правила должна указывать на строку подписи после сдвига")
	})
}

// ТМЦ «Заявок на ввоз» в бланке заявки на работы: списочная секция бланка занята
// сотрудниками, поэтому ввозимый товар идёт перечнем в одной ячейке. Позиции берутся
// из всех items-вложений заявки в порядке вложений.
func crossAttachmentItemsSection(t *testing.T, db *gorm.DB, td testutil.TestData) {
	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blankcrosssender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	name := "works_blank"
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	sheet := f.GetSheetName(0)
	// Заглушка в ячейке перечня и заданная высота строки: на заявке без ввоза заглушка
	// обязана остаться, на заявке с ввозом - высота уйти в авто.
	require.NoError(t, f.SetCellValue(sheet, "C5", "заполняется бюро"))
	require.NoError(t, f.SetRowHeight(sheet, 5, 12))
	path := filepath.Join(t.TempDir(), "works.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: "works.xlsx", ListStartRow: 10, ListEndRow: 12, MaxListRows: 3,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "C5", FieldPath: "app_items.names"},
		{TemplateID: tpl.ID, CellRef: "C6", FieldPath: "app_items.names_with_count"},
		{TemplateID: tpl.ID, CellRef: "C7", FieldPath: "app_items.total_count"},
		{TemplateID: tpl.ID, CellRef: "C8", FieldPath: "app_items.positions_count"},
		{TemplateID: tpl.ID, CellRef: "C9", FieldPath: "app_items.sources"},
		{TemplateID: tpl.ID, CellRef: "B10", FieldPath: "employee.last_name", IsListField: true},
	}).Error)

	makeApp := func(t *testing.T) (int, int) {
		t.Helper()
		now := time.Now()
		conf, status := "Согласовано", models.StatusInWork
		app := models.Application{
			OrganizationID: td.OrgID, SenderUserID: sender.ID,
			Confirmation: &conf, Status: &status, SendingDatetime: &now,
		}
		require.NoError(t, db.Create(&app).Error)
		works := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", UniqueAttachmentID: &ua.ID}
		require.NoError(t, db.Create(&works).Error)
		last := "Сидоров"
		require.NoError(t, db.Create(&models.Employee{AttachmentID: &works.ID, LastName: &last}).Error)
		return app.ID, works.ID
	}

	addImport := func(t *testing.T, appID int, display string, items ...models.Item) {
		t.Helper()
		nm := display
		imp := models.Attachment{
			ApplicationID: &appID, AttachmentType: "items", AttachmentDisplayName: &nm,
		}
		require.NoError(t, db.Create(&imp).Error)
		for i := range items {
			items[i].AttachmentID = imp.ID
			require.NoError(t, db.Create(&items[i]).Error)
		}
	}

	generate := func(t *testing.T, appID, attID int) *excelize.File {
		t.Helper()
		reader, _, err := services.NewAttachmentBlankService(db).
			GenerateBlank(context.Background(), appID, attID, services.BlankOptions{IncludeDocuments: true})
		require.NoError(t, err)
		out, err := excelize.OpenReader(reader)
		require.NoError(t, err)
		return out
	}
	cell := func(t *testing.T, out *excelize.File, ref string) string {
		t.Helper()
		v, err := out.GetCellValue(out.GetSheetName(0), ref)
		require.NoError(t, err)
		return v
	}

	t.Run("перечень собирается из всех заявок на ввоз", func(t *testing.T) {
		appID, attID := makeApp(t)
		cable, cableCount := "Кабель ВВГнг 3х2.5", 200
		shield, shieldCount := "Щит распределительный", 2
		ladder := "Лестница-стремянка"
		addImport(t, appID, "Заявка на ввоз",
			models.Item{Name: &cable, Count: &cableCount},
			models.Item{Name: &shield, Count: &shieldCount})
		addImport(t, appID, "Заявка на ввоз №2", models.Item{Name: &ladder})

		out := generate(t, appID, attID)
		defer func() { require.NoError(t, out.Close()) }()

		require.Equal(t, "Кабель ВВГнг 3х2.5\nЩит распределительный\nЛестница-стремянка",
			cell(t, out, "C5"), "позиции идут в порядке вложений заявки")
		require.Equal(t, "Кабель ВВГнг 3х2.5 - 200\nЩит распределительный - 2\nЛестница-стремянка",
			cell(t, out, "C6"))
		require.Equal(t, "202", cell(t, out, "C7"))
		require.Equal(t, "3", cell(t, out, "C8"))
		require.Equal(t, "Заявка на ввоз, Заявка на ввоз №2", cell(t, out, "C9"))

		// Списочная секция осталась за сотрудниками: ТМЦ в её строки не лезут.
		require.Equal(t, "Сидоров", cell(t, out, "B10"))
		require.Empty(t, cell(t, out, "B11"))

		// Перечень с переносами читается только при включённом переносе текста, а высота
		// строки должна пересчитаться, а не остаться шаблонной.
		styleID, err := out.GetCellStyle(out.GetSheetName(0), "C5")
		require.NoError(t, err)
		style, err := out.GetStyle(styleID)
		require.NoError(t, err)
		require.NotNil(t, style.Alignment)
		require.True(t, style.Alignment.WrapText, "у ячейки перечня должен быть включён перенос текста")

		height, err := out.GetRowHeight(out.GetSheetName(0), 5)
		require.NoError(t, err)
		require.NotEqual(t, 12.0, height, "заданная в шаблоне высота строки перечня должна смениться на авто")
	})

	t.Run("заявка без ввоза оставляет ячейку шаблона", func(t *testing.T) {
		appID, attID := makeApp(t)
		out := generate(t, appID, attID)
		defer func() { require.NoError(t, out.Close()) }()

		require.Equal(t, "заполняется бюро", cell(t, out, "C5"),
			"без заявок на ввоз содержимое шаблона не затирается")
		require.Empty(t, cell(t, out, "C8"))
	})
}

// Когда таблица не помещается на страницу, её продолжение начинается со своей шапки:
// генератор сам ставит разрыв страницы и повторяет заголовки ИМЕННО той таблицы,
// которая переносится. Сквозная строка Excel для этого не годится - она одна на лист
// и печаталась бы над чужой таблицей.
func printTitlesSection(t *testing.T, db *gorm.DB, td testutil.TestData) {
	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blanktitlesender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	name := "titles_blank"
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	// A4 при полях 5 мм и высоте строк 15 pt вмещает около 51 строки, поэтому таблицы
	// такого размера гарантированно переезжают на вторую и третью страницы.
	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	sheet := f.GetSheetName(0)
	paperA4, scale100 := 9, uint(100)
	portrait := "portrait"
	marginTop, marginBottom := 0.2, 0.2
	require.NoError(t, f.SetPageLayout(sheet, &excelize.PageLayoutOptions{
		Size: &paperA4, Orientation: &portrait, AdjustTo: &scale100,
	}))
	require.NoError(t, f.SetPageMargins(sheet, &excelize.PageLayoutMarginsOptions{
		Top: &marginTop, Bottom: &marginBottom,
	}))
	require.NoError(t, f.SetCellValue(sheet, "A9", "ШАПКА СОТРУДНИКОВ"))
	require.NoError(t, f.SetCellValue(sheet, "A39", "ШАПКА ТМЦ"))
	for row := 1; row <= 60; row++ {
		require.NoError(t, f.SetRowHeight(sheet, row, 15))
	}
	path := filepath.Join(t.TempDir(), "titles.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path, OriginalFileName: "titles.xlsx",
		ListStartRow: 10, ListEndRow: 12, MaxListRows: 3, ItemsMaxListRows: 3,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "B10", FieldPath: "employee.last_name", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "B40", FieldPath: "item.name", IsListField: true},
	}).Error)

	makeApp := func(t *testing.T, employees, items int) (int, int) {
		t.Helper()
		now := time.Now()
		conf, status := "Согласовано", models.StatusInWork
		app := models.Application{
			OrganizationID: td.OrgID, SenderUserID: sender.ID,
			Confirmation: &conf, Status: &status, SendingDatetime: &now,
		}
		require.NoError(t, db.Create(&app).Error)
		works := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", UniqueAttachmentID: &ua.ID}
		require.NoError(t, db.Create(&works).Error)
		for i := 0; i < employees; i++ {
			last := fmt.Sprintf("Сотрудник%03d", i+1)
			require.NoError(t, db.Create(&models.Employee{AttachmentID: &works.ID, LastName: &last}).Error)
		}
		if items > 0 {
			imp := models.Attachment{ApplicationID: &app.ID, AttachmentType: "items"}
			require.NoError(t, db.Create(&imp).Error)
			for i := 0; i < items; i++ {
				nm := fmt.Sprintf("Позиция %03d", i+1)
				require.NoError(t, db.Create(&models.Item{AttachmentID: imp.ID, Name: &nm}).Error)
			}
		}
		return app.ID, works.ID
	}

	// repeatedHeaders - сколько раз каждая шапка встретилась в бланке и сколько в нём
	// разрывов страниц. Разрывы читаем из XML книги: публичного метода для них нет.
	repeatedHeaders := func(t *testing.T, appID, attID int) (map[string]int, int) {
		t.Helper()
		reader, _, err := services.NewAttachmentBlankService(db).
			GenerateBlank(context.Background(), appID, attID, services.BlankOptions{IncludeDocuments: true})
		require.NoError(t, err)
		raw, err := io.ReadAll(reader)
		require.NoError(t, err)

		out, err := excelize.OpenReader(bytes.NewReader(raw))
		require.NoError(t, err)
		defer func() { require.NoError(t, out.Close()) }()

		counts := map[string]int{}
		rows, err := out.GetRows(out.GetSheetName(0))
		require.NoError(t, err)
		for _, row := range rows {
			if len(row) > 0 && (row[0] == "ШАПКА СОТРУДНИКОВ" || row[0] == "ШАПКА ТМЦ") {
				counts[row[0]]++
			}
		}
		// Сквозная строка листа больше не используется: шапки стоят в самом бланке.
		for _, dn := range out.GetDefinedName() {
			require.NotEqual(t, "_xlnm.Print_Titles", dn.Name, "сквозная строка листа не нужна")
		}

		zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		require.NoError(t, err)
		breaks := 0
		for _, file := range zr.File {
			if file.Name != "xl/worksheets/sheet1.xml" {
				continue
			}
			rc, err := file.Open()
			require.NoError(t, err)
			content, err := io.ReadAll(rc)
			require.NoError(t, rc.Close())
			require.NoError(t, err)
			if idx := strings.Index(string(content), "<rowBreaks"); idx >= 0 {
				tail := string(content)[idx:]
				breaks = strings.Count(tail[:strings.Index(tail, "</rowBreaks>")+len("</rowBreaks>")], "<brk ")
			}
		}
		return counts, breaks
	}

	t.Run("переносится список сотрудников - повторяется его шапка", func(t *testing.T) {
		appID, attID := makeApp(t, 80, 2)
		counts, breaks := repeatedHeaders(t, appID, attID)
		require.GreaterOrEqual(t, counts["ШАПКА СОТРУДНИКОВ"], 2, "шапка сотрудников повторилась на продолжении")
		require.Equal(t, 1, counts["ШАПКА ТМЦ"], "таблица ТМЦ поместилась - её шапка не дублируется")
		require.Positive(t, breaks, "разрыв страницы должен быть проставлен")
	})

	t.Run("переносится таблица ТМЦ - повторяется её шапка", func(t *testing.T) {
		appID, attID := makeApp(t, 1, 90)
		counts, _ := repeatedHeaders(t, appID, attID)
		require.GreaterOrEqual(t, counts["ШАПКА ТМЦ"], 2, "шапка ТМЦ повторилась на продолжении")
		require.Equal(t, 1, counts["ШАПКА СОТРУДНИКОВ"], "список сотрудников поместился - его шапка одна")
	})

	t.Run("обе таблицы короткие - ничего не повторяется", func(t *testing.T) {
		appID, attID := makeApp(t, 2, 2)
		counts, breaks := repeatedHeaders(t, appID, attID)
		require.Equal(t, 1, counts["ШАПКА СОТРУДНИКОВ"])
		require.Equal(t, 1, counts["ШАПКА ТМЦ"])
		require.Zero(t, breaks, "всё поместилось на страницу - разрывов нет")
	})
}

// Добавленные под список строки сдвигают диапазоны условного форматирования. Формулы
// внутри правил excelize не правит (adjustConditionalFormats меняет только SQRef),
// поэтому правило уезжало вниз, а условие читало прежнюю строку - подсветка работала
// не по той ячейке.
func listOverflowConditionalSection(t *testing.T, db *gorm.DB, td testutil.TestData) {
	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blankcfsender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	name := "cf_blank"
	ua := models.UniqueAttachment{AttachmentType: "cars", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	// Список 10-12, под ним подпись в 20-й строке с правилом «пусто - подсветить»,
	// и такое же правило выше списка - оно двигаться не должно.
	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	sheet := f.GetSheetName(0)
	style, err := f.NewConditionalStyle(&excelize.Style{Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFC7CE"}}})
	require.NoError(t, err)
	require.NoError(t, f.SetConditionalFormat(sheet, "A20:D20", []excelize.ConditionalFormatOptions{
		{Type: "formula", Criteria: `$A$20=""`, Format: &style},
	}))
	require.NoError(t, f.SetConditionalFormat(sheet, "A5", []excelize.ConditionalFormatOptions{
		{Type: "formula", Criteria: `$A$5=""`, Format: &style},
	}))
	path := filepath.Join(t.TempDir(), "cf.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: "cf.xlsx", ListStartRow: 10, ListEndRow: 12, MaxListRows: 3,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "B10", FieldPath: "car.car_number", IsListField: true},
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

	const carCount = 5 // на две больше, чем строк в шаблоне
	for i := 0; i < carCount; i++ {
		number := fmt.Sprintf("С %03d СС 777", 301+i)
		require.NoError(t, db.Create(&models.Car{AttachmentID: att.ID, CarNumber: &number}).Error)
	}

	reader, _, err := services.NewAttachmentBlankService(db).
		GenerateBlank(context.Background(), app.ID, att.ID, services.BlankOptions{IncludeDocuments: true})
	require.NoError(t, err)
	out, err := excelize.OpenReader(reader)
	require.NoError(t, err)
	defer func() { require.NoError(t, out.Close()) }()

	formats, err := out.GetConditionalFormats(out.GetSheetName(0))
	require.NoError(t, err)

	// Диапазон съехал на две строки (это делает excelize), формула обязана съехать так же.
	moved, ok := formats["A22:D22"]
	require.True(t, ok, "правило должно съехать в A22:D22, есть: %v", keysOf(formats))
	require.Len(t, moved, 1)
	require.Equal(t, `$A$22=""`, moved[0].Criteria,
		"формула правила должна указывать на съехавшую строку, а не на прежнюю")

	// Правило выше списка не двигается.
	kept, ok := formats["A5:A5"]
	if !ok {
		kept, ok = formats["A5"]
	}
	require.True(t, ok, "правило над списком должно остаться, есть: %v", keysOf(formats))
	require.Len(t, kept, 1)
	require.Equal(t, `$A$5=""`, kept[0].Criteria)
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Поле заявки, поставленное внутрь строк списка: значение у него одно на всю заявку
// (организация, компания), но в разметке бланка это колонка таблицы - значит стоять
// оно должно в каждой строке списка, а не только в первой. Так настроена боевая
// «Автозаявка»: F19 = application.organization при списке 19-33.
func listRepeatedFieldSection(t *testing.T, db *gorm.DB, td testutil.TestData) {
	userTypeID := secUserTypeIDByCode(t, db, "user")
	last := "Петров"
	sender := models.User{
		Username: "blankrepeatsender", Password: "x", TypeID: userTypeID,
		OrganizationID: secPtrInt(td.OrgID), LastName: &last,
	}
	require.NoError(t, db.Create(&sender).Error)

	name := "repeat_blank"
	ua := models.UniqueAttachment{AttachmentType: "cars", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), "repeat.xlsx")
	require.NoError(t, f.SaveAs(path))

	// Список на две строки, машин будет три - повтор обязан покрыть и добавленную строку.
	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: "repeat.xlsx", ListStartRow: 19, ListEndRow: 20, MaxListRows: 2,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "A19", FieldPath: "car.row_number", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "B19", FieldPath: "car.car_number", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "F19", FieldPath: "application.organization"},
		// Совмещённая ячейка внутри списка: повторяется склейка, а не первое поле.
		{TemplateID: tpl.ID, CellRef: "G19", FieldPath: "application.organization"},
		{TemplateID: tpl.ID, CellRef: "G19", FieldPath: "application.sender.last_name"},
		// Та же организация в шапке: вне строк списка остаётся одной ячейкой.
		{TemplateID: tpl.ID, CellRef: "H5", FieldPath: "application.organization"},
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

	const carCount = 3
	for i := 0; i < carCount; i++ {
		number := fmt.Sprintf("Р %03d РР 777", 201+i)
		require.NoError(t, db.Create(&models.Car{AttachmentID: att.ID, CarNumber: &number}).Error)
	}

	reader, _, err := services.NewAttachmentBlankService(db).
		GenerateBlank(context.Background(), app.ID, att.ID, services.BlankOptions{IncludeDocuments: true})
	require.NoError(t, err)
	out, err := excelize.OpenReader(reader)
	require.NoError(t, err)
	defer func() { require.NoError(t, out.Close()) }()
	outSheet := out.GetSheetName(0)

	cell := func(ref string) string {
		v, err := out.GetCellValue(outSheet, ref)
		require.NoError(t, err)
		return v
	}

	for i := 0; i < carCount; i++ {
		row := tpl.ListStartRow + i
		require.Equal(t, "Test Organization", cell(fmt.Sprintf("F%d", row)),
			"организация должна стоять в строке %d списка", row)
		require.Equal(t, "Test Organization, Петров", cell(fmt.Sprintf("G%d", row)),
			"совмещённая ячейка повторяется склейкой в строке %d", row)
	}

	// Пустые строки списка не заполняем: записей три, четвёртой строки нет.
	require.Empty(t, cell(fmt.Sprintf("F%d", tpl.ListStartRow+carCount)),
		"под последней записью организация не пишется")

	// Шапка не превращается в столбец.
	require.Equal(t, "Test Organization", cell("H5"))
	require.Empty(t, cell("H6"), "поле вне строк списка остаётся одной ячейкой")
}

func listTypeSection(t *testing.T, db *gorm.DB, td testutil.TestData) {

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
		GenerateBlank(context.Background(), app.ID, att.ID, services.BlankOptions{IncludeDocuments: true})
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
func concatSeparatorSection(t *testing.T, db *gorm.DB, td testutil.TestData) {

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
			GenerateBlank(context.Background(), app.ID, att.ID, services.BlankOptions{IncludeDocuments: true})
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
func attachmentPlacesSection(t *testing.T, db *gorm.DB, td testutil.TestData) {

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
		GenerateBlank(context.Background(), app.ID, att.ID, services.BlankOptions{IncludeDocuments: true})
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
func carBindingsSection(t *testing.T, db *gorm.DB, td testutil.TestData) {

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
		GenerateBlank(context.Background(), app.ID, att.ID, services.BlankOptions{IncludeDocuments: true})
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
func templateParamsSection(t *testing.T, w blankWorld) {
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

	// Таблица ТМЦ задаётся числом строк, а её начало сервис берёт из привязки полей ТМЦ.
	t.Run("без привязок к ТМЦ число строк отклоняется", func(t *testing.T) {
		rec := testutil.PUT(t, w.h.e, url, `{"list_start_row":7,"list_end_row":20,"items_max_list_rows":8}`,
			testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("начало таблицы ТМЦ берётся из привязки", func(t *testing.T) {
		var tplID int
		require.NoError(t, db.Table("attachment_templates").
			Where("unique_attachment_id = ? AND is_active = ?", uaID, true).
			Select("id").Scan(&tplID).Error)
		require.NoError(t, db.Create(&models.AttachmentTemplateMapping{
			TemplateID: tplID, CellRef: "B30", FieldPath: "item.name", IsListField: true,
		}).Error)

		rec := testutil.PUT(t, w.h.e, url, `{"list_start_row":7,"list_end_row":20,"items_max_list_rows":8}`,
			testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var tpl models.AttachmentTemplate
		require.NoError(t, db.Where("unique_attachment_id = ? AND is_active = ?", uaID, true).First(&tpl).Error)
		require.Equal(t, 8, tpl.ItemsMaxListRows)
		require.Equal(t, 30, tpl.ItemsListStartRow, "снимок границ считается по привязке")
		require.Equal(t, 37, tpl.ItemsListEndRow)
	})

	t.Run("таблица ТМЦ поверх списка отклоняется", func(t *testing.T) {
		rec := testutil.PUT(t, w.h.e, url, `{"list_start_row":25,"list_end_row":40,"items_max_list_rows":8}`,
			testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("ноль строк убирает таблицу ТМЦ из шаблона", func(t *testing.T) {
		rec := testutil.PUT(t, w.h.e, url, `{"list_start_row":7,"list_end_row":20,"items_max_list_rows":0}`,
			testutil.AuthHeader(w.h.adminToken))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var tpl models.AttachmentTemplate
		require.NoError(t, db.Where("unique_attachment_id = ? AND is_active = ?", uaID, true).First(&tpl).Error)
		require.Zero(t, tpl.ItemsMaxListRows)
		require.Zero(t, tpl.ItemsListStartRow)
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

// Переполнение списка (#1480): машин больше, чем строк в шаблоне. Строки должны
// добавляться ровно по недостаче, а разметка под таблицей - съезжать на столько же.
func listOverflowSection(t *testing.T, db *gorm.DB, td testutil.TestData) {
	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blankoverflowsender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	name := "overflow_blank"
	ua := models.UniqueAttachment{AttachmentType: "cars", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	// список занимает строки 10-15 (шесть), сразу под ним подпись в двадцатой
	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	sheet := f.GetSheetName(0)
	require.NoError(t, f.SetCellValue(sheet, "A9", "ШАПКА"))
	require.NoError(t, f.SetCellValue(sheet, "A20", "ПОДПИСЬ"))
	path := filepath.Join(t.TempDir(), "overflow.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: "overflow.xlsx", ListStartRow: 10, ListEndRow: 15, MaxListRows: 6,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "A10", FieldPath: "car.row_number", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "B10", FieldPath: "car.car_number", IsListField: true},
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

	const carCount = 8 // на две больше, чем строк в шаблоне
	for i := 0; i < carCount; i++ {
		number := fmt.Sprintf("Х %03d ХХ 777", 101+i)
		require.NoError(t, db.Create(&models.Car{AttachmentID: att.ID, CarNumber: &number}).Error)
	}

	reader, _, err := services.NewAttachmentBlankService(db).
		GenerateBlank(context.Background(), app.ID, att.ID, services.BlankOptions{IncludeDocuments: true})
	require.NoError(t, err)
	out, err := excelize.OpenReader(reader)
	require.NoError(t, err)
	defer func() { require.NoError(t, out.Close()) }()
	outSheet := out.GetSheetName(0)

	for i := 0; i < carCount; i++ {
		row := tpl.ListStartRow + i
		got, err := out.GetCellValue(outSheet, fmt.Sprintf("B%d", row))
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("Х %03d ХХ 777", 101+i), got, "строка %d списка", row)
	}

	// шапка над списком стоять остаётся
	head, err := out.GetCellValue(outSheet, "A9")
	require.NoError(t, err)
	require.Equal(t, "ШАПКА", head)

	// подпись съезжает ровно на недостающие строки: 20 + (8 - 6) = 22.
	// До фикса вставка шла дважды (InsertRows плюс DuplicateRowTo, который сам
	// вставляет строку), и подпись уезжала на 24, оставляя пустые строки в бланке.
	signRow := 0
	for r := tpl.ListEndRow; r <= 30; r++ {
		v, err := out.GetCellValue(outSheet, fmt.Sprintf("A%d", r))
		require.NoError(t, err)
		if v == "ПОДПИСЬ" {
			signRow = r
			break
		}
	}
	require.Equal(t, 22, signRow, "подпись должна съехать на две строки, а не на четыре")
}

// Бланк «Заявки на ввоз» печатает марку и номер транспорта из «Автозаявки» той же
// заявки: под это в бланке отведена одна ячейка, своих машин у вложения-ввоза нет.
func crossAttachmentCarsSection(t *testing.T, db *gorm.DB, td testutil.TestData) {
	userTypeID := secUserTypeIDByCode(t, db, "user")
	sender := models.User{Username: "blankcrosscarsender", Password: "x", TypeID: userTypeID, OrganizationID: secPtrInt(td.OrgID)}
	require.NoError(t, db.Create(&sender).Error)

	name := "import_cars_blank"
	ua := models.UniqueAttachment{AttachmentType: "items", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	require.NoError(t, f.SetCellValue(f.GetSheetName(0), "I21", "заполняется бюро"))
	path := filepath.Join(t.TempDir(), "import_cars.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path, OriginalFileName: "import_cars.xlsx",
		ListStartRow: 30, ListEndRow: 32, MaxListRows: 3,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "I21", FieldPath: "app_cars.marks_numbers"},
		{TemplateID: tpl.ID, CellRef: "I22", FieldPath: "app_cars.count"},
		{TemplateID: tpl.ID, CellRef: "B30", FieldPath: "item.name", IsListField: true},
	}).Error)

	// car - машина автозаявки: номер и марка.
	type car struct{ number, mark string }
	makeApp := func(t *testing.T, cars ...car) (int, int) {
		t.Helper()
		now := time.Now()
		conf, status := "Согласовано", models.StatusInWork
		app := models.Application{
			OrganizationID: td.OrgID, SenderUserID: sender.ID,
			Confirmation: &conf, Status: &status, SendingDatetime: &now,
		}
		require.NoError(t, db.Create(&app).Error)
		imp := models.Attachment{ApplicationID: &app.ID, AttachmentType: "items", UniqueAttachmentID: &ua.ID}
		require.NoError(t, db.Create(&imp).Error)
		cargo := "Кабель"
		require.NoError(t, db.Create(&models.Item{AttachmentID: imp.ID, Name: &cargo}).Error)
		if len(cars) > 0 {
			auto := models.Attachment{ApplicationID: &app.ID, AttachmentType: "cars"}
			require.NoError(t, db.Create(&auto).Error)
			for _, c := range cars {
				number, mark := c.number, c.mark
				row := models.Car{AttachmentID: auto.ID, CarNumber: &number}
				if mark != "" {
					row.MarkName = &mark
				}
				require.NoError(t, db.Create(&row).Error)
			}
		}
		return app.ID, imp.ID
	}

	cells := func(t *testing.T, appID, attID int, refs ...string) []string {
		t.Helper()
		reader, _, err := services.NewAttachmentBlankService(db).
			GenerateBlank(context.Background(), appID, attID, services.BlankOptions{IncludeDocuments: true})
		require.NoError(t, err)
		out, err := excelize.OpenReader(reader)
		require.NoError(t, err)
		defer func() { require.NoError(t, out.Close()) }()
		got := make([]string, 0, len(refs))
		for _, ref := range refs {
			v, err := out.GetCellValue(out.GetSheetName(0), ref)
			require.NoError(t, err)
			got = append(got, v)
		}
		return got
	}

	t.Run("марка и номер приходят из автозаявки", func(t *testing.T) {
		appID, attID := makeApp(t,
			car{number: "О 593 УЕ 325", mark: "ГАЗель"},
			car{number: "Х 101 ХХ 777"})
		got := cells(t, appID, attID, "I21", "I22", "B30")
		require.Equal(t, "ГАЗель О 593 УЕ 325\nХ 101 ХХ 777", got[0],
			"машина без марки печатается одним номером")
		require.Equal(t, "2", got[1])
		require.Equal(t, "Кабель", got[2], "собственный список ТМЦ заполняется как раньше")
	})

	t.Run("заявка без автозаявки оставляет ячейку шаблона", func(t *testing.T) {
		appID, attID := makeApp(t)
		got := cells(t, appID, attID, "I21", "I22")
		require.Equal(t, "заполняется бюро", got[0])
		require.Empty(t, got[1])
	})

	// Бланк несут на пост как документ допуска, поэтому машина непринятого дополнения
	// в нём означала бы проход мимо согласования - тот же гейт, что у своего состава
	// вложения и у ТМЦ соседних вложений (#1685).
	t.Run("машина непринятого дополнения в бланк не попадает", func(t *testing.T) {
		appID, attID := makeApp(t, car{number: "О 001 АА 777", mark: "Газель"})

		var auto models.Attachment
		require.NoError(t, db.Where("application_id = ? AND attachment_type = ?", appID, "cars").
			First(&auto).Error)
		sup := models.ApplicationSupplement{
			ApplicationID: appID, Number: 1,
			Status: models.SupplementPending, CreatedByUserID: sender.ID,
		}
		require.NoError(t, db.Create(&sup).Error)
		pendingNumber := "В 002 ВВ 777"
		require.NoError(t, db.Create(&models.Car{
			AttachmentID: auto.ID, CarNumber: &pendingNumber, SupplementID: &sup.ID,
		}).Error)

		got := cells(t, appID, attID, "I21")
		require.Equal(t, "Газель О 001 АА 777", got[0],
			"печатается только машина основной заявки")
	})
}
