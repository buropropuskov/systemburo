package handlers_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// Выгрузка реестра заявок (#1832). Проверяется не «файл отдался», а три обещания:
// без права выгрузки нет, в файле ровно то, что человек видит на экране (та же
// выборка и та же маскировка ФИО), и фильтры доезжают до выгрузки.

// exportRegistry скачивает реестр и возвращает содержимое первого листа одной
// строкой - так проверки читаются по смыслу («в файле нет этой фамилии»), а не по
// координатам ячеек, которые поедут при смене порядка колонок.
func exportRegistry(t *testing.T, e *echo.Echo, token, query string) string {
	t.Helper()
	path := "/applications/export"
	if query != "" {
		path += "?" + query
	}
	rec := testutil.GET(t, e, path, testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Content-Disposition"), ".xlsx")

	f, err := excelize.OpenReader(bytes.NewReader(rec.Body.Bytes()))
	require.NoError(t, err, "ответ должен быть читаемым xlsx")
	defer func() { _ = f.Close() }()

	rows, err := f.GetRows(f.GetSheetName(0))
	require.NoError(t, err)
	var sb strings.Builder
	for _, r := range rows {
		sb.WriteString(strings.Join(r, "|"))
		sb.WriteString("\n")
	}
	return sb.String()
}

// grantExport выдаёт право выгрузки лично, минуя роли: тесту нужен человек с
// правом, но без админства - иначе adminAll скрыл бы работу скоупинга видимости.
func grantExport(t *testing.T, db *gorm.DB, username string) {
	t.Helper()
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        userIDByName(t, db, username),
		PermissionKey: services.KeyActionExportApplications,
		Value:         "allow",
	}).Error)
}

func TestApplicationsExport_WithoutPermission_Forbidden(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	user := testutil.RegisterAndLogin(t, e, "exp_plain", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	rec := testutil.GET(t, e, "/applications/export", testutil.AuthHeader(user))
	assert.Equal(t, http.StatusForbidden, rec.Code, "право «Экспорт заявок» обязательно")
}

func TestApplicationsExport_Admin_GetsRegistryWithHeaders(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	sender := testutil.RegisterAndLogin(t, e, "exp_sender", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	createSimpleApplication(t, e, sender, td.OrgID)

	body := exportRegistry(t, e, admin, "")
	assert.Contains(t, body, "Реестр заявок", "заголовок документа")
	assert.Contains(t, body, "Номер заявки", "шапка колонок")
	assert.Contains(t, body, "Людей", "состав заявки числами")
	assert.Contains(t, body, "заявок: 1", "подзаголовок сообщает объём выборки")
}

// Ключевая проверка: выгрузка не должна становиться способом снять маску ФИО у
// тех, кто не давал согласия на обработку персональных данных (#1567).
func TestApplicationsExport_MasksSenderWithoutConsent(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	sender := testutil.RegisterAndLogin(t, e, "exp_masked", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	setUserName(t, db, "exp_masked", "Заявкин", "Афанасий", "Афанасьевич")
	createSimpleApplication(t, e, sender, td.OrgID)

	before := exportRegistry(t, e, admin, "")
	require.Contains(t, before, "Заявкин", "до включения запроса согласия ФИО в файле есть")

	enableConsent(t, e, admin, "<p>Согласие</p>")

	after := exportRegistry(t, e, admin, "")
	assert.NotContains(t, after, "Заявкин", "ФИО без согласия не уезжает в файл")
	assert.Contains(t, after, "@exp_masked", "вместо ФИО в файле логин, как на экране")
}

// Видимость у выгрузки та же, что у списка: заявитель забирает только свои заявки,
// даже имея право выгрузки.
func TestApplicationsExport_ScopedToOwnApplications(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	mine := testutil.RegisterAndLogin(t, e, "exp_mine", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	other := testutil.RegisterAndLogin(t, e, "exp_other", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	grantExport(t, db, "exp_mine")

	myApp := createSimpleApplication(t, e, mine, td.OrgID)
	otherApp := createSimpleApplication(t, e, other, td.OrgID)
	require.NotEqual(t, myApp, otherApp)

	body := exportRegistry(t, e, mine, "")
	assert.Contains(t, body, "заявок: 1", "в файле только своя заявка")
	assert.NotContains(t, body, "@exp_other", "чужая заявка в выгрузку не попадает")
}

// Фильтры доезжают до выгрузки: иначе кнопка «выгрузить» отдавала бы не то, что
// человек отобрал на экране.
func TestApplicationsExport_RespectsDateFilter(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	admin := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	makeApprover(t, db, "testadmin")

	sender := testutil.RegisterAndLogin(t, e, "exp_filtered", "password123456789012345678901234", 1, td.OrgID, td.CompanyID)
	createSimpleApplication(t, e, sender, td.OrgID)

	all := exportRegistry(t, e, admin, "")
	require.Contains(t, all, "заявок: 1")

	// Период, в который заявка не попадает: выборка пуста, но файл всё равно валиден
	// и с шапкой - пустой отчёт это ответ «за период ничего нет», а не ошибка.
	empty := exportRegistry(t, e, admin, "date_from=2000-01-01&date_to=2000-01-31")
	assert.Contains(t, empty, "заявок: 0")
	assert.Contains(t, empty, "Номер заявки")
	assert.Contains(t, empty, "период: 01.01.2000 - 31.01.2000")
}
