package handlers_test

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// documentsGateFixture - заявка с одним сотрудником, у которого заполнены все три поля
// раздела «Документы», и бланк, куда эти поля привязаны. Возвращает id заявки и вложения.
type documentsGateFixture struct {
	appID    int
	attID    int
	passport string
	patent   string
	other    string
}

func seedDocumentsGateApplication(t *testing.T, db *gorm.DB, td testutil.TestData, senderID int) documentsGateFixture {
	t.Helper()

	name := "docs_gate_blank"
	ua := models.UniqueAttachment{AttachmentType: "people", Name: &name, IsActive: true}
	require.NoError(t, db.Create(&ua).Error)

	f := excelize.NewFile()
	defer func() { require.NoError(t, f.Close()) }()
	path := filepath.Join(t.TempDir(), "docs_gate.xlsx")
	require.NoError(t, f.SaveAs(path))

	tpl := models.AttachmentTemplate{
		UniqueAttachmentID: ua.ID, IsActive: true, FilePath: path,
		OriginalFileName: "docs_gate.xlsx", ListStartRow: 10, ListEndRow: 12, MaxListRows: 3,
	}
	require.NoError(t, db.Create(&tpl).Error)
	require.NoError(t, db.Create(&[]models.AttachmentTemplateMapping{
		{TemplateID: tpl.ID, CellRef: "A10", FieldPath: "employee.last_name", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "B10", FieldPath: "employee.passport_series_number", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "C10", FieldPath: "employee.patent_number", IsListField: true},
		{TemplateID: tpl.ID, CellRef: "D10", FieldPath: "employee.other_permission", IsListField: true},
	}).Error)

	now := time.Now()
	conf, status := "Согласовано", models.StatusInWork
	number := "№ 20260820/001"
	app := models.Application{
		OrganizationID: td.OrgID, SenderUserID: senderID, ApplicationNumber: &number,
		Confirmation: &conf, Status: &status, SendingDatetime: &now,
	}
	require.NoError(t, db.Create(&app).Error)

	att := models.Attachment{ApplicationID: &app.ID, AttachmentType: "people", UniqueAttachmentID: &ua.ID}
	require.NoError(t, db.Create(&att).Error)

	fx := documentsGateFixture{
		appID:    app.ID,
		attID:    att.ID,
		passport: "4515 123456",
		patent:   "77 № 9988776",
		other:    "РВП 77-2026",
	}
	last := "Документов"
	require.NoError(t, db.Create(&models.Employee{
		AttachmentID:         &att.ID,
		LastName:             &last,
		PassportSeriesNumber: &fx.passport,
		PatentNumber:         &fx.patent,
		OtherPermission:      &fx.other,
	}).Error)
	return fx
}

// documentMaskInBlank - длинное тире, которое встаёт на месте закрытых сведений.
// Зеркало documentMask из attachment_blank_resolver.go: короткий дефис в печатной
// форме теряется рядом с цифрами соседних столбцов.
const documentMaskInBlank = "—"

// blankCell читает ячейку скачанного бланка. Отдельная функция, потому что каждая
// проверка ниже смотрит на одну и ту же книгу с разных сторон.
func blankCell(t *testing.T, body []byte, ref string) string {
	t.Helper()
	out, err := excelize.OpenReader(bytes.NewReader(body))
	require.NoError(t, err)
	defer func() { require.NoError(t, out.Close()) }()
	value, err := out.GetCellValue(out.GetSheetName(0), ref)
	require.NoError(t, err)
	return value
}

// TestBlankDownload_DocumentsGate: бланк уносится из системы файлом, поэтому паспорт,
// патент и иное разрешение попадают в него только у того, кому выданы оба права пары
// (detail.documents и detail.documents.export). Остальным на их месте прочерк - в том
// числе автору заявки, который этих же людей заводил руками.
func TestBlankDownload_DocumentsGate(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	t.Cleanup(cleanup)
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	senderToken := testutil.RegisterAndLogin(t, e, "docsgatesender", "pass123", 1, td.OrgID, td.CompanyID)
	var sender models.User
	require.NoError(t, db.Where("username = ?", "docsgatesender").First(&sender).Error)
	fx := seedDocumentsGateApplication(t, db, td, sender.ID)

	adminToken := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)

	// Ответственный по заявке: доступ к ней есть, но подавал не он.
	responsibleToken := testutil.RegisterAndLogin(t, e, "docsgateresp", "pass123", 1, td.OrgID, td.CompanyID)
	var responsible models.User
	require.NoError(t, db.Where("username = ?", "docsgateresp").First(&responsible).Error)
	require.NoError(t, db.Create(&models.ApplicationResponsibleUser{
		ApplicationID: fx.appID, UserID: responsible.ID,
	}).Error)

	url := fmt.Sprintf("/applications/%d/blank?attachment_id=%d", fx.appID, fx.attID)

	t.Run("инициатор заявки получает документы без права", func(t *testing.T) {
		// Паспорта и патенты участников он сам набрал в форме подачи, из своей же
		// заявки они и уходят - права на собственные сведения он не спрашивает.
		rec := testutil.GET(t, e, url+"&documents=1", testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		body := rec.Body.Bytes()
		assert.Equal(t, "Документов", blankCell(t, body, "A10"))
		assert.Equal(t, fx.passport, blankCell(t, body, "B10"), "паспорт")
		assert.Equal(t, fx.patent, blankCell(t, body, "C10"), "патент")
		assert.Equal(t, fx.other, blankCell(t, body, "D10"), "иное разрешение")
	})

	t.Run("участник чужой заявки без права получает прочерки", func(t *testing.T) {
		// Заявка доступна ему как ответственному, но вводил документы не он: круг
		// открытой выгрузки - именно подавший, а не всякий, кому заявка видна.
		rec := testutil.GET(t, e, url+"&documents=1", testutil.AuthHeader(responsibleToken))
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		body := rec.Body.Bytes()
		assert.Equal(t, "Документов", blankCell(t, body, "A10"), "фамилия документом не является и остаётся на месте")
		assert.Equal(t, documentMaskInBlank, blankCell(t, body, "B10"), "паспорт")
		assert.Equal(t, documentMaskInBlank, blankCell(t, body, "C10"), "патент")
		assert.Equal(t, documentMaskInBlank, blankCell(t, body, "D10"), "иное разрешение")
	})

	t.Run("параметр documents без права ничего не открывает", func(t *testing.T) {
		// Гейт живёт на сервере, а не в модалке: подставленный руками параметр не
		// должен работать, иначе право обходится правкой адресной строки.
		rec := testutil.GET(t, e, url+"&documents=true", testutil.AuthHeader(responsibleToken))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, documentMaskInBlank, blankCell(t, rec.Body.Bytes(), "B10"))
	})

	t.Run("с правом и запрошенным режимом документы на месте", func(t *testing.T) {
		rec := testutil.GET(t, e, url+"&documents=1", testutil.AuthHeader(adminToken))
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		body := rec.Body.Bytes()
		assert.Equal(t, fx.passport, blankCell(t, body, "B10"))
		assert.Equal(t, fx.patent, blankCell(t, body, "C10"))
		assert.Equal(t, fx.other, blankCell(t, body, "D10"))
	})

	t.Run("умолчание закрытое даже у инициатора", func(t *testing.T) {
		// Скачивание без параметра - это старый клиент или прямая ссылка. Вынос
		// персональных данных должен быть выбран явно, а не достаться по умолчанию.
		rec := testutil.GET(t, e, url, testutil.AuthHeader(senderToken))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, documentMaskInBlank, blankCell(t, rec.Body.Bytes(), "B10"))
	})

	t.Run("умолчание закрытое даже при праве", func(t *testing.T) {
		rec := testutil.GET(t, e, url, testutil.AuthHeader(adminToken))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, documentMaskInBlank, blankCell(t, rec.Body.Bytes(), "B10"))
	})

	// Закрытие сохранённого файла проверяется там, где этот файл реально есть:
	// attachment_blank_archive_test.go, «отправитель без права на документы
	// сохранённый файл не получает». Здесь строки реестра нет, и ответом был бы
	// честный 404 - гейт до него просто не доходит.
}
