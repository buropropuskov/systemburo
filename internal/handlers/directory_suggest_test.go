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

// suggestAnswer возвращает ответ подсказок целиком: записи, канон оформления и признак
// того, что наименование в справочнике уже есть.
func suggestAnswer(t *testing.T, e *echo.Echo, token, path, q string) services.DirectorySuggestAnswer {
	t.Helper()
	rec := testutil.GET(t, e, path+"?q="+url.QueryEscape(q), testutil.AuthHeader(token))
	require.Equal(t, http.StatusOK, rec.Code, "подсказки по %q: %s", q, rec.Body.String())
	return testutil.ParseResponse[services.DirectorySuggestAnswer](t, rec)
}

// suggestNames возвращает наименования подсказок по запросу q.
func suggestNames(t *testing.T, e *echo.Echo, token, path, q string) []string {
	t.Helper()
	answer := suggestAnswer(t, e, token, path, q)
	names := make([]string, 0, len(answer.Items))
	for _, s := range answer.Items {
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

	// Ядро записи снимает Postgres выражением services.DirectoryCoreSQL, ядро запроса
	// считает normalize.OrgNameCore: разъедутся - подсказки молча перестанут находить.
	// Проверяем на живом движке, потому что границы POSIX в Go-regexp не воспроизводятся
	// (\b там только для ASCII), а сравнение подсказок этот класс маскирует: у ядер,
	// отличающихся на служебный префикс, триграммная близость всё равно высокая.
	t.Run("ядро в SQL совпадает с ядром в Go", func(t *testing.T) {
		names := []string{
			`ООО "Максима Групп"`,
			`АО "Регионы-Энтертейнмент"`,
			`ЧОП "АРЕС"`,
			`Технический департамент`,
			`м-н Летуаль`,
			// ОПФ через дефис - часть наименования: словесная граница POSIX резала её,
			// strings.Fields в Go - нет, и ядра расходились.
			`ИП-Сервис`,
			`ООО "Ромашка-Строй"`,
			// Две формы подряд: вторая обязана сняться вслед за первой.
			`ООО ИП Ромашка`,
			// Наименование из одной формы ядра не имеет - остаётся ключом.
			`ООО`,
		}
		for _, name := range names {
			var fromSQL string
			require.NoError(t, db.Raw("SELECT "+services.DirectoryCoreSQL("@key"), map[string]any{
				"key": normalize.OrgName(name),
				"opf": normalize.OrgLegalFormPattern(),
			}).Row().Scan(&fromSQL))
			assert.Equal(t, normalize.OrgNameCore(name), fromSQL, "ядра разошлись на %q", name)
		}
	})

	t.Run("запись с формой через дефис находится по наименованию", func(t *testing.T) {
		hyphen := seedOrg(t, db, `ИП-Сервис`, models.ModerationApproved, true)
		assert.Contains(t, suggestNames(t, e, token, "/organizations/suggest", "ип-сервис"), hyphen.Name)
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

	// Кавычку в наименовании ставят и опускают, а пробел после организационно-правовой
	// формы часто пропускают. Все три написания одного юрлица обязаны находить одну запись,
	// иначе форма предлагает завести дубль (репорт с боя: «ооо"демо-партнёр»).
	t.Run("наименование находится и с кавычками, и без них, и слитно с формой", func(t *testing.T) {
		existing := seedOrg(t, db, `ООО "Кавычки Групп"`, models.ModerationApproved, true)

		for _, writing := range []string{
			`ООО "Кавычки Групп"`, "ООО Кавычки Групп", "ооо кавычки групп",
			`ооо"кавычки групп`, `ООО"Кавычки Групп"`, `ооо «кавычки групп»`,
		} {
			answer := suggestAnswer(t, e, token, "/organizations/suggest", writing)
			require.NotNil(t, answer.Matched, "написание %q: признак совпадения должен быть посчитан", writing)
			assert.True(t, *answer.Matched, "написание %q должно найти существующую запись", writing)
			assert.Contains(t, namesOf(answer.Items), existing.Name, "написание %q", writing)
		}

		// Слитная форма ещё и оформляется как надо: раньше выходило «Ооо"кавычки групп».
		answer := suggestAnswer(t, e, token, "/organizations/suggest", `ооо"кавычки групп`)
		assert.Equal(t, `ООО "Кавычки групп"`, answer.Canonical)
	})

	// Канон оформления и признак «уже есть в справочнике» форма получает вместе с
	// подсказками: правила оформления и ключ дедупликации живут в Go, второй копии на
	// фронте быть не должно.
	t.Run("ответ несёт канон оформления и признак совпадения", func(t *testing.T) {
		existing := seedOrg(t, db, `ООО "Совпадение"`, models.ModerationApproved, true)

		answer := suggestAnswer(t, e, token, "/organizations/suggest", `ооо "братишк`)
		assert.Equal(t, `ООО "Братишк"`, answer.Canonical, "канон оформления считает бэк")
		require.NotNil(t, answer.Matched)
		assert.False(t, *answer.Matched, "такого наименования в справочнике нет")

		// Другое написание существующей записи - тот же ключ, значит подача ляжет на неё.
		answer = suggestAnswer(t, e, token, "/organizations/suggest", "ооо совпадение")
		require.NotNil(t, answer.Matched)
		assert.True(t, *answer.Matched)
		assert.Equal(t, "ООО Совпадение", answer.Canonical)
		assert.Contains(t, namesOf(answer.Items), existing.Name)

		// Канон нужен и на коротком вводе, где подсказок ещё нет: поле подставляет
		// оформление независимо от порога выдачи.
		answer = suggestAnswer(t, e, token, "/organizations/suggest", "ип")
		assert.Equal(t, "ИП", answer.Canonical)
		assert.Empty(t, answer.Items)
		assert.Nil(t, answer.Matched, "по двум буквам в базу не ходим и «такого нет» не утверждаем")

		// Вырожденное наименование (одни кавычки): ключа нет, запись из него не заведётся,
		// и форма обязана сказать это, а не обещать проверку.
		answer = suggestAnswer(t, e, token, "/organizations/suggest", `""`)
		assert.True(t, answer.Degenerate)
		assert.Nil(t, answer.Matched)

		// Дефисы ключ имеют, но содержания в них столько же: подача такое наименование
		// отклоняет, значит и форма обещать создание не должна.
		answer = suggestAnswer(t, e, token, "/organizations/suggest", "---")
		assert.True(t, answer.Degenerate, "наименование без букв и цифр")
		assert.Nil(t, answer.Matched)

		// Черновик чужой заявки в подсказки не идёт, но ключ занимает: подача ляжет на
		// него, новой записи не появится - предупреждать не о чем.
		draft := seedOrg(t, db, `ООО "Черновик ключа"`, models.ModerationPending, true)
		answer = suggestAnswer(t, e, token, "/organizations/suggest", "ооо черновик ключа")
		require.NotNil(t, answer.Matched)
		assert.True(t, *answer.Matched, "ключ занят черновиком")
		assert.NotContains(t, namesOf(answer.Items), draft.Name)
	})
}

// namesOf - наименования подсказок из ответа.
func namesOf(items []services.DirectorySuggestion) []string {
	names := make([]string, 0, len(items))
	for _, s := range items {
		names = append(names, s.Name)
	}
	return names
}
