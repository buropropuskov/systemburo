package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"systemburo/internal/database"
	"systemburo/internal/models"
	"systemburo/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestOrgNameNormalized покрывает ключ дедупликации наименований (#1437) целиком.
// Секции живут на одном SetupTestApp: пакет handlers идёт в CI под -race у самой
// границы go test -timeout, и шесть отдельных тестов с собственными CleanDB и Seed
// уже перебивали её (#1437, следом за уроком про таймаут handlers).
func TestOrgNameNormalized(t *testing.T) {
	e, db, cleanup := testutil.SetupTestApp(t)
	defer cleanup()
	testutil.CleanDB(t, db)
	td := testutil.SeedTestData(t, db)
	token := testutil.RegisterAdmin(t, e, td.OrgID, td.CompanyID)
	auth := testutil.AuthHeader(token)

	t.Run("ключ проставляется при создании", func(t *testing.T) {
		rec := testutil.POST(t, e, "/organizations", `{"name":"ООО \"Петрушка\"","type":"Подрядчик"}`, auth)
		require.Equal(t, http.StatusOK, rec.Code)

		var org models.Organization
		require.NoError(t, db.Where("name = ?", `ООО "Петрушка"`).First(&org).Error)
		assert.Equal(t, "ооо петрушка", org.NameNormalized)
	})

	t.Run("другое написание не создаёт вторую запись", func(t *testing.T) {
		for _, writing := range []string{`ооо петрушка`, `ООО Петрушка`, `Общество с ограниченной ответственностью "Петрушка"`} {
			rec := testutil.POST(t, e, "/organizations", `{"name":"`+writing+`","type":"Подрядчик"}`, auth)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "написание %q должно упереться в существующую запись", writing)
		}

		var count int64
		require.NoError(t, db.Model(&models.Organization{}).Where("name_normalized = ?", "ооо петрушка").Count(&count).Error)
		assert.Equal(t, int64(1), count)
	})

	// Update идёт map-обновлением, куда хук модели не достаёт: если не записать поле
	// явно, ключ останется от старого имени и дедупликация начнёт врать.
	t.Run("переименование пересчитывает ключ", func(t *testing.T) {
		rec := testutil.POST(t, e, "/organizations", `{"name":"ООО \"Старое\"","type":"Подрядчик"}`, auth)
		require.Equal(t, http.StatusOK, rec.Code)
		id := int(testutil.ParseMap(t, rec)["id"].(float64))

		require.Equal(t, http.StatusOK,
			testutil.PUT(t, e, "/organizations/"+strconv.Itoa(id), `{"name":"ЗАО \"Новое\"","type":"Подрядчик"}`, auth).Code)

		var org models.Organization
		require.NoError(t, db.First(&org, id).Error)
		assert.Equal(t, "зао новое", org.NameNormalized)
	})

	// Сервисы организаций и компаний зеркальны, и расхождение между ними тихое:
	// компанию с другим написанием заводит тот же экран справочника.
	t.Run("у компаний дедупликация работает так же", func(t *testing.T) {
		require.Equal(t, http.StatusOK,
			testutil.POST(t, e, "/companies", `{"name":"ООО \"Ромашка\"","type":"Подрядчик"}`, auth).Code)
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/companies", `{"name":"ооо ромашка","type":"Подрядчик"}`, auth).Code)

		var company models.Company
		require.NoError(t, db.Where("name = ?", `ООО "Ромашка"`).First(&company).Error)
		assert.Equal(t, "ооо ромашка", company.NameNormalized)
	})

	// Наименование без букв и цифр не заводится нигде: ни подачей, ни разбором, ни из
	// админского справочника. Раньше кавычки отклонялись пустым ключом, а «--» проходило
	// и оставалось в справочнике мусором, с которым потом ничего не сделать.
	t.Run("наименование без букв и цифр не создаётся", func(t *testing.T) {
		for _, junk := range []string{`\"`, "--", "...", "!!!"} {
			rec := testutil.POST(t, e, "/organizations", `{"name":"`+junk+`","type":"Подрядчик"}`, auth)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "наименование %q: %s", junk, rec.Body.String())
		}
		assert.Equal(t, http.StatusBadRequest,
			testutil.POST(t, e, "/companies", `{"name":"---","type":"Подрядчик"}`, auth).Code,
			"у компаний правило то же")

		// Переименование в такое наименование тоже не проходит.
		rec := testutil.POST(t, e, "/organizations", `{"name":"ООО Переименуемая","type":"Подрядчик"}`, auth)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		id := int(testutil.ParseMap(t, rec)["id"].(float64))
		assert.Equal(t, http.StatusBadRequest,
			testutil.PUT(t, e, "/organizations/"+strconv.Itoa(id), `{"name":"---","type":"Подрядчик"}`, auth).Code)
	})

	// Бэкфилл нужен записям, созданным в обход хука, и прогоняется на каждом старте,
	// поэтому повторный проход обязан быть безобидным.
	t.Run("бэкфилл заполняет ключ и идемпотентен", func(t *testing.T) {
		require.NoError(t, db.Exec(
			`INSERT INTO organizations (name, type, is_active, name_normalized) VALUES (?, ?, true, '')`,
			`ООО "Без ключа"`, models.OrgTypeContractor).Error)
		require.NoError(t, db.Exec(
			`INSERT INTO companies (name, type, is_active, name_normalized) VALUES (?, ?, true, '')`,
			`ЗАО «Тоже без ключа»`, models.OrgTypeContractor).Error)

		require.NoError(t, database.BackfillOrgNameNormalized(db))

		var org models.Organization
		require.NoError(t, db.Where("name = ?", `ООО "Без ключа"`).First(&org).Error)
		assert.Equal(t, "ооо без ключа", org.NameNormalized)

		var company models.Company
		require.NoError(t, db.Where("name = ?", `ЗАО «Тоже без ключа»`).First(&company).Error)
		assert.Equal(t, "зао тоже без ключа", company.NameNormalized)

		require.NoError(t, database.BackfillOrgNameNormalized(db))
		var again models.Organization
		require.NoError(t, db.Where("name = ?", `ООО "Без ключа"`).First(&again).Error)
		assert.Equal(t, org.NameNormalized, again.NameNormalized)
	})

	// Оформление наименования держит система, а не аккуратность вводящего: заявку от
	// «ооо "братишк» уже подавали, и в справочник уезжало ровно это (#1437).
	t.Run("оформление наименования канонизируется", func(t *testing.T) {
		rec := testutil.POST(t, e, "/organizations", `{"name":"ооо \"братишк","type":"Подрядчик"}`, auth)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		id := int(testutil.ParseMap(t, rec)["id"].(float64))

		var org models.Organization
		require.NoError(t, db.First(&org, id).Error)
		assert.Equal(t, `ООО "Братишк"`, org.Name, "ОПФ заглавными, название с заглавной, кавычка закрыта")
		assert.Equal(t, "ооо братишк", org.NameNormalized, "ключ дедупликации канонизация не меняет")

		// Не только ООО: форма берётся из общего списка ОПФ.
		require.Equal(t, http.StatusOK,
			testutil.PUT(t, e, "/organizations/"+strconv.Itoa(id), `{"name":"зао братишк","type":"Подрядчик"}`, auth).Code)
		require.NoError(t, db.First(&org, id).Error)
		assert.Equal(t, "ЗАО Братишк", org.Name)

		// Наименование без ОПФ получает заглавную первого слова, а служебное сокращение
		// справочника остаётся строчным.
		rec = testutil.POST(t, e, "/companies", `{"name":"м-н летуаль-два","type":"Подрядчик"}`, auth)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var company models.Company
		require.NoError(t, db.Where("name_normalized = ?", "м-н летуаль-два").First(&company).Error)
		assert.Equal(t, "м-н Летуаль-два", company.Name)
	})

	// Дубль сервис ищет запросом перед записью, но между проверкой и записью блокировки
	// нет: два админа, правящих одно наименование одновременно, проходят проверку оба, и
	// второго отбивает уникальный индекс. Пользователь должен увидеть тот же понятный
	// текст, что и при обычной проверке, а не 500.
	t.Run("конфликт индекса отдаётся понятной ошибкой, а не пятисоткой", func(t *testing.T) {
		t.Run("создание", func(t *testing.T) {
			rec := underRivalKey(t, db, `ООО "Гонка создания"`, "ооо гонка создания", func() *httptest.ResponseRecorder {
				return testutil.POST(t, e, "/organizations", `{"name":"ООО Гонка создания","type":"Подрядчик"}`, auth)
			})
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), "уже существует")
		})

		t.Run("переименование", func(t *testing.T) {
			rec := testutil.POST(t, e, "/organizations", `{"name":"ООО Гонка до переименования","type":"Подрядчик"}`, auth)
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			id := int(testutil.ParseMap(t, rec)["id"].(float64))

			got := underRivalKey(t, db, `ООО "Гонка переименования"`, "ооо гонка переименования", func() *httptest.ResponseRecorder {
				return testutil.PUT(t, e, "/organizations/"+strconv.Itoa(id),
					`{"name":"ООО Гонка переименования","type":"Подрядчик"}`, auth)
			})
			assert.Equal(t, http.StatusBadRequest, got.Code, got.Body.String())
			assert.Contains(t, got.Body.String(), "уже существует")
		})

		t.Run("восстановление из архива", func(t *testing.T) {
			require.NoError(t, insertOrgRaw(db, "ООО Гонка восстановления", "ооо гонка восстановления", false))
			var archived models.Organization
			require.NoError(t, db.Where("name = ?", "ООО Гонка восстановления").First(&archived).Error)

			got := underRivalKey(t, db, `ООО "Гонка восстановления"`, "ооо гонка восстановления", func() *httptest.ResponseRecorder {
				return testutil.POST(t, e, "/organizations/"+strconv.Itoa(archived.ID)+"/restore", ``, auth)
			})
			assert.Equal(t, http.StatusBadRequest, got.Code, got.Body.String())
			assert.Contains(t, got.Body.String(), "уже существует")
		})
	})

	// Канонизация трогает наименование только когда его РЕАЛЬНО меняют. Групповая смена
	// типа передаёт в Update текущее имя записи, и легаси-наименование из старых данных
	// не должно от этого поехать, а история - получить ложное «переименована».
	t.Run("групповая смена типа не переименовывает записи", func(t *testing.T) {
		require.NoError(t, insertOrgRaw(db, `ооо "легаси`, "ооо легаси", true))
		var legacy models.Organization
		require.NoError(t, db.Where("name = ?", `ооо "легаси`).First(&legacy).Error)

		rec := testutil.POST(t, e, "/organizations/bulk/type",
			`{"ids":[`+strconv.Itoa(legacy.ID)+`],"type":"Арендатор"}`, auth)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		var after models.Organization
		require.NoError(t, db.First(&after, legacy.ID).Error)
		assert.Equal(t, `ооо "легаси`, after.Name, "смена типа не должна переписывать наименование")
		require.NotNil(t, after.Type)
		assert.Equal(t, "Арендатор", *after.Type)

		// Сохранение той же записи без правки имени тоже её не меняет.
		require.Equal(t, http.StatusOK, testutil.PUT(t, e, "/organizations/"+strconv.Itoa(legacy.ID),
			`{"name":"ооо \"легаси","type":"Подрядчик"}`, auth).Code)
		require.NoError(t, db.First(&after, legacy.ID).Error)
		assert.Equal(t, `ооо "легаси`, after.Name)
	})

	// Индекс - последний рубеж дедупликации: проверки в сервисах гонку двух
	// одновременных подач с одним новым наименованием не ловят, между их SELECT и INSERT
	// никакой блокировки нет.
	t.Run("уникальный индекс держит один ключ на одну активную запись", func(t *testing.T) {
		require.True(t, indexExists(t, db, database.OrgNameKeyIndexName("organizations")))
		require.True(t, indexExists(t, db, database.OrgNameKeyIndexName("companies")))

		require.NoError(t, insertOrgRaw(db, `ООО "Индекс"`, "ооо индекс", true))
		// INSERT в обход сервиса - именно то, чего проверки в коде не видят.
		assert.Error(t, insertOrgRaw(db, "ООО Индекс", "ооо индекс", true),
			"второй активной записи с тем же ключом быть не должно")

		assert.NoError(t, insertOrgRaw(db, "ЗАО Индекс архивный", "ооо индекс", false),
			"архивный тёзка индексу не мешает: иначе архив блокировал бы создание активной записи (#412)")
		assert.NoError(t, insertOrgRaw(db, `"""`, "", true))
		assert.NoError(t, insertOrgRaw(db, `---`, "", true),
			"вырожденные наименования индекс не индексирует - их сверяет код по точной строке")
	})

	// На базе с неслитыми дублями индекс не создать, а падать при запуске нельзя: слить
	// дубли можно только через разбор справочника, а он недоступен, пока сервер не поднялся.
	t.Run("коллизии не роняют запуск, индекс встаёт после слияния", func(t *testing.T) {
		withoutOrgNameKeyIndex(t, db)
		index := database.OrgNameKeyIndexName("organizations")

		require.NoError(t, insertOrgRaw(db, `ООО "Коллизия"`, "ооо коллизия", true))
		require.NoError(t, insertOrgRaw(db, "ООО Коллизия", "ооо коллизия", true))

		require.NoError(t, database.BackfillOrgNameNormalized(db), "запуск не должен падать из-за дублей")
		assert.False(t, indexExists(t, db, index), "с дублями индекс не поставить")

		require.NoError(t, db.Exec(`DELETE FROM organizations WHERE name = ?`, "ООО Коллизия").Error)
		require.NoError(t, database.BackfillOrgNameNormalized(db))
		assert.True(t, indexExists(t, db, index), "дубль слит - индекс должен встать сам")
	})

	// Правила нормализации уточняются, и пересчёт может привести запись к ключу, который
	// уже занят: UPDATE отбился бы индексом и уронил запуск.
	t.Run("пересчёт в занятый ключ не роняет бэкфилл", func(t *testing.T) {
		require.NoError(t, insertOrgRaw(db, `ООО "Занятый"`, "ооо занятый", true))
		// Ключ записан вручную «старым» значением, а нормализация даст тот же «ооо занятый».
		require.NoError(t, insertOrgRaw(db, "ООО Занятый", "ооо занятый старый ключ", true))

		require.NoError(t, database.BackfillOrgNameNormalized(db))

		var kept models.Organization
		require.NoError(t, db.Where("name = ?", "ООО Занятый").First(&kept).Error)
		assert.Equal(t, "ооо занятый старый ключ", kept.NameNormalized,
			"запись остаётся с прежним ключом, а пара уходит в отчёт коллизий")
	})

	// Конфликтом считается и черновик: ключ он занимает так же, и без этой проверки
	// UPDATE упёрся бы в индекс, а пользователь получил бы 500 вместо объяснения.
	t.Run("переименование в наименование другого черновика отбивается с объяснением", func(t *testing.T) {
		first := seedModerationOrg(t, db, "Черновик Первый", models.ModerationPending)
		second := seedModerationOrg(t, db, "Черновик Второй", models.ModerationPending)

		rec := testutil.PATCH(t, e,
			"/organizations/"+strconv.Itoa(second.ID)+"/moderation/rename",
			`{"name":"`+first.Name+`"}`, auth)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), "ждёт разбора")

		var unchanged models.Organization
		require.NoError(t, db.First(&unchanged, second.ID).Error)
		assert.Equal(t, "Черновик Второй", unchanged.Name)
		assert.Equal(t, models.ModerationPending, unchanged.ModerationStatus)
	})
}

// withoutOrgNameKeyIndex снимает partial unique index по ключу дедупликации на время
// секции и возвращает его бэкфиллом после. Нужен тем проверкам, что описывают поведение
// на базе с неслитыми дублями: с индексом такое состояние уже не создать.
func withoutOrgNameKeyIndex(t *testing.T, db *gorm.DB) {
	t.Helper()
	var lastID int
	require.NoError(t, db.Raw("SELECT COALESCE(MAX(id), 0) FROM organizations").Scan(&lastID).Error)
	require.NoError(t, db.Exec("DROP INDEX IF EXISTS "+database.OrgNameKeyIndexName("organizations")).Error)
	t.Cleanup(func() {
		// Записи секции удаляются целиком: среди них дубли по ключу, с которыми индекс
		// не встанет, а следующим секциям он нужен.
		require.NoError(t, db.Exec("DELETE FROM organizations WHERE id > ?", lastID).Error)
		require.NoError(t, database.BackfillOrgNameNormalized(db))
	})
}

// indexExists отвечает, есть ли индекс с таким именем в текущей схеме.
func indexExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var cnt int64
	require.NoError(t, db.Raw("SELECT COUNT(*) FROM pg_indexes WHERE indexname = ?", name).Scan(&cnt).Error)
	return cnt > 0
}

// underRivalKey воспроизводит гонку двух правок одного наименования: соперник держит
// незакоммиченную запись с ключом key, поэтому проверка дубля в сервисе её не видит, а
// уникальный индекс отбивает запись. Запрос уходит в горутине и виснет на блокировке,
// коммит соперника его отпускает - без этого ожидания гонка не воспроизводится, и тест
// зеленел бы, ничего не проверив.
func underRivalKey(t *testing.T, db *gorm.DB, rivalName, key string, request func() *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	t.Helper()
	rival := db.Begin()
	require.NoError(t, rival.Error)
	defer rival.Rollback()
	require.NoError(t, rival.Exec(
		`INSERT INTO organizations (name, type, is_active, name_normalized, moderation_status) VALUES (?, ?, true, ?, ?)`,
		rivalName, models.OrgTypeContractor, key, models.ModerationApproved).Error)

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- request() }()

	waitForIndexLock(t, db)
	require.NoError(t, rival.Commit().Error)
	return <-done
}

// insertOrgRaw вставляет организацию напрямую, минуя сервис и хук модели: только так
// проверяется сам индекс, а не проверки в коде поверх него.
func insertOrgRaw(db *gorm.DB, name, key string, active bool) error {
	return db.Exec(
		`INSERT INTO organizations (name, type, is_active, name_normalized, moderation_status) VALUES (?, ?, ?, ?, ?)`,
		name, models.OrgTypeContractor, active, key, models.ModerationApproved).Error
}
