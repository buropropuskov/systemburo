package handlers_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"systemburo/internal/models"
	"systemburo/internal/normalize"
	"systemburo/internal/services"
	"systemburo/internal/testutil"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// suggestNames возвращает наименования подсказок по запросу q.
func suggestNames(t *testing.T, e *echo.Echo, token, path, q string) []string {
	t.Helper()
	rec := testutil.GET(t, e, path+"?q="+url.QueryEscape(q), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "подсказки по %q: %s", q, rec.Body.String())
	suggestions := testutil.ParseResponse[[]services.DirectorySuggestion](t, rec)
	names := make([]string, 0, len(suggestions))
	for _, s := range suggestions {
		names = append(names, s.Name)
	}
	return names
}

// seedOrg заводит организацию с заданным статусом разбора и признаком активности.
func seedOrg(t *testing.T, db *gorm.DB, name, moderation string, active bool) models.Organization {
	t.Helper()
	orgType := models.OrgTypeContractor
	org := models.Organization{Name: name, Type: &orgType, IsActive: true, ModerationStatus: moderation}
	require.NoError(t, db.Create(&org).Error)
	if !active {
		// is_active объявлен с default:true, поэтому false на Create gorm пропускает
		// как нулевое значение и запись создаётся активной - гасим отдельным update.
		require.NoError(t, db.Model(&org).Update("is_active", false).Error)
		org.IsActive = false
	}
	return org
}

// TestDirectorySuggest покрывает подсказки справочников организаций и компаний (#1437):
// гейт по праву, порог близости, отбор только проверенных записей и совпадение ядра,
// которое считает Postgres, с ядром normalize.OrgNameCore.
// Секции живут на одном SetupTestApp: пакет handlers идёт в CI под -race у самой границы
// go test -timeout, и отдельные тесты со своими CleanDB и Seed её уже перебивали.
func TestDirectorySuggest(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)

	plainToken := testutil.RegisterAndLogin(t, e, "suggestplain", "pass123", 1, td.OrgID, td.CompanyID)
	token := testutil.RegisterAndLogin(t, e, "suggestor", "pass123", 1, td.OrgID, td.CompanyID)

	var suggestor models.User
	require.NoError(t, db.Where("username = ?", "suggestor").First(&suggestor).Error)
	require.NoError(t, db.Create(&models.UserPermissionOverride{
		UserID:        suggestor.ID,
		PermissionKey: services.KeyApplicationOrganizationOverride,
		Value:         "allow",
	}).Error)

	// Наименования сняты с реального справочника staging: на них калибровался порог.
	seedOrg(t, db, `ООО "Максима Групп"`, models.ModerationApproved, true)
	seedOrg(t, db, `ООО "Победа"`, models.ModerationApproved, true)
	seedOrg(t, db, `АО "Регионы-Энтертейнмент"`, models.ModerationApproved, true)
	seedOrg(t, db, `ЧОП "АРЕС"`, models.ModerationApproved, true)
	seedOrg(t, db, `Технический департамент`, models.ModerationApproved, true)
	pending := seedOrg(t, db, `ООО "Максима-Черновик"`, models.ModerationPending, true)
	archived := seedOrg(t, db, `ООО "Максима-Архив"`, models.ModerationApproved, false)

	t.Run("без права эндпоинт закрыт", func(t *testing.T) {
		rec := testutil.GET(t, e, "/organizations/suggest?q=максима", testutil.AuthHeader(plainToken))
		assert.Equal(t, http.StatusForbidden, rec.Code, "подсказки открыты только тем, кто вправе указать чужую организацию")

		rec = testutil.GET(t, e, "/companies/suggest?q=максима", testutil.AuthHeader(plainToken))
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("ядро запроса короче трёх символов даёт пустой список", func(t *testing.T) {
		for _, q := range []string{"", "ма", "ООО ма", "ООО"} {
			assert.Empty(t, suggestNames(t, e, token, "/organizations/suggest", q), "запрос %q не должен ничего подсказывать", q)
		}
	})

	t.Run("часть наименования находит запись", func(t *testing.T) {
		assert.Contains(t, suggestNames(t, e, token, "/organizations/suggest", "максима"), `ООО "Максима Групп"`)
	})

	t.Run("опечатка находит запись", func(t *testing.T) {
		assert.Contains(t, suggestNames(t, e, token, "/organizations/suggest", "побега"), `ООО "Победа"`,
			"порог близости обязан пропускать опечатку в одну букву")
	})

	t.Run("организационно-правовая форма в запросе не мешает", func(t *testing.T) {
		// Сравниваются ядра, поэтому «ООО Максима» и «ЗАО Максима» дают одну подсказку:
		// это список выбора, а не привязка - выбирает пользователь.
		for _, q := range []string{"ООО Максима", "ЗАО Максима", `Общество с ограниченной ответственностью "Максима"`} {
			assert.Contains(t, suggestNames(t, e, token, "/organizations/suggest", q), `ООО "Максима Групп"`, "запрос %q", q)
		}
	})

	t.Run("запись на проверке и архивная не подсказываются", func(t *testing.T) {
		names := suggestNames(t, e, token, "/organizations/suggest", "максима")
		assert.NotContains(t, names, pending.Name, "черновик заявителя нельзя предлагать остальным")
		assert.NotContains(t, names, archived.Name)
	})

	t.Run("неродственное наименование не подсказывается", func(t *testing.T) {
		assert.Empty(t, suggestNames(t, e, token, "/organizations/suggest", "ромашка"))
	})

	t.Run("шаблон LIKE в запросе не вытаскивает справочник", func(t *testing.T) {
		for _, q := range []string{"%%%", "___", `\\\`} {
			assert.Empty(t, suggestNames(t, e, token, "/organizations/suggest", q), "запрос %q должен экранироваться", q)
		}
	})

	t.Run("длинный ввод обрезается и не ломает поиск", func(t *testing.T) {
		long := `ООО "Максима Групп"` + strings.Repeat(" очень длинный хвост", 40)
		assert.Contains(t, suggestNames(t, e, token, "/organizations/suggest", long), `ООО "Максима Групп"`)
	})

	t.Run("не больше пяти подсказок", func(t *testing.T) {
		for _, suffix := range []string{"один", "два", "три", "четыре", "пять", "шесть", "семь"} {
			seedOrg(t, db, `ООО "Лимит `+suffix+`"`, models.ModerationApproved, true)
		}
		assert.Len(t, suggestNames(t, e, token, "/organizations/suggest", "лимит"), 5)
	})

	// Ядро записи снимает Postgres по normalize.OrgLegalFormPattern, ядро запроса считает
	// normalize.OrgNameCore: разъедутся - подсказки перестанут находить, а Go-юнит границы
	// слова POSIX не воспроизводит (\b в Go-regexp только для ASCII).
	t.Run("ядро в SQL совпадает с ядром в Go", func(t *testing.T) {
		names := []string{
			`ООО "Максима Групп"`,
			`АО "Регионы-Энтертейнмент"`,
			`ЧОП "АРЕС"`,
			`Технический департамент`,
		}
		for _, name := range names {
			core := normalize.OrgNameCore(name)
			assert.Contains(t, suggestNames(t, e, token, "/organizations/suggest", core), name,
				"запись %q должна находиться по собственному ядру %q", name, core)
		}
	})

	t.Run("компании подсказываются так же", func(t *testing.T) {
		compType := models.OrgTypeContractor
		company := models.Company{Name: `ООО "Парк развлечений"`, Type: &compType, IsActive: true, ModerationStatus: models.ModerationApproved}
		require.NoError(t, db.Create(&company).Error)
		pendingCompany := models.Company{Name: `ООО "Парк черновик"`, Type: &compType, IsActive: true, ModerationStatus: models.ModerationPending}
		require.NoError(t, db.Create(&pendingCompany).Error)

		names := suggestNames(t, e, token, "/companies/suggest", "парк развлечений")
		assert.Contains(t, names, company.Name)
		assert.NotContains(t, names, pendingCompany.Name)
	})
}
