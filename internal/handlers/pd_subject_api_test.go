package handlers_test

// Раздел «Сведения о субъекте персональных данных» через интерфейс (#2356).
//
// Запросы государственных органов приходят регулярно, а лезть под каждый в консоль
// сервера - плохая практика: доступ к консоли шире, чем нужно для ответа на письмо.
// Поэтому те же операции доступны администратору, и проверяется здесь именно то, что
// отличает интерфейс от консоли: гейты прав и невозможность выдать файл мимо журнала.

import (
	"encoding/json"
	"net/http"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func pdSubjectFixture(t *testing.T, db *gorm.DB) int {
	t.Helper()
	org := models.Organization{Name: "Раздел-эксперимент"}
	require.NoError(t, db.Create(&org).Error)
	last, first := "Разделов", "Кирилл"
	passport := "4510 777888"
	emp := models.UniqueEmployee{
		LastName: &last, FirstName: &first, PassportSeriesNumber: &passport,
		OrganizationID: &org.ID,
	}
	require.NoError(t, db.Create(&emp).Error)
	return emp.ID
}

func TestPDSubjectAPI_ReportAndExport(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	registryID := pdSubjectFixture(t, db)
	h := testutil.AuthHeader(testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID))

	t.Run("поиск по имени находит кандидата", func(t *testing.T) {
		rec := testutil.GET(t, e, "/pd-subject/candidates?fio=Разделов%20Кирилл", h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Разделов")
		// Строки склеены по документу: один человек - один пункт списка, сколько за
		// ним записей, видно счётчиками. Иначе по одному работнику выходило десять
		// пунктов, и выбрать было не из чего.
		assert.Contains(t, rec.Body.String(), `"registry_rows"`)
	})

	t.Run("состав сведений содержит разделы и основание", func(t *testing.T) {
		rec := testutil.GET(t, e, "/pd-subject/report?registry_id="+itoa(registryID), h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var resp struct {
			Data struct {
				Basis    string `json:"basis"`
				Sections []struct {
					Title string     `json:"title"`
					Rows  [][]string `json:"rows"`
				} `json:"sections"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Contains(t, resp.Data.Basis, "пропускного режима")
		assert.NotContains(t, resp.Data.Basis, "152-ФЗ",
			"ссылок на статьи закона в справке быть не должно: квалификацию даёт тот, кто готовит ответ")
		require.Len(t, resp.Data.Sections, 4, "разделы справки: сведения, заявки, проходы, посты")

		// Раздел без строк обязан приходить пустым массивом, а не null: nil-срез
		// уезжает в JSON как null, и экран падает на rows.length - у человека без
		// заявок это роняло всю страницу целиком (поймано на стенде).
		assert.NotContains(t, rec.Body.String(), `"rows":null`,
			"пустой раздел обязан быть [], иначе экран падает на null.length")
	})

	t.Run("выгрузка без реквизитов запроса отклоняется", func(t *testing.T) {
		rec := testutil.POST(t, e, "/pd-subject/export",
			`{"registry_id":`+itoa(registryID)+`,"recipient":"УМВД"}`, h)
		assert.Equal(t, http.StatusBadRequest, rec.Code,
			"выдача без реквизитов запроса не фиксируется в журнале, а значит невозможна")
	})

	t.Run("выгрузка отдаёт файл и попадает в журнал", func(t *testing.T) {
		rec := testutil.POST(t, e, "/pd-subject/export",
			`{"registry_id":`+itoa(registryID)+`,"recipient":"УМВД по г. Москве","request_ref":"исх. 12/345"}`, h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		// Имя справки кириллическое, а заголовки HTTP - ASCII: без filename*=UTF-8''
		// браузер сохраняет файл кракозябрами (поймано ручной проверкой на стенде).
		disposition := rec.Header().Get("Content-Disposition")
		assert.Contains(t, disposition, "filename*=UTF-8''", "кириллическое имя обязано идти закодированным")
		assert.Contains(t, disposition, "%D0%A1%D0%B2%D0%B5%D0%B4%D0%B5%D0%BD%D0%B8%D1%8F",
			"в закодированном имени должно читаться «Сведения»")
		assert.Greater(t, rec.Body.Len(), 0, "файл справки пуст")

		var count int64
		require.NoError(t, db.Model(&models.PDDisclosure{}).Count(&count).Error)
		assert.Equal(t, int64(1), count, "выдача обязана попасть в журнал")

		var entry models.PDDisclosure
		require.NoError(t, db.First(&entry).Error)
		assert.Equal(t, "УМВД по г. Москве", entry.Recipient)
		require.NotNil(t, entry.IssuedByUserID, "в журнале должен остаться человек, а не «интерфейс»")
	})

	t.Run("журнал выдач читается через раздел", func(t *testing.T) {
		rec := testutil.GET(t, e, "/pd-subject/disclosures?registry_id="+itoa(registryID), h)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "УМВД по г. Москве")
	})
}

// TestPDSubjectAPI_GatedByPermissions - раздел закрыт правом, а выгрузка - ещё и
// парным. Без гейта на выгрузке право «посмотреть» открывало бы и вынос файла.
func TestPDSubjectAPI_GatedByPermissions(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	registryID := pdSubjectFixture(t, db)

	testutil.RegisterUser(t, e, "pd-subject-stranger", "password123", 1, td.OrgID, td.CompanyID)
	token, _ := testutil.LoginUser(t, e, "pd-subject-stranger", "password123")
	h := testutil.AuthHeader(token)

	for _, path := range []string{
		"/pd-subject/candidates?fio=Разделов%20Кирилл",
		"/pd-subject/report?registry_id=" + itoa(registryID),
		"/pd-subject/disclosures",
	} {
		rec := testutil.GET(t, e, path, h)
		assert.Equal(t, http.StatusForbidden, rec.Code, "посторонний не должен читать %s", path)
	}

	rec := testutil.POST(t, e, "/pd-subject/export",
		`{"registry_id":`+itoa(registryID)+`,"recipient":"УМВД","request_ref":"исх. 1"}`, h)
	assert.Equal(t, http.StatusForbidden, rec.Code, "и выгружать справку тоже")
}
